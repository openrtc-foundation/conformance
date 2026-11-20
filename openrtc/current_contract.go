package openrtc

import "context"

// SpecRevision identifies the normative contract set.
const SpecRevision = "0.1.0"
const SPEC_REVISION = SpecRevision

type OperationID string
type ReceiveSourceID string
type ControlUnitID string

// Async preserves operation identity before waiting, including failed and void operations.
// The producer must allocate the ID before starting work and retain settlement once.
type Async[T any] interface {
	OperationID() OperationID
	Result(context.Context) (T, error)
}
type Void = struct{}

type MediaEffectField string
type MediaEffectValues struct {
	Capture               Known[string] `json:"capture"`
	Outbound              Known[string] `json:"outbound"`
	Publication           Known[string] `json:"publication"`
	Receive               Known[string] `json:"receive"`
	ReacquisitionRequired Known[string] `json:"reacquisitionRequired"`
	Recording             Known[string] `json:"recording"`
	Playback              Known[string] `json:"playback"`
}
type MediaEffectRequirements struct {
	Capture               *string `json:"capture,omitempty"`
	Outbound              *string `json:"outbound,omitempty"`
	Publication           *string `json:"publication,omitempty"`
	Receive               *string `json:"receive,omitempty"`
	ReacquisitionRequired *string `json:"reacquisitionRequired,omitempty"`
	Recording             *string `json:"recording,omitempty"`
	Playback              *string `json:"playback,omitempty"`
}
type ReceiveSelection string
type ReceivePolicyRequest struct {
	Audio           ReceiveSelection         `json:"audio"`
	Video           ReceiveSelection         `json:"video"`
	RequiredEffects *MediaEffectRequirements `json:"requiredEffects,omitempty"`
}
type ReceiveKindPolicy struct {
	Selection  string        `json:"selection"`
	Authority  Known[string] `json:"authority"`
	Adaptation Known[bool]   `json:"adaptation"`
	Controls   []string      `json:"controls"`
}
type EffectiveSuppression struct {
	Suppressed Known[bool]         `json:"suppressed"`
	Reasons    Known[[]string]     `json:"reasons"`
	Freshness  SourceFreshness     `json:"freshness"`
	Evidence   []EvidenceReference `json:"evidence"`
}
type SourceFreshness struct {
	Owner      LocalOwnerID        `json:"owner"`
	Source     string              `json:"source"`
	Coverage   []FreshnessCoverage `json:"coverage"`
	ObservedAt Known[Instant]      `json:"observedAt"`
	Status     string              `json:"status"`
	Reason     string              `json:"reason"`
}
type ReceiveSourceReference struct {
	Kind        string          `json:"kind"`
	Publication PublicationID   `json:"publication,omitempty"`
	ID          ReceiveSourceID `json:"id,omitempty"`
	Leg         CallLegID       `json:"leg,omitempty"`
	Direction   string          `json:"direction,omitempty"`
	Kinds       []MediaKind     `json:"kinds,omitempty"`
}
type ReceiveSourceView struct {
	ReceiveSourceReference
	Generation   GenerationID                     `json:"generation,omitempty"`
	NativeID     Known[ScopedNativeID]            `json:"nativeId,omitzero"`
	Contributors Known[[]ParticipantConnectionID] `json:"contributors,omitzero"`
	Coverage     string                           `json:"coverage,omitempty"`
}
type MediaResources struct {
	Sources      []CaptureSourceID `json:"sources"`
	Publications []PublicationID   `json:"publications"`
}
type RecoveryContinuity struct {
	PublicIdentity string `json:"publicIdentity"`
	NativeIdentity string `json:"nativeIdentity"`
}
type RecoveryOptions struct {
	AllowWrapperReadmission *bool `json:"allowWrapperReadmission,omitempty"`
}
type RecoveryPlacement struct {
	Plane        PlacementPlane `json:"plane"`
	Enforcement  Known[bool]    `json:"enforcement"`
	Interception Known[bool]    `json:"interception"`
}
type RecoveryPolicy struct {
	Mechanism   string                        `json:"mechanism"`
	Planes      []PlacementPlane              `json:"planes"`
	Continuity  map[string]RecoveryContinuity `json:"continuity"`
	Control     string                        `json:"control"`
	RetryBudget Known[uint64]                 `json:"retryBudget"`
	Deadline    Known[Duration]               `json:"deadline"`
	Placement   []RecoveryPlacement           `json:"placement"`
}
type PlacementSupport struct {
	Scope               CapabilityScope          `json:"scope"`
	Plane               PlacementPlane           `json:"plane"`
	Boundary            PlacementBoundary        `json:"boundary"`
	RegionSets          [][]RegionRef            `json:"regionSets"`
	Strengths           []string                 `json:"strengths"`
	BindingLifetime     ConfigurationLifetime    `json:"bindingLifetime"`
	Enforcement         Known[bool]              `json:"enforcement"`
	RecoveryEnforcement Known[bool]              `json:"recoveryEnforcement"`
	Prerequisites       []CapabilityPrerequisite `json:"prerequisites"`
	Evidence            []EvidenceReference      `json:"evidence"`
}

type DtmfMethod string
type RemoteUpdatePolicy string
type IncomingTransferPolicy string
type RenegotiationChanges struct {
	KindsAdded   []MediaKind `json:"kindsAdded"`
	KindsRemoved []MediaKind `json:"kindsRemoved"`
	HoldChanged  bool        `json:"holdChanged"`
}
type RenegotiationUpdate struct {
	ID       string                      `json:"id"`
	Changes  Known[RenegotiationChanges] `json:"changes"`
	Policy   RemoteUpdatePolicy          `json:"policy"`
	Decision string                      `json:"decision"`
}
type CallingPolicy struct {
	IncomingTransfer       IncomingTransferPolicy `json:"incomingTransfer"`
	RemoteUpdatePolicy     *RemoteUpdatePolicy    `json:"remoteUpdatePolicy,omitempty"`
	NativeCallRequirements []CapabilityRequest    `json:"nativeCallRequirements,omitempty"`
	NativeCallPlacement    *PlacementPolicy       `json:"nativeCallPlacement,omitempty"`
	NativeCallEncryption   *EncryptionRequest     `json:"nativeCallEncryption,omitempty"`
}
type CallingPolicyObservation struct {
	IncomingTransfer   IncomingTransferPolicy `json:"incomingTransfer"`
	RemoteUpdatePolicy RemoteUpdatePolicy     `json:"remoteUpdatePolicy"`
	Interception       Known[bool]            `json:"interception"`
	Evidence           []EvidenceReference    `json:"evidence"`
}
type CallReference struct {
	Session LocalSessionID `json:"session"`
	Leg     CallLegID      `json:"leg"`
}
type TransferStatus struct {
	ID               string        `json:"id"`
	Request          CallOutcome   `json:"request"`
	Remote           string        `json:"remote"`
	OriginalLegEnded Known[bool]   `json:"originalLegEnded"`
	NativeState      Known[string] `json:"nativeState"`
}
type TransferResponse struct {
	Leg       CallLegID `json:"leg"`
	RequestID string    `json:"requestId"`
	Decision  string    `json:"decision"`
}
type DtmfResult struct {
	Outcome           CallOutcome         `json:"outcome"`
	Methods           Known[[]DtmfMethod] `json:"methods"`
	SubmittedPrefix   Known[string]       `json:"submittedPrefix"`
	Completion        string              `json:"completion"`
	RemoteConsumption string              `json:"remoteConsumption"`
}

type InProcessHandle any
type PlaybackControlTarget struct {
	Kind string                  `json:"kind"`
	ID   ParticipantConnectionID `json:"id,omitempty"`
}
type RenderSource struct {
	Kind        string          `json:"kind"`
	Publication PublicationID   `json:"publication,omitempty"`
	ID          ReceiveSourceID `json:"id,omitempty"`
	Leg         CallLegID       `json:"leg,omitempty"`
	Direction   string          `json:"direction,omitempty"`
	Kinds       []MediaKind     `json:"kinds,omitempty"`
	Source      CaptureSourceID `json:"source,omitempty"`
}
type RenderRequest struct {
	Source               RenderSource `json:"source"`
	Target               RenderTarget `json:"target"`
	AcceptedControlScope string       `json:"acceptedControlScope,omitempty"`
}
type RenderBindingView struct {
	ID            RenderBindingID `json:"id"`
	Source        RenderSource    `json:"source"`
	Target        RenderTarget    `json:"target"`
	ControlScope  string          `json:"controlScope"`
	Playback      string          `json:"playback"`
	PlaybackMuted Known[bool]     `json:"playbackMuted"`
	ReceiveEffect string          `json:"receiveEffect"`
}

type CallLeg struct {
	Answered          Known[bool]                      `json:"answered"`
	ID                CallLegID                        `json:"id"`
	NativeID          Known[ScopedNativeID]            `json:"nativeId"`
	Direction         string                           `json:"direction"`
	Peer              Known[ProviderIdentity]          `json:"peer"`
	State             CallState                        `json:"state"`
	NativeState       Known[string]                    `json:"nativeState"`
	EarlyMedia        Known[bool]                      `json:"earlyMedia"`
	MediaReadiness    MediaReadiness                   `json:"mediaReadiness"`
	Outcome           CallOutcome                      `json:"outcome"`
	LocalHold         Known[bool]                      `json:"localHold"`
	RemoteHold        Known[bool]                      `json:"remoteHold"`
	Renegotiation     RenegotiationState               `json:"renegotiation"`
	Origin            Known[CallOrigin]                `json:"origin"`
	Conference        Known[CallConferenceAssociation] `json:"conference"`
	IncomingTransfers []IncomingTransferRequest        `json:"incomingTransfers"`
	Transfers         []TransferStatus                 `json:"transfers"`
}
type RenegotiationState struct {
	State            Known[string]         `json:"state"`
	CompletedUpdates []RenegotiationUpdate `json:"completedUpdates"`
}
type CallOrigin struct {
	Session   LocalSessionID `json:"session"`
	Leg       CallLegID      `json:"leg"`
	Cause     string         `json:"cause"`
	RequestID Known[string]  `json:"requestId"`
}
type IncomingTransferRequest struct {
	ID           string                 `json:"id"`
	Destination  Known[SessionAddress]  `json:"destination"`
	Consultation Known[CallReference]   `json:"consultation"`
	Policy       IncomingTransferPolicy `json:"policy"`
	Disposition  string                 `json:"disposition"`
	NativeState  Known[string]          `json:"nativeState"`
}
type CallConferenceAssociation struct {
	Instance       Known[SessionInstanceID]       `json:"instance"`
	Member         Known[ParticipantConnectionID] `json:"member"`
	NativeInstance Known[ScopedNativeID]          `json:"nativeInstance"`
	NativeMember   Known[ScopedNativeID]          `json:"nativeMember"`
	Evidence       []EvidenceReference            `json:"evidence"`
	HangupOwner    string                         `json:"hangupOwner"`
}
type ProvisionalResponseRequest struct {
	Leg          CallLegID           `json:"leg"`
	Response     string              `json:"response"`
	Media        string              `json:"media"`
	InitialMedia *InitialMediaIntent `json:"initialMedia,omitempty"`
	Encryption   *EncryptionRequest  `json:"encryption,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitempty"`
}
type RejectOptions struct {
	Reason string `json:"reason,omitempty"`
}
type DtmfEvent struct {
	Leg    CallLegID         `json:"leg"`
	Digit  string            `json:"digit"`
	Method Known[DtmfMethod] `json:"method"`
}
type CallLegChange struct {
	LegID    CallLegID `json:"legId"`
	Revision Revision  `json:"revision"`
	Facet    string    `json:"facet"`
	Previous *CallLeg  `json:"previous,omitempty"`
	Current  CallLeg   `json:"current"`
}
type CallSetupResult struct {
	LegID          CallLegID           `json:"legId"`
	Outcome        CallOutcome         `json:"outcome"`
	EarlyMedia     Known[bool]         `json:"earlyMedia"`
	MediaReadiness MediaReadiness      `json:"mediaReadiness"`
	InitialMedia   InitialMediaOutcome `json:"initialMedia"`
}
type CallAttempt interface {
	ID() AttemptID
	Generation() GenerationID
	Result(context.Context) Async[CallSetupResult]
	Cancel(context.Context) Async[CancellationReport]
}
type RegistrationStatus struct {
	Status                  string                    `json:"status"`
	Account                 Known[ProviderIdentity]   `json:"account"`
	Error                   Known[SerializedRTCError] `json:"error"`
	Validity                string                    `json:"validity"`
	Refresh                 string                    `json:"refresh"`
	ExpiresAt               Known[Instant]            `json:"expiresAt"`
	IncomingDeliveryEnabled Known[bool]               `json:"incomingDeliveryEnabled"`
	NativeState             Known[string]             `json:"nativeState"`
	Evidence                []EvidenceReference       `json:"evidence"`
}

type FreshnessCoverage struct {
	Kind       string `json:"kind"`
	ID         string `json:"id,omitempty"`
	EntityKind string `json:"entityKind,omitempty"`
	Facet      string `json:"facet,omitempty"`
}
type CaptureSourceView struct {
	Owner                ResourceRef           `json:"owner"`
	ID                   CaptureSourceID       `json:"id"`
	NativeID             Known[ScopedNativeID] `json:"nativeId"`
	Ownership            string                `json:"ownership"`
	Kinds                []MediaKind           `json:"kinds"`
	State                CaptureState          `json:"state"`
	Tracks               []TrackID             `json:"tracks"`
	SourceDisableIntent  Known[bool]           `json:"sourceDisableIntent"`
	EffectiveSuppression EffectiveSuppression  `json:"effectiveSuppression"`
}
type TrackView struct {
	ID        TrackID                `json:"id"`
	Kind      MediaKind              `json:"kind"`
	NativeID  Known[ScopedNativeID]  `json:"nativeId"`
	Direction string                 `json:"direction"`
	Source    Known[CaptureSourceID] `json:"source"`
}
type ActiveSpeakerObservation struct {
	Participants []Known[ParticipantConnectionID] `json:"participants"`
	Freshness    SourceFreshness                  `json:"freshness"`
	Coverage     string                           `json:"coverage"`
	Ranking      string                           `json:"ranking"`
}
type NetworkQualityObservation struct {
	Participant Known[ParticipantConnectionID] `json:"participant"`
	Direction   string                         `json:"direction"`
	Quality     Known[string]                  `json:"quality"`
	Freshness   SourceFreshness                `json:"freshness"`
}

type SourceSharePermissions struct {
	Use    bool `json:"use"`
	Mutate bool `json:"mutate"`
}

type AdapterCommand struct {
	AdapterID AdapterID `json:"adapterId"`
	CommandID string    `json:"commandId"`
	SchemaID  string    `json:"schemaId"`
	Revision  string    `json:"revision"`
	Payload   JSONValue `json:"payload"`
}
type InitialMediaOperation struct {
	ID       string              `json:"id"`
	Strength string              `json:"strength"`
	Request  InitialMediaRequest `json:"request"`
}
type InitialMediaRequest struct {
	Kind    string          `json:"kind"`
	Capture *CaptureRequest `json:"-"`
	Send    *SendRequest    `json:"-"`
}

type SourceReplacementRequest struct {
	Source          CaptureSourceID          `json:"source"`
	Replacement     CaptureSourceID          `json:"replacement"`
	RequiredEffects *MediaEffectRequirements `json:"requiredEffects,omitempty"`
	Preserve        []string                 `json:"preserve"`
}

type CredentialChannelDescriptor struct {
	Owner     LocalOwnerID        `json:"owner"`
	Channel   string              `json:"channel"`
	Audience  CredentialAudience  `json:"audience"`
	Scope     CredentialScope     `json:"scope"`
	Lifecycle CredentialLifecycle `json:"lifecycle"`
}
type ExtensionContract struct {
	Version SpecVersion       `json:"version"`
	Schemas []SchemaReference `json:"schemas"`
}
type CapabilityReference struct {
	Kind      string       `json:"kind"`
	ID        CapabilityID `json:"id,omitempty"`
	AdapterID AdapterID    `json:"adapterId,omitempty"`
	Name      string       `json:"name,omitempty"`
}
type ExtensionManifest struct {
	Key           ExtensionReference    `json:"key"`
	Owner         string                `json:"owner"`
	SessionKinds  []string              `json:"sessionKinds"`
	Contracts     []ExtensionContract   `json:"contracts"`
	Applicability []CapabilityReference `json:"applicability"`
}
type ExtensionReference struct {
	Kind      string                `json:"kind"`
	Name      string                `json:"name"`
	AdapterID AdapterID             `json:"adapterId,omitempty"`
	Versions  *VersionCompatibility `json:"versions,omitempty"`
}

type DeviceView = DeviceDescriptor
type DeviceInventory struct {
	Entries  []DeviceView `json:"entries"`
	Coverage string       `json:"coverage"`
}
type DeviceSelectionTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type DeviceSnapshot struct {
	Revision    Revision                    `json:"revision"`
	Inventory   DeviceInventory             `json:"inventory"`
	Selections  []DeviceSelection           `json:"selections"`
	Permissions map[MediaKind]Known[string] `json:"permissions"`
	Freshness   SourceFreshness             `json:"freshness"`
}
type NativeStatistics struct {
	AdapterID AdapterID `json:"adapterId"`
	SchemaID  string    `json:"schemaId"`
	Revision  string    `json:"revision"`
	Payload   JSONValue `json:"payload"`
}
type Measurement struct {
	Unit            string                  `json:"unit"` // RFC §11.10:999: each measurement identifies its units.
	Metric          string                  `json:"metric"`
	Value           Known[float64]          `json:"value"`
	Direction       string                  `json:"direction"`
	Target          StatisticsTarget        `json:"target"`
	Layer           Known[string]           `json:"layer"`
	Provenance      string                  `json:"provenance"`
	Boundary        string                  `json:"boundary"`
	Interval        Known[Duration]         `json:"interval"`
	IncludedStreams Known[[]ScopedNativeID] `json:"includedStreams"`
}

type CapabilityPrerequisite struct {
	Kind              string `json:"kind"`
	Target            string `json:"target"`
	Status            string `json:"status"`
	Reason            string `json:"reason"`
	ResolutionChannel string `json:"resolutionChannel"`
	EvaluateAt        string `json:"evaluateAt"`
}

type CapabilityAvailability struct {
	Status    string         `json:"status"`
	Reason    string         `json:"reason"`
	CheckedAt Known[Instant] `json:"checkedAt"`
	ExpiresAt Known[Instant] `json:"expiresAt"`
}
type CapabilityDetails struct {
	Kind           string        `json:"kind"`
	SourceCoverage Known[string] `json:"sourceCoverage,omitzero"`
	OutputForm     Known[string] `json:"outputForm,omitzero"`
}
type AdapterCapabilityReference struct {
	Kind      string    `json:"kind"`
	AdapterID AdapterID `json:"adapterId"`
	Name      string    `json:"name"`
}
type AdapterCapabilityDescriptor struct {
	ID             AdapterCapabilityReference `json:"id"`
	Revision       Revision                   `json:"revision"`
	Scope          CapabilityScope            `json:"scope"`
	Support        SupportDescription         `json:"support"`
	Prerequisites  []CapabilityPrerequisite   `json:"prerequisites"`
	Availability   CapabilityAvailability     `json:"availability"`
	Authorization  CapabilityAuthorization    `json:"authorization"`
	SchemaID       string                     `json:"schemaId"`
	SchemaRevision string                     `json:"schemaRevision"`
	Payload        JSONValue                  `json:"payload"`
	Evidence       []EvidenceReference        `json:"evidence"`
}

type CapabilityResource struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
}
type GuaranteeRequest = GuaranteeRequirement

type CredentialSnapshot struct {
	Revision Revision         `json:"revision"`
	Channels []CredentialView `json:"channels"`
}
type CredentialView struct {
	Owner           LocalOwnerID              `json:"owner"`
	Channel         string                    `json:"channel"`
	RequestRevision uint64                    `json:"requestRevision"`
	Metadata        Known[CredentialMetadata] `json:"metadata"`
	State           CredentialState           `json:"state"`
	Validity        string                    `json:"validity"`
	Application     string                    `json:"application"`
	Error           Known[SerializedRTCError] `json:"error"`
	// Used only by the fake's deliberate credential disclosure defect.
	LeakedForSelfTest string `json:"leakedForSelfTest,omitempty"`
}

type AdministrativeEffect struct {
	Kind        string                  `json:"kind"`
	Instance    SessionInstanceID       `json:"instance,omitempty"`
	Definition  SessionDefinitionID     `json:"definition,omitempty"`
	Participant ParticipantConnectionID `json:"participant,omitempty"`
	AdapterID   AdapterID               `json:"adapterId,omitempty"`
	SchemaID    string                  `json:"schemaId,omitempty"`
	Revision    string                  `json:"revision,omitempty"`
	Payload     JSONValue               `json:"payload,omitempty"`
}
type AdministrationRequest struct {
	Command     string                  `json:"command"`
	Instance    SessionInstanceID       `json:"instance,omitempty"`
	Definition  SessionDefinitionID     `json:"definition,omitempty"`
	Participant ParticipantConnectionID `json:"participant,omitempty"`
}
type AdministrationPlan struct {
	Request            AdministrationRequest  `json:"request"`
	RequiredEffects    []AdministrativeEffect `json:"requiredEffects"`
	CollateralEffects  []AdministrativeEffect `json:"collateralEffects"`
	DescriptorRevision Revision               `json:"descriptorRevision"`
	DiscoveryRevision  Revision               `json:"discoveryRevision"`
}

type RecordingPlacement struct {
	RecordingProcessingRegion *PlacementConstraint `json:"recordingProcessingRegion,omitempty"`
	RecordingStorageRegion    *PlacementConstraint `json:"recordingStorageRegion,omitempty"`
}
type RecordingOutput struct {
	NativeID        Known[ScopedNativeID] `json:"nativeId"`
	Availability    string                `json:"availability"`
	Location        Known[string]         `json:"location"`
	MediaType       Known[string]         `json:"mediaType"`
	RetentionUntil  Known[Instant]        `json:"retentionUntil"`
	Persistence     Known[string]         `json:"persistence"`
	AccessAuthority Known[string]         `json:"accessAuthority"`
}
type MediaJobView struct {
	ID             JobID                     `json:"id"`
	Revision       Revision                  `json:"revision"`
	NativeID       Known[ScopedNativeID]     `json:"nativeId"`
	Target         SessionInstanceID         `json:"target"`
	Status         JobStatus                 `json:"status"`
	Configuration  ConfigurationOutcome      `json:"configuration"`
	Outputs        []RecordingOutput         `json:"outputs"`
	OutputCoverage string                    `json:"outputCoverage"`
	Placement      []PlacementObservation    `json:"placement"`
	Error          Known[SerializedRTCError] `json:"error"`
	Lifetime       string                    `json:"lifetime"`
}
type JobStopReceipt struct {
	Status string       `json:"status"`
	Job    MediaJobView `json:"job"`
}

type SessionDefinitionDescriptor struct {
	NativeID   Known[ScopedNativeID] `json:"nativeId"`
	Address    SessionAddress        `json:"address"`
	Allocation string                `json:"allocation"`
	Policy     Known[SessionPolicy]  `json:"policy"`
}

type CredentialInput struct {
	Material OpaqueCredentialMaterial `json:"-"`
	Metadata CredentialInputMetadata  `json:"metadata"`
}
type CredentialInputMetadata struct {
	Audience  CredentialAudience      `json:"audience"`
	Scope     CredentialScope         `json:"scope"`
	Subject   Known[ProviderIdentity] `json:"subject"`
	IssuedAt  Known[Instant]          `json:"issuedAt"`
	ExpiresAt Known[Instant]          `json:"expiresAt"`
}
type ServiceJobCredentialTarget struct {
	Kind     string                `json:"kind"`
	Instance SessionInstanceID     `json:"instance,omitempty"`
	Job      JobID                 `json:"job,omitempty"`
	NativeID Known[ScopedNativeID] `json:"nativeId"`
	Address  SessionAddress        `json:"address"`
}

type TransferableSessionTarget struct {
	Mode            string                              `json:"mode"`
	Definition      *SessionDefinitionDescriptor        `json:"-"`
	KnownDefinition *Known[SessionDefinitionDescriptor] `json:"-"`
	Address         *SessionAddress                     `json:"address,omitempty"`
	CreationPolicy  *Known[SessionPolicy]               `json:"creationPolicy,omitempty"`
	// These permit construction of deliberately invalid nested responses by fixtures.
	Broker  SessionBroker         `json:"-"`
	Request *BrokerSessionRequest `json:"-"`
}

type EphemeralMessage struct {
	Payload     []byte            `json:"payload"`
	Recipients  MessageRecipients `json:"recipients"`
	Reliability string            `json:"reliability"`
}
type ReceivedEphemeralMessage struct {
	Payload     []byte                   `json:"payload"`
	Sender      Known[MessageSender]     `json:"sender"`
	Recipients  Known[MessageRecipients] `json:"recipients"`
	Reliability Known[string]            `json:"reliability"`
}
type MessageSender struct {
	Kind string                  `json:"kind"`
	ID   ParticipantConnectionID `json:"id,omitempty"`
	Leg  CallLegID               `json:"leg,omitempty"`
}
type EncryptionSourceScope struct {
	Kind string            `json:"kind"`
	IDs  []CaptureSourceID `json:"ids,omitempty"`
}
type EncryptionBoundary struct {
	Sources    EncryptionSourceScope `json:"sources"`
	Services   []string              `json:"services"`
	Exclusions []string              `json:"exclusions"`
}
type EncryptionKeyRequest struct {
	KeyID   string `json:"keyId"`
	Purpose string `json:"purpose"`
}
type OpaqueKeyMaterial interface{ Use(func([]byte) any) any }

type AdapterDescriptor = ProviderDescriptor
type DefaultProvisioning struct {
	Mode       string               `json:"mode"`
	Namespace  string               `json:"namespace"`
	Deployment *Known[DeploymentID] `json:"deployment,omitempty"`
}
type Registration struct {
	AdapterID       AdapterID   `json:"adapterId"`
	SelectedVersion SpecVersion `json:"selectedVersion"`
}

type SchemaDocument struct {
	AdapterID  AdapterID                  `json:"adapterId"`
	SchemaID   string                     `json:"schemaId"`
	Revision   string                     `json:"revision"`
	Lifetime   ConfigurationLifetime      `json:"lifetime"`
	JSONSchema JSONValue                  `json:"jsonSchema"`
	Bindings   []ConfigurationBinding     `json:"bindings"`
	Commands   []AdapterCommandDescriptor `json:"commands"`
}
type AdapterCommandDescriptor struct {
	CommandID    string                `json:"commandId"`
	Owner        string                `json:"owner"`
	Authority    AuthorityPlane        `json:"authority"`
	Lifetime     ConfigurationLifetime `json:"lifetime"`
	Activation   string                `json:"activation"`
	JSONSchema   JSONValue             `json:"jsonSchema"`
	Requirements []CapabilityRequest   `json:"requirements"`
}
type AdapterQualifiedExtension interface {
	Execute(context.Context, ValidatedExtensionCommand) Async[Void]
}

// DisplayedCallState preserves uncertainty on an answered Call (RFC §11.1:600–612,
// A.3:1342). Transfer wins display priority while independent holds remain.
func DisplayedCallState(answered, transferActive bool, local, remote Known[bool]) CallState {
	if !answered {
		return "unknown"
	}
	if transferActive {
		return "transferring"
	}
	isTrue := func(v Known[bool]) bool { return v.Status == "known" && v.Value != nil && *v.Value }
	isFalse := func(v Known[bool]) bool { return v.Status == "known" && v.Value != nil && !*v.Value }
	if isTrue(local) || isTrue(remote) {
		return "held"
	}
	if isFalse(local) && isFalse(remote) {
		return "active"
	}
	return "unknown"
}
