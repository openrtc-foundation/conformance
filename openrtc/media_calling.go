package openrtc

import "context"

// CaptureState describes capture lifetime, distinct from publication; MED-006, RFC 0001 §9.
type CaptureState string

// IndependentPublicationControl provides optional independent publication operations; EXT-001, MED-001–003.
type IndependentPublicationControl interface {
	Publish(context.Context, CaptureSourceID, PublishOptions) Async[[]TrackPublication]
	Unpublish(context.Context, PublicationID, UnpublishOptions) Async[Void]
}

// ManualSubscriptionControl provides optional client-side per-publication receive selection; MED-002.
type ManualSubscriptionControl interface {
	Subscribe(context.Context, PublicationID, []MediaKind) Async[TrackSubscription]
	Unsubscribe(context.Context, SubscriptionID) Async[Void]
}

// DeviceDescriptor is endpoint-scoped, with permission-qualified labels; RFC 0001 §3.2.
type DeviceDescriptor struct {
	ID        DeviceID              `json:"id"`
	Direction string                `json:"direction"`
	Kind      MediaKind             `json:"kind"`
	Label     Known[string]         `json:"label"`
	NativeID  Known[ScopedNativeID] `json:"nativeId"`
}

// DeviceSelection is explicit selection, not implicit capture; MED-001, RFC 0001 §3.2.
type DeviceSelection struct {
	Device DeviceID              `json:"device"`
	Target DeviceSelectionTarget `json:"target"`
}

// MediaDeviceManager provides optional endpoint device operations, not a server device scan; EXT-001, RFC 0001 §3.2.
type MediaDeviceManager interface {
	Enumerate(context.Context) Async[DeviceInventory]
	Snapshot() DeviceSnapshot
	Watch(func(DeviceSnapshot)) Unsubscribe
	Select(context.Context, DeviceSelection) Async[Void]
}

// StatisticsTarget qualifies a measurement target; RFC 0001 §3.2.
type StatisticsTarget struct {
	Kind   string                  `json:"kind"`
	ID     string                  `json:"id,omitempty"`
	Source *ReceiveSourceReference `json:"source,omitempty"`
}

// StatsSample records unit/source/time/coverage to prevent inferred guarantees; CAP-001, RFC 0001 §3.2.
type StatsSample struct {
	Target          StatisticsTarget        `json:"target"`
	Generation      GenerationID            `json:"generation"`
	Source          string                  `json:"source"`
	CollectedAt     Instant                 `json:"collectedAt"`
	Interval        Known[Duration]         `json:"interval"`
	SourceTimestamp Known[Instant]          `json:"sourceTimestamp"`
	Coverage        string                  `json:"coverage"`
	Measurements    []Measurement           `json:"measurements"`
	Native          Known[NativeStatistics] `json:"native"`
}

// MediaStatistics is an optional measurement facility; EXT-001, RFC 0001 §3.2.
type MediaStatistics interface {
	Sample(context.Context, StatisticsTarget) Async[StatsSample]
}

// ClientExtensionMap contains client-scoped facilities; acquisition/method authorization are independent; EXT-001–002.
type ClientExtensionMap struct {
	Calling CallingExtension   `json:"calling"`
	Devices MediaDeviceManager `json:"devices"`
}

// SessionExtensionMap contains session-scoped controls, never a giant mandatory base class; EXT-001–002.
type SessionExtensionMap struct {
	IndependentPublication IndependentPublicationControl  `json:"independent-publication"`
	ManualSubscription     ManualSubscriptionControl      `json:"manual-subscription"`
	Statistics             MediaStatistics                `json:"statistics"`
	Recording              RecordingExtension             `json:"recording"`
	Messaging              MessagingExtension             `json:"messaging"`
	ScreenSharing          ScreenSharingExtension         `json:"screen-sharing"`
	Encryption             EncryptionExtension            `json:"encryption"`
	SessionAdministration  SessionAdministrationExtension `json:"session-administration"`
}

// ServerExtensionMap keeps trusted controls under separate owners and credentials; ADP-001, EXT-002.
type ServerExtensionMap struct {
	SessionAdministration SessionAdministrationExtension `json:"session-administration"`
	Recording             RecordingExtension             `json:"recording"`
}

// CallingEvents includes incoming calls before accept/reject; RFC 0001 §3.2, STM-007.
type CallingEvents struct {
	IncomingCall RTCCallSession `json:"incomingCall"`
	Error        RTCError       `json:"error"`
}

// CallingExtension creates a distinct call session, with no conference requirement; EXT-001, STM-007.
type CallingExtension interface {
	RTCEventEmitter
	Dial(context.Context, DialOptions) RTCCallSession
	StartReceiving(context.Context, ProviderIdentity) Async[CallOutcome]
	StopReceiving(context.Context, ProviderIdentity) Async[CallOutcome]
	Registration() RegistrationStatus
	OnRegistrationChanged(func(RegistrationStatus)) Unsubscribe
}

// CallDestination is an explicit calling address, not a conference name; RFC 0001 §§4–5.
type CallDestination struct {
	Kind    string          `json:"kind"`
	URI     string          `json:"uri,omitempty"`
	Number  string          `json:"number,omitempty"`
	Address *ScopedNativeID `json:"address,omitempty"`
}

// DialOptions is participant-join lifetime call intent; no room or management secret; CFG-001, CFG-009.
type DialOptions struct {
	Account      ProviderIdentity    `json:"account"`
	Destination  SessionAddress      `json:"destination"`
	InitialMedia *InitialMediaIntent `json:"initialMedia,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitzero"`
	Placement    *PlacementPolicy    `json:"placement,omitempty"`
	Extensions   ProviderExtensions  `json:"extensions,omitzero"`
	Encryption   *EncryptionRequest  `json:"encryption,omitempty"`
	Recovery     *RecoveryOptions    `json:"recovery,omitempty"`
	Signal       context.Context     `json:"-"`
}

// AnswerOptions is incoming call admission, not conference join; RFC 0001 §§3.2,4.
type AnswerOptions struct {
	Leg          CallLegID           `json:"leg"`
	InitialMedia *InitialMediaIntent `json:"initialMedia,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitzero"`
	Placement    *PlacementPolicy    `json:"placement,omitempty"`
	Extensions   ProviderExtensions  `json:"extensions,omitzero"`
	Encryption   *EncryptionRequest  `json:"encryption,omitempty"`
	Recovery     *RecoveryOptions    `json:"recovery,omitempty"`
	Signal       context.Context     `json:"-"`
}

// RTCCallSession is a call profile session, without fabricated roster members; CORE-004, STM-007.
type RTCCallSession interface {
	RTCSessionBase
	Kind() string
	Legs() map[CallLegID]CallLegHandle
	Attempt() Known[CallAttempt]
}

// CallHoldState is orthogonal to admission held and connection recovering; STM-007.
type CallHoldState string

// CallState is the call leg lifecycle; detailed native transaction timing remains unverified; STM-007.
type CallState string

// CallLeg is one signaling leg; peer identity does not imply conference membership; CORE-004, STM-007.
type CallLegHandle interface {
	Snapshot() CallLeg
	Accept(AnswerOptions) CallAttempt
	ProvisionalResponse(context.Context, ProvisionalResponseRequest) Async[CallOutcome]
	Reject(context.Context, *RejectOptions) Async[CallOutcome]
	RespondToTransfer(context.Context, TransferResponse) Async[CallOutcome]
	Terminate(context.Context) Async[CallOutcome]
	Telephony() TelephonyControl
}

// CallRejectReason is rejection intent, not a universal SIP response-code mapping; STM-007.
type CallRejectReason string

// CallOutcome: accepted commands do not prove remote completion; STM-007.
type CallOutcome struct {
	Status         string                    `json:"status"`
	NativeReason   Known[string]             `json:"nativeReason"`
	ProtocolStatus Known[string]             `json:"protocolStatus"`
	Error          Known[SerializedRTCError] `json:"error"`
	TerminalCause  Known[string]             `json:"terminalCause"`
}

// TransferRequest separates blind and attended negotiated procedures; STM-007, CAP-001.
type TransferRequest struct {
	Leg          CallLegID       `json:"leg"`
	Kind         string          `json:"kind"`
	Destination  *SessionAddress `json:"destination,omitempty"`
	Consultation *CallReference  `json:"consultation,omitempty"`
}

// DtmfRequest delivery method and timing require independent support; EXT-001, RFC 0001 §12 OQ-009.
type DtmfRequest struct {
	Leg      CallLegID    `json:"leg"`
	Digits   string       `json:"digits"`
	Method   DtmfMethod   `json:"method"`
	Fallback []DtmfMethod `json:"fallback,omitempty"`
	Duration Duration     `json:"duration"`
	Gap      Duration     `json:"gap"`
}

// DtmfObservation is an observed telephone tone, not generic data messaging; RFC 0001 §12 OQ-009.
type DtmfObservation struct {
	Leg        CallLegID             `json:"leg"`
	Digit      string                `json:"digit"`
	ObservedAt Instant               `json:"observedAt"`
	NativeID   Known[ScopedNativeID] `json:"nativeId"`
}

// TelephonyControl actions are independently gated and authorized; EXT-001, STM-007.
type TelephonyControl interface {
	Hold(context.Context, ...OperationOptions) Async[CallOutcome]
	Resume(context.Context, ...OperationOptions) Async[CallOutcome]
	Transfer(context.Context, TransferRequest) Async[TransferStatus]
	SendDtmf(context.Context, DtmfRequest) Async[DtmfResult]
	ReceiveDtmf(func(DtmfEvent)) Unsubscribe
}

// RecordingRequest service variants require their own authority/placement; EXT-001–002.
type RecordingRequest struct {
	Target       SessionInstanceID   `json:"target"`
	Placement    *RecordingPlacement `json:"placement,omitempty"`
	Requirements []CapabilityRequest `json:"requirements,omitempty"`
	Extensions   ProviderExtensions  `json:"extensions,omitzero"`
}

// RecordingExtension provides optional authorized recording controls; EXT-001–002.
type RecordingExtension interface {
	Start(context.Context, RecordingRequest) Async[MediaJob]
	Get(context.Context, JobID) Async[MediaJob]
}

// JobStatus is separate from local session and stop acknowledgment; EXT-001, RFC 0001 §9.
type JobStatus string

// JobOutput availability is independently observed; EXT-001.
type JobOutput struct {
	Reference  string           `json:"reference"`
	Status     string           `json:"status"`
	MediaKinds []MediaKind      `json:"mediaKinds"`
	Storage    Known[RegionRef] `json:"storage"`
}

// JobOutcome: stopping a job does not imply completed outputs; EXT-001.
type JobOutcome struct {
	Status  string      `json:"status"`
	Outputs []JobOutput `json:"outputs,omitempty"`
	Error   RTCError    `json:"error,omitempty"`
	Reason  string      `json:"reason,omitempty"`
}

// MediaJob is an independent service job that can outlive local participation; EXT-001–002.
type MediaJob interface {
	Inspection() OwnerInspection
	Snapshot() MediaJobView
	Refresh(context.Context) Async[MediaJobView]
	Stop(context.Context) Async[JobStopReceipt]
	OnChanged(func(MediaJobView)) Unsubscribe
	Dispose(context.Context) Async[Void]
}
