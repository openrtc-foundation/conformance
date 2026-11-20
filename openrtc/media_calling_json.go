package openrtc

import (
	"encoding/json"
	"fmt"
)

func (t StatisticsTarget) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"kind": t.Kind}
	switch t.Kind {
	case "session":
	case "publication", "subscription", "leg":
		fields["id"] = t.ID
	case "receive-source":
		fields["source"] = t.Source
	default:
		return nil, fmt.Errorf("invalid statistics target %q", t.Kind)
	}
	return json.Marshal(fields)
}
func (d CallDestination) MarshalJSON() ([]byte, error) {
	switch d.Kind {
	case "sip":
		return json.Marshal(struct {
			Kind string `json:"kind"`
			URI  string `json:"uri"`
		}{d.Kind, d.URI})
	case "telephone":
		return json.Marshal(struct {
			Kind   string `json:"kind"`
			Number string `json:"number"`
		}{d.Kind, d.Number})
	default:
		return json.Marshal(struct {
			Kind    string          `json:"kind"`
			Address *ScopedNativeID `json:"address"`
		}{d.Kind, d.Address})
	}
}
