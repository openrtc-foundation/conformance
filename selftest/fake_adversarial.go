package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type boundaryControl struct{ target *FakeTarget }

func (t *FakeTarget) BoundaryLoss() harness.BoundaryLossControl { return boundaryControl{t} }
func (c boundaryControl) Establish(_ context.Context, session openrtc.RTCConferenceSession) error {
	s := session.(*fakeSession)
	placement := []openrtc.PlacementObservation{{Plane: openrtc.PlacementMedia, Selection: "selected", Enforcement: "enforced", Requested: openrtc.PlacementConstraint{Strength: "required", Regions: []openrtc.RegionRef{{Namespace: "fixture", Code: "inside"}}}, Effective: openrtc.KnownValue([]openrtc.RegionRef{{Namespace: "fixture", Code: "inside"}}), Observed: openrtc.Unknown[[]openrtc.RegionRef](), Reason: "fixture service policy requires this boundary"}}
	st := storeFor(s)
	st.mu.Lock()
	v := emptyConfigurationResult(placement)
	st.configuration = &v
	st.mu.Unlock()
	s.apply(func() {}, nil)
	return nil
}
func (c boundaryControl) Withdraw(_ context.Context, session openrtc.RTCConferenceSession) error {
	s := session.(*fakeSession)
	m := s.media
	m.mu.Lock()
	m.boundaryLost = true
	if !s.defects.BoundaryFallback {
		for id, p := range m.publications {
			p.State = "interrupted"
			p.EffectiveSuppression = m.suppression(true)
			m.publications[id] = p
		}
	} else {
		s.world.mu.Lock()
		s.world.fallbackEffects++
		s.world.mu.Unlock()
	}
	m.mu.Unlock()
	caps := s.capabilities
	caps.mu.Lock()
	caps.revision += 2
	caps.overrides["media.send"] = "unknown"
	caps.mu.Unlock()
	st := storeFor(s)
	st.mu.Lock()
	if st.configuration != nil {
		st.configuration.Placement[0].Enforcement = "unknown"
		st.configuration.Placement[0].Effective = openrtc.Unknown[[]openrtc.RegionRef]()
		st.configuration.Placement[0].Reason = "required boundary is no longer verifiable"
	}
	st.mu.Unlock()
	e := fail("placement-unsatisfied", "placement", "required media boundary lost", failureExtras{kind: "observation"})
	s.apply(func() {}, []sessionMutationEvent{{key: "error", payload: func(_, _ openrtc.SessionSnapshot) any { return e }}})
	return nil
}
func (c boundaryControl) FallbackEffects() int {
	c.target.world.mu.Lock()
	defer c.target.world.mu.Unlock()
	return c.target.world.fallbackEffects
}

type leakageControl struct{ target *FakeTarget }

func (t *FakeTarget) Leakage() harness.LeakageControl { return leakageControl{t} }
func (c leakageControl) Inject(_ context.Context, session openrtc.RTCConferenceSession, canaries []string, original any) error {
	m := session.(*fakeSession).media
	m.mu.Lock()
	m.canaries = append([]string(nil), canaries...)
	m.nativeSecret = original
	m.mu.Unlock()
	c.target.world.mu.Lock()
	c.target.world.canaries = append([]string(nil), canaries...)
	c.target.world.canarySubject = m.session.client.identity
	c.target.world.mu.Unlock()
	m.session.apply(func() {}, nil)
	return nil
}
func (c leakageControl) DefaultLogs(context.Context) ([]string, error) {
	c.target.world.mu.Lock()
	defer c.target.world.mu.Unlock()
	logs := append([]string(nil), c.target.world.logs...)
	return logs, nil
}
func (leakageControl) DiagnosticKey() openrtc.ExtensionKey { return "adapter:harness-fake/diagnostics" }
