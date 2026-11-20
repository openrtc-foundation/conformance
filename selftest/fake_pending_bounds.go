package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"time"
)

// The fake's additional controlled scope is its actually advertised statistics
// sample operation. Provider targets enumerate every supported async scope.
func (*FakeTarget) PendingOperationClaims() []harness.OperationClaim {
	return []harness.OperationClaim{{Operation: "statsSample", Scope: "session", Support: "supported"}}
}
func (t *FakeTarget) HoldPendingOperation(_ context.Context, claim harness.OperationClaim, session openrtc.RTCConferenceSession) (harness.PendingOperationFixture, error) {
	g := &worldGate{reached: harness.NewDeferred[struct{}](), outcome: harness.NewDeferred[any](), reachedCh: make(chan struct{})}
	s := session.(*fakeSession)
	t.world.mu.Lock()
	t.world.pendingAsync[s] = g
	t.world.mu.Unlock()
	cap := descriptorFor("media.statistics", 1)
	statistics := s.Extension("statistics").(openrtc.MediaStatistics)
	return harness.PendingOperationFixture{PublicOperation: "session.statistics.Sample", Capability: cap, Gate: joinGateControl{g}, Invoke: func(ctx context.Context) openrtc.Async[openrtc.Void] {
		return openrtc.Discard(statistics.Sample(ctx, openrtc.StatisticsTarget{Kind: "session"}))
	}}, nil
}
func (w *world) awaitPending(ctx context.Context, s *fakeSession) error {
	w.mu.Lock()
	g := w.pendingAsync[s]
	delete(w.pendingAsync, s)
	w.mu.Unlock()
	if g == nil {
		return nil
	}
	close(g.reachedCh)
	native := make(chan struct{})
	go func() { _, _, _, _ = g.outcome.Await(context.Background()); close(native) }()
	if s.defects.PendingAsyncWaitsProvider {
		<-native
		return nil
	}
	poll := time.NewTicker(time.Millisecond)
	defer poll.Stop()
	for {
		select {
		case <-native:
			return nil
		case <-ctx.Done():
			return fail("cancelled", "statsSample", "invocation cancelled while native sample remains unresolved", failureExtras{})
		case <-poll.C:
			if terminal(s.connectionState()) {
				return fail("cancelled", "statsSample", "owner termination fenced pending sample", failureExtras{})
			}
		}
	}
}
