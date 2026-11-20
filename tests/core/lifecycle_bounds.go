package core

import (
	"context"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// Binding bounds are documented target promises; OperationMs remains a separate
// harness watchdog. Requirement locators refer to the amended core RFC.
func bindingBounds(c *harness.TestContext) harness.BindingTiming {
	t, ok := c.Target.(harness.BindingTimingTarget)
	if !ok {
		must(c.NotRun("RFC §8.5:487–500: binding timing documentation unavailable"))
	}
	b := t.BindingTiming()
	check(c, b.Reference != "" && b.Environment != "" && b.Scope != "" && b.SuspensionRules != "", "binding timing lacks documentary scope or clock rules")
	for _, d := range []time.Duration{b.Cancellation, b.DeadlineLag, b.TerminationFence, b.Release, b.CleanupRetention} {
		check(c, d > 0, "documented timing limit must be positive and finite")
		if d >= time.Duration(c.Target.Timing().OperationMs)*time.Millisecond {
			must(c.NotRun("declared binding limit exceeds available harness run budget"))
		}
	}
	return b
}
func boundControl(c *harness.TestContext) harness.LifecycleBoundControl {
	t, ok := c.Target.(harness.LifecycleBoundTarget)
	if !ok {
		must(c.NotRun("fixture lacks independent cleanup/create controls"))
	}
	return t.LifecycleBounds()
}
func waitLifecycleGate(ctx context.Context, c *harness.TestContext, g harness.JoinGate) {
	must(c.Within(ctx, "native checkpoint", 0, func(ctx context.Context) error {
		select {
		case <-g.Reached():
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}))
}
func inBindingBound(ctx context.Context, c *harness.TestContext, d time.Duration, label string, f func(context.Context) error) {
	// The action receives an uncancelled operation-result waiter. A watchdog's
	// expiry is a failure, never a portable target timeout or cancellation result.
	start := time.Now()
	must(c.Within(ctx, label, int((d+time.Millisecond-1)/time.Millisecond), f))
	check(c, time.Since(start) <= d, label+" exceeded its documented binding limit")
}
func expectCancelled(ctx context.Context, c *harness.TestContext, a openrtc.JoinAttempt, d time.Duration) *openrtc.RTCFailure {
	var e *openrtc.RTCFailure
	inBindingBound(ctx, c, d, "cancelled result", func(ctx context.Context) error {
		_, err := a.Result(ctx).Result(ctx)
		e = value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "operation settlement"))
		return nil
	})
	_, err := a.Result(ctx).Result(ctx)
	again := value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "repeated result"))
	check(c, e.ID == again.ID, "repeated result changed its final failure")
	return e
}
func gatedJoin(ctx context.Context, c *harness.TestContext) (openrtc.RTCClient, openrtc.RTCConferenceSession, openrtc.JoinAttempt, harness.JoinGate) {
	cl := client(ctx, c)
	s := c.Open(cl, value(c.Conference(ctx)))
	g := value(c.RequireControl("gateNextJoin")).(func() harness.JoinGate)()
	a := s.Join(opts(c))
	waitLifecycleGate(ctx, c, g)
	return cl, s, a, g
}
func cancelWithin(ctx context.Context, c *harness.TestContext, a openrtc.JoinAttempt, d time.Duration) openrtc.CancellationReport {
	var r openrtc.CancellationReport
	inBindingBound(ctx, c, d, "cancellation receipt to disposition and settlement", func(ctx context.Context) error {
		r = value(a.Cancel(ctx).Result(ctx))
		if r.Won {
			expectCancelled(ctx, c, a, d)
		}
		return nil
	})
	return r
}
func boundTest(id, title string, refs []string, body func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	return harness.DefineTest(harness.ConformanceTest{ID: id, Title: title, Suite: harness.Core, Profile: harness.Common, Requirements: refs, Observable: title, Run: run(body)})
}

var LifecycleBoundTests = []harness.ConformanceTest{
	boundTest("CT-BOUND-001", "local cancellation and owner fences settle while native operations never resolve", []string{"RFC §7.1:84", "RFC §8.4:483", "RFC §8.5:487–500", "RFC A.2:1331"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		p := value(c.Provider(ctx, nil))
		control := boundControl(c)
		g := control.HoldCreateClient()
		defer g.ReleaseSuccess()
		invocation, cancel := context.WithCancel(ctx)
		defer cancel()
		op := p.CreateClient(invocation, c.Target.ClientConfig(harness.ClientConfigInput{Identity: c.FreshIdentity("blocked-create"), Credentials: c.Spy.Provider(false)}))
		waitLifecycleGate(ctx, c, g)
		start := time.Now()
		cancel()
		inBindingBound(ctx, c, b.Cancellation, "createClient invocation cancellation", func(ctx context.Context) error {
			_, err := op.Result(ctx)
			value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "createClient settlement"))
			return nil
		})
		check(c, time.Since(start) <= b.Cancellation, "create cancellation exceeded receipt limit")
		for _, route := range []string{"cancel", "leave", "dispose"} {
			cl, s, a, g := gatedJoin(ctx, c)
			defer g.ReleaseSuccess()
			if route == "cancel" {
				check(c, cancelWithin(ctx, c, a, b.Cancellation).Won, "pending cancellation did not win")
			} else {
				release := s.Leave
				if route == "dispose" {
					release = cl.Dispose
				}
				var terminalCommit atomic.Int64
				off := s.Observation().Watch(func(snapshot openrtc.SessionSnapshot) {
					if snapshot.Connection.State == "ended" {
						terminalCommit.CompareAndSwap(0, time.Now().UnixNano())
					}
				})
				inBindingBound(ctx, c, b.Release, "local release invocation", func(ctx context.Context) error { _, err := release(ctx).Result(ctx); return err })
				off()
				// Disposal may remove the observer before closure delivery. Its returned
				// terminal public snapshot is then the available termination checkpoint.
				if terminalCommit.Load() == 0 && s.Connection().Snapshot().State == "ended" {
					terminalCommit.Store(time.Now().UnixNano())
				}
				check(c, terminalCommit.Load() != 0, "local termination checkpoint not observed")
				remaining := b.TerminationFence - time.Since(time.Unix(0, terminalCommit.Load()))
				check(c, remaining > 0, "fenced operation did not settle within terminal-commit interval")
				expectCancelled(ctx, c, a, remaining)
			}
			check(c, s.Connection().Snapshot().State == "ended", "pending owner was not terminal")
			checkReport(c, s.Inspection().Cleanup(), "native operation remains blocked")
			check(c, s.Inspection().Cleanup().Status.Kind != "complete", "unobserved native outcome was invented as release")
		}
		additionalPendingBounds(ctx, c, b)
	}),
	boundTest("CT-BOUND-002", "actual invocation deadline settles without native completion and preserves early success", []string{"RFC §8.5:487–500", "RFC §9:539–570"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		g := value(c.RequireControl("gateNextJoin")).(func() harness.JoinGate)()
		defer g.ReleaseSuccess()
		expiry := time.Now().Add(90 * time.Millisecond)
		signal, cancel := context.WithDeadline(ctx, expiry)
		defer cancel()
		o := opts(c)
		o.Signal = signal
		a := s.Join(o)
		waitLifecycleGate(ctx, c, g)
		var err error
		inBindingBound(ctx, c, time.Until(expiry)+b.DeadlineLag, "deadline expiry and settlement", func(ctx context.Context) error { _, err = a.Result(ctx).Result(ctx); return nil })
		check(c, time.Now().After(expiry) || time.Now().Equal(expiry), "operation timed out before its actual deadline")
		check(c, time.Now().Before(expiry.Add(b.DeadlineLag)), "operation exceeded deadline expiry lag")
		e := value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"timeout"}}, "deadline result"))
		_, err = a.Result(ctx).Result(ctx)
		check(c, value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"timeout"}}, "stable timeout")).ID == e.ID, "deadline result changed")
		g.ReleaseSuccess()
		must(lifecycleFaults(c).Quiesce(ctx, s))
		check(c, s.Connection().Snapshot().State != "connected", "late success resurrected timed out operation")
		s2 := c.Open(cl, value(c.Conference(ctx)))
		signal2, stop := context.WithTimeout(ctx, 30*time.Millisecond)
		defer stop()
		o = opts(c)
		o.Signal = signal2
		a2 := s2.Join(o)
		result := value(a2.Result(ctx).Result(ctx))
		must(c.Sleep(ctx, 40))
		again := value(a2.Result(ctx).Result(ctx))
		check(c, reflect.DeepEqual(result, again), "expiry overwrote early successful completion")
	}),
	boundTest("CT-RACE-001", "cancellation and successful connection commit have one stable winner", []string{"RFC §7.2:117–131", "RFC §8.4:483", "RFC §8.5:494–495"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		for _, schedule := range []string{"cancel-first", "complete-first", "overlap"} {
			_, s, a, g := gatedJoin(ctx, c)
			defer g.ReleaseSuccess()
			var r openrtc.CancellationReport
			if schedule == "complete-first" {
				g.ReleaseSuccess()
				value(a.Result(ctx).Result(ctx))
				r = cancelWithin(ctx, c, a, b.Cancellation)
				check(c, !r.Won, "completion winner was overwritten")
			} else if schedule == "cancel-first" {
				r = cancelWithin(ctx, c, a, b.Cancellation)
				check(c, r.Won, "held cancellation did not win")
				g.ReleaseSuccess()
			} else {
				var wg sync.WaitGroup
				wg.Add(1)
				go func() { defer wg.Done(); g.ReleaseSuccess() }()
				r = cancelWithin(ctx, c, a, b.Cancellation)
				wg.Wait()
			}
			must(lifecycleFaults(c).Quiesce(ctx, s))
			if r.Won {
				expectCancelled(ctx, c, a, b.Cancellation)
				check(c, s.Connection().Snapshot().State != "connected", "cancel winner resurrected")
			} else {
				first := value(a.Result(ctx).Result(ctx))
				second := value(a.Result(ctx).Result(ctx))
				check(c, first.Admission == "admitted" && reflect.DeepEqual(first, second) && s.Connection().Snapshot().State == "connected", "completion winner lacks stable admitted result")
			}
			value(s.Leave(ctx).Result(ctx))
		}
	}),
	boundTest("CT-RACE-002", "late admission cannot rewrite cancellation and cleanup remains independently observed", []string{"RFC §7.2:131", "RFC §8.5:497–500", "RFC §8.6:510"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		control := boundControl(c)
		control.HoldCleanup(true)
		_, s, a, g := gatedJoin(ctx, c)
		defer g.ReleaseSuccess()
		check(c, cancelWithin(ctx, c, a, b.Cancellation).Won, "cancellation lost held join")
		g.ReleaseSuccess()
		must(lifecycleFaults(c).Quiesce(ctx, s))
		expectCancelled(ctx, c, a, b.Cancellation)
		check(c, s.Connection().Snapshot().State != "connected", "late admission resurrected cancelled participation")
		r := s.Inspection().Cleanup()
		checkReport(c, r, "late acquisition cleanup")
		check(c, r.Status.Kind != "complete", "late release submission falsely established cleanup")
		value(s.Dispose(ctx).Result(ctx))
		checkReport(c, s.Inspection().Cleanup(), "post-disposal late cleanup")
		must(control.ConfirmRelease(ctx))
		check(c, s.Inspection().Cleanup().Status.Kind == "complete", "independently observed cleanup did not progress")
		expectCancelled(ctx, c, a, b.Cancellation)
	}),
	boundTest("CT-CLEAN-001", "partial cleanup remains identifiable throughout the documented retention window", []string{"RFC §8.6:508–510", "RFC §9:570", "RFC §14.3:1131"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		f := lifecycleFaults(c)
		s := joined(ctx, c).Session
		baseline := f.Resources()
		f.FailNext("leave", "cleanup", "unavailable")
		inBindingBound(ctx, c, b.Release, "partially failing leave", func(ctx context.Context) error { _, err := s.Leave(ctx).Result(ctx); return err })
		resources := f.Resources()
		check(c, resources.Acquired == baseline.Acquired+2 && resources.Released == baseline.Released+1, "two-resource fixture did not independently establish partial cleanup")
		first := s.Inspection().Cleanup()
		checkReport(c, first, "partial cleanup")
		check(c, first.Status.Kind == "failed" && len(first.Status.Errors) > 0, "partial cleanup failure absent")
		inBindingBound(ctx, c, b.Release, "dispose with unresolved cleanup", func(ctx context.Context) error { _, err := s.Dispose(ctx).Result(ctx); return err })
		start := time.Now()
		for _, checkpoint := range []time.Duration{b.CleanupRetention / 2, b.CleanupRetention * 9 / 10} {
			remaining := checkpoint - time.Since(start)
			if remaining > 0 {
				must(c.Sleep(ctx, int((remaining+time.Millisecond-1)/time.Millisecond)))
			}
			r := s.Inspection().Cleanup()
			checkReport(c, r, "retained cleanup")
			check(c, r.Status.Kind != "complete" && len(r.Status.Outstanding) > 0, "disposal lost unresolved cleanup during retention")
		}
		must(f.CompleteCleanup(ctx))
		check(c, s.Inspection().Cleanup().Status.Kind == "complete", "later observed release is inaccessible")
	}),
	boundTest("CT-CLEAN-002", "repeated concurrent release remains bounded and terminal while remote release is unknown", []string{"RFC §7.2:151–169", "RFC §8.5:487", "RFC §8.6:506–510"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		control := boundControl(c)
		control.HoldCleanup(false)
		s := joined(ctx, c).Session
		sibling := joined(ctx, c).Session
		inBindingBound(ctx, c, b.Release, "initial leave", func(ctx context.Context) error { _, err := s.Leave(ctx).Result(ctx); return err })
		identity := s.ID()
		first := s.Inspection().Cleanup()
		checkReport(c, first, "unobserved release")
		check(c, first.Status.Kind != "complete", "unknown remote cleanup presented complete")
		releaseStart := time.Now()
		ops := []openrtc.Async[openrtc.Void]{s.Leave(ctx), s.Dispose(ctx), s.Leave(ctx), s.Dispose(ctx)}
		for _, op := range ops {
			inBindingBound(ctx, c, b.Release, "repeated release", func(ctx context.Context) error { _, err := op.Result(ctx); return err })
		}
		check(c, time.Since(releaseStart) <= b.Release, "concurrent release invocation exceeded binding limit")
		check(c, sibling.Connection().Snapshot().State == "connected", "release affected another owner's live participation")
		check(c, s.ID() == identity && s.Connection().Snapshot().State == "ended", "repeated release changed identity or resurrected participation")
		r := s.Inspection().Cleanup()
		checkReport(c, r, "repeated release retained cleanup")
		check(c, r.Status.Kind != "complete" && reflect.DeepEqual(first.Status.Outstanding, r.Status.Outstanding), "repeated release lost cleanup identity")
	}),
	boundTest("CT-CLEAN-003", "native release acknowledgement and stalled release remain unknown until separate release evidence", []string{"RFC §7.2:140–158", "RFC §8.5:500", "RFC §8.6:508–510"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		control := boundControl(c)
		for _, ack := range []bool{true, false} {
			control.HoldCleanup(ack)
			_, s, a, g := gatedJoin(ctx, c)
			defer g.ReleaseSuccess()
			check(c, cancelWithin(ctx, c, a, b.Cancellation).Won, "held join cancellation lost")
			g.ReleaseSuccess()
			must(lifecycleFaults(c).Quiesce(ctx, s))
			value(s.Dispose(ctx).Result(ctx))
			expectCancelled(ctx, c, a, b.Cancellation)
			r := s.Inspection().Cleanup()
			checkReport(c, r, "unobserved compensation")
			check(c, r.Status.Kind != "complete" && len(r.Status.Outstanding) > 0, "acknowledgement or blocked cleanup invented remote release")
			must(control.ConfirmRelease(ctx))
			check(c, s.Inspection().Cleanup().Status.Kind == "complete", "independent release evidence not retained")
		}
	}),
	boundTest("CT-STALE-001", "obsolete generation callbacks cannot resurrect owners or alter a new incarnation", []string{"RFC §6:35", "RFC §8.2:461", "RFC §8.4:483", "RFC §8.5:497–498", "RFC §8.6:510", "RFC §14.3:1149"}, func(ctx context.Context, c *harness.TestContext) {
		bindingBounds(c)
		f := lifecycleFaults(c)
		events := lifecycleEvents(c)
		cl := client(ctx, c)
		room := value(c.Conference(ctx))
		a := c.Open(cl, room)
		awaitJoin(ctx, c, a.Join(opts(c)), "join A")
		value(a.Dispose(ctx).Result(ctx))
		b := c.Open(cl, room)
		awaitJoin(ctx, c, b.Join(opts(c)), "join B")
		before := b.Observation().Snapshot()
		check(c, a.Generation() != b.Generation(), "new public incarnation reused generation identity")
		resources := f.Resources()
		batch := []harness.LifecycleNativeEvent{{Kind: "connected", NativeUID: "reused-native-id", NativeKey: "old-incarnation"}, {Kind: "media", NativeUID: "reused-native-id", NativeKey: "old-incarnation"}, {Kind: "upsert", NativeUID: "reused-native-id", NativeKey: "old-incarnation", DisplayName: "obsolete"}, {Kind: "allocation", NativeUID: "reused-native-id", NativeKey: "late-allocation"}}
		must(events.Submit(ctx, a, a.Generation(), batch))
		must(events.Submit(ctx, a, a.Generation(), batch))
		check(c, a.Connection().Snapshot().State == "ended" && reflect.DeepEqual(before, b.Observation().Snapshot()), "obsolete callback resurrected A or mutated B")
		check(c, f.Resources().Acquired-resources.Acquired == 1, "duplicate allocation callback fabricated another acquired resource")
		r := a.Inspection().Cleanup()
		checkReport(c, r, "obsolete allocation cleanup")
		check(c, r.Status.Kind != "complete", "late allocation lost identifiable cleanup")
		must(f.CompleteCleanup(ctx))
		check(c, a.Inspection().Cleanup().Status.Kind == "complete", "late resource release evidence inaccessible")
	}),
	boundTest("CT-ADMIT-001", "all legacy join selectors retain pending admission until admission-positive success", []string{"RFC §7.2:117–131", "RFC §8.1:417–419", "RFC A.2:1314"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		f := lifecycleFaults(c)
		admit := value(c.RequireControl("admission")).(func() harness.AdmissionControl)()
		cl := client(ctx, c)
		for _, selector := range []string{"", "admitted", "admission-pending"} {
			admit.Hold()
			s := c.Open(cl, value(c.Conference(ctx)))
			o := opts(c)
			o.Completion = selector
			a := s.Join(o)
			must(f.AwaitJoinMilestone(ctx, s))
			probe, stop := context.WithCancel(ctx)
			stop()
			_, err := a.Result(ctx).Result(probe)
			check(c, err == context.Canceled, "join succeeded before admission")
			check(c, s.Connection().Snapshot().Admission != "admitted" && s.Connection().Snapshot().State != "connected", "held join invented admitted connection")
			admit.Admit()
			result := value(a.Result(ctx).Result(ctx))
			check(c, result.Admission == "admitted" && result.Connection.State == "connected", "admitted result lacks coherent success snapshot")
			value(s.Leave(ctx).Result(ctx))
		}
		admit.Deny()
		s := c.Open(cl, value(c.Conference(ctx)))
		_, err := s.Join(opts(c)).Result(ctx).Result(ctx)
		value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"permission-denied"}}, "admission denied"))
		check(c, s.Connection().Snapshot().State == "ended", "denied participation remained live")
		admit.Hold()
		s = c.Open(cl, value(c.Conference(ctx)))
		a := s.Join(opts(c))
		must(f.AwaitJoinMilestone(ctx, s))
		check(c, cancelWithin(ctx, c, a, b.Cancellation).Won, "held admission cancellation lost")
		must(f.Quiesce(ctx, s))
		admit.Hold()
		s = c.Open(cl, value(c.Conference(ctx)))
		deadline, stop := context.WithTimeout(ctx, 25*time.Millisecond)
		defer stop()
		o := opts(c)
		o.Signal = deadline
		a = s.Join(o)
		must(f.AwaitJoinMilestone(ctx, s))
		inBindingBound(ctx, c, 25*time.Millisecond+b.DeadlineLag, "held admission invocation deadline", func(ctx context.Context) error {
			_, err := a.Result(ctx).Result(ctx)
			value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"timeout"}}, "held admission deadline"))
			return nil
		})
	}),
}

// CT-CANCEL-REASON-001 requires selection only where the exact operation is
// advertised; lack of a fixture cannot manufacture unsupported evidence.
var SelectionCancellationTests = []harness.ConformanceTest{
	boundTest("CT-CANCEL-REASON-001", "selection dismissal and operation cancellation retain distinct winning reasons", []string{"RFC §8.5:489", "RFC §9:538–546", "RFC §14.3:1128"}, func(ctx context.Context, c *harness.TestContext) {
		b := bindingBounds(c)
		target, ok := c.Target.(harness.SelectionCancellationTarget)
		if !ok {
			must(c.NotRun("selection capability and controlled dismissal mapping unavailable"))
		}
		for _, schedule := range []string{"dismiss-first", "cancel-first"} {
			fixture := target.SelectionCancellation()
			switch fixture.Capability.Support.Level {
			case "unsupported", "not-applicable":
				must(c.NotApplicable("exact screen selection capability is advertised unsupported"))
			case "supported":
			default:
				must(c.Unknown("scoped selection applicability unresolved"))
			}
			if fixture.Start == nil || fixture.Dismiss == nil {
				must(c.NotRun("supported selection has no operation/dismissal fixture"))
			}
			invocation, cancel := context.WithCancel(ctx)
			op := fixture.Start.Start(invocation, openrtc.ScreenShareRequest{Mode: "capture", Kinds: []openrtc.MediaKind{"screen"}, Ownership: "session"})
			var err error
			inBindingBound(ctx, c, b.Cancellation, "selection winning disposition", func(ctx context.Context) error {
				if schedule == "dismiss-first" {
					fixture.Dismiss()
				} else {
					cancel()
				}
				_, err = op.Result(ctx)
				return nil
			})
			e := value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "selection termination"))
			if schedule == "dismiss-first" {
				check(c, containsReason(e.Reasons, "selection-dismissed"), "explicit dismissal lost selection-dismissed reason")
				cancel()
			} else {
				check(c, !containsReason(e.Reasons, "selection-dismissed"), "operation cancellation falsely labeled selection-dismissed")
				fixture.Dismiss()
			}
			_, err = op.Result(ctx)
			again := value(c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "selection finality"))
			check(c, e.ID == again.ID, "selection race rewrote final result")
		}
	}),
}

func containsReason(reasons []string, want string) bool {
	for _, r := range reasons {
		if r == want {
			return true
		}
	}
	return false
}
