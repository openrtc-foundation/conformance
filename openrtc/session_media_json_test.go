package openrtc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAuditedUnionRoundTrips(t *testing.T) {
	cases := []struct {
		name, input string
		value       any
	}{
		{"negotiation", `{"status":"not-selected","request":{"kind":"portable","id":"x","strength":"preferred","scope":{"adapterId":"","artifact":"","platform":"","profile":"common","authority":"client","resource":"client"},"guarantees":[],"constraints":[]},"reason":"","descriptorRevision":{"status":"unknown"},"accepted":{"status":"unknown"},"pending":[],"alternatives":[]}`, &NegotiationEntry{}},
		{"connection", `"ended"`, new(ConnectionState)},
		{"cleanup", `{"kind":"pending","outstanding":[]}`, &CleanupStatus{}},
		{"entity", `{"kind":"upsert","previous":{"status":"known","value":"before"},"current":"after"}`, &EntityChange[string]{}},
		{"effect", `{"kind":"receive","value":"unchanged"}`, &MediaEffectRequirement{}},
		{"media configuration", `{"command":"set-muted","publication":"p","muted":false}`, &MediaConfigurationRequest{}},
		{"receive", `{"kind":"publication","publication":"p"}`, &ReceiveSourceReference{}},
		{"render", `{"kind":"provider-managed","target":{"kind":"session"}}`, &RenderTarget{}},
		{"statistics", `{"kind":"publication","id":""}`, &StatisticsTarget{}},
		{"destination", `{"kind":"sip","uri":""}`, &CallDestination{}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.input), tt.value); err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			var a, z any
			_ = json.Unmarshal([]byte(tt.input), &a)
			_ = json.Unmarshal(b, &z)
			if !reflect.DeepEqual(a, z) {
				t.Fatalf("round trip: %s", b)
			}
		})
	}
}
func TestAuditedUnionRejectsInvalidShape(t *testing.T) {
	cases := []struct {
		input string
		value any
	}{
		{`"bogus"`, new(ConnectionState)}, {`{"kind":"idle","reason":"extra"}`, new(ConnectionState)},
		{`{"kind":"remove","previous":"x"}`, &EntityChange[string]{}}, {`{"kind":"receive","value":"released"}`, &MediaEffectRequirement{}},
		{`{"kind":"publication","id":"x"}`, &ReceiveSourceReference{}},
		{`{"command":"set-sending"}`, &MediaConfigurationRequest{}}, {`{"kind":"sip","uri":"x","number":"y"}`, &CallDestination{}},
		{`"invalid"`, new(MediaKind)},
	}
	for _, tt := range cases {
		if err := json.Unmarshal([]byte(tt.input), tt.value); err == nil {
			t.Errorf("accepted %s as %T", tt.input, tt.value)
		}
	}
}
