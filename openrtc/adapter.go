package openrtc

import "context"

// SpecVersion is the specification semantic version; registration checks compatibility; ADP-002.
type SpecVersion struct {
	Major uint64 `json:"major"`
	Minor uint64 `json:"minor"`
	Patch uint64 `json:"patch"`
}

// VersionCompatibility is an inclusive/exclusive compatibility interval; ADP-002.
type VersionCompatibility struct {
	MinimumInclusive SpecVersion `json:"minimumInclusive"`
	MaximumExclusive SpecVersion `json:"maximumExclusive"`
}

// ProviderDescriptor qualifies artifact, platform, deployment and profile metadata; ADP-002, ADP-006, ADP-100.
type ProviderDescriptor struct {
	Artifact            string                  `json:"artifact"`
	Prerequisites       []string                `json:"prerequisites"`
	DefaultProvisioning *DefaultProvisioning    `json:"defaultProvisioning,omitempty"`
	DeliveryExecutor    string                  `json:"deliveryExecutor"`
	BridgeExecutor      string                  `json:"bridgeExecutor"`
	AdapterID           AdapterID               `json:"id"`
	AdapterVersion      string                  `json:"version"`
	Specification       VersionCompatibility    `json:"specification"`
	SDKArtifact         string                  `json:"nativeArtifact"`
	SDKVersion          Known[string]           `json:"nativeVersion"`
	ServerArtifact      Known[string]           `json:"serverVersion"`
	Platform            string                  `json:"platform"`
	Deployment          Known[DeploymentID]     `json:"-"`
	Profiles            []string                `json:"profiles"`
	CollisionPolicy     IdentityCollisionPolicy `json:"-"`
	Evidence            []EvidenceReference     `json:"evidence"`
}

// RTCAdapter is a registration and initialization factory; initialization performs no admission; ADP-002–003.
type RTCAdapter interface {
	Descriptor() ProviderDescriptor
	Schemas(ConfigurationLifetime) []ConfigurationSchema
	Initialize(context.Context, ProviderConfig, AdapterHost) Async[AdapterProvider]
}

// AdapterHost owns serialization and credential dispatch; ADP-003–005.
type AdapterHost interface {
	Observe(NormalizedObservation)
	Capabilities(CapabilitySnapshot)
	Fail(RTCError)
	Credentials(context.Context, CredentialRequest) Async[Credential]
}

// ClientHost is the endpoint-scoped host; RFC 0001 §3.3, ADP-003.
type ClientHost interface{ AdapterHost }

// SessionHost fences stale native completions by generation; ADP-003, ADP-005.
type SessionHost interface{ AdapterHost }

// NormalizedObservation is a keyed update with source qualifications; ADP-003, ADP-005, CORE-009.
type NormalizedObservation struct {
	Source     ObservationSource   `json:"source"`
	Kind       string              `json:"kind"`
	Generation GenerationID        `json:"generation"`
	Snapshot   *SessionSnapshot    `json:"snapshot,omitempty"`
	Status     any                 `json:"status,omitempty"`
	Call       RTCCallSession      `json:"call,omitempty"`
	Message    *MessageObservation `json:"message,omitempty"`
}

// ObservationSource preserves native source qualifications; CORE-008–009, ADP-003, ADP-005.
type ObservationSource struct {
	Origin          string                `json:"origin"`
	NativeID        Known[ScopedNativeID] `json:"nativeId"`
	NativeTimestamp Known[Instant]        `json:"nativeTimestamp"`
	NativeSequence  Known[ScopedSequence] `json:"nativeSequence"`
	Evidence        []EvidenceReference   `json:"evidence"`
}

// AdapterProvider owns the SDK with separate participant and trusted factories; ADP-001, ADP-004.
type AdapterProvider interface {
	CreateClient(context.Context, ClientConfig, ClientHost) Async[AdapterClient]
	CreateAdministration(context.Context, ServerAdminConfig) Async[RTCServerAdministration]
	Dispose(context.Context) Async[Void]
}

// AdapterClient is an endpoint bridge, not a room administration surface; ADP-003–005.
type AdapterClient interface {
	OpenSession(ConferenceTarget, SessionHost) AdapterSession
	Extension(ExtensionKey) any
	Configure(context.Context, ClientRuntimeCommand) Async[Void]
	Dispose(context.Context) Async[Void]
}

// AdapterSession bridges native sessions; the host commits serialized normalized observations; ADP-003, ADP-005.
type AdapterSession interface {
	Kind() string
	Start(any) ConnectionAttempt
	Snapshot() SessionSnapshot
	Configure(context.Context, SessionRuntimeCommand) Async[Void]
	Extension(ExtensionKey) any
	Leave(context.Context) Async[Void]
	Dispose(context.Context) Async[Void]
}

// ServerAdminConfig carries trusted credentials and separately scoped native settings; ADP-004, CFG-009.
type ServerAdminConfig struct {
	Credentials ServerCredentialSource `json:"-"`
	Endpoint    *ServiceEndpoint       `json:"endpoint,omitempty"`
	Extensions  ProviderExtensions     `json:"extensions,omitzero"`
}

// RTCServerAdministration has server authority never obtained from ClientConfig; ADP-001, ADP-004.
type RTCServerAdministration interface {
	Inspection() OwnerInspection
	Credentials() CredentialSnapshot
	Capabilities() RTCCapabilities
	Provisioner() SessionProvisioner
	Extension(ExtensionKey) any
	Dispose(context.Context) Async[Void]
}

// SessionProvisioner allocates trusted resources or reservations; SES-003–006, ADP-004.
type SessionProvisioner interface {
	Create(context.Context, CreateSessionOptions) Async[SessionDefinition]
}

// PublicationState never uses mute as an unpublish transition; STM-005.
type PublicationState string

// SubscriptionState distinguishes selection acceptance from receiving/rendering; MED-002, RFC 0001 §9.
type SubscriptionState string

// TrackPublication is a sending unit observation, not proof of independent control; MED-003.
type TrackPublication struct {
	Owner                ResourceRef                    `json:"owner"`
	SetSendingMuteIntent Known[bool]                    `json:"setSendingMuteIntent"`
	ID                   PublicationID                  `json:"id"`
	Generation           GenerationID                   `json:"generation"`
	NativeID             Known[ScopedNativeID]          `json:"nativeId"`
	Participant          Known[ParticipantConnectionID] `json:"participant"`
	State                PublicationState               `json:"state"`
	Kinds                []MediaKind                    `json:"kinds"`
	ControlUnit          string                         `json:"controlUnit"`
	Origin               string                         `json:"origin"`
	Source               Known[CaptureSourceID]         `json:"source"`
	LocalMuteIntent      Known[bool]                    `json:"localMuteIntent"`
	SourceDisableIntent  Known[bool]                    `json:"sourceDisableIntent"`
	EffectiveSuppression EffectiveSuppression           `json:"effectiveSuppression"`
}

// TrackSubscription may be automatic or policy-driven; MED-002.
type TrackSubscription struct {
	ID             SubscriptionID         `json:"id"`
	Generation     GenerationID           `json:"generation"`
	NativeID       Known[ScopedNativeID]  `json:"nativeId"`
	Source         ReceiveSourceReference `json:"source"`
	State          SubscriptionState      `json:"state"`
	Kinds          []MediaKind            `json:"kinds"`
	Authority      string                 `json:"authority"`
	Tracks         []TrackID              `json:"tracks"`
	RenderBindings []RenderBindingID      `json:"renderBindings"`
}

// PlatformTrackHandle uses scoped native references rather than fabricated DOM objects; MED-004.
type PlatformTrackHandle struct {
	Platform string          `json:"platform"`
	Handle   InProcessHandle `json:"-"`
}

// RemoteTrack is an optional raw received track surface; MED-004.
type RemoteTrack interface {
	Snapshot() TrackView
	Raw() PlatformTrackHandle
}

// LocalTrack is an optional raw captured track surface; raw rejects when unavailable; MED-004.
type LocalTrack interface {
	Snapshot() TrackView
	Raw() PlatformTrackHandle
}

// CaptureSource stop irreversibly releases owned capture, not necessarily publication; MED-006.
type CaptureSource interface {
	Inspection() OwnerInspection
	Snapshot() CaptureSourceView
	Tracks() []LocalTrack
	Stop(context.Context, ...OperationOptions) Async[Void]
}

// MediaControlModel honestly describes coupling and raw access; MED-001–004.
type MediaControlModel struct {
	Publication         string                `json:"publication"`
	Receive             ReceivePolicy         `json:"receive"`
	RawTrack            Known[bool]           `json:"rawTrack"`
	Effects             MediaOperationEffects `json:"effects"`
	ControlUnitCoverage string                `json:"controlUnitCoverage"`
	SetupMedia          Known[string]         `json:"setupMedia"`
}

// RenderTarget is an optional DOM or native sink; MED-004.
type RenderTarget struct {
	Kind     string                 `json:"kind"`
	Platform string                 `json:"platform,omitempty"`
	Sink     InProcessHandle        `json:"-"`
	Target   *PlaybackControlTarget `json:"target,omitempty"`
}

// RenderBinding success is not autoplay success; detach reports reception effects; MED-005.
type RenderBinding interface {
	Snapshot() RenderBindingView
	SetPlaybackMuted(context.Context, bool) Async[Void]
	RetryPlayback(context.Context) Async[Void]
	Detach(context.Context) Async[Void]
}

// MediaController verbs retain distinct postconditions; MED-001–006, RFC 0001 §8.
type MediaController interface {
	ShareSource(CaptureSourceID, SourceSharePermissions)
	Source(CaptureSourceID) CaptureSource
	Track(TrackID) LocalTrack
	Capture(context.Context, CaptureRequest) Async[CaptureSource]
	SetSending(context.Context, SendRequest) Async[MediaResources]
	SetMuted(context.Context, PublicationID, bool, ...OperationOptions) Async[Void]
	SetDisabled(context.Context, CaptureSourceID, bool, ...OperationOptions) Async[Void]
	SetReceivePolicy(context.Context, ReceivePolicyRequest) Async[Void]
	Render(context.Context, RenderRequest) Async[RenderBinding]
}
