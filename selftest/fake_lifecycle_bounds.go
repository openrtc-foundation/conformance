package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"time"
)

// The fake's documented scheduling bounds apply to its in-process test environment.
// RFC §8.5:487–500, §8.6:510. A run watchdog is independently 1500 ms.
func (*FakeTarget) BindingTiming() harness.BindingTiming {
	return harness.BindingTiming{Reference: "selftest/fake_lifecycle_bounds.go", Environment: "in-process Go scheduler", Scope: "all fake owners", SuspensionRules: "monotonic elapsed time; measurements invalid during process suspension", Cancellation: 50 * time.Millisecond, DeadlineLag: 50 * time.Millisecond, TerminationFence: 50 * time.Millisecond, Release: 50 * time.Millisecond, CleanupRetention: 80 * time.Millisecond}
}

type boundControl struct{ t *FakeTarget }

func (t *FakeTarget) LifecycleBounds() harness.LifecycleBoundControl { return boundControl{t} }
func (c boundControl) HoldCreateClient() harness.JoinGate {
	g := &worldGate{reached: harness.NewDeferred[struct{}](), outcome: harness.NewDeferred[any](), reachedCh: make(chan struct{})}
	c.t.world.mu.Lock()
	c.t.world.createGate = g
	c.t.world.mu.Unlock()
	return joinGateControl{g}
}
func (c boundControl) HoldCleanup(acknowledge bool) {
	c.t.world.mu.Lock()
	defer c.t.world.mu.Unlock()
	c.t.world.holdCleanup = true
	c.t.world.cleanupAcknowledges = acknowledge
}
func (c boundControl) ConfirmRelease(ctx context.Context) error {
	return lifecycleControl{c.t}.CompleteCleanup(ctx)
}
func (w *world) compensation(s *fakeSession) {
	w.mu.Lock()
	w.counts.Leaves++
	hold := w.holdCleanup
	if hold {
		w.pendingCleanup[s] = true
	}
	delete(w.sessions, s)
	delete(w.held, s)
	w.mu.Unlock()
	s.stateMu.Lock()
	s.nativeJoined = false
	s.stateMu.Unlock()
	s.releaseIdentity()
	if hold && !s.defects.CleanupAcknowledgedAsReleased {
		s.setCleanup(openrtc.CleanupStatus{Kind: "unknown", Outstanding: []string{"late admission / remote participation release"}, Reason: "submission acknowledgement is not release evidence"})
	} else {
		s.setCleanup(openrtc.CleanupStatus{Kind: "complete"})
	}
}
