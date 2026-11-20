package openrtc

import "context"

// CapabilityAuthorization: discovery is never a grant; CAP-004.
type CapabilityAuthorization struct {
	Requirement string                     `json:"requirement"`
	Authorities []AuthorizationRequirement `json:"authorities"`
	Decision    string                     `json:"decision"`
}

// CapabilityDescriptor is a scoped semantic descriptor; CAP-001–005.
type CapabilityDescriptor struct {
	Details           CapabilityDetails         `json:"details"`
	IdentityCollision string                    `json:"identityCollision"`
	ID                CapabilityID              `json:"id"`
	Revision          Revision                  `json:"revision"`
	Support           SupportDescription        `json:"support"`
	Scope             CapabilityDescriptorScope `json:"scope"`
	Guarantees        []GuaranteeDescriptor     `json:"guarantees"`
	Constraints       []CapabilityConstraint    `json:"constraints"`
	Prerequisites     []CapabilityPrerequisite  `json:"prerequisites"`
	Availability      CapabilityAvailability    `json:"availability"`
	Authorization     CapabilityAuthorization   `json:"authorization"`
	Evidence          []EvidenceReference       `json:"evidence"`
}
type CapabilityDescriptorScope = CapabilityScope

// CapabilityRegistry is the closed portable descriptor map; vendor descriptors remain adapter-namespaced; CAP-001, RFC 0001 §7.
type CapabilityRegistry map[PortableCapabilityID]CapabilityDescriptor

// CapabilityRequestRegistry is the closed portable request map; CAP-002, RFC 0001 §7.
type CapabilityRequestRegistry map[PortableCapabilityID]CapabilityRequest

// GuaranteeRegistry entries do not imply support; CAP-001, RFC 0001 §7.
type GuaranteeRegistry map[PortableGuaranteeID]GuaranteeDescriptor

// CapabilitySnapshot descriptors are plans, never bearer authority; CAP-004–005.
type CapabilitySnapshot struct {
	Revision       Revision                      `json:"revision"`
	Scope          CapabilityScope               `json:"scope"`
	Entries        []CapabilityDescriptor        `json:"entries"`
	AdapterEntries []AdapterCapabilityDescriptor `json:"adapterEntries"`
}

// NegotiationEntry explicitly blocks the dependent side effect when authorization is required; CAP-002, CAP-004.
type NegotiationEntry struct {
	Request            CapabilityRequest        `json:"request"`
	Status             string                   `json:"status"`
	DescriptorRevision Known[Revision]          `json:"descriptorRevision"`
	Reason             string                   `json:"reason"`
	Accepted           Known[CapabilityRequest] `json:"accepted"`
	Pending            []CapabilityPrerequisite `json:"pending"`
	Alternatives       []CapabilityRequest      `json:"alternatives"`
}

// NegotiationResult gates execution of required entries; CAP-002–004.
type NegotiationResult struct {
	Entries []NegotiationEntry `json:"entries"`
}

// RTCCapabilities provides discovery, negotiation, and invalidation; CAP-001–005.
type RTCCapabilities interface {
	Snapshot() CapabilitySnapshot
	Negotiate(context.Context, []CapabilityRequest) Async[NegotiationResult]
	OnChanged(func(CapabilitySnapshot)) Unsubscribe
}

// RTCAdapterRegistry rejects duplicates and incompatible specification ranges; ADP-002.
type RTCAdapterRegistry interface {
	Register(RTCAdapter) (Registration, error)
	Initialize(context.Context, AdapterID, ProviderConfig) InitializationOperation
}

// RTCProvider is the initialized SDK owner, independent of admission; CORE-003, ADP-003.
type RTCProvider interface {
	ExportDefinition(SessionDefinitionID) (SessionDefinitionDescriptor, error)
	ImportDefinition(context.Context, SessionDefinitionDescriptor) Async[SessionDefinition]
	Credentials() CredentialSnapshot
	Inspection() OwnerInspection
	Descriptor() ProviderDescriptor
	Capabilities() RTCCapabilities
	Schemas(ConfigurationLifetime) []ConfigurationSchema
	CreateClient(context.Context, ClientConfig) Async[RTCClient]
	Dispose(context.Context) Async[Void]
}

// RTCClient is an endpoint resource owner; RFC 0001 §3.1.
type RTCClient interface {
	Credentials() CredentialSnapshot
	Inspection() OwnerInspection
	ID() ClientID
	Capabilities() RTCCapabilities
	Sessions() map[LocalSessionID]RTCSession
	OpenSession(ConferenceTarget) RTCConferenceSession
	Extension(ExtensionKey) any
	Configure(context.Context, ClientRuntimeCommand) Async[Void]
	Dispose(context.Context) Async[Void]
}

// RTCSession represents distinct application interaction models; CORE-001, RFC 0001 §3.1.
type RTCSession interface {
	RTCSessionBase
	Kind() string
}

// RTCSessionBase is the common local connection generation surface; SES-002, STM-001.
type RTCSessionBase interface {
	Generation() GenerationID
	Inspection() SessionInspection
	ID() LocalSessionID
	Connection() SessionConnection
	Capabilities() RTCCapabilities
	Observation() SessionObservation
	Media() MediaController
	Extension(ExtensionKey) any
	Configure(context.Context, SessionRuntimeCommand) Async[Void]
	Dispose(context.Context) Async[Void]
}

// RTCConferenceSession construction is inert, with one join per handle; SES-001–002.
type RTCConferenceSession interface {
	RTCSession
	Target() ConferenceTarget
	Join(JoinSessionOptions) JoinAttempt
	Leave(context.Context) Async[Void]
}

// ConnectionState terminal states are scoped to one generation; STM-001–003.
type ConnectionState string

// AdmissionStatus is orthogonal to signaling and media; STM-002.
type AdmissionStatus string

// MediaReadiness is not inferred from connected; STM-002.
type MediaReadiness string

// RecoveryStatus keeps planes, native causes and continuity separate; STM-003.
type RecoveryStatus struct {
	Planes       []string       `json:"planes"`
	NativeState  Known[string]  `json:"nativeState"`
	NativeReason Known[string]  `json:"nativeReason"`
	Cause        string         `json:"cause"`
	Continuity   string         `json:"continuity"`
	RetryBudget  Known[float64] `json:"retryBudget"`
}

// CleanupStatus cannot claim remote rollback; STM-010–013.
type CleanupStatus struct {
	Kind        string     `json:"kind"`
	Outstanding []string   `json:"outstanding,omitempty"`
	Errors      []RTCError `json:"errors,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

// CleanupReport is queryable independently of listener disposal; STM-011–013.
type CleanupReport struct {
	Status     CleanupStatus       `json:"status"`
	Generation Known[GenerationID] `json:"generation"`
}

// CancellationReport fences success independently of native cleanup; STM-010–011.
type CancellationReport struct {
	Won     bool          `json:"won"`
	Mode    string        `json:"mode"`
	Cleanup CleanupReport `json:"cleanup"`
}

// SessionConnection is one local connection generation; terminal generations never reconnect; STM-001.
type SessionConnection interface {
	Snapshot() ConnectionSnapshot
	Leave(context.Context) Async[Void]
}

// ConnectionResult indicates signaling readiness, not necessarily admitted/usable media; STM-002.
type ConnectionResult struct {
	Generation    GenerationID        `json:"generation"`
	Connection    ConnectionSnapshot  `json:"connection"`
	Configuration ConfigurationResult `json:"configuration"`
}

// JoinResult preserves orthogonal admission state; STM-002, SES-002.
type JoinResult struct {
	Completion     string                         `json:"completion"`
	Achieved       string                         `json:"achieved"`
	Connection     ConnectionSnapshot             `json:"connection"`
	Admission      AdmissionStatus                `json:"admission"`
	Membership     Known[ParticipantConnectionID] `json:"membership"`
	InitialMedia   InitialMediaOutcome            `json:"initialMedia"`
	MediaReadiness MediaReadiness                 `json:"mediaReadiness"`
}

// ConnectionAttempt settles exactly once; late cancellation reports Won=false (RFC §8.5:487).
type ConnectionAttempt interface {
	ID() AttemptID
	Generation() GenerationID
	Result(context.Context) Async[ConnectionResult]
	Cancel(context.Context) Async[CancellationReport]
}

// JoinAttempt is a conference-specific setup attempt; SES-002, STM-010.
type JoinAttempt interface {
	ID() AttemptID
	Generation() GenerationID
	Result(context.Context) Async[JoinResult]
	Cancel(context.Context) Async[CancellationReport]
}

// SessionInstance is an observed active run and optional known resource identity; SES-004, CORE-005.
type SessionInstance struct {
	ID           SessionInstanceID          `json:"id"`
	NativeID     Known[ScopedNativeID]      `json:"nativeId"`
	Address      SessionAddress             `json:"address"`
	DefinitionID Known[SessionDefinitionID] `json:"definitionId"`
	Status       string                     `json:"status"`
}

// ParticipantPresence: partial absence cannot establish departure; STM-004.
type ParticipantPresence string

// RTCParticipant is an observed endpoint, not a person or fabricated call peer; CORE-004.
type RTCParticipant struct {
	ID           ParticipantConnectionID    `json:"id"`
	NativeID     Known[ScopedNativeID]      `json:"nativeId"`
	Identity     ParticipantIdentity        `json:"identity"`
	Presence     ParticipantPresence        `json:"presence"`
	DisplayName  Known[string]              `json:"displayName"`
	Metadata     Known[ParticipantMetadata] `json:"metadata"`
	NativeRole   Known[string]              `json:"nativeRole"`
	Publications []PublicationID            `json:"publications"`
}

// IdentityCollisionPolicy is discovery, never an assumed setter; SES-007.
type IdentityCollisionPolicy struct {
	Behavior string              `json:"behavior"`
	Scope    string              `json:"scope"`
	Evidence []EvidenceReference `json:"evidence"`
}

// RTCEventEmitter carries typed envelopes; CORE-006–009. The event name selects the payload type.
type RTCEventEmitter interface {
	On(name string, listener func(EventEnvelope[any])) Unsubscribe
	Once(name string, listener func(EventEnvelope[any])) Unsubscribe
	Off(name string, listener func(EventEnvelope[any]))
}

// EventEnvelope local ordering cannot establish global/source causality; CORE-006–009, STM-008.
type EventEnvelope[T any] struct {
	EventID         LocalEventID          `json:"eventId"`
	OwnerID         LocalOwnerID          `json:"ownerId"`
	Generation      GenerationID          `json:"generation"`
	Sequence        uint64                `json:"sequence"`
	Revision        Revision              `json:"revision"`
	ObservedAt      Instant               `json:"observedAt"`
	Origin          string                `json:"origin"`
	NativeID        Known[ScopedNativeID] `json:"nativeId"`
	NativeTimestamp Known[Instant]        `json:"nativeTimestamp"`
	NativeSequence  Known[ScopedSequence] `json:"nativeSequence"`
	Transaction     TransactionID         `json:"transaction"`
	Payload         T                     `json:"payload"`
}

// StateChange is a typed previous/current fact; inapplicable previous state is explicit; CORE-006.
type StateChange[T any] struct {
	Previous Known[T] `json:"previous"`
	Current  T        `json:"current"`
}

// EntityChange includes removal without fabricated terminal state; STM-004–005.
type EntityChange[T any] struct {
	Kind     string `json:"kind"`
	Previous any    `json:"previous"`
	Current  *T     `json:"current,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// SessionEvents has fixed keys; ordinary listeners do not replay a snapshot; CORE-006–009.
type SessionEvents struct {
	ConnectionChanged   StateChange[ConnectionSnapshot] `json:"connectionChanged"`
	AdmissionChanged    StateChange[AdmissionStatus]    `json:"admissionChanged"`
	ParticipantChanged  EntityChange[RTCParticipant]    `json:"participantChanged"`
	PublicationChanged  EntityChange[TrackPublication]  `json:"publicationChanged"`
	SubscriptionChanged EntityChange[TrackSubscription] `json:"subscriptionChanged"`
	CredentialChanged   StateChange[CredentialView]     `json:"credentialChanged"`
	CapabilitiesChanged StateChange[CapabilitySnapshot] `json:"capabilitiesChanged"`
	CleanupChanged      StateChange[CleanupStatus]      `json:"cleanupChanged"`
	SnapshotChanged     StateChange[SessionSnapshot]    `json:"snapshotChanged"`
	Error               RTCError                        `json:"error"`
}

// SessionObservation provides an atomic local watch handoff; upstream consistency remains qualified; STM-008–009.
type SessionObservation interface {
	RTCEventEmitter
	Snapshot() SessionSnapshot
	Watch(func(SessionSnapshot)) Unsubscribe
}

// ConnectionSnapshot is an immutable projection excluding bearer material; STM-001–003, CFG-009.
type ConnectionSnapshot struct {
	Attempt        Known[AttemptID]       `json:"attempt"`
	Instance       Known[SessionInstance] `json:"instance"`
	RecoveryPolicy RecoveryPolicy         `json:"recoveryPolicy"`
	Generation     GenerationID           `json:"-"`
	State          ConnectionState        `json:"state"`
	Admission      AdmissionStatus        `json:"admission"`
	MediaReadiness MediaReadiness         `json:"mediaReadiness"`
	Recovery       Known[RecoveryStatus]  `json:"recovery"`
	NativeState    Known[string]          `json:"-"`
	NativeReason   Known[string]          `json:"-"`
}

// SessionSnapshot is reconciled before event delivery, not a lossless provider history; STM-008–009.
// PopulationCoverage is explicit; a nil pointer means missing metadata, not
// unknown or complete. RFC §4:23, §7.4:282, A.1:1248–1255/1290.
type PopulationCoverage struct {
	Scope        CapabilityScope `json:"scope"`
	Completeness string          `json:"completeness"`
}
type SessionSnapshot struct {
	SourceCoverage        *PopulationCoverage                   `json:"sourceCoverage"`
	ReceiveSourceCoverage *PopulationCoverage                   `json:"receiveSourceCoverage"`
	ID                    LocalSessionID                        `json:"id"`
	Kind                  string                                `json:"kind"`
	Sources               map[CaptureSourceID]CaptureSourceView `json:"sources"`
	ReceiveSources        map[ReceiveSourceID]ReceiveSourceView `json:"receiveSources"`
	Tracks                map[TrackID]TrackView                 `json:"tracks"`
	Renders               map[RenderBindingID]RenderBindingView `json:"renders"`
	Legs                  map[CallLegID]CallLeg                 `json:"legs,omitempty"`
	Placement             []PlacementObservation                `json:"placement"`
	Encryption            EncryptionCoverage                    `json:"encryption"`
	ActiveSpeakers        Known[ActiveSpeakerObservation]       `json:"activeSpeakers"`
	NetworkQuality        Known[[]NetworkQualityObservation]    `json:"networkQuality"`

	Revision        Revision                             `json:"revision"`
	Generation      GenerationID                         `json:"generation"`
	Connection      ConnectionSnapshot                   `json:"connection"`
	Roster          RosterView                           `json:"roster"`
	Publications    map[PublicationID]TrackPublication   `json:"publications"`
	Subscriptions   map[SubscriptionID]TrackSubscription `json:"subscriptions"`
	Credentials     []CredentialView                     `json:"credentials"`
	Cleanup         CleanupStatus                        `json:"-"`
	Consistency     string                               `json:"-"`
	SourceFreshness Known[Instant]                       `json:"-"`
}

// RosterView coverage and counts are independent; call peers need not be roster entries; CORE-004, STM-004.
type RosterView struct {
	Entries        map[ParticipantConnectionID]RTCParticipant `json:"entries"`
	Completeness   string                                     `json:"completeness"`
	ObservedCount  uint64                                     `json:"observedCount"`
	ReportedCount  Known[uint64]                              `json:"reportedCount"`
	CoverageReason string                                     `json:"reason"`
}

// PublishOptions ensures independent publication cannot silently acquire devices or change mute; MED-001.
type PublishOptions struct {
	Options         *OperationOptions        `json:"options,omitempty"`
	Kinds           []MediaKind              `json:"kinds"`
	RequiredEffects *MediaEffectRequirements `json:"requiredEffects,omitempty"`
}

// UnpublishOptions negotiates capture retention/explicit owned-stop; MED-001, MED-006.
type UnpublishOptions struct {
	Options *OperationOptions `json:"options,omitempty"`
	Capture string            `json:"capture"`
}
