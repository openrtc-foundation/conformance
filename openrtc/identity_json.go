package openrtc

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// bindingObject enforces the closed object shapes of the reference binding.
func bindingObject(data []byte, required, optional string) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("expected object")
	}
	allowed := map[string]bool{}
	for _, name := range strings.Fields(required + " " + optional) {
		allowed[name] = true
	}
	for name := range fields {
		if !allowed[name] {
			return nil, fmt.Errorf("unknown property %q", name)
		}
	}
	for _, name := range strings.Fields(required) {
		if _, ok := fields[name]; !ok {
			return nil, fmt.Errorf("missing property %q", name)
		}
	}
	return fields, nil
}
func bindingLiteral(value string, choices string) error {
	for _, choice := range strings.Split(choices, "|") {
		if value == choice {
			return nil
		}
	}
	return fmt.Errorf("invalid literal %q", value)
}
func (k *Known[T]) UnmarshalJSON(data []byte) error {
	var discriminator struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return err
	}
	var required string
	switch discriminator.Status {
	case "known":
		required = "status value"
	case "unknown":
		required = "status"
	case "not-applicable":
		required = "status reason"
	default:
		return fmt.Errorf("invalid Known status %q", discriminator.Status)
	}
	fields, err := bindingObject(data, required, "")
	if err != nil {
		return err
	}
	var result Known[T]
	result.Status = discriminator.Status
	if raw, ok := fields["value"]; ok {
		var value T
		if string(raw) == "null" {
			t := reflect.TypeOf((*T)(nil)).Elem()
			switch t.Kind() {
			case reflect.Interface, reflect.Pointer, reflect.Map, reflect.Slice:
			default:
				return fmt.Errorf("null known value")
			}
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			return err
		}
		result.Value = &value
	}
	if raw, ok := fields["reason"]; ok {
		if string(raw) == "null" {
			return fmt.Errorf("null reason")
		}
		if err := json.Unmarshal(raw, &result.Reason); err != nil {
			return err
		}
	}
	*k = result
	return nil
}
func (r *ResourceRef) UnmarshalJSON(data []byte) error {
	var discriminator struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return err
	}
	if err := bindingLiteral(discriminator.Kind, "provider|client|session|definition|instance|participant|publication|subscription|capture|call-leg|job|device|unspecified"); err != nil {
		return err
	}
	required := "kind id"
	if discriminator.Kind == "unspecified" {
		required = "kind reason"
	}
	fields, err := bindingObject(data, required, "")
	if err != nil {
		return err
	}
	var result ResourceRef
	result.Kind = discriminator.Kind
	name := "id"
	dest := &result.ID
	if result.Kind == "unspecified" {
		name = "reason"
		dest = &result.Reason
	}
	if string(fields[name]) == "null" {
		return fmt.Errorf("null %s", name)
	}
	if err := json.Unmarshal(fields[name], dest); err != nil {
		return err
	}
	*r = result
	return nil
}

func (v *Duration) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "milliseconds", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain Duration
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = Duration(result)
	return nil
}

func (v *EvidenceReference) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "kind artifact version inspectedAt source gaps", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain EvidenceReference
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := bindingLiteral(result.Kind, "fact(verified)|inference|design-proposal"); err != nil {
		return err
	}
	*v = EvidenceReference(result)
	return nil
}

func (v *ScopedNativeID) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "value adapterId namespace deployment origin lifetime", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ScopedNativeID
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := bindingLiteral(result.Origin, "provider|adapter"); err != nil {
		return err
	}
	if err := bindingLiteral(result.Lifetime, "account|definition|instance|generation|unknown"); err != nil {
		return err
	}
	*v = ScopedNativeID(result)
	return nil
}

func (v *ScopedSequence) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "scope value", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ScopedSequence
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = ScopedSequence(result)
	return nil
}

func (v *ApplicationIdentity) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "namespace id", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ApplicationIdentity
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = ApplicationIdentity(result)
	return nil
}

func (v *ProviderIdentity) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "adapterId deployment account namespace id origin lifetime", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ProviderIdentity
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = ProviderIdentity(result)
	return nil
}

func (v *ParticipantIdentity) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "application provider deviceId nativeMembershipIds", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ParticipantIdentity
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = ParticipantIdentity(result)
	return nil
}

func (v *ServiceEndpoint) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "adapterId url", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain ServiceEndpoint
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = ServiceEndpoint(result)
	return nil
}

func (v *SessionAddress) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "adapterId namespace value deployment", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain SessionAddress
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	*v = SessionAddress(result)
	return nil
}

func (v *OperationRef) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "name attempt generation", "")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain OperationRef
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := bindingLiteral(result.Name, "initialize|create-client|open-session|join|leave|cancel|configure|negotiate|credential-get|credential-renew|credential-apply|capture|send|mute|disable|publish|unpublish|subscribe|unsubscribe|render|detach|stop|dial|accept|reject|terminate|hold|transfer|dtmf|record|message|encrypt|provision|end|delete|remove|dispose|observe|extension"); err != nil {
		return err
	}
	*v = OperationRef(result)
	return nil
}

func (v *ProviderError) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "original provenance related originalAvailability", "nativeCode nativeName nativeMessage nativeState protocol status")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "original" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	if raw, ok := fields["nativeCode"]; ok {
		var code any
		if err := json.Unmarshal(raw, &code); err != nil {
			return err
		}
		switch code.(type) {
		case string, float64:
		default:
			return fmt.Errorf("invalid nativeCode")
		}
	}
	type plain ProviderError
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := bindingLiteral(result.OriginalAvailability, "available|redacted|unavailable"); err != nil {
		return err
	}
	*v = ProviderError(result)
	return nil
}

func (v *IdentityKind) UnmarshalJSON(data []byte) error {
	var s string
	if string(data) == "null" {
		return fmt.Errorf("null IdentityKind")
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err := bindingLiteral(s, "adapter|deployment|client|local-session|generation|attempt|definition|instance|participant|publication|subscription|capture|track|render|call-leg|job|credential|error|event|owner|transaction|device"); err != nil {
		return err
	}
	*v = IdentityKind(s)
	return nil
}

func (v *AuthorityPlane) UnmarshalJSON(data []byte) error {
	var s string
	if string(data) == "null" {
		return fmt.Errorf("null AuthorityPlane")
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err := bindingLiteral(s, "client|server-admin|service|protocol|deployment"); err != nil {
		return err
	}
	*v = AuthorityPlane(s)
	return nil
}

func (v *ErrorCategory) UnmarshalJSON(data []byte) error {
	var s string
	if string(data) == "null" {
		return fmt.Errorf("null ErrorCategory")
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err := bindingLiteral(s, "configuration|capability|authentication|authorization|credential|placement|state|cancellation|network|media|resource|quota|service|protocol|internal|unknown"); err != nil {
		return err
	}
	*v = ErrorCategory(s)
	return nil
}

func (v *ErrorCode) UnmarshalJSON(data []byte) error {
	var s string
	if string(data) == "null" {
		return fmt.Errorf("null ErrorCode")
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if !IsCoreCode(ErrorCode(s)) {
		return fmt.Errorf("invalid core code %q", s)
	}
	*v = ErrorCode(s)
	return nil
}

func (v *RecoveryAction) UnmarshalJSON(data []byte) error {
	var s string
	if string(data) == "null" {
		return fmt.Errorf("null RecoveryAction")
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err := bindingLiteral(s, "retry-after-change|request-authorization|obtain-credential|fresh-session|inspect-cleanup|contact-operator"); err != nil {
		return err
	}
	*v = RecoveryAction(s)
	return nil
}

func (d *RTCErrorDetails) UnmarshalJSON(data []byte) error {
	fields, err := bindingObject(data, "id code category operation adapterId resource retryability retryAfter recoveryAction providerError", "optionPath lifetime partialEffects cause")
	if err != nil {
		return err
	}
	for name, raw := range fields {
		if name != "cause" && string(raw) == "null" {
			return fmt.Errorf("null property %q", name)
		}
	}
	type plain RTCErrorDetails
	var result plain
	if err := json.Unmarshal(data, &result); err != nil {
		return err
	}
	if err := bindingLiteral(result.Retryability, "never|after-change|transient|unknown"); err != nil {
		return err
	}
	*d = RTCErrorDetails(result)
	return nil
}
