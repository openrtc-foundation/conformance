package selftest

import (
	"context"
	"errors"
	"strings"

	"github.com/openrtc/conformance/openrtc"
)

// bindingAdapter is the adapter used by the self-test fake. The fake
// registry constructs its provider itself, rather than using the adapter bridge.
type bindingAdapter struct{}

var _ openrtc.RTCAdapter = bindingAdapter{}

func (bindingAdapter) Descriptor() openrtc.ProviderDescriptor {
	return openrtc.ProviderDescriptor{Artifact: "harness-fake", Prerequisites: []string{}, DeliveryExecutor: "fake-observation-queue", BridgeExecutor: "none",
		AdapterID: adapterID, AdapterVersion: "0.0.0-selftest",
		Specification: openrtc.VersionCompatibility{MinimumInclusive: hostSpec, MaximumExclusive: openrtc.SpecVersion{Major: 0, Minor: 2, Patch: 0}},
		SDKArtifact:   "none (in-memory fake)", SDKVersion: openrtc.Unknown[string](),
		ServerArtifact: openrtc.Unknown[string](), Platform: "node", Deployment: openrtc.Unknown[openrtc.DeploymentID](),
		Profiles: []string{"conference"}, CollisionPolicy: openrtc.IdentityCollisionPolicy{Behavior: "reject-new", Scope: "application identity within one address", Evidence: []openrtc.EvidenceReference{}},
		Evidence: []openrtc.EvidenceReference{},
	}
}

func (bindingAdapter) Schemas(lifetime openrtc.ConfigurationLifetime) []openrtc.ConfigurationSchema {
	if lifetime == openrtc.ClientInit {
		return []openrtc.ConfigurationSchema{buildSchema()}
	}
	return []openrtc.ConfigurationSchema{}
}

func (receiver bindingAdapter) Initialize(context.Context, openrtc.ProviderConfig, openrtc.AdapterHost) openrtc.Async[openrtc.AdapterProvider] {
	return startOperation(receiver, func() (openrtc.AdapterProvider, error) {
		return nil, errors.New("the self-test fake does not implement the AdapterProvider bridge (ADP-106)")

	})
}

func compareVersion(a, b openrtc.SpecVersion) int {
	for _, pair := range [][2]uint64{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	return 0
}

// validateAdapterDescriptor implements the checks performed before registration.
func validateAdapterDescriptor(d openrtc.ProviderDescriptor) error {
	if strings.TrimSpace(d.SDKArtifact) == "" || len(d.Profiles) == 0 || strings.TrimSpace(string(d.AdapterID)) == "" {
		return fail("incompatible-adapter", "initialize", "descriptor is missing required metadata", failureExtras{})
	}
	if compareVersion(d.Specification.MinimumInclusive, hostSpec) > 0 || compareVersion(d.Specification.MaximumExclusive, hostSpec) <= 0 {
		return fail("incompatible-adapter", "initialize", "no specification version is supported by both host and adapter", failureExtras{})
	}
	return nil
}
