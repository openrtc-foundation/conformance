// Mandatory core tests: credentials (acquisition, failure, renewal, secrecy), cancellation,
// resource cleanup.
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

var cleanupKinds = []string{"complete", "pending", "failed", "unknown"}

// Which credential outcome each advertised replacement mode permits; SES-009, SES-113.
var outcomesForMode = map[string][]string{
	"application-live": {"acknowledged", "submitted"}, "application-next-reconnect": {"cached-for-reconnect"},
	"sdk-managed": {"acknowledged", "unknown"}, "fresh-session": {"unknown"},
	"unsupported": {"unknown"}, "unknown": {"acquired", "cached-for-reconnect", "submitted", "acknowledged", "unknown"}, "not-applicable": {},
}

func checkReport(c *harness.TestContext, report openrtc.CleanupReport, label string) {
	check(c, slices.Contains(cleanupKinds, report.Status.Kind), label+": invalid cleanup status")
	if report.Status.Kind == "pending" || report.Status.Kind == "unknown" || report.Status.Kind == "failed" {
		check(c, len(report.Status.Outstanding) > 0, label+": pending with no outstanding work")
	}
	if report.Status.Kind == "failed" {
		check(c, len(report.Status.Errors) > 0, label+": failed with no errors (ERR-001)")
	}
}
func captureAttempt(ctx context.Context, c *harness.TestContext, a openrtc.JoinAttempt) <-chan harness.Captured[any] {
	ch := make(chan harness.Captured[any], 1)
	go func() {
		ch <- c.Capture(ctx, func(ctx context.Context) (any, error) { return a.Result(ctx).Result(ctx) })
	}()
	return ch
}
func cancelledResult(c *harness.TestContext, ch <-chan harness.Captured[any], success, label string) {
	out := <-ch
	check(c, !out.OK, success)
	value(c.ExpectError(out.Error, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, label))
}
func held(c *harness.TestContext) *harness.Deferred[struct{}] {
	d := harness.NewDeferred[struct{}]()
	c.Spy.SetBehavior(harness.CredentialBehavior{Kind: "hold", Released: d})
	return d
}
func awaitHeld(ctx context.Context, c *harness.TestContext, d *harness.Deferred[struct{}], message string) {
	if !c.Spy.Reached(ctx, 1, c.Target.Timing().OperationMs) {
		d.Resolve(struct{}{})
		must(c.Unknown(message))
	}
}
func credentialStatusEvent(e harness.RecordedEvent) (openrtc.CredentialView, bool) {
	switch p := e.Envelope.Payload.(type) {
	case openrtc.StateChange[openrtc.CredentialView]:
		return p.Current, true
	case *openrtc.StateChange[openrtc.CredentialView]:
		if p != nil {
			return p.Current, true
		}
	}
	return openrtc.CredentialView{}, false
}
func cleanupStatusEvent(e harness.RecordedEvent) (openrtc.CleanupStatus, bool) {
	switch p := e.Envelope.Payload.(type) {
	case openrtc.StateChange[openrtc.CleanupStatus]:
		return p.Current, true
	case *openrtc.StateChange[openrtc.CleanupStatus]:
		if p != nil {
			return p.Current, true
		}
	}
	return openrtc.CleanupStatus{}, false
}

// CredentialsCancelTests defines the credential, cancellation and cleanup core tests.
var CredentialsCancelTests = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-001", Title: "the initial credential request is scoped to this adapter, subject and session", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-111", "CFG-009", "SES-009"}, Observable: "first callback: get, phase admission, reason initial, this adapter, the client's subject, session-admission scope with this adapter's address", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		id := c.FreshIdentity("scope")
		value(c.Joined(ctx, harness.JoinedOptions{Identity: &id}))
		calls := c.Spy.Calls()
		if len(calls) == 0 {
			must(c.Unknown("the adapter never requested a credential; the specification does not say whether credential-less admission is permitted (review finding)"))
		}
		first := calls[0]
		r := first.Request
		check(c, first.Kind == "get", "first credential callback was not get (SES-111)")
		check(c, r.AdapterID == c.Descriptor.AdapterID, "request names another adapter")
		check(c, r.Phase == "admission", fmt.Sprintf("phase '%s', expected admission", r.Phase))
		check(c, r.Reason == "initial", fmt.Sprintf("reason '%s', expected initial", r.Reason))
		check(c, r.Subject.Subject == id.Subject && r.Subject.Namespace == id.Namespace, "subject is not the client's application identity (SES-111)")
		if r.Scope.Kind == "session-admission" {
			check(c, r.Scope.Address != nil && r.Scope.Address.AdapterID == c.Descriptor.AdapterID, "scope address is not scoped to this adapter (SES-111)")
		} else {
			c.Note(fmt.Sprintf("scope kind '%s' for a conference join: [ambiguity] SES-111", r.Scope.Kind))
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-002", Title: "credential acquisition failure retains safe acquisition reason", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §9:544–546", "RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-008", "SES-112", "ERR-001", "ERR-109"}, Observable: "join rejects credential-failed; the callback's own error is retained; no admission; no automatic retry", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		sentinel := errors.New("issuer unavailable (conformance sentinel)")
		c.Spy.SetBehavior(harness.CredentialBehavior{Kind: "reject", Failure: sentinel})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		before := c.NativeCounts()
		out := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
		check(c, !out.OK, "join succeeded although the credential callback failed")
		e := value(c.ExpectError(out.Error, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"credential-failed"}}, "acquisition"))
		check(c, slices.Contains(e.Reasons, "credential-acquisition-failed"), "credential acquisition reason missing (RFC §9:546)")
		check(c, e.Cause == nil, "raw credential acquisition error exposed by default")
		calls := len(c.Spy.Calls())
		check(c, calls == 1, fmt.Sprintf("credential callback invoked %d times; no retry policy was configured", calls))
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.Admissions == before.Admissions, "admission despite no credential")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-003", Title: "a credential the service rejects is authentication-failed, distinct from authorization", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"ERR-109", "ERR-102", "SES-111"}, Observable: "join rejects with code authentication-failed and category authentication, never authorization-denied", Needs: []harness.FixtureNeed{"invalid-credential"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		if target(c).Credentials().Invalid == nil {
			must(c.NotRun("fixture cannot mint an invalid credential"))
		}
		c.Spy.SetBehavior(harness.CredentialBehavior{Kind: "invalid"})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		out := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
		check(c, !out.OK, "join succeeded with an invalid credential")
		e := value(c.ExpectError(out.Error, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"authentication-failed"}}, "invalid"))
		check(c, e.Category == "authentication", fmt.Sprintf("category '%s' (ERR-102)", e.Category))
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(s.Connection().Snapshot().State) }, "terminal after failure", 0))
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-004", Title: "credential bytes never appear in snapshots, events, results or serialized errors", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CFG-009", "CFG-114", "ERR-107"}, Observable: "a deep scan of snapshots, recorded envelopes, capability snapshots and JSON of failures finds none of the credential's UTF-8 or base64 text", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		rec := c.Record(s)
		result := awaitJoin(ctx, c, s.Join(opts(c)), "join")
		needles := c.Spy.Needles()
		if len(needles) == 0 {
			must(c.Unknown("no credential bytes of useful length were observed to scan for"))
		}
		surfaces := []struct {
			label string
			v     any
		}{{"join result", result}, {"snapshot", s.Observation().Snapshot()}, {"connection", s.Connection()}, {"session capabilities", s.Capabilities().Snapshot()}, {"client capabilities", cl.Capabilities().Snapshot()}, {"media model", s.Inspection().Snapshot().Discovery.Media}}
		for _, surface := range surfaces {
			where := harness.FindLeak(surface.v, needles)
			check(c, where == nil || strings.HasPrefix(*where, harness.Truncated), fmt.Sprintf("credential material found in %s at %s", surface.label, leakPath(where)))
			if where != nil && strings.HasPrefix(*where, harness.Truncated) {
				c.Partial(surface.label + ": " + *where)
			}
		}
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		for _, event := range rec.Events() {
			where := harness.FindLeak(event.Envelope, needles)
			check(c, where == nil, fmt.Sprintf("credential material in event %s at %s", event.Key, leakPath(where)))
		}
		if c.Target.(harness.ConformanceTarget).Credentials().Invalid != nil {
			c.Spy.SetBehavior(harness.CredentialBehavior{Kind: "invalid"})
			other := c.Open(cl, value(c.Conference(ctx)))
			failure := c.Capture(ctx, func(ctx context.Context) (any, error) { return other.Join(opts(c)).Result(ctx).Result(ctx) })
			if !failure.OK {
				text := harness.SafeJSON(failure.Error)
				for _, needle := range c.Spy.Needles() {
					check(c, !strings.Contains(text, needle), "credential material in the serialized failure (ERR-107)")
				}
			}
		} else {
			c.Partial("failure serialization not scanned: fixture cannot mint an invalid credential")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-005", Title: "renewal asks for the same scope and reports how the new credential was applied", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-009", "SES-010", "SES-113", "SES-115", "STM-111", "CFG-115"}, Observable: "replacement phase request with unchanged subject/scope; credentialChanged carries a result kind permitted by the advertised mode; the session stays connected", Needs: []harness.FixtureNeed{"credential-renewal"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		trigger := value(c.RequireControl("triggerCredentialRenewal")).(func(context.Context, openrtc.RTCSession) error)
		s := joined(ctx, c).Session
		rec := c.Record(s)
		mode := credentialLifecycle(c, s).ReplacementMode
		if mode == "not-applicable" {
			must(c.NotApplicable("credential replacement not applicable"))
		}
		calls := c.Spy.Calls()
		if len(calls) == 0 {
			must(c.Unknown("no initial credential request to compare against"))
		}
		initial := calls[0]
		count := len(calls)
		must(c.Within(ctx, "trigger renewal", 0, func(ctx context.Context) error { return trigger(ctx, s) }))
		must(c.WaitFor(ctx, func() bool { return len(c.Spy.Calls()) > count || mode == "sdk-managed" }, "renewal callback", 0))
		if mode != "sdk-managed" && mode != "fresh-session" && mode != "unsupported" {
			calls = c.Spy.Calls()
			check(c, len(calls) > 0, "renewal callback missing")
			renewal := calls[len(calls)-1]
			check(c, renewal.Request.Phase == "replacement", "renewal phase is not replacement")
			check(c, renewal.Kind == "renew", "renew was supplied but the adapter called get (SES-009, SES-113)")
			check(c, sameCredentialScope(renewal.Request.Scope, initial.Request.Scope), "renewal scope differs from the original grant (SES-009, SES-115)")
			check(c, renewal.Request.Subject.Subject == initial.Request.Subject.Subject, "renewal changed the subject (SES-115)")
		}
		event := value(helpers.NextEvent(ctx, c, rec.Events, "credentialChanged", "credentialChanged", func(e harness.RecordedEvent) bool {
			status, ok := credentialStatusEvent(e)
			return ok && status.Application != "unknown"
		}, 0))
		status, _ := credentialStatusEvent(event)
		outcome := status.Application
		check(c, slices.Contains(outcomesForMode[mode], outcome), fmt.Sprintf("outcome '%s' not permitted by replacementMode '%s' (SES-113)", outcome, mode))
		check(c, s.Connection().Snapshot().State == "connected", "renewal disturbed the connection")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-006", Title: "a failed renewal is reported without fabricating revocation or disconnection", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-008", "SES-112", "STM-006", "STM-111", "ERR-003", "ERR-101"}, Observable: "credentialChanged and an observation-kind error arrive; the connection stays connected; validity is not expired/revoked unless the expiry is known", Needs: []harness.FixtureNeed{"credential-renewal"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		trigger := value(c.RequireControl("triggerCredentialRenewal")).(func(context.Context, openrtc.RTCSession) error)
		s := joined(ctx, c).Session
		rec := c.Record(s)
		mode := credentialLifecycle(c, s).ReplacementMode
		if mode != "application-live" && mode != "application-next-reconnect" {
			must(c.NotApplicable(fmt.Sprintf("replacementMode '%s' has no application renewal to fail", mode)))
		}
		c.Spy.SetBehavior(harness.CredentialBehavior{Kind: "reject", Failure: errors.New("renewal failure (conformance)")})
		must(c.Within(ctx, "trigger renewal", 0, func(ctx context.Context) error { return trigger(ctx, s) }))
		value(helpers.NextEvent(ctx, c, rec.Events, "error", "error event for renewal failure", nil, 0))
		value(helpers.NextEvent(ctx, c, rec.Events, "credentialChanged", "credentialChanged", nil, 0))
		events := rec.Of("error")
		kind := ""
		if len(events) > 0 {
			switch e := events[0].Envelope.Payload.(type) {
			case *openrtc.RTCObservationError:
				kind = e.Kind
			case *openrtc.RTCFailure:
				kind = e.Kind
			case openrtc.RTCFailure:
				kind = e.Kind
			case map[string]any:
				kind, _ = e["kind"].(string)
			}
		}
		check(c, kind == "observation", "renewal failure was not surfaced as an observation error (ERR-003)")
		status := credentialView(c, s)
		check(c, status.Error.Status == "known", "renewalError not recorded (SES-112)")
		check(c, s.Connection().Snapshot().State == "connected", "renewal failure disconnected a healthy session")
		if status.Metadata.Status != "known" || status.Metadata.Value == nil || status.Metadata.Value.ExpiresAt.Status != "known" {
			check(c, status.Validity != "expired" && status.Validity != "revoked", fmt.Sprintf("validity '%s' fabricated without a known expiry (STM-006)", status.Validity))
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CRD-007", Title: "a credential delivered after cancellation is fenced and never admits", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-114", "STM-010", "STM-011"}, Observable: "cancel aborts the callback's signal; after the late credential is released the session is not connected and no admission occurred", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		d := held(c)
		defer d.Resolve(struct{}{})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		before := c.NativeCounts()
		a := s.Join(opts(c))
		settled := captureAttempt(ctx, c, a)
		awaitHeld(ctx, c, d, "adapter did not request credentials during setup; fencing cannot be observed")
		value(harness.WithinValue(ctx, c, "cancel", 0, func(arg0 context.Context) (openrtc.CancellationReport, error) { return a.Cancel(arg0).Result(arg0) }))
		calls := c.Spy.Calls()
		check(c, len(calls) > 0 && calls[0].Signal.Err() != nil, "credential callback signal not aborted by cancel (SES-114)")
		d.Resolve(struct{}{})
		cancelledResult(c, settled, "a cancelled join resolved successfully", "cancelled join")
		must(c.Settle(ctx))
		check(c, s.Connection().Snapshot().State != "connected", "late credential resurrected the session")
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.Admissions == before.Admissions, "late credential reached the service (SES-114)")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-001", Title: "an already-aborted signal rejects cancelled with no setup effects", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-121", "STM-010"}, Observable: "result rejects cancelled; no credential callback; native admissions and captures unchanged", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		signal, cancel := context.WithCancel(ctx)
		cancel()
		before := c.NativeCounts()
		calls := len(c.Spy.Calls())
		_, err := c.Rejects(ctx, func(ctx context.Context) (any, error) {
			return s.Join(c.JoinOptions(harness.JoinOptionsOverride{Signal: &signal})).Result(ctx).Result(ctx)
		}, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"cancelled"}}, "pre-aborted join")
		must(err)
		check(c, len(c.Spy.Calls()) == calls, "credentials requested for an aborted join")
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, reflect.DeepEqual(before, after), "native effects for an aborted join")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-002", Title: "cancel during setup fences success and reports mode and cleanup honestly", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-010", "SES-121", "SES-122", "STM-115"}, Observable: "cancel() resolves a CancellationReport with a defined mode and cleanup status; result rejects cancelled; state never becomes connected", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		d := held(c)
		defer d.Resolve(struct{}{})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		rec := c.Record(s)
		a := s.Join(opts(c))
		settled := captureAttempt(ctx, c, a)
		awaitHeld(ctx, c, d, "setup never reached a cancellable pause; cannot make cancellation win the race")
		report := value(harness.WithinValue(ctx, c, "cancel", 0, func(arg0 context.Context) (openrtc.CancellationReport, error) { return a.Cancel(arg0).Result(arg0) }))
		d.Resolve(struct{}{})
		check(c, slices.Contains([]string{"native-abort", "deferred-leave", "compensating-leave", "local-fence"}, report.Mode), fmt.Sprintf("invalid cancellation mode '%s'", report.Mode))
		checkReport(c, report.Cleanup, "cancellation cleanup")
		cancelledResult(c, settled, "cancelled join resolved", "cancelled join")
		must(c.Settle(ctx))
		check(c, !slices.Contains(helpers.ConnectionKinds(rec.Events()), "connected"), "a connected state was announced for a cancelled generation (STM-010)")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-003", Title: "aborting the join signal during setup behaves as cancel", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-121", "STM-010"}, Observable: "result rejects cancelled; session is not connected", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		d := held(c)
		defer d.Resolve(struct{}{})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		signal, cancel := context.WithCancel(ctx)
		defer cancel()
		a := s.Join(c.JoinOptions(harness.JoinOptionsOverride{Signal: &signal}))
		settled := captureAttempt(ctx, c, a)
		awaitHeld(ctx, c, d, "setup never reached a pause where abort can win")
		cancel()
		d.Resolve(struct{}{})
		cancelledResult(c, settled, "aborted join resolved", "aborted join")
		check(c, s.Connection().Snapshot().State != "connected", "aborted join is connected")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-004", Title: "cancel after settlement reports non-win and keeps participation", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-121", "STM-115"}, Observable: "RFC §8.5:487: cancel reports non-win; the connection stays connected", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		a := s.Join(opts(c))
		awaitJoin(ctx, c, a, "join")
		report := value(a.Cancel(ctx).Result(ctx))
		check(c, !report.Won, "late cancellation incorrectly won the completed operation")
		check(c, s.Connection().Snapshot().State == "connected", "late cancel disturbed the session")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-005", Title: "leave during pending setup requests cancellation", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-123", "STM-012", "STM-116"}, Observable: "leave() completes and inspection retains a CleanupReport and the pending join rejects cancelled", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		d := held(c)
		defer d.Resolve(struct{}{})
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		settled := captureAttempt(ctx, c, s.Join(opts(c)))
		awaitHeld(ctx, c, d, "setup never reached a pause where leave can race it")
		report := value(harness.WithinValue(ctx, c, "leave during setup", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, s.Inspection(), s.Leave(arg0))
		}))
		d.Resolve(struct{}{})
		checkReport(c, report, "leave during setup")
		cancelledResult(c, settled, "join resolved although leave was requested during setup", "join interrupted by leave")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CAN-006", Title: "late native success after cancel is compensated and does not resurrect the session", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "STM-011", "SES-122", "STM-116"}, Observable: "after cancel and a released native success: result stays cancelled, state not connected, cleanup leaves pending, a native leave was issued", Needs: []harness.FixtureNeed{"join-gate"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		gate := value(c.RequireControl("gateNextJoin")).(func() harness.JoinGate)()
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		before := c.NativeCounts()
		a := s.Join(opts(c))
		settled := captureAttempt(ctx, c, a)
		must(c.Within(ctx, "native join held", 0, func(ctx context.Context) error {
			select {
			case <-gate.Reached():
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
		value(harness.WithinValue(ctx, c, "cancel", 0, func(arg0 context.Context) (openrtc.CancellationReport, error) { return a.Cancel(arg0).Result(arg0) }))
		gate.ReleaseSuccess()
		cancelledResult(c, settled, "late native success resurrected a cancelled join (STM-011)", "cancelled join")
		must(c.Settle(ctx))
		check(c, s.Connection().Snapshot().State != "connected", "session became connected after cancel")
		checkReport(c, s.Inspection().Cleanup(), "late compensation")
		c.Note("RFC §8.6:508: local cancellation and compensation submission do not establish remote release")
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.Leaves > before.Leaves, "no compensating native leave was issued (STM-011)")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CLN-001", Title: "repeated and concurrent leave/dispose share one cleanup and then block operations", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-123", "STM-012", "STM-013", "STM-116", "CORE-114"}, Observable: "concurrent leave/dispose all complete with queryable inspection CleanupReports with terminal identity and retained cleanup; join and configure then reject invalid-state", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		reports := value(harness.WithinValue(ctx, c, "concurrent cleanup", 0, func(ctx context.Context) ([]openrtc.CleanupReport, error) {
			ops := []func(context.Context) (openrtc.CleanupReport, error){func(arg0 context.Context) (openrtc.CleanupReport, error) {
				return cleanupAfter(arg0, s.Inspection(), s.Leave(arg0))
			}, func(arg0 context.Context) (openrtc.CleanupReport, error) {
				return cleanupAfter(arg0, s.Inspection(), s.Leave(arg0))
			}, func(arg0 context.Context) (openrtc.CleanupReport, error) {
				return cleanupAfter(arg0, s.Inspection(), s.Dispose(arg0))
			}, func(arg0 context.Context) (openrtc.CleanupReport, error) {
				return cleanupAfter(arg0, s.Inspection(), s.Dispose(arg0))
			}}
			reports := make([]openrtc.CleanupReport, len(ops))
			errs := make([]error, len(ops))
			var wg sync.WaitGroup
			for i, op := range ops {
				wg.Add(1)
				go func(i int, op func(context.Context) (openrtc.CleanupReport, error)) {
					defer wg.Done()
					reports[i], errs[i] = op(ctx)
				}(i, op)
			}
			wg.Wait()
			for _, err := range errs {
				if err != nil {
					return nil, err
				}
			}
			return reports, nil
		}))
		for i, report := range reports {
			checkReport(c, report, fmt.Sprintf("report %d", i))
		}
		check(c, s.Connection().Snapshot().State == "ended", "repeated release resurrected local participation")
		rejectsJoin(ctx, c, s, "invalid-state", "join after dispose")
		_, err := c.Rejects(ctx, func(ctx context.Context) (any, error) {
			return s.Configure(ctx, openrtc.SessionRuntimeCommand{Target: "participant", ID: new(openrtc.ParticipantConnectionID("conformance-ghost")), Metadata: &openrtc.ParticipantMetadata{}}).Result(ctx)
		}, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"invalid-state"}}, "configure after dispose")
		must(err)
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CLN-002", Title: "cleanup reports do not claim release and stay queryable after disposal", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-153", "SES-150", "STM-011", "STM-114"}, Observable: "inspection cleanup uses the four statuses; pending names outstanding work; the last status is readable after dispose and agrees with the last cleanupChanged", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		rec := c.Record(s)
		report := value(harness.WithinValue(ctx, c, "leave", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, s.Inspection(), s.Leave(arg0))
		}))
		checkReport(c, report, "leave report")
		must(c.Settle(ctx))
		second := value(harness.WithinValue(ctx, c, "dispose", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, s.Inspection(), s.Dispose(arg0))
		}))
		checkReport(c, second, "dispose report")
		must(c.Settle(ctx))
		status := s.Inspection().Cleanup().Status
		check(c, slices.Contains(cleanupKinds, status.Kind), "cleanup status unreadable after dispose")
		check(c, s.Inspection().Snapshot().Cleanup.Status.Kind == status.Kind, "snapshot cleanup differs from connection cleanup")
		events := rec.Of("cleanupChanged")
		if len(events) > 0 {
			current, ok := cleanupStatusEvent(events[len(events)-1])
			check(c, ok && current.Kind == status.Kind, "last cleanupChanged disagrees with the queryable status")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CLN-003", Title: "disposing one client leaves its sibling working; provider disposal aggregates", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-113", "CORE-114", "STM-013"}, Observable: "after client A is disposed, client B's session is connected and leaves normally; provider.dispose reports a valid CleanupReport", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		provider := value(c.Provider(ctx, nil))
		target := value(c.Conference(ctx))
		clientA := value(c.Client(ctx, harness.ClientOptions{Provider: provider}))
		clientB := value(c.Client(ctx, harness.ClientOptions{Provider: provider}))
		a := c.Open(clientA, target)
		b := c.Open(clientB, target)
		awaitJoin(ctx, c, a.Join(opts(c)), "join A")
		awaitJoin(ctx, c, b.Join(opts(c)), "join B")
		checkReport(c, value(harness.WithinValue(ctx, c, "dispose A", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, clientA.Inspection(), clientA.Dispose(arg0))
		})), "client A report")
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(a.Connection().Snapshot().State) }, "A's session to end", 0))
		check(c, b.Connection().Snapshot().State == "connected", "disposing client A affected an unrelated client (CORE-113)")
		checkReport(c, value(harness.WithinValue(ctx, c, "dispose provider", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, provider.Inspection(), provider.Dispose(arg0))
		})), "provider report")
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(b.Connection().Snapshot().State) }, "B's session to end with provider", 0))
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CLN-004", Title: "disposing client and provider never ends the session for other participants", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-153", "SES-119", "SES-120", "STM-013"}, Observable: "a remote endpoint stays connected after the local client and provider dispose", Needs: []harness.FixtureNeed{"remote-endpoint"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		target := value(c.Conference(ctx))
		remote := value(c.Remote(ctx, target, "bystander"))
		provider := value(c.Provider(ctx, nil))
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: provider}))
		value(c.JoinWith(ctx, cl, &target))
		_, err := cleanupAfter(ctx, cl.Inspection(), cl.Dispose(ctx))
		must(err)
		_, err = cleanupAfter(ctx, provider.Inspection(), provider.Dispose(ctx))
		must(err)
		must(c.Settle(ctx))
		check(c, remote.IsConnected(), "local disposal ended the session for a remote endpoint")
	})}),
}

func leakPath(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func sameCredentialScope(a, b openrtc.CredentialScope) bool {
	x, ex := json.Marshal(a)
	y, ey := json.Marshal(b)
	return ex == nil && ey == nil && string(x) == string(y)
}

func cleanupAfter(ctx context.Context, inspection openrtc.InspectionCommon, operation openrtc.Async[openrtc.Void]) (openrtc.CleanupReport, error) {
	_, err := operation.Result(ctx)
	return inspection.Cleanup(), err
}

func credentialLifecycle(c *harness.TestContext, s openrtc.RTCSession) openrtc.CredentialLifecycle {
	channels := s.Inspection().Snapshot().Discovery.CredentialChannels
	for _, channel := range channels {
		if channel.Audience == "participant" {
			return channel.Lifecycle
		}
	}
	must(c.Unknown("no participant credential channel declared"))
	return openrtc.CredentialLifecycle{}
}
func credentialView(c *harness.TestContext, s openrtc.RTCSession) openrtc.CredentialView {
	for _, view := range s.Observation().Snapshot().Credentials {
		if view.Channel != "" {
			return view
		}
	}
	must(c.Unknown("no credential channel view available"))
	return openrtc.CredentialView{}
}
