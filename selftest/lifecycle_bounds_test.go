package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/tests/core"
)

// Every oracle runs its real public operations against the conforming fake and a
// targeted defect. This validates harness behavior, not provider conformance.
// RFC §8.5:487–500, §8.6:506–510, §14.3:1149.
func checkLifecycleOracle(t *testing.T, id string, defect FakeDefects) {
	t.Helper()
	for _, candidate := range []struct {
		name   string
		defect FakeDefects
		want   harness.ConformanceStatus
	}{{"conforming", NoDefects, harness.Pass}, {"defective", defect, harness.Fail}} {
		t.Run(candidate.name, func(t *testing.T) {
			target := MakeFakeTarget(candidate.defect)
			report := harness.RunConformance(context.Background(), target, core.LifecycleBoundTests, harness.RunOptions{Only: []string{id}})
			if got := statusOf(report, id); got != candidate.want {
				for _, r := range report.Results {
					if r.TestID == id {
						t.Fatalf("%s status %s, want %s: %s", id, got, candidate.want, r.Reason)
					}
				}
			}
		})
	}
}
func TestNeverResolvingProviderSettlesOnCancellationAndTermination(t *testing.T) {
	checkLifecycleOracle(t, "CT-BOUND-001", FakeDefects{CancellationTooSlow: true})
	t.Run("create ignores cancellation", func(t *testing.T) {
		checkLifecycleOracle(t, "CT-BOUND-001", FakeDefects{CreateCancellationWaitsNative: true})
	})
}
func TestDeadlineExpiresWithoutProviderCompletion(t *testing.T) {
	for _, d := range []FakeDefects{{DeadlineEarly: true}, {DeadlineTooSlow: true}, {DeadlineRawWaiter: true}, {LateSuccessResurrects: true}} {
		checkLifecycleOracle(t, "CT-BOUND-002", d)
	}
}
func TestCancellationConnectionCommitRace(t *testing.T) {
	checkLifecycleOracle(t, "CT-RACE-001", FakeDefects{CancelAfterSettleSucceeds: true})
}
func TestLateJoinCompletionAfterCancellation(t *testing.T) {
	checkLifecycleOracle(t, "CT-RACE-002", FakeDefects{LateSuccessResurrects: true})
}
func TestHeldJoinCannotSucceedBeforeAdmission(t *testing.T) {
	checkLifecycleOracle(t, "CT-ADMIT-001", FakeDefects{HeldCompletesDefault: true})
}

func TestSelectionDismissalAndCancellationRemainDistinct(t *testing.T) {
	// RFC §8.5:489, §9:538–546. Both controlled schedules retain immutable reasons.
	for _, wrong := range []bool{false, true} {
		target := selectionTarget{MakeFakeTarget(NoDefects), wrong}
		r := harness.RunConformance(context.Background(), target, core.SelectionCancellationTests, harness.RunOptions{})
		want := harness.Pass
		if wrong {
			want = harness.Fail
		}
		for _, v := range r.Results {
			if v.TestID == "CT-CANCEL-REASON-001" && v.Status != want {
				t.Fatalf("selection oracle: %s %s", v.Status, v.Reason)
			}
		}
	}
}

func TestNeverResolvingSupportedAsyncOperation(t *testing.T) {
	checkLifecycleOracle(t, "CT-BOUND-001", FakeDefects{PendingAsyncWaitsProvider: true})
}
