package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// CurrentMediaProfileTarget supplies only mappings to real public operations and
// independent native/service stimulus controls. Controls must not implement the
// postconditions being measured. Missing mappings are incomplete, not unsupported.
// RFC §14.2:1102–1115, Appendix A.0:1176.
type CurrentMediaProfileTarget interface {
	CurrentMediaProfiles() CurrentMediaProfileFixture
}
type CurrentMediaProfileFixture struct {
	CallMedia    *CallMediaProfileFixture
	Calling      *CallingProfileFixture
	Subscription *SubscriptionProfileFixture
	Grants       *GrantProfileFixture
	Screen       *ScreenProfileFixture
	Recording    *RecordingProfileFixture
	Encryption   *EncryptionProfileFixture
}
type CallingProfileFixture struct {
	Account                  openrtc.ProviderIdentity
	RefreshRegistration      func(context.Context) error
	ExpireRegistration       func(context.Context) error
	ObserveRegistration      func(func(openrtc.RegistrationStatus)) openrtc.Unsubscribe
	QueueIncoming            func(context.Context) error
	DeliverQueuedIncoming    func(context.Context) error
	IncomingOffers           func() int
	DisposeReceiver          func(context.Context) error
	Effects                  func() MediaFixtureEffects
	Dial                     func(context.Context) openrtc.Async[openrtc.CurrentCall]
	Incoming                 func(context.Context) (openrtc.CurrentCall, error)
	RemoteHold               func(context.Context, openrtc.CurrentCall, openrtc.Known[bool]) error
	FinishTransfer           func(context.Context, openrtc.CurrentCall) error
	TransferClaimed          bool
	RemoteOffer              func(context.Context, openrtc.CurrentCall, openrtc.RemoteUpdatePolicy, bool) error
	WithholdTransferDecision func(context.Context, openrtc.CurrentCall) error
	StartReceiving           func(context.Context) openrtc.Async[openrtc.Void]
	StopReceiving            func(context.Context) openrtc.Async[openrtc.Void]
	Registration             func() openrtc.RegistrationStatus
	FailDigitsAfter          func(context.Context, int) error
	SubmittedDigits          func() string
}
type SubscriptionProfileFixture struct {
	HoldNextSubscribe func() JoinGate
	Control           openrtc.ManualSubscriptionControl
	Publication       openrtc.TrackPublication
	Snapshot          func(openrtc.SubscriptionID) openrtc.TrackSubscription
	RemotePublication func() openrtc.TrackPublication
	Teardown          func(context.Context) error
}
type GrantProfileFixture struct {
	Replacement       *SourceReplacementFixture
	Control           openrtc.SourceGrantOperations
	Source            openrtc.CaptureSource
	B, C              openrtc.ResourceRef
	Scope             openrtc.CapabilityScope
	Use               func(context.Context, openrtc.ResourceRef, openrtc.CapabilityScope) openrtc.Async[openrtc.Void]
	TeardownRecipient func(context.Context, openrtc.ResourceRef) error
}
type ScreenProfileFixture struct {
	ChangeSelection         func(context.Context, openrtc.CaptureSourceID) error
	ChangedSelection        openrtc.Known[string]
	ChangedAudioAssociation openrtc.Known[openrtc.CaptureSourceID]
	Capture                 func(context.Context, openrtc.OperationOptions) openrtc.Async[openrtc.SelectedScreen]
	DismissNext             func(context.Context) error
	EndSelection            func(context.Context, openrtc.CaptureSourceID) error
	Snapshot                func(openrtc.CaptureSourceID) openrtc.SelectedScreen
	Observe                 func(func(openrtc.SelectedScreen)) openrtc.Unsubscribe
	Effects                 func() MediaFixtureEffects
	UnsupportedCondition    openrtc.Condition
}
type RecordingProfileFixture struct {
	StartIndependent                               func(context.Context) openrtc.Async[openrtc.MediaJob]
	IndependentOwner                               func(openrtc.JobID) openrtc.ResourceRef
	ServiceContentBoundary, ServiceContentEvidence string
	ServiceContent                                 func() openrtc.Known[int]
	Start                                          func(context.Context) openrtc.Async[openrtc.MediaJob]
	SetConsent                                     func(context.Context, string) error
	ServiceJobs                                    func() openrtc.Known[int]
	ObserveRelease                                 func(context.Context, openrtc.JobID) error
	LeaveOwner                                     func(context.Context) error
	Independent                                    bool
}
type EncryptionProfileFixture struct {
	Control          openrtc.EncryptionExtension
	Request          openrtc.EncryptionRequest
	WithdrawEvidence func(context.Context) error
	ProtectedEffects func() int
	SendProtected    func(context.Context) openrtc.Async[openrtc.Void]
	DefaultSurfaces  func() []any
	KeyCanary        string
}

// Call-only media must not fabricate/provision a conference Session.
// RFC §11.1:592, A.1:1241–1242, A.3:1339.
type CallMediaProfileFixture struct {
	Call              openrtc.CurrentCall
	CaptureCall       func(context.Context) openrtc.Async[openrtc.CaptureSource]
	GrantedSource     openrtc.CaptureSource
	PublishCall       func(context.Context, openrtc.CaptureSource) openrtc.Async[openrtc.TrackPublication]
	Publication       func(openrtc.PublicationID) openrtc.TrackPublication
	ConferenceEffects func() int
}

// SourceReplacementFixture supplies public replacement operations plus separate
// continuity/failure stimuli. The setup identifies already eligible, granted
// sources; setter operations below establish the suppression preconditions.
// RFC §11.3:681–683, §9:566–570.
type SourceReplacementFixture struct {
	Publication            openrtc.PublicationID
	OldSource, Replacement openrtc.CaptureSource
	SetMuted               func(context.Context, bool) openrtc.Async[openrtc.Void]
	SetDisabled            func(context.Context, bool) openrtc.Async[openrtc.Void]
	Replace                func(context.Context, openrtc.PublicationID, openrtc.CaptureSourceID, bool, openrtc.OperationOptions) openrtc.Async[ReplacementObservation]
	Snapshot               func() openrtc.TrackPublication
	SetContinuity          func(context.Context, string) error
	PartialNext            func(context.Context) error
	Effects                func() MediaFixtureEffects
}
type ReplacementObservation struct {
	Publication        openrtc.TrackPublication
	Identity           string // preserved, changed, unknown
	Atomic             bool
	OldRemovalObserved bool
}
