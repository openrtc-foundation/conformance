package core

import (
	"context"
	"github.com/openrtc/conformance/harness"
)

var CollisionContractTests = []harness.ConformanceTest{
	boundTest("CT-COLLISION-001", "unknown collision policy neither authorizes replacement nor an automatic identity retry", []string{"RFC §8.2:461–463", "RFC §14.3:1134", "RFC §6:35"}, func(ctx context.Context, c *harness.TestContext) {
		target, ok := c.Target.(harness.CollisionTarget)
		if !ok {
			must(c.NotRun("controlled unknown-policy duplicate admission and independent submission witness unavailable"))
		}
		f := value(target.UnknownCollision(ctx))
		if f.Existing == nil || f.Candidate == nil || f.Attempt == nil || f.Policy == nil || f.Observe == nil || f.Boundary == "" {
			must(c.NotRun("unknown collision fixture lacks public mapping or independent boundary"))
		}
		c.Track("existing collision incarnation", func(ctx context.Context) error { _, err := f.Existing.Dispose(ctx).Result(ctx); return err })
		c.Track("candidate collision incarnation", func(ctx context.Context) error { _, err := f.Candidate.Dispose(ctx).Result(ctx); return err })
		check(c, f.Policy().Behavior == "unknown", "fixture did not establish unknown effective collision policy")
		existing := f.Existing.Observation().Snapshot()
		check(c, existing.Connection.State == "connected" && existing.Connection.Admission == "admitted", "existing participation was not independently established")
		check(c, f.Existing.ID() != f.Candidate.ID() && f.Existing.Generation() != f.Candidate.Generation(), "duplicate admission reused a public incarnation")
		_, err := f.Attempt(ctx).Result(ctx)
		if err != nil {
			value(c.ExpectError(err, harness.ExpectedFailure{}, "duplicate while policy unknown"))
		}
		// A documented unknown/pending candidate is permissible. It cannot establish
		// the other incarnation's departure or grant replacement authority.
		must(c.Settle(ctx))
		witness := value(f.Observe(ctx))
		check(c, witness.AdmissionSubmissions == 1, "implementation automatically retried same-identity admission without a known safe policy")
		check(c, !witness.ReplacementAuthorized, "unknown collision policy was treated as replacement authorization")
		after := f.Existing.Observation().Snapshot()
		if !witness.ExistingDepartureEstablished {
			check(c, after.Connection.State == "connected", "duplicate attempt invented existing incarnation departure")
		}
		check(c, f.Existing.Generation() == existing.Generation, "duplicate attempt changed existing incarnation identity")
	}),
}
