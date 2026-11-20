package openrtc

import (
	"encoding/json"
	"fmt"
)

// unionFields validates the shape before decoding a discriminated union.
func unionFields(data []byte, tag string, variants map[string][]string, optional map[string][]string) (map[string]json.RawMessage, string, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, "", err
	}
	var kind string
	if err := json.Unmarshal(m[tag], &kind); err != nil {
		return nil, "", fmt.Errorf("invalid %s: %w", tag, err)
	}
	required, ok := variants[kind]
	if !ok {
		return nil, "", fmt.Errorf("invalid %s %q", tag, kind)
	}
	allowed := map[string]bool{tag: true}
	for _, key := range required {
		allowed[key] = true
		v, found := m[key]
		if !found || string(v) == "null" {
			return nil, "", fmt.Errorf("%s requires %s", kind, key)
		}
	}
	for _, key := range optional[kind] {
		allowed[key] = true
	}
	for key := range m {
		if !allowed[key] {
			return nil, "", fmt.Errorf("unexpected property %q for %s", key, kind)
		}
	}
	return m, kind, nil
}

func decodeRTCError(data json.RawMessage) (RTCError, error) {
	var v SerializedRTCError
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	f := &RTCFailure{Reasons: v.Reasons, Outcome: v.Outcome, Details: v.Details, Context: v.Context, Kind: v.Kind, ID: v.ID, Code: v.Code, Category: v.Category, Message: v.Message, Operation: v.Operation, AdapterID: v.AdapterID, Retryability: v.Retryability, OptionPath: v.OptionPath, Lifetime: v.Lifetime}
	if v.Native != nil {
		f.ProviderError = KnownValue(ProviderError{NativeCode: v.Native.Code, NativeName: v.Native.Name, Status: v.Native.Status, Protocol: v.Native.Protocol, OriginalAvailability: v.Native.OriginalAvailability})
	}
	switch v.Kind {
	case "operation":
		return &RTCOperationError{RTCFailure: f}, nil
	case "observation":
		return &RTCObservationError{RTCFailure: f}, nil
	default:
		return nil, fmt.Errorf("invalid RTC error kind %q", v.Kind)
	}
}

func (s *ConnectionState) UnmarshalJSON(data []byte) error {
	value, err := decodeLiteral(data, "idle", "offered", "preparing", "joining", "connected", "recovering", "leaving", "ended", "failed")
	if err == nil {
		*s = ConnectionState(value)
	}
	return err
}
func (s *CleanupStatus) UnmarshalJSON(data []byte) error {
	m, kind, err := unionFields(data, "kind", map[string][]string{"complete": {}, "pending": {"outstanding"}, "failed": {"errors", "outstanding"}, "unknown": {"reason"}}, nil)
	if err != nil {
		return err
	}
	type plain CleanupStatus
	raw := m["errors"]
	delete(m, "errors")
	b, _ := json.Marshal(m)
	var v plain
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	if kind == "failed" {
		var items []json.RawMessage
		if err = json.Unmarshal(raw, &items); err != nil {
			return err
		}
		v.Errors = make([]RTCError, 0, len(items))
		for _, item := range items {
			e, eerr := decodeRTCError(item)
			if eerr != nil {
				return eerr
			}
			v.Errors = append(v.Errors, e)
		}
	}
	*s = CleanupStatus(v)
	return nil
}
func (r *MediaEffectRequirement) UnmarshalJSON(data []byte) error {
	_, kind, err := unionFields(data, "kind", map[string][]string{"capture": {"value"}, "publication": {"value"}, "receive": {"value"}, "outbound-content": {"value"}}, nil)
	if err != nil {
		return err
	}
	type plain MediaEffectRequirement
	var v plain
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	allowed := map[string]map[string]bool{"capture": {"retained": true, "released": true, "unchanged": true}, "publication": {"retained": true, "removed": true, "unchanged": true}, "receive": {"unchanged": true}, "outbound-content": {"suppressed": true, "enabled": true}}
	if !allowed[kind][v.Value] {
		return fmt.Errorf("invalid effect %q", v.Value)
	}
	*r = MediaEffectRequirement(v)
	return nil
}
func (r *PlatformTrackHandle) UnmarshalJSON([]byte) error {
	return fmt.Errorf("platform track handles cannot be decoded from JSON")
}
func (r PlatformTrackHandle) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("platform track handles are in-process values")
}
func (r *RenderTarget) UnmarshalJSON(data []byte) error {
	_, _, err := unionFields(data, "kind", map[string][]string{"provider-managed": {"target"}}, nil)
	if err != nil {
		return err
	}
	type plain RenderTarget
	var v plain
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = RenderTarget(v)
	return nil
}
func (r *StatisticsTarget) UnmarshalJSON(data []byte) error {
	_, _, err := unionFields(data, "kind", map[string][]string{"session": {}, "publication": {"id"}, "subscription": {"id"}, "leg": {"id"}, "receive-source": {"source"}}, nil)
	if err != nil {
		return err
	}
	type plain StatisticsTarget
	var v plain
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = StatisticsTarget(v)
	return nil
}
func (r *CallDestination) UnmarshalJSON(data []byte) error {
	_, _, err := unionFields(data, "kind", map[string][]string{"sip": {"uri"}, "telephone": {"number"}, "provider": {"address"}}, nil)
	if err != nil {
		return err
	}
	type plain CallDestination
	var v plain
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = CallDestination(v)
	return nil
}
func decodeLiteral(data []byte, allowed ...string) (string, error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return "", err
	}
	for _, v := range allowed {
		if s == v {
			return s, nil
		}
	}
	return "", fmt.Errorf("invalid literal %q", s)
}
func (v *AdmissionStatus) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "unknown", "pending", "held", "admitted", "denied", "not-applicable")
	if e == nil {
		*v = AdmissionStatus(s)
	}
	return e
}
func (v *MediaReadiness) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "unknown", "inactive", "starting", "active", "degraded", "not-applicable")
	if e == nil {
		*v = MediaReadiness(s)
	}
	return e
}
func (v *ParticipantPresence) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "discovered", "present", "recovering", "departed", "unknown")
	if e == nil {
		*v = ParticipantPresence(s)
	}
	return e
}
func (v *MediaKind) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "audio", "video", "screen")
	if e == nil {
		*v = MediaKind(s)
	}
	return e
}
func (v *RecordingEffect) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "content-suppressed", "content-retained", "not-recorded", "unchanged")
	if e == nil {
		*v = RecordingEffect(s)
	}
	return e
}
func (v *CaptureState) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "acquiring", "active", "disabled", "interrupted", "stopped", "failed", "unknown")
	if e == nil {
		*v = CaptureState(s)
	}
	return e
}
func (v *PublicationState) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "discovered", "publishing", "published", "interrupted", "unpublishing", "unpublished", "failed")
	if e == nil {
		*v = PublicationState(s)
	}
	return e
}
func (v *SubscriptionState) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "discovered", "subscribing", "subscribed", "receiving", "interrupted", "unsubscribing", "unsubscribed", "failed", "unknown")
	if e == nil {
		*v = SubscriptionState(s)
	}
	return e
}
func (v *CallHoldState) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "active", "local-held", "remote-held", "both-held", "unknown")
	if e == nil {
		*v = CallHoldState(s)
	}
	return e
}
func (v *CallState) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "incoming", "outgoing", "dialing", "ringing", "early-media", "active", "held", "transferring", "unknown", "established", "terminating", "ended", "failed")
	if e == nil {
		*v = CallState(s)
	}
	return e
}
func (v *ExtensionKey) UnmarshalJSON(b []byte) error {
	s, e := decodeLiteral(b, "calling", "devices", "independent-publication", "manual-subscription", "statistics", "recording", "messaging", "screen-sharing", "encryption", "session-administration")
	if e == nil {
		*v = ExtensionKey(s)
	}
	return e
}
