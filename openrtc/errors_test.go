package openrtc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRTCFailureSerializationRedactsNativeError(t *testing.T) {
	secret := "native-secret"
	err := &RTCFailure{
		Kind: "operation", Message: "operation failed", Code: CredentialFailed,
		Cause:         errors.New(secret),
		ProviderError: KnownValue(ProviderError{Original: secret, NativeMessage: &secret, Related: []ProviderError{{Original: secret}}, OriginalAvailability: "available"}),
	}
	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("serialized error contains native secret: %s", data)
	}
	if !strings.Contains(string(data), `"originalAvailability":"available"`) {
		t.Fatalf("missing native availability: %s", data)
	}
	if errors.Unwrap(err) != nil {
		t.Fatal("default unwrap exposed unrestricted cause; RFC §9:544")
	}
}

func TestConcreteErrorsAndSerialization(t *testing.T) {
	cause := errors.New("cause")
	details := RTCErrorDetails{ID: "error-1", Code: Cancelled, Cause: cause, ProviderError: Unknown[ProviderError]()}
	for _, e := range []RTCError{NewRTCOperationError("failed", details), NewRTCObservationError("failed", details)} {
		if errors.Is(e, cause) {
			t.Fatal("default unwrap exposed unrestricted cause; RFC §9:544")
		}
		data, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"native":null`) || !strings.Contains(string(data), `"optionPath":null`) {
			t.Fatalf("unexpected serialization: %s", data)
		}
	}
	if NewRTCOperationError("", details).Kind != "operation" || NewRTCObservationError("", details).Kind != "observation" {
		t.Fatal("wrong error kind")
	}
}

func TestUnionSerializationPreservesRequiredEmptyValues(t *testing.T) {
	for _, tc := range []struct {
		value any
		want  string
	}{
		{NotApplicable[string](""), `{"status":"not-applicable","reason":""}`},
		{DefaultDescription{Source: "portable", Value: KnownValue[JSONValue](nil)}, `{"source":"portable","value":{"status":"known","value":null}}`},
		{ResourceRef{Kind: "unspecified"}, `{"kind":"unspecified","reason":""}`},
	} {
		data, err := json.Marshal(tc.value)
		if err != nil || string(data) != tc.want {
			t.Fatalf("got %s, %v; want %s", data, err, tc.want)
		}
	}
}

// RFC §9:544, §12.1:1044–1046: raw diagnostics require an explicit authorized request.
func TestDefaultErrorFormattingExcludesRawDiagnostics(t *testing.T) {
	marker := "raw-diagnostic-canary-8675309"
	original := map[string]any{"header": marker}
	details := RTCErrorDetails{Code: UnknownCode, Reasons: []string{"unknown"}, Outcome: "unknown", Cause: original, ProviderError: KnownValue(ProviderError{Original: original, NativeMessage: &marker})}
	failure := NewRTCOperationError("safe", details)
	for _, format := range []string{"%v", "%+v", "%#v"} {
		for _, v := range []any{failure, failure.RTCFailure} {
			if strings.Contains(fmt.Sprintf(format, v), marker) {
				t.Fatalf("default %s leaked raw native value", format)
			}
		}
	}
	if _, err := failure.RequestDiagnostics(context.Background()); err == nil {
		t.Fatal("unrequested unauthorized diagnostics available")
	}
	details.DiagnosticAccess = func(context.Context) (any, error) { return original, nil }
	authorized := NewRTCOperationError("safe", details)
	v, err := authorized.RequestDiagnostics(context.Background())
	if err != nil || v == nil {
		t.Fatal("authorized explicit request unavailable")
	}
	for _, format := range []string{"%v", "%+v", "%#v"} {
		if strings.Contains(fmt.Sprintf(format, authorized.RTCFailure), marker) {
			t.Fatalf("authorized diagnostics leaked into default %s", format)
		}
	}
}
