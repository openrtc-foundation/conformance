package selftest

import (
	"context"
	"slices"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type mediaControl struct{ target *FakeTarget }

func (t *FakeTarget) MediaContracts() harness.MediaContractControl { return mediaControl{t} }
func (c mediaControl) Prepare(_ context.Context, s openrtc.RTCConferenceSession) error {
	m := s.(*fakeSession).media
	m.mu.Lock()
	m.enabled = true
	m.sources = map[openrtc.CaptureSourceID]*mediaSource{}
	m.bindings = map[openrtc.RenderBindingID]*mediaBinding{}
	m.effects = map[openrtc.PublicationID]openrtc.MediaOperationEffects{}
	m.selection = map[openrtc.PublicationID]bool{}
	p := openrtc.ReceiveKindPolicy{Selection: "automatic", Authority: openrtc.KnownValue("client"), Adaptation: openrtc.Unknown[bool](), Controls: []string{"select", "deselect", "render"}}
	m.policy = openrtc.ReceivePolicy{Audio: p, Video: p}
	m.mu.Unlock()
	caps := m.session.capabilities
	caps.mu.Lock()
	caps.overrides = map[string]string{}
	for _, id := range []string{"media.capture", "media.send", "media.publish.independent", "media.mute", "media.disable", "media.receive-policy", "media.render"} {
		caps.overrides[id] = "supported"
	}
	caps.mu.Unlock()
	m.session.apply(func() {}, nil)
	return nil
}
func (mediaControl) RenderTarget() openrtc.RenderTarget {
	return openrtc.RenderTarget{Kind: "provider-managed", Target: &openrtc.PlaybackControlTarget{Kind: "session"}}
}
func (c mediaControl) Condition(_ context.Context, s openrtc.RTCConferenceSession, condition string) error {
	m := s.(*fakeSession).media
	m.mu.Lock()
	m.condition = condition
	if condition == "stale-mute" {
		for id, p := range m.publications {
			p.EffectiveSuppression.Freshness.Status = "last-known"
			m.publications[id] = p
		}
	}
	m.mu.Unlock()
	if condition == "unsupported" || condition == "unknown-with-channel" || condition == "denied-with-pending" {
		caps := m.session.capabilities
		caps.mu.Lock()
		level := "unknown"
		if condition == "unsupported" {
			level = "unsupported"
		}
		caps.overrides["media.capture"] = level
		caps.captureCondition = condition
		caps.revision += 2
		caps.mu.Unlock()
	}
	m.session.apply(func() {}, nil)
	return nil
}
func (c mediaControl) CompleteCleanup(ctx context.Context) error {
	return lifecycleControl{c.target}.CompleteCleanup(ctx)
}
func (c mediaControl) Effects() harness.MediaFixtureEffects {
	c.target.world.mu.Lock()
	defer c.target.world.mu.Unlock()
	return c.target.world.mediaEffects
}
func (mediaControl) GateRender(s openrtc.RTCConferenceSession) harness.JoinGate {
	g := &worldGate{reachedCh: make(chan struct{}), outcome: harness.NewDeferred[any]()}
	m := s.(*fakeSession).media
	m.mu.Lock()
	m.gate = g
	m.mu.Unlock()
	return joinGateControl{g}
}
func (mediaControl) RemotePublication(_ context.Context, s openrtc.RTCConferenceSession) (openrtc.PublicationID, error) {
	m := s.(*fakeSession).media
	m.mu.Lock()
	id := openrtc.PublicationID(newID("remote-publication"))
	m.publications[id] = openrtc.TrackPublication{ID: id, Generation: s.Generation(), State: "published", Kinds: []openrtc.MediaKind{"audio"}, Source: openrtc.NotApplicable[openrtc.CaptureSourceID]("remote"), Origin: "observed", ControlUnit: "track", NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), Participant: openrtc.Unknown[openrtc.ParticipantConnectionID](), LocalMuteIntent: openrtc.NotApplicable[bool]("remote"), SourceDisableIntent: openrtc.Unknown[bool](), EffectiveSuppression: m.suppression(false)}
	m.mu.Unlock()
	m.session.apply(func() {}, nil)
	return id, nil
}
func (mediaControl) UnpublishRemote(_ context.Context, s openrtc.RTCConferenceSession, id openrtc.PublicationID) error {
	m := s.(*fakeSession).media
	m.mu.Lock()
	p := m.publications[id]
	p.State = "unpublished"
	m.publications[id] = p
	for _, b := range m.bindings {
		if b.view.Source.Publication == id {
			b.view.Playback = "stopped"
		}
	}
	m.mu.Unlock()
	m.session.apply(func() {}, []sessionMutationEvent{{key: "publicationChanged", payload: func(_, n openrtc.SessionSnapshot) any { return n.Publications[id] }}})
	return nil
}
func (mediaControl) EvictSuperseded(s openrtc.RTCConferenceSession, id openrtc.OperationID) error {
	st := storeFor(s)
	st.mu.Lock()
	defer st.mu.Unlock()
	delete(st.records, id)
	st.evicted[id] = true
	return nil
}
func (m *fakeMedia) count(f func(*harness.MediaFixtureEffects)) {
	m.session.world.mu.Lock()
	f(&m.session.world.mediaEffects)
	m.session.world.mu.Unlock()
}
func (m *fakeMedia) suppression(v bool) openrtc.EffectiveSuppression {
	return openrtc.EffectiveSuppression{Suppressed: openrtc.KnownValue(v), Reasons: openrtc.KnownValue([]string{}), Freshness: openrtc.SourceFreshness{Owner: openrtc.LocalOwnerID(m.session.id), Source: "adapter:media", Status: "current", ObservedAt: openrtc.KnownValue(m.session.world.clock()), Coverage: []openrtc.FreshnessCoverage{{Kind: "facet", Facet: "publications"}}}, Evidence: []openrtc.EvidenceReference{}}
}
func mediaEffects(outbound string) openrtc.MediaOperationEffects {
	e := unknownEffects()
	e.Guaranteed.Outbound = openrtc.KnownValue(outbound)
	e.Observed.Outbound = openrtc.KnownValue(outbound)
	return e
}
func knownBool(v openrtc.Known[bool]) bool { return v.Status == "known" && v.Value != nil && *v.Value }
func (m *fakeMedia) refresh() {
	for id, p := range m.publications {
		if p.Source.Value == nil {
			continue
		}
		src := m.sources[*p.Source.Value]
		if src == nil {
			continue
		}
		p.SourceDisableIntent = src.view.SourceDisableIntent
		p.SetSendingMuteIntent = openrtc.KnownValue(!m.selection[id])
		p.EffectiveSuppression = m.suppression(!m.selection[id] || knownBool(p.LocalMuteIntent) || knownBool(p.SourceDisableIntent))
		m.publications[id] = p
	}
}

type mediaObserver struct {
	projection *fakeSession

	operationObserver
	effects func() openrtc.MediaOperationEffects
}

func (o mediaObserver) OperationSettled(id openrtc.OperationID, v any, err error) {
	if e, ok := err.(*openrtc.RTCOperationError); ok {
		if e.Context == nil {
			e.Context = &openrtc.SafeErrorContext{}
		}
		e.Context.OperationID = id
	}
	effects := o.effects()
	o.operationObserver.settle(id, v, err, &effects)
	if o.projection != nil && operationSession(o.owner) == nil {
		o.projection.apply(func() {}, nil)
	}
}
func mediaOperation[T any](owner any, e func() openrtc.MediaOperationEffects, work func() (T, error), projection ...*fakeSession) openrtc.Async[T] {
	var session *fakeSession
	if len(projection) > 0 {
		session = projection[0]
	}
	return openrtc.StartOperation(mediaObserver{operationObserver: operationObserver{owner}, effects: e, projection: session}, work)
}
func (m *fakeMedia) Capture(_ context.Context, r openrtc.CaptureRequest) openrtc.Async[openrtc.CaptureSource] {
	var owner any = m
	if r.Ownership == "application" {
		owner = m.session.client
	}
	return mediaOperation(owner, func() openrtc.MediaOperationEffects {
		e := mediaEffects("unchanged")
		e.Guaranteed.Capture = openrtc.KnownValue("acquired")
		return e
	}, func() (openrtc.CaptureSource, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.allowedKinds != nil {
			for _, kind := range r.Kinds {
				if !slices.Contains(m.allowedKinds, kind) {
					return nil, fail(openrtc.Unsupported, "capture", "kind not supported", failureExtras{})
				}
			}
		}
		if len(r.Kinds) == 0 || !slices.Contains([]string{"session", "application"}, r.Ownership) {
			return nil, fail("invalid-option", "capture", "invalid capture request", failureExtras{})
		}
		if m.nativeSecret != nil {
			log := "media operation failed: provider-failure (native detail redacted)"
			if m.session.defects.LeakLogCanary {
				log = m.canaries[0]
			}
			m.session.world.mu.Lock()
			m.session.world.logs = append(m.session.world.logs, log)
			m.session.world.mu.Unlock()
			msg := "native media operation failed"
			if m.session.defects.LeakErrorCanary {
				msg = m.canaries[0]
			}
			return nil, fail("provider-failure", "capture", msg, failureExtras{original: m.nativeSecret, retryability: "unknown"})
		}
		if !m.enabled || slices.Contains([]string{"unsupported", "unknown-with-channel", "denied-with-pending"}, m.condition) {
			if m.session.defects.UnsupportedMediaEffects {
				m.count(func(e *harness.MediaFixtureEffects) { e.Captures++ })
			}
			if m.condition == "denied-with-pending" {
				return nil, fail("authorization-denied", "capture", "established denial precedes pending service evaluation", failureExtras{})
			}
			return nil, m.session.unsupported("capture")
		}
		if strings.HasPrefix(m.condition, "effect-stop:") {
			target := openrtc.CaptureSourceID(strings.TrimPrefix(m.condition, "effect-stop:"))
			if !effectAccepted(r.Options, target) && m.testMutation != "release-acquisition-unaccepted" {
				return nil, fail("unsupported", "capture", "inseparable release effect needs prior bounded acceptance", failureExtras{})
			}
		}
		src := m.capture(r)
		if strings.HasPrefix(m.condition, "effect-stop:") {
			src.releaseExtra = openrtc.CaptureSourceID(strings.TrimPrefix(m.condition, "effect-stop:"))
		}
		if strings.HasPrefix(m.condition, "effect-late:") {
			src.unresolved = true
		}
		if m.session.defects.CapturePublishes {
			m.publish(src, r.Kinds)
		}
		return src, nil
	}, m.session)
}
func (m *fakeMedia) capture(r openrtc.CaptureRequest) *mediaSource {
	m.count(func(e *harness.MediaFixtureEffects) { e.Captures++ })
	owner := openrtc.ResourceRef{Kind: "local-session", ID: string(m.session.id)}
	if r.Ownership == "application" {
		owner = openrtc.ResourceRef{Kind: "client", ID: string(m.session.client.id)}
	}
	src := &mediaSource{m: m, view: openrtc.CaptureSourceView{Owner: owner, ID: openrtc.CaptureSourceID(newID("source")), Ownership: r.Ownership, Kinds: slices.Clone(r.Kinds), State: "active", Tracks: []openrtc.TrackID{}, NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), SourceDisableIntent: openrtc.KnownValue(false), EffectiveSuppression: m.suppression(false)}}
	m.sources[src.view.ID] = src
	return src
}
func (m *fakeMedia) publish(src *mediaSource, kinds []openrtc.MediaKind) []openrtc.TrackPublication {
	var list []openrtc.TrackPublication
	for _, kind := range kinds {
		m.count(func(e *harness.MediaFixtureEffects) { e.Publications++ })
		id := openrtc.PublicationID(newID("publication"))
		p := openrtc.TrackPublication{Owner: openrtc.ResourceRef{Kind: "local-session", ID: string(m.session.id)}, SetSendingMuteIntent: openrtc.KnownValue(false), ID: id, Generation: m.session.generation, State: "published", Kinds: []openrtc.MediaKind{kind}, ControlUnit: "track", Origin: "explicit", Source: openrtc.KnownValue(src.view.ID), LocalMuteIntent: openrtc.KnownValue(false), SourceDisableIntent: src.view.SourceDisableIntent, EffectiveSuppression: m.suppression(knownBool(src.view.SourceDisableIntent)), NativeID: openrtc.KnownValue(openrtc.ScopedNativeID{AdapterID: adapterID, Namespace: "media", Value: newID("native-publication"), Generation: openrtc.KnownValue(m.session.generation), Deployment: openrtc.Unknown[openrtc.DeploymentID](), Lifetime: "generation", Origin: "adapter"}), Participant: openrtc.Unknown[openrtc.ParticipantConnectionID]()}
		m.publications[id] = p
		m.selection[id] = true
		m.effects[id] = mediaEffects("sending")
		list = append(list, p)
	}
	return list
}

// SetSending changes only the convenience mute intent of an existing publication.
// RFC §7.4:262–264, §8.1:448–451; the legacy source acquisition path is not Core.
func (m *fakeMedia) SetSending(_ context.Context, r openrtc.SendRequest) openrtc.Async[openrtc.MediaResources] {
	return mediaOperation(m, func() openrtc.MediaOperationEffects { return mediaEffects("unchanged") }, func() (openrtc.MediaResources, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		out := openrtc.MediaResources{Sources: []openrtc.CaptureSourceID{}, Publications: []openrtc.PublicationID{}}
		if m.boundaryLost {
			return out, fail("unsupported", "set-sending", "required boundary unverifiable", failureExtras{})
		}
		if r.Publication == "" || r.Source != nil || r.Ownership != "" {
			return out, fail("invalid-argument", "set-sending", "existing publication target required", failureExtras{})
		}
		p, ok := m.publications[r.Publication]
		if !ok || p.State != "published" {
			return out, fail("invalid-state", "set-sending", "ineligible publication", failureExtras{})
		}
		if r.RequiredEffects != nil && r.RequiredEffects.Outbound != nil && *r.RequiredEffects.Outbound == "suppressed" && r.Sending && !m.session.defects.SendingIgnoresRequirements {
			return out, fail("unsupported", "set-sending", "incompatible required effect", failureExtras{})
		}
		if !m.enabled || m.condition == "unsupported" {
			return out, m.session.unsupported("set-sending")
		}
		if !r.Sending && m.session.defects.SendingFalseMissesExternal {
			return out, nil
		}
		if r.Sending && m.session.defects.SendingInverseClearsMute {
			p.LocalMuteIntent = openrtc.KnownValue(false)
		}
		m.selection[p.ID] = r.Sending
		p.SetSendingMuteIntent = openrtc.KnownValue(!r.Sending)
		p.EffectiveSuppression = m.suppression(!r.Sending || knownBool(p.LocalMuteIntent) || knownBool(p.SourceDisableIntent))
		if r.Sending && m.session.defects.SendingTrueSuppressed {
			p.EffectiveSuppression = m.suppression(true)
			p.SetSendingMuteIntent = openrtc.KnownValue(true)
		}
		m.publications[p.ID] = p
		out.Publications = append(out.Publications, p.ID)
		if p.Source.Value != nil {
			out.Sources = append(out.Sources, *p.Source.Value)
		}
		m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
		return out, nil
	})
}

func (m *fakeMedia) SetMuted(_ context.Context, id openrtc.PublicationID, v bool, options ...openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return mediaOperation(m, func() openrtc.MediaOperationEffects { return mediaEffects("suppressed") }, func() (openrtc.Void, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		p, ok := m.publications[id]
		if !ok || p.State != "published" {
			return openrtc.Void{}, fail("invalid-state", "set-muted", "unknown publication", failureExtras{})
		}
		if strings.HasPrefix(m.condition, "effect-mute:") || m.condition == "effect-unknown" {
			target := openrtc.CaptureSourceID(strings.TrimPrefix(m.condition, "effect-mute:"))
			var opts *openrtc.OperationOptions
			if len(options) > 0 {
				opts = &options[0]
			}
			if m.condition == "effect-unknown" || !effectAccepted(opts, target) {
				if m.testMutation != "effect-without-acceptance" && m.testMutation != "effect-wrong-acceptance" {
					return openrtc.Void{}, fail("unsupported", "set-muted", "bounded extra effect not accepted before operation", failureExtras{})
				}
			}
			if extra := m.sources[target]; extra != nil && m.testMutation != "effect-accepted-noop" {
				extra.view.State = "disabled"
				extra.view.SourceDisableIntent = openrtc.KnownValue(true)
			}
		}
		if m.condition == "stale-mute" {
			if m.session.defects.StaleMuteNoop {
				return openrtc.Void{}, nil
			}
			m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
			return openrtc.Void{}, fail("media-failure", "set-muted", "suppression could not be restored", failureExtras{})
		}
		p.LocalMuteIntent = openrtc.KnownValue(v)
		if m.testMutation == "effect-destroys-target" && strings.HasPrefix(m.condition, "effect-mute:") {
			p.State = "unpublished"
		}
		m.publications[id] = p
		if !v && m.session.defects.UnmuteClearsDisable && p.Source.Value != nil {
			m.sources[*p.Source.Value].view.SourceDisableIntent = openrtc.KnownValue(false)
		}
		m.refresh()
		m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
		return openrtc.Void{}, nil
	})
}
func (m *fakeMedia) SetDisabled(_ context.Context, id openrtc.CaptureSourceID, v bool, options ...openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return mediaOperation(m, unknownEffects, func() (openrtc.Void, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		src := m.sources[id]
		if src == nil || src.view.State == "stopped" {
			return openrtc.Void{}, fail("invalid-state", "set-disabled", "unknown or stopped source", failureExtras{})
		}
		if !v && m.condition == "reenable-fails" {
			return openrtc.Void{}, fail("media-failure", "setDisabled", "reenable failed", failureExtras{})
		}
		src.view.SourceDisableIntent = openrtc.KnownValue(v)
		src.view.State = "active"
		if v {
			src.view.State = "disabled"
		}
		src.view.EffectiveSuppression = m.suppression(v)
		if !v && m.session.defects.EnableClearsMute {
			for id, p := range m.publications {
				p.LocalMuteIntent = openrtc.KnownValue(false)
				m.publications[id] = p
			}
		}
		m.refresh()
		m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
		return openrtc.Void{}, nil
	})
}
func (m *fakeMedia) SetReceivePolicy(_ context.Context, r openrtc.ReceivePolicyRequest) openrtc.Async[openrtc.Void] {
	return mediaOperation(m, unknownEffects, func() (openrtc.Void, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		for _, v := range []openrtc.ReceiveSelection{r.Audio, r.Video} {
			if !slices.Contains([]openrtc.ReceiveSelection{"automatic", "manual", "none"}, v) {
				return openrtc.Void{}, fail("invalid-option", "set-receive-policy", "complete request required", failureExtras{})
			}
		}
		if !m.enabled {
			return openrtc.Void{}, m.session.unsupported("set-receive-policy")
		}
		if m.condition == "unsupported-receive" && r.Audio == "automatic" && r.Video == "automatic" {
			if m.session.defects.ReceiveUnsupportedEffects {
				m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
			}
			return openrtc.Void{}, m.session.unsupported("set-receive-policy")
		}
		m.policy.Audio.Selection = string(r.Audio)
		if !m.session.defects.ReceiveDropsVideo {
			m.policy.Video.Selection = string(r.Video)
		}
		m.count(func(e *harness.MediaFixtureEffects) { e.Mutations++ })
		return openrtc.Void{}, nil
	})
}

type mediaSource struct {
	releaseExtra openrtc.CaptureSourceID
	unresolved   bool
	m            *fakeMedia
	view         openrtc.CaptureSourceView
}

func (s *mediaSource) Snapshot() openrtc.CaptureSourceView {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	return detached(s.view)
}
func (s *mediaSource) Inspection() openrtc.OwnerInspection {
	if s.view.Ownership == "application" {
		return s.m.session.client.Inspection()
	}
	return fakeOwnerInspection{owner: s.m.session, caps: s.m.session.capabilities, session: s.m.session, descriptor: s.m.session.client.providerDescriptor()}
}
func (*mediaSource) Tracks() []openrtc.LocalTrack { return nil }
func (s *mediaSource) Stop(context.Context, ...openrtc.OperationOptions) openrtc.Async[openrtc.Void] {
	return mediaOperation(s.m, unknownEffects, func() (openrtc.Void, error) {
		s.m.mu.Lock()
		if s.releaseExtra != "" {
			if extra := s.m.sources[s.releaseExtra]; extra != nil {
				extra.view.State = "disabled"
				extra.view.SourceDisableIntent = openrtc.KnownValue(true)
			}
		}
		unresolved := s.unresolved && s.m.testMutation != "late-release-false-confirmation"
		s.unresolved = false
		s.view.State = "stopped"
		s.view.EffectiveSuppression = s.m.suppression(true)
		for id, p := range s.m.publications {
			if p.Source.Value != nil && *p.Source.Value == s.view.ID {
				p.State = "unpublished"
				s.m.publications[id] = p
			}
		}
		s.m.mu.Unlock()
		if unresolved {
			s.m.session.world.acquireUnreturned(s.m.session, NoDefects)
		}
		return openrtc.Void{}, nil
	})
}

type independentMedia struct{ m *fakeMedia }

func (p independentMedia) Publish(_ context.Context, id openrtc.CaptureSourceID, r openrtc.PublishOptions) openrtc.Async[[]openrtc.TrackPublication] {
	return mediaOperation(p.m, func() openrtc.MediaOperationEffects { return mediaEffects("sending") }, func() ([]openrtc.TrackPublication, error) {
		m := p.m
		m.mu.Lock()
		defer m.mu.Unlock()
		src := m.sources[id]
		if src == nil {
			return nil, fail("invalid-state", "publish", "unknown source", failureExtras{})
		}
		if len(r.Kinds) == 0 {
			return nil, fail("invalid-option", "publish", "empty kinds", failureExtras{})
		}
		for _, k := range r.Kinds {
			if !slices.Contains(src.view.Kinds, k) {
				return nil, fail("invalid-option", "publish", "kind outside source", failureExtras{})
			}
		}
		kinds := r.Kinds
		if m.condition == "partial-publish" {
			kinds = kinds[:1]
		}
		list := m.publish(src, kinds)
		if m.condition == "partial-publish" {
			e := fail("media-failure", "publish", "second native publication failed", failureExtras{}).(*openrtc.RTCOperationError)
			resources := openrtc.MediaResources{Sources: []openrtc.CaptureSourceID{id}, Publications: []openrtc.PublicationID{list[0].ID}}
			if m.session.defects.PartialPublishDropsIdentities {
				resources.Publications = nil
			}
			e.Context = &openrtc.SafeErrorContext{MediaResources: &resources, PartialEffects: ptrMediaEffects(mediaEffects("sending"))}
			return nil, e
		}
		if m.session.defects.PublishCollapsesIdentities && len(list) > 0 {
			list = list[:0]
		}
		return list, nil
	})
}
func ptrMediaEffects(e openrtc.MediaOperationEffects) *openrtc.MediaOperationEffects { return &e }
func (p independentMedia) Unpublish(_ context.Context, id openrtc.PublicationID, r openrtc.UnpublishOptions) openrtc.Async[openrtc.Void] {
	return mediaOperation(p.m, func() openrtc.MediaOperationEffects {
		e := mediaEffects("suppressed")
		e.Guaranteed.Publication = openrtc.KnownValue("removed")
		e.Guaranteed.Capture = openrtc.KnownValue("retained")
		return e
	}, func() (openrtc.Void, error) {
		m := p.m
		m.mu.Lock()
		defer m.mu.Unlock()
		v, ok := m.publications[id]
		if !ok {
			return openrtc.Void{}, fail("invalid-state", "unpublish", "unknown publication", failureExtras{})
		}
		v.State = "unpublished"
		m.publications[id] = v
		if m.session.defects.UnpublishStopsCapture && v.Source.Value != nil {
			m.sources[*v.Source.Value].view.State = "stopped"
		}
		return openrtc.Void{}, nil
	})
}

func (m *fakeMedia) Render(ctx context.Context, r openrtc.RenderRequest) openrtc.Async[openrtc.RenderBinding] {
	return mediaOperation(m, unknownEffects, func() (openrtc.RenderBinding, error) {
		m.mu.Lock()
		if !m.enabled {
			m.mu.Unlock()
			return nil, m.session.unsupported("render")
		}
		g := m.gate
		m.gate = nil
		m.mu.Unlock()
		if g != nil {
			close(g.reachedCh)
			_, _, _, err := g.outcome.Await(ctx)
			if err != nil {
				return nil, err
			}
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		if r.Source.Kind == "publication" {
			p, ok := m.publications[r.Source.Publication]
			if (!ok || p.State == "unpublished") && !m.session.defects.RenderResurrectsRemote {
				return nil, fail("invalid-state", "render", "remote publication ended", failureExtras{})
			}
		}
		playback := "blocked"
		if m.session.defects.RenderClaimsPlaying {
			playback = "playing"
		}
		b := &mediaBinding{m: m, view: openrtc.RenderBindingView{ID: openrtc.RenderBindingID(newID("render")), Source: r.Source, Target: r.Target, ControlScope: "binding", Playback: playback, PlaybackMuted: openrtc.KnownValue(false), ReceiveEffect: "none"}}
		m.bindings[b.view.ID] = b
		m.count(func(e *harness.MediaFixtureEffects) { e.Attachments++ })
		return b, nil
	})
}

type mediaBinding struct {
	m    *fakeMedia
	view openrtc.RenderBindingView
}

func (b *mediaBinding) Snapshot() openrtc.RenderBindingView {
	b.m.mu.Lock()
	defer b.m.mu.Unlock()
	return detached(b.view)
}
func (b *mediaBinding) SetPlaybackMuted(_ context.Context, v bool) openrtc.Async[openrtc.Void] {
	return mediaOperation(b.m, unknownEffects, func() (openrtc.Void, error) {
		b.m.mu.Lock()
		defer b.m.mu.Unlock()
		b.view.PlaybackMuted = openrtc.KnownValue(v)
		return openrtc.Void{}, nil
	})
}
func (b *mediaBinding) RetryPlayback(context.Context) openrtc.Async[openrtc.Void] {
	return mediaOperation(b.m, unknownEffects, func() (openrtc.Void, error) { return openrtc.Void{}, nil })
}
func (b *mediaBinding) Detach(context.Context) openrtc.Async[openrtc.Void] {
	return mediaOperation(b.m, unknownEffects, func() (openrtc.Void, error) {
		b.m.mu.Lock()
		if b.view.Playback == "stopped" {
			b.m.mu.Unlock()
			return openrtc.Void{}, nil
		}
		b.view.Playback = "stopped"
		failure := b.m.condition == "detach-failure"
		b.m.count(func(e *harness.MediaFixtureEffects) { e.Detachments++ })
		b.m.mu.Unlock()
		if failure {
			b.m.session.world.acquireUnreturned(b.m.session, NoDefects)
			if b.m.session.defects.DetachRejectsNativeFailure {
				return openrtc.Void{}, fail("cleanup-failed", "detach", "native release failed", failureExtras{})
			}
		}
		return openrtc.Void{}, nil
	})
}

// project runs under the session state lock and takes a detached media snapshot.
func (m *fakeMedia) project(s *openrtc.SessionSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.canaries) > 0 && m.session.defects.LeakSnapshotCanary {
		s.Kind = m.canaries[0]
	}
	s.SourceCoverage = &openrtc.PopulationCoverage{Scope: openrtc.CapabilityScope{Resource: "local-session", Authority: "client", Profile: "common"}, Completeness: "complete"}
	s.ReceiveSourceCoverage = &openrtc.PopulationCoverage{Scope: openrtc.CapabilityScope{Resource: "local-session", Authority: "client", Profile: "common"}, Completeness: "unknown"}
	if !m.enabled {
		return
	}
	if len(m.canaries) > 0 && m.session.defects.LeakSnapshotCanary {
		s.Kind = m.canaries[0]
	}
	st := storeFor(m.session)
	st.mu.Lock()
	if st.configuration != nil {
		s.Placement = detached(st.configuration.Placement)
	}
	st.mu.Unlock()
	for id, v := range m.sources {
		if m.testMutation != "omit-unpublished" {
			s.Sources[id] = detached(v.view)
		}
	}
	for id, v := range m.publications {
		if m.testMutation == "unknown-false" && v.LocalMuteIntent.Value != nil && !*v.LocalMuteIntent.Value {
			v.LocalMuteIntent = openrtc.Unknown[bool]()
		}
		if m.testMutation == "nil-false" && v.LocalMuteIntent.Value != nil && !*v.LocalMuteIntent.Value {
			v.LocalMuteIntent = openrtc.Known[bool]{Status: "known"}
		}
		s.Publications[id] = detached(v)
	}
	for id, v := range m.bindings {
		s.Renders[id] = detached(v.view)
	}
}

func effectAccepted(options *openrtc.OperationOptions, target openrtc.CaptureSourceID) bool {
	if options == nil {
		return false
	}
	for _, e := range options.AcceptedEffects {
		if e.Target.Resource != nil && e.Target.Resource.Kind == "capture" && e.Target.Resource.ID == string(target) && e.Action == "disable" && (e.Certainty == "possible" || e.Certainty == "guaranteed") {
			return true
		}
	}
	return false
}

func (mediaControl) Kinds(_ context.Context, s openrtc.RTCConferenceSession, operation string) ([]openrtc.MediaKind, error) {
	m := s.(*fakeSession).media
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.enabled {
		return nil, nil
	}
	return []openrtc.MediaKind{"audio", "video"}, nil
}

func (mediaControl) ObserveOutput(_ context.Context, s openrtc.RTCConferenceSession, id openrtc.CaptureSourceID, kind openrtc.MediaKind) (harness.MediaOutputEvidence, error) {
	m := s.(*fakeSession).media
	m.mu.Lock()
	defer m.mu.Unlock()
	status := "absent"
	content := ""
	for _, p := range m.publications {
		if p.State == "published" && p.Source.Value != nil && *p.Source.Value == id && !knownBool(p.EffectiveSuppression.Suppressed) {
			status = "observed"
			content = "synthetic-changing-" + string(kind)
		}
	}
	return harness.MediaOutputEvidence{Boundary: "selftest:sink", Source: string(id), Kind: string(kind), Direction: "send", Interval: "controlled-frame-1", Evidence: "harness-self-test synthetic output only", Status: status, Content: content}, nil
}
