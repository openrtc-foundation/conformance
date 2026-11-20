package core

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"reflect"
)

func acceptedDisable(id openrtc.CaptureSourceID) openrtc.OperationOptions {
	r := openrtc.ResourceRef{Kind: "capture", ID: string(id)}
	return openrtc.OperationOptions{AcceptedEffects: []openrtc.EffectAcceptance{{Target: openrtc.EffectTarget{Resource: &r}, Action: "disable", Certainty: "possible"}}}
}
func effectCase(id, title string, f func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	return mediaCase(id, title, "RFC §7.1:86–88; §7.4:310/322–323; §14.3:1129", []string{"RFC §7.1:86–88", "RFC §7.4:310–323", "RFC §14.3:1129"}, f)
}
func effectSetup(ctx context.Context, c *harness.TestContext) (openrtc.RTCConferenceSession, harness.MediaContractControl, openrtc.CaptureSource, openrtc.CaptureSource, openrtc.TrackPublication) {
	s, f := mediaSession(ctx, c)
	a := mediaCapture(ctx, c, s, "session")
	b := mediaCapture(ctx, c, s, "session")
	p := mediaPublications(ctx, c, s, a)[0]
	must(f.Condition(ctx, s, "effect-mute:"+string(b.Snapshot().ID)))
	return s, f, a, b, p
}

var EffectTests = []harness.ConformanceTest{
	effectCase("CT-EFFECT-001", "unaccepted bounded possible effect rejects before effects", func(ctx context.Context, c *harness.TestContext) {
		s, f, a, b, p := effectSetup(ctx, c)
		before := f.Effects()
		a0, b0 := a.Snapshot(), b.Snapshot()
		_, e := s.Media().SetMuted(ctx, p.ID, true, openrtc.OperationOptions{}).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.Unsupported}, "unaccepted effect")
		check(c, f.Effects() == before && reflect.DeepEqual(a0, a.Snapshot()) && reflect.DeepEqual(b0, b.Snapshot()), "unaccepted effect crossed boundary")
	}),
	effectCase("CT-EFFECT-002", "exact prior bounded acceptance permits otherwise valid mute", func(ctx context.Context, c *harness.TestContext) {
		s, _, a, b, p := effectSetup(ctx, c)
		value(s.Media().SetMuted(ctx, p.ID, true, acceptedDisable(b.Snapshot().ID)).Result(ctx))
		check(c, a.Snapshot().State != "stopped" && s.Observation().Snapshot().Publications[p.ID].State == "published", "accepted extra effect waived target retention")
		check(c, b.Snapshot().State == "disabled", "accepted controlled extra effect not observed")
	}),
	effectCase("CT-EFFECT-003", "wrong action resource or unidentified isolation never grants blanket consent", func(ctx context.Context, c *harness.TestContext) {
		s, f, a, b, p := effectSetup(ctx, c)
		before := f.Effects()
		wrong := acceptedDisable(a.Snapshot().ID)
		action := acceptedDisable(b.Snapshot().ID)
		action.AcceptedEffects[0].Action = "stop"
		unidentified := acceptedDisable(b.Snapshot().ID)
		unidentified.AcceptedEffects[0].Target = openrtc.EffectTarget{}
		for _, opts := range []openrtc.OperationOptions{wrong, action, unidentified} {
			_, e := s.Media().SetMuted(ctx, p.ID, true, opts).Result(ctx)
			expect(c, e, []openrtc.ErrorCode{openrtc.Unsupported}, "wrong acceptance")
		}
		must(f.Condition(ctx, s, "effect-unknown"))
		_, e := s.Media().SetMuted(ctx, p.ID, true, acceptedDisable(b.Snapshot().ID)).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.Unsupported}, "unknown isolation")
		check(c, f.Effects() == before, "invalid acceptance crossed effects")
	}),
	effectCase("CT-EFFECT-004", "accepted extra effect does not waive target postconditions or eligibility", func(ctx context.Context, c *harness.TestContext) {
		s, f, a, b, p := effectSetup(ctx, c)
		value(s.Media().SetMuted(ctx, p.ID, true, acceptedDisable(b.Snapshot().ID)).Result(ctx))
		v := s.Observation().Snapshot().Publications[p.ID]
		check(c, v.State == "published" && mediaBool(v.LocalMuteIntent) && a.Snapshot().State != "stopped", "successful mute lost target/source")
		value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Unpublish(ctx, p.ID, openrtc.UnpublishOptions{}).Result(ctx))
		before := f.Effects()
		_, e := s.Media().SetMuted(ctx, p.ID, true, acceptedDisable(b.Snapshot().ID)).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.InvalidState}, "ended target")
		check(c, f.Effects() == before, "acceptance bypassed target eligibility")
	}),
	effectCase("CT-EFFECT-005", "known inseparable release effect requires acceptance before acquisition", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		other := mediaCapture(ctx, c, s, "session")
		must(f.Condition(ctx, s, "effect-stop:"+string(other.Snapshot().ID)))
		before := f.Effects()
		_, e := s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"audio"}, Ownership: "session"}).Result(ctx)
		expect(c, e, []openrtc.ErrorCode{openrtc.Unsupported}, "unaccepted release dependency")
		check(c, f.Effects() == before, "acquisition preceded required acceptance")
		opts := acceptedDisable(other.Snapshot().ID)
		src := value(s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"audio"}, Ownership: "session", Options: &opts}).Result(ctx))
		value(src.Stop(ctx, openrtc.OperationOptions{}).Result(ctx))
		check(c, src.Snapshot().State == "stopped" && other.Snapshot().State == "disabled", "owner release dropped prior bounded acceptance")
	}),
	effectCase("CT-EFFECT-006", "late undisclosed release coupling retains an identifiable unresolved obligation", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		other := mediaCapture(ctx, c, s, "session")
		must(f.Condition(ctx, s, "effect-late:"+string(other.Snapshot().ID)))
		src := mediaCapture(ctx, c, s, "session")
		value(src.Stop(ctx, openrtc.OperationOptions{}).Result(ctx))
		report := s.Inspection().Cleanup()
		check(c, report.Status.Kind != "complete" && len(report.Status.Outstanding) > 0, "undisclosed late cleanup falsely reported released")
		check(c, other.Snapshot().State == "active", "late release applied unaccepted other-owner effect")
	}),
}
