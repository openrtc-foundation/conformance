package openrtc

import "context"

// MessageRequest keeps modes separate; no implied durable chat; EXT-001, CAP-001.
type MessageRequest struct {
	Mode         string              `json:"mode"`
	Recipients   *MessageRecipients  `json:"recipients,omitempty"`
	Conversation *ScopedNativeID     `json:"conversation,omitempty"`
	Content      MessageContent      `json:"content"`
	Requirements []CapabilityRequest `json:"requirements"`
}

// MessageRecipients Explicit destination set is not guaranteed delivery; EXT-001.
type MessageRecipients struct {
	Kind string                    `json:"kind"`
	IDs  []ParticipantConnectionID `json:"ids,omitempty"`
}

// MessageContent Typed application data, without unrestricted vendor dictionaries; RFC 0001 §3.2.
type MessageContent struct {
	Kind      string `json:"kind"`
	Text      string `json:"text,omitempty"`
	Bytes     []byte `json:"bytes,omitempty"`
	MediaType string `json:"mediaType,omitempty"`
}

// MessageReceipt Local acceptance and provider/recipient acknowledgment differ; CAP-001, EXT-001.
type MessageReceipt struct {
	Status     string `json:"status"`
	ByteLength uint64 `json:"byteLength"`
}

// MessageObservation Transient message delivery is not a durable session snapshot; CORE-008–009.
type MessageObservation struct {
	ID         Known[ScopedNativeID]          `json:"id"`
	Sender     Known[ParticipantConnectionID] `json:"sender"`
	Content    MessageContent                 `json:"content"`
	ObservedAt Instant                        `json:"observedAt"`
	Mode       string                         `json:"mode"`
}

// MessagingExtension Optional data facility with descriptor-qualified delivery semantics; EXT-001.
type MessagingExtension interface {
	Send(context.Context, EphemeralMessage) Async[MessageReceipt]
	OnMessage(func(ReceivedEphemeralMessage)) Unsubscribe
}

// ScreenShareRequest Screen acquisition and compound sending differ; MED-001, EXT-001.
type ScreenShareRequest struct {
	Mode            string                   `json:"mode"`
	Kinds           []MediaKind              `json:"kinds"`
	Ownership       string                   `json:"ownership"`
	RequiredEffects *MediaEffectRequirements `json:"requiredEffects,omitempty"`
}

// ScreenShareResult Actual capture/sending outcome, not an inferred publication; MED-001–004.
type ScreenShareResult struct {
	Source       Known[CaptureSourceID] `json:"source"`
	Publications []PublicationID        `json:"publications"`
}

// ScreenSharingExtension Optional screen operation; EXT-001, MED-001.
type ScreenSharingExtension interface {
	Start(context.Context, ScreenShareRequest) Async[ScreenShareResult]
}

// SenderEncoding Sender encoding definition is not receiver quality selection; CAP-001, RFC 0001 §3.2.
type SenderEncoding struct {
	ID                    string   `json:"id"`
	Active                bool     `json:"active"`
	MaxBitrateBps         *float64 `json:"maxBitrateBps,omitempty"`
	MaxFrameRate          *float64 `json:"maxFrameRate,omitempty"`
	ScaleResolutionDownBy *float64 `json:"scaleResolutionDownBy,omitempty"`
}

// EncodingRequest Encoding request respects discovered binding lifetime/mutability; CFG-001, EXT-001.
type EncodingRequest struct {
	Publication PublicationID      `json:"publication"`
	Encodings   []SenderEncoding   `json:"encodings"`
	Extensions  ProviderExtensions `json:"extensions,omitzero"`
}

// SimulcastExtension Optional sender encoding control, not a generic quality setter; EXT-001.
type SimulcastExtension interface {
	Configure(context.Context, EncodingRequest) Async[MediaOperationEffects]
}

// EncryptionCoverage Cryptographic coverage declares trust, exclusions and key access; CAP-001, EXT-001.
type EncryptionCoverage struct {
	Mode              Known[string]       `json:"mode"`
	Directions        []string            `json:"directions"`
	MediaKinds        []MediaKind         `json:"mediaKinds"`
	Data              Known[bool]         `json:"data"`
	ServerAccess      Known[bool]         `json:"serverAccess"`
	TrustedEndpoints  []ProviderIdentity  `json:"trustedEndpoints"`
	Effective         Known[bool]         `json:"effective"`
	Boundary          EncryptionBoundary  `json:"boundary"`
	KeyResponsibility Known[string]       `json:"keyResponsibility"`
	KeyLifecycle      Known[string]       `json:"keyLifecycle"`
	Evidence          []EvidenceReference `json:"evidence"`
}

// EncryptionKeySource is a non-serializing key channel; CFG-009. Go does not support generic methods.
type EncryptionKeySource interface {
	Get(context.Context, EncryptionKeyRequest) Async[OpaqueKeyMaterial]
}

// EncryptionRequest Explicit key setup; no silent encryption fallback; CAP-002–004.
type EncryptionRequest struct {
	Mode             string              `json:"mode"`
	Directions       []string            `json:"directions"`
	MediaKinds       []MediaKind         `json:"mediaKinds"`
	Data             bool                `json:"data"`
	TrustedEndpoints []ProviderIdentity  `json:"trustedEndpoints"`
	Boundary         EncryptionBoundary  `json:"boundary"`
	Keys             EncryptionKeySource `json:"-"`
	Requirements     []CapabilityRequest `json:"requirements,omitempty"`
}

// EncryptionOutcome Accepted configuration and effective coverage are distinct; CFG-004, CAP-001.
type EncryptionOutcome struct {
	Status   string              `json:"status"`
	Coverage *EncryptionCoverage `json:"coverage,omitempty"`
	Reason   string              `json:"reason,omitempty"`
}

// EncryptionExtension Optional explicit cryptographic control; EXT-001.
type EncryptionExtension interface {
	Coverage() EncryptionCoverage
	Configure(context.Context, EncryptionRequest) Async[Void]
}

// EndSessionRequest End-run intent does not implicitly delete definition or disable admission; SES-005.
type EndSessionRequest struct {
	Reason       *string             `json:"reason,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitzero"`
}

// AdminOutcome Command acknowledgment is separate from observed administrative effect; SES-005, EXT-002.
type AdminOutcome struct {
	Status              string                 `json:"status"`
	AcceptedEffects     []AdministrativeEffect `json:"acceptedEffects"`
	ObservedEffects     []AdministrativeEffect `json:"observedEffects"`
	CollateralEffects   []AdministrativeEffect `json:"collateralEffects"`
	SubsequentAdmission Known[string]          `json:"subsequentAdmission"`
}

// SessionAdministrationExtension Individually authorized run/definition/endpoint effects; SES-005, EXT-001–002.
type SessionAdministrationExtension interface {
	Plan(context.Context, AdministrationRequest) Async[AdministrationPlan]
	Execute(context.Context, AdministrationPlan) Async[AdminOutcome]
}
