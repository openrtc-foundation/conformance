// Package openrtc contains the portable OpenRTC binding.
package openrtc

import "encoding/json"

// IdentityKind identifies an opaque identity domain (RFC 0001 §6, CORE-005).
type IdentityKind string

// OpaqueID represents a domain-separated local or provider identity (CORE-004–005).
// The concrete ID types below provide the domain separation in Go.
type OpaqueID string

// Distinct identifiers preserve the identity boundaries of CORE-005 and CORE-106.
type AdapterID string
type DeploymentID string
type ClientID string
type LocalSessionID string
type GenerationID string
type AttemptID string
type SessionDefinitionID string
type SessionInstanceID string
type ParticipantConnectionID string
type PublicationID string
type SubscriptionID string
type CaptureSourceID string
type TrackID string
type RenderBindingID string
type CallLegID string
type JobID string
type CredentialID string
type ErrorID string
type LocalEventID string
type LocalOwnerID string
type TransactionID string
type DeviceID string

// Revision is a monotonic local revision, not global causality (CORE-006–009).
type Revision uint64

// Instant is a UTC timestamp as an ISO 8601 string.
type Instant string

// Duration is an explicit duration in milliseconds (ERR-004).
type Duration struct {
	Milliseconds uint64 `json:"milliseconds"`
}

// Known distinguishes an unknown or inapplicable value from a known zero value (CORE-002).
type Known[T any] struct {
	Status string `json:"status"`
	Value  *T     `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func KnownValue[T any](value T) Known[T] { return Known[T]{Status: "known", Value: &value} }
func Unknown[T any]() Known[T]           { return Known[T]{Status: "unknown"} }
func NotApplicable[T any](reason string) Known[T] {
	return Known[T]{Status: "not-applicable", Reason: reason}
}

// MarshalJSON preserves the fields required by each CORE-002 status, including
// an empty inapplicability reason and a known null value.
func (k Known[T]) MarshalJSON() ([]byte, error) {
	switch k.Status {
	case "known":
		return json.Marshal(struct {
			Status string `json:"status"`
			Value  *T     `json:"value"`
		}{k.Status, k.Value})
	case "not-applicable":
		return json.Marshal(struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}{k.Status, k.Reason})
	default:
		return json.Marshal(struct {
			Status string `json:"status"`
		}{k.Status})
	}
}

// Unsubscribe removes a callback (CORE-007).
type Unsubscribe func()

// EvidenceReference is scoped evidence, never a conformance pass (ADP-006).
type EvidenceReference struct {
	Kind        string        `json:"kind"`
	Artifact    string        `json:"artifact"`
	Version     Known[string] `json:"version"`
	InspectedAt Instant       `json:"inspectedAt"`
	Source      string        `json:"source"`
	Gaps        []string      `json:"gaps"`
}

// ScopedNativeID retains namespace, provenance and lifetime (CORE-005).
type ScopedNativeID struct {
	Value      string              `json:"value"`
	AdapterID  AdapterID           `json:"adapterId"`
	Namespace  string              `json:"namespace"`
	Deployment Known[DeploymentID] `json:"deployment"`
	Origin     string              `json:"origin"`
	Lifetime   string              `json:"lifetime"`
	Generation Known[GenerationID] `json:"-"`
}
type ScopedSequence struct {
	Scope ScopedNativeID `json:"scope"`
	Value string         `json:"value"`
}
type ApplicationIdentity struct {
	Namespace string `json:"namespace"`
	Subject   string `json:"id"`
}
type ProviderIdentity struct {
	AdapterID  AdapterID           `json:"adapterId"`
	Deployment Known[DeploymentID] `json:"deployment"`
	Account    Known[string]       `json:"account"`
	Namespace  string              `json:"namespace"`
	ID         string              `json:"id"`
	Origin     string              `json:"origin"`
	Lifetime   string              `json:"lifetime"`
}
type ParticipantIdentity struct {
	Application   Known[ApplicationIdentity] `json:"application"`
	Provider      Known[ProviderIdentity]    `json:"provider"`
	DeviceID      Known[ScopedNativeID]      `json:"deviceId"`
	MembershipIDs []ScopedNativeID           `json:"membershipIds"`
}

// ServiceEndpoint is a nonsecret service location, not proof of placement (CFG-006, CFG-009).
type ServiceEndpoint struct {
	AdapterID AdapterID `json:"adapterId"`
	URL       string    `json:"url"`
}
type SessionAddress struct {
	AdapterID  AdapterID           `json:"adapterId"`
	Namespace  string              `json:"namespace"`
	Value      string              `json:"value"`
	Deployment Known[DeploymentID] `json:"deployment"`
}
type AuthorityPlane string

// ResourceRef preserves the kind of a referenced resource.
type ResourceRef struct {
	Kind   string `json:"kind"`
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// MarshalJSON retains the required reason even when it is empty.
func (r ResourceRef) MarshalJSON() ([]byte, error) {
	if r.Kind == "unspecified" {
		return json.Marshal(struct {
			Kind   string `json:"kind"`
			Reason string `json:"reason"`
		}{r.Kind, r.Reason})
	}
	return json.Marshal(struct {
		Kind string `json:"kind"`
		ID   string `json:"id"`
	}{r.Kind, r.ID})
}
