package selftest

import (
	"context"
	"testing"
	"time"

	"github.com/openrtc/conformance/openrtc"
)

type blockingCredentials struct{ entered, release chan struct{} }

func (p blockingCredentials) Get(context.Context, openrtc.CredentialRequest) openrtc.Async[openrtc.CredentialInput] {
	return startOperation(p, func() (openrtc.CredentialInput, error) {
		close(p.entered)
		<-p.release
		return openrtc.CredentialInput{}, nil

	})
}

func TestDefectiveOpenDoesNotBlockOnCredentials(t *testing.T) {
	p := blockingCredentials{make(chan struct{}), make(chan struct{})}
	defer close(p.release)
	c := newFakeClient(openrtc.ClientConfig{Credentials: p}, newWorld(), FakeDefects{OpenSessionRequestsCredentials: true})
	opened := make(chan struct{})
	go func() { c.OpenSession(unitConferenceTarget()); close(opened) }()
	for _, ch := range []<-chan struct{}{p.entered, opened} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("opening or credential dispatch blocked")
		}
	}
}

type descriptorAdapter struct {
	bindingAdapter
	descriptor openrtc.ProviderDescriptor
}

func (a descriptorAdapter) Descriptor() openrtc.ProviderDescriptor { return a.descriptor }
func TestBindingRegistryAndLifecycle(t *testing.T) {
	ctx := context.Background()
	w := newWorld()
	r := makeRegistry(w, NoDefects)
	a := bindingAdapter{}
	if _, err := r.Register(a); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register(a); err == nil {
		t.Fatal("duplicate adapter accepted")
	}
	for _, change := range []func(*openrtc.ProviderDescriptor){func(d *openrtc.ProviderDescriptor) { d.AdapterID = " " }, func(d *openrtc.ProviderDescriptor) { d.Specification.MinimumInclusive.Patch++ }, func(d *openrtc.ProviderDescriptor) { d.Specification.MaximumExclusive = hostSpec }} {
		d := a.Descriptor()
		change(&d)
		if _, err := r.Register(descriptorAdapter{descriptor: d}); err == nil {
			t.Fatal("invalid adapter accepted")
		}
	}
	if _, err := r.Initialize(ctx, "missing", openrtc.ProviderConfig{}).Result(ctx); err == nil {
		t.Fatal("missing adapter accepted")
	}
	p, err := r.Initialize(ctx, adapterID, openrtc.ProviderConfig{}).Result(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c, err := p.CreateClient(ctx, openrtc.ClientConfig{}).Result(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if w.counts.ClientsCreated != 1 || len(p.Schemas(openrtc.ClientInit)) != 1 {
		t.Fatal("client count or schemas incorrect")
	}
	s := c.OpenSession(unitConferenceTarget())
	if c.Sessions()[s.ID()] != s {
		t.Fatal("session missing")
	}
	op := c.Configure(ctx, openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "debug"}})
	_, err = op.Result(ctx)
	record, _ := c.Inspection().Operation(op.OperationID())
	result := record.Details.Configuration.Value.Outcome
	if err != nil || len(result.Fields) != 1 || result.Fields[0].Requested.Value == nil || *result.Fields[0].Requested.Value != "debug" {
		t.Fatalf("configuration: %+v %v", result, err)
	}
	if _, err := cleanupAfter(ctx, p.Inspection(), p.Dispose(ctx)); err != nil {
		t.Fatal(err)
	}
	if !s.(*fakeSession).disposed {
		t.Fatal("session was not disposed")
	}
}

func unitConferenceTarget() openrtc.ConferenceTarget {
	return openrtc.ConferenceTarget{Provisioning: openrtc.SessionProvisioning{Mode: "external-provisioned", Address: &openrtc.SessionAddress{AdapterID: adapterID, Namespace: "fake", Value: "unit-room"}}}
}
