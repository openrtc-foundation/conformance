package openrtc

import (
	"encoding/json"
	"fmt"
)

// checkVariant rejects missing, null, and foreign fields at discriminated JSON boundaries.
func checkVariant(data []byte, tag string, variants map[string][]string, optional ...string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected object")
	}
	var kind string
	if err := json.Unmarshal(fields[tag], &kind); err != nil {
		return nil, fmt.Errorf("invalid %s", tag)
	}
	required, ok := variants[kind]
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", tag, kind)
	}
	allowed := map[string]bool{tag: true}
	for _, name := range optional {
		allowed[name] = true
	}
	for _, name := range required {
		allowed[name] = true
		v, ok := fields[name]
		if !ok || (string(v) == "null" && !(tag == "status" && kind == "specified" && name == "value")) {
			return nil, fmt.Errorf("%s requires %s", kind, name)
		}
	}
	for name := range fields {
		if !allowed[name] {
			return nil, fmt.Errorf("unexpected %s for %s", name, kind)
		}
	}
	return fields, nil
}

func (d *DefaultDescription) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"source", "value"}, nil, map[string][]string{"source": {"portable", "provider", "unspecified", "unknown"}}); err != nil {
		return err
	}
	type plain DefaultDescription
	return json.Unmarshal(data, (*plain)(d))
}
func (p *PlacementRequirement) UnmarshalJSON(data []byte) error {
	_, err := checkVariant(data, "strength", map[string][]string{"requirement": {"regions", "boundary"}, "preference": {"regions", "boundary"}})
	if err != nil {
		return err
	}
	type plain PlacementRequirement
	var v plain
	if err = json.Unmarshal(data, &v); err == nil {
		*p = PlacementRequirement(v)
	}
	return err
}
func (r *ClientRuntimeCommand) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "target", map[string][]string{"client": {}, "adapter": {"request"}}, map[string][]string{"client": {"diagnostics", "captureDefaults", "extensions"}}); err != nil {
		return err
	}
	type plain ClientRuntimeCommand
	return json.Unmarshal(data, (*plain)(r))
}
func (r *SessionRuntimeCommand) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "target", map[string][]string{"participant": {"id", "metadata"}, "media": {"request"}, "adapter": {"request"}}, map[string][]string{"participant": {"extensions"}, "media": {"extensions"}})
	if err != nil {
		return err
	}
	type plain SessionRuntimeCommand
	if kind == "adapter" {
		var request AdapterCommand
		if err = json.Unmarshal(m["request"], &request); err != nil {
			return err
		}
		*r = SessionRuntimeCommand{Target: kind, AdapterRequest: &request}
		return nil
	}
	return json.Unmarshal(data, (*plain)(r))
}
func (r SessionRuntimeCommand) MarshalJSON() ([]byte, error) {
	if r.Target == "adapter" {
		return json.Marshal(struct {
			Target  string          `json:"target"`
			Request *AdapterCommand `json:"request"`
		}{r.Target, r.AdapterRequest})
	}
	type plain SessionRuntimeCommand
	return json.Marshal(plain(r))
}
func (s *SessionProvisioning) UnmarshalJSON(data []byte) error {
	_, err := checkVariant(data, "mode", map[string][]string{"explicit-create": {"definition"}, "create-on-join": {"address", "creationPolicy"}, "external-provisioned": {"address", "definitionId"}})
	if err != nil {
		return err
	}
	type plain SessionProvisioning
	var v plain
	if err = json.Unmarshal(data, &v); err == nil {
		*s = SessionProvisioning(v)
	}
	return err
}
func (s *CredentialScope) UnmarshalJSON(data []byte) error {
	_, err := checkVariant(data, "kind", map[string][]string{"session-admission": {"address", "actions"}, "endpoint-account": {"account", "actions"}, "transport": {"endpoint"}, "service-job": {"target", "actions"}})
	if err != nil {
		return err
	}
	type plain CredentialScope
	var v plain
	if err = json.Unmarshal(data, &v); err == nil {
		*s = CredentialScope(v)
	}
	return err
}
func (s *SupportDescription) UnmarshalJSON(data []byte) error {
	_, err := checkVariant(data, "level", map[string][]string{"supported": {"reason", "subset"}, "partial": {"reason", "subset"}, "unsupported": {"reason", "subset"}, "unknown": {"reason", "subset"}, "not-applicable": {"reason", "subset"}})
	if err != nil {
		return err
	}
	type plain SupportDescription
	var v plain
	if err = json.Unmarshal(data, &v); err == nil {
		*s = SupportDescription(v)
	}
	return err
}
func (v *ConstraintValue) UnmarshalJSON(data []byte) error {
	f, err := checkVariant(data, "kind", map[string][]string{"number": {"value"}, "boolean": {"value"}, "range": {"minimum", "maximum"}, "choice": {"values"}})
	if err != nil {
		return err
	}
	var kind string
	_ = json.Unmarshal(f["kind"], &kind)
	if kind == "number" {
		var x float64
		if err = json.Unmarshal(f["value"], &x); err != nil {
			return err
		}
	}
	if kind == "boolean" {
		var x bool
		if err = json.Unmarshal(f["value"], &x); err != nil {
			return err
		}
	}
	type plain ConstraintValue
	var x plain
	if err = json.Unmarshal(data, &x); err == nil {
		*v = ConstraintValue(x)
	}
	return err
}
func (p *CapabilityPredicate) UnmarshalJSON(data []byte) error {
	f, err := checkVariant(data, "kind", map[string][]string{"all": {"terms", "evaluation"}, "any": {"terms", "evaluation"}, "not": {"term", "evaluation"}, "atom": {"subject", "reference", "operator", "value", "evaluation"}})
	if err != nil {
		return err
	}
	if err = checkLiteral(f, "evaluation", "satisfied", "unsatisfied", "unknown"); err != nil {
		return err
	}
	if _, ok := f["subject"]; ok {
		if err = checkLiteral(f, "subject", "capability", "entitlement", "device", "role", "topology", "key-access"); err != nil {
			return err
		}
		if err = checkLiteral(f, "operator", "equals", "includes", "at-least"); err != nil {
			return err
		}
	}
	type plain CapabilityPredicate
	var v plain
	if err = json.Unmarshal(data, &v); err == nil {
		*p = CapabilityPredicate(v)
	}
	return err
}
func checkLiteral(fields map[string]json.RawMessage, name string, values ...string) error {
	var value string
	if err := json.Unmarshal(fields[name], &value); err != nil {
		return err
	}
	for _, v := range values {
		if value == v {
			return nil
		}
	}
	return fmt.Errorf("invalid %s %q", name, value)
}
