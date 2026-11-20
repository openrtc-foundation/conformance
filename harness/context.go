package harness

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/openrtc/conformance/openrtc"
	"reflect"
	"strings"
	"sync/atomic"
)

var identitySequence atomic.Uint64
var runID = func() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "run"
	}
	return hex.EncodeToString(b[:])
}()

func NewTestContext(target ConformanceTarget, descriptor openrtc.ProviderDescriptor) *TestContext {
	return &TestContext{Target: target, Descriptor: descriptor, Spy: NewCredentialSpy(target.Credentials())}
}
func (c *TestContext) RequireControl(name string) (any, error) {
	x := c.Controls()
	m := map[string]any{"gateNextJoin": x.GateNextJoin, "interruptTransport": x.InterruptTransport, "triggerCredentialRenewal": x.TriggerCredentialRenewal, "failNextNativeOperation": x.FailNextNativeOperation, "remoteEndpoint": x.RemoteEndpoint, "admission": x.Admission, "authority": x.Authority, "incomingCall": x.IncomingCall, "enforceablePlacement": x.EnforceablePlacement}
	v, ok := m[name]
	if !ok || reflect.ValueOf(v).IsNil() {
		return nil, c.NotRun(fmt.Sprintf("fixture provides no '%s' control", name))
	}
	return v, nil
}
func WithinValue[T any](ctx context.Context, c *TestContext, label string, ms int, action func(context.Context) (T, error)) (T, error) {
	type result struct {
		v T
		e error
	}
	ch := make(chan result, 1)
	err := c.Within(ctx, label, ms, func(ctx context.Context) error { v, e := action(ctx); ch <- result{v, e}; return nil })
	if err != nil {
		var zero T
		return zero, err
	}
	r := <-ch
	return r.v, r.e
}
func (c *TestContext) Capture(ctx context.Context, action func(context.Context) (any, error)) Captured[any] {
	v, e := WithinValue(ctx, c, "captured action", 0, action)
	if e != nil {
		return Captured[any]{Error: e}
	}
	return Captured[any]{OK: true, Value: v}
}
func (c *TestContext) Rejects(ctx context.Context, action func(context.Context) (any, error), expected ExpectedFailure, label string) (*openrtc.RTCFailure, error) {
	r := c.Capture(ctx, action)
	if r.OK {
		return nil, c.Fail(label + ": expected a rejection, but the operation succeeded")
	}
	return c.ExpectError(r.Error, expected, label)
}
func (c *TestContext) ExpectError(value any, expected ExpectedFailure, label string) (*openrtc.RTCFailure, error) {
	var e *openrtc.RTCFailure
	switch v := value.(type) {
	case *openrtc.RTCFailure:
		e = v
	case *openrtc.RTCOperationError:
		if v != nil {
			e = v.RTCFailure
		}
	case *openrtc.RTCObservationError:
		if v != nil {
			e = v.RTCFailure
		}
	}
	if e == nil {
		return nil, c.Fail(fmt.Sprintf("%s: failure is not an RTCError (%v); ERR-001", label, value))
	}
	// RFC §9:529–570 and A.1:1220–1227: correlation is optional;
	// safe message, current code, reasons and outcome are mandatory.
	if e.Message == "" || !openrtc.IsCoreCode(e.Code) || len(e.Reasons) == 0 || !contains([]string{"none", "partial", "unknown"}, e.Outcome) {
		return nil, c.Fail(label + ": safe error lacks current code, message, reasons or outcome (RFC §9:529–570)")
	}
	for _, rejection := range e.Details.RejectedConditions {
		if rejection.Condition.Name == "" || !contains([]string{"unsupported", "unknown", "pending"}, rejection.Assessment) {
			return nil, c.Fail(label + ": missing rejected condition assessment")
		}
		if rejection.Assessment == "pending" && (rejection.Resolution == nil || rejection.Resolution.Mechanism == "" || rejection.Resolution.EvaluationBoundary == "") {
			return nil, c.Fail(label + ": pending condition lacks resolution mechanism/boundary")
		}
	}
	expectedCodes := make([]openrtc.ErrorCode, len(expected.Codes))
	for i, code := range expected.Codes {
		expectedCodes[i] = openrtc.CoreCode(code)
	}

	if expected.Codes != nil && !contains(expectedCodes, e.Code) {
		codes := make([]string, len(expected.Codes))
		for i, v := range expected.Codes {
			codes[i] = string(v)
		}
		return nil, c.Fail(fmt.Sprintf("%s: expected code %s, observed %s", label, strings.Join(codes, " | "), e.Code))
	}
	if expected.Kind != "" && e.Kind != expected.Kind {
		return nil, c.Fail(fmt.Sprintf("%s: expected a %s error, observed %s; ERR-003", label, expected.Kind, e.Kind))
	}
	return e, nil
}
func (c *TestContext) FreshIdentity(label string) openrtc.ApplicationIdentity {
	return openrtc.ApplicationIdentity{Namespace: "openrtc-conformance", Subject: fmt.Sprintf("%s-%s-%d", label, runID, identitySequence.Add(1))}
}
func (c *TestContext) Provider(ctx context.Context, config *openrtc.ProviderConfig) (openrtc.RTCProvider, error) {
	t := c.Target
	if direct, ok := t.(PublicBindingTarget); ok {
		bridge := direct.PublicBinding().ProviderBridge
		if bridge == nil {
			return nil, c.NotRun("direct binding has no harness-only Provider bridge for this legacy case")
		}
		cfg := t.ProviderConfig()
		if config != nil {
			cfg = *config
		}
		p, err := bridge(ctx, cfg)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, c.Fail("public binding bridge returned nil Provider")
		}
		c.Track("mapped provider", func(ctx context.Context) error { _, err := p.Dispose(ctx).Result(ctx); return err })
		return p, nil
	}
	var e error
	r := t.CreateRegistry()
	a := t.CreateAdapter()
	if _, e = r.Register(a); e != nil {
		return nil, e
	}
	cfg := t.ProviderConfig()
	if config != nil {
		cfg = *config
	}
	p, e := WithinValue(ctx, c, "provider initialization", 0, func(ctx context.Context) (openrtc.RTCProvider, error) {
		return r.Initialize(ctx, a.Descriptor().AdapterID, cfg).Result(ctx)
	})
	if e != nil {
		return nil, e
	}
	c.Track("provider", func(ctx context.Context) error { _, e := p.Dispose(ctx).Result(ctx); return e })
	return p, nil
}

type ClientOptions struct {
	Provider openrtc.RTCProvider
	Config   *openrtc.ClientConfig
	Identity *openrtc.ApplicationIdentity
	Renew    *bool
}

func (c *TestContext) ClientConfig(o ClientOptions) (openrtc.ClientConfig, error) {
	t := c.Target
	var id openrtc.ApplicationIdentity
	if o.Identity != nil {
		id = *o.Identity
	} else {
		id = c.FreshIdentity("local")
	}
	renew := true
	if o.Renew != nil {
		renew = *o.Renew
	}
	if c.Spy == nil {
		c.Spy = NewCredentialSpy(t.Credentials())
	}
	return t.ClientConfig(ClientConfigInput{Identity: id, Credentials: c.Spy.Provider(renew)}), nil
}
func (c *TestContext) Client(ctx context.Context, o ClientOptions) (openrtc.RTCClient, error) {
	p := o.Provider
	var e error
	if p == nil {
		p, e = c.Provider(ctx, nil)
		if e != nil {
			return nil, e
		}
	}
	var cfg openrtc.ClientConfig
	if o.Config != nil {
		cfg = *o.Config
	} else {
		cfg, e = c.ClientConfig(o)
		if e != nil {
			return nil, e
		}
	}
	client, e := WithinValue(ctx, c, "client creation", 0, func(ctx context.Context) (openrtc.RTCClient, error) { return p.CreateClient(ctx, cfg).Result(ctx) })
	if e != nil {
		return nil, e
	}
	c.Track("client", func(ctx context.Context) error { _, e := client.Dispose(ctx).Result(ctx); return e })
	return client, nil
}
func (c *TestContext) Conference(ctx context.Context, modes ...ProvisioningModeName) (openrtc.ConferenceTarget, error) {
	mode := ExternalProvisioned
	if len(modes) > 0 {
		mode = modes[0]
	}
	t := c.Target
	target, e := t.ConferenceTarget(ctx, mode)
	if e != nil {
		return openrtc.ConferenceTarget{}, e
	}
	if target == nil {
		return openrtc.ConferenceTarget{}, c.NotRun(fmt.Sprintf("fixture cannot produce a '%s' conference target", mode))
	}
	return *target, nil
}
func (c *TestContext) Open(client openrtc.RTCClient, target openrtc.ConferenceTarget) openrtc.RTCConferenceSession {
	s := client.OpenSession(target)
	c.Track("session", func(ctx context.Context) error { _, e := s.Dispose(ctx).Result(ctx); return e })
	return s
}

type JoinOptionsOverride struct {
	Participant  *openrtc.ParticipantJoinIntent
	InitialMedia **openrtc.InitialMediaIntent
	Receive      **openrtc.ReceivePolicyRequest
	Placement    **openrtc.PlacementPolicy
	Requirements *[]openrtc.CapabilityRequest
	Signal       *context.Context
	Extensions   *openrtc.ProviderExtensions
}

func (c *TestContext) JoinOptions(o JoinOptionsOverride) openrtc.JoinSessionOptions {
	var b openrtc.JoinSessionOptions
	if t, ok := c.Target.(JoinOptionsTarget); ok {
		b = t.JoinOptions()
	}
	if o.Participant != nil {
		b.Participant = *o.Participant
	}
	if o.InitialMedia != nil {
		b.InitialMedia = *o.InitialMedia
	}
	if o.Receive != nil {
		b.Receive = *o.Receive
	}
	if o.Placement != nil {
		b.Placement = *o.Placement
	}
	if o.Requirements != nil {
		b.Requirements = *o.Requirements
	}
	if o.Signal != nil {
		b.Signal = *o.Signal
	}
	if o.Extensions != nil {
		b.Extensions = *o.Extensions
	}
	return b
}

type JoinedOptions struct {
	Identity *openrtc.ApplicationIdentity
	Join     JoinOptionsOverride
	Target   *openrtc.ConferenceTarget
	Client   openrtc.RTCClient
}
type JoinedResult struct {
	Client  openrtc.RTCClient
	Session openrtc.RTCConferenceSession
	Result  openrtc.JoinResult
}

func (c *TestContext) Joined(ctx context.Context, o JoinedOptions) (JoinedResult, error) {
	var out JoinedResult
	var e error
	out.Client = o.Client
	if out.Client == nil {
		out.Client, e = c.Client(ctx, ClientOptions{Identity: o.Identity})
		if e != nil {
			return out, e
		}
	}
	var target openrtc.ConferenceTarget
	if o.Target != nil {
		target = *o.Target
	} else {
		target, e = c.Conference(ctx)
		if e != nil {
			return out, e
		}
	}
	out.Session = c.Open(out.Client, target)
	attempt := out.Session.Join(c.JoinOptions(o.Join))
	out.Result, e = WithinValue(ctx, c, "join result", 0, func(arg0 context.Context) (openrtc.JoinResult, error) { return attempt.Result(arg0).Result(arg0) })
	return out, e
}
func (c *TestContext) JoinWith(ctx context.Context, client openrtc.RTCClient, target *openrtc.ConferenceTarget) (JoinedResult, error) {
	return c.Joined(ctx, JoinedOptions{Client: client, Target: target})
}
func (c *TestContext) Administration(ctx context.Context) (openrtc.RTCServerAdministration, error) {
	t, ok := c.Target.(AdministrationTarget)
	if !ok {
		return nil, c.NotRun("fixture provides no trusted administration")
	}
	a, e := WithinValue(ctx, c, "administration", 0, t.Administration)
	if e != nil {
		return nil, e
	}
	c.Track("administration", func(ctx context.Context) error { _, e := a.Dispose(ctx).Result(ctx); return e })
	return a, nil
}
func (c *TestContext) Record(session openrtc.RTCSession) *EventRecorder {
	r := NewEventRecorder(session)
	c.Track("event recorder", func(context.Context) error { r.Stop(); return nil })
	return r
}
func (c *TestContext) DescriptorFor(snapshot openrtc.CapabilitySnapshot, id openrtc.CapabilityID) *openrtc.CapabilityDescriptor {
	for i := range snapshot.Entries {
		if snapshot.Entries[i].ID == id {
			return &snapshot.Entries[i]
		}
	}
	return nil
}
func (c *TestContext) Remote(ctx context.Context, target openrtc.ConferenceTarget, label string) (RemoteEndpoint, error) {
	v, e := c.RequireControl("remoteEndpoint")
	if e != nil {
		return nil, e
	}
	control := v.(func(context.Context, openrtc.ConferenceTarget, RemoteEndpointIntent) (RemoteEndpoint, error))
	id := c.FreshIdentity(label)
	endpoint, e := WithinValue(ctx, c, "remote endpoint", 0, func(ctx context.Context) (RemoteEndpoint, error) {
		return control(ctx, target, RemoteEndpointIntent{Identity: id, DisplayName: &label})
	})
	if e != nil {
		return nil, e
	}
	c.Track("remote "+label, endpoint.Leave)
	return endpoint, nil
}
