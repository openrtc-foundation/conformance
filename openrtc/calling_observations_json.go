package openrtc

import (
	"encoding/json"
	"fmt"
)

func checkAudit1905Variant(data []byte, tag string, variants map[string][]string) (map[string]json.RawMessage, string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, "", err
	}
	var kind string
	if err := json.Unmarshal(fields[tag], &kind); err != nil {
		return nil, "", fmt.Errorf("missing or invalid %s", tag)
	}
	names, ok := variants[kind]
	if !ok {
		return nil, "", fmt.Errorf("invalid %s %q", tag, kind)
	}
	allowed := map[string]bool{tag: true}
	for _, name := range names {
		allowed[name] = true
		v, present := fields[name]
		if !present || string(v) == "null" {
			return nil, "", fmt.Errorf("%s requires %s", kind, name)
		}
	}
	for name := range fields {
		if !allowed[name] {
			return nil, "", fmt.Errorf("unexpected field %q for %s", name, kind)
		}
	}
	return fields, kind, nil
}
func encodeAudit1905Variant(tag, kind string, variants map[string][]string, fields map[string]any) ([]byte, error) {
	names, ok := variants[kind]
	if !ok {
		return nil, fmt.Errorf("invalid %s %q", tag, kind)
	}
	obj := map[string]any{tag: kind}
	for _, n := range names {
		obj[n] = fields[n]
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	_, _, err = checkAudit1905Variant(data, tag, variants)
	return data, err
}

func decodeOutcomeError(data []byte) (RTCError, error) {
	var e SerializedRTCError
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	f := &RTCFailure{Kind: e.Kind, Message: e.Message, ID: e.ID, Code: e.Code, Category: e.Category, Operation: e.Operation, AdapterID: e.AdapterID, Retryability: e.Retryability, OptionPath: e.OptionPath, Lifetime: e.Lifetime}
	if e.Native != nil {
		p := ProviderError{NativeCode: e.Native.Code, NativeName: e.Native.Name, Status: e.Native.Status, Protocol: e.Native.Protocol, OriginalAvailability: e.Native.OriginalAvailability}
		f.ProviderError = KnownValue(p)
	}
	switch e.Kind {
	case "operation":
		return &RTCOperationError{f}, nil
	case "observation":
		return &RTCObservationError{f}, nil
	default:
		return nil, fmt.Errorf("invalid RTC error kind %q", e.Kind)
	}
}
func (o *CallOutcome) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"status", "nativeReason", "protocolStatus", "error", "terminalCause"}, nil, map[string][]string{"status": {"accepted", "confirmed", "failed", "unknown"}}); err != nil {
		return err
	}
	type plain CallOutcome
	return json.Unmarshal(data, (*plain)(o))
}
func (v TransferRequest) MarshalJSON() ([]byte, error) {
	fields := map[string]any{"kind": v.Kind, "leg": v.Leg}
	if v.Kind == "blind" {
		fields["destination"] = v.Destination
	} else if v.Kind == "attended" {
		fields["consultation"] = v.Consultation
	} else {
		return nil, fmt.Errorf("invalid transfer kind %q", v.Kind)
	}
	return json.Marshal(fields)
}
func (v *TransferRequest) UnmarshalJSON(data []byte) error {
	if _, _, err := unionFields(data, "kind", map[string][]string{"blind": {"leg", "destination"}, "attended": {"leg", "consultation"}}, nil); err != nil {
		return err
	}
	type plain TransferRequest
	return json.Unmarshal(data, (*plain)(v))
}

var jobOutcomeVariants = map[string][]string{"accepted": {"outputs"}, "confirmed": {"outputs"}, "failed": {"error"}, "unknown": {"reason"}}

func (o JobOutcome) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("status", o.Status, jobOutcomeVariants, map[string]any{"outputs": o.Outputs, "error": o.Error, "reason": o.Reason})
}
func (o *JobOutcome) UnmarshalJSON(data []byte) error {
	fields, kind, err := checkAudit1905Variant(data, "status", jobOutcomeVariants)
	if err != nil {
		return err
	}
	var v struct {
		Outputs []JobOutput `json:"outputs"`
		Reason  string      `json:"reason"`
	}
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = JobOutcome{Status: kind, Outputs: v.Outputs, Reason: v.Reason}
	if kind == "failed" {
		o.Error, err = decodeOutcomeError(fields["error"])
	}
	return err
}

var messageRequestVariants = map[string][]string{"ephemeral": {"recipients", "content", "requirements"}, "durable": {"conversation", "content", "requirements"}}

func (v MessageRequest) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("mode", v.Mode, messageRequestVariants, map[string]any{"recipients": v.Recipients, "conversation": v.Conversation, "content": v.Content, "requirements": v.Requirements})
}
func (v *MessageRequest) UnmarshalJSON(data []byte) error {
	if _, _, err := checkAudit1905Variant(data, "mode", messageRequestVariants); err != nil {
		return err
	}
	type plain MessageRequest
	return json.Unmarshal(data, (*plain)(v))
}

var recipientsVariants = map[string][]string{"session": {}, "participants": {"ids"}}

func (v MessageRecipients) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("kind", v.Kind, recipientsVariants, map[string]any{"ids": v.IDs})
}
func (v *MessageRecipients) UnmarshalJSON(data []byte) error {
	if _, _, err := checkAudit1905Variant(data, "kind", recipientsVariants); err != nil {
		return err
	}
	type plain MessageRecipients
	return json.Unmarshal(data, (*plain)(v))
}

var contentVariants = map[string][]string{"text": {"text"}, "binary": {"bytes", "mediaType"}}

func (v MessageContent) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("kind", v.Kind, contentVariants, map[string]any{"text": v.Text, "bytes": v.Bytes, "mediaType": v.MediaType})
}
func (v *MessageContent) UnmarshalJSON(data []byte) error {
	if _, _, err := checkAudit1905Variant(data, "kind", contentVariants); err != nil {
		return err
	}
	type plain MessageContent
	return json.Unmarshal(data, (*plain)(v))
}

var encryptionVariants = map[string][]string{"effective": {"coverage"}, "pending": {"reason"}}

func (v EncryptionOutcome) MarshalJSON() ([]byte, error) {
	return encodeAudit1905Variant("status", v.Status, encryptionVariants, map[string]any{"coverage": v.Coverage, "reason": v.Reason})
}
func (v *EncryptionOutcome) UnmarshalJSON(data []byte) error {
	if _, _, err := checkAudit1905Variant(data, "status", encryptionVariants); err != nil {
		return err
	}
	type plain EncryptionOutcome
	return json.Unmarshal(data, (*plain)(v))
}

func checkAudit1905Object(data []byte, required, optional []string, literals map[string][]string) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("expected object")
	}
	allowed := map[string]bool{}
	for _, k := range append(required, optional...) {
		allowed[k] = true
	}
	for k, v := range m {
		if !allowed[k] {
			return fmt.Errorf("unexpected field %q", k)
		}
		if string(v) == "null" {
			return fmt.Errorf("null field %q", k)
		}
	}
	for _, k := range required {
		if _, ok := m[k]; !ok {
			return fmt.Errorf("missing field %q", k)
		}
	}
	for k, values := range literals {
		if raw, ok := m[k]; ok {
			var value string
			if err := json.Unmarshal(raw, &value); err != nil {
				return err
			}
			valid := false
			for _, v := range values {
				if value == v {
					valid = true
				}
			}
			if !valid {
				return fmt.Errorf("invalid %s %q", k, value)
			}
		}
	}
	return nil
}
func (v *DtmfRequest) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"leg", "digits", "method", "duration", "gap"}, []string{"fallback"}, map[string][]string{"method": []string{"rtp-event", "signaling", "in-band"}}); err != nil {
		return err
	}
	type plain DtmfRequest
	return json.Unmarshal(data, (*plain)(v))
}
func (v *DtmfObservation) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"leg", "digit", "observedAt", "nativeId"}, []string{}, map[string][]string{}); err != nil {
		return err
	}
	type plain DtmfObservation
	return json.Unmarshal(data, (*plain)(v))
}
func (v *RecordingRequest) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"target"}, []string{"placement", "requirements", "extensions"}, nil); err != nil {
		return err
	}
	type plain RecordingRequest
	return json.Unmarshal(data, (*plain)(v))
}
func (v *JobOutput) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"reference", "status", "mediaKinds", "storage"}, []string{}, map[string][]string{"status": []string{"pending", "available", "failed", "unknown"}}); err != nil {
		return err
	}
	type plain JobOutput
	return json.Unmarshal(data, (*plain)(v))
}
func (v *MessageReceipt) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"status", "byteLength"}, nil, map[string][]string{"status": {"local-queued", "provider-accepted"}}); err != nil {
		return err
	}
	type plain MessageReceipt
	return json.Unmarshal(data, (*plain)(v))
}
func (v *MessageObservation) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"id", "sender", "content", "observedAt", "mode"}, []string{}, map[string][]string{"mode": []string{"ephemeral", "durable"}}); err != nil {
		return err
	}
	type plain MessageObservation
	return json.Unmarshal(data, (*plain)(v))
}
func (v *ScreenShareRequest) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"mode", "kinds", "ownership"}, []string{"requiredEffects"}, map[string][]string{"mode": []string{"capture-only", "capture-and-send"}, "ownership": []string{"application", "session"}}); err != nil {
		return err
	}
	type plain ScreenShareRequest
	return json.Unmarshal(data, (*plain)(v))
}
func (v *ScreenShareResult) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"source", "publications"}, []string{}, map[string][]string{}); err != nil {
		return err
	}
	type plain ScreenShareResult
	return json.Unmarshal(data, (*plain)(v))
}
func (v *SenderEncoding) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"id", "active"}, []string{"maxBitrateBps", "maxFrameRate", "scaleResolutionDownBy"}, map[string][]string{}); err != nil {
		return err
	}
	type plain SenderEncoding
	return json.Unmarshal(data, (*plain)(v))
}
func (v *EncodingRequest) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"publication", "encodings"}, []string{"extensions"}, map[string][]string{}); err != nil {
		return err
	}
	type plain EncodingRequest
	return json.Unmarshal(data, (*plain)(v))
}
func (v *EncryptionCoverage) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"mode", "directions", "mediaKinds", "data", "serverAccess", "trustedEndpoints", "effective", "boundary", "keyResponsibility", "keyLifecycle", "evidence"}, nil, nil); err != nil {
		return err
	}
	type plain EncryptionCoverage
	return json.Unmarshal(data, (*plain)(v))
}
func (v *EndSessionRequest) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{}, []string{"reason", "requirements"}, map[string][]string{}); err != nil {
		return err
	}
	type plain EndSessionRequest
	return json.Unmarshal(data, (*plain)(v))
}
func (v *AdminOutcome) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"status", "acceptedEffects", "observedEffects", "collateralEffects", "subsequentAdmission"}, []string{}, map[string][]string{"status": []string{"accepted", "confirmed", "unknown"}}); err != nil {
		return err
	}
	type plain AdminOutcome
	return json.Unmarshal(data, (*plain)(v))
}
func (v *SpecVersion) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"major", "minor", "patch"}, []string{}, map[string][]string{}); err != nil {
		return err
	}
	type plain SpecVersion
	return json.Unmarshal(data, (*plain)(v))
}
func (v *VersionCompatibility) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"minimumInclusive", "maximumExclusive"}, []string{}, map[string][]string{}); err != nil {
		return err
	}
	type plain VersionCompatibility
	return json.Unmarshal(data, (*plain)(v))
}
func (v *ProviderDescriptor) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"id", "artifact", "version", "specification", "nativeArtifact", "nativeVersion", "serverVersion", "platform", "prerequisites", "profiles", "deliveryExecutor", "bridgeExecutor", "evidence"}, []string{"defaultProvisioning"}, map[string][]string{}); err != nil {
		return err
	}
	type plain ProviderDescriptor
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	for _, profile := range decoded.Profiles {
		if profile != "common" && profile != "conference" && profile != "calling" && profile != "conference-audiovisual" {
			return fmt.Errorf("invalid profile %q", profile)
		}
	}
	*v = ProviderDescriptor(decoded)
	return nil
}
func (v *ObservationSource) UnmarshalJSON(data []byte) error {
	if err := checkAudit1905Object(data, []string{"origin", "nativeId", "nativeTimestamp", "nativeSequence", "evidence"}, []string{}, map[string][]string{"origin": {"native", "reconciliation", "local-operation", "native-replay"}}); err != nil {
		return err
	}
	type plain ObservationSource
	return json.Unmarshal(data, (*plain)(v))
}
func (v *CallRejectReason) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, x := range []string{"declined", "busy", "unavailable"} {
		if s == x {
			*v = CallRejectReason(s)
			return nil
		}
	}
	return fmt.Errorf("invalid CallRejectReason %q", s)
}
func (v *JobStatus) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	for _, x := range []string{"pending", "starting", "running", "stopping", "completed", "failed", "cancelled", "unknown"} {
		if s == x {
			*v = JobStatus(s)
			return nil
		}
	}
	return fmt.Errorf("invalid JobStatus %q", s)
}

var observationVariants = map[string][]string{
	"snapshot":      {"source", "generation", "snapshot"},
	"credential":    {"source", "generation", "status"},
	"cleanup":       {"source", "generation", "status"},
	"incoming-call": {"source", "generation", "call"},
	"message":       {"source", "generation", "message"},
}

func (v NormalizedObservation) MarshalJSON() ([]byte, error) {
	switch v.Kind {
	case "credential":
		if _, ok := v.Status.(CredentialStatus); !ok {
			if _, ok := v.Status.(*CredentialStatus); !ok {
				return nil, fmt.Errorf("credential observation requires CredentialStatus")
			}
		}
	case "cleanup":
		if _, ok := v.Status.(CleanupStatus); !ok {
			if _, ok := v.Status.(*CleanupStatus); !ok {
				return nil, fmt.Errorf("cleanup observation requires CleanupStatus")
			}
		}
	}
	return encodeAudit1905Variant("kind", v.Kind, observationVariants, map[string]any{"source": v.Source, "generation": v.Generation, "snapshot": v.Snapshot, "status": v.Status, "call": v.Call, "message": v.Message})
}
func (v *NormalizedObservation) UnmarshalJSON(data []byte) error {
	fields, kind, err := checkAudit1905Variant(data, "kind", observationVariants)
	if err != nil {
		return err
	}
	var common struct {
		Source     ObservationSource `json:"source"`
		Generation GenerationID      `json:"generation"`
	}
	if err = json.Unmarshal(data, &common); err != nil {
		return err
	}
	out := NormalizedObservation{Kind: kind, Source: common.Source, Generation: common.Generation}
	switch kind {
	case "snapshot":
		var x SessionSnapshot
		err = json.Unmarshal(fields["snapshot"], &x)
		out.Snapshot = &x
	case "credential":
		var x CredentialStatus
		err = json.Unmarshal(fields["status"], &x)
		out.Status = x
	case "cleanup":
		var x CleanupStatus
		err = json.Unmarshal(fields["status"], &x)
		out.Status = x
	case "message":
		var x MessageObservation
		err = json.Unmarshal(fields["message"], &x)
		out.Message = &x
	case "incoming-call":
		return fmt.Errorf("incoming-call contains a live RTCCallSession and cannot be decoded from JSON")
	}
	if err != nil {
		return err
	}
	*v = out
	return nil
}
