package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type lifecycleEvents struct{ target *FakeTarget }

func (t *FakeTarget) LifecycleEvents() harness.LifecycleEventControl { return lifecycleEvents{t} }
func (c lifecycleEvents) Submit(_ context.Context, session openrtc.RTCConferenceSession, g openrtc.GenerationID, events []harness.LifecycleNativeEvent) error {
	s := session.(*fakeSession)
	fenced := terminal(s.connectionState()) || ownerDisposed(s)
	if fenced && !s.defects.StaleCallbacksCommit {
		for _, e := range events {
			if e.Kind == "allocation" {
				s.world.mu.Lock()
				key := string(s.generation) + "/" + e.NativeKey
				duplicate := s.world.allocations[key]
				s.world.allocations[key] = true
				s.world.mu.Unlock()
				if !duplicate || s.defects.DuplicateAllocation {
					s.world.acquireUnreturned(s, s.defects)
				}
			}
		}
		return nil
	}
	if g != s.generation {
		e := fail("adapter-contract-violation", "observe", "live generation mismatch", failureExtras{})
		s.apply(func() {}, []sessionMutationEvent{{key: "error", payload: func(_, _ openrtc.SessionSnapshot) any { return e }}})
		return e
	}
	s.apply(func() { s.applyNative(events) }, []sessionMutationEvent{{key: "participantChanged", payload: func(_, n openrtc.SessionSnapshot) any {
		for _, p := range n.Roster.Entries {
			return openrtc.EntityChange[openrtc.RTCParticipant]{Kind: "upsert", Current: &p}
		}
		return openrtc.EntityChange[openrtc.RTCParticipant]{Kind: "remove", Reason: "native departure"}
	}}})
	return nil
}
func (s *fakeSession) applyNative(events []harness.LifecycleNativeEvent) {
	if s.nativeParticipants == nil {
		s.nativeParticipants = map[string]openrtc.RTCParticipant{}
		s.nativeCurrent = map[string]string{}
	}
	for _, e := range events {
		switch e.Kind {
		case "connected":
			s.state = "connected"
		case "media":
			// A defective stale media-ready callback revives the old connection.
			s.state = "connected"
		case "upsert":
			key := e.NativeKey
			if s.defects.ReuseNativeIdentity {
				key = e.NativeUID
			}
			p, ok := s.nativeParticipants[key]
			if !ok {
				p = s.participantFor(&remote{identity: openrtc.ApplicationIdentity{Namespace: "fixture", Subject: e.NativeUID}})
			}
			p.DisplayName = openrtc.KnownValue(e.DisplayName)
			p.NativeID = openrtc.KnownValue(openrtc.ScopedNativeID{AdapterID: adapterID, Namespace: "fixture", Value: e.NativeUID, Generation: openrtc.KnownValue(s.generation), Deployment: openrtc.Unknown[openrtc.DeploymentID](), Lifetime: "generation", Origin: "adapter"})
			s.nativeParticipants[key] = p
			s.nativeCurrent[e.NativeUID] = key
		case "remove":
			if s.nativeCurrent[e.NativeUID] == e.NativeKey || s.defects.OldRemovalDeletesNew {
				delete(s.nativeCurrent, e.NativeUID)
			}
		}
	}
	s.roster = nil
	for _, key := range s.nativeCurrent {
		s.roster = append(s.roster, s.nativeParticipants[key])
	}
}
func (c lifecycleEvents) Reconcile(_ context.Context, session openrtc.RTCConferenceSession, events []harness.LifecycleNativeEvent) error {
	s := session.(*fakeSession)
	if s.defects.ReconcileIgnoresCurrent {
		return nil
	}
	s.apply(func() { s.nativeCurrent = map[string]string{}; s.applyNative(events) }, nil)
	return nil
}
