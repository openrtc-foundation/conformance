package openrtc

import "encoding/json"

// MarshalJSON retains required fields, even when a union variant contains empty values.
func (s CleanupStatus) MarshalJSON() ([]byte, error) {
	m := map[string]any{"kind": s.Kind}
	switch s.Kind {
	case "pending":
		m["outstanding"] = s.Outstanding
	case "failed":
		m["errors"] = s.Errors
		m["outstanding"] = s.Outstanding
	case "unknown":
		m["reason"] = s.Reason
	}
	return json.Marshal(m)
}
func (c EntityChange[T]) MarshalJSON() ([]byte, error) {
	m := map[string]any{"kind": c.Kind, "previous": c.Previous}
	switch c.Kind {
	case "upsert":
		m["current"] = c.Current
	case "remove":
		m["reason"] = c.Reason
	}
	return json.Marshal(m)
}

// UnmarshalJSON retains the distinct types of previous in the two variants.
func (c *EntityChange[T]) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "kind", map[string][]string{"upsert": {"previous", "current"}, "remove": {"previous", "reason"}}, nil); err != nil {
		return err
	}
	var raw struct {
		Kind     string          `json:"kind"`
		Previous json.RawMessage `json:"previous"`
		Current  *T              `json:"current"`
		Reason   string          `json:"reason"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*c = EntityChange[T]{Kind: raw.Kind, Current: raw.Current, Reason: raw.Reason}
	if raw.Kind == "upsert" {
		var previous Known[T]
		if err := json.Unmarshal(raw.Previous, &previous); err != nil {
			return err
		}
		c.Previous = previous
	} else {
		var previous T
		if err := json.Unmarshal(raw.Previous, &previous); err != nil {
			return err
		}
		c.Previous = previous
	}
	return nil
}
