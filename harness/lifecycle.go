package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
	"time"
)

// LifecycleFaultTarget is optional. Conditions must be established by the fixture,
// not guessed from provider error numbers. Admission creates no native resources;
// execution acquires an unreturned resource whose cleanup is initially pending.
// A fence fault prevents local closure; a cleanup fault permits local closure
// while one of two releases fails. CompleteCleanup evidences the later release.
type LifecycleFaultTarget interface{ LifecycleFaults() LifecycleFaultControl }
type LifecycleFaultControl interface {
	FailNext(operation, phase string, code openrtc.ErrorCode)
	CompleteCleanup(context.Context) error
	Resources() LifecycleResources
	// Quiesce waits for work already released by gates, never for a blocked gate.
	Quiesce(context.Context, openrtc.RTCConferenceSession) error
	// AwaitJoinMilestone waits until signaling and the current admission assessment
	// have committed and the selected milestone has either settled or is held.
	AwaitJoinMilestone(context.Context, openrtc.RTCConferenceSession) error
}
type LifecycleResources struct{ Acquired, Released int }

// LifecycleEventTarget scripts provider callbacks independently of public calls.
// Reconcile supplies authoritative current endpoint facts after reordered or
// duplicated callbacks. NativeKey distinguishes incarnations reusing NativeUID.
type LifecycleEventTarget interface{ LifecycleEvents() LifecycleEventControl }
type LifecycleEventControl interface {
	Submit(context.Context, openrtc.RTCConferenceSession, openrtc.GenerationID, []LifecycleNativeEvent) error
	Reconcile(context.Context, openrtc.RTCConferenceSession, []LifecycleNativeEvent) error
}
type LifecycleNativeEvent struct {
	Kind                 string // upsert, remove, or allocation (an acquired unreturned resource)
	NativeUID, NativeKey string
	DisplayName          string
	Timestamp            openrtc.Instant // source timestamp, never a global ordering guarantee
}

// BindingTiming is fixture metadata from binding documentation, not a runtime API.
// RFC §8.5:487–500, §8.6:506–510; suspension rules identify valid measurements.
type BindingTiming struct {
	Reference, Environment, Scope, SuspensionRules                         string
	Cancellation, DeadlineLag, TerminationFence, Release, CleanupRetention time.Duration
}
type BindingTimingTarget interface{ BindingTiming() BindingTiming }

// LifecycleBoundControl separates native acknowledgement from observed release.
// No internal generation representation or SDK call count is required.
type LifecycleBoundTarget interface{ LifecycleBounds() LifecycleBoundControl }
type LifecycleBoundControl interface {
	HoldCreateClient() JoinGate
	HoldCleanup(acknowledge bool)
	ConfirmRelease(context.Context) error
}

// SelectionCancellationTarget maps an actually supported screen selector and its
// scoped public capability into the test. Dismiss is test input; Start is public.
// RFC §11.4, §8.5:489, §9:538/546.
type SelectionCancellationTarget interface {
	SelectionCancellation() SelectionCancellationFixture
}
type SelectionCancellationFixture struct {
	Start      openrtc.ScreenSharingExtension
	Capability openrtc.CapabilityDescriptor
	Dismiss    func()
}

// PendingOperationTarget enumerates the supported asynchronous operation scopes
// declared by the target, then gates their provider work without native abort.
// The fixture must invoke the actual mapped operation, not a cancelled waiter.
type PendingOperationTarget interface {
	PendingOperationClaims() []OperationClaim
	HoldPendingOperation(context.Context, OperationClaim, openrtc.RTCConferenceSession) (PendingOperationFixture, error)
}
type PendingOperationFixture struct {
	PublicOperation string
	Capability      openrtc.CapabilityDescriptor
	Gate            JoinGate
	Invoke          func(context.Context) openrtc.Async[openrtc.Void]
}
