package openrtc

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCredentialResultSDKManagedRoundTrip(t *testing.T) {
	input := `{"kind":"sdk-managed","metadata":{"status":"unknown"}}`
	var result CredentialResult
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		t.Fatal(err)
	}
	if result.KnownMetadata == nil || result.KnownMetadata.Status != "unknown" {
		t.Fatalf("lost SDK-managed metadata: %+v", result)
	}
	data, err := json.Marshal(result)
	if err != nil || string(data) != input {
		t.Fatalf("round trip: %s, %v", data, err)
	}
}

func TestOptionalEmptyCollectionsRemainPresent(t *testing.T) {
	data, err := json.Marshal(ClientConfig{Requirements: []CapabilityRequest{}, Extensions: ProviderExtensions{}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"requirements":[]`) || !strings.Contains(string(data), `"extensions":{}`) {
		t.Fatalf("empty collections omitted: %s", data)
	}
}

func TestEntityChangePreviousTypes(t *testing.T) {
	for _, input := range []string{
		`{"kind":"upsert","previous":{"status":"known","value":"before"},"current":"after"}`,
		`{"kind":"remove","previous":"before","reason":"gone"}`,
	} {
		var change EntityChange[string]
		if err := json.Unmarshal([]byte(input), &change); err != nil {
			t.Fatal(err)
		}
		if change.Kind == "upsert" {
			if previous, ok := change.Previous.(Known[string]); !ok || previous.Value == nil || *previous.Value != "before" {
				t.Fatalf("upsert previous: %#v", change.Previous)
			}
		} else if change.Previous != "before" {
			t.Fatalf("remove previous: %#v", change.Previous)
		}
	}
}
