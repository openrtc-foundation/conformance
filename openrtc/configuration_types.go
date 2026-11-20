package openrtc

import "context"

// ProviderConfig is public SDK setup without participant or issuer secrets (CFG-001, CFG-009).
type ProviderConfig struct {
	ApplicationID *string            `json:"applicationId,omitempty"`
	Diagnostics   *DiagnosticsPolicy `json:"diagnostics,omitempty"`
	Placement     *PlacementPolicy   `json:"placement,omitempty"`
	Extensions    ProviderExtensions `json:"extensions,omitzero"`
}

// CaptureDefaults do not acquire devices (CFG-001, MED-001).
type CaptureDefaults struct {
	Audio *bool `json:"audio,omitempty"`
	Video *bool `json:"video,omitempty"`
}

// ClientConfig identifies the endpoint and credential callback owner (CFG-001, CFG-009).
type ClientConfig struct {
	Identity        ApplicationIdentity `json:"identity"`
	Credentials     CredentialProvider  `json:"-"`
	Endpoint        *ServiceEndpoint    `json:"endpoint,omitempty"`
	CaptureDefaults *CaptureDefaults    `json:"captureDefaults,omitempty"`
	Requirements    []CapabilityRequest `json:"requirements,omitzero"`
	Placement       *PlacementPolicy    `json:"placement,omitempty"`
	Calling         *CallingPolicy      `json:"calling,omitempty"`
	Extensions      ProviderExtensions  `json:"extensions,omitzero"`
}

// SessionMetadata is presentation rather than authenticated identity or native options (CORE-005, CFG-001).
type SessionMetadata map[string]JSONValue

// ParticipantMetadata is presentation, not an identity assertion (CORE-005, CFG-004).
type ParticipantMetadata map[string]JSONValue

// SessionPolicy creation intents require scoped discovery (SES-003–004, CFG-001).
type SessionPolicy struct {
	Placement    []PlacementPolicy   `json:"placement,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitempty"`
	Extensions   ProviderExtensions  `json:"extensions,omitzero"`
}

// CreateSessionOptions allocates resources or reservations on a trusted surface (SES-003–006).
type CreateSessionOptions struct {
	Name         *string             `json:"name,omitempty"`
	Metadata     *SessionMetadata    `json:"metadata,omitempty"`
	Policy       *SessionPolicy      `json:"policy,omitempty"`
	Placement    *PlacementPolicy    `json:"placement,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitzero"`
	Extensions   ProviderExtensions  `json:"extensions,omitzero"`
}

// ParticipantJoinIntent does not grant authority (CORE-005, CAP-004).
type ParticipantJoinIntent struct {
	Metadata    *ParticipantMetadata `json:"metadata,omitempty"`
	DisplayName *string              `json:"displayName,omitempty"`
}

// InitialMediaIntent declares capture ownership and compound sending effects (MED-001, MED-006).
type InitialMediaIntent struct {
	Operations []InitialMediaOperation `json:"operations"`
}

// JoinSessionOptions keeps credentials on a separate channel (CFG-001, CFG-009).
type JoinSessionOptions struct {
	Encryption   *EncryptionRequest    `json:"encryption,omitempty"`
	Recovery     *RecoveryOptions      `json:"recovery,omitempty"`
	Completion   string                `json:"completion,omitempty"`
	Participant  ParticipantJoinIntent `json:"participant"`
	InitialMedia *InitialMediaIntent   `json:"initialMedia,omitempty"`
	Receive      *ReceivePolicyRequest `json:"receive,omitempty"`
	Placement    *PlacementPolicy      `json:"placement,omitempty"`
	Requirements []CapabilityRequest   `json:"requirements,omitzero"`
	Signal       context.Context       `json:"-"`
	Extensions   ProviderExtensions    `json:"extensions,omitzero"`
}

// ClientRuntimeCommand and SessionRuntimeCommand are separate lifetime holders.
type ClientRuntimeCommand struct {
	Target          string             `json:"target"`
	Diagnostics     *DiagnosticsPatch  `json:"diagnostics,omitempty"`
	CaptureDefaults *CaptureDefaults   `json:"captureDefaults,omitempty"`
	Extensions      ProviderExtensions `json:"extensions,omitzero"`
	Request         *AdapterCommand    `json:"request,omitempty"`
}
type SessionRuntimeCommand struct {
	Target         string                     `json:"target"`
	ID             *ParticipantConnectionID   `json:"id,omitempty"`
	Metadata       *ParticipantMetadata       `json:"metadata,omitempty"`
	Request        *MediaConfigurationRequest `json:"request,omitempty"`
	AdapterRequest *AdapterCommand            `json:"-"`
	Extensions     ProviderExtensions         `json:"extensions,omitzero"`
}

// SessionProvisioning describes four explicit modes (SES-003–006).
type SessionProvisioning struct {
	Mode           string                      `json:"mode"`
	Definition     *SessionDefinition          `json:"definition,omitempty"`
	Address        *SessionAddress             `json:"address,omitempty"`
	CreationPolicy *Known[SessionPolicy]       `json:"creationPolicy,omitempty"`
	DefinitionID   *Known[SessionDefinitionID] `json:"definitionId,omitempty"`
	Broker         SessionBroker               `json:"-"`
	Request        *BrokerSessionRequest       `json:"request,omitempty"`
}

// ResolvedSessionTarget excludes backend-brokered provisioning (SES-003, SES-006).
type ResolvedSessionTarget = TransferableSessionTarget

// ConferenceTarget is routing, not a call destination (RFC 0001 §5).
type ConferenceTarget struct {
	Existence    string              `json:"existence,omitempty"`
	Kind         string              `json:"kind"`
	Provisioning SessionProvisioning `json:"provisioning"`
}

// BrokerSessionRequest contains no privileged material (SES-006, CFG-009).
type BrokerSessionRequest struct {
	Application ApplicationIdentity `json:"application"`
	Resource    string              `json:"resource"`
}

// SessionBroker is an application-owned transport/auth/retry/rollback boundary (SES-006).
type SessionBroker interface {
	Resolve(context.Context, BrokerSessionRequest) Async[TransferableSessionTarget]
}

// CredentialProvider acquires endpoint credentials; renewal is optional (SES-008–010, CFG-009).
type CredentialProvider interface {
	Get(context.Context, CredentialRequest) Async[CredentialInput]
}
type CredentialRenewer interface {
	Renew(context.Context, CredentialMetadata, CredentialRequest) Async[CredentialInput]
}

// CredentialAudience distinguishes consumers from trusted server management (CFG-009, RFC 0001 §6).
type CredentialAudience string

// CredentialRequest is phase-specific and does not assume a token TTL (SES-008–010).
type CredentialRequest struct {
	AdapterID AdapterID           `json:"adapterId"`
	Audience  CredentialAudience  `json:"audience"`
	Subject   ApplicationIdentity `json:"subject"`
	Scope     CredentialScope     `json:"scope"`
	Phase     CredentialPhase     `json:"phase"`
	Reason    string              `json:"reason"`
}

// CredentialPhase separates admission, replacement and reconnect (SES-009–010).
type CredentialPhase string

// CredentialScope cannot silently widen on renewal (SES-009).
type CredentialScope struct {
	Kind     string                      `json:"kind"`
	Address  *SessionAddress             `json:"address,omitempty"`
	Account  *ProviderIdentity           `json:"account,omitempty"`
	Actions  []CapabilityReference       `json:"actions,omitempty"`
	Endpoint *ServiceEndpoint            `json:"endpoint,omitempty"`
	Target   *ServiceJobCredentialTarget `json:"target,omitempty"`
}

// OpaqueCredentialMaterial provides non-serializing access to bytes; never expose them in events or logs (CFG-009).
type OpaqueCredentialMaterial interface {
	Use(consumer func([]byte) any) any
}

// Credential combines scoped material with nonsecret lifecycle metadata (CFG-009, SES-009).
type Credential struct {
	Material OpaqueCredentialMaterial `json:"-"`
	Metadata CredentialMetadata       `json:"metadata"`
}

// CredentialMetadata is not bearer authority; unknown expiry remains unknown (SES-009–010).
type CredentialMetadata struct {
	ID        CredentialID            `json:"id"`
	Audience  CredentialAudience      `json:"audience"`
	Scope     CredentialScope         `json:"scope"`
	Subject   Known[ProviderIdentity] `json:"subject"`
	IssuedAt  Known[Instant]          `json:"issuedAt"`
	ExpiresAt Known[Instant]          `json:"expiresAt"`
}

// ExpiryEffect is scoped to a phase (STM-006, RFC 0001 §6).
type ExpiryEffect string

// CredentialLifecycle qualifies native replacement semantics with evidence (SES-009–010).
type CredentialLifecycle struct {
	ReplacementMode           string                 `json:"replacementMode"`
	AdmissionExpiry           ExpiryEffect           `json:"admissionExpiry"`
	LiveExpiry                ExpiryEffect           `json:"liveExpiry"`
	ReconnectExpiry           ExpiryEffect           `json:"reconnectExpiry"`
	RevocationEffect          ExpiryEffect           `json:"revocationEffect"`
	ReplacementAcknowledgment Known[CredentialPhase] `json:"replacementAcknowledgment"`
}

// CredentialResult acknowledgment is not proof of live reauthorization (SES-009).
type CredentialResult struct {
	Kind          string                     `json:"kind"`
	Metadata      *CredentialMetadata        `json:"metadata,omitempty"`
	KnownMetadata *Known[CredentialMetadata] `json:"-"`
	Reason        *string                    `json:"reason,omitempty"`
}

// CredentialState distinguishes work state from validity (STM-006).
type CredentialState string

// CredentialStatus contains metadata only (CFG-009, SES-008–010).
type CredentialStatus struct {
	// LeakedForSelfTest is used solely to inject the fake credential leak defect.
	LeakedForSelfTest string                    `json:"leakedForSelfTest,omitempty"`
	State             CredentialState           `json:"state"`
	Validity          string                    `json:"validity"`
	Metadata          Known[CredentialMetadata] `json:"metadata"`
	Lifecycle         CredentialLifecycle       `json:"lifecycle"`
	Replacement       Known[CredentialResult]   `json:"replacement"`
	RenewalError      Known[RTCError]           `json:"renewalError"`
}

// ServerCredentialSource is a separate trusted source, never accepted by CredentialProvider (ADP-001, CFG-009).
type ServerCredentialSource interface {
	Kind() string
	Acquire(context.Context) Async[OpaqueCredentialMaterial]
}

// PortableCapabilityID is a closed portable registry (CAP-001).
type PortableCapabilityID string

// CapabilityID includes adapter-namespaced vendor IDs (CAP-001, RFC 0001 §7).
type CapabilityID string

// PortableGuaranteeID requires scoped evidence (CAP-001–003).
type PortableGuaranteeID string

// GuaranteeID includes adapter-namespaced vendor guarantees (RFC 0001 §7, CAP-001).
type GuaranteeID string

// SupportDescription names a partial subset; unknown never means unsupported (CORE-002, CAP-001).
type SupportDescription struct {
	Level  string   `json:"level"`
	Reason string   `json:"reason"`
	Subset []string `json:"subset"`
}

// CapabilityScope is the request/resource scope (RFC 0001 §7, CAP-001).
type CapabilityScope struct {
	AdapterID   AdapterID      `json:"adapterId"`
	Artifact    string         `json:"artifact"`
	Platform    string         `json:"platform"`
	Profile     string         `json:"profile"`
	Authority   AuthorityPlane `json:"authority"`
	Resource    string         `json:"resource"`
	SessionKind string         `json:"sessionKind,omitempty"`
}

// GuaranteeBoundary is evidence-backed (CAP-001).
type GuaranteeBoundary struct {
	Resources  []CapabilityResource      `json:"resources"`
	Endpoints  Known[[]ProviderIdentity] `json:"endpoints"`
	Exclusions []string                  `json:"exclusions"`
	Direction  string                    `json:"direction"`
	Lifetime   ConfigurationLifetime     `json:"lifetime"`
}

// GuaranteeDescriptor describes a claim and permitted unmet handling (CAP-001–003).
type GuaranteeDescriptor struct {
	ID       GuaranteeID         `json:"id"`
	Value    Known[JSONValue]    `json:"value"`
	Boundary GuaranteeBoundary   `json:"boundary"`
	Level    string              `json:"level"`
	Unmet    string              `json:"unmet"`
	Evidence []EvidenceReference `json:"evidence"`
}

// ConstraintValue is the concrete value/range union (RFC 0001 §7).
type ConstraintValue struct {
	Kind    string   `json:"kind"`
	Value   any      `json:"value,omitempty"`
	Minimum *float64 `json:"minimum,omitempty"`
	Maximum *float64 `json:"maximum,omitempty"`
	Values  []string `json:"values,omitempty"`
}

// ConstraintName is a portable name or adapter-namespaced entry (RFC 0001 §7).
type ConstraintName string

// CapabilityConstraint includes unit, direction, lifetime and mutability (CAP-001–002).
type CapabilityConstraint struct {
	ID            ConstraintName        `json:"id"`
	Value         Known[JSONValue]      `json:"value"`
	Unit          string                `json:"unit"`
	Direction     string                `json:"direction"`
	ResourceScope string                `json:"resourceScope"`
	Lifetime      ConfigurationLifetime `json:"lifetime"`
	Mutability    string                `json:"mutability"`
}

// RequestedConstraint uses the same typed dimensions (CAP-002).
type RequestedConstraint = CapabilityConstraint

// CapabilityPredicate retains explicit unknown evaluation (CAP-002, CAP-005).
type CapabilityPredicate struct {
	Kind       string                `json:"kind"`
	Terms      []CapabilityPredicate `json:"terms,omitempty"`
	Term       *CapabilityPredicate  `json:"term,omitempty"`
	Subject    *string               `json:"subject,omitempty"`
	Reference  *string               `json:"reference,omitempty"`
	Operator   *string               `json:"operator,omitempty"`
	Value      *ConstraintValue      `json:"value,omitempty"`
	Evaluation string                `json:"evaluation"`
}

// AuthorizationRequirement is a separate principal/action/resource decision with freshness (CAP-004–005).
type AuthorizationRequirement struct {
	Principal Known[ApplicationIdentity] `json:"principal"`
	Action    CapabilityID               `json:"action"`
	Resource  ResourceRef                `json:"resource"`
	Grant     string                     `json:"grant"`
	Authority string                     `json:"authority"`
	Decision  string                     `json:"decision"`
	CheckedAt Known[Instant]             `json:"checkedAt"`
	ExpiresAt Known[Instant]             `json:"expiresAt"`
}

// CapabilityRuntime availability does not replace implementation support (CAP-001–005).
type CapabilityRuntime struct {
	State     string         `json:"state"`
	Reasons   []string       `json:"reasons"`
	CheckedAt Known[Instant] `json:"checkedAt"`
	ExpiresAt Known[Instant] `json:"expiresAt"`
}

// Dependencies from later binding sections.
// MediaKind does not imply one independently addressable track per kind (MED-003).
type MediaKind string

// AudioCaptureSettings expresses portable capture intents (MED-001).
type AudioCaptureSettings struct {
	Device           *DeviceID `json:"device,omitempty"`
	EchoCancellation *bool     `json:"echoCancellation,omitempty"`
	NoiseSuppression *bool     `json:"noiseSuppression,omitempty"`
}

// VideoCaptureSettings numeric intents have explicit units (MED-001, RFC 0001 §8).
type VideoCaptureSettings struct {
	Device    *DeviceID `json:"device,omitempty"`
	Width     *float64  `json:"width,omitempty"`
	Height    *float64  `json:"height,omitempty"`
	FrameRate *float64  `json:"frameRate,omitempty"`
}

// CaptureRequest acquires without publishing or sending (MED-001, MED-006).
type CaptureRequest struct {
	Options      *OperationOptions   `json:"options,omitempty"`
	Kinds        []MediaKind         `json:"kinds"`
	Ownership    string              `json:"ownership"`
	Requirements []CapabilityRequest `json:"requirements,omitempty"`
}

// MediaEffectRequirement names required postconditions (MED-001).
type MediaEffectRequirement struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// SendRequest names source, kinds and effects (MED-001, RFC 0001 §8).
type SendRequest struct {
	Publication     PublicationID            `json:"publication,omitempty"`
	Options         *OperationOptions        `json:"options,omitempty"`
	Source          *CaptureSourceID         `json:"source,omitempty"`
	Kinds           []MediaKind              `json:"kinds"`
	Sending         bool                     `json:"sending"`
	Ownership       string                   `json:"ownership,omitempty"`
	RequiredEffects *MediaEffectRequirements `json:"requiredEffects,omitempty"`
}

// MediaConfigurationRequest is a target-specific runtime media command (CFG-004, MED-001).
type MediaConfigurationRequest struct {
	Command     string                    `json:"command"`
	Publication PublicationID             `json:"publication,omitempty"`
	Muted       *bool                     `json:"muted,omitempty"`
	Source      CaptureSourceID           `json:"source,omitempty"`
	Disabled    *bool                     `json:"disabled,omitempty"`
	Request     *SendRequest              `json:"-"`
	Policy      *ReceivePolicyRequest     `json:"policy,omitempty"`
	Target      *PlaybackControlTarget    `json:"target,omitempty"`
	Replacement *SourceReplacementRequest `json:"-"`
}

// ReceivePolicy separates strategy and authority (MED-002).
type ReceivePolicy struct {
	Audio ReceiveKindPolicy `json:"audio"`
	Video ReceiveKindPolicy `json:"video"`
}

// ExtensionKey is a closed portable extension key (RFC 0001 §3.2).
type ExtensionKey string

// SessionDefinition allocation does not imply a running instance (SES-004).
type SessionDefinition struct {
	ID         SessionDefinitionID   `json:"id"`
	NativeID   Known[ScopedNativeID] `json:"nativeId"`
	Address    SessionAddress        `json:"address"`
	Allocation string                `json:"allocation"`
	Policy     Known[SessionPolicy]  `json:"policy"`
}

// GuaranteeRequirement states strength and boundary (CAP-002–003).
type GuaranteeRequirement struct {
	ID       GuaranteeID       `json:"id"`
	Value    JSONValue         `json:"value"`
	Boundary GuaranteeBoundary `json:"boundary"`
}

// CapabilityRequest states required versus preferred negotiation intent (CAP-002–004).
type CapabilityRequest struct {
	Kind           string                      `json:"-"`
	ID             CapabilityID                `json:"-"`
	Adapter        *AdapterCapabilityReference `json:"-"`
	Strength       string                      `json:"-"`
	Scope          CapabilityScope             `json:"-"`
	Operations     []string                    `json:"-"`
	Guarantees     []GuaranteeRequirement      `json:"-"`
	Constraints    []CapabilityConstraint      `json:"-"`
	SchemaID       string                      `json:"-"`
	SchemaRevision string                      `json:"-"`
	Payload        JSONValue                   `json:"-"`
}

const (
	CapabilitySessionConference       PortableCapabilityID = "session.conference"
	CapabilitySessionCreate           PortableCapabilityID = "session.create"
	CapabilityCallingDial             PortableCapabilityID = "calling.dial"
	CapabilityCallingAccept           PortableCapabilityID = "calling.accept"
	CapabilityMediaCapture            PortableCapabilityID = "media.capture"
	CapabilityMediaPublishIndependent PortableCapabilityID = "media.publish.independent"
	CapabilityMediaSubscribeManual    PortableCapabilityID = "media.subscribe.manual"
	CapabilityMediaRender             PortableCapabilityID = "media.render"
	CapabilityMediaRawTrack           PortableCapabilityID = "media.raw-track"
	CapabilityMediaDevices            PortableCapabilityID = "devices.enumerate"
	CapabilityMediaStatistics         PortableCapabilityID = "media.statistics"
	CapabilityTelephonyHold           PortableCapabilityID = "telephony.hold"
	CapabilityTelephonyTransfer       PortableCapabilityID = "telephony.transfer.blind"
	CapabilityTelephonyDTMFSend       PortableCapabilityID = "telephony.dtmf.send"
	CapabilityTelephonyDTMFReceive    PortableCapabilityID = "telephony.dtmf.receive"
	CapabilityRecordingServer         PortableCapabilityID = "recording.server"
	CapabilityMessagingEphemeral      PortableCapabilityID = "messaging.ephemeral"
	CapabilityScreenSharing           PortableCapabilityID = "media.screen-share"
	CapabilityEncryptionMedia         PortableCapabilityID = "encryption.media"
	CapabilitySessionAdministration   PortableCapabilityID = "session.end"
)

const (
	GuaranteeEffectsCapture               PortableGuaranteeID = "effects.capture"
	GuaranteeEffectsOutbound              PortableGuaranteeID = "effects.outbound"
	GuaranteeEffectsPublication           PortableGuaranteeID = "effects.publication"
	GuaranteeEffectsReceive               PortableGuaranteeID = "effects.receive"
	GuaranteeEffectsReacquisitionRequired PortableGuaranteeID = "effects.reacquisitionRequired"
	GuaranteeEffectsRecording             PortableGuaranteeID = "effects.recording"
	GuaranteeEffectsPlayback              PortableGuaranteeID = "effects.playback"
	GuaranteeIdentityPreservation         PortableGuaranteeID = "identity.preservation"
	GuaranteePlaybackIsolation            PortableGuaranteeID = "playback.isolation"
	GuaranteeAdmissionExistingOnly        PortableGuaranteeID = "admission.existing-only"
	GuaranteeMessagingReliability         PortableGuaranteeID = "messaging.reliability"
	GuaranteeMessagingMaxPayload          PortableGuaranteeID = "messaging.max-payload"
	GuaranteeEncryptionMode               PortableGuaranteeID = "encryption.mode"
	GuaranteeEncryptionServerPlaintext    PortableGuaranteeID = "encryption.server-plaintext"
)
