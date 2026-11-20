package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

// These tables expand operations.md A/E. Only conditions established by fixture
// evidence may use the corresponding code; these are not native-number mappings.
var lifecycleAdmissionCodes = []openrtc.ErrorCode{"invalid-option", "unsupported-option", "invalid-state", "operation-conflict", "authorization-denied", "credential-revoked", "credential-expired", "authentication-failed", "credential-failed", "placement-unsatisfied", "unsupported-capability", "capability-pending", "resource-not-found", "resource-conflict", "quota-exceeded", "network-failure", "service-unavailable"}
var lifecycleExecutionCodes = []openrtc.ErrorCode{"authorization-denied", "credential-revoked", "credential-expired", "authentication-failed", "credential-failed", "placement-unsatisfied", "unsupported-option", "unsupported-capability", "capability-pending", "invalid-option", "invalid-state", "resource-not-found", "resource-conflict", "quota-exceeded", "network-failure", "service-unavailable", "protocol-failure", "media-failure", "provider-failure", "operation-conflict", "adapter-contract-violation", "internal-failure"}

func lifecycleFaults(c *harness.TestContext) harness.LifecycleFaultControl {
	t, ok := c.Target.(harness.LifecycleFaultTarget)
	if !ok {
		must(c.NotRun("fixture provides no LifecycleFaultTarget"))
	}
	f := t.LifecycleFaults()
	if f == nil {
		must(c.NotRun("fixture provides no lifecycle fault control"))
	}
	return f
}
func lifecycleEvents(c *harness.TestContext) harness.LifecycleEventControl {
	t, ok := c.Target.(harness.LifecycleEventTarget)
	if !ok {
		must(c.NotRun("fixture provides no LifecycleEventTarget"))
	}
	f := t.LifecycleEvents()
	if f == nil {
		must(c.NotRun("fixture provides no lifecycle event control"))
	}
	return f
}
func syncError(f func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(error); ok {
				err = e
			} else {
				panic(v)
			}
		}
	}()
	f()
	return
}
func sameCleanup(c *harness.TestContext, a, b openrtc.CleanupReport) {
	check(c, reflect.DeepEqual(a, b), "rejected command changed cleanup obligations")
}
func retained(c *harness.TestContext, i openrtc.OperationInspectionAccess, id openrtc.OperationID) {
	r := value(i.Operation(id))
	check(c, r.Status == "retained" && r.Details != nil && r.Details.OperationID == id, "operation inspection not retained")
}
func assertReleased(ctx context.Context, c *harness.TestContext, f harness.LifecycleFaultControl, b harness.LifecycleResources) {
	must(f.CompleteCleanup(ctx))
	a := f.Resources()
	check(c, a.Acquired-b.Acquired == a.Released-b.Released, "unreturned allocation not released")
}

// LifecycleTests map explicitly to operations.md contracts and conformance.md
// lifecycle scenarios. Public interfaces and optional fixture controls only.
var LifecycleTests = []harness.ConformanceTest{
	{ID: "CT-LIFE-001", Needs: []harness.FixtureNeed{"lifecycle-faults"}, Title: "registration rejects without replacing the valid registration", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-100", "ADP-101", "ADP-102", "STM-200", "CORE-201"}, Observable: "operations.md Registry register: exact error, unchanged registry and compatible success", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		r := c.Target.CreateRegistry()
		a := c.Target.CreateAdapter()
		reg := value(r.Register(a))
		check(c, reg.AdapterID == a.Descriptor().AdapterID, "wrong registered ID")
		_, nilErr := r.Register(nil)
		expect(c, nilErr, []openrtc.ErrorCode{"incompatible-adapter"}, "nil adapter")
		_, err := r.Register(a)
		expect(c, err, []openrtc.ErrorCode{"incompatible-adapter"}, "duplicate")
		bad := a.Descriptor()
		bad.SDKArtifact = ""
		_, err = r.Register(descriptorAdapter{a, bad})
		expect(c, err, []openrtc.ErrorCode{"incompatible-adapter"}, "bad metadata")
		bad = a.Descriptor()
		bad.Specification.MinimumInclusive = openrtc.SpecVersion{Major: 9999}
		bad.Specification.MaximumExclusive = openrtc.SpecVersion{Major: 10000}
		_, err = r.Register(descriptorAdapter{a, bad})
		expect(c, err, []openrtc.ErrorCode{"incompatible-adapter"}, "version nonoverlap")
		fresh := a.Descriptor()
		fresh.AdapterID += "-construction-failure"
		f.FailNext("register", "admission", "internal-failure")
		_, err = r.Register(descriptorAdapter{a, fresh})
		expect(c, err, []openrtc.ErrorCode{"internal-failure"}, "registration invariant failure")
		value(r.Register(descriptorAdapter{a, fresh}))
		p := value(r.Initialize(ctx, reg.AdapterID, c.Target.ProviderConfig()).Result(ctx))
		defer disposeProvider(ctx, p)
		check(c, reg.SelectedVersion == p.Descriptor().Specification.MinimumInclusive || versionWithin(reg.SelectedVersion, p.Descriptor().Specification), "selected version outside interval")
	})},
	{ID: "CT-LIFE-002", Title: "unknown initialization returns a retained failed operation and no provider", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CORE-116", "CORE-201", "IDL-120", "IDL-150", "STM-200"}, Observable: "operations.md Registry initialize: invalid-option, registry reusable, cleanup readable", Run: run(func(ctx context.Context, c *harness.TestContext) {
		r := c.Target.CreateRegistry()
		op := r.Initialize(ctx, "conformance-unregistered", c.Target.ProviderConfig())
		check(c, op != nil && op.OperationID() != "", "no initialization handle")
		p, err := op.Result(ctx)
		check(c, p == nil, "failed initialize exposed provider")
		expect(c, err, []openrtc.ErrorCode{"invalid-option"}, "unknown ID")
		retained(c, op.Inspection(), op.OperationID())
		check(c, op.Cleanup().Status.Kind == "complete", "unknown ID acquired cleanup")
		a := c.Target.CreateAdapter()
		value(r.Register(a))
		p = value(r.Initialize(ctx, a.Descriptor().AdapterID, c.Target.ProviderConfig()).Result(ctx))
		defer disposeProvider(ctx, p)
	})},
	{ID: "CT-LIFE-003", Title: "initialization admission and execution errors retain cleanup without returning a provider", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CORE-116", "CORE-200", "CORE-201", "IDL-120", "IDL-150", "STM-200"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "native-effects"}, Observable: "operations.md Registry initialize A/E: every established code, no wrapper, retry and cleanup", Run: run(func(ctx context.Context, c *harness.TestContext) { lifecycleConstructionErrors(ctx, c, "initialize") })},
	{ID: "CT-LIFE-004", Title: "createClient admission and execution errors leave provider usable", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CORE-113", "CORE-116", "CORE-200", "CORE-201", "CFG-150", "IDL-150", "STM-200"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "native-effects"}, Observable: "operations.md Provider createClient A/E: every established code, no failed child, cleanup retained", Run: run(func(ctx context.Context, c *harness.TestContext) {
		lifecycleConstructionErrors(ctx, c, "create-client")
	})},
	{ID: "CT-LIFE-005", Title: "disposed provider rejects child creation after validating configuration", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CORE-113", "CORE-124", "CORE-202", "STM-200"}, Needs: []harness.FixtureNeed{"native-effects"}, Observable: "operations.md createClient and Owner dispose: invalid-state and unchanged native effects", Run: run(func(ctx context.Context, c *harness.TestContext) {
		n := value(c.RequireNative())
		p := value(c.Provider(ctx, nil))
		value(p.Dispose(ctx).Result(ctx))
		b := n.Counts()
		cfg := value(c.ClientConfig(harness.ClientOptions{}))
		invalid := cfg
		invalid.Extensions = helpers.ForgedExtensions(helpers.ForeignAdapter, "client-init")
		_, shapeErr := p.CreateClient(ctx, invalid).Result(ctx)
		expect(c, shapeErr, []openrtc.ErrorCode{"unsupported-option"}, "configuration validation before disposal admission")
		cl, err := p.CreateClient(ctx, cfg).Result(ctx)
		if cl != nil {
			defer disposeClient(ctx, cl)
		}
		expect(c, err, []openrtc.ErrorCode{"invalid-state"}, "disposed provider")
		check(c, cl == nil && n.Counts() == b, "disposed provider created child")
		check(c, p.Inspection().Cleanup().Status.Kind == "complete", "disposed provider lost cleanup")
	})},
	{ID: "CT-LIFE-006", Title: "openSession creates independent inert generations and rejects invalid or disposed opens", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-104", "SES-105", "CORE-116", "CORE-154", "CORE-203", "STM-101", "STM-200"}, Needs: []harness.FixtureNeed{"native-effects"}, Observable: "operations.md Client openSession: idle owned handles, no effects; invalid-option/invalid-state preserve sessions", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		n := value(c.RequireNative())
		b := n.Counts()
		calls := len(c.Spy.Calls())
		a := c.Open(cl, room)
		s := c.Open(cl, room)
		check(c, a.ID() != s.ID() && a.Generation() != s.Generation(), "opens reused generation")
		check(c, a.Connection().Snapshot().State == "idle" && s.Connection().Snapshot().State == "idle", "open was not idle")
		check(c, cl.Sessions()[s.ID()] != nil, "client does not own opened session")
		check(c, n.Counts() == b && len(c.Spy.Calls()) == calls, "open performed external work")
		count := len(cl.Sessions())
		err := syncError(func() { cl.OpenSession(openrtc.ConferenceTarget{}) })
		expect(c, err, []openrtc.ErrorCode{"invalid-option"}, "malformed target")
		check(c, len(cl.Sessions()) == count && n.Counts() == b, "invalid open changed owner")
		value(cl.Dispose(ctx).Result(ctx))
		err = syncError(func() { cl.OpenSession(room) })
		expect(c, err, []openrtc.ErrorCode{"invalid-state"}, "disposed client open")
		check(c, n.Counts() == b, "disposed open had effects")
	})},
	{ID: "CT-LIFE-007", Title: "join admission errors preserve unused idle generation and allow corrected setup", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-105", "SES-106", "CORE-200", "STM-101", "IDL-150"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "native-effects"}, Observable: "operations.md Conference join A: each condition rejects before effects; retry succeeds", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		n := value(c.RequireNative())
		for _, code := range lifecycleAdmissionCodes {
			s := c.Open(cl, room)
			b := n.Counts()
			cleanup := s.Inspection().Cleanup()
			calls := len(c.Spy.Calls())
			f.FailNext("join", "admission", code)
			a := s.Join(opts(c))
			_, err := a.Result(ctx).Result(ctx)
			expect(c, err, []openrtc.ErrorCode{code}, "join admission")
			check(c, s.Connection().Snapshot().State == "idle" && n.Counts() == b && len(c.Spy.Calls()) == calls, "preflight changed state/effects")
			sameCleanup(c, cleanup, s.Inspection().Cleanup())
			retained(c, s.Inspection(), a.Result(ctx).OperationID())
			value(s.Join(opts(c)).Result(ctx).Result(ctx))
			value(s.Leave(ctx).Result(ctx))
		}
	})},
	{ID: "CT-LIFE-008", Title: "invalid join options do not consume the sole attempt", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-105", "CORE-200", "CFG-100", "STM-101"}, Observable: "operations.md Conference join A: malformed/unsupported schema keeps idle; corrected join succeeds", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		for _, bad := range []openrtc.JoinSessionOptions{{Completion: "conformance-invalid"}, {Extensions: helpers.ForgedExtensions(helpers.ForeignAdapter, "participant-join")}} {
			s := c.Open(cl, room)
			_, err := s.Join(bad).Result(ctx).Result(ctx)
			expect(c, err, optionErrors, "invalid join")
			check(c, s.Connection().Snapshot().State == "idle", "invalid join consumed state")
			value(s.Join(opts(c)).Result(ctx).Result(ctx))
			value(s.Leave(ctx).Result(ctx))
		}
	})},
	{ID: "CT-LIFE-009", Title: "pre-aborted join consumes and terminates generation without native work", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-105", "CORE-119", "STM-101", "STM-115", "STM-122"}, Needs: []harness.FixtureNeed{"native-effects"}, Observable: "operations.md Conference join F: cancelled, ended, no effects, second setup invalid-state", Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		n := value(c.RequireNative())
		b := n.Counts()
		signal, cancel := context.WithCancel(ctx)
		cancel()
		o := opts(c)
		o.Signal = signal
		_, err := s.Join(o).Result(ctx).Result(ctx)
		expect(c, err, []openrtc.ErrorCode{"cancelled"}, "preabort")
		check(c, s.Connection().Snapshot().State == "ended" && n.Counts() == b, "preabort did not fence without effects")
		rejectsJoin(ctx, c, s, "invalid-state", "consumed generation")
		check(c, s.Inspection().Cleanup().Status.Kind == "complete", "preabort invented resource obligation")
	})},
	{ID: "CT-LIFE-010", Title: "join execution errors terminate setup and retain unreturned acquisition cleanup", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-105", "CORE-116", "CORE-200", "STM-101", "IDL-150"}, Needs: []harness.FixtureNeed{"lifecycle-faults"}, Observable: "operations.md Conference join E: each execution code, failed setup, no rejoin, compensation", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		for _, code := range lifecycleExecutionCodes {
			s := c.Open(cl, room)
			b := f.Resources()
			f.FailNext("join", "execution", code)
			a := s.Join(opts(c))
			_, err := a.Result(ctx).Result(ctx)
			expect(c, err, []openrtc.ErrorCode{code}, "join execution")
			check(c, s.Connection().Snapshot().State == "failed", "execution setup failure did not commit failed")
			check(c, s.Inspection().Cleanup().Status.Kind != "complete", "failed acquisition cleanup lost")
			rejectsJoin(ctx, c, s, "invalid-state", "failed generation")
			retained(c, s.Inspection(), a.Result(ctx).OperationID())
			assertReleased(ctx, c, f, b)
			check(c, s.Inspection().Cleanup().Status.Kind == "complete", "compensation not reflected")
		}
	})},
	{ID: "CT-LIFE-011", Title: "cancel fences blocked native connect before late success and compensates it", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "CORE-119", "STM-010", "STM-011", "STM-012", "STM-013", "STM-115", "STM-122", "STM-140"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "gate-next-join", "native-effects"}, Observable: "conformance.md cancellation during connection: pending cleanup, repeatable cancelled result, late leave only", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		gate := value(c.RequireControl("gateNextJoin")).(func() harness.JoinGate)()
		defer gate.ReleaseSuccess()
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		a := s.Join(opts(c))
		waitLifecycleGate(ctx, c, gate)
		report := value(a.Cancel(ctx).Result(ctx))
		check(c, report.Cleanup.Status.Kind == "pending" || report.Cleanup.Status.Kind == "unknown", "cancel asserted completed native cleanup while connect blocked")
		_, err := a.Result(ctx).Result(ctx)
		first := expect(c, err, []openrtc.ErrorCode{"cancelled"}, "cancelled result")
		check(c, s.Connection().Snapshot().State == "ended", "cancel failed to terminate locally")
		n := value(c.RequireNative())
		b := n.Counts()
		gate.ReleaseSuccess()
		must(f.Quiesce(ctx, s))
		_, err = a.Result(ctx).Result(ctx)
		last := expect(c, err, []openrtc.ErrorCode{"cancelled"}, "late success result")
		check(c, first.ID == last.ID && s.Connection().Snapshot().State == "ended", "late native success resurrected generation")
		late := value(a.Cancel(ctx).Result(ctx))
		check(c, !late.Won, "final cancellation result was overwritten")
		check(c, n.Counts().Leaves > b.Leaves, "late participation release was not initiated")
		checkReport(c, s.Inspection().Cleanup(), "late cleanup remains truthfully qualified")
	})},
	{ID: "CT-LIFE-012", Title: "leave terminates unused generations and shares idempotent cleanup across routes", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "CORE-114", "CORE-153", "STM-116", "STM-122", "STM-201", "IDL-150"}, Needs: []harness.FixtureNeed{"native-effects"}, Observable: "operations.md Conference/Connection leave: idle and connected become ended, all release routes share work", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		s := c.Open(cl, room)
		value(s.Leave(ctx).Result(ctx))
		check(c, s.Connection().Snapshot().State == "ended", "idle leave not terminal")
		rejectsJoin(ctx, c, s, "invalid-state", "join after idle leave")
		s = c.Open(cl, room)
		value(s.Join(opts(c)).Result(ctx).Result(ctx))
		var wg sync.WaitGroup
		errs := make(chan error, 3)
		for _, release := range []func(context.Context) openrtc.Async[openrtc.Void]{s.Leave, s.Connection().Leave, s.Dispose} {
			wg.Go(func() { _, err := release(ctx).Result(ctx); errs <- err })
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			must(err)
		}
		check(c, s.Connection().Snapshot().State == "ended", "release routes resurrected terminal participation")
		value(s.Leave(ctx).Result(ctx))
		checkReport(c, s.Inspection().Cleanup(), "repeat leave preserves identifiable obligations")
	})},
	{ID: "CT-LIFE-013", Title: "leave and dispose retain state when a local fence fails", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-114", "STM-116", "STM-201", "CORE-200"}, Needs: []harness.FixtureNeed{"lifecycle-faults"}, Observable: "operations.md R local fence error: correct code, actual connection retained, cleanup independently retryable", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		for _, op := range []string{"leave", "dispose", "client-dispose"} {
			for _, code := range append([]openrtc.ErrorCode{"invalid-option", "invalid-state", "operation-conflict"}, lifecycleExecutionCodes...) {
				j := joined(ctx, c)
				b := j.Session.Inspection().Cleanup()
				f.FailNext(op, "fence", code)
				release := j.Session.Leave
				if op == "dispose" {
					release = j.Session.Dispose
				}
				if op == "client-dispose" {
					release = j.Client.Dispose
				}
				_, err := release(ctx).Result(ctx)
				expect(c, err, []openrtc.ErrorCode{code}, "local fence")
				check(c, j.Session.Connection().Snapshot().State == "connected", "failed fence fabricated closure")
				sameCleanup(c, b, j.Session.Inspection().Cleanup())
				value(release(ctx).Result(ctx))
				check(c, j.Session.Connection().Snapshot().State == "ended", "retry fence failed")
			}
		}
	})},
	{ID: "CT-LIFE-014", Title: "native cleanup failure does not reject leave or disposal and progress remains observable", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-114", "CORE-153", "CORE-202", "STM-114", "STM-116", "STM-140", "IDL-110", "ERR-106"}, Needs: []harness.FixtureNeed{"lifecycle-faults"}, Observable: "operations.md R / conformance.md partial cleanup: one release succeeds, one fails, later progress retained", Run: run(func(ctx context.Context, c *harness.TestContext) { lifecyclePartialCleanup(ctx, c) })},
	{ID: "CT-LIFE-015", Title: "client disposal cascades sessions and closes operational listeners", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-113", "CORE-114", "CORE-202", "STM-114", "STM-200"}, Observable: "operations.md Owner dispose: child sessions ended, new work rejected, operational listeners closed", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		s := c.Open(cl, room)
		value(s.Join(opts(c)).Result(ctx).Result(ctx))
		var mu sync.Mutex
		calls := 0
		off := s.Observation().Watch(func(openrtc.SessionSnapshot) { mu.Lock(); calls++; mu.Unlock() })
		defer off()
		value(cl.Dispose(ctx).Result(ctx))
		mu.Lock()
		b := calls
		mu.Unlock()
		value(s.Leave(ctx).Result(ctx))
		value(cl.Dispose(ctx).Result(ctx))
		mu.Lock()
		after := calls
		mu.Unlock()
		check(c, after == b && s.Connection().Snapshot().State == "ended", "dispose left operational listener or child live")
		err := syncError(func() { cl.OpenSession(room) })
		expect(c, err, []openrtc.ErrorCode{"invalid-state"}, "open after client dispose")
		_ = cl.Inspection().Snapshot()
		_ = s.Observation().Snapshot()
	})},
	{ID: "CT-LIFE-016", Title: "snapshots are detached and remain readable after disposal", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-202", "STM-108", "STM-124", "IDL-150"}, Needs: []harness.FixtureNeed{"native-effects"}, Observable: "operations.md Observation/Connection/Inspection snapshot: detached nested state; no native effects", Run: run(func(ctx context.Context, c *harness.TestContext) {
		j := joined(ctx, c)
		s := j.Session
		n := value(c.RequireNative())
		b := n.Counts()
		original := s.Observation().Snapshot()
		copy := s.Observation().Snapshot()
		copy.Roster.Entries["invented"] = openrtc.RTCParticipant{ID: "invented"}
		copy.Credentials[0].State = "invented"
		check(c, reflect.DeepEqual(s.Observation().Snapshot(), original), "snapshot aliases committed nested facts")
		connection := s.Connection().Snapshot()
		connection.RecoveryPolicy.Planes[0] = "invented"
		check(c, s.Connection().Snapshot().RecoveryPolicy.Planes[0] != "invented", "connection snapshot aliases nested facts")
		check(c, n.Counts() == b, "snapshot started native work")
		value(s.Dispose(ctx).Result(ctx))
		check(c, s.Observation().Snapshot().Connection.State == "ended" && s.Connection().Snapshot().State == "ended", "disposed snapshot unreadable")
		_ = s.Inspection().Snapshot()
	})},
	{ID: "CT-LIFE-017", Title: "watch rejects invalid listeners before disposed admission", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-110", "CORE-202", "STM-108", "IDL-150"}, Observable: "operations.md Observation/Inspection/Cleanup watch: invalid-option then invalid-state, no registration effects", Run: run(func(ctx context.Context, c *harness.TestContext) { lifecycleWatchErrors(ctx, c) })},
	{ID: "CT-LIFE-018", Title: "watch bootstraps current revision and isolates listener failures with idempotent unsubscribe", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-110", "CORE-111", "CORE-112", "STM-108", "IDL-150"}, Observable: "operations.md Observation watch: current first, newer committed revisions, exceptions isolated", Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		bad := s.Observation().Watch(func(openrtc.SessionSnapshot) { panic("listener failure") })
		defer bad()
		var mu sync.Mutex
		var revisions []openrtc.Revision
		off := s.Observation().Watch(func(v openrtc.SessionSnapshot) { mu.Lock(); revisions = append(revisions, v.Revision); mu.Unlock() })
		mu.Lock()
		initial := len(revisions)
		mu.Unlock()
		check(c, initial == 1, "watch did not bootstrap once")
		value(s.Join(opts(c)).Result(ctx).Result(ctx))
		mu.Lock()
		rs := append([]openrtc.Revision(nil), revisions...)
		mu.Unlock()
		check(c, len(rs) > 1, "throwing watcher blocked later updates")
		for i := 1; i < len(rs); i++ {
			check(c, rs[i] > rs[i-1], "watch revisions not newer")
		}
		off()
		off()
		value(s.Leave(ctx).Result(ctx))
		mu.Lock()
		count := len(revisions)
		mu.Unlock()
		check(c, count == len(rs), "unsubscribe did not stop watch")
	})},
	{ID: "CT-LIFE-019", Title: "stale generation callbacks are fenced and late allocations cleaned up", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-008", "STM-011", "STM-121", "CORE-120", "ERR-110"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "lifecycle-events"}, Observable: "conformance.md stale callbacks: old updates silent, late allocation cleanup, wrong live generation rejected", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		events := lifecycleEvents(c)
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		old := c.Open(cl, room)
		value(old.Join(opts(c)).Result(ctx).Result(ctx))
		value(old.Leave(ctx).Result(ctx))
		fresh := c.Open(cl, room)
		value(fresh.Join(opts(c)).Result(ctx).Result(ctx))
		before := fresh.Observation().Snapshot()
		oldRoster := old.Observation().Snapshot().Roster
		b := f.Resources()
		must(events.Submit(ctx, old, old.Generation(), []harness.LifecycleNativeEvent{{Kind: "upsert", NativeUID: "late", NativeKey: "old"}, {Kind: "allocation"}}))
		check(c, reflect.DeepEqual(before, fresh.Observation().Snapshot()) && reflect.DeepEqual(oldRoster, old.Observation().Snapshot().Roster), "stale callback changed operational facts")
		check(c, f.Resources().Acquired == b.Acquired+1 && old.Inspection().Cleanup().Status.Kind != "complete", "stale allocation obligation lost")
		assertReleased(ctx, c, f, b)
		rec := c.Record(fresh)
		err := events.Submit(ctx, fresh, old.Generation(), []harness.LifecycleNativeEvent{{Kind: "upsert", NativeUID: "bad", NativeKey: "bad"}})
		failure := expect(c, err, []openrtc.ErrorCode{"adapter-contract-violation"}, "live generation mismatch")
		correlated := false
		for _, e := range rec.Events() {
			if e.Key == "error" {
				observed := expect(c, e.Envelope.Payload.(error), []openrtc.ErrorCode{"adapter-contract-violation"}, "live host error event")
				correlated = correlated || observed.ID == failure.ID
			}
		}
		check(c, correlated, "live host rejection lacks correlated error observation")
		check(c, reflect.DeepEqual(before.Roster, fresh.Observation().Snapshot().Roster) && fresh.Connection().Snapshot().State == "connected", "malformed live callback committed or terminated owner")
	})},
	{ID: "CT-LIFE-020", Title: "reused native identifiers create distinct public incarnations", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-007", "SES-116", "SES-117", "SES-118", "ADP-007", "ADP-113", "IDL-107"}, Needs: []harness.FixtureNeed{"lifecycle-events"}, Observable: "conformance.md reused native identifiers: new public identity, delayed old removal harmless", Run: run(func(ctx context.Context, c *harness.TestContext) {
		events := lifecycleEvents(c)
		s := joined(ctx, c).Session
		first := harness.LifecycleNativeEvent{Kind: "upsert", NativeUID: "same-uid", NativeKey: "incarnation-1", DisplayName: "first"}
		must(events.Submit(ctx, s, s.Generation(), []harness.LifecycleNativeEvent{first}))
		a := onlyFixtureParticipant(c, s, "first")
		first.Kind = "remove"
		must(events.Submit(ctx, s, s.Generation(), []harness.LifecycleNativeEvent{first}))
		second := first
		second.Kind = "upsert"
		second.NativeKey = "incarnation-2"
		second.DisplayName = "second"
		must(events.Submit(ctx, s, s.Generation(), []harness.LifecycleNativeEvent{second}))
		b := onlyFixtureParticipant(c, s, "second")
		check(c, a.ID != b.ID, "native UID reuse invented public identity continuity")
		must(events.Submit(ctx, s, s.Generation(), []harness.LifecycleNativeEvent{first}))
		check(c, onlyFixtureParticipant(c, s, "second").ID == b.ID, "old tombstone removed new incarnation")
	})},
	{ID: "CT-LIFE-021", Title: "out-of-order and duplicate native callbacks reconcile to evidenced current state", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"ADP-112", "ADP-113", "STM-107", "STM-109", "STM-117", "IDL-129", "IDL-133"}, Needs: []harness.FixtureNeed{"lifecycle-events"}, Observable: "conformance.md out-of-order events: newer then older duplicate, keyed reconciliation, local revision progression", Run: run(func(ctx context.Context, c *harness.TestContext) {
		e := lifecycleEvents(c)
		s := joined(ctx, c).Session
		newer := harness.LifecycleNativeEvent{Kind: "upsert", NativeUID: "endpoint", NativeKey: "endpoint-1", DisplayName: "newer", Timestamp: "2000-01-01T00:00:02.000Z"}
		older := newer
		older.DisplayName = "older"
		older.Timestamp = "2000-01-01T00:00:01.000Z"
		b := s.Observation().Snapshot().Revision
		must(e.Submit(ctx, s, s.Generation(), []harness.LifecycleNativeEvent{newer, older, older}))
		must(e.Reconcile(ctx, s, []harness.LifecycleNativeEvent{newer}))
		_ = onlyFixtureParticipant(c, s, "newer")
		snap := s.Observation().Snapshot()
		check(c, snap.Revision > b && snap.Roster.ObservedCount == uint64(len(snap.Roster.Entries)), "reconciliation violated local projection")
		check(c, snap.Consistency == "best-effort" || snap.Consistency == "reconciled", "reconciliation fabricated complete upstream history")
	})},
}

func versionWithin(v openrtc.SpecVersion, r openrtc.VersionCompatibility) bool {
	less := func(a, b openrtc.SpecVersion) bool {
		if a.Major != b.Major {
			return a.Major < b.Major
		}
		if a.Minor != b.Minor {
			return a.Minor < b.Minor
		}
		return a.Patch < b.Patch
	}
	return !less(v, r.MinimumInclusive) && less(v, r.MaximumExclusive)
}
func onlyFixtureParticipant(c *harness.TestContext, s openrtc.RTCConferenceSession, name string) openrtc.RTCParticipant {
	var found []openrtc.RTCParticipant
	for _, p := range s.Observation().Snapshot().Roster.Entries {
		if p.DisplayName.Value != nil && *p.DisplayName.Value == name {
			found = append(found, p)
		}
	}
	check(c, len(found) == 1, "reconciled endpoint missing or duplicated: "+name)
	return found[0]
}

func lifecycleConstructionErrors(ctx context.Context, c *harness.TestContext, kind string) {
	f := lifecycleFaults(c)
	n := value(c.RequireNative())
	r := c.Target.CreateRegistry()
	a := c.Target.CreateAdapter()
	value(r.Register(a))
	var p openrtc.RTCProvider
	if kind == "create-client" {
		p = value(c.Provider(ctx, nil))
	}
	for _, phase := range []string{"admission", "execution"} {
		codes := lifecycleAdmissionCodes
		if phase == "execution" {
			codes = lifecycleExecutionCodes
		}
		for _, code := range codes {
			b := f.Resources()
			before := n.Counts()
			f.FailNext(kind, phase, code)
			var cleanup openrtc.CleanupReport
			if kind == "initialize" {
				op := r.Initialize(ctx, a.Descriptor().AdapterID, c.Target.ProviderConfig())
				made, err := op.Result(ctx)
				if made != nil {
					defer disposeProvider(ctx, made)
				}
				expect(c, err, []openrtc.ErrorCode{code}, kind+" "+phase)
				check(c, made == nil, "failed initialization returned wrapper")
				retained(c, op.Inspection(), op.OperationID())
				cleanup = op.Cleanup()
			} else {
				cfg := value(c.ClientConfig(harness.ClientOptions{}))
				op := p.CreateClient(ctx, cfg)
				made, err := op.Result(ctx)
				if made != nil {
					defer disposeClient(ctx, made)
				}
				expect(c, err, []openrtc.ErrorCode{code}, kind+" "+phase)
				check(c, made == nil, "failed construction returned child")
				retained(c, p.Inspection(), op.OperationID())
				cleanup = p.Inspection().Cleanup()
			}
			check(c, n.Counts() == before, "failed construction changed unrelated native effects")
			if phase == "admission" {
				check(c, cleanup.Status.Kind == "complete" && f.Resources() == b, "admission acquired resources/obligations")
			} else {
				check(c, cleanup.Status.Kind != "complete" && len(cleanup.Status.Outstanding) > 0 && f.Resources().Acquired == b.Acquired+1, "execution lost acquired resource cleanup")
				assertReleased(ctx, c, f, b)
			}
			// A failed child or initialization must not poison its retaining registry/provider.
			if kind == "initialize" {
				ok := value(r.Initialize(ctx, a.Descriptor().AdapterID, c.Target.ProviderConfig()).Result(ctx))
				disposeProvider(ctx, ok)
			} else {
				ok := value(p.CreateClient(ctx, value(c.ClientConfig(harness.ClientOptions{}))).Result(ctx))
				check(c, ok.Inspection().Snapshot().Descriptor.AdapterID == c.Descriptor.AdapterID, "child not bootstrapped")
				disposeClient(ctx, ok)
			}
		}
	}
}
func lifecyclePartialCleanup(ctx context.Context, c *harness.TestContext) {
	f := lifecycleFaults(c)
	for _, route := range []string{"leave", "session-dispose", "client-dispose"} {
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		value(s.Join(opts(c)).Result(ctx).Result(ctx))
		b := f.Resources()
		var mu sync.Mutex
		var reports []openrtc.CleanupReport
		off := s.Inspection().WatchCleanup(func(r openrtc.CleanupReport) { mu.Lock(); reports = append(reports, r); mu.Unlock() })
		defer off()
		op := "leave"
		if route != "leave" {
			op = "dispose"
		}
		f.FailNext(op, "cleanup", "cleanup-failed")
		release := s.Leave
		if route == "session-dispose" {
			release = s.Dispose
		}
		if route == "client-dispose" {
			release = cl.Dispose
		}
		value(release(ctx).Result(ctx))
		check(c, s.Connection().Snapshot().State == "ended", "cleanup failure prevented local terminal fence")
		report := s.Inspection().Cleanup()
		check(c, report.Status.Kind == "failed" && len(report.Status.Outstanding) > 0 && len(report.Status.Errors) > 0, "cleanup failure not qualified/retained")
		expect(c, report.Status.Errors[0], []openrtc.ErrorCode{"cleanup-failed"}, "cleanup diagnostic")
		a := f.Resources()
		check(c, a.Acquired == b.Acquired+2 && a.Released == b.Released+1, "partial release did not preserve exactly one obligation")
		// Cleanup subscriptions are admitted after disposal and observe later progress.
		calls := 0
		later := s.Inspection().WatchCleanup(func(openrtc.CleanupReport) { mu.Lock(); calls++; mu.Unlock() })
		defer later()
		must(f.CompleteCleanup(ctx))
		check(c, s.Inspection().Cleanup().Status.Kind == "complete", "later cleanup progress missing")
		if route == "client-dispose" {
			check(c, cl.Inspection().Cleanup().Status.Kind == "complete", "child cleanup progress not visible on disposed client")
		}
		mu.Lock()
		count := calls
		countReports := len(reports)
		mu.Unlock()
		check(c, count >= 2 && countReports >= 2, "cleanup watcher did not survive operational disposal")
		later()
		later()
		value(release(ctx).Result(ctx))
		mu.Lock()
		after := calls
		mu.Unlock()
		check(c, after == count, "cleanup unsubscribe ineffective")
	}
}
func lifecycleWatchErrors(ctx context.Context, c *harness.TestContext) {
	cl := client(ctx, c)
	s := c.Open(cl, value(c.Conference(ctx)))
	p := value(c.Provider(ctx, nil))
	watches := []func(){func() { s.Observation().Watch(nil) }, func() { s.Inspection().Watch(nil) }, func() { cl.Inspection().Watch(nil) }, func() { p.Inspection().Watch(nil) }, func() { s.Inspection().WatchCleanup(nil) }}
	for _, disposed := range []bool{false, true} {
		if disposed {
			value(cl.Dispose(ctx).Result(ctx))
			value(p.Dispose(ctx).Result(ctx))
		}
		b := s.Inspection().Cleanup()
		state := s.Connection().Snapshot().State
		for _, watch := range watches {
			err := syncError(watch)
			expect(c, err, []openrtc.ErrorCode{"invalid-option"}, "nil watch listener")
		}
		check(c, s.Connection().Snapshot().State == state, "invalid listener mutated connection")
		sameCleanup(c, b, s.Inspection().Cleanup())
	}
	for _, watch := range []func(){func() { s.Observation().Watch(func(openrtc.SessionSnapshot) {}) }, func() { s.Inspection().Watch(func(openrtc.SessionInspectionSnapshot) {}) }, func() { cl.Inspection().Watch(func(openrtc.OwnerInspectionSnapshot) {}) }, func() { p.Inspection().Watch(func(openrtc.OwnerInspectionSnapshot) {}) }} {
		expect(c, syncError(watch), []openrtc.ErrorCode{"invalid-state"}, "watch disposed owner")
	}
}
