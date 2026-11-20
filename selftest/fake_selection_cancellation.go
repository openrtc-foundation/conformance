package selftest

import (
	"context"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// The normal fake advertises no screen profile. This selftest-only target maps a
// scoped supported selector into the same public screen operation interface.
type selectionTarget struct {
	*FakeTarget
	wrongReason bool
}

func (t selectionTarget) SelectionCancellation() harness.SelectionCancellationFixture {
	s := &fakeSelector{done: harness.NewDeferred[openrtc.ScreenShareResult](), wrongReason: t.wrongReason}
	cap := descriptorFor(openrtc.CapabilityScreenSharing, 1)
	cap.Support.Level = "supported"
	return harness.SelectionCancellationFixture{Start: s, Capability: cap, Dismiss: s.dismiss}
}

type fakeSelector struct {
	mu          sync.Mutex
	done        *harness.Deferred[openrtc.ScreenShareResult]
	wrongReason bool
}

func (s *fakeSelector) settle(dismissed bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done.Settled() {
		return
	}
	e := fail("cancelled", "captureScreen", "selection ended", failureExtras{})
	if dismissed && !s.wrongReason {
		e.(*openrtc.RTCOperationError).Reasons = []string{"selection-dismissed"}
	}
	s.done.Reject(e)
}
func (s *fakeSelector) dismiss() { s.settle(true) }
func (s *fakeSelector) Start(ctx context.Context, _ openrtc.ScreenShareRequest) openrtc.Async[openrtc.ScreenShareResult] {
	go func() {
		select {
		case <-ctx.Done():
			s.settle(false)
		}
	}()
	return startOperation(s, func() (openrtc.ScreenShareResult, error) {
		v, reason, rejected, err := s.done.Await(context.Background())
		if rejected {
			return v, reason.(error)
		}
		return v, err
	})
}
