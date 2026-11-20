package core

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func mediaFixture(c *harness.TestContext) harness.MediaContractControl {
	t, ok := c.Target.(harness.MediaContractTarget)
	if !ok {
		must(c.NotRun("fixture provides no MediaContractTarget"))
	}
	f := t.MediaContracts()
	if f == nil {
		must(c.NotRun("fixture provides no media contract control"))
	}
	return f
}
func mediaSession(ctx context.Context, c *harness.TestContext) (openrtc.RTCConferenceSession, harness.MediaContractControl) {
	f := mediaFixture(c)
	s := joined(ctx, c).Session
	must(f.Prepare(ctx, s))
	return s, f
}
func mediaKinds(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, operation string) []openrtc.MediaKind {
	f := mediaFixture(c)
	k, ok := f.(harness.MediaKindControl)
	if !ok {
		must(c.NotRun("missing exact session operation/kind applicability mapping: " + operation))
	}
	kinds := value(k.Kinds(ctx, s, operation))
	if len(kinds) == 0 {
		must(c.NotRun("no established supported kind for " + operation + "; optional positives require scoped capability evidence"))
	}
	return kinds
}
func mediaCaptureKind(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, ownership string, kind openrtc.MediaKind) openrtc.CaptureSource {
	op := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{kind}, Ownership: ownership})
	src := value(op.Result(ctx))
	c.RecordCoverage(harness.OperationClaim{Operation: "capture", Kinds: []string{string(kind)}, Scope: ownership, Support: "supported"})
	c.Track("capture", func(ctx context.Context) error {
		_, e := src.Stop(ctx, openrtc.OperationOptions{}).Result(ctx)
		return e
	})
	return src
}
func mediaCapture(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, ownership string) openrtc.CaptureSource {
	kinds := mediaKinds(ctx, c, s, "capture")
	return mediaCaptureKind(ctx, c, s, ownership, kinds[0])
}
func mediaPublications(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, src openrtc.CaptureSource) []openrtc.TrackPublication {
	result := value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Publish(ctx, src.Snapshot().ID, openrtc.PublishOptions{Kinds: src.Snapshot().Kinds, Options: &openrtc.OperationOptions{}}).Result(ctx))
	for _, kind := range src.Snapshot().Kinds {
		c.RecordCoverage(harness.OperationClaim{Operation: "publish", Kinds: []string{string(kind)}, Scope: "session", Support: "supported"})
	}
	return result
}

func mediaReport(c *harness.TestContext, s openrtc.RTCConferenceSession, id openrtc.OperationID) openrtc.MediaOperationEffects {
	r := value(s.Inspection().Operation(id))
	check(c, r.Status == "retained" && r.Details != nil && r.Details.OperationID == id, "missing correlated media inspection")
	check(c, r.Details.Effects.Status == "known" && r.Details.Effects.Value != nil, "missing committed media effects")
	return *r.Details.Effects.Value
}
func mediaString(v openrtc.Known[string]) string {
	if v.Value != nil && v.Status == "known" {
		return *v.Value
	}
	return "unknown"
}
func mediaBool(v openrtc.Known[bool]) bool { return v.Status == "known" && v.Value != nil && *v.Value }
func mediaFalse(v openrtc.Known[bool]) bool {
	return v.Status == "known" && v.Value != nil && !*v.Value
}
func mediaCase(id, title, contract string, requirements []string, f func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	// Current assertion revision: historical requirement aliases are not evidence
	// for the amended postconditions. RFC §7.4:245–385, §14.2:1102–1115.
	requirements = append(requirements, "RFC §7.4:245–385", "RFC §14.2:1102–1115")
	needs := []harness.FixtureNeed{"media-contracts"}
	if id == "CT-MADV-001" {
		needs = append(needs, "boundary-loss")
	}
	if len(id) >= 7 && id[:7] == "CT-MADV" && id != "CT-MADV-001" {
		needs = []harness.FixtureNeed{"leakage", "native-fault"}
	}
	return harness.ConformanceTest{OptionalOperation: mediaSubjectOperation(id), ID: id, Title: title, Suite: harness.Core, Profile: harness.Conference, Requirements: requirements, Needs: needs, Observable: contract, Run: run(f)}
}

// Each case maps to an operation contract or the conformance scenario named in
// Observable. The optional fixture establishes conditions; all assertions read
// public handles, snapshots, inspection or native-effect instrumentation.
var MediaContractTests = []harness.ConformanceTest{
	mediaCase("CT-MOP-001", "capture commits owned production without publishing or admission", "operations.md Media capture", []string{"MED-104", "MED-117", "MED-202", "IDL-118", "IDL-150", "STM-112", "STM-124"}, func(ctx context.Context, c *harness.TestContext) {
		f := mediaFixture(c)
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		must(f.Prepare(ctx, s))
		before := f.Effects()
		connection := s.Connection().Snapshot()
		for _, kind := range mediaKinds(ctx, c, s, "capture") {
			ownership := "session"
			op := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{kind}, Ownership: ownership})
			src := value(op.Result(ctx))
			c.Track("source", func(ctx context.Context) error { _, err := src.Stop(ctx).Result(ctx); return err })
			v := src.Snapshot()
			check(c, v.ID != "" && v.Ownership == ownership && v.State == "active", "capture did not commit owned production")
			check(c, s.Observation().Snapshot().Sources[v.ID].ID == v.ID, "source missing before capture settlement")
			retained(c, src.Inspection(), op.OperationID())
			if ownership == "application" {
				retained(c, cl.Inspection(), op.OperationID())
			}
		}
		check(c, len(s.Observation().Snapshot().Publications) == 0 && f.Effects().Publications == before.Publications, "capture published implicitly")
		check(c, reflect.DeepEqual(connection, s.Connection().Snapshot()), "standalone capture changed connection or admission")
	}),
	mediaCase("CT-MOP-002", "setSending changes an existing publication without acquisition", "RFC §7.4:262–264; §8.1:448–451", []string{"RFC §7.4:262–264", "RFC §8.1:448–451"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		before := f.Effects()
		for _, sending := range []bool{false, true, true} {
			out := value(s.Media().SetSending(ctx, openrtc.SendRequest{Publication: p.ID, Sending: sending, Options: &openrtc.OperationOptions{}}).Result(ctx))
			v := s.Observation().Snapshot().Publications[p.ID]
			check(c, len(out.Publications) == 1 && out.Publications[0] == p.ID && v.Source.Value != nil && *v.Source.Value == src.Snapshot().ID, "setSending replaced target or source")
			check(c, mediaFalse(v.LocalMuteIntent) && mediaBool(v.SetSendingMuteIntent) == !sending && (mediaFalse(v.SetSendingMuteIntent) == sending), "convenience intent not established")
			check(c, mediaBool(v.EffectiveSuppression.Suppressed) == !sending && mediaFalse(v.EffectiveSuppression.Suppressed) == sending, "effective union incorrect")
		}
		check(c, f.Effects().Captures == before.Captures && f.Effects().Publications == before.Publications, "setSending acquired or published")
	}),
	mediaCase("CT-MOP-003", "setSending targets P while Q remains independently controlled", "RFC §7.4:262–264; §7.1:86–88", []string{"RFC §7.4:262–264", "RFC §7.1:86–88"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		q := mediaPublications(ctx, c, s, src)[0]
		before := f.Effects()
		qBefore := s.Observation().Snapshot().Publications[q.ID]
		value(s.Media().SetSending(ctx, openrtc.SendRequest{Publication: p.ID, Sending: false, Options: &openrtc.OperationOptions{}}).Result(ctx))
		check(c, mediaBool(s.Observation().Snapshot().Publications[p.ID].SetSendingMuteIntent), "P convenience suppression missing")
		check(c, reflect.DeepEqual(qBefore, s.Observation().Snapshot().Publications[q.ID]), "setSending modified Q without prior acceptance")
		check(c, f.Effects().Captures == before.Captures && f.Effects().Publications == before.Publications, "setSending acquired resources")
	}),
	mediaCase("CT-MOP-004", "setSending retains independent mute and source disable", "RFC §7.4:262–264/310; §8.1:448–451", []string{"RFC §7.4:262–264", "RFC §8.1:448–451"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		value(s.Media().SetMuted(ctx, p.ID, true, openrtc.OperationOptions{}).Result(ctx))
		value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
		for _, sending := range []bool{false, true} {
			value(s.Media().SetSending(ctx, openrtc.SendRequest{Publication: p.ID, Sending: sending, Options: &openrtc.OperationOptions{}}).Result(ctx))
			v := s.Observation().Snapshot().Publications[p.ID]
			check(c, v.State == "published" && mediaBool(v.LocalMuteIntent) && mediaBool(v.SourceDisableIntent) && mediaBool(v.EffectiveSuppression.Suppressed), "convenience setter cleared independent suppression")
			check(c, mediaBool(v.SetSendingMuteIntent) == !sending && mediaFalse(v.SetSendingMuteIntent) == sending, "convenience intent incorrect")
		}
	}),
	mediaCase("CT-MOP-005", "unmute removes only mute intent and retains disabled production", "operations.md Media setMuted", []string{"MED-106", "MED-107", "MED-202", "STM-105"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		value(s.Media().SetMuted(ctx, p.ID, true).Result(ctx))
		value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true).Result(ctx))
		value(s.Media().SetMuted(ctx, p.ID, false).Result(ctx))
		v := s.Observation().Snapshot().Publications[p.ID]
		check(c, v.State == "published" && mediaFalse(v.LocalMuteIntent) && mediaBool(v.SourceDisableIntent) && mediaBool(v.EffectiveSuppression.Suppressed), "unmute cleared disable intent or removed publication")
		check(c, src.Snapshot().State == "disabled", "unmute restarted capture")
	}),
	mediaCase("CT-MOP-006", "reenable removes only disable intent and retains publication mute", "operations.md Media setDisabled", []string{"MED-106", "MED-107", "MED-202", "STM-112"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		value(s.Media().SetMuted(ctx, p.ID, true).Result(ctx))
		for _, v := range []bool{true, false} {
			value(s.Media().SetDisabled(ctx, src.Snapshot().ID, v).Result(ctx))
		}
		v := s.Observation().Snapshot().Publications[p.ID]
		check(c, src.Snapshot().State == "active" && mediaFalse(v.SourceDisableIntent) && mediaBool(v.LocalMuteIntent) && mediaBool(v.EffectiveSuppression.Suppressed) && v.State == "published", "reenable cleared mute or lost production eligibility")
	}),
	mediaCase("CT-MOP-007", "complete receive requests commit both selections and authority", "operations.md Media setReceivePolicy", []string{"MED-114", "MED-201", "MED-202", "STM-202", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		readiness := s.Connection().Snapshot().MediaReadiness
		for _, r := range []openrtc.ReceivePolicyRequest{{Audio: "manual", Video: "none"}, {Audio: "none", Video: "automatic"}} {
			op := s.Media().SetReceivePolicy(ctx, r)
			value(op.Result(ctx))
			m := s.Inspection().Snapshot().Discovery.Media
			check(c, m.Value != nil && m.Value.Receive.Audio.Selection == string(r.Audio) && m.Value.Receive.Video.Selection == string(r.Video), "receive request not committed as complete replacement")
			check(c, mediaString(m.Value.Receive.Audio.Authority) == "client" && mediaString(m.Value.Receive.Video.Authority) == "client", "accepted authority missing")
			retained(c, s.Inspection(), op.OperationID())
		}
		check(c, s.Connection().Snapshot().MediaReadiness == readiness, "policy invented media readiness")
	}),
	mediaCase("CT-MOP-008", "invalid and unsupported receive combinations reject without effects", "operations.md Media setReceivePolicy preconditions", []string{"MED-201", "MED-114", "CORE-200", "ERR-109"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		must(f.Condition(ctx, s, "unsupported-receive"))
		before := f.Effects()
		policy := s.Inspection().Snapshot().Discovery.Media.Value.Receive
		for _, r := range []openrtc.ReceivePolicyRequest{{Audio: "automatic"}, {Audio: "automatic", Video: "automatic"}} {
			_, err := s.Media().SetReceivePolicy(ctx, r).Result(ctx)
			code := openrtc.InvalidOption
			if r.Video != "" {
				code = openrtc.UnsupportedCapability
			}
			expect(c, err, []openrtc.ErrorCode{code}, "receive admission")
		}
		check(c, f.Effects() == before && reflect.DeepEqual(policy, s.Inspection().Snapshot().Discovery.Media.Value.Receive), "unsupported request crossed admission or changed policy")
	}),
	mediaCase("CT-MOP-009", "render commits the explicit binding without inventing playback", "operations.md Media render (blocked playback fixture)", []string{"MED-110", "MED-113", "MED-202", "IDL-150", "STM-118"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		id := value(f.RemotePublication(ctx, s))
		target := f.RenderTarget()
		op := s.Media().Render(ctx, openrtc.RenderRequest{Source: openrtc.RenderSource{Kind: "publication", Publication: id}, Target: target})
		b := value(op.Result(ctx))
		c.Track("binding", func(ctx context.Context) error { _, e := b.Detach(ctx).Result(ctx); return e })
		v := b.Snapshot()
		check(c, v.ID != "" && v.Source.Publication == id && reflect.DeepEqual(v.Target, target) && v.ControlScope == "binding", "binding lost target, identity or scope")
		check(c, s.Observation().Snapshot().Renders[v.ID].ID == v.ID, "binding not committed before result")
		check(c, v.Playback == "blocked" && v.ReceiveEffect == "none", "render invented playback or receive success")
		mediaReport(c, s, op.OperationID())
	}),
	mediaCase("CT-MOP-010", "detach fences locally and retains failed native cleanup", "operations.md Binding detach", []string{"MED-111", "MED-113", "CAP-107", "STM-118", "STM-201", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		id := value(f.RemotePublication(ctx, s))
		b := value(s.Media().Render(ctx, openrtc.RenderRequest{Source: openrtc.RenderSource{Kind: "publication", Publication: id}, Target: f.RenderTarget()}).Result(ctx))
		must(f.Condition(ctx, s, "detach-failure"))
		op := b.Detach(ctx)
		value(op.Result(ctx))
		check(c, b.Snapshot().Playback == "stopped", "detach did not fence binding")
		check(c, s.Inspection().Cleanup().Status.Kind != "complete", "detach discarded failed cleanup")
		retained(c, s.Inspection(), op.OperationID())
		before := f.Effects()
		value(b.Detach(ctx).Result(ctx))
		check(c, f.Effects() == before, "repeated detach repeated native effects")
		must(f.CompleteCleanup(ctx))
		check(c, s.Inspection().Cleanup().Status.Kind == "complete", "cleanup progress lost")
	}),
	mediaCase("CT-MOP-011", "publish returns every native publication as a distinct committed identity", "conformance.md Conference/calling/extension scenarios and operation Success row; media.md MED-161", []string{"MED-161", "MED-160", "MED-101", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "application")
		before := f.Effects()
		pubs := mediaPublications(ctx, c, s, src)
		check(c, len(pubs) > 0, "publish returned no independently controllable publications")
		ids := map[openrtc.PublicationID]bool{}
		kinds := map[openrtc.MediaKind]bool{}
		for _, p := range pubs {
			check(c, p.ID != "" && !ids[p.ID], "duplicate publication identity")
			ids[p.ID] = true
			for _, k := range p.Kinds {
				kinds[k] = true
			}
			check(c, s.Observation().Snapshot().Publications[p.ID].ID == p.ID, "publication missing at settlement")
		}
		check(c, len(kinds) > 0 && f.Effects().Publications-before.Publications == len(pubs) && f.Effects().Captures == before.Captures, "publish omitted kind or acquired capture")
	}),
	mediaCase("CT-MOP-012", "partial publish failure retains created identities and effects", "conformance.md Execution errors row; media.md MED-161 partial creation", []string{"MED-161", "MED-160", "ERR-103", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		must(f.Condition(ctx, s, "partial-publish"))
		op := openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Publish(ctx, src.Snapshot().ID, openrtc.PublishOptions{Kinds: src.Snapshot().Kinds})
		_, err := op.Result(ctx)
		e := expect(c, err, []openrtc.ErrorCode{"media-failure"}, "partial publish")
		check(c, e.Context != nil && e.Context.MediaResources != nil && len(e.Context.MediaResources.Publications) == 1 && e.Context.PartialEffects != nil, "partial failure lost created publication identities/effects")
		serialized := value(json.Marshal(e))
		var wire openrtc.SerializedRTCError
		must(json.Unmarshal(serialized, &wire))
		check(c, wire.Context != nil && wire.Context.MediaResources != nil && len(wire.Context.MediaResources.Publications) == 1, "safe serialization lost partial creation identities")
		id := e.Context.MediaResources.Publications[0]
		check(c, s.Observation().Snapshot().Publications[id].ID == id, "failure identity was never committed")
		retained(c, s.Inspection(), op.OperationID())
		value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Unpublish(ctx, id, openrtc.UnpublishOptions{Capture: "retain"}).Result(ctx))
		check(c, e.Context.MediaResources.Publications[0] == id, "compensation removed retained identity")
	}),
	mediaCase("CT-MOP-013", "unpublish retains supplied capture and release survives support loss", "conformance.md Cleanup row; media.md MED-108 independent-publication unpublish", []string{"MED-108", "MED-111", "CAP-107", "IDL-150"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "application")
		pubs := mediaPublications(ctx, c, s, src)
		must(f.Condition(ctx, s, "unsupported"))
		ctrl := openrtc.SessionExtension(s, openrtc.IndependentPublicationKey)
		for _, p := range pubs {
			op := ctrl.Unpublish(ctx, p.ID, openrtc.UnpublishOptions{Capture: "retain"})
			value(op.Result(ctx))
			e := mediaReport(c, s, op.OperationID())
			check(c, mediaString(e.Guaranteed.Publication) == "removed" && src.Snapshot().State == "active", "unpublish retain stopped capture or failed to remove publication")
			check(c, s.Observation().Snapshot().Publications[p.ID].State == "unpublished", "publication still active")
		}
		before := f.Effects()
		value(ctrl.Unpublish(ctx, pubs[0].ID, openrtc.UnpublishOptions{Capture: "retain"}).Result(ctx))
		check(c, f.Effects() == before, "repeated unpublish acquired resources")
	}),
	mediaCase("CT-MOP-014", "operation inspection correlates stable handles and detached effects", "operations.md Inspection operation", []string{"IDL-100", "IDL-150", "IDL-200", "CORE-109"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		op := s.Media().SetMuted(ctx, p.ID, true)
		id := op.OperationID()
		check(c, id != "" && id == op.OperationID(), "operation identity unstable")
		value(op.Result(ctx))
		mediaReport(c, s, id)
		r := value(s.Inspection().Operation(id))
		r.Details.Effects.Value.Collateral = append(r.Details.Effects.Value.Collateral, "capture")
		again := value(s.Inspection().Operation(id))
		check(c, !slices.Contains(again.Details.Effects.Value.Collateral, "capture"), "inspection aliases retained effects")
		before := harness.SafeJSON(again)
		value(op.Result(ctx))
		check(c, harness.SafeJSON(value(s.Inspection().Operation(id))) == before, "repeat result changed operation inspection")
	}),
	mediaCase("CT-MOP-015", "evicted operation remains distinguishable from never issued", "operations.md Inspection operation evicted classification", []string{"IDL-150", "CORE-202"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		old := s.Media().SetMuted(ctx, p.ID, true)
		value(old.Result(ctx))
		value(s.Media().SetMuted(ctx, p.ID, false).Result(ctx))
		must(f.EvictSuperseded(s, old.OperationID()))
		r := value(s.Inspection().Operation(old.OperationID()))
		check(c, r.Status == "evicted" && r.OperationID == old.OperationID() && r.Details == nil, "issued eviction not explicitly classified")
		_, err := s.Inspection().Operation("conformance-never-issued")
		expect(c, err, []openrtc.ErrorCode{"invalid-state"}, "never issued")
	}),
	mediaCase("CT-MOP-016", "operation remains retained while any resource facet key is latest", "operations.md media Inspection keys; Inspection operation IDL-150.5", []string{"IDL-150", "IDL-200", "MED-202"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		pubs := mediaPublications(ctx, c, s, src)
		pubs = append(pubs, mediaPublications(ctx, c, s, src)...)
		first := s.Media().SetMuted(ctx, pubs[0].ID, true)
		value(first.Result(ctx))
		second := s.Media().SetMuted(ctx, pubs[1].ID, true)
		value(second.Result(ctx))
		retained(c, s.Inspection(), first.OperationID())
		value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true).Result(ctx))
		retained(c, s.Inspection(), first.OperationID())
		retained(c, s.Inspection(), second.OperationID())
		value(s.Dispose(ctx).Result(ctx))
		retained(c, s.Inspection(), first.OperationID())
	}),
	mediaCase("CT-MOP-017", "last-known suppression cannot satisfy no-op admission", "operations.md admission order and Media setMuted; CORE-124.5", []string{"CORE-124", "MED-106", "MED-130", "IDL-133"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		value(s.Media().SetMuted(ctx, p.ID, true).Result(ctx))
		must(f.Condition(ctx, s, "stale-mute"))
		v := s.Observation().Snapshot().Publications[p.ID]
		check(c, v.EffectiveSuppression.Freshness.Status == "last-known", "fixture did not establish stale suppression")
		_, err := s.Media().SetMuted(ctx, p.ID, true, openrtc.OperationOptions{}).Result(ctx)
		if err != nil {
			expect(c, err, nil, "stale suppression must reject safely or reestablish")
		} else {
			current := s.Observation().Snapshot().Publications[p.ID]
			check(c, current.EffectiveSuppression.Freshness.Status == "current" && mediaBool(current.EffectiveSuppression.Suppressed), "successful repeated mute relied on stale suppression")
		}

	}),
	mediaCase("CT-MOP-018", "remote unpublish during render cannot resurrect a binding", "conformance.md participant/track lifecycle races; operations.md Media render", []string{"STM-105", "STM-108", "STM-121", "CORE-109", "ADP-113", "MED-110"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		id := value(f.RemotePublication(ctx, s))
		gate := f.GateRender(s)
		defer gate.ReleaseSuccess()
		op := s.Media().Render(ctx, openrtc.RenderRequest{Source: openrtc.RenderSource{Kind: "publication", Publication: id}, Target: f.RenderTarget()})
		select {
		case <-gate.Reached():
		case <-ctx.Done():
			must(ctx.Err())
		}
		retained(c, s.Inspection(), op.OperationID())
		must(f.UnpublishRemote(ctx, s, id))
		check(c, s.Observation().Snapshot().Publications[id].State == "unpublished", "remote terminal fact not committed")
		gate.ReleaseSuccess()
		b, err := op.Result(ctx)
		if err == nil {
			check(c, b != nil && b.Snapshot().Playback == "stopped", "late render resurrected removed remote publication")
		} else {
			expect(c, err, []openrtc.ErrorCode{"invalid-state", "cancelled"}, "render remote terminal race")
		}
		for _, v := range s.Observation().Snapshot().Renders {
			check(c, v.Source.Publication != id || v.Playback == "stopped", "active binding after remote unpublish")
		}
	}),
	mediaCase("CT-MOP-019", "unsupported capture rejects with correct code and no native effects", "conformance.md Unsupported capabilities; operations.md Media capture A", []string{"CAP-003", "CAP-115", "CAP-118", "ADP-117", "ERR-109"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		for _, condition := range []string{"unsupported", "unknown-with-channel", "denied-with-pending"} {
			must(f.Condition(ctx, s, condition))
			found := false
			for _, d := range s.Capabilities().Snapshot().Entries {
				if d.ID == "media.capture" {
					found = true
					if condition == "unsupported" {
						check(c, d.Support.Level == "unsupported", "fixture did not advertise unsupported capture")
					} else {
						check(c, d.Support.Level == "unknown" && len(d.Prerequisites) > 0 && d.Prerequisites[0].ResolutionChannel != "", "fixture lacks nominal resolution channel")
						if condition == "denied-with-pending" {
							check(c, d.Authorization.Decision == "denied" && d.Prerequisites[0].Status == "pending", "fixture lacks established denial alongside pending evaluation")
						}
					}
				}
			}
			check(c, found, "capture descriptor missing")
			before := f.Effects()
			sources := s.Observation().Snapshot().Sources
			op := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"audio"}, Ownership: "session"})
			_, err := op.Result(ctx)
			code := openrtc.UnsupportedCapability
			if condition == "denied-with-pending" {
				code = openrtc.AuthorizationDenied
			}
			expect(c, err, []openrtc.ErrorCode{code}, "unsupported or unknown capture, denial takes precedence")
			check(c, f.Effects() == before && reflect.DeepEqual(sources, s.Observation().Snapshot().Sources), "unsupported operation reached native layer")
			retained(c, s.Inspection(), op.OperationID())
		}
	}),
	mediaCase("CT-MOP-020", "setSending rejects absent foreign and ended publication targets before effects", "RFC §7.4:262–264; §7.1:86–88", []string{"RFC §7.4:262–264", "RFC §7.1:86–88"}, func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		p := mediaPublications(ctx, c, s, src)[0]
		before := f.Effects()
		for _, target := range []openrtc.PublicationID{"", "foreign-publication"} {
			_, e := s.Media().SetSending(ctx, openrtc.SendRequest{Publication: target, Sending: true}).Result(ctx)
			expect(c, e, []openrtc.ErrorCode{openrtc.InvalidArgument, openrtc.InvalidState}, "invalid target")
		}
		suppressed := "suppressed"
		_, e := s.Media().SetSending(ctx, openrtc.SendRequest{Publication: p.ID, Sending: true, RequiredEffects: &openrtc.MediaEffectRequirements{Outbound: &suppressed}}).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.Unsupported}, "incompatible requirement")
		check(c, f.Effects() == before, "rejected request crossed effects")
		value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Unpublish(ctx, p.ID, openrtc.UnpublishOptions{}).Result(ctx))
		_, e = s.Media().SetSending(ctx, openrtc.SendRequest{Publication: p.ID, Sending: true}).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.InvalidState}, "ended target")
	}),
}

// The optional positive subject is separate from its setup prerequisites. Negative
// entry/security/cleanup scenarios retain their independent applicability.
func mediaSubjectOperation(id string) string {
	switch id {
	case "CT-MOP-001", "CT-SHAPE-002", "CT-MEDIA-POS-001", "CT-MEDIA-POS-008":
		return "capture"
	case "CT-MOP-002", "CT-MOP-003", "CT-MOP-004", "CT-MOP-020":
		return "setSending"
	case "CT-MOP-005", "CT-MOP-017", "CT-MEDIA-POS-003", "CT-EFFECT-001", "CT-EFFECT-002", "CT-EFFECT-003", "CT-EFFECT-004":
		return "setMuted"
	case "CT-MOP-006", "CT-MEDIA-POS-004", "CT-MEDIA-POS-005":
		return "setDisabled"
	case "CT-MOP-011", "CT-MOP-012", "CT-MEDIA-POS-002":
		return "publish"
	case "CT-MOP-013", "CT-MEDIA-POS-006":
		return "unpublish"
	case "CT-MEDIA-POS-007", "CT-EFFECT-005", "CT-EFFECT-006":
		return "stop"
	case "CT-MOP-009", "CT-MOP-010", "CT-MOP-018":
		return "attachRender"
	}
	return ""
}
