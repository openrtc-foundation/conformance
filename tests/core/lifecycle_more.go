package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

var lifecycleMoreTests = []harness.ConformanceTest{
	{ID: "CT-LIFE-022", Title: "openSession errors expose no child and preserve client usability", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-104", "CORE-116", "CORE-200", "CORE-203", "STM-200"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "native-effects"}, Observable: "operations.md Client openSession A / E invariant: exact errors, no new handle, retained cleanup", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		n := value(c.RequireNative())
		for _, phase := range []string{"admission", "execution"} {
			codes := lifecycleAdmissionCodes
			if phase == "execution" {
				codes = []openrtc.ErrorCode{"internal-failure", "adapter-contract-violation"}
			}
			for _, code := range codes {
				b := n.Counts()
				resources := f.Resources()
				count := len(cl.Sessions())
				f.FailNext("open-session", phase, code)
				err := syncError(func() { cl.OpenSession(room) })
				expect(c, err, []openrtc.ErrorCode{code}, "open "+phase)
				check(c, len(cl.Sessions()) == count && n.Counts() == b, "failed open exposed child or native work")
				if phase == "admission" {
					check(c, f.Resources() == resources && cl.Inspection().Cleanup().Status.Kind == "complete", "open preflight acquired resources")
				} else {
					check(c, cl.Inspection().Cleanup().Status.Kind != "complete", "failed wrapper construction lost cleanup")
					assertReleased(ctx, c, f, resources)
				}
				s := c.Open(cl, room)
				check(c, s.Connection().Snapshot().State == "idle", "failed open poisoned client")
				value(s.Dispose(ctx).Result(ctx))
			}
		}
	})},
	{ID: "CT-LIFE-023", Title: "join returns selected committed milestone and settled cancel preserves the winning result", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-105", "SES-106", "SES-107", "STM-101", "STM-102", "STM-115", "STM-201", "IDL-150"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "admission-control"}, Observable: "operations.md Conference join success and Attempt cancel invalid-state: committed connection, selected milestone, repeatable result", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		admission := value(c.RequireControl("admission")).(func() harness.AdmissionControl)()
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		for _, milestone := range []string{"", "admitted"} {
			o := opts(c)
			o.Completion = milestone
			if milestone == "admission-pending" {
				admission.Hold()
			}
			s := c.Open(cl, room)
			a := s.Join(o)
			result := value(a.Result(ctx).Result(ctx))
			expected := milestone
			if expected == "" {
				expected = "admitted"
			}
			snap := s.Observation().Snapshot()
			check(c, result.Completion == expected && result.Connection.State == "connected" && snap.Connection.State == "connected", "join returned before selected connection milestone committed")
			check(c, result.Admission == snap.Connection.Admission && result.MediaReadiness == snap.Connection.MediaReadiness, "result differs from committed snapshot")
			if expected == "admitted" {
				check(c, result.Achieved == "admitted" && result.Admission == "admitted", "default join did not achieve admission")
			} else {
				check(c, result.Achieved == "held" && result.Admission == "held", "explicit pending join did not preserve held admission")
				admission.Admit()
				check(c, s.Connection().Snapshot().Admission == "admitted", "later admission not observed")
			}
			check(c, len(result.InitialMedia.Operations) == 0, "omitted media invented outcomes")
			cleanup := s.Inspection().Cleanup()
			report := value(a.Cancel(ctx).Result(ctx))
			check(c, !report.Won, "settled cancellation incorrectly won (RFC §8.5:487)")
			sameCleanup(c, cleanup, s.Inspection().Cleanup())
			again := value(a.Result(ctx).Result(ctx))
			check(c, reflect.DeepEqual(result, again), "settled result changed after admission/cancel")
			rejectsJoin(ctx, c, s, "invalid-state", "second join after settlement")
			value(s.Leave(ctx).Result(ctx))
		}
		// A held default join must remain pending after its signaling checkpoint.
		admission.Hold()
		s := c.Open(cl, room)
		a := s.Join(opts(c))
		must(f.AwaitJoinMilestone(ctx, s))
		waiter, cancel := context.WithCancel(ctx)
		cancel()
		_, err := a.Result(ctx).Result(waiter)
		check(c, err == context.Canceled, "held default join settled before admission")
		admission.Admit()
		result := value(a.Result(ctx).Result(ctx))
		check(c, result.Completion == "admitted" && result.Admission == "admitted", "default held join failed admitted milestone")
		value(s.Leave(ctx).Result(ctx))
	})},
	{ID: "CT-LIFE-024", Title: "leave and owner disposal fence pending join and compensate late native success", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "CORE-113", "CORE-114", "CORE-119", "CORE-201", "STM-115", "STM-116", "STM-122", "STM-140"}, Needs: []harness.FixtureNeed{"lifecycle-faults", "gate-next-join"}, Observable: "operations.md join F / Owner dispose: pending setup cancelled before native completion; no late resurrection", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		gates := value(c.RequireControl("gateNextJoin")).(func() harness.JoinGate)
		for _, route := range []string{"leave", "session-dispose", "client-dispose"} {
			cl := client(ctx, c)
			s := c.Open(cl, value(c.Conference(ctx)))
			gate := gates()
			defer func() { gate.ReleaseSuccess(); must(f.Quiesce(ctx, s)) }()
			a := s.Join(opts(c))
			waitLifecycleGate(ctx, c, gate)
			release := s.Leave
			if route == "session-dispose" {
				release = s.Dispose
			}
			if route == "client-dispose" {
				release = cl.Dispose
			}
			value(release(ctx).Result(ctx))
			check(c, s.Connection().Snapshot().State == "ended", "owner fence did not terminate pending setup")
			_, err := a.Result(ctx).Result(ctx)
			expect(c, err, []openrtc.ErrorCode{"cancelled"}, "owner-fenced join")
			check(c, s.Inspection().Cleanup().Status.Kind != "complete", "pending native setup obligation lost")
			gate.ReleaseSuccess()
			must(f.Quiesce(ctx, s))
			_, err = a.Result(ctx).Result(ctx)
			expect(c, err, []openrtc.ErrorCode{"cancelled"}, "late fenced result")
			check(c, s.Connection().Snapshot().State == "ended", "late acquisition resurrected setup")
		}
	})},
	{ID: "CT-LIFE-025", Title: "failed join retains its initiating error when compensation also fails", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-200", "ERR-103", "ERR-106", "STM-114", "STM-201", "IDL-150"}, Needs: []harness.FixtureNeed{"lifecycle-faults"}, Observable: "operations.md E cleanup-failed: original execution code remains primary, separate cleanup diagnostic and outstanding release", Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := lifecycleFaults(c)
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		b := f.Resources()
		f.FailNext("join", "execution-cleanup", "network-failure")
		a := s.Join(opts(c))
		_, err := a.Result(ctx).Result(ctx)
		primary := expect(c, err, []openrtc.ErrorCode{"network-failure"}, "setup primary")
		r := s.Inspection().Cleanup()
		check(c, s.Connection().Snapshot().State == "failed" && r.Status.Kind == "failed" && len(r.Status.Outstanding) > 0 && len(r.Status.Errors) > 0, "failed compensation obligations missing")
		secondary := expect(c, r.Status.Errors[0], []openrtc.ErrorCode{"cleanup-failed"}, "compensation")
		check(c, primary.ID != secondary.ID, "cleanup failure replaced initiating error identity")
		assertReleased(ctx, c, f, b)
		_, err = a.Result(ctx).Result(ctx)
		check(c, expect(c, err, []openrtc.ErrorCode{"network-failure"}, "retained primary").ID == primary.ID, "cleanup progress rewrote primary failure")
	})},
	{ID: "CT-LIFE-026", Title: "emitter admission validates event and listener before disposed state", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-108", "CORE-109", "CORE-110", "CORE-202", "IDL-200"}, Observable: "operations.md Emitter on/once/off: invalid-option and invalid-state preserve state, off retained after disposal", Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		listener := func(openrtc.EventEnvelope[any]) {}
		o := s.Observation()
		for _, disposed := range []bool{false, true} {
			if disposed {
				value(s.Dispose(ctx).Result(ctx))
			}
			before := s.Inspection().Cleanup()
			state := s.Connection().Snapshot().State
			for _, bad := range []func(){func() { o.On("conformance-unknown", listener) }, func() { o.Once("conformance-unknown", listener) }, func() { o.Off("conformance-unknown", listener) }, func() { o.On("connectionChanged", nil) }, func() { o.Once("connectionChanged", nil) }, func() { o.Off("connectionChanged", nil) }} {
				expect(c, syncError(bad), []openrtc.ErrorCode{"invalid-option"}, "invalid emitter registration")
			}
			sameCleanup(c, before, s.Inspection().Cleanup())
			check(c, state == s.Connection().Snapshot().State, "invalid emitter input changed state")
		}
		expect(c, syncError(func() { o.On("connectionChanged", listener) }), []openrtc.ErrorCode{"invalid-state"}, "on disposed")
		expect(c, syncError(func() { o.Once("connectionChanged", listener) }), []openrtc.ErrorCode{"invalid-state"}, "once disposed")
		o.Off("connectionChanged", listener)
		o.Off("connectionChanged", listener)
	})},
	{ID: "CT-LIFE-027", Title: "inspection rejects unknown and foreign operation identities without losing retained facts", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-202", "IDL-120", "IDL-150", "IDL-200"}, Observable: "operations.md retained inspection reads: invalid-state for never-issued/foreign IDs, disposed owner's own operation readable", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		a := c.Open(cl, room)
		b := c.Open(cl, room)
		op := a.Leave(ctx)
		value(op.Result(ctx))
		retained(c, a.Inspection(), op.OperationID())
		old := b.Observation().Snapshot()
		cleanup := b.Inspection().Cleanup()
		for _, id := range []openrtc.OperationID{"conformance-never-issued", op.OperationID()} {
			_, err := b.Inspection().Operation(id)
			expect(c, err, []openrtc.ErrorCode{"invalid-state"}, "foreign inspection ID")
		}
		check(c, reflect.DeepEqual(old, b.Observation().Snapshot()), "invalid inspection changed snapshot")
		sameCleanup(c, cleanup, b.Inspection().Cleanup())
		value(a.Dispose(ctx).Result(ctx))
		retained(c, a.Inspection(), op.OperationID())
	})},
	{ID: "CT-LIFE-028", Title: "inspection watches bootstrap and publish inspection-only commits", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-110", "CORE-111", "CORE-202", "STM-108", "IDL-150"}, Observable: "operations.md Inspection watch: current snapshot and committed configuration receipt, detached snapshot and unsubscribe", Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		var mu sync.Mutex
		var seen []openrtc.OwnerInspectionSnapshot
		off := cl.Inspection().Watch(func(v openrtc.OwnerInspectionSnapshot) { mu.Lock(); seen = append(seen, v); mu.Unlock() })
		defer off()
		mu.Lock()
		initial := len(seen)
		mu.Unlock()
		check(c, initial == 1, "inspection watch missing bootstrap")
		op := cl.Configure(ctx, openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "debug"}})
		value(op.Result(ctx))
		mu.Lock()
		count := len(seen)
		mu.Unlock()
		check(c, count > initial, "inspection-only commit omitted by watch")
		snap := cl.Inspection().Snapshot()
		if len(snap.Configuration.Fields) > 0 {
			snap.Configuration.Fields[0].Reason = "conformance-mutated"
			check(c, cl.Inspection().Snapshot().Configuration.Fields[0].Reason != "conformance-mutated", "inspection snapshot aliases configuration")
		}
		off()
		off()
		value(cl.Dispose(ctx).Result(ctx))
		mu.Lock()
		after := len(seen)
		mu.Unlock()
		check(c, after == count, "inspection unsubscribe ineffective")
	})},
}
