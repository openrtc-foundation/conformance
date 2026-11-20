package openrtc

import (
	"context"
	"encoding/json"
	"errors"
)

// ErrorCode is a portable classification; the constants mirror ERR-102.
type ErrorCode string

// The ten portable codes are defined by RFC §9:533–542, A.1:1183–1184.
const (
	InvalidArgument      ErrorCode = "invalid-argument"
	InvalidState         ErrorCode = "invalid-state"
	Unsupported          ErrorCode = "unsupported"
	PermissionDenied     ErrorCode = "permission-denied"
	AuthenticationFailed ErrorCode = "authentication-failed"
	Cancelled            ErrorCode = "cancelled"
	Timeout              ErrorCode = "timeout"
	Unavailable          ErrorCode = "unavailable"
	Internal             ErrorCode = "internal"
	UnknownCode          ErrorCode = "unknown"
	// Historical Go names are source aliases, never additional portable codes.
	UnsupportedOption        = Unsupported
	InvalidOption            = InvalidArgument
	UnsupportedCapability    = Unsupported
	CapabilityPending        = Unsupported
	PlacementUnsatisfied     = Unsupported
	OperationConflict        = InvalidState
	AuthorizationDenied      = PermissionDenied
	CredentialExpired        = AuthenticationFailed
	CredentialRevoked        = AuthenticationFailed
	CredentialFailed         = UnknownCode
	NetworkFailure           = Unavailable
	OperationTimeout         = Timeout
	MediaFailure             = UnknownCode
	ResourceNotFound         = InvalidState
	ResourceConflict         = InvalidState
	QuotaExceeded            = Unavailable
	ServiceUnavailable       = Unavailable
	ProtocolFailure          = UnknownCode
	ProviderFailure          = UnknownCode
	CleanupFailed            = UnknownCode
	AdapterContractViolation = Internal
	InternalFailure          = Internal
	IncompatibleAdapter      = InvalidArgument
)

// CoreCode maps historical bridge classifications by established condition.
// New bindings supply current codes directly and independently supply all reasons.
func CoreCode(code ErrorCode) ErrorCode {
	switch code {
	case "invalid-option", "incompatible-adapter":
		return InvalidArgument
	case "unsupported-option", "unsupported-capability", "capability-pending", "placement-unsatisfied":
		return Unsupported
	case "operation-conflict", "resource-not-found", "resource-conflict":
		return InvalidState
	case "authorization-denied":
		return PermissionDenied
	case "credential-expired", "credential-revoked":
		return AuthenticationFailed
	case "network-failure", "service-unavailable":
		return Unavailable
	case "operation-timeout":
		return Timeout
	case "adapter-contract-violation", "internal-failure":
		return Internal
	case "credential-failed", "media-failure", "provider-failure", "cleanup-failed", "protocol-failure", "quota-exceeded":
		return UnknownCode
	default:
		return code
	}
}
func IsCoreCode(code ErrorCode) bool {
	switch code {
	case InvalidArgument, InvalidState, Unsupported, PermissionDenied, AuthenticationFailed, Cancelled, Timeout, Unavailable, Internal, UnknownCode:
		return true
	}
	return false
}

type FailureDetails struct {
	KnownEffects       []Effect            `json:"knownEffects"`
	UnknownEffects     []Effect            `json:"unknownEffects"`
	SurvivingResources []ResourceRef       `json:"survivingResources"`
	UnstartedSteps     []string            `json:"unstartedSteps"`
	RejectedOptions    []RejectedOption    `json:"rejectedOptions"`
	RejectedConditions []RejectedCondition `json:"rejectedConditions"`
	CleanupFailures    []ResourceEffect    `json:"cleanupFailures"`
}
type RejectedOption struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Problem   string `json:"problem"`
}
type Resolution struct {
	Mechanism          string `json:"mechanism"`
	EvaluationBoundary string `json:"evaluationBoundary"`
}
type RejectedCondition struct {
	Condition  Condition   `json:"condition"`
	Assessment string      `json:"assessment"`
	Resolution *Resolution `json:"resolution,omitempty"`
}

type ErrorCategory string
type ExtensionLifetime string
type RecoveryAction string
type OptionPath []string

// SafeErrorContext contains the currently bound safe media failure context.
// Additional IDL context variants remain outside this partial binding.
type SafeErrorContext struct {
	OperationID    OperationID            `json:"operationId,omitempty"`
	MediaResources *MediaResources        `json:"mediaResources,omitempty"`
	PartialEffects *MediaOperationEffects `json:"partialEffects,omitempty"`
}

// RTCErrorDetails is the construction record for portable failures (ERR-001–004).
type RTCErrorDetails struct {
	Reasons          []string                           `json:"reasons"`
	Outcome          string                             `json:"outcome"`
	Details          FailureDetails                     `json:"details"`
	DiagnosticAccess func(context.Context) (any, error) `json:"-"`
	Context          *SafeErrorContext                  `json:"context,omitempty"`
	ID               ErrorID                            `json:"id"`
	Code             ErrorCode                          `json:"code"`
	Category         ErrorCategory                      `json:"category"`
	Operation        OperationRef                       `json:"operation"`
	AdapterID        Known[AdapterID]                   `json:"adapterId"`
	Resource         Known[ResourceRef]                 `json:"resource"`
	Retryability     string                             `json:"retryability"`
	RetryAfter       Known[Duration]                    `json:"retryAfter"`
	RecoveryAction   Known[RecoveryAction]              `json:"recoveryAction"`
	OptionPath       OptionPath                         `json:"optionPath,omitempty"`
	Lifetime         *ExtensionLifetime                 `json:"lifetime,omitempty"`
	PartialEffects   *OperationEffects                  `json:"partialEffects,omitempty"`
	ProviderError    Known[ProviderError]               `json:"providerError"`
	Cause            any                                `json:"cause,omitempty"`
}

// OperationRef provides portable operation correlation (ERR-003).
type OperationRef struct {
	Name       string              `json:"name"`
	Attempt    Known[AttemptID]    `json:"attempt"`
	Generation Known[GenerationID] `json:"generation"`
}

// ProviderError retains the original native failure in process (ERR-001, ERR-004).
type ProviderError struct {
	Original             any               `json:"original"`
	NativeCode           any               `json:"nativeCode,omitempty"`
	NativeName           *string           `json:"nativeName,omitempty"`
	NativeMessage        *string           `json:"nativeMessage,omitempty"`
	NativeState          *string           `json:"nativeState,omitempty"`
	Protocol             *string           `json:"protocol,omitempty"`
	Status               *float64          `json:"status,omitempty"`
	Provenance           EvidenceReference `json:"provenance"`
	Related              []ProviderError   `json:"related"`
	OriginalAvailability string            `json:"originalAvailability"`
}

// RTCFailure preserves native errors and causes in process. Its JSON representation
// excludes native originals, messages, causes and related errors (ERR-114).
type RTCFailure struct {
	Reasons          []string
	Outcome          string
	Details          FailureDetails
	diagnosticAccess func(context.Context) (any, error)
	Context          *SafeErrorContext

	Kind           string
	Message        string
	ID             ErrorID
	Code           ErrorCode
	Category       ErrorCategory
	Operation      OperationRef
	AdapterID      Known[AdapterID]
	Resource       Known[ResourceRef]
	Retryability   string
	RetryAfter     Known[Duration]
	RecoveryAction Known[RecoveryAction]
	OptionPath     OptionPath
	Lifetime       *ExtensionLifetime
	PartialEffects *OperationEffects
	ProviderError  Known[ProviderError]
	Cause          any
}

func (e *RTCFailure) Error() string { return e.Message }

// Unwrap never exposes unrestricted provider diagnostics (RFC §9:544, §12.1).
func (e *RTCFailure) Unwrap() error { return nil }
func (e *RTCFailure) RequestDiagnostics(ctx context.Context) (any, error) {
	if e.diagnosticAccess == nil {
		return nil, errors.New("diagnostic access denied")
	}
	return e.diagnosticAccess(ctx)
}

// SerializedProviderError is the redacted native classification (ERR-114).
type SerializedProviderError struct {
	Code                 any      `json:"code"`
	Name                 *string  `json:"name"`
	Status               *float64 `json:"status"`
	Protocol             *string  `json:"protocol"`
	OriginalAvailability string   `json:"originalAvailability"`
}

// SerializedRTCError is the safe public form of an RTC error (ERR-114).
type SerializedRTCError struct {
	Reasons      []string                 `json:"reasons"`
	Outcome      string                   `json:"outcome"`
	Details      FailureDetails           `json:"details"`
	Context      *SafeErrorContext        `json:"context,omitempty"`
	Name         string                   `json:"name"`
	Kind         string                   `json:"kind"`
	ID           ErrorID                  `json:"id"`
	Code         ErrorCode                `json:"code"`
	Category     ErrorCategory            `json:"category"`
	Message      string                   `json:"message"`
	Operation    OperationRef             `json:"operation"`
	AdapterID    Known[AdapterID]         `json:"adapterId"`
	Retryability string                   `json:"retryability"`
	OptionPath   OptionPath               `json:"optionPath"`
	Lifetime     *ExtensionLifetime       `json:"lifetime"`
	Native       *SerializedProviderError `json:"native"`
}

// ToJSON excludes native originals, messages, causes and related errors, which
// may contain credentials. Adapters must keep Message free of secrets (ERR-114).
func (e *RTCFailure) ToJSON() SerializedRTCError {
	var native *SerializedProviderError
	if e.ProviderError.Status == "known" && e.ProviderError.Value != nil {
		p := e.ProviderError.Value
		native = &SerializedProviderError{p.NativeCode, p.NativeName, p.Status, p.Protocol, p.OriginalAvailability}
	}
	return SerializedRTCError{Reasons: e.Reasons, Outcome: e.Outcome, Details: e.Details, Context: e.Context, Name: "RTCError", Kind: e.Kind, ID: e.ID, Code: e.Code, Category: e.Category, Message: e.Message, Operation: e.Operation, AdapterID: e.AdapterID, Retryability: e.Retryability, OptionPath: e.OptionPath, Lifetime: e.Lifetime, Native: native}
}

func (e *RTCFailure) MarshalJSON() ([]byte, error) { return json.Marshal(e.ToJSON()) }

func newRTCFailure(kind, message string, d RTCErrorDetails) *RTCFailure {
	// Construction deliberately strips all raw originals from default public fields.

	if d.ProviderError.Value != nil {
		p := *d.ProviderError.Value
		p.Original = nil
		p.NativeMessage = nil
		p.NativeState = nil
		p.Related = nil
		d.ProviderError = KnownValue(p)
	}
	return &RTCFailure{Reasons: d.Reasons, Outcome: d.Outcome, Details: d.Details, diagnosticAccess: d.DiagnosticAccess,
		Kind: kind, Message: message, ID: d.ID, Code: CoreCode(d.Code), Category: d.Category,
		Operation: d.Operation, AdapterID: d.AdapterID, Resource: d.Resource, Retryability: d.Retryability,
		RetryAfter: d.RetryAfter, RecoveryAction: d.RecoveryAction, OptionPath: d.OptionPath, Context: d.Context,
		Lifetime: d.Lifetime, PartialEffects: d.PartialEffects, ProviderError: d.ProviderError}
}

// RTCOperationError is a rejected operation retaining its initiating cause (ERR-001, ERR-003).
type RTCOperationError struct{ *RTCFailure }

func NewRTCOperationError(message string, details RTCErrorDetails) *RTCOperationError {
	return &RTCOperationError{newRTCFailure("operation", message, details)}
}

// RTCObservationError is an asynchronous failure correlated by ID (ERR-003).
type RTCObservationError struct{ *RTCFailure }

func NewRTCObservationError(message string, details RTCErrorDetails) *RTCObservationError {
	return &RTCObservationError{newRTCFailure("observation", message, details)}
}

// RTCError is the portable operation or observation error union (ERR-001–004).
type RTCError interface {
	error
	ToJSON() SerializedRTCError
	rtcError()
}

func (*RTCOperationError) rtcError()   {}
func (*RTCObservationError) rtcError() {}
