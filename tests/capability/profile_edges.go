package capability

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"slices"
)

var ProfileEdgeTests = []harness.ConformanceTest{
	profileCase("CT-SUB-004", "duplicate pending selection conflicts before the first selection can complete", "media.subscribe.manual", "RFC §11.2:656–667", func(ctx context.Context, c *harness.TestContext) {
		f := subscriptionFixture(c)
		if f.HoldNextSubscribe == nil {
			must(c.NotRun("no controlled pending subscription mapping"))
		}
		gate := f.HoldNextSubscribe()
		if gate == nil {
			must(c.NotRun("no supported pending subscription gate"))
		}
		defer gate.ReleaseSuccess()
		if f.Teardown != nil {
			c.Track("pending subscription scope", f.Teardown)
		}
		p := f.Publication
		check(c, p.ID != "" && len(p.Kinds) > 0, "remote publication fixture lacks exact kinds")
		first := f.Control.Subscribe(ctx, p.ID, p.Kinds)
		must(c.Within(ctx, "pending selection checkpoint", 0, func(ctx context.Context) error {
			select {
			case <-gate.Reached():
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}))
		_, err := f.Control.Subscribe(ctx, p.ID, p.Kinds).Result(ctx)
		portableFailure(c, err, openrtc.InvalidState)
		gate.ReleaseSuccess()
		selected := value(first.Result(ctx))
		check(c, selected.ID != "" && selected.State == "subscribed" && selected.Source.Publication == p.ID && slices.Equal(selected.Kinds, p.Kinds), "pending duplicate displaced the original exact selection")
		value(f.Control.Unsubscribe(ctx, selected.ID).Result(ctx))
	}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-REC-005", Title: "independent trusted-server recording survives participant leave and remains independently stoppable", Suite: harness.Capability, Profile: "recording", Requirements: []string{"RFC §7.0:57", "RFC §11.6:802–824", "RFC §8.6:508–510"}, Capability: &harness.CapabilityDependence{ID: "recording.independent", Operation: "startIndependentRecording"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		f := recordingFixture(c)
		if f.StartIndependent == nil || f.IndependentOwner == nil || f.LeaveOwner == nil || f.ObserveRelease == nil {
			must(c.NotRun("separately advertised trusted-server recording mapping/authority/observations unavailable"))
		}
		job := value(f.StartIndependent(ctx).Result(ctx))
		check(c, job != nil, "independent recording returned no job")
		c.Track("independent recording", func(ctx context.Context) error { _, err := job.Dispose(ctx).Result(ctx); return err })
		initial := job.Snapshot()
		check(c, initial.ID != "" && initial.Status == "running" && initial.Lifetime == "independent", "independent start did not establish scoped independent recording")
		owner := f.IndependentOwner(initial.ID)
		check(c, owner.Kind == "server" && owner.ID != "", "independent job lacks identified trusted-server owner")
		must(f.LeaveOwner(ctx))
		live := job.Snapshot()
		check(c, live.ID == initial.ID && live.Status == "running" && f.IndependentOwner(initial.ID) == owner, "participant leave ended or reassigned independently owned recording")
		receipt := value(job.Stop(ctx).Result(ctx))
		if receipt.Status != "confirmed" {
			check(c, job.Snapshot().Status != "stopped", "independent stop acknowledgement falsely established remote termination")
		}
		must(f.ObserveRelease(ctx, initial.ID))
		check(c, job.Snapshot().Status == "stopped", "independent authorized release evidence lost after participant leave")
		value(job.Stop(ctx).Result(ctx))
	})}),
}

func init() { CapabilityTests = append(CapabilityTests, ProfileEdgeTests...) }
