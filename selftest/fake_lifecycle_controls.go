package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type lifecycleFault struct {
	operation, phase string
	code             openrtc.ErrorCode
}
type lifecycleControl struct{ target *FakeTarget }

func (t *FakeTarget) LifecycleFaults() harness.LifecycleFaultControl { return lifecycleControl{t} }
func (c lifecycleControl) FailNext(op, phase string, code openrtc.ErrorCode) {
	w := c.target.world
	w.mu.Lock()
	defer w.mu.Unlock()
	w.fault = &lifecycleFault{op, phase, code}
	if phase == "cleanup" {
		w.resources.Acquired += 2
	}
}
func (c lifecycleControl) Resources() harness.LifecycleResources {
	w := c.target.world
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.resources
}
func (w *world) takeFault(op, phase string) *lifecycleFault {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fault != nil && w.fault.operation == op && w.fault.phase == phase {
		f := w.fault
		w.fault = nil
		return f
	}
	return nil
}
func setOwnerCleanup(owner any, r openrtc.CleanupReport) {
	if s, ok := owner.(*fakeSession); ok {
		s.setCleanup(r.Status)
		return
	}
	st := storeFor(owner)
	st.mu.Lock()
	st.cleanup = &r
	st.revision++
	fs := make([]func(), 0, len(st.watchers))
	for _, f := range st.watchers {
		fs = append(fs, f)
	}
	st.mu.Unlock()
	for _, f := range fs {
		isolated(f)
	}
	notifyCleanup(owner)
}
func (w *world) acquireUnreturned(owner any, defects FakeDefects) {
	w.mu.Lock()
	w.resources.Acquired++
	if !defects.ExecutionDropsCleanup {
		w.pendingCleanup[owner] = true
	}
	w.mu.Unlock()
	if !defects.ExecutionDropsCleanup {
		setOwnerCleanup(owner, openrtc.CleanupReport{Status: openrtc.CleanupStatus{Kind: "pending", Outstanding: []string{"unreturned native resource"}}, Generation: openrtc.Unknown[openrtc.GenerationID]()})
	}
}
func (c lifecycleControl) CompleteCleanup(context.Context) error {
	w := c.target.world
	w.mu.Lock()
	owners := make([]any, 0, len(w.pendingCleanup))
	for o := range w.pendingCleanup {
		owners = append(owners, o)
		w.resources.Released++
		delete(w.pendingCleanup, o)
	}
	w.mu.Unlock()
	for _, o := range owners {
		setOwnerCleanup(o, completeCleanup())
	}
	return nil
}
func (c lifecycleControl) Quiesce(ctx context.Context, session openrtc.RTCConferenceSession) error {
	s := session.(*fakeSession)
	s.joinMu.Lock()
	done := s.workerDone
	s.joinMu.Unlock()
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func faultError(f *lifecycleFault) error {
	return fail(f.code, f.operation, "fixture established "+f.phase+" condition", failureExtras{})
}
func isolated(f func()) { defer func() { _ = recover() }(); f() }

func ownerDefects(o any) FakeDefects {
	switch v := o.(type) {
	case *fakeMedia:
		return v.session.defects
	case *fakeSession:
		return v.defects
	case *fakeClient:
		return v.defects
	case *fakeProvider:
		return v.defects
	}
	return NoDefects
}
func ownerDisposed(o any) bool {
	switch v := o.(type) {
	case *fakeSession:
		v.stateMu.Lock()
		defer v.stateMu.Unlock()
		return v.disposed
	case *fakeClient:
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.disposed
	case *fakeProvider:
		v.mu.Lock()
		defer v.mu.Unlock()
		return v.disposed
	}
	return false
}
func validateWatch(o any, nilListener bool) {
	d := ownerDefects(o)
	if nilListener && !d.WatchAcceptsNil {
		panic(fail("invalid-option", "watch", "nil listener", failureExtras{}))
	}
	if ownerDisposed(o) && !d.WatchAfterDispose {
		panic(fail("invalid-state", "watch", "disposed", failureExtras{}))
	}
}
func closeOwnerListeners(o any, d FakeDefects) {
	if d.DisposeKeepsListeners {
		return
	}
	if s, ok := o.(*fakeSession); ok {
		s.observation.mu.Lock()
		s.observation.closed = true
		for _, r := range s.observation.registrations {
			r.active = false
		}
		for _, w := range s.observation.watchers {
			w.active = false
		}
		s.observation.mu.Unlock()
	}
	st := storeFor(o)
	st.mu.Lock()
	st.watchers = map[uint64]func(){}
	st.mu.Unlock()
}
func notifyCleanup(o any) {
	st := storeFor(o)
	st.mu.Lock()
	fs := make([]func(), 0, len(st.cleanupWatchers))
	for _, f := range st.cleanupWatchers {
		fs = append(fs, f)
	}
	st.mu.Unlock()
	for _, f := range fs {
		isolated(f)
	}
}

func (c lifecycleControl) AwaitJoinMilestone(ctx context.Context, session openrtc.RTCConferenceSession) error {
	select {
	case <-session.(*fakeSession).milestoneReached:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
