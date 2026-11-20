package core

import (
	"context"
	"encoding/json"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// Source-covering snapshots include unpublished sources, with explicit coverage;
// wire projection preserves unknown versus complete-empty (RFC §7.4:282,
// §8.3:469–477, A.1:1248–1255/1290).
var MediaShapeTests = []harness.ConformanceTest{
	mediaCase("CT-SHAPE-002", "unpublished sources remain observable with independent states", "RFC §7.4:282; §8.3:469–477", []string{"RFC §7.4:282", "RFC §8.3:469–477", "RFC A.1:1248–1255"}, func(ctx context.Context, c *harness.TestContext) {
		s, _ := mediaSession(ctx, c)
		a := mediaCapture(ctx, c, s, "session")
		b := mediaCapture(ctx, c, s, "session")
		value(s.Media().SetDisabled(ctx, a.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
		snap := s.Observation().Snapshot()
		check(c, snap.SourceCoverage != nil && snap.SourceCoverage.Completeness == "complete", "source inventory coverage omitted or unestablished")
		check(c, len(snap.Publications) == 0, "source setup published implicitly")
		check(c, snap.Sources[a.Snapshot().ID].State == "disabled" && snap.Sources[b.Snapshot().ID].State == "active", "unpublished source population/state missing")
		stream, ok := s.Observation().(openrtc.StreamObservation)
		if !ok {
			must(c.NotRun("missing ordered source-covering observe mapping"))
		}
		log := &observationLog{}
		stop := stream.Observe(log.add)
		defer stop()
		updates := log.read()
		check(c, len(updates) > 0 && updates[0].Snapshot != nil, "source observer lacks initial snapshot")
		initial := updates[0].Snapshot
		check(c, initial.Sources[a.Snapshot().ID].State == "disabled" && initial.Sources[b.Snapshot().ID].State == "active", "source observer omitted unpublished initial state")
		target, ok := c.Target.(harness.ObservationTarget)
		if !ok {
			must(c.NotRun("missing controlled source observation loss"))
		}
		control := target.ObservationControl()
		must(control.LoseNext(ctx, s))
		value(s.Media().SetDisabled(ctx, b.Snapshot().ID, true, openrtc.OperationOptions{}).Result(ctx))
		value(s.Media().SetDisabled(ctx, b.Snapshot().ID, false, openrtc.OperationOptions{}).Result(ctx))
		lost := false
		for _, u := range log.read() {
			lost = lost || u.Loss
		}
		check(c, lost, "source delivery loss unobservable")
		replacement := &observationLog{}
		stopReplacement := stream.Observe(replacement.add)
		defer stopReplacement()
		fresh := replacement.read()
		check(c, len(fresh) > 0 && fresh[0].Snapshot != nil, "replacement observer lacks snapshot")
		check(c, fresh[0].Snapshot.Sources[a.Snapshot().ID].State == "disabled" && fresh[0].Snapshot.Sources[b.Snapshot().ID].State == "active", "replacement snapshot lost source population/state")
		value(s.Media().SetDisabled(ctx, a.Snapshot().ID, false, openrtc.OperationOptions{}).Result(ctx))
		seen := false
		for _, u := range replacement.read() {
			if u.Snapshot != nil && u.Snapshot.Sources[a.Snapshot().ID].State == "active" {
				seen = true
			}
		}
		check(c, seen, "replacement source handoff missed subsequent established change")
	}),
	mediaCase("CT-SHAPE-003", "receive inventories preserve explicit completeness through wire projection", "RFC §4:23; A.1:1252/1290", []string{"RFC §4:23", "RFC A.1:1252–1255", "RFC A.1:1290"}, func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		snap := s.Observation().Snapshot()
		check(c, snap.ReceiveSourceCoverage != nil, "receive inventory has no coverage metadata")
		check(c, snap.ReceiveSourceCoverage.Completeness == "complete" || snap.ReceiveSourceCoverage.Completeness == "incomplete" || snap.ReceiveSourceCoverage.Completeness == "unknown", "invalid receive inventory completeness")
		raw := value(json.Marshal(struct {
			ReceiveSourceCoverage *openrtc.PopulationCoverage                           `json:"receiveSourceCoverage"`
			ReceiveSources        map[openrtc.ReceiveSourceID]openrtc.ReceiveSourceView `json:"receiveSources"`
		}{snap.ReceiveSourceCoverage, snap.ReceiveSources}))
		var decoded openrtc.SessionSnapshot
		must(json.Unmarshal(raw, &decoded))
		check(c, decoded.ReceiveSourceCoverage != nil && decoded.ReceiveSourceCoverage.Completeness == snap.ReceiveSourceCoverage.Completeness, "wire projection changed receive population completeness")
		if snap.ReceiveSourceCoverage.Completeness != "complete" {
			c.Note("empty non-complete inventory does not establish remote absence")
		}
	}),
}
