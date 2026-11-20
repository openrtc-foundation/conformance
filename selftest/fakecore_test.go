package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func TestFakeTargetSmoke(t *testing.T) {
	target := MakeFakeTarget(FakeDefects{})
	if target.Evidence() != harness.HarnessSelfTest {
		t.Fatal("incorrect evidence type")
	}
	registry := target.CreateRegistry()
	adapter := target.CreateAdapter()
	if _, err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	provider, err := registry.Initialize(context.Background(), adapter.Descriptor().AdapterID, target.ProviderConfig()).Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Dispose(context.Background()).Result(context.Background())
	for _, d := range provider.Capabilities().Snapshot().Entries {
		if d.ID == "session.conference" {
			if d.Support.Level != "supported" {
				t.Fatal("conference not supported")
			}
			return
		}
	}
	t.Fatal("conference capability missing")
}

func TestFakeTargetFactoryDefectsAndIsolation(t *testing.T) {
	target := MakeFakeTarget(FakeDefects{AcceptUnknownRequired: true})
	first, second := target.CreateRegistry(), target.CreateRegistry()
	if _, err := first.Register(target.CreateAdapter()); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Register(target.CreateAdapter()); err != nil {
		t.Fatal(err)
	}
	provider, err := first.Initialize(context.Background(), adapterID, target.ProviderConfig()).Result(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.Capabilities().Negotiate(context.Background(), []openrtc.CapabilityRequest{{ID: "unknown", Strength: "required"}}).Result(context.Background())
	if err != nil || len(result.Entries) != 1 || result.Entries[0].Status != "accepted" {
		t.Fatalf("defect not propagated: %+v, %v", result, err)
	}
}

func TestFakeFixtureProvisioningAndCredentials(t *testing.T) {
	ctx := context.Background()
	target := MakeFakeTarget(FakeDefects{})
	for _, mode := range []harness.ProvisioningModeName{harness.ExternalProvisioned, harness.CreateOnJoin, harness.ExplicitCreate, harness.BackendBrokered} {
		conference, err := target.ConferenceTarget(ctx, mode)
		if err != nil || conference.Provisioning.Mode != string(mode) {
			t.Fatalf("mode %s: %v", mode, err)
		}
		if mode == harness.BackendBrokered {
			resolved, err := conference.Provisioning.Broker.Resolve(ctx, *conference.Provisioning.Request).Result(ctx)
			if err != nil || resolved.Address.Value != "room-selftest" {
				t.Fatalf("broker: %+v, %v", resolved, err)
			}
		}
	}
	c, err := target.Credentials().Valid(ctx, openrtc.CredentialRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if value := c.Material.Use(func(b []byte) any { return string(b) }); value != "fake-token-1-abcdef" {
		t.Fatalf("credential: %v", value)
	}
}
