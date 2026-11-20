package core

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

type nativeSentinel struct {
	Code                 int
	Detail, Message, URL string
	Headers              map[string]string
}

func (s *nativeSentinel) Error() string { return s.Message }
func originalKept(e *openrtc.RTCFailure, s *nativeSentinel) bool {
	return e.Cause == s || (e.ProviderError.Status == "known" && e.ProviderError.Value != nil && e.ProviderError.Value.Original == s)
}

var placementCapsErrorsTests2 = []harness.ConformanceTest{
	{ID: "CT-CAP-004", Title: "negotiation has no side effects and does not change authorization", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CAP-004", "CAP-110", "CAP-005"}, Observable: "after negotiating every portable capability: no credential callback, no capture, no admission, and no authorization decision changed", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		decisions := func() string {
			var d [][2]string
			for _, v := range s.Capabilities().Snapshot().Entries {
				d = append(d, [2]string{string(v.ID), v.Authorization.Decision})
			}
			return harness.SafeJSON(d)
		}
		before := decisions()
		calls := len(c.Spy.Calls())
		native := c.NativeCounts()
		requests := make([]openrtc.CapabilityRequest, 0, len(helpers.PortableCapabilities))
		for _, id := range helpers.PortableCapabilities {
			requests = append(requests, helpers.RequestAt(openrtc.CapabilityID(id), "preferred"))
		}
		value(harness.WithinValue(ctx, c, "negotiate all", 0, func(ctx context.Context) (openrtc.NegotiationResult, error) {
			return s.Capabilities().Negotiate(ctx, requests).Result(ctx)
		}))
		check(c, len(c.Spy.Calls()) == calls, "negotiation requested credentials (CAP-110)")
		check(c, decisions() == before, "negotiation changed an authorization decision (CAP-004)")
		after := c.NativeCounts()
		if native != nil && after != nil {
			check(c, after.Captures == native.Captures, "negotiation prompted capture")
			check(c, after.Admissions == native.Admissions, "negotiation admitted")
		}
	})},
	{ID: "CT-CAP-005", Title: "an authority change invalidates descriptors and emits a revision before the next use", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CAP-005", "CAP-111", "CAP-004", "ADP-117"}, Observable: "after revoke(), onChanged delivers a higher revision; a later required negotiation is not accepted", Needs: []harness.FixtureNeed{"authority-control"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		control := value(c.RequireControl("authority")).(func() harness.AuthorityControl)
		s := joined(ctx, c).Session
		snapshot := s.Capabilities().Snapshot()
		var subject *openrtc.CapabilityDescriptor
		for i := range snapshot.Entries {
			d := &snapshot.Entries[i]
			if (d.Support.Level == "supported" || d.Support.Level == "partial") && d.Authorization.Requirement == "required" {
				subject = d
				break
			}
		}
		if subject == nil {
			must(c.NotApplicable("no capability with a required authorization"))
		}
		var mu sync.Mutex
		var revisions []openrtc.Revision
		stop := s.Capabilities().OnChanged(func(next openrtc.CapabilitySnapshot) {
			mu.Lock()
			revisions = append(revisions, next.Revision)
			mu.Unlock()
		})
		c.Track("onChanged", func(context.Context) error { stop(); return nil })
		authority := control()
		must(c.Within(ctx, "revoke", 0, func(ctx context.Context) error { return authority.Revoke(ctx, subject.ID) }))
		c.Track("restore authority", func(ctx context.Context) error { return authority.Restore(ctx, subject.ID) })
		must(c.WaitFor(ctx, func() bool {
			mu.Lock()
			defer mu.Unlock()
			for _, v := range revisions {
				if v > snapshot.Revision {
					return true
				}
			}
			return false
		}, "a newer capability revision", 0))
		result := value(harness.WithinValue(ctx, c, "negotiate after revoke", 0, func(ctx context.Context) (openrtc.NegotiationResult, error) {
			return s.Capabilities().Negotiate(ctx, []openrtc.CapabilityRequest{helpers.RequestAt(subject.ID)}).Result(ctx)
		}))
		check(c, negotiationStatus(result) != "accepted", fmt.Sprintf("%s still accepted after revocation (CAP-004)", subject.ID))
	})},
	{ID: "CT-CAP-006", Title: "onChanged unsubscribe is idempotent and revisions never decrease", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CAP-111", "CORE-007"}, Observable: "unsubscribing twice does not throw; consecutive snapshot() revisions are non-decreasing", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		stop := cl.Capabilities().OnChanged(func(openrtc.CapabilitySnapshot) {})
		stop()
		stop()
		a := cl.Capabilities().Snapshot().Revision
		b := cl.Capabilities().Snapshot().Revision
		check(c, b >= a, "capability revision decreased")
	})},
	{ID: "CT-ERR-001", Title: "an opaque native failure reports safe unknown code and outcome", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §9:529–570", "RFC §9:529–570", "RFC §12.1:1044–1046", "ERR-001", "ERR-002", "ERR-100", "ERR-104", "ERR-106", "ERR-108", "ADP-114"}, Observable: "opaque failure yields safe unknown classification and unknown outcome, without exposing raw originals", Needs: []harness.FixtureNeed{"native-fault"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		fault := value(c.RequireControl("failNextNativeOperation")).(func(string, any))
		sentinel := &nativeSentinel{Code: 987654, Detail: "opaque native failure (conformance sentinel)"}
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		fault("join", sentinel)
		outcome := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
		check(c, !outcome.OK, "join succeeded although the native layer failed")
		e := value(c.ExpectError(outcome.Error, harness.ExpectedFailure{Kind: "operation"}, "native failure"))
		check(c, e.Cause == nil && (e.ProviderError.Value == nil || e.ProviderError.Value.Original == nil), "default error exposed raw provider payload (RFC §9:544)")
		check(c, slices.Contains(e.Reasons, "unknown") && e.Outcome == "unknown", "unmapped error lost unknown reason/outcome (RFC §9:546–568)")
		check(c, e.Code == openrtc.UnknownCode, fmt.Sprintf("code '%s' for an unrecognized failure (ERR-002)", e.Code))
		check(c, e.Category == "unknown", fmt.Sprintf("category '%s' for an unrecognized failure", e.Category))
		check(c, e.Retryability == "unknown", fmt.Sprintf("retryability '%s' inferred from a numeric code alone (ERR-002)", e.Retryability))
	})},
	{ID: "CT-ERR-002", Title: "serializing a failure does not leak secrets carried by the native original", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §9:544; §12.1:1044–1046", "RFC §9:544–546", "RFC §12.1:1044–1046", "ERR-004", "ERR-107", "ERR-114", "CFG-009"}, Observable: "default JSON, nested diagnostics and formatting omit raw native secrets", Needs: []harness.FixtureNeed{"native-fault"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		fault := value(c.RequireControl("failNextNativeOperation")).(func(string, any))
		marker := "conformance-secret-token-9f31c2"
		sentinel := &nativeSentinel{Message: "request refused", URL: "wss://rtc.invalid/join?access_token=" + marker, Headers: map[string]string{"authorization": "Bearer " + marker}}
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		fault("join", sentinel)
		outcome := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
		check(c, !outcome.OK, "join succeeded although the native layer failed")
		e := value(c.ExpectError(outcome.Error, harness.ExpectedFailure{}, "native failure with secret"))
		serialized, err := json.Marshal(outcome.Error)
		must(err)
		check(c, !strings.Contains(string(serialized), marker), "serialized error contains the embedded secret (ERR-107)")
		check(c, harness.FindLeak(e.Message, []string{marker}) == nil, "the error message contains the embedded secret (ERR-107)")
		check(c, harness.FindLeak(e, []string{marker}) == nil && !strings.Contains(fmt.Sprintf("%+v", e), marker) && !strings.Contains(fmt.Sprintf("%#v", e), marker), "default diagnostics contain raw native secret (RFC §9:544)")
		c.Note("RFC §9:544 requires safe default formatting and serialization; unrestricted diagnostics require explicit authorized access")
	})},
	{ID: "CT-EXT-001", Title: "extension acquisition agrees with the advertised descriptors and never fakes support", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §14.2:1102–1115", "EXT-001", "EXT-100", "EXT-101", "CAP-001", "ADP-117"}, Observable: "unsupported/not-applicable: acquisition throws unsupported-capability; supported/partial: acquisition does not throw it", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		j := joined(ctx, c)
		judged := 0
		sessionDescriptors := j.Session.Capabilities().Snapshot().Entries
		for _, x := range sessionExtensions {
			level := strongest(sessionDescriptors, x.ids)
			a, e := acquire(func() any { return j.Session.Extension(x.key) })
			judged += judge(c, fmt.Sprintf("session.extension('%s')", x.key), level, a, e)
		}
		clientDescriptors := j.Client.Capabilities().Snapshot().Entries
		for _, x := range clientExtensions {
			level := strongest(clientDescriptors, x.ids)
			a, e := acquire(func() any { return j.Client.Extension(x.key) })
			judged += judge(c, fmt.Sprintf("client.extension('%s')", x.key), level, a, e)
		}
		if judged == 0 {
			must(c.Unknown("no extension is described by a descriptor; nothing could be judged"))
		}
	})},
	{ID: "CT-EXT-002", Title: "acquiring an extension provisions, captures, dials and requests credentials for nothing", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"EXT-100", "EXT-001", "CAP-110"}, Observable: "native counters and credential callbacks are unchanged by acquiring every extension key", Needs: []harness.FixtureNeed{"native-effects"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		native := value(c.RequireNative())
		j := joined(ctx, c)
		before := harness.SafeJSON(native.Counts())
		calls := len(c.Spy.Calls())
		for _, x := range sessionExtensions {
			acquire(func() any { return j.Session.Extension(x.key) })
		}
		for _, x := range clientExtensions {
			acquire(func() any { return j.Client.Extension(x.key) })
		}
		check(c, harness.SafeJSON(native.Counts()) == before, "extension acquisition caused native effects")
		check(c, len(c.Spy.Calls()) == calls, "extension acquisition requested credentials")
	})},
	{ID: "CT-EXT-003", Title: "trusted-server extensions follow the administration surface's own descriptors", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §14.2:1102–1115", "EXT-001", "EXT-115", "ADP-111", "ADP-004"}, Observable: "admin.extension('recording'|'session-administration') throws unsupported-capability exactly when the administration descriptors say unsupported", Needs: []harness.FixtureNeed{"admin"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		admin := value(c.Administration(ctx))
		descriptors := admin.Capabilities().Snapshot().Entries
		judged := 0
		for _, x := range []extensionCapabilities{{"recording", []openrtc.PortableCapabilityID{"recording.server"}}, {"session-administration", []openrtc.PortableCapabilityID{"session.end"}}} {
			a, e := acquire(func() any { return admin.Extension(x.key) })
			judged += judge(c, fmt.Sprintf("admin.extension('%s')", x.key), strongest(descriptors, x.ids), a, e)
		}
		if judged == 0 {
			must(c.Unknown("administration descriptors do not mention server extensions"))
		}
	})},
	{ID: "CT-MED-001", Title: "the media control model never claims more than the descriptors do", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"MED-100", "MED-002", "MED-004", "CAP-001", "ADP-117"}, Observable: "independent publication, manual selection and raw-track access appear in the media model only when the matching capability is supported or partial", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		modelValue := s.Inspection().Snapshot().Discovery.Media
		check(c, modelValue.Status == "known" && modelValue.Value != nil, "session media model missing")
		model := *modelValue.Value
		descriptors := s.Capabilities().Snapshot().Entries
		has := func(id openrtc.PortableCapabilityID) bool {
			level := strongest(descriptors, []openrtc.PortableCapabilityID{id})
			return level == "supported" || level == "partial"
		}
		check(c, slices.Contains([]string{"independent", "coupled", "policy-driven", "not-applicable", "unknown"}, model.Publication), "invalid publication mode")
		if !has("media.publish.independent") {
			check(c, model.Publication != "independent", "model claims independent publication without the capability")
		}
		if !has("media.subscribe.manual") {
			for _, x := range []struct {
				name   string
				policy openrtc.ReceiveKindPolicy
			}{{"audio", model.Receive.Audio}, {"video", model.Receive.Video}} {
				check(c, x.policy.Selection != "manual", x.name+": manual selection claimed without the capability (MED-002)")
				check(c, !slices.Contains(x.policy.Controls, "select"), x.name+": lists manual-subscription control without the capability")
			}
		}
		if !has("media.raw-track") {
			check(c, model.RawTrack.Status != "known" || model.RawTrack.Value == nil || !*model.RawTrack.Value, "raw track access claimed without the capability (MED-004)")
		}
	})},
	{ID: "CT-MED-002", Title: "stale or unknown media handles are rejected, not ignored", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"MED-117", "MED-001", "ERR-001"}, Observable: "setMuted, setDisabled and render on identifiers this session never issued reject with RTCError", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		media := joined(ctx, c).Session.Media()
		_, err := c.Rejects(ctx, func(ctx context.Context) (any, error) {
			return media.SetMuted(ctx, "conformance-ghost-publication", true).Result(ctx)
		}, harness.ExpectedFailure{}, "setMuted on an unknown publication")
		must(err)
		_, err = c.Rejects(ctx, func(ctx context.Context) (any, error) {
			return media.SetDisabled(ctx, "conformance-ghost-source", true).Result(ctx)
		}, harness.ExpectedFailure{}, "setDisabled on an unknown source")
		must(err)

		_, err = c.Rejects(ctx, func(ctx context.Context) (any, error) {
			return media.Render(ctx, openrtc.RenderRequest{Source: openrtc.RenderSource{Kind: "publication", Publication: "conformance-ghost-publication"}, Target: openrtc.RenderTarget{Kind: "provider-managed", Target: &openrtc.PlaybackControlTarget{Kind: "session"}}}).Result(ctx)
		}, harness.ExpectedFailure{}, "render of an unknown subscription")
		must(err)
		c.Note("[ambiguity] MED-117 says 'invalid targets' but names no error code; any RTCError is accepted")
	})},
}

// judge compares acquisition with advertised support and returns one when judged.
func judge(c *harness.TestContext, label, level string, acquired bool, thrown any) int {
	if level == "unsupported" || level == "not-applicable" {
		check(c, !acquired, fmt.Sprintf("%s: advertised %s but acquisition returned an object (EXT-001: no dummy implementation)", label, level))
		_, err := c.ExpectError(thrown, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"unsupported-capability"}}, label)
		must(err)
		return 1
	}
	if level == "supported" || level == "partial" {
		check(c, acquired && thrown == nil, label+": advertised support but accessor returned nil or failed (RFC §14.2:1102–1108)")
		c.Partial(label + ": accessor acquisition is not defining operation success; mandatory positive execution required")
		return 1
	}

	c.Note(fmt.Sprintf("%s: capability %s; no verdict", label, level))
	return 0
}
