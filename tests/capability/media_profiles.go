package capability

import (
	"context"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func currentMedia(c *harness.TestContext) harness.CurrentMediaProfileFixture {
	t, ok := c.Target.(harness.CurrentMediaProfileTarget)
	if !ok {
		must(c.NotRun("missing current public profile mapping"))
	}
	return t.CurrentMediaProfiles()
}
func callingFixture(c *harness.TestContext) *harness.CallingProfileFixture {
	f := currentMedia(c).Calling
	if f == nil {
		must(c.NotRun("missing Calling operation/stimulus mapping"))
	}
	return f
}
func subscriptionFixture(c *harness.TestContext) *harness.SubscriptionProfileFixture {
	f := currentMedia(c).Subscription
	if f == nil {
		must(c.NotRun("missing Subscription mapping"))
	}
	return f
}
func grantFixture(c *harness.TestContext) *harness.GrantProfileFixture {
	f := currentMedia(c).Grants
	if f == nil {
		must(c.NotRun("missing Source Grants mapping"))
	}
	return f
}
func screenFixture(c *harness.TestContext) *harness.ScreenProfileFixture {
	f := currentMedia(c).Screen
	if f == nil {
		must(c.NotRun("missing Screen Sharing mapping"))
	}
	return f
}
func recordingFixture(c *harness.TestContext) *harness.RecordingProfileFixture {
	f := currentMedia(c).Recording
	if f == nil {
		must(c.NotRun("missing Recording/service mapping"))
	}
	return f
}
func encryptionFixture(c *harness.TestContext) *harness.EncryptionProfileFixture {
	f := currentMedia(c).Encryption
	if f == nil {
		must(c.NotRun("missing Encryption mapping"))
	}
	return f
}
func knownFalse(v openrtc.Known[bool]) bool {
	return v.Status == "known" && v.Value != nil && !*v.Value
}
func knownTrue(v openrtc.Known[bool]) bool { return v.Status == "known" && v.Value != nil && *v.Value }
func portableFailure(c *harness.TestContext, e error, codes ...openrtc.ErrorCode) *openrtc.RTCFailure {
	return value(c.ExpectError(e, harness.ExpectedFailure{Codes: codes}, "profile operation"))
}
func profileCase(id, title, capability, trace string, f func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	profile := ""
	operation := capability
	mandatory := ""
	switch {
	case strings.HasPrefix(id, "CT-CALL") || strings.HasPrefix(id, "CT-HOLD") || strings.HasPrefix(id, "CT-DIGIT"):
		profile = "Calling"
	case strings.HasPrefix(id, "CT-SUB"):
		profile = "Subscriptions"
	case strings.HasPrefix(id, "CT-GRANT"):
		profile = "Source Grants"
	case strings.HasPrefix(id, "CT-SCREEN"):
		profile = "Screen Sharing"
	case strings.HasPrefix(id, "CT-REC"):
		profile = "Recording"
	case strings.HasPrefix(id, "CT-ENC"):
		profile = "Encryption"
	}
	if strings.HasPrefix(id, "CT-CALL-POS") || strings.HasPrefix(id, "CT-SUB") || strings.HasPrefix(id, "CT-GRANT") || strings.HasPrefix(id, "CT-SCREEN") || strings.HasPrefix(id, "CT-REC") || strings.HasPrefix(id, "CT-ENC") {
		mandatory = profile
	}
	return harness.DefineTest(harness.ConformanceTest{ID: id, Title: title, Observable: title, Suite: harness.Capability, Profile: harness.ProfileName(profile), Requirements: []string{trace, "RFC §14.2:1102–1115"}, Capability: &harness.CapabilityDependence{ID: capability, Operation: operation, MandatoryProfile: mandatory, AppliesWhen: "supported-or-partial"}, Run: run(f)})
}

func profileDial(ctx context.Context, c *harness.TestContext, f *harness.CallingProfileFixture) openrtc.CurrentCall {
	if f.Dial == nil {
		must(c.NotRun("missing public dial mapping"))
	}
	call := value(f.Dial(ctx).Result(ctx))
	check(c, call != nil && call.Snapshot().ID != "", "dial returned no Call")
	c.Track("call", func(ctx context.Context) error { _, e := call.Hangup(ctx).Result(ctx); return e })
	return call
}

var currentMediaProfileTests = []harness.ConformanceTest{
	profileCase("CT-CALL-POS-001", "outgoing dial creates distinct Calls", "calling.dial", "RFC §11.1:590–600", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		a := profileDial(ctx, c, f)
		b := profileDial(ctx, c, f)
		check(c, a.Snapshot().ID != b.Snapshot().ID, "distinct dial reused Call identity")
	}),
	profileCase("CT-CALL-POS-002", "incoming delivery neither answers nor sends", "calling.accept", "RFC §11.1:590–600", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.Incoming == nil {
			must(c.NotRun("missing independent incoming stimulus"))
		}
		call := value(f.Incoming(ctx))
		v := call.Snapshot()
		check(c, !v.Answered && (v.State == "ringing" || v.State == "incoming"), "incoming delivery implicitly answered")
		if f.Effects == nil {
			must(c.NotRun("missing incoming no-capture/send boundary observation"))
		}
		effects := f.Effects()
		check(c, effects.Captures == 0 && effects.Publications == 0, "incoming delivery acquired or sent media")
	}),
	profileCase("CT-CALL-POS-003", "accept establishes answer on a separate incoming Call", "calling.accept", "RFC §11.1:590–600", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.Incoming == nil {
			must(c.NotRun("missing incoming stimulus"))
		}
		call := value(f.Incoming(ctx))
		value(call.Accept(ctx, openrtc.OperationOptions{}).Result(ctx))
		check(c, call.Snapshot().Answered && call.Snapshot().State != "ended", "accept succeeded without answer")
	}),
	profileCase("CT-CALL-POS-004", "reject needs no optional reason and hangup is idempotent", "calling.reject", "RFC §11.1:590–630", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.Incoming == nil {
			must(c.NotRun("missing incoming stimulus"))
		}
		call := value(f.Incoming(ctx))
		value(call.Reject(ctx).Result(ctx))
		check(c, call.Snapshot().State == "ended", "reject did not end offered Call")
		value(call.Hangup(ctx).Result(ctx))
		value(call.Hangup(ctx).Result(ctx))
		check(c, call.Snapshot().State == "ended", "repeat hangup revived Call")
	}),
	profileCase("CT-CALL-POS-005", "hold and resume establish only local intent", "calling.hold", "RFC §11.1:600–630", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		call := profileDial(ctx, c, f)
		value(call.SetHold(ctx, true, openrtc.OperationOptions{}).Result(ctx))
		check(c, knownTrue(call.Snapshot().LocalHold), "hold returned success without establishment")
		value(call.SetHold(ctx, false, openrtc.OperationOptions{}).Result(ctx))
		check(c, knownFalse(call.Snapshot().LocalHold), "resume did not establish local false")
	}),
	profileCase("CT-HOLD-001", "answered unknown hold and transfer overlays retain independent facts", "calling.hold", "RFC §11.1:600–616; A.3:1342", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.RemoteHold == nil {
			must(c.NotRun("missing independent hold stimulus"))
		}
		call := profileDial(ctx, c, f)
		value(call.SetHold(ctx, false, openrtc.OperationOptions{}).Result(ctx))
		for _, tc := range []struct {
			hold  openrtc.Known[bool]
			state openrtc.CallState
		}{{openrtc.Unknown[bool](), "unknown"}, {openrtc.KnownValue(true), "held"}, {openrtc.KnownValue(false), "active"}} {
			must(f.RemoteHold(ctx, call, tc.hold))
			v := call.Snapshot()
			check(c, v.Answered && v.State == tc.state, "displayed hold lost answer or uncertainty")
		}
		must(f.RemoteHold(ctx, call, openrtc.Unknown[bool]()))
		value(call.SetHold(ctx, true, openrtc.OperationOptions{}).Result(ctx))
		value(call.SetHold(ctx, false, openrtc.OperationOptions{}).Result(ctx))
		check(c, call.Snapshot().RemoteHold.Status == "unknown" && call.Snapshot().State == "unknown", "resume cleared unknown remote hold")
		if claim, ok := c.Target.(harness.ClaimTarget); ok {
			for _, operation := range claim.ClaimDeclaration().Operations {
				if operation.Operation == "transfer" && (operation.Support == "supported" || operation.Support == "conditional") && !f.TransferClaimed {
					must(c.NotRun("claimed transfer positive lacks supported transfer fixture"))
				}
			}
		}
		if f.TransferClaimed {
			if f.FinishTransfer == nil {
				must(c.NotRun("claimed transfer lacks completion stimulus"))
			}
			value(call.Transfer(ctx, openrtc.OperationOptions{}).Result(ctx))
			check(c, call.Snapshot().TransferActive && call.Snapshot().State == "transferring", "transfer overlay missing")
			must(f.FinishTransfer(ctx, call))
			check(c, call.Snapshot().State == "unknown" && call.Snapshot().Answered, "transfer completion erased unknown hold")
		}
		value(call.Hangup(ctx).Result(ctx))
		check(c, call.Snapshot().State == "ended", "unknown answered Call cannot hangup")
	}),
	profileCase("CT-CALL-POLICY-001", "kind-adding remote offers follow selected policy before setup", "calling.dial", "RFC §11.1:596/622; §14.3:1132", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.RemoteOffer == nil {
			must(c.NotRun("missing remote kind-adding offer stimulus"))
		}
		call := profileDial(ctx, c, f)
		for _, tc := range []struct {
			policy   openrtc.RemoteUpdatePolicy
			decision string
		}{{"reject", "rejected"}, {"accept-without-new-kinds", "accepted-without-new-kinds"}} {
			must(f.RemoteOffer(ctx, call, tc.policy, true))
			v := call.Snapshot()
			check(c, len(v.Updates) > 0, "remote update decision unobservable")
			u := v.Updates[len(v.Updates)-1]
			check(c, u.Decision == tc.decision && !slices.Contains(v.Kinds, openrtc.MediaKind("video")), "remote offer violated selected no-new-kinds policy")
		}
		// Unsupported policy enforcement must fail before native setup, not silently weaken.
		e := f.RemoteOffer(ctx, call, "accept-without-new-kinds", false)
		portableFailure(c, e, openrtc.Unsupported)
	}),
	profileCase("CT-CALL-POLICY-002", "withheld application transfer decision rejects by deadline", "calling.transfer", "RFC §11.1:622–630; §14.3:1135", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.WithholdTransferDecision == nil {
			must(c.NotRun("missing incoming transfer/deadline stimulus"))
		}
		call := profileDial(ctx, c, f)
		must(f.WithholdTransferDecision(ctx, call))
		check(c, !call.Snapshot().TransferActive && call.Snapshot().State != "ended", "withheld transfer executed or ended original Call")
	}),
	profileCase("CT-CALL-POLICY-003", "receiving registration respects expiry refresh stop and late-offer fences", "calling.receive", "RFC §11.1:645–647", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.StartReceiving == nil || f.StopReceiving == nil || f.Registration == nil || f.RefreshRegistration == nil || f.ExpireRegistration == nil || f.ObserveRegistration == nil || f.QueueIncoming == nil || f.DeliverQueuedIncoming == nil || f.IncomingOffers == nil || f.DisposeReceiver == nil {
			must(c.NotRun("missing public registration operations, expiry clock or late-offer controls"))
		}
		var mu sync.Mutex
		var observations []openrtc.RegistrationStatus
		stop := f.ObserveRegistration(func(r openrtc.RegistrationStatus) { mu.Lock(); observations = append(observations, r); mu.Unlock() })
		defer stop()
		value(f.StartReceiving(ctx).Result(ctx))
		r := f.Registration()
		check(c, (r.Status == "registered" || r.Status == "receiving") && knownTrue(r.IncomingDeliveryEnabled), "start succeeded without evidenced local receiving readiness")
		check(c, r.Account.Status == "known" && r.Account.Value != nil && reflect.DeepEqual(*r.Account.Value, f.Account), "registration omitted actual account scope")
		check(c, r.ExpiresAt.Status == "known" || r.ExpiresAt.Status == "unknown" || r.ExpiresAt.Status == "not-applicable", "registration expiry unqualified")
		must(f.RefreshRegistration(ctx))
		check(c, knownTrue(f.Registration().IncomingDeliveryEnabled), "refresh invalidated still-evidenced readiness")
		must(f.ExpireRegistration(ctx))
		expired := f.Registration()
		check(c, expired.Validity != "valid" && !knownTrue(expired.IncomingDeliveryEnabled), "expired registration remained valid without renewal")
		value(f.StartReceiving(ctx).Result(ctx))
		existing := profileDial(ctx, c, f)
		must(f.QueueIncoming(ctx))
		before := f.IncomingOffers()
		value(f.StopReceiving(ctx).Result(ctx))
		value(f.StopReceiving(ctx).Result(ctx))
		must(f.DeliverQueuedIncoming(ctx))
		check(c, f.IncomingOffers() == before, "stopped receiving delivered queued late invitation")
		check(c, existing.Snapshot().State != "ended", "stop receiving terminated existing Call")
		value(f.StartReceiving(ctx).Result(ctx))
		must(f.QueueIncoming(ctx))
		before = f.IncomingOffers()
		must(f.DisposeReceiver(ctx))
		must(f.DeliverQueuedIncoming(ctx))
		check(c, f.IncomingOffers() == before, "disposed client delivered obsolete late invitation")
		mu.Lock()
		seen := len(observations)
		mu.Unlock()
		check(c, seen >= 3, "registration readiness/expiry changes unobservable")
	}),
	profileCase("CT-DIGIT-001", "prefix failure and authorized fallback never duplicate digits", "calling.dtmf", "RFC §11.1:632–646; A.3:1363–1364", func(ctx context.Context, c *harness.TestContext) {
		f := callingFixture(c)
		if f.FailDigitsAfter == nil || f.SubmittedDigits == nil {
			must(c.NotRun("missing independent digit observation"))
		}
		call := profileDial(ctx, c, f)
		must(f.FailDigitsAfter(ctx, 2))
		r, e := call.SendDigits(ctx, "1234", []openrtc.DtmfMethod{"rtp-event", "signaling"}, openrtc.OperationOptions{}).Result(ctx)
		if e != nil {
			portableFailure(c, e)
		}
		submitted := f.SubmittedDigits()
		check(c, submitted == "12" || submitted == "1234", "fallback duplicated or reordered submitted digits")
		check(c, r.SubmittedPrefix.Status == "unknown" || (r.SubmittedPrefix.Status == "known" && r.SubmittedPrefix.Value != nil && *r.SubmittedPrefix.Value == submitted), "digit prefix misreported")
	}),
	profileCase("CT-SUB-001", "subscribe selects exact remote publication and conflicts on duplicate", "media.subscribe.manual", "RFC §11.2:656–667", func(ctx context.Context, c *harness.TestContext) {
		f := subscriptionFixture(c)
		p := f.Publication
		check(c, p.ID != "" && len(p.Kinds) > 0, "missing requested remote publication")
		sub := value(f.Control.Subscribe(ctx, p.ID, p.Kinds).Result(ctx))
		check(c, sub.ID != "" && sub.State == "subscribed" && sub.Source.Publication == p.ID, "subscribe did not establish exact selection")
		_, e := f.Control.Subscribe(ctx, p.ID, p.Kinds).Result(ctx)
		portableFailure(c, e, openrtc.InvalidState)
	}),
	profileCase("CT-SUB-002", "unsubscribe ends only local Subscription and repeated release is safe", "media.subscribe.manual", "RFC §11.2:656–667", func(ctx context.Context, c *harness.TestContext) {
		f := subscriptionFixture(c)
		if f.Snapshot == nil || f.RemotePublication == nil {
			must(c.NotRun("missing public subscription/publication observations"))
		}
		p := f.Publication
		sub := value(f.Control.Subscribe(ctx, p.ID, p.Kinds).Result(ctx))
		value(f.Control.Unsubscribe(ctx, sub.ID).Result(ctx))
		value(f.Control.Unsubscribe(ctx, sub.ID).Result(ctx))
		check(c, f.Snapshot(sub.ID).State == "unsubscribed" && f.RemotePublication().ID == p.ID && f.RemotePublication().State == p.State, "unsubscribe removed remote resource or retained selection")
	}),
	profileCase("CT-SUB-003", "later subscription has new identity and teardown releases local selection", "media.subscribe.manual", "RFC §11.2:656–667", func(ctx context.Context, c *harness.TestContext) {
		f := subscriptionFixture(c)
		if f.Teardown == nil || f.Snapshot == nil {
			must(c.NotRun("missing teardown/observation mapping"))
		}
		a := value(f.Control.Subscribe(ctx, f.Publication.ID, f.Publication.Kinds).Result(ctx))
		value(f.Control.Unsubscribe(ctx, a.ID).Result(ctx))
		b := value(f.Control.Subscribe(ctx, f.Publication.ID, f.Publication.Kinds).Result(ctx))
		check(c, a.ID != b.ID, "later subscribe reused ended identity")
		must(f.Teardown(ctx))
		check(c, f.Snapshot(b.ID).State == "unsubscribed", "owner teardown left local selection active")
	}),
}

func init() { CapabilityTests = append(CapabilityTests, currentMediaProfileTests...) }

var additionalMediaProfileTests = []harness.ConformanceTest{
	profileCase("CT-GRANT-001", "source grant authorizes B while C remains denied", "media.source-grants", "RFC §11.3:669–682", func(ctx context.Context, c *harness.TestContext) {
		f := grantFixture(c)
		src := f.Source.Snapshot()
		g := value(f.Control.ShareSource(ctx, src.ID, f.B, f.Scope, openrtc.OperationOptions{}).Result(ctx))
		check(c, g.ID != "" && g.Source == src.ID && g.Recipient.ID == f.B.ID && g.Owner == src.Owner && g.State == "active", "grant omitted exact source/recipient")
		value(f.Use(ctx, f.B, f.Scope).Result(ctx))
		_, e := f.Use(ctx, f.C, f.Scope).Result(ctx)
		portableFailure(c, e, openrtc.PermissionDenied)
	}),
	profileCase("CT-GRANT-002", "bounded grant never authorizes another scope", "media.source-grants", "RFC §11.3:669–682", func(ctx context.Context, c *harness.TestContext) {
		f := grantFixture(c)
		value(f.Control.ShareSource(ctx, f.Source.Snapshot().ID, f.B, f.Scope, openrtc.OperationOptions{}).Result(ctx))
		wrong := f.Scope
		wrong.Resource = "ungranted-scope"
		_, e := f.Use(ctx, f.B, wrong).Result(ctx)
		portableFailure(c, e, openrtc.PermissionDenied)
	}),
	profileCase("CT-GRANT-003", "revoke is idempotent and regrant has distinct identity", "media.source-grants", "RFC §11.3:669–682", func(ctx context.Context, c *harness.TestContext) {
		f := grantFixture(c)
		a := value(f.Control.ShareSource(ctx, f.Source.Snapshot().ID, f.B, f.Scope, openrtc.OperationOptions{}).Result(ctx))
		for i := 0; i < 2; i++ {
			value(f.Control.RevokeSourceGrant(ctx, a.ID, openrtc.OperationOptions{}).Result(ctx))
		}
		check(c, f.Control.Grant(a.ID).State == "revoked", "grant remains active after revoke")
		_, e := f.Use(ctx, f.B, f.Scope).Result(ctx)
		portableFailure(c, e, openrtc.PermissionDenied)
		b := value(f.Control.ShareSource(ctx, f.Source.Snapshot().ID, f.B, f.Scope, openrtc.OperationOptions{}).Result(ctx))
		check(c, a.ID != b.ID, "regrant reused revoked identity")
	}),
	profileCase("CT-GRANT-004", "recipient teardown preserves foreign owner's source", "media.source-grants", "RFC §11.3:669–682; §6:35", func(ctx context.Context, c *harness.TestContext) {
		f := grantFixture(c)
		if f.TeardownRecipient == nil {
			must(c.NotRun("missing recipient teardown mapping"))
		}
		before := f.Source.Snapshot()
		value(f.Control.ShareSource(ctx, before.ID, f.B, f.Scope, openrtc.OperationOptions{}).Result(ctx))
		must(f.TeardownRecipient(ctx, f.B))
		after := f.Source.Snapshot()
		check(c, after.ID == before.ID && after.State != "stopped" && after.Owner == before.Owner, "recipient teardown ended or reowned foreign source")
		replacementContract(ctx, c, f.Replacement)
	}),
	profileCase("CT-SCREEN-001", "screen capture reports selection without implicit publication", "media.screen-share", "RFC §11.4:686–733", func(ctx context.Context, c *harness.TestContext) {
		f := screenFixture(c)
		r := value(f.Capture(ctx, openrtc.OperationOptions{}).Result(ctx))
		check(c, r.Source.ID != "" && r.Source.Owner.ID != "" && (r.Source.Owner.Kind == "local-session" || r.Source.Owner.Kind == "session") && r.Source.State != "stopped" && r.Selection.Status == "known" && r.Selection.Value != nil && *r.Selection.Value != "", "screen selection not established")
		check(c, len(r.Publications) == 0, "screen acquisition implicitly published")
		check(c, r.AudioAssociation.Status == "known" || r.AudioAssociation.Status == "unknown" || r.AudioAssociation.Status == "not-applicable", "audio association unqualified")
	}),
	profileCase("CT-SCREEN-002", "dismissal termination and screen scope/audio changes remain observable", "media.screen-share", "RFC §11.4:713–733; §9:538/546; §8.3:469–477", func(ctx context.Context, c *harness.TestContext) {
		f := screenFixture(c)
		if f.DismissNext == nil || f.EndSelection == nil || f.Snapshot == nil || f.Observe == nil || f.ChangeSelection == nil {
			must(c.NotRun("missing screen platform scope-change/termination stimulus and stream mapping"))
		}
		must(f.DismissNext(ctx))
		_, e := f.Capture(ctx, openrtc.OperationOptions{}).Result(ctx)
		failure := portableFailure(c, e, openrtc.Cancelled)
		check(c, slices.Contains(failure.Reasons, "selection-dismissed"), "dismissal lost selection-dismissed reason")
		r := value(f.Capture(ctx, openrtc.OperationOptions{}).Result(ctx))
		var mu sync.Mutex
		var events []openrtc.SelectedScreen
		stop := f.Observe(func(v openrtc.SelectedScreen) { mu.Lock(); events = append(events, v); mu.Unlock() })
		defer stop()
		must(f.ChangeSelection(ctx, r.Source.ID))
		changed := f.Snapshot(r.Source.ID)
		check(c, reflect.DeepEqual(changed.Selection, f.ChangedSelection) && reflect.DeepEqual(changed.AudioAssociation, f.ChangedAudioAssociation), "selection scope/audio association change misrepresented")
		must(f.EndSelection(ctx, r.Source.ID))
		check(c, f.Snapshot(r.Source.ID).Source.State == "stopped", "platform end not reflected")
		mu.Lock()
		copyEvents := append([]openrtc.SelectedScreen(nil), events...)
		mu.Unlock()
		scopeSeen, endSeen := false, false
		for _, event := range copyEvents {
			if event.Source.ID != r.Source.ID {
				continue
			}
			scopeSeen = scopeSeen || (event.Source.State != "stopped" && reflect.DeepEqual(event.Selection, f.ChangedSelection) && reflect.DeepEqual(event.AudioAssociation, f.ChangedAudioAssociation))
			endSeen = endSeen || event.Source.State == "stopped"
		}
		check(c, scopeSeen && endSeen, "screen scope or termination established change missing from stream")
	}),
	profileCase("CT-SCREEN-003", "unsupported required screen condition rejects before acquisition", "media.screen-share", "RFC §11.4:686–733; §7.1:69–88", func(ctx context.Context, c *harness.TestContext) {
		f := screenFixture(c)
		if f.Effects == nil || f.UnsupportedCondition.Name == "" {
			must(c.NotRun("missing unsupported-condition coverage"))
		}
		before := f.Effects()
		_, e := f.Capture(ctx, openrtc.OperationOptions{Conditions: []openrtc.Condition{f.UnsupportedCondition}}).Result(ctx)
		portableFailure(c, e, openrtc.Unsupported)
		check(c, f.Effects() == before, "unsupported screen selection acquired resources")
	}),
	profileCase("CT-REC-001", "recording start requires established recording", "recording.server", "RFC §11.6:784–826", func(ctx context.Context, c *harness.TestContext) {
		f := recordingFixture(c)
		job := value(f.Start(ctx).Result(ctx))
		check(c, job != nil && job.Snapshot().ID != "" && job.Snapshot().Status == "running", "recording start succeeded at pending/unknown milestone")
	}),
	profileCase("CT-REC-002", "withheld or denied consent starts no service job", "recording.server", "RFC §11.6:784–826; §12.2:1052–1058", func(ctx context.Context, c *harness.TestContext) {
		f := recordingFixture(c)
		if f.SetConsent == nil || f.ServiceJobs == nil || f.ServiceContent == nil {
			must(c.NotRun("missing independent service consent controls"))
		}
		for _, consent := range []string{"unknown", "denied"} {
			must(f.SetConsent(ctx, consent))
			check(c, f.ServiceContentBoundary != "" && f.ServiceContentEvidence != "", "recorded-content observation lacks exact service boundary/evidence")
			contentBefore := f.ServiceContent()
			if contentBefore.Status != "known" || contentBefore.Value == nil {
				must(c.Unknown("independent recorded-content observation unavailable"))
			}
			before := f.ServiceJobs()
			if before.Status != "known" || before.Value == nil {
				must(c.Unknown("independent service job population unavailable"))
			}
			_, e := f.Start(ctx).Result(ctx)
			portableFailure(c, e, openrtc.PermissionDenied, openrtc.Unsupported)
			after := f.ServiceJobs()
			if after.Status != "known" || after.Value == nil {
				must(c.Unknown("service job observation unavailable"))
			}
			check(c, *after.Value == *before.Value, "missing consent created service job")
			contentAfter := f.ServiceContent()
			if contentAfter.Status != "known" || contentAfter.Value == nil {
				must(c.Unknown("recorded-content observation unavailable"))
			}
			check(c, *contentAfter.Value == *contentBefore.Value, "missing consent produced protected recorded content")
		}
	}),
	profileCase("CT-REC-003", "stop acknowledgment cannot establish remote recording stop", "recording.server", "RFC §11.6:784–826; §8.6:506–510", func(ctx context.Context, c *harness.TestContext) {
		f := recordingFixture(c)
		if f.ObserveRelease == nil {
			must(c.NotRun("missing independent recording release observation"))
		}
		job := value(f.Start(ctx).Result(ctx))
		receipt := value(job.Stop(ctx).Result(ctx))
		if receipt.Status != "confirmed" {
			check(c, job.Snapshot().Status != "stopped", "submission acknowledgment falsely established recording stop")
		}
		must(f.ObserveRelease(ctx, job.Snapshot().ID))
		check(c, job.Snapshot().Status == "stopped", "observed stop not retained")
	}),
	profileCase("CT-REC-004", "recording lifetime follows declared owner", "recording.server", "RFC §11.6:784–826", func(ctx context.Context, c *harness.TestContext) {
		f := recordingFixture(c)
		if f.LeaveOwner == nil {
			must(c.NotRun("missing recording owner teardown mapping"))
		}
		job := value(f.Start(ctx).Result(ctx))
		must(f.LeaveOwner(ctx))
		if f.Independent {
			check(c, job.Snapshot().Status == "running", "independent trusted-server job ended on local leave")
			value(job.Stop(ctx).Result(ctx))
		} else {
			check(c, job.Snapshot().Status != "running", "session leave did not initiate owned recording stop")
		}
	}),
	profileCase("CT-ENC-001", "encryption configure establishes requested exact coverage", "encryption.media", "RFC §11.8:879–945; §14.2:1113–1115", func(ctx context.Context, c *harness.TestContext) {
		f := encryptionFixture(c)
		value(f.Control.Configure(ctx, f.Request).Result(ctx))
		v := f.Control.Coverage()
		check(c, knownTrue(v.Effective) && v.Mode.Status == "known" && v.Mode.Value != nil && *v.Mode.Value == f.Request.Mode, "encryption positive lacks effective requested mode")
		check(c, len(v.Evidence) > 0 && len(v.TrustedEndpoints) > 0 && v.KeyResponsibility.Status == "known", "encryption omitted boundary/key evidence")
		check(c, sameStringSet(v.Directions, f.Request.Directions) && sameEndpoints(v.TrustedEndpoints, f.Request.TrustedEndpoints) && sameEncryptionBoundary(v.Boundary, f.Request.Boundary), "encryption established unrelated direction/endpoints/content boundary")
		if f.Request.Data {
			check(c, knownTrue(v.Data), "required data encryption coverage missing")
		}
		for _, kind := range f.Request.MediaKinds {
			check(c, slices.Contains(v.MediaKinds, kind), "encryption omitted requested kind")
		}
	}),
	profileCase("CT-ENC-002", "verification loss contains further protected flow", "encryption.media", "RFC §11.8:879–945", func(ctx context.Context, c *harness.TestContext) {
		f := encryptionFixture(c)
		if f.WithdrawEvidence == nil || f.ProtectedEffects == nil || f.SendProtected == nil {
			must(c.NotRun("missing independent encryption loss/output stimulus"))
		}
		value(f.Control.Configure(ctx, f.Request).Result(ctx))
		must(f.WithdrawEvidence(ctx))
		check(c, !knownTrue(f.Control.Coverage().Effective), "withdrawn evidence still claimed effective")
		before := f.ProtectedEffects()
		_, e := f.SendProtected(ctx).Result(ctx)
		portableFailure(c, e, openrtc.Unsupported, openrtc.InvalidState)
		check(c, f.ProtectedEffects() == before, "verification loss allowed affected content outside requirement")
	}),
	profileCase("CT-ENC-003", "key material stays inside authorized boundary", "encryption.media", "RFC §11.8:879–945; §12.3:1064", func(ctx context.Context, c *harness.TestContext) {
		f := encryptionFixture(c)
		if f.DefaultSurfaces == nil || f.KeyCanary == "" {
			must(c.NotRun("missing key boundary canary and public surfaces"))
		}
		value(f.Control.Configure(ctx, f.Request).Result(ctx))
		for _, surface := range f.DefaultSurfaces() {
			check(c, harness.FindLeak(surface, []string{f.KeyCanary}) == nil, "public surface exposed key material")
		}
	}),
}

func init() { CapabilityTests = append(CapabilityTests, additionalMediaProfileTests...) }

func init() {
	CapabilityTests = append(CapabilityTests, profileCase("CT-SHAPE-001", "call-only publications retain Call ownership and foreign source owner", "calling.media", "RFC §11.1:592; A.1:1241–1242; A.3:1339", func(ctx context.Context, c *harness.TestContext) {
		f := currentMedia(c).CallMedia
		if f == nil || f.Call == nil || f.PublishCall == nil || f.Publication == nil || f.ConferenceEffects == nil {
			must(c.NotRun("missing call-only media mapping and conference boundary observation"))
		}
		before := f.ConferenceEffects()
		var sources []openrtc.CaptureSource
		if target, ok := c.Target.(harness.ClaimTarget); ok {
			for _, operation := range target.ClaimDeclaration().Operations {
				if operation.Operation == "captureCall" && operation.Support != "unsupported" && f.CaptureCall == nil {
					must(c.NotRun("claimed captureCall has no actual acquisition fixture"))
				}
			}
		}
		if f.CaptureCall != nil {
			source := value(f.CaptureCall(ctx).Result(ctx))
			view := source.Snapshot()
			check(c, view.ID != "" && view.Owner.Kind == "call" && view.Owner.ID == f.Call.Snapshot().ID && view.State == "active", "captureCall did not establish call-owned source")
			for _, kind := range view.Kinds {
				c.RecordCoverage(harness.OperationClaim{Operation: "captureCall", Kinds: []string{string(kind)}, Scope: "call", Support: "supported"})
			}
			sources = append(sources, source)
		}
		if f.GrantedSource != nil {
			sources = append(sources, f.GrantedSource)
		}
		if len(sources) == 0 {
			must(c.NotRun("advertised call media has no supported captureCall or owner-granted source fixture"))
		}
		type ownedPublication struct {
			publication openrtc.TrackPublication
			source      openrtc.CaptureSource
			owner       openrtc.ResourceRef
		}
		var publications []ownedPublication
		for _, source := range sources {
			owner := source.Snapshot().Owner
			p := value(f.PublishCall(ctx, source).Result(ctx))
			check(c, p.Owner.Kind == "call" && p.Owner.ID == f.Call.Snapshot().ID, "publishCall fabricated conference owner")
			check(c, source.Snapshot().Owner == owner, "publishCall changed granted source owner")
			for _, kind := range source.Snapshot().Kinds {
				c.RecordCoverage(harness.OperationClaim{Operation: "publishCall", Kinds: []string{string(kind)}, Scope: "call", Support: "supported"})
			}
			publications = append(publications, ownedPublication{p, source, owner})
		}
		value(f.Call.Hangup(ctx).Result(ctx))
		for _, entry := range publications {
			check(c, f.Publication(entry.publication.ID).State == "unpublished", "hangup left call-owned publication active")
			if entry.owner.Kind != "call" {
				check(c, entry.source.Snapshot().State != "stopped", "hangup ended foreign source")
			} else {
				check(c, entry.source.Snapshot().State == "stopped", "hangup left call-owned source active")
			}
		}
		check(c, f.ConferenceEffects() == before, "call media required conference setup")
	}))
}

// RFC §11.3:681–683: replacement retains both intents and source owners;
// required unverified continuity rejects before effects, partial effects survive.
func replacementContract(ctx context.Context, c *harness.TestContext, f *harness.SourceReplacementFixture) {
	if f == nil || f.Replace == nil || f.SetMuted == nil || f.SetDisabled == nil || f.Snapshot == nil || f.SetContinuity == nil || f.PartialNext == nil || f.Effects == nil {
		must(c.NotRun("missing separately advertised source-replacement fixture and continuity/failure controls"))
	}
	oldOwner, newOwner := f.OldSource.Snapshot().Owner, f.Replacement.Snapshot().Owner
	value(f.SetMuted(ctx, true).Result(ctx))
	value(f.SetDisabled(ctx, true).Result(ctx))
	before := f.Effects()
	must(f.SetContinuity(ctx, "unknown"))
	_, err := f.Replace(ctx, f.Publication, f.Replacement.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx)
	portableFailure(c, err, openrtc.Unsupported)
	check(c, f.Effects() == before, "required unknown identity continuity crossed effects")
	must(f.SetContinuity(ctx, "established"))
	result := value(f.Replace(ctx, f.Publication, f.Replacement.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
	v := f.Snapshot()
	check(c, (result.Identity == "preserved" || result.Identity == "changed" || result.Identity == "unknown") && v.ID == result.Publication.ID && v.State == "published" && v.Source.Value != nil && *v.Source.Value == f.Replacement.Snapshot().ID, "replacement success preceded observable promised state")
	check(c, result.Identity == "preserved" && v.ID == f.Publication, "required identity continuity not preserved")
	check(c, knownTrue(v.LocalMuteIntent) && knownTrue(f.Replacement.Snapshot().SourceDisableIntent), "replacement cleared publication mute or source disable intent")
	check(c, f.OldSource.Snapshot().Owner == oldOwner && f.Replacement.Snapshot().Owner == newOwner && f.OldSource.Snapshot().State != "stopped" && f.Replacement.Snapshot().State != "stopped", "replacement reowned or stopped source without authorization")
	must(f.PartialNext(ctx))
	_, err = f.Replace(ctx, f.Publication, f.OldSource.Snapshot().ID, false, openrtc.OperationOptions{}).Result(ctx)
	failure := portableFailure(c, err)
	check(c, (failure.Outcome == "partial" || failure.Outcome == "unknown") && len(failure.Details.SurvivingResources) > 0 && len(failure.Details.UnknownEffects) > 0, "partial replacement omitted survivors or unknown effects")
	for _, source := range []openrtc.CaptureSource{f.OldSource, f.Replacement} {
		found := false
		for _, resource := range failure.Details.SurvivingResources {
			found = found || resource.ID == string(source.Snapshot().ID)
		}
		check(c, found, "replacement failed to identify known surviving source")
	}
}

func sameStringSet(a, b []string) bool {
	a = append([]string{}, a...)
	b = append([]string{}, b...)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
func sameEndpoints(a, b []openrtc.ProviderIdentity) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		found := false
		for _, w := range b {
			found = found || reflect.DeepEqual(v, w)
		}
		if !found {
			return false
		}
	}
	return true
}
func sameEncryptionBoundary(a, b openrtc.EncryptionBoundary) bool {
	if a.Sources.Kind != b.Sources.Kind || !sameStringSet(a.Services, b.Services) || !sameStringSet(a.Exclusions, b.Exclusions) || len(a.Sources.IDs) != len(b.Sources.IDs) {
		return false
	}
	for _, id := range a.Sources.IDs {
		if !slices.Contains(b.Sources.IDs, id) {
			return false
		}
	}
	return true
}
