package selftest

import (
	"context"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/tests/core"
)

// RFC §9:529–570, §12:1042–1064. Each oracle executes the public synthetic
// binding and its targeted defect; this is never provider conformance evidence.
func TestFailureAndAuthorityOracles(t *testing.T) {
	catalog := append(append([]harness.ConformanceTest{}, core.FailureContractTests...), core.AuthorityContractTests...)
	cases := []struct{ id, mutation string }{
		{"CT-ERROR-001", "unknown-as-none"}, {"CT-ERROR-002", "omit-reasons"}, {"CT-ERROR-003", "empty-rejections"}, {"CT-ERROR-004", "hide-cleanup"}, {"CT-ERROR-005", "automatic-retry"}, {"CT-SECRET-001", "leak-original"},
		{"CT-AUTH-001", "ambient-authority"}, {"CT-AUTH-002", "ambient-authority"}, {"CT-AUTH-003", "ambient-authority"}, {"CT-AUTH-004", "ambient-authority"},
	}
	for _, test := range cases {
		t.Run(test.id, func(t *testing.T) {
			for _, mutant := range []bool{false, true} {
				mutation := ""
				want := harness.Pass
				if mutant {
					mutation = test.mutation
					want = harness.Fail
				}
				target := failureTarget{MakeFakeTarget(NoDefects), mutation}
				report := harness.RunConformance(context.Background(), target, catalog, harness.RunOptions{Only: []string{test.id}})
				found := false
				for _, r := range report.Results {
					if r.TestID == test.id {
						found = true
						if r.Status != want {
							t.Fatalf("mutant=%v: %s %s", mutant, r.Status, r.Reason)
						}
					}
				}
				if !found {
					t.Fatal("oracle was not executed")
				}
			}
		})
	}
}
