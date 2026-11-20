package selftest

import (
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

func TestValidationInspectsOpaquePayloads(t *testing.T) {
	// The payload can contain values that JSON cannot encode. Validation must
	// still inspect the namespace and metadata surrounding it.
	opaque := func() {}
	foreign := openrtc.ProviderConfig{Extensions: openrtc.ProviderExtensions{
		"foreign": {Payload: opaque},
	}}
	if _, err := validateHolder("provider-init", foreign, NoDefects, "initialize"); err == nil {
		t.Fatal("accepted a foreign namespace containing an opaque payload")
	}
	wrongLifetime := openrtc.ProviderExtensions{
		adapterID: {Lifetime: "client-init", SchemaID: schemaID, Payload: opaque},
	}
	if err := validateExtensions(wrongLifetime, "runtime", "configure"); err == nil {
		t.Fatal("accepted an extension validated for another lifetime")
	}
	valid := openrtc.ProviderExtensions{
		adapterID: {Lifetime: "client-init", SchemaID: schemaID, Payload: opaque},
	}
	if err := validateExtensions(valid, "client-init", "create-client"); err != nil {
		t.Fatalf("rejected valid metadata: %v", err)
	}
}
