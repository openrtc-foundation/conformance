package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

func TestInspectionRetainsConfigurationAndFailures(t *testing.T) {
	ctx := context.Background()
	client := newFakeClient(openrtc.ClientConfig{}, newWorld(), NoDefects)
	inspection := client.Inspection()
	revisions := 0
	stop := inspection.Watch(func(openrtc.OwnerInspectionSnapshot) { revisions++ })
	defer stop()
	op := client.Configure(ctx, openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "debug"}})
	if op.OperationID() == "" {
		t.Fatal("missing pre-settlement operation ID")
	}
	if _, err := op.Result(ctx); err != nil {
		t.Fatal(err)
	}
	record, err := inspection.Operation(op.OperationID())
	if err != nil || record.Status != "retained" || record.Details == nil || record.Details.Configuration.Value == nil {
		t.Fatalf("retained configuration: %+v %v", record, err)
	}
	if got := record.Details.Configuration.Value.Outcome.Fields[0].Requested; got.Value == nil || *got.Value != "debug" {
		t.Fatalf("configuration: %+v", got)
	}
	record.Details.Configuration.Value.Outcome.Fields[0].Reason = "mutated"
	reread, err := inspection.Operation(op.OperationID())
	if err != nil || reread.Details.Configuration.Value.Outcome.Fields[0].Reason == "mutated" {
		t.Fatal("operation inspection is mutable")
	}
	if len(inspection.Snapshot().Configuration.Fields) != 1 || revisions != 2 {
		t.Fatalf("inspection did not publish configuration; revisions=%d", revisions)
	}
	if _, err = client.Dispose(ctx).Result(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = inspection.Operation(op.OperationID()); err != nil {
		t.Fatal("disposed owner lost operation", err)
	}
	if _, err = inspection.Operation("never-issued"); err == nil {
		t.Fatal("accepted foreign operation")
	}

	freshClient := newFakeClient(openrtc.ClientConfig{}, newWorld(), NoDefects)
	session := freshClient.OpenSession(unitConferenceTarget())
	failed := session.Media().SetMuted(ctx, "unknown-publication", true)
	if _, err = failed.Result(ctx); err == nil {
		t.Fatal("unknown media mutation succeeded")
	}
	if record, err = session.Inspection().Operation(failed.OperationID()); err != nil || record.Details == nil || record.Details.OperationID != failed.OperationID() {
		t.Fatalf("failed operation correlation: %+v %v", record, err)
	}
	initialization := makeRegistry(newWorld(), NoDefects).Initialize(ctx, "missing", openrtc.ProviderConfig{})
	if _, err = initialization.Result(ctx); err == nil {
		t.Fatal("unknown adapter initialization succeeded")
	}
	if record, err = initialization.Inspection().Operation(initialization.OperationID()); err != nil || record.Status != "retained" {
		t.Fatalf("failed initialization lost inspection: %+v %v", record, err)
	}

}

func TestInspectionEvictionAndStableAttemptResult(t *testing.T) {
	a := newFakeAttempt("g")
	first, second := a.Result(context.Background()), a.Result(context.Background())
	if first.OperationID() != second.OperationID() {
		t.Fatal("attempt result allocated a new operation")
	}
	a.done.Resolve(openrtc.JoinResult{})
	if _, err := first.Result(context.Background()); err != nil {
		t.Fatal(err)
	}
	client := newFakeClient(openrtc.ClientConfig{}, newWorld(), NoDefects)
	store := storeFor(client)
	store.mu.Lock()
	store.evicted[first.OperationID()] = true
	store.mu.Unlock()
	record, err := client.Inspection().Operation(first.OperationID())
	if err != nil || record.Status != "evicted" || record.OperationID != first.OperationID() || record.Details != nil {
		t.Fatalf("eviction: %+v %v", record, err)
	}
}

func TestFakeObservationUsesInjectedClock(t *testing.T) {
	const instant openrtc.Instant = "2001-02-03T04:05:06.000Z"
	target := MakeFakeTargetWithClock(NoDefects, func() openrtc.Instant { return instant })
	client := newFakeClient(openrtc.ClientConfig{}, target.world, NoDefects)
	session := newFakeSession(client, openrtc.ConferenceTarget{}, target.world, NoDefects)
	observed := openrtc.Instant("")
	session.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) { observed = e.ObservedAt })
	session.setState("preparing")
	if observed != instant {
		t.Fatalf("timestamp %q", observed)
	}
	sample, err := (fakeStatistics{owner: session}).Sample(context.Background(), openrtc.StatisticsTarget{Kind: "session"}).Result(context.Background())
	if err != nil || sample.CollectedAt != instant {
		t.Fatalf("sample: %+v %v", sample, err)
	}
}
