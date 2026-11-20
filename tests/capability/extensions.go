package capability

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func test11(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	m := openrtc.SessionExtension(s, openrtc.MessagingKey)
	payload := []byte("conformance")
	receipt := within(ctx, c, "send", func(ctx context.Context) (openrtc.MessageReceipt, error) {
		return m.Send(ctx, openrtc.EphemeralMessage{Payload: payload, Recipients: openrtc.MessageRecipients{Kind: "session"}, Reliability: "best-effort"}).Result(ctx)
	})
	check(c, slices.Contains([]string{"local-queued", "provider-accepted"}, receipt.Status), "receipt claims more than local/provider acceptance")
	check(c, receipt.ByteLength == uint64(len(payload)), "receipt byte length differs from the application payload")
}

type testKeySource []byte

func (k testKeySource) Use(f func([]byte) any) any { return f([]byte(k)) }
func (k testKeySource) Get(context.Context, openrtc.EncryptionKeyRequest) openrtc.Async[openrtc.OpaqueKeyMaterial] {
	return openrtc.StartOperation(k, func() (openrtc.OpaqueKeyMaterial, error) { return k, nil })
}
func test12(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	enc := openrtc.SessionExtension(s, openrtc.EncryptionKey)
	own := enc.Coverage()
	if own.Mode.Value != nil && *own.Mode.Value == "end-to-end" {
		check(c, len(own.Evidence) > 0, "end-to-end coverage without evidence")
		check(c, len(own.TrustedEndpoints) > 0, "end-to-end coverage names no trusted endpoints")
	}
	marker := "conformance-media-key-5e7a19"
	request := openrtc.EncryptionRequest{Mode: "end-to-end", Directions: own.Directions, MediaKinds: own.MediaKinds, TrustedEndpoints: own.TrustedEndpoints, Boundary: own.Boundary, Keys: testKeySource(marker)}
	r := c.Capture(ctx, func(ctx context.Context) (any, error) { return enc.Configure(ctx, request).Result(ctx) })
	if !r.OK {
		_, e := c.ExpectError(r.Error, harness.ExpectedFailure{}, "encryption configure")
		must(e)
		must(c.Fail("claimed encryption profile defining positive rejected (RFC §7.0:65, §11.8:877–884)"))
	} else {
		observed := enc.Coverage()
		if observed.Effective.Value != nil && *observed.Effective.Value {
			check(c, observed.Mode.Value != nil && *observed.Mode.Value == "end-to-end", "effective encryption coverage is weaker than requested")
		} else {
			c.Partial("effective encryption coverage has not been observed")
		}
	}
	for _, surface := range []any{s.Observation().Snapshot(), s.Capabilities().Snapshot(), enc.Coverage()} {
		check(c, harness.FindLeak(surface, []string{marker}) == nil, "key material appeared in an observable surface")
	}
}
func instanceID(c *harness.TestContext, s openrtc.RTCSession) openrtc.SessionInstanceID {
	i := s.Connection().Snapshot().Instance
	if i.Status != "known" || i.Value == nil || i.Value.ID == "" {
		must(c.Unknown("the instance ID is not known, so end() cannot be aimed"))
	}
	return i.Value.ID
}
func test13(ctx context.Context, c *harness.TestContext) {
	target := value(c.Conference(ctx))
	remote := value(c.Remote(ctx, target, "witness"))
	s := value(c.Joined(ctx, harness.JoinedOptions{Target: &target})).Session
	d := c.DescriptorFor(s.Capabilities().Snapshot(), "session.end")
	if d != nil && d.Authorization.Decision == "allowed" {
		must(c.NotApplicable("the client is authorized for administration; denial cannot be tested"))
	}
	admin := openrtc.SessionExtension(s, openrtc.SessionAdministrationKey)
	id := instanceID(c, s)
	_, e := c.Rejects(ctx, func(ctx context.Context) (any, error) {
		plan, err := admin.Plan(ctx, openrtc.AdministrationRequest{Command: "end", Instance: id}).Result(ctx)
		if err != nil {
			return nil, err
		}
		return admin.Execute(ctx, plan).Result(ctx)
	}, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"authorization-denied", "unsupported-capability"}}, "end without authority")
	must(e)
	must(c.Settle(ctx))
	check(c, remote.IsConnected(), "an unauthorized end() ended the session for others")
}
func test14(ctx context.Context, c *harness.TestContext) {
	admin := value(c.Administration(ctx))
	target := value(c.Conference(ctx))
	remote := value(c.Remote(ctx, target, "ended-by-admin"))
	s := value(c.Joined(ctx, harness.JoinedOptions{Target: &target})).Session
	id := instanceID(c, s)
	out := within(ctx, c, "end", func(ctx context.Context) (openrtc.AdminOutcome, error) {
		extension := openrtc.ServerExtension(admin, openrtc.ServerAdministrationKey)
		plan, err := extension.Plan(ctx, openrtc.AdministrationRequest{Command: "end", Instance: id}).Result(ctx)
		if err != nil {
			return openrtc.AdminOutcome{}, err
		}
		for _, effect := range append(plan.RequiredEffects, plan.CollateralEffects...) {
			check(c, effect.Kind != "definition-deleted", "end plan deletes the definition")
		}
		return extension.Execute(ctx, plan).Result(ctx)
	})
	check(c, slices.Contains([]string{"accepted", "confirmed", "unknown"}, out.Status), "invalid admin outcome")
	ended := false
	for _, e := range out.ObservedEffects {
		check(c, e.Kind != "definition-deleted", "end-instance reported the definition deleted (SES-005)")
		if e.Kind == "instance-ended" {
			ended = true
		}
	}
	if out.Status == "confirmed" {
		check(c, ended, "confirmed without instance-ended")
		must(c.WaitFor(ctx, func() bool { return !remote.IsConnected() }, "remote endpoint to be disconnected", 0))
	} else {
		c.Partial(fmt.Sprintf("end() outcome was '%s'; the ending was not confirmed (D-09)", out.Status))
	}
}
func firstLeg(call openrtc.RTCCallSession) openrtc.CallLegHandle {
	for _, leg := range call.Legs() {
		return leg
	}
	return nil
}
func noJoin(call openrtc.RTCCallSession) bool {
	_, ok := any(call).(interface {
		Join(openrtc.JoinSessionOptions) openrtc.JoinAttempt
	})
	return !ok
}
func test15(ctx context.Context, c *harness.TestContext) {
	_, call := dial(ctx, c)
	check(c, call.Kind() == "call", "dial did not return a call session")
	check(c, noJoin(call), "call session exposes a conference join (EXT-105)")
	check(c, call.Observation().Snapshot().Roster.Completeness == "not-applicable", "call roster is not not-applicable (CORE-004)")
	leg := firstLeg(call)
	if leg == nil {
		must(c.Fail("an established dial created no call leg"))
	}
	out := within(ctx, c, "terminate", func(arg0 context.Context) (openrtc.CallOutcome, error) { return leg.Terminate(arg0).Result(arg0) })
	check(c, slices.Contains([]string{"accepted", "confirmed", "failed", "unknown"}, out.Status), "invalid outcome status")
	must(c.WaitFor(ctx, func() bool { return leg.Snapshot().State == "ended" || leg.Snapshot().State == "failed" }, "leg to end", 0))
	within(ctx, c, "repeat hangup", func(ctx context.Context) (openrtc.CallOutcome, error) { return leg.Terminate(ctx).Result(ctx) })
	check(c, leg.Snapshot().State == "ended" || leg.Snapshot().State == "failed", "repeat hangup revived call (RFC §11.1:626–630)")
}
func test16(ctx context.Context, c *harness.TestContext) {
	trigger := value(c.RequireControl("incomingCall")).(func(context.Context, openrtc.RTCClient) (harness.IncomingCallFixture, error))
	client := value(c.Client(ctx, harness.ClientOptions{}))
	calling := openrtc.ClientExtension(client, openrtc.CallingKey)
	var mu sync.Mutex
	var received openrtc.RTCCallSession
	stop := calling.On("incomingCall", func(event openrtc.EventEnvelope[any]) {
		mu.Lock()
		defer mu.Unlock()
		received, _ = event.Payload.(openrtc.RTCCallSession)
	})
	c.Track("incomingCall listener", func(context.Context) error { stop(); return nil })
	fixture := within(ctx, c, "incoming call", func(ctx context.Context) (harness.IncomingCallFixture, error) { return trigger(ctx, client) })
	c.Track("caller", fixture.HangUp)
	get := func() openrtc.RTCCallSession { mu.Lock(); defer mu.Unlock(); return received }
	must(c.WaitFor(ctx, func() bool { return get() != nil }, "incomingCall event", 0))
	call := get()
	if call == nil {
		must(c.Fail("incomingCall never delivered"))
	}
	leg := firstLeg(call)
	state := "undefined"
	if leg != nil {
		state = string(leg.Snapshot().State)
	}
	check(c, leg != nil && leg.Snapshot().State == "incoming", fmt.Sprintf("leg state '%s' on delivery (STM-110)", state))
	reason := openrtc.RejectOptions{Reason: "declined"}
	out := within(ctx, c, "reject", func(ctx context.Context) (openrtc.CallOutcome, error) { return leg.Reject(ctx, &reason).Result(ctx) })
	check(c, slices.Contains([]string{"accepted", "confirmed", "failed", "unknown"}, out.Status), "invalid outcome status")
	check(c, noJoin(call), "incoming call exposes a conference join")
}

// receiveDtmf is synchronous in the binding; a thrown RTC error is represented by a panic.
func receiveDtmf(t openrtc.TelephonyControl, id openrtc.CallLegID) (result any, err error) {
	defer func() {
		if p := recover(); p != nil {
			if e, ok := p.(error); ok {
				err = e
			} else {
				panic(p)
			}
		}
	}()
	return t.ReceiveDtmf(func(openrtc.DtmfEvent) {}), nil
}
func test17(ctx context.Context, c *harness.TestContext) {
	client, call := dial(ctx, c)
	leg := firstLeg(call)
	if leg == nil {
		must(c.Fail("an established dial created no call leg"))
	}
	snapshot := client.Capabilities().Snapshot()
	unsupported := func(id openrtc.CapabilityID) bool {
		d := c.DescriptorFor(snapshot, id)
		return d != nil && (d.Support.Level == "unsupported" || d.Support.Level == "not-applicable")
	}
	t := leg.Telephony()
	judged := 0
	reject := func(label string, f func(context.Context) (any, error)) {
		judged++
		_, e := c.Rejects(ctx, f, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"unsupported-capability"}}, label)
		must(e)
	}
	if unsupported("telephony.dtmf.send") {
		reject("dtmf send", func(ctx context.Context) (any, error) {
			return t.SendDtmf(ctx, openrtc.DtmfRequest{Leg: leg.Snapshot().ID, Digits: "1", Method: "rtp-event", Duration: openrtc.Duration{Milliseconds: 100}, Gap: openrtc.Duration{Milliseconds: 100}}).Result(ctx)
		})
	}
	if unsupported("telephony.dtmf.receive") {
		reject("dtmf receive", func(context.Context) (any, error) { return receiveDtmf(t, leg.Snapshot().ID) })
	}
	if judged < 2 {
		c.Partial(fmt.Sprintf("%d telephony operation(s) advertised supported or unknown; positive behavior, authorization and revocation are not exercised (A-15, D-03)", 2-judged))
	}
}
func test18(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	views := s.Observation().Snapshot().Credentials
	check(c, len(views) > 0, "credential channel view missing")
	status := views[0]
	channels := s.Inspection().Snapshot().Discovery.CredentialChannels
	check(c, len(channels) > 0, "credential lifecycle discovery missing")
	lifecycle := channels[0].Lifecycle
	check(c, slices.Contains([]openrtc.CredentialState{"empty", "acquiring", "valid", "expiring", "renewing", "expired", "revoked", "failed", "disposed"}, status.State), "invalid credential state")
	check(c, slices.Contains([]string{"valid", "expired", "revoked", "unknown"}, status.Validity), "invalid validity")
	for _, effect := range []openrtc.ExpiryEffect{lifecycle.AdmissionExpiry, lifecycle.LiveExpiry, lifecycle.ReconnectExpiry, lifecycle.RevocationEffect} {
		check(c, slices.Contains([]openrtc.ExpiryEffect{"reject", "disconnect", "restrict-actions", "no-effect", "unknown", "not-applicable"}, effect), "invalid expiry effect")
	}
	events := c.Record(s).Of("credentialChanged")
	if len(events) > 0 {
		_, _ = events[len(events)-1].Envelope.Payload.(openrtc.StateChange[openrtc.CredentialView])
	}
}
