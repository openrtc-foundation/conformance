package selftest

import (
	"context"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

var _ openrtc.RTCConferenceSession = (*fakeSession)(nil)
var _ openrtc.SessionConnection = (*fakeConnection)(nil)

type fakeConnection struct{ session *fakeSession }

func (s *fakeSession) Connection() openrtc.SessionConnection { return fakeConnection{s} }
func (c fakeConnection) Generation() openrtc.GenerationID    { return c.session.generation }
func (c fakeConnection) State() openrtc.ConnectionState {
	return c.session.observation.Snapshot().Connection.State
}
func (c fakeConnection) Admission() openrtc.AdmissionStatus {
	return c.session.observation.Snapshot().Connection.Admission
}
func (fakeConnection) MediaReadiness() openrtc.MediaReadiness { return "inactive" }
func (fakeConnection) Recovery() openrtc.Known[openrtc.RecoveryStatus] {
	return openrtc.Unknown[openrtc.RecoveryStatus]()
}
func (fakeConnection) Attempt() openrtc.Known[openrtc.ConnectionAttempt] {
	return openrtc.Unknown[openrtc.ConnectionAttempt]()
}
func (c fakeConnection) Cleanup() openrtc.CleanupStatus {
	return c.session.observation.Snapshot().Cleanup
}
func (c fakeConnection) Leave(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(c, func() (openrtc.CleanupReport, error) {
		_, err := c.session.Leave(ctx).Result(ctx)
		return c.session.Inspection().Cleanup(), err

	}))
}

func (s *fakeSession) Leave(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(s, func() (openrtc.CleanupReport, error) {
		if f := s.world.takeFault("leave", "fence"); f != nil {
			if s.defects.FenceFailureCloses {
				s.setState("ended")
			}
			return openrtc.CleanupReport{}, faultError(f)
		}
		return s.release(ctx)

	}))
}
func (s *fakeSession) Dispose(ctx context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(s, func() (openrtc.CleanupReport, error) {
		if f := s.world.takeFault("dispose", "fence"); f != nil {
			if s.defects.FenceFailureCloses {
				s.setState("ended")
			}
			return openrtc.CleanupReport{}, faultError(f)
		}
		closeOwnerListeners(s, s.defects)
		s.stateMu.Lock()
		s.disposed = true
		s.stateMu.Unlock()
		report, err := s.release(ctx)
		if s.defects.CleanupRetentionLost {
			s.setCleanup(openrtc.CleanupStatus{Kind: "complete"})
		}
		return report, err

	}))
}

func (s *fakeSession) release(ctx context.Context) (openrtc.CleanupReport, error) {
	s.joinMu.Lock()
	a := s.attempt
	s.joinMu.Unlock()
	if a != nil && !a.done.Settled() && !a.isFenced() {
		if s.defects.FenceLeavesSetupRunning {
			a.done.Reject(fail("invalid-state", "join", "owner fence failed to cancel setup", failureExtras{}))
		} else {
			_, _ = s.cancel(a)
		}
	}
	if s.defects.RepeatedReleaseResurrects && terminal(s.connectionState()) {
		s.setState("connected")
	}
	s.cleanupMu.Lock()
	promise := s.cleanupPromise
	if promise == nil || s.defects.DuplicateCleanup {
		promise = harness.NewDeferred[openrtc.CleanupReport]()
		s.cleanupPromise = promise
		go func() { promise.Resolve(s.runCleanup()) }()
	}
	s.cleanupMu.Unlock()
	report, _, _, err := promise.Await(ctx)
	if s.defects.CleanupRejectsClosure && report.Status.Kind == "failed" {
		return report, fail("cleanup-failed", "leave", "release failed", failureExtras{})
	}
	return report, err
}

func (s *fakeSession) runCleanup() openrtc.CleanupReport {
	s.cleanupRunMu.Lock()
	defer s.cleanupRunMu.Unlock()
	s.leaveCount++
	state := s.observation.Snapshot().Connection.State
	s.stateMu.Lock()
	joined := s.nativeJoined
	s.stateMu.Unlock()
	wasJoined := joined && state != "ended" && state != "failed"
	if wasJoined {
		s.setState(openrtc.ConnectionState("leaving"))
		s.setCleanup(openrtc.CleanupStatus{Kind: "pending", Outstanding: []string{"native leave"}})
		s.world.mu.Lock()
		s.world.counts.Leaves++
		delete(s.world.sessions, s)
		delete(s.world.held, s)
		delete(s.world.activeIdentities, s.identityKey)
		if s.defects.LeaveEndsForEveryone {
			for _, remote := range s.world.remotesAt(s.addressValue) {
				remote.connected = false
			}
		}
		s.world.mu.Unlock()
		s.setState(openrtc.ConnectionState("ended"))
		s.world.mu.Lock()
		hold := s.world.holdCleanup
		if hold {
			s.world.pendingCleanup[s] = true
		}
		s.world.mu.Unlock()
		if hold && !s.defects.CleanupAcknowledgedAsReleased {
			s.setCleanup(openrtc.CleanupStatus{Kind: "unknown", Outstanding: []string{"remote participation release"}})
		} else {
			s.setCleanup(openrtc.CleanupStatus{Kind: "complete"})
		}
	}
	if !wasJoined && s.defects.DuplicateCleanup && s.leaveCount > 1 {
		s.world.mu.Lock()
		s.world.counts.Leaves++
		s.world.mu.Unlock()
	}
	if !wasJoined && !s.defects.IdleLeaveStaysIdle && !terminal(state) {
		s.setState("leaving")
		s.setState("ended")
	}
	f := s.world.takeFault("leave", "cleanup")
	if f == nil {
		f = s.world.takeFault("dispose", "cleanup")
	}
	if f != nil {
		s.world.mu.Lock()
		s.world.resources.Released++
		s.world.pendingCleanup[s] = true
		s.world.mu.Unlock()
		s.setCleanup(openrtc.CleanupStatus{Kind: "failed", Outstanding: []string{"native resource still owned"}, Errors: []openrtc.RTCError{fail("cleanup-failed", "leave", "native cleanup failed", failureExtras{})}})
	}
	return openrtc.CleanupReport{Status: s.observation.Snapshot().Cleanup, Generation: openrtc.KnownValue(s.generation)}
}

func (c fakeConnection) Snapshot() openrtc.ConnectionSnapshot {
	return c.session.observation.Snapshot().Connection
}
func (s *fakeSession) Generation() openrtc.GenerationID { return s.generation }

func terminal(s openrtc.ConnectionState) bool { return s == "ended" || s == "failed" }
