package selftest

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/tests/capability"
)

// This exercises catalog selection and fixture verdicts; it does not
// substitute for provider behavioral tests.
func TestCapabilityCatalog(t *testing.T) {
	// RFC §14.3:1125–1149: current scenario groups must remain registered;
	// a fixed historical count would reject valid stabilization additions.
	required := map[string]bool{}
	for _, test := range capability.CurrentProfileTests {
		required[test.ID] = false
	}
	for _, test := range capability.CapabilityTests {
		if _, ok := required[test.ID]; ok {
			required[test.ID] = true
		}
	}
	for id, registered := range required {
		if !registered {
			t.Errorf("missing current profile assertion %s", id)
		}
	}
	ids := make(map[string]bool)
	requirement := regexp.MustCompile(`^(?:(CORE|CFG|SES|MED|CAP|EXT|ADP|ERR|STM)-\d{3}|RFC (?:§|A\.)[^ ]+)`)
	for _, test := range capability.CapabilityTests {
		if ids[test.ID] {
			t.Errorf("duplicate ID %s", test.ID)
		}
		ids[test.ID] = true
		if len(test.Requirements) == 0 {
			t.Errorf("%s has no requirements", test.ID)
		}
		for _, id := range test.Requirements {
			if !requirement.MatchString(id) {
				t.Errorf("%s: invalid requirement %s", test.ID, id)
			}
		}
	}
	report := harness.RunConformance(context.Background(), MakeFakeTarget(FakeDefects{}), capability.CapabilityTests, harness.RunOptions{})
	if report.Counts[harness.Fail] != 0 {
		t.Fatalf("coherent optional-scope fake has unexpected failures: %v", report.Counts)
	}
	if report.Claim.Status == harness.Pass {
		t.Fatal("unrun profile/service obligations must remain incomplete")
	}
	if report.Claim.UnqualifiedCoreClaim {
		t.Error("self-test evidence cannot support a claim")
	}
	for _, result := range report.Results {
		if result.Status != harness.Pass && strings.TrimSpace(result.Reason) == "" {
			t.Errorf("%s has no reason", result.TestID)
		}
	}
}
