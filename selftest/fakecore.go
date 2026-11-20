// Package selftest contains the in-memory harness self-test target.
// It is not a provider implementation or evidence of provider conformance (ADP-006).
package selftest

import (
	"reflect"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// FakeDefects lists the deliberate defects available to the self-test fake.
// Observation and serialization defects are implemented; later stages use the remaining flags.
type FakeDefects struct {
	PendingAsyncWaitsProvider     bool
	CancellationTooSlow           bool
	DeadlineEarly                 bool
	DeadlineTooSlow               bool
	DeadlineRawWaiter             bool
	CleanupAcknowledgedAsReleased bool
	CleanupRetentionLost          bool
	DuplicateAllocation           bool
	RepeatedReleaseResurrects     bool
	CreateCancellationWaitsNative bool
	SendingIgnoresRequirements    bool

	BoundaryFallback     bool
	LeakErrorCanary      bool
	LeakSnapshotCanary   bool
	LeakInspectionCanary bool
	LeakLogCanary        bool
	LeakExtensionCanary  bool

	CapturePublishes              bool
	SendingTrueSuppressed         bool
	SendingFalseMissesExternal    bool
	SendingInverseClearsMute      bool
	UnmuteClearsDisable           bool
	EnableClearsMute              bool
	ReceiveDropsVideo             bool
	ReceiveUnsupportedEffects     bool
	RenderClaimsPlaying           bool
	DetachRejectsNativeFailure    bool
	PublishCollapsesIdentities    bool
	PartialPublishDropsIdentities bool
	UnpublishStopsCapture         bool
	MediaInspectionWrongID        bool
	InspectionEvictionError       bool
	RetentionDropsIndependent     bool
	StaleMuteNoop                 bool
	RenderResurrectsRemote        bool
	UnsupportedMediaEffects       bool

	HeldCompletesDefault           bool
	InspectionWatchSkipsCommit     bool
	OpenFailureCreatesSession      bool
	JoinResultBeforeCommit         bool
	FenceLeavesSetupRunning        bool
	CleanupReplacesPrimary         bool
	EmitterAcceptsInvalid          bool
	ForeignInspectionAccepted      bool
	IgnoreUnknownOptions           bool
	RegistrationPoisoned           bool
	UnknownInitCode                bool
	AdmissionChangesState          bool
	ExecutionDropsCleanup          bool
	CreateAfterDispose             bool
	OpenAfterDispose               bool
	JoinPreflightConsumes          bool
	PreabortStaysIdle              bool
	JoinFailureStaysJoining        bool
	CancelClaimsComplete           bool
	IdleLeaveStaysIdle             bool
	DisposeKeepsListeners          bool
	CleanupRejectsClosure          bool
	FenceFailureCloses             bool
	SnapshotAliases                bool
	WatchAcceptsNil                bool
	WatchAfterDispose              bool
	CleanupWatchCloses             bool
	WatchSkipsBootstrap            bool
	StaleCallbacksCommit           bool
	ReuseNativeIdentity            bool
	OldRemovalDeletesNew           bool
	ReconcileIgnoresCurrent        bool
	SilentDuplicateJoin            bool
	ReplayOnSubscribe              bool
	ListenersBeforeCommit          bool
	ThrowingListenerBlocks         bool
	CancelAfterSettleSucceeds      bool
	LateSuccessResurrects          bool
	LeaveEndsForEveryone           bool
	PlacementDowngrade             bool
	CredentialLeak                 bool
	FakeExtensionForUnsupported    bool
	ReplaceOriginalError           bool
	HeldPresentedAsAdmitted        bool
	AcceptUnknownRequired          bool
	OpenSessionRequestsCredentials bool
	DuplicateCleanup               bool
	SerializeLeaksSecret           bool
	RenewalFailureDisconnects      bool
}

// FakeTarget provides the binding-based self-test fixture.
type FakeTarget struct {
	defects FakeDefects
	world   *world
}

var _ harness.ConformanceTarget = (*FakeTarget)(nil)
var _ harness.AdministrationTarget = (*FakeTarget)(nil)
var _ harness.FixtureTarget = (*FakeTarget)(nil)

func MakeFakeTarget(defects FakeDefects) *FakeTarget {
	return &FakeTarget{defects: defects, world: newWorld()}
}

// MakeFakeTargetWithClock injects logical observation time. The clock must be
// safe for concurrent use; the default target uses a fixed epoch.
func MakeFakeTargetWithClock(defects FakeDefects, clock func() openrtc.Instant) *FakeTarget {
	t := MakeFakeTarget(defects)
	if clock != nil {
		t.world.clock = clock
	}
	return t
}
func (*FakeTarget) Name() string                   { return "harness-fake (self-test)" }
func (*FakeTarget) Author() string                 { return "OpenRTC conformance harness self-test" }
func (*FakeTarget) Evidence() harness.EvidenceType { return harness.HarnessSelfTest }
func (t *FakeTarget) Environment() harness.EnvironmentDescription {
	names := []string{}
	v := reflect.ValueOf(t.defects)
	typ := v.Type()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).Bool() {
			name := typ.Field(i).Name
			names = append(names, strings.ToLower(name[:1])+name[1:])
		}
	}
	value := strings.Join(names, ",")
	if value == "" {
		value = "none"
	}
	return harness.EnvironmentDescription{AdapterArtifact: "harness-fake", AdapterVersion: "0.0.0-selftest", SDKArtifact: "none", SDKVersion: "n/a", ServerArtifact: "none", Platform: "node (in-memory)", Deployment: "none", AccountPrerequisites: []string{}, Options: []harness.EnvironmentOption{{Name: "defects", Value: value}}}
}
func (*FakeTarget) Timing() harness.TimingPolicy {
	return harness.TimingPolicy{OperationMs: 1500, SettleMs: 15}
}
func (t *FakeTarget) CreateRegistry() openrtc.RTCAdapterRegistry {
	return makeRegistry(t.world, t.defects)
}
func (*FakeTarget) CreateAdapter() openrtc.RTCAdapter      { return bindingAdapter{} }
func (*FakeTarget) ProviderConfig() openrtc.ProviderConfig { return openrtc.ProviderConfig{} }
