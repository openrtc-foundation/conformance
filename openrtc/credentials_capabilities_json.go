package openrtc

import "encoding/json"

// UnmarshalJSON decodes the metadata according to the result's discriminant.
func (r *CredentialResult) UnmarshalJSON(data []byte) error {
	if _, err := checkVariant(data, "kind", map[string][]string{
		"applied-live": {"metadata"}, "cached-for-reconnect": {"metadata"},
		"sdk-managed": {"metadata"}, "fresh-session-required": {"reason"},
		"unsupported": {"reason"}, "unknown": {"reason"},
	}); err != nil {
		return err
	}
	var v struct {
		Kind     string          `json:"kind"`
		Metadata json.RawMessage `json:"metadata"`
		Reason   *string         `json:"reason"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = CredentialResult{Kind: v.Kind, Reason: v.Reason}
	if len(v.Metadata) == 0 || string(v.Metadata) == "null" {
		return nil
	}
	if v.Kind == "sdk-managed" {
		var metadata Known[CredentialMetadata]
		if err := json.Unmarshal(v.Metadata, &metadata); err != nil {
			return err
		}
		r.KnownMetadata = &metadata
	} else {
		var metadata CredentialMetadata
		if err := json.Unmarshal(v.Metadata, &metadata); err != nil {
			return err
		}
		r.Metadata = &metadata
	}
	return nil
}

// MarshalJSON preserves the required value of each credential result variant.
func (r CredentialResult) MarshalJSON() ([]byte, error) {
	switch r.Kind {
	case "sdk-managed":
		return json.Marshal(struct {
			Kind     string                     `json:"kind"`
			Metadata *Known[CredentialMetadata] `json:"metadata"`
		}{r.Kind, r.KnownMetadata})
	case "applied-live", "cached-for-reconnect":
		return json.Marshal(struct {
			Kind     string              `json:"kind"`
			Metadata *CredentialMetadata `json:"metadata"`
		}{r.Kind, r.Metadata})
	default:
		return json.Marshal(struct {
			Kind   string  `json:"kind"`
			Reason *string `json:"reason"`
		}{r.Kind, r.Reason})
	}
}

// MarshalJSON preserves required empty arrays and zero-valued constraint values.
func (v ConstraintValue) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case "number", "boolean":
		return json.Marshal(struct {
			Kind  string `json:"kind"`
			Value any    `json:"value"`
		}{v.Kind, v.Value})
	case "range":
		return json.Marshal(struct {
			Kind    string   `json:"kind"`
			Minimum *float64 `json:"minimum"`
			Maximum *float64 `json:"maximum"`
		}{v.Kind, v.Minimum, v.Maximum})
	case "choice":
		return json.Marshal(struct {
			Kind   string   `json:"kind"`
			Values []string `json:"values"`
		}{v.Kind, v.Values})
	default:
		return json.Marshal(struct {
			Kind string `json:"kind"`
		}{v.Kind})
	}
}

// MarshalJSON retains the subset required for partial support, even when empty.

// MarshalJSON retains required action lists in scoped credentials.
func (s CredentialScope) MarshalJSON() ([]byte, error) {
	switch s.Kind {
	case "session-admission":
		return json.Marshal(struct {
			Kind    string                `json:"kind"`
			Address *SessionAddress       `json:"address"`
			Actions []CapabilityReference `json:"actions"`
		}{s.Kind, s.Address, s.Actions})
	case "endpoint-account":
		return json.Marshal(struct {
			Kind    string                `json:"kind"`
			Account *ProviderIdentity     `json:"account"`
			Actions []CapabilityReference `json:"actions"`
		}{s.Kind, s.Account, s.Actions})
	case "service-job":
		return json.Marshal(struct {
			Kind    string                      `json:"kind"`
			Target  *ServiceJobCredentialTarget `json:"target"`
			Actions []CapabilityReference       `json:"actions"`
		}{s.Kind, s.Target, s.Actions})
	default:
		return json.Marshal(struct {
			Kind     string           `json:"kind"`
			Endpoint *ServiceEndpoint `json:"endpoint"`
		}{s.Kind, s.Endpoint})
	}
}

// MarshalJSON retains required fields of each nested predicate variant.
func (p CapabilityPredicate) MarshalJSON() ([]byte, error) {
	switch p.Kind {
	case "all", "any":
		return json.Marshal(struct {
			Kind       string                `json:"kind"`
			Terms      []CapabilityPredicate `json:"terms"`
			Evaluation string                `json:"evaluation"`
		}{p.Kind, p.Terms, p.Evaluation})
	case "not":
		return json.Marshal(struct {
			Kind       string               `json:"kind"`
			Term       *CapabilityPredicate `json:"term"`
			Evaluation string               `json:"evaluation"`
		}{p.Kind, p.Term, p.Evaluation})
	default:
		return json.Marshal(struct {
			Kind       string           `json:"kind"`
			Subject    *string          `json:"subject"`
			Reference  *string          `json:"reference"`
			Operator   *string          `json:"operator"`
			Value      *ConstraintValue `json:"value"`
			Evaluation string           `json:"evaluation"`
		}{p.Kind, p.Subject, p.Reference, p.Operator, p.Value, p.Evaluation})
	}
}
