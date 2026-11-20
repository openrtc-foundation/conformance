package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/tests/core"
	"testing"
)

// RFC §8.2:461–463, §14.3:1134. Unknown remains permissible; invented authority,
// automatic retry and invented departure are independently rejected.
func TestUnknownCollisionPolicyPreservesIncarnations(t *testing.T) {
	for _, mutation := range []string{"", "automatic-retry", "invent-departure", "assume-replacement"} {
		t.Run(mutation, func(t *testing.T) {
			target := collisionTarget{MakeFakeTarget(NoDefects), mutation}
			report := harness.RunConformance(context.Background(), target, core.CollisionContractTests, harness.RunOptions{})
			want := harness.Pass
			if mutation != "" {
				want = harness.Fail
			}
			for _, r := range report.Results {
				if r.TestID == "CT-COLLISION-001" && r.Status != want {
					t.Fatalf("%s: %s", r.Status, r.Reason)
				}
			}
		})
	}
}
