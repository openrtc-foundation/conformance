package helpers

import (
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

func TestForgedPayloadDefaultAndExplicitSchema(t *testing.T) {
	if got := ForgedPayload("adapter", "runtime").SchemaID; got != "openrtc-conformance/forged" {
		t.Fatalf("default schema: %q", got)
	}
	if got := ForgedPayload("adapter", "runtime", "").SchemaID; got != "" {
		t.Fatalf("explicit empty schema: %q", got)
	}
	payload := ForgedExtensions("adapter", "runtime", "client-init")["adapter"]
	if payload.Lifetime != "client-init" || payload.SchemaID != "openrtc-conformance/forged" {
		t.Fatalf("forged extension: %+v", payload)
	}
}

func TestEvaluatePredicate(t *testing.T) {
	atom := func(e string) Predicate { return Predicate{Kind: "atom", Evaluation: e} }
	for _, a := range []string{"satisfied", "unsatisfied", "unknown"} {
		for _, b := range []string{"satisfied", "unsatisfied", "unknown"} {
			all, any := "unknown", "unknown"
			if a == "unsatisfied" || b == "unsatisfied" {
				all = "unsatisfied"
			} else if a == "satisfied" && b == "satisfied" {
				all = "satisfied"
			}
			if a == "satisfied" || b == "satisfied" {
				any = "satisfied"
			} else if a == "unsatisfied" && b == "unsatisfied" {
				any = "unsatisfied"
			}
			for _, tc := range []struct{ kind, want string }{{"all", all}, {"any", any}} {
				p := Predicate{Kind: tc.kind, Terms: []openrtc.CapabilityPredicate{atom(a), atom(b)}}
				if got := EvaluatePredicate(p); got != tc.want {
					t.Errorf("%s(%s, %s) = %s, want %s", tc.kind, a, b, got, tc.want)
				}
			}
		}
		want := map[string]string{"satisfied": "unsatisfied", "unsatisfied": "satisfied", "unknown": "unknown"}[a]
		term := atom(a)
		if got := EvaluatePredicate(Predicate{Kind: "not", Term: &term}); got != want {
			t.Errorf("not(%s) = %s, want %s", a, got, want)
		}
	}
	if EvaluatePredicate(Predicate{Kind: "all"}) != "satisfied" || EvaluatePredicate(Predicate{Kind: "any"}) != "unsatisfied" {
		t.Fatal("empty predicate identity differs")
	}
}
