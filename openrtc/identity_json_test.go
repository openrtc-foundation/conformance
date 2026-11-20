package openrtc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAuditedUnionsRoundTrip(t *testing.T) {
	for _, input := range []string{`{"status":"known","value":""}`, `{"status":"unknown"}`, `{"status":"not-applicable","reason":""}`} {
		var value Known[string]
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(value)
		if err != nil || string(out) != input {
			t.Fatalf("%s: %s %v", input, out, err)
		}
	}
	for _, kind := range []string{"provider", "client", "session", "definition", "instance", "participant", "publication", "subscription", "capture", "call-leg", "job", "device", "unspecified"} {
		input := `{"kind":"` + kind + `","id":""}`
		if kind == "unspecified" {
			input = `{"kind":"unspecified","reason":""}`
		}
		var value ResourceRef
		if err := json.Unmarshal([]byte(input), &value); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(value)
		if err != nil || string(out) != input {
			t.Fatalf("%s: %s %v", input, out, err)
		}
	}
	var value Known[any]
	if err := json.Unmarshal([]byte(`{"status":"known","value":null}`), &value); err != nil || value.Value == nil || *value.Value != nil {
		t.Fatalf("known null: %#v %v", value, err)
	}
}

func TestAuditedJSONRejectsInvalidShapes(t *testing.T) {
	for _, tc := range []struct {
		input  string
		target any
	}{
		{`{"status":"known"}`, &Known[string]{}}, {`{"status":"known","value":null}`, &Known[string]{}}, {`{"status":"unknown","value":"x"}`, &Known[string]{}},
		{`{"status":"other"}`, &Known[string]{}}, {`{"status":"not-applicable","reason":null}`, &Known[string]{}},
		{`{"kind":"provider","reason":"x"}`, &ResourceRef{}}, {`{"kind":"invalid","id":"x"}`, &ResourceRef{}},
		{`{"kind":"session","id":"x","extra":1}`, &ResourceRef{}},
		{`{"milliseconds":1,"extra":2}`, &Duration{}}, {`{"milliseconds":null}`, &Duration{}},
		{`{"name":"bad","attempt":{"status":"unknown"},"generation":{"status":"unknown"}}`, &OperationRef{}},
		{`"bad"`, new(AuthorityPlane)}, {`"invalid-code"`, new(ErrorCode)},
		{`{"original":null,"provenance":{},"related":[],"originalAvailability":"bad"}`, &ProviderError{}},
	} {
		if err := json.Unmarshal([]byte(tc.input), tc.target); err == nil {
			t.Errorf("accepted %s into %s", tc.input, reflect.TypeOf(tc.target))
		}
	}
}

func TestAdapterContractViolationDecode(t *testing.T) {
	var code ErrorCode
	if err := json.Unmarshal([]byte(`"internal"`), &code); err != nil || code != Internal {
		t.Fatalf("decoded %q: %v", code, err)
	}
}
