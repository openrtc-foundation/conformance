package selftest

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
	"sync"
)

func newFakeSession(client *fakeClient, target openrtc.ConferenceTarget, w *world, defects FakeDefects) *fakeSession {
	s := &fakeSession{milestoneReached: make(chan struct{}), admissionReady: make(chan struct{}), instanceID: openrtc.SessionInstanceID(newID("instance")), client: client, target: target, world: w, defects: defects,
		id: openrtc.LocalSessionID(newID("session")), generation: openrtc.GenerationID(newID("gen")),
		recovery: openrtc.Unknown[openrtc.RecoveryStatus](), state: openrtc.ConnectionState("idle"), admission: "unknown", cleanup: openrtc.CleanupStatus{Kind: "complete"},
		credentials: initialCredentials(), roster: []openrtc.RTCParticipant{}, capabilities: newFakeCapabilities(defects)}
	s.observation = newFakeObservation(s.buildSnapshot(), defects)
	s.observation.clock = w.clock
	s.media = &fakeMedia{session: s, publications: map[openrtc.PublicationID]openrtc.TrackPublication{}, subscriptions: map[openrtc.SubscriptionID]openrtc.TrackSubscription{}}
	if defects.OpenSessionRequestsCredentials {
		request := s.credentialRequest("admission", "initial")
		go func() {
			_, _ = client.credentialProvider.Get(context.Background(), request).Result(context.Background())
		}()
	}
	return s
}
func (s *fakeSession) Kind() string                            { return "conference" }
func (s *fakeSession) ID() openrtc.LocalSessionID              { return s.id }
func (s *fakeSession) Target() openrtc.ConferenceTarget        { return s.target }
func (s *fakeSession) Capabilities() openrtc.RTCCapabilities   { return s.capabilities }
func (s *fakeSession) Observation() openrtc.SessionObservation { return s.observation }
func (s *fakeSession) Media() openrtc.MediaController          { return s.media }

type fakeMedia struct {
	allowedKinds []openrtc.MediaKind
	testMutation string
	selection    map[openrtc.PublicationID]bool

	canaries     []string
	nativeSecret any
	boundaryLost bool

	mu        sync.Mutex
	enabled   bool
	sources   map[openrtc.CaptureSourceID]*mediaSource
	bindings  map[openrtc.RenderBindingID]*mediaBinding
	effects   map[openrtc.PublicationID]openrtc.MediaOperationEffects
	policy    openrtc.ReceivePolicy
	condition string
	gate      *worldGate

	session       *fakeSession
	publications  map[openrtc.PublicationID]openrtc.TrackPublication
	subscriptions map[openrtc.SubscriptionID]openrtc.TrackSubscription
}

var _ openrtc.MediaController = (*fakeMedia)(nil)
var _ openrtc.RTCCapabilities = (*fakeCapabilities)(nil)

func (m *fakeMedia) Model() openrtc.MediaControlModel {
	m.mu.Lock()
	defer m.mu.Unlock()
	receive := openrtc.ReceiveKindPolicy{Selection: "not-applicable", Authority: openrtc.KnownValue("client"), Adaptation: openrtc.Unknown[bool](), Controls: []string{}}
	policy := openrtc.ReceivePolicy{Audio: receive, Video: receive}
	publication := "not-applicable"
	if m.enabled {
		policy = m.policy
		publication = "independent"
	}
	return openrtc.MediaControlModel{Publication: publication, Receive: policy, RawTrack: openrtc.KnownValue(false), Effects: unknownEffects(), ControlUnitCoverage: "complete", SetupMedia: openrtc.NotApplicable[string]("no setup media")}
}
func (m *fakeMedia) ShareSource(openrtc.CaptureSourceID, openrtc.SourceSharePermissions) {
	panic(fail("invalid-state", "share-source", "no shared source", failureExtras{}))
}
func (m *fakeMedia) Source(id openrtc.CaptureSourceID) openrtc.CaptureSource {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.sources[id]; s != nil {
		return s
	}
	panic(fail("invalid-state", "source", "unknown source", failureExtras{}))
}
func (m *fakeMedia) Track(openrtc.TrackID) openrtc.LocalTrack {
	panic(fail("invalid-state", "track", "no raw tracks", failureExtras{}))
}
