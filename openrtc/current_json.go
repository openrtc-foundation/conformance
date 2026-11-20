package openrtc

import (
	"encoding/json"
	"fmt"
)

func (v *OperationInspectionResult) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "status", map[string][]string{"retained": {"details"}, "evicted": {"operationId"}}, nil); err != nil {
		return err
	}
	type plain OperationInspectionResult
	return json.Unmarshal(data, (*plain)(v))
}
func (v OperationInspectionResult) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("status", v.Status, map[string][]string{"retained": {"details"}, "evicted": {"operationId"}}, map[string]any{"details": v.Details, "operationId": v.OperationID})
}
func (v *ReceiveSourceReference) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "kind", map[string][]string{"publication": {"publication"}, "aggregate": {"id"}, "call-media": {"leg", "direction", "kinds"}}, nil); err != nil {
		return err
	}
	type plain ReceiveSourceReference
	return json.Unmarshal(data, (*plain)(v))
}
func (v ReceiveSourceReference) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("kind", v.Kind, map[string][]string{"publication": {"publication"}, "aggregate": {"id"}, "call-media": {"leg", "direction", "kinds"}}, map[string]any{"publication": v.Publication, "id": v.ID, "leg": v.Leg, "direction": v.Direction, "kinds": v.Kinds})
}

// ReceiveSourceView uses a reference-shaped variant except for aggregate views.
func (v ReceiveSourceView) MarshalJSON() ([]byte, error) {
	if v.Kind != "aggregate" {
		return json.Marshal(v.ReceiveSourceReference)
	}
	return json.Marshal(struct {
		Kind         string                           `json:"kind"`
		ID           ReceiveSourceID                  `json:"id"`
		Generation   GenerationID                     `json:"generation"`
		NativeID     Known[ScopedNativeID]            `json:"nativeId"`
		Kinds        []MediaKind                      `json:"kinds"`
		Contributors Known[[]ParticipantConnectionID] `json:"contributors"`
		Coverage     string                           `json:"coverage"`
	}{v.Kind, v.ID, v.Generation, v.NativeID, v.Kinds, v.Contributors, v.Coverage})
}
func (v *ReceiveSourceView) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "kind", map[string][]string{"publication": {"publication"}, "aggregate": {"id", "generation", "nativeId", "kinds", "contributors", "coverage"}, "call-media": {"leg", "direction", "kinds"}}, nil); err != nil {
		return err
	}
	// Avoid the embedded reference's unmarshaler consuming the whole view.
	var raw struct {
		Kind         string
		Publication  PublicationID
		ID           ReceiveSourceID
		Leg          CallLegID
		Direction    string
		Kinds        []MediaKind
		Generation   GenerationID
		NativeID     Known[ScopedNativeID]
		Contributors Known[[]ParticipantConnectionID]
		Coverage     string
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*v = ReceiveSourceView{ReceiveSourceReference: ReceiveSourceReference{Kind: raw.Kind, Publication: raw.Publication, ID: raw.ID, Leg: raw.Leg, Direction: raw.Direction, Kinds: raw.Kinds}, Generation: raw.Generation, NativeID: raw.NativeID, Contributors: raw.Contributors, Coverage: raw.Coverage}
	return nil
}
func (v InitialMediaRequest) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case "capture":
		return json.Marshal(struct {
			Kind    string          `json:"kind"`
			Request *CaptureRequest `json:"request"`
		}{v.Kind, v.Capture})
	case "send":
		return json.Marshal(struct {
			Kind    string       `json:"kind"`
			Request *SendRequest `json:"request"`
		}{v.Kind, v.Send})
	default:
		return nil, fmt.Errorf("invalid initial media kind %q", v.Kind)
	}
}
func (v *InitialMediaRequest) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "kind", map[string][]string{"capture": {"request"}, "send": {"request"}}, nil)
	if err != nil {
		return err
	}
	out := InitialMediaRequest{Kind: kind}
	if kind == "capture" {
		out.Capture = new(CaptureRequest)
		err = json.Unmarshal(m["request"], out.Capture)
	} else {
		out.Send = new(SendRequest)
		err = json.Unmarshal(m["request"], out.Send)
	}
	if err == nil {
		*v = out
	}
	return err
}
func (v MediaConfigurationRequest) MarshalJSON() ([]byte, error) {
	m := map[string]any{"command": v.Command}
	switch v.Command {
	case "set-muted":
		m["publication"] = v.Publication
		m["muted"] = v.Muted
	case "set-disabled":
		m["source"] = v.Source
		m["disabled"] = v.Disabled
	case "set-sending":
		m["request"] = v.Request
	case "set-receive-policy":
		m["policy"] = v.Policy
	case "set-playback-muted":
		m["target"] = v.Target
		m["muted"] = v.Muted
	case "replace-source":
		m["request"] = v.Replacement
	default:
		return nil, fmt.Errorf("invalid media command %q", v.Command)
	}
	return json.Marshal(m)
}
func (v *MediaConfigurationRequest) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "command", map[string][]string{"set-muted": {"publication", "muted"}, "set-disabled": {"source", "disabled"}, "set-sending": {"request"}, "set-receive-policy": {"policy"}, "set-playback-muted": {"target", "muted"}, "replace-source": {"request"}}, nil)
	if err != nil {
		return err
	}
	type plain MediaConfigurationRequest
	raw := m["request"]
	delete(m, "request")
	b, _ := json.Marshal(m)
	var out plain
	if err = json.Unmarshal(b, &out); err != nil {
		return err
	}
	switch kind {
	case "set-sending":
		out.Request = new(SendRequest)
		err = json.Unmarshal(raw, out.Request)
	case "replace-source":
		out.Replacement = new(SourceReplacementRequest)
		err = json.Unmarshal(raw, out.Replacement)
	}
	if err == nil {
		*v = MediaConfigurationRequest(out)
	}
	return err
}
func (v *MediaEffectValues) UnmarshalJSON(data []byte) error {
	keys := []string{"capture", "outbound", "publication", "receive", "reacquisitionRequired", "recording", "playback"}
	if err := checkAudit1905Object(data, keys, nil, nil); err != nil {
		return err
	}
	type plain MediaEffectValues
	var out plain
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	values := []Known[string]{out.Capture, out.Outbound, out.Publication, out.Receive, out.ReacquisitionRequired, out.Recording, out.Playback}
	allowed := [][]string{{"unchanged", "acquired", "retained", "released"}, {"unchanged", "suppressed", "sending"}, {"unchanged", "created", "retained", "removed"}, {"unchanged", "selected", "removed"}, {"required", "not-required"}, {"unchanged", "excluded"}, {"unchanged", "started", "stopped"}}
	for i, k := range values {
		if k.Status == "known" {
			valid := false
			for _, s := range allowed[i] {
				valid = valid || (k.Value != nil && *k.Value == s)
			}
			if !valid {
				return fmt.Errorf("invalid %s effect", keys[i])
			}
		}
	}
	*v = MediaEffectValues(out)
	return nil
}
func (v *MediaEffectField) UnmarshalJSON(data []byte) error {
	s, err := decodeLiteral(data, "capture", "outbound", "publication", "receive", "reacquisitionRequired", "recording", "playback")
	if err == nil {
		*v = MediaEffectField(s)
	}
	return err
}
func (v *RemoteUpdatePolicy) UnmarshalJSON(data []byte) error {
	s, err := decodeLiteral(data, "accept", "accept-without-new-kinds", "reject-additions")
	if err == nil {
		*v = RemoteUpdatePolicy(s)
	}
	return err
}

func (v RenderTarget) MarshalJSON() ([]byte, error) {
	if v.Kind != "provider-managed" {
		return nil, fmt.Errorf("application sinks are in-process values")
	}
	return json.Marshal(struct {
		Kind   string                 `json:"kind"`
		Target *PlaybackControlTarget `json:"target"`
	}{v.Kind, v.Target})
}

func (v CapabilityRequest) MarshalJSON() ([]byte, error) {
	kind := v.Kind
	if kind == "" {
		kind = "portable"
	}
	fields := map[string]any{"kind": kind, "strength": v.Strength, "scope": v.Scope}
	if kind == "portable" {
		fields["id"] = v.ID
		fields["guarantees"] = v.Guarantees
		fields["constraints"] = v.Constraints
		if v.Operations != nil {
			fields["operations"] = v.Operations
		}
	} else if kind == "adapter" {
		fields["id"] = v.Adapter
		fields["schemaId"] = v.SchemaID
		fields["schemaRevision"] = v.SchemaRevision
		fields["payload"] = v.Payload
	} else {
		return nil, fmt.Errorf("invalid capability request kind %q", kind)
	}
	return json.Marshal(fields)
}
func (v *CapabilityRequest) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "kind", map[string][]string{"portable": {"id", "strength", "scope", "guarantees", "constraints"}, "adapter": {"id", "strength", "scope", "schemaId", "schemaRevision", "payload"}}, map[string][]string{"portable": {"operations"}})
	if err != nil {
		return err
	}
	var common struct {
		Strength       string
		Scope          CapabilityScope
		Operations     []string
		Guarantees     []GuaranteeRequirement
		Constraints    []CapabilityConstraint
		SchemaID       string
		SchemaRevision string
		Payload        JSONValue
	}
	if err = json.Unmarshal(data, &common); err != nil {
		return err
	}
	out := CapabilityRequest{Kind: kind, Strength: common.Strength, Scope: common.Scope, Operations: common.Operations, Guarantees: common.Guarantees, Constraints: common.Constraints, SchemaID: common.SchemaID, SchemaRevision: common.SchemaRevision, Payload: common.Payload}
	if kind == "portable" {
		err = json.Unmarshal(m["id"], &out.ID)
	} else {
		out.Adapter = new(AdapterCapabilityReference)
		err = json.Unmarshal(m["id"], out.Adapter)
	}
	if err == nil {
		*v = out
	}
	return err
}

func (v TransferableSessionTarget) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"mode": v.Mode}
	switch v.Mode {
	case "explicit-create":
		fields["definition"] = v.Definition
	case "create-on-join":
		fields["address"] = v.Address
		if v.CreationPolicy != nil {
			fields["creationPolicy"] = v.CreationPolicy
		}
	case "external-provisioned":
		fields["address"] = v.Address
		fields["definition"] = v.KnownDefinition
	default:
		return nil, fmt.Errorf("invalid transferable target mode %q", v.Mode)
	}
	return json.Marshal(fields)
}
func (v *TransferableSessionTarget) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "mode", map[string][]string{"explicit-create": {"definition"}, "create-on-join": {"address"}, "external-provisioned": {"address", "definition"}}, map[string][]string{"create-on-join": {"creationPolicy"}})
	if err != nil {
		return err
	}
	out := TransferableSessionTarget{Mode: kind}
	if raw, ok := m["address"]; ok {
		out.Address = new(SessionAddress)
		if err = json.Unmarshal(raw, out.Address); err != nil {
			return err
		}
	}
	if raw, ok := m["creationPolicy"]; ok {
		out.CreationPolicy = new(Known[SessionPolicy])
		if err = json.Unmarshal(raw, out.CreationPolicy); err != nil {
			return err
		}
	}
	if kind == "explicit-create" {
		out.Definition = new(SessionDefinitionDescriptor)
		err = json.Unmarshal(m["definition"], out.Definition)
	} else if kind == "external-provisioned" {
		out.KnownDefinition = new(Known[SessionDefinitionDescriptor])
		err = json.Unmarshal(m["definition"], out.KnownDefinition)
	}
	if err == nil {
		*v = out
	}
	return err
}
