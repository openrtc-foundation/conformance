package core

import (
	"fmt"
	"github.com/openrtc/conformance/harness"
	"slices"
	"testing"
)

// RFC §14.3:1125–1149: stabilization scenarios must remain in the executable catalog.
func TestCoreTestsCount(t *testing.T) {
	required := []string{"CT-BOUND-001", "CT-BOUND-002", "CT-RACE-001", "CT-RACE-002", "CT-CLEAN-001", "CT-CLEAN-002", "CT-CLEAN-003", "CT-STALE-001", "CT-ADMIT-001", "CT-CANCEL-REASON-001"}
	registered := map[string]bool{}
	for _, test := range CoreTests {
		registered[test.ID] = true
	}
	for _, id := range required {
		if !registered[id] {
			t.Errorf("required scenario %s is unregistered", id)
		}
	}
}

func TestCoreTestsOrder(t *testing.T) {
	want := []string{
		"CT-REG-001", "CT-REG-002", "CT-REG-003", "CT-REG-004", "CT-REG-005", "CT-CFG-001",
		"CT-CFG-002", "CT-CFG-003", "CT-CFG-004", "CT-CFG-005", "CT-CFG-006", "CT-CFG-007",
		"CT-CFG-008", "CT-CFG-009", "CT-SES-001", "CT-SES-002", "CT-SES-003", "CT-SES-004",
		"CT-SES-005", "CT-SES-006", "CT-SES-007", "CT-SES-008", "CT-SES-009", "CT-SES-010",
		"CT-SES-011", "CT-EVT-001", "CT-EVT-002", "CT-EVT-003", "CT-EVT-004", "CT-EVT-005",
		"CT-EVT-006", "CT-EVT-007", "CT-EVT-008", "CT-CON-001", "CT-CON-002", "CT-CON-003",
		"CT-CON-004", "CT-ID-001", "CT-ID-002", "CT-CRD-001", "CT-CRD-002", "CT-CRD-003",
		"CT-CRD-004", "CT-CRD-005", "CT-CRD-006", "CT-CRD-007", "CT-CAN-001", "CT-CAN-002",
		"CT-CAN-003", "CT-CAN-004", "CT-CAN-005", "CT-CAN-006", "CT-CLN-001", "CT-CLN-002",
		"CT-CLN-003", "CT-CLN-004", "CT-PLC-001", "CT-PLC-002", "CT-PLC-003", "CT-PLC-004",
		"CT-PLC-005", "CT-CAP-001", "CT-CAP-002", "CT-CAP-003", "CT-CAP-004", "CT-CAP-005",
		"CT-CAP-006", "CT-ERR-001", "CT-ERR-002", "CT-EXT-001", "CT-EXT-002", "CT-EXT-003",
		"CT-MED-001", "CT-MED-002",
	}
	for i := 1; i <= 28; i++ {
		want = append(want, fmt.Sprintf("CT-LIFE-%03d", i))
	}
	for i := 1; i <= 20; i++ {
		want = append(want, fmt.Sprintf("CT-MOP-%03d", i))
	}
	for i := 1; i <= 6; i++ {
		want = append(want, fmt.Sprintf("CT-MADV-%03d", i))
	}
	for _, catalog := range [][]harness.ConformanceTest{EffectTests, MediaShapeTests, MediaPositiveTests, LifecycleBoundTests, SelectionCancellationTests, ObservationContractTests, PublicBindingTests, FailureContractTests, AuthorityContractTests, CollisionContractTests} {
		for _, test := range catalog {
			want = append(want, test.ID)
		}
	}
	got := make([]string, len(CoreTests))
	for i, test := range CoreTests {
		got[i] = test.ID
	}
	if !slices.Equal(got, want) {
		t.Fatalf("CoreTests IDs or order differ from the expected inventory: got %v, want %v", got, want)
	}
}
