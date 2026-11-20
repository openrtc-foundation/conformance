package selftest

import (
	"context"
	"fmt"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type fakeStatistics struct{ owner *fakeSession }

var _ openrtc.MediaStatistics = (*fakeStatistics)(nil)

func (receiver fakeStatistics) Sample(ctx context.Context, target openrtc.StatisticsTarget) openrtc.Async[openrtc.StatsSample] {
	return startOperation(receiver, func() (openrtc.StatsSample, error) {
		if err := receiver.owner.world.awaitPending(ctx, receiver.owner); err != nil {
			return openrtc.StatsSample{}, err
		}
		return openrtc.StatsSample{Target: target, Generation: receiver.owner.generation, CollectedAt: receiver.owner.world.clock(), Source: "adapter", Interval: openrtc.Unknown[openrtc.Duration](), SourceTimestamp: openrtc.Unknown[openrtc.Instant](), Coverage: "partial", Measurements: []openrtc.Measurement{{Metric: "payload-bitrate", Value: openrtc.KnownValue(float64(0)), Direction: "send", Target: target, Layer: openrtc.Unknown[string](), Provenance: "fake synthetic sample", Boundary: "transport", Interval: openrtc.Unknown[openrtc.Duration](), IncludedStreams: openrtc.Unknown[[]openrtc.ScopedNativeID]()}}, Native: openrtc.NotApplicable[openrtc.NativeStatistics]("fake has no native statistics")}, nil

	})
}

type fakeMessaging struct{}

var _ openrtc.MessagingExtension = (*fakeMessaging)(nil)

func (receiver fakeMessaging) Send(_ context.Context, message openrtc.EphemeralMessage) openrtc.Async[openrtc.MessageReceipt] {
	return startOperation(receiver, func() (openrtc.MessageReceipt, error) {
		if message.Reliability != "best-effort" {
			return openrtc.MessageReceipt{}, fail("unsupported-capability", "message", "fake supports only best-effort reliability", failureExtras{})
		}
		if len(message.Payload) > 16384 {
			return openrtc.MessageReceipt{}, fail("invalid-option", "message", "payload too large", failureExtras{})
		}
		return openrtc.MessageReceipt{Status: "local-queued", ByteLength: uint64(len(message.Payload))}, nil
	})
}
func (fakeMessaging) OnMessage(func(openrtc.ReceivedEphemeralMessage)) openrtc.Unsubscribe {
	return func() {}
}
func (s *fakeSession) Extension(key openrtc.ExtensionKey) any {
	switch key {
	case "adapter:harness-fake/diagnostics":
		s.media.mu.Lock()
		defer s.media.mu.Unlock()
		payload := map[string]any{"schemaId": "harness-fake/diagnostics-v1", "status": "redacted"}
		if len(s.media.canaries) > 0 && s.defects.LeakExtensionCanary {
			payload["status"] = s.media.canaries[0]
		}
		return payload
	case "independent-publication":
		s.media.mu.Lock()
		enabled := s.media.enabled
		s.media.mu.Unlock()
		if enabled {
			return independentMedia{s.media}
		}
	case "statistics":
		return fakeStatistics{owner: s}
	case "messaging":
		return fakeMessaging{}
	}
	if s.defects.FakeExtensionForUnsupported {
		return map[string]any{}
	}
	panic(fail("unsupported-capability", "extension", fmt.Sprintf("extension '%s' is not implemented by the fake", key), failureExtras{}))
}

type fakeDevices struct{ owner *fakeClient }

var _ openrtc.MediaDeviceManager = (*fakeDevices)(nil)

func (receiver fakeDevices) Enumerate(context.Context) openrtc.Async[openrtc.DeviceInventory] {
	return startOperation(receiver, func() (openrtc.DeviceInventory, error) {
		return receiver.Snapshot().Inventory, nil

	})
}
func (receiver fakeDevices) Select(context.Context, openrtc.DeviceSelection) openrtc.Async[openrtc.Void] {
	return startOperation(receiver, func() (openrtc.Void, error) {
		return openrtc.Void{}, fail("unsupported-capability", "extension", "selection not implemented", failureExtras{})

	})
}
func (c *fakeClient) Extension(key openrtc.ExtensionKey) any {
	if key == "devices" {
		return fakeDevices{owner: c}
	}
	if c.defects.FakeExtensionForUnsupported {
		return map[string]any{}
	}
	panic(fail("unsupported-capability", "extension", fmt.Sprintf("client extension '%s' is not implemented by the fake", key), failureExtras{}))
}

type fakeAdministration struct {
	world        *world
	capabilities *fakeCapabilities
}

var _ openrtc.RTCServerAdministration = (*fakeAdministration)(nil)
var _ openrtc.SessionProvisioner = (*fakeAdministration)(nil)
var _ openrtc.OpaqueCredentialMaterial = fakeMaterial(nil)
var _ openrtc.SessionBroker = fakeBroker{}

func makeAdministration(w *world) *fakeAdministration {
	return &fakeAdministration{world: w, capabilities: newFakeCapabilities(NoDefects)}
}
func (a *fakeAdministration) Capabilities() openrtc.RTCCapabilities   { return a.capabilities }
func (a *fakeAdministration) Provisioner() openrtc.SessionProvisioner { return a }
func (a *fakeAdministration) Create(_ context.Context, options openrtc.CreateSessionOptions) openrtc.Async[openrtc.SessionDefinition] {
	return startOperation(a, func() (openrtc.SessionDefinition, error) {
		if _, err := validateHolder("session-create", options, NoDefects, "provision"); err != nil {
			return openrtc.SessionDefinition{}, err
		}
		a.world.mu.Lock()
		a.world.counts.ProvisioningCalls++
		a.world.mu.Unlock()
		id := openrtc.SessionDefinitionID(newID("definition"))
		name := fmt.Sprintf("room-%d", counter.Load())
		if options.Name != nil {
			name = *options.Name
		}
		return openrtc.SessionDefinition{ID: id, Address: openrtc.SessionAddress{AdapterID: adapterID, Namespace: "fake", Value: name, Deployment: openrtc.Unknown[openrtc.DeploymentID]()}, Allocation: "resource", NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), Policy: openrtc.KnownValue(openrtc.SessionPolicy{})}, nil

	})
}
func (*fakeAdministration) Extension(key openrtc.ExtensionKey) any {
	panic(fail("unsupported-capability", "extension", fmt.Sprintf("server extension '%s' is not implemented by the fake", key), failureExtras{}))
}
func (receiver *fakeAdministration) Dispose(context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(receiver, func() (openrtc.CleanupReport, error) {
		return openrtc.CleanupReport{Status: openrtc.CleanupStatus{Kind: "complete"}, Generation: openrtc.Unknown[openrtc.GenerationID]()}, nil

	}))
}
func (t *FakeTarget) Administration(context.Context) (openrtc.RTCServerAdministration, error) {
	return makeAdministration(t.world), nil
}
func (*FakeTarget) ClientConfig(input harness.ClientConfigInput) openrtc.ClientConfig {
	return openrtc.ClientConfig{Identity: input.Identity, Credentials: input.Credentials}
}

type fakeMaterial []byte

func (m fakeMaterial) Use(consumer func([]byte) any) any { return consumer([]byte(m)) }
func credential(request openrtc.CredentialRequest, text string) openrtc.CredentialInput {
	return openrtc.CredentialInput{Material: fakeMaterial(text), Metadata: openrtc.CredentialInputMetadata{Audience: request.Audience, Scope: request.Scope, Subject: openrtc.Unknown[openrtc.ProviderIdentity](), IssuedAt: openrtc.Unknown[openrtc.Instant](), ExpiresAt: openrtc.Unknown[openrtc.Instant]()}}
}
func (t *FakeTarget) Credentials() harness.CredentialSource {
	return harness.CredentialSource{Valid: func(_ context.Context, r openrtc.CredentialRequest) (openrtc.CredentialInput, error) {
		t.world.mu.Lock()
		t.world.tokenCounter++
		n := t.world.tokenCounter
		t.world.mu.Unlock()
		t.world.mu.Lock()
		canaries := append([]string(nil), t.world.canaries...)
		subject := t.world.canarySubject
		t.world.mu.Unlock()
		if len(canaries) > 0 && r.Subject == subject {
			return credential(r, "fake-token-"+canaries[0]), nil
		}
		return credential(r, fmt.Sprintf("fake-token-%d-abcdef", n)), nil
	}, Invalid: func(_ context.Context, r openrtc.CredentialRequest) (openrtc.CredentialInput, error) {
		return credential(r, "invalid-token-abcdef"), nil
	}}
}

type fakeBroker struct{ address openrtc.SessionAddress }

func (b fakeBroker) Resolve(context.Context, openrtc.BrokerSessionRequest) openrtc.Async[openrtc.ResolvedSessionTarget] {
	return startOperation(b, func() (openrtc.ResolvedSessionTarget, error) {
		unknown := openrtc.Unknown[openrtc.SessionDefinitionDescriptor]()
		return openrtc.ResolvedSessionTarget{Mode: "external-provisioned", Address: &b.address, KnownDefinition: &unknown}, nil

	})
}
func (t *FakeTarget) ConferenceTarget(ctx context.Context, mode harness.ProvisioningModeName) (*openrtc.ConferenceTarget, error) {
	address := openrtc.SessionAddress{AdapterID: adapterID, Namespace: "fake", Value: "room-selftest", Deployment: openrtc.Unknown[openrtc.DeploymentID]()}
	p := openrtc.SessionProvisioning{Mode: string(mode)}
	switch mode {
	case harness.ExternalProvisioned:
		u := openrtc.Unknown[openrtc.SessionDefinitionID]()
		p.Address = &address
		p.DefinitionID = &u
	case harness.CreateOnJoin:
		u := openrtc.Unknown[openrtc.SessionPolicy]()
		p.Address = &address
		p.CreationPolicy = &u
	case harness.ExplicitCreate:
		name := fmt.Sprintf("room-%d", counter.Load())
		d, err := makeAdministration(t.world).Create(ctx, openrtc.CreateSessionOptions{Name: &name}).Result(ctx)
		if err != nil {
			return nil, err
		}
		p.Definition = &d
	default:
		p.Broker = fakeBroker{address}
		p.Request = &openrtc.BrokerSessionRequest{Application: openrtc.ApplicationIdentity{Namespace: "selftest", Subject: "broker"}, Resource: "selftest"}
	}
	return &openrtc.ConferenceTarget{Kind: "conference", Provisioning: p}, nil
}

func (d fakeDevices) Snapshot() openrtc.DeviceSnapshot {
	return openrtc.DeviceSnapshot{Revision: 1, Inventory: openrtc.DeviceInventory{Entries: []openrtc.DeviceView{{ID: openrtc.DeviceID(string(d.owner.id) + "-input"), Direction: "input", Kind: "audio", Label: openrtc.Unknown[string](), NativeID: openrtc.Unknown[openrtc.ScopedNativeID]()}}, Coverage: "complete"}, Selections: []openrtc.DeviceSelection{}, Permissions: map[openrtc.MediaKind]openrtc.Known[string]{"audio": openrtc.Unknown[string](), "video": openrtc.Unknown[string]()}, Freshness: openrtc.SourceFreshness{Owner: openrtc.LocalOwnerID(d.owner.id), Source: "fake", Coverage: []openrtc.FreshnessCoverage{{Kind: "facet", Facet: "devices"}}, ObservedAt: openrtc.KnownValue(d.owner.world.clock()), Status: "current", Reason: "synthetic inventory"}}
}
func (d fakeDevices) Watch(f func(openrtc.DeviceSnapshot)) openrtc.Unsubscribe {
	f(d.Snapshot())
	return func() {}
}
