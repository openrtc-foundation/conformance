package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
	"strings"
	"testing"
	"time"
)

func TestContextTimeoutAndCleanup(t *testing.T) {
	c := &TestContext{Target: testTarget{}}
	done := make(chan struct{})
	err := c.Within(context.Background(), "operation", 1, func(context.Context) error { <-done; return nil })
	if err == nil || len(c.Observations) != 1 {
		t.Fatalf("timeout: %v %v", err, c.Observations)
	}
	close(done)
	var order []string
	c.Track("first", func(context.Context) error { order = append(order, "first"); return nil })
	c.Track("second", func(context.Context) error { order = append(order, "second"); return nil })
	if failures := c.Cleanup(context.Background()); len(failures) != 0 || strings.Join(order, ",") != "second,first" {
		t.Fatalf("cleanup %v %v", order, failures)
	}
	if len(c.Cleanup(context.Background())) != 0 {
		t.Fatal("cleanup repeated")
	}
}

// RFC §9:529–570 and §12.1:1044–1046: safe portable error structure.
func TestContextErrorChecks(t *testing.T) {
	c := &TestContext{Target: testTarget{}}
	e := openrtc.NewRTCOperationError("failure", openrtc.RTCErrorDetails{Reasons: []string{"invalid-state"}, Outcome: "none", ID: "id", Code: openrtc.InvalidState, Operation: openrtc.OperationRef{Name: "join"}, Retryability: "never", AdapterID: openrtc.Known[openrtc.AdapterID]{Status: "unknown"}, ProviderError: openrtc.Known[openrtc.ProviderError]{Status: "unknown"}})
	if _, err := c.ExpectError(e, ExpectedFailure{Codes: []openrtc.ErrorCode{openrtc.InvalidState}, Kind: "operation"}, "join"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExpectError(e, ExpectedFailure{Codes: []openrtc.ErrorCode{openrtc.Cancelled}}, "join"); err == nil {
		t.Fatal("accepted wrong code")
	}
	if _, err := c.RequireControl("remoteEndpoint"); err == nil {
		t.Fatal("missing control accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := c.Capture(ctx, func(context.Context) (any, error) { return 42, nil })
	if !out.OK || out.Value != 42 {
		t.Fatalf("capture: %+v", out)
	}
}

func TestWithinCapturesPanic(t *testing.T) {
	tc := NewTestContext(testTarget{}, openrtc.ProviderDescriptor{})
	err := tc.Within(context.Background(), "panic", 100, func(context.Context) error { panic("operation failed") })
	if err == nil || err.Error() != "operation failed" {
		t.Fatalf("panic result: %v", err)
	}
}
