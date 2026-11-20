package openrtc

import (
	"encoding/json"
	"testing"
)

func TestBindingPart3UnionRequiredEmptyFields(t *testing.T) {
	tests := []struct {
		value any
		key   string
	}{
		{CleanupStatus{Kind: "pending", Outstanding: []string{}}, "outstanding"},
		{NegotiationEntry{Status: "pending", Reason: "authorization-required", Pending: []CapabilityPrerequisite{}}, "pending"},
		{EntityChange[string]{Kind: "remove", Previous: "x"}, "reason"},
	}
	for _, tt := range tests {
		data, err := json.Marshal(tt.value)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if _, ok := got[tt.key]; !ok {
			t.Errorf("%T: missing %s in %s", tt.value, tt.key, data)
		}
	}
}
