package selftest

import (
	"context"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"github.com/openrtc/conformance/tests/core"
	"testing"
)

type streamMutant struct {
	*FakeTarget
	defect string
}
type mutantObservationControl struct {
	observationControl
	defect string
}

func (t streamMutant) ObservationControl() harness.ObservationControl {
	return mutantObservationControl{observationControl{t.FakeTarget}, t.defect}
}
func (c mutantObservationControl) Commit(ctx context.Context, s openrtc.RTCConferenceSession, m string) error {
	o := s.(*fakeSession).observation
	o.mu.Lock()
	o.streamDefect = c.defect
	o.mu.Unlock()
	return c.observationControl.Commit(ctx, s, m)
}
func (c mutantObservationControl) LoseNext(ctx context.Context, s openrtc.RTCConferenceSession) error {
	o := s.(*fakeSession).observation
	o.mu.Lock()
	o.streamDefect = c.defect
	o.mu.Unlock()
	return c.observationControl.LoseNext(ctx, s)
}

// RFC §8.3:467–477, A.1:1301–1308: these are harness oracle tests,
// each defective fake must fail a reusable public observation scenario.
func TestOrderedObservationOracles(t *testing.T) {
	for _, test := range core.ObservationContractTests {
		t.Run(test.ID, func(t *testing.T) {
			r := runSelf(MakeFakeTarget(NoDefects), core.ObservationContractTests, test.ID)
			if statusOf(r, test.ID) != harness.Pass {
				t.Fatal(harness.FormatReport(r))
			}
		})
	}
	for _, m := range []struct{ id, defect string }{{"CT-OBS-001", "duplicate"}, {"CT-OBS-002", "drop"}, {"CT-OBS-003", "hide-loss"}, {"CT-OBS-004", "remove-all"}, {"CT-OBS-001", "alias"}, {"CT-OBS-002", "invert"}, {"CT-OBS-003", "stale-resync"}} {
		t.Run(m.defect, func(t *testing.T) {
			r := runSelf(streamMutant{MakeFakeTarget(NoDefects), m.defect}, core.ObservationContractTests, m.id)
			if statusOf(r, m.id) != harness.Fail {
				t.Fatal("defective fake escaped oracle", harness.FormatReport(r))
			}
		})
	}
}
