package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

func TestBindingAdapterMetadata(t *testing.T) {
	a := bindingAdapter{}
	d := a.Descriptor()
	if err := validateAdapterDescriptor(d); err != nil {
		t.Fatal(err)
	}
	if d.AdapterID != adapterID || len(a.Schemas(openrtc.ClientInit)) != 1 || len(a.Schemas(openrtc.ProviderInit)) != 0 {
		t.Fatal("unexpected descriptor or schemas")
	}
	if _, err := a.Initialize(context.Background(), openrtc.ProviderConfig{}, nil).Result(context.Background()); err == nil {
		t.Fatal("adapter bridge unexpectedly available")
	}
	for _, mutate := range []func(*openrtc.ProviderDescriptor){
		func(d *openrtc.ProviderDescriptor) { d.AdapterID = " " },
		func(d *openrtc.ProviderDescriptor) { d.SDKArtifact = " " },
		func(d *openrtc.ProviderDescriptor) { d.Profiles = nil },
		func(d *openrtc.ProviderDescriptor) { d.Specification.MinimumInclusive.Patch++ },
		func(d *openrtc.ProviderDescriptor) { d.Specification.MaximumExclusive = hostSpec },
	} {
		bad := a.Descriptor()
		mutate(&bad)
		if err := validateAdapterDescriptor(bad); err == nil {
			t.Fatalf("accepted invalid descriptor: %+v", bad)
		}
	}
}
