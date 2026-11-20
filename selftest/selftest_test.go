package selftest

// HARNESS SELF-TEST. Not provider evidence.
import (
	"context"
	"errors"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/capability"
	"github.com/openrtc/conformance/tests/core"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var allSelfTests = append(append([]harness.ConformanceTest{}, core.CoreTests...), capability.CapabilityTests...)

func statusOf(r harness.ConformanceReport, id string) harness.ConformanceStatus {
	for _, v := range r.Results {
		if v.TestID == id {
			return v.Status
		}
	}
	return "missing"
}

var mutations = []struct {
	defect string
	ids    []string
}{
	{"SendingIgnoresRequirements", []string{"CT-MOP-020"}},
	{"CapturePublishes", []string{"CT-MOP-001"}},
	{"SendingTrueSuppressed", []string{"CT-MOP-002"}},
	{"SendingFalseMissesExternal", []string{"CT-MOP-003"}},
	{"SendingInverseClearsMute", []string{"CT-MOP-004"}},
	{"UnmuteClearsDisable", []string{"CT-MOP-005"}},
	{"EnableClearsMute", []string{"CT-MOP-006"}},
	{"ReceiveDropsVideo", []string{"CT-MOP-007"}},
	{"ReceiveUnsupportedEffects", []string{"CT-MOP-008"}},
	{"RenderClaimsPlaying", []string{"CT-MOP-009"}},
	{"DetachRejectsNativeFailure", []string{"CT-MOP-010"}},
	{"PublishCollapsesIdentities", []string{"CT-MOP-011"}},
	{"PartialPublishDropsIdentities", []string{"CT-MOP-012"}},
	{"UnpublishStopsCapture", []string{"CT-MOP-013"}},
	{"MediaInspectionWrongID", []string{"CT-MOP-014"}},
	{"InspectionEvictionError", []string{"CT-MOP-015"}},
	{"RetentionDropsIndependent", []string{"CT-MOP-016"}},
	{"StaleMuteNoop", []string{"CT-MOP-017"}},
	{"RenderResurrectsRemote", []string{"CT-MOP-018"}},
	{"UnsupportedMediaEffects", []string{"CT-MOP-019"}},
	{"BoundaryFallback", []string{"CT-MADV-001"}},
	{"LeakErrorCanary", []string{"CT-MADV-002"}},
	{"LeakSnapshotCanary", []string{"CT-MADV-003"}},
	{"LeakInspectionCanary", []string{"CT-MADV-004"}},
	{"LeakLogCanary", []string{"CT-MADV-005"}},
	{"LeakExtensionCanary", []string{"CT-MADV-006"}},
	{"HeldCompletesDefault", []string{"CT-LIFE-023"}},
	{"OpenFailureCreatesSession", []string{"CT-LIFE-022"}},
	{"JoinResultBeforeCommit", []string{"CT-LIFE-023"}},
	{"FenceLeavesSetupRunning", []string{"CT-LIFE-024"}},
	{"CleanupReplacesPrimary", []string{"CT-LIFE-025"}},
	{"EmitterAcceptsInvalid", []string{"CT-LIFE-026"}},
	{"ForeignInspectionAccepted", []string{"CT-LIFE-027"}},
	{"InspectionWatchSkipsCommit", []string{"CT-LIFE-028"}},
	{"RegistrationPoisoned", []string{"CT-LIFE-001"}},

	{"AdmissionChangesState", []string{"CT-LIFE-003", "CT-LIFE-004", "CT-LIFE-007"}},
	{"ExecutionDropsCleanup", []string{"CT-LIFE-003", "CT-LIFE-004", "CT-LIFE-010"}},
	{"CreateAfterDispose", []string{"CT-LIFE-005"}},
	{"OpenAfterDispose", []string{"CT-LIFE-006"}},
	{"JoinPreflightConsumes", []string{"CT-LIFE-008"}},
	{"PreabortStaysIdle", []string{"CT-LIFE-009"}},
	{"JoinFailureStaysJoining", []string{"CT-LIFE-010"}},
	{"CancelClaimsComplete", []string{"CT-LIFE-011"}},
	{"LateSuccessResurrects", []string{"CT-LIFE-011"}},
	{"IdleLeaveStaysIdle", []string{"CT-LIFE-012"}},
	{"FenceFailureCloses", []string{"CT-LIFE-013"}},
	{"CleanupRejectsClosure", []string{"CT-LIFE-014"}},
	{"DisposeKeepsListeners", []string{"CT-LIFE-015"}},
	{"SnapshotAliases", []string{"CT-LIFE-016"}},
	{"WatchAcceptsNil", []string{"CT-LIFE-017"}},
	{"WatchAfterDispose", []string{"CT-LIFE-017"}},
	{"WatchSkipsBootstrap", []string{"CT-LIFE-018"}},
	{"CleanupWatchCloses", []string{"CT-LIFE-014"}},
	{"StaleCallbacksCommit", []string{"CT-LIFE-019"}},
	{"ReuseNativeIdentity", []string{"CT-LIFE-020"}},
	{"OldRemovalDeletesNew", []string{"CT-LIFE-020"}},
	{"ReconcileIgnoresCurrent", []string{"CT-LIFE-021"}},
	{"IgnoreUnknownOptions", []string{"CT-CFG-001", "CT-CFG-004", "CT-PLC-005"}}, {"SilentDuplicateJoin", []string{"CT-SES-002"}}, {"ReplayOnSubscribe", []string{"CT-EVT-004"}}, {"ListenersBeforeCommit", []string{"CT-EVT-002"}}, {"ThrowingListenerBlocks", []string{"CT-EVT-003"}}, {"CancelAfterSettleSucceeds", []string{"CT-CAN-004"}}, {"LateSuccessResurrects", []string{"CT-CAN-006"}}, {"LeaveEndsForEveryone", []string{"CT-SES-011"}}, {"PlacementDowngrade", []string{"CT-PLC-001"}}, {"CredentialLeak", []string{"CT-CRD-004"}}, {"FakeExtensionForUnsupported", []string{"CT-EXT-001"}}, {"ReplaceOriginalError", []string{"CT-ERR-001"}}, {"HeldPresentedAsAdmitted", []string{"CT-SES-006"}}, {"AcceptUnknownRequired", []string{"CT-CAP-002"}}, {"OpenSessionRequestsCredentials", []string{"CT-SES-001"}}, {"SerializeLeaksSecret", []string{"CT-ERR-002"}}, {"RenewalFailureDisconnects", []string{"CT-CRD-006"}},
}

type noLifecycleTarget struct{ harness.ConformanceTarget }

type bareTarget struct{ *FakeTarget }

func (bareTarget) Controls() harness.FixtureControls { return harness.FixtureControls{} }

type realTarget struct{ *FakeTarget }

func (realTarget) Evidence() harness.EvidenceType { return harness.RealProvider }

type leakingTarget struct{ *FakeTarget }

func (t leakingTarget) CreateRegistry() openrtc.RTCAdapterRegistry {
	return leakingRegistry{t.FakeTarget.CreateRegistry()}
}

type leakingRegistry struct{ openrtc.RTCAdapterRegistry }

func (r leakingRegistry) Initialize(ctx context.Context, id openrtc.AdapterID, cfg openrtc.ProviderConfig) openrtc.InitializationOperation {
	return newInitialization(r, startOperation(r, func() (openrtc.RTCProvider, error) {
		p, err := r.RTCAdapterRegistry.Initialize(ctx, id, cfg).Result(ctx)
		if err != nil {
			return nil, err
		}
		return leakingProvider{p}, nil

	}))
}

type leakingProvider struct{ openrtc.RTCProvider }

func (receiver leakingProvider) Dispose(context.Context) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(receiver, func() (openrtc.CleanupReport, error) {
		return openrtc.CleanupReport{}, errors.New("leaked session")

	}))
}
func runSelf(target harness.ConformanceTarget, tests []harness.ConformanceTest, only ...string) harness.ConformanceReport {
	return harness.RunConformance(context.Background(), target, tests, harness.RunOptions{Only: only})
}

// RFC §14.2:1102–1115 and §14.3:1143–1149: fake results validate the harness only.
func TestHarnessSelfTest(t *testing.T) {
	t.Run("coherent fake", func(t *testing.T) {
		r := runSelf(MakeFakeTarget(NoDefects), allSelfTests)
		t.Log("\n" + harness.FormatReport(r))
		for _, v := range r.Results {
			if v.Status == harness.Fail {
				t.Errorf("%s: %s", v.TestID, v.Reason)
			}
			if v.Status != harness.Pass && v.Reason == "" {
				t.Errorf("%s: empty reason", v.TestID)
			}
		}
		if r.EvidenceType != harness.HarnessSelfTest || r.Claim.UnqualifiedCoreClaim {
			t.Error("invalid evidence or claim")
		}
		if r.Counts[harness.Fail] != 0 || r.Counts[harness.Incomplete] == 0 {
			t.Errorf("distribution: %v", r.Counts)
		}

	})
	t.Run("non-passes are not counted as passes", func(t *testing.T) {
		r := runSelf(MakeFakeTarget(NoDefects), allSelfTests)
		n := 0
		for _, s := range []harness.ConformanceStatus{harness.Pass, harness.Fail, harness.Incomplete, harness.NotApplicable} {
			n += r.Counts[s]
		}
		if n != len(r.Results) || r.Counts[harness.Incomplete] == 0 {
			t.Errorf("counts: %v", r.Counts)
		}
	})
	t.Run("missing controls", func(t *testing.T) {
		r := runSelf(bareTarget{MakeFakeTarget(NoDefects)}, core.CoreTests, "CT-CAN-006", "CT-SES-004")
		for _, id := range []string{"CT-CAN-006", "CT-SES-004"} {
			if statusOf(r, id) != harness.Incomplete {
				t.Errorf("%s: %s", id, statusOf(r, id))
			}
		}
	})
	t.Run("missing lifecycle controls", func(t *testing.T) {
		ids := []string{"CT-LIFE-001", "CT-LIFE-003", "CT-LIFE-004", "CT-LIFE-007", "CT-LIFE-010", "CT-LIFE-011", "CT-LIFE-013", "CT-LIFE-014", "CT-LIFE-019", "CT-LIFE-020", "CT-LIFE-021", "CT-LIFE-022", "CT-LIFE-023", "CT-LIFE-024", "CT-LIFE-025"}
		r := runSelf(noLifecycleTarget{MakeFakeTarget(NoDefects)}, core.CoreTests, ids...)
		for _, id := range ids {
			if statusOf(r, id) != harness.Incomplete {
				t.Errorf("%s: status %s", id, statusOf(r, id))
			}
		}
	})
	t.Run("missing media and adversarial controls", func(t *testing.T) {
		catalog := append(append([]harness.ConformanceTest{}, core.MediaContractTests...), core.MediaAdversarialTests...)
		r := runSelf(noLifecycleTarget{MakeFakeTarget(NoDefects)}, catalog)
		for _, test := range catalog {
			if statusOf(r, test.ID) != harness.Incomplete {
				t.Errorf("%s: %+v", test.ID, r.Results)
			}
		}
	})
	t.Run("undeclared profile", func(t *testing.T) {
		r := runSelf(MakeFakeTarget(NoDefects), capability.CapabilityTests, "CT-CAPX-CALL-001")
		if statusOf(r, "CT-CAPX-CALL-001") != harness.NotApplicable {
			t.Error(r.Results)
		}
	})
	t.Run("partial real-provider claim D-01", func(t *testing.T) {
		target := realTarget{MakeFakeTarget(NoDefects)}
		r := runSelf(target, allSelfTests, "CT-REG-001")
		if r.Complete || r.Claim.UnqualifiedCoreClaim || !strings.Contains(strings.Join(r.Claim.Reasons, " "), "omitted") {
			t.Error(r.Claim)
		}
		r = runSelf(target, allSelfTests)
		if r.Claim.UnqualifiedCoreClaim {
			t.Error(r.Claim)
		}
	})
	t.Run("cleanup failure D-02", func(t *testing.T) {
		r := runSelf(leakingTarget{MakeFakeTarget(NoDefects)}, allSelfTests, "CT-CAP-006")
		if statusOf(r, "CT-CAP-006") != harness.Fail || len(r.Results) == 0 || !strings.Contains(r.Results[0].Reason, "cleanup failed") {
			t.Error(r.Results)
		}
	})
	t.Run("unobserved side effects D-06 D-16", func(t *testing.T) {
		r := runSelf(bareTarget{MakeFakeTarget(NoDefects)}, allSelfTests, "CT-SES-001")
		if statusOf(r, "CT-SES-001") != harness.Incomplete || !strings.Contains(r.Results[0].Reason, "not observed") {
			t.Error(r.Results)
		}
	})
	t.Run("report redaction E-03", func(t *testing.T) {
		r := runSelf(MakeFakeTarget(NoDefects), allSelfTests, "CT-ERR-002")
		for _, v := range r.Results {
			if len(v.Reason) > 600 || strings.Contains(v.Reason, "conformance-secret-token") {
				t.Error(v.Reason)
			}
		}
	})
	t.Run("IDs and requirements", func(t *testing.T) {
		ids := map[string]bool{}
		re := regexp.MustCompile(`^(CORE|CFG|SES|MED|CAP|EXT|ADP|ERR|STM|IDL)-\d{3}$`)
		for _, v := range allSelfTests {
			if ids[v.ID] || len(v.Requirements) == 0 {
				t.Error(v.ID)
			}
			ids[v.ID] = true
			for _, r := range v.Requirements {
				if !re.MatchString(r) && !strings.HasPrefix(r, "RFC ") {
					t.Errorf("%s: %s", v.ID, r)
				}
			}
		}
	})
	t.Run("injected defects", func(t *testing.T) {
		for _, m := range mutations {
			t.Run(m.defect, func(t *testing.T) {
				d := FakeDefects{}
				reflect.ValueOf(&d).Elem().FieldByName(m.defect).SetBool(true)
				r := runSelf(MakeFakeTarget(d), allSelfTests, m.ids...)
				for _, id := range m.ids {
					if statusOf(r, id) != harness.Fail {
						t.Errorf("%s: status %s", id, statusOf(r, id))
					}
				}
			})
		}
	})
}
