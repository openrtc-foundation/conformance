package harness

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

type testMaterial []byte

func (m testMaterial) Use(f func([]byte) any) any { return f(m) }
func TestLeakAndSafeJSON(t *testing.T) {
	m := map[string]any{}
	m["self"] = m
	m["credential"] = []byte("secret-token")
	if p := FindLeak(m, []string{"secret-token"}); p == nil || !strings.Contains(*p, "credential") {
		t.Fatalf("leak: %v", p)
	}
	if p := FindLeak(m, nil, 1); p == nil || !strings.HasPrefix(*p, Truncated) {
		t.Fatalf("truncation: %v", p)
	}
	if s := SafeJSON(m); !strings.Contains(s, `"[cycle]"`) || !strings.Contains(s, `[115,101,99`) {
		t.Fatal(s)
	}
	if s := SafeJSON(map[string]any{"toJSON": "evil"}); s != `"[object with own toJSON omitted]"` {
		t.Fatal(s)
	}
}
func TestCredentialSpy(t *testing.T) {
	spy := NewCredentialSpy(CredentialSource{Valid: func(context.Context, openrtc.CredentialRequest) (openrtc.CredentialInput, error) {
		return openrtc.CredentialInput{Material: testMaterial("secret-token")}, nil
	}})
	p := spy.Provider(false)
	if _, ok := p.(openrtc.CredentialRenewer); ok {
		t.Fatal("unexpected renewal")
	}
	if _, err := p.Get(context.Background(), openrtc.CredentialRequest{}).Result(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !spy.Reached(context.Background(), 1, 10) || len(spy.Needles()) != 2 {
		t.Fatal(spy.Calls(), spy.Needles())
	}
	failure := errors.New("rejected")
	spy.SetBehavior(CredentialBehavior{Kind: "reject", Failure: failure})
	if _, err := p.Get(context.Background(), openrtc.CredentialRequest{}).Result(context.Background()); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, ok := spy.Provider(true).(openrtc.CredentialRenewer); !ok {
		t.Fatal("missing renewal")
	}
}
