package openrtc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAuditedUnionJSON(t *testing.T) {
	tests := []struct {
		name, input string
		value       any
	}{
		{"default null", `{"source":"portable","value":{"status":"known","value":null}}`, &DefaultDescription{}},
		{"default provider", `{"source":"provider","value":{"status":"unknown"}}`, &DefaultDescription{}},
		{"placement", `{"strength":"requirement","regions":[],"boundary":{"resources":[],"transit":"included"}}`, &PlacementRequirement{}},
		{"runtime", `{"target":"client","diagnostics":{"level":"off"}}`, &ClientRuntimeCommand{}},
		{"provisioning", `{"mode":"create-on-join","address":{"adapterId":"a","namespace":"n","value":"v","deployment":{"status":"unknown"}},"creationPolicy":{"status":"unknown"}}`, &SessionProvisioning{}},
		{"resolved", `{"mode":"external-provisioned","address":{"adapterId":"a","namespace":"n","value":"v","deployment":{"status":"unknown"}},"definition":{"status":"unknown"}}`, &ResolvedSessionTarget{}},
		{"scope", `{"kind":"service-job","target":{"kind":"job","job":"j","nativeId":{"status":"unknown"},"address":{"adapterId":"a","namespace":"n","value":"v","deployment":{"status":"unknown"}}},"actions":[]}`, &CredentialScope{}},
		{"result", `{"kind":"unsupported","reason":""}`, &CredentialResult{}},
		{"support", `{"level":"partial","reason":"","subset":[]}`, &SupportDescription{}},
		{"constraint", `{"kind":"boolean","value":false}`, &ConstraintValue{}},
		{"predicate", `{"kind":"all","terms":[],"evaluation":"unknown"}`, &CapabilityPredicate{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(tt.input), tt.value); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(tt.value)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			_ = json.Unmarshal([]byte(tt.input), &want)
			_ = json.Unmarshal(out, &got)
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("round trip got %s, want %s", out, tt.input)
			}
		})
	}
}

func TestAuditedUnionRejectsInvalidJSON(t *testing.T) {
	tests := []struct {
		input string
		value any
	}{
		{`{"status":"specified"}`, &DefaultDescription{}},
		{`{"strength":"maybe","regions":[],"boundary":{}}`, &PlacementRequirement{}},
		{`{"target":"client","diagnostics":{},"id":"x"}`, &ClientRuntimeCommand{}},
		{`{"mode":"backend-brokered","request":{}}`, &ResolvedSessionTarget{}},
		{`{"kind":"transport","endpoint":{},"actions":[]}`, &CredentialScope{}},
		{`{"kind":"unsupported"}`, &CredentialResult{}},
		{`{"level":"partial","reason":""}`, &SupportDescription{}},
		{`{"kind":"number","value":false}`, &ConstraintValue{}},
		{`{"kind":"all","terms":[],"evaluation":"yes"}`, &CapabilityPredicate{}},
	}
	for _, tt := range tests {
		if err := json.Unmarshal([]byte(tt.input), tt.value); err == nil {
			t.Errorf("accepted %s", tt.input)
		}
	}
}
