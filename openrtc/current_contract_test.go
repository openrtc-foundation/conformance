package openrtc

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCurrentIDLJSONRoundTrips(t *testing.T) {
	cases := []struct {
		name, input string
		target      any
	}{
		{"call transferred", `{"status":"confirmed","nativeReason":{"status":"unknown"},"protocolStatus":{"status":"unknown"},"error":{"status":"unknown"},"terminalCause":{"status":"known","value":"transferred"}}`, &CallOutcome{}},
		{"effects", `{"guaranteed":{"capture":{"status":"unknown"},"outbound":{"status":"known","value":"suppressed"},"publication":{"status":"known","value":"retained"},"receive":{"status":"known","value":"unchanged"},"reacquisitionRequired":{"status":"unknown"},"recording":{"status":"unknown"},"playback":{"status":"unknown"}},"observed":{"capture":{"status":"unknown"},"outbound":{"status":"unknown"},"publication":{"status":"unknown"},"receive":{"status":"unknown"},"reacquisitionRequired":{"status":"unknown"},"recording":{"status":"unknown"},"playback":{"status":"unknown"}},"collateral":[]}`, &MediaOperationEffects{}},

		{"evicted", `{"status":"evicted","operationId":"op-1"}`, &OperationInspectionResult{}},
		{"publication reference", `{"kind":"publication","publication":"p"}`, &ReceiveSourceReference{}},
		{"aggregate reference", `{"kind":"aggregate","id":"r"}`, &ReceiveSourceReference{}},
		{"aggregate view", `{"kind":"aggregate","id":"r","generation":"g","nativeId":{"status":"unknown"},"kinds":["audio"],"contributors":{"status":"unknown"},"coverage":"unknown"}`, &ReceiveSourceView{}},
		{"call receive view", `{"kind":"call-media","leg":"l","direction":"receive","kinds":["audio"]}`, &ReceiveSourceView{}},
		{"initial capture", `{"kind":"capture","request":{"kinds":["audio"],"ownership":"session"}}`, &InitialMediaRequest{}},
		{"initial send", `{"kind":"send","request":{"kinds":["audio"],"sending":true}}`, &InitialMediaRequest{}},
		{"receive request", `{"audio":"automatic","video":"none","requiredEffects":{"receive":"selected"}}`, &ReceivePolicyRequest{}},
		{"publish kinds", `{"kinds":["audio","video"],"requiredEffects":{"capture":"retained"}}`, &PublishOptions{}},
		{"receive observation", `{"audio":{"selection":"automatic","authority":{"status":"unknown"},"adaptation":{"status":"unknown"},"controls":[]},"video":{"selection":"not-applicable","authority":{"status":"unknown"},"adaptation":{"status":"unknown"},"controls":[]}}`, &ReceivePolicy{}},
		{"remote update", `{"id":"u","changes":{"status":"known","value":{"kindsAdded":["video"],"kindsRemoved":[],"holdChanged":false}},"policy":"accept-without-new-kinds","decision":"accepted-without-new-kinds"}`, &RenegotiationUpdate{}},
		{"continuity", `{"publicIdentity":"preserved","nativeIdentity":"replaced"}`, &RecoveryContinuity{}},
		{"dtmf", `{"leg":"l","digits":"1#","method":"rtp-event","fallback":["signaling"],"duration":{"milliseconds":100},"gap":{"milliseconds":50}}`, &DtmfRequest{}},
		{"session runtime", `{"target":"media","request":{"command":"set-disabled","source":"s","disabled":false}}`, &SessionRuntimeCommand{}},
		{"client no-op", `{"target":"client"}`, &ClientRuntimeCommand{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tc.input), tc.target); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			_ = json.Unmarshal([]byte(tc.input), &want)
			_ = json.Unmarshal(data, &got)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("got %s; want %s", data, tc.input)
			}
		})
	}
}
func TestCurrentIDLRejectsMixedVariants(t *testing.T) {
	cases := []struct {
		input  string
		target any
	}{
		{`{"status":"evicted","operationId":"o","details":{}}`, &OperationInspectionResult{}},
		{`{"status":"retained","operationId":"o"}`, &OperationInspectionResult{}},
		{`{"kind":"publication","publication":"p","id":"r"}`, &ReceiveSourceReference{}},
		{`{"kind":"aggregate","id":"r"}`, &ReceiveSourceView{}},
		{`{"target":"client","request":{"command":"set-muted"}}`, &ClientRuntimeCommand{}},
		{`{"target":"client","diagnostics":{"level":"debug"}}`, &SessionRuntimeCommand{}},
		{`"add-anything"`, new(RemoteUpdatePolicy)},
	}
	for _, tc := range cases {
		if err := json.Unmarshal([]byte(tc.input), tc.target); err == nil {
			t.Errorf("accepted %s as %T", tc.input, tc.target)
		}
	}
}
func TestAsyncIdentityAndSettlement(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	operation := StartOperation(nil, func() (string, error) { close(entered); <-release; return "done", nil })
	if operation.OperationID() == "" {
		t.Fatal("empty operation ID")
	}
	<-entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := operation.Result(ctx); err != context.Canceled {
		t.Fatalf("cancelled waiter: %v", err)
	}
	close(release)
	for range 2 {
		got, err := operation.Result(context.Background())
		if err != nil || got != "done" {
			t.Fatalf("settlement: %q %v", got, err)
		}
	}
	if Discard(operation).OperationID() != operation.OperationID() {
		t.Fatal("void projection changed correlation")
	}
}

// RFC §11.1:600–612, A.3:1342: unknown hold retains an answered call.
func TestAnsweredCallDisplayPreservesUnknownHold(t *testing.T) {
	unknown := Unknown[bool]()
	yes := KnownValue(true)
	no := KnownValue(false)
	for _, tc := range []struct {
		local, remote Known[bool]
		transfer      bool
		want          CallState
	}{
		{no, unknown, false, "unknown"}, {no, yes, false, "held"}, {no, no, false, "active"}, {unknown, no, true, "transferring"}, {unknown, no, false, "unknown"},
	} {
		if got := DisplayedCallState(true, tc.transfer, tc.local, tc.remote); got != tc.want {
			t.Fatalf("got %s want %s", got, tc.want)
		}
	}
	var v CallState
	if err := json.Unmarshal([]byte(`"unknown"`), &v); err != nil || v != "unknown" {
		t.Fatalf("unknown display wire: %v %s", err, v)
	}
}

// RFC §4:23, A.1:1248–1255/1290: omission is not unknown or complete.
func TestPopulationCoverageDoesNotInventCompleteness(t *testing.T) {
	for _, status := range []string{"complete", "incomplete", "unknown"} {
		before := SessionSnapshot{Sources: map[CaptureSourceID]CaptureSourceView{}, ReceiveSources: map[ReceiveSourceID]ReceiveSourceView{}, SourceCoverage: &PopulationCoverage{Scope: CapabilityScope{Authority: "client", Profile: "common"}, Completeness: status}, ReceiveSourceCoverage: &PopulationCoverage{Scope: CapabilityScope{Authority: "client", Profile: "common"}, Completeness: status}}
		raw, err := json.Marshal(struct {
			Sources               map[CaptureSourceID]CaptureSourceView `json:"sources"`
			ReceiveSources        map[ReceiveSourceID]ReceiveSourceView `json:"receiveSources"`
			SourceCoverage        *PopulationCoverage                   `json:"sourceCoverage"`
			ReceiveSourceCoverage *PopulationCoverage                   `json:"receiveSourceCoverage"`
		}{before.Sources, before.ReceiveSources, before.SourceCoverage, before.ReceiveSourceCoverage})
		if err != nil {
			t.Fatal(err)
		}
		var after SessionSnapshot
		if err := json.Unmarshal(raw, &after); err != nil {
			t.Fatal(err)
		}
		if after.SourceCoverage == nil || after.SourceCoverage.Completeness != status || after.ReceiveSourceCoverage == nil || after.ReceiveSourceCoverage.Completeness != status {
			t.Fatalf("lost %s coverage", status)
		}
	}
	var omitted SessionSnapshot
	if err := json.Unmarshal([]byte(`{"sources":{},"receiveSources":{}}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if omitted.SourceCoverage != nil || omitted.ReceiveSourceCoverage != nil {
		t.Fatal("omission invented coverage")
	}
}
