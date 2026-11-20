package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func TestFakeRemoteControl(t *testing.T) {
	target := MakeFakeTarget(NoDefects)
	conference, err := target.ConferenceTarget(context.Background(), harness.ExternalProvisioned)
	if err != nil {
		t.Fatal(err)
	}
	controls := target.Controls()
	endpoint, err := controls.RemoteEndpoint(context.Background(), *conference, harness.RemoteEndpointIntent{Identity: openrtc.ApplicationIdentity{Namespace: "test", Subject: "remote"}})
	if err != nil {
		t.Fatal(err)
	}
	if !endpoint.IsConnected() {
		t.Fatal("remote should start connected")
	}
	if err := endpoint.SendText(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if got := endpoint.ReceivedText(); len(got) != 1 || got[0] != "hello" {
		t.Fatalf("received: %v", got)
	}
	if err := endpoint.Leave(context.Background()); err != nil {
		t.Fatal(err)
	}
	if endpoint.IsConnected() {
		t.Fatal("remote should be disconnected")
	}
}

func cleanupAfter(ctx context.Context, inspection openrtc.InspectionCommon, operation openrtc.Async[openrtc.Void]) (openrtc.CleanupReport, error) {
	_, err := operation.Result(ctx)
	return inspection.Cleanup(), err
}
