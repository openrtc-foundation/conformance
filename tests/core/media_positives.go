package core

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func mediaPositive(id, title string, f func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	return mediaCase(id, title, "RFC §7.4:245–385; §8.1:440–455", []string{"RFC §7.4:245–385", "RFC §8.1:440–455", "RFC §14.1:1093", "RFC §14.3:1149"}, f)
}
func outputState(ctx context.Context, c *harness.TestContext, f harness.MediaContractControl, s openrtc.RTCConferenceSession, source openrtc.CaptureSourceID, kind openrtc.MediaKind, want string) {
	observer, ok := f.(harness.MediaOutputControl)
	if !ok {
		must(c.NotRun("missing independent per-kind content observation at claimed output boundary"))
	}
	e := value(observer.ObserveOutput(ctx, s, source, kind))
	if e.Status == "unknown" {
		must(c.Unknown("output observation unavailable at " + e.Boundary))
	}
	check(c, e.Boundary != "" && e.Source == string(source) && e.Kind == string(kind) && e.Interval != "" && e.Evidence != "", "output evidence omits exact source/kind/time/boundary")
	check(c, e.Status == want, "independent output evidence violates established media postcondition")
	if want == "observed" {
		check(c, e.Content != "", "positive output lacks identifiable synthetic content")
	}
}

var MediaPositiveTests = []harness.ConformanceTest{
	mediaPositive("CT-MEDIA-POS-001", "each supported capture kind acquires without publication or admission", func(ctx context.Context, c *harness.TestContext) {
		f := mediaFixture(c)
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		must(f.Prepare(ctx, s))
		before := f.Effects()
		connection := s.Connection().Snapshot()
		for _, kind := range mediaKinds(ctx, c, s, "capture") {
			src := mediaCaptureKind(ctx, c, s, "session", kind)
			check(c, src.Snapshot().State == "active" && len(s.Observation().Snapshot().Publications) == 0, "capture postcondition missing or implicit publication")
		}
		check(c, f.Effects().Publications == before.Publications && s.Connection().Snapshot().State == connection.State, "capture admitted or published")
	}),
	mediaPositive("CT-MEDIA-POS-002", "publish uses eligible existing disabled source without acquisition", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		kinds := mediaKinds(ctx, c, s, "publish")
		for _, kind := range kinds {
			var src openrtc.CaptureSource
			if external, ok := f.(harness.EligibleSourceControl); ok {
				src = value(external.EligibleSource(ctx, s, kind))
			} else {
				src = mediaCaptureKind(ctx, c, s, "session", kind)
			}
			if src.Snapshot().State != "disabled" {
				value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
			}
			before := f.Effects()
			pubs := mediaPublications(ctx, c, s, src)
			check(c, len(pubs) > 0, "publish returned no publication")
			check(c, f.Effects().Captures == before.Captures && src.Snapshot().State == "disabled", "publish acquired or enabled source")
		}
	}),
	mediaPositive("CT-MEDIA-POS-003", "two same-kind publications support publication-specific mute", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		for _, kind := range mediaKinds(ctx, c, s, "capture") {
			src := mediaCaptureKind(ctx, c, s, "session", kind)
			a := mediaPublications(ctx, c, s, src)[0]
			b := mediaPublications(ctx, c, s, src)[0]
			check(c, a.ID != b.ID, "independent publications collapsed")
			value(s.Media().SetMuted(ctx, a.ID, true, openrtc.OperationOptions{}).Result(ctx))
			snap := s.Observation().Snapshot()
			check(c, mediaBool(snap.Publications[a.ID].LocalMuteIntent) && mediaFalse(snap.Publications[b.ID].LocalMuteIntent), "mute did not isolate selected publication")
			outputState(ctx, c, f, s, src.Snapshot().ID, kind, "observed")
		}
	}),
	mediaPositive("CT-MEDIA-POS-004", "disable suppresses every publication dependent on selected source", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		for _, kind := range mediaKinds(ctx, c, s, "capture") {
			src := mediaCaptureKind(ctx, c, s, "session", kind)
			a := mediaPublications(ctx, c, s, src)[0]
			b := mediaPublications(ctx, c, s, src)[0]
			outputState(ctx, c, f, s, src.Snapshot().ID, kind, "observed")
			value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
			snap := s.Observation().Snapshot()
			check(c, mediaBool(snap.Publications[a.ID].EffectiveSuppression.Suppressed) && mediaBool(snap.Publications[b.ID].EffectiveSuppression.Suppressed), "disable missed a dependent publication")
			outputState(ctx, c, f, s, src.Snapshot().ID, kind, "absent")
		}
	}),
	mediaPositive("CT-MEDIA-POS-005", "failed reenable reports uncertainty without restored production claim", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		value(s.Media().SetDisabled(ctx, src.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
		must(f.Condition(ctx, s, "reenable-fails"))
		_, e := s.Media().SetDisabled(ctx, src.Snapshot().ID, false, openrtc.OperationOptions{}).Result(ctx)
		failure := expect(c, e, []openrtc.ErrorCode{openrtc.UnknownCode, openrtc.Unavailable, openrtc.Internal}, "failed reenable")
		check(c, failure.Outcome != "" && src.Snapshot().State != "active", "failed reenable falsely restored production")
	}),
	mediaPositive("CT-MEDIA-POS-006", "unpublish retains source and disable across live suppression states", func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		for _, disabled := range []bool{false, true} {
			value(s.Media().SetDisabled(ctx, src.Snapshot().ID, disabled, openrtc.OperationOptions{}).Result(ctx))
			for _, muted := range []bool{false, true} {
				p := mediaPublications(ctx, c, s, src)[0]
				value(s.Media().SetMuted(ctx, p.ID, muted, openrtc.OperationOptions{}).Result(ctx))
				value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Unpublish(ctx, p.ID, openrtc.UnpublishOptions{Options: &openrtc.OperationOptions{}}).Result(ctx))
				v := src.Snapshot()
				check(c, v.State != "stopped" && mediaBool(v.SourceDisableIntent) == disabled && mediaFalse(v.SourceDisableIntent) == !disabled, "unpublish changed retained source disable")
			}
		}
	}),
	mediaPositive("CT-MEDIA-POS-007", "stop ends dependent publications and repeated release never reacquires", func(ctx context.Context, c *harness.TestContext) {
		s, f := mediaSession(ctx, c)
		src := mediaCapture(ctx, c, s, "session")
		a := mediaPublications(ctx, c, s, src)[0]
		b := mediaPublications(ctx, c, s, src)[0]
		before := f.Effects()
		value(src.Stop(ctx, openrtc.OperationOptions{}).Result(ctx))
		value(src.Stop(ctx, openrtc.OperationOptions{}).Result(ctx))
		for _, p := range []openrtc.TrackPublication{a, b} {
			check(c, s.Observation().Snapshot().Publications[p.ID].State == "unpublished", "stop left dependent publication active")
			value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Unpublish(ctx, p.ID, openrtc.UnpublishOptions{}).Result(ctx))
		}
		check(c, f.Effects().Captures == before.Captures && src.Snapshot().State == "stopped", "repeated release reacquired or revived")
	}),
	mediaPositive("CT-MEDIA-POS-008", "later capture has distinct public identity after stop", func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		a := mediaCapture(ctx, c, s, "session")
		value(a.Stop(ctx, openrtc.OperationOptions{}).Result(ctx))
		b := mediaCapture(ctx, c, s, "session")
		check(c, a.Snapshot().ID != b.Snapshot().ID && a.Snapshot().State == "stopped", "later acquisition reused ended source identity")
	}),
}
