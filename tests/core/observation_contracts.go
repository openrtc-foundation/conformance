package core

import (
	"context"
	"reflect"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type observationLog struct {
	mu      sync.Mutex
	updates []openrtc.ObservationUpdate
}

func (l *observationLog) add(u openrtc.ObservationUpdate) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.updates = append(l.updates, u)
}
func (l *observationLog) read() []openrtc.ObservationUpdate {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]openrtc.ObservationUpdate(nil), l.updates...)
}
func markerCount(us []openrtc.ObservationUpdate, marker string) int {
	n := 0
	for _, u := range us {
		if u.Snapshot != nil {
			for _, p := range u.Snapshot.Roster.Entries {
				if p.DisplayName.Status == "known" && p.DisplayName.Value != nil && *p.DisplayName.Value == marker {
					n++
				}
			}
		}
	}
	return n
}
func observeFixture(ctx context.Context, c *harness.TestContext) (openrtc.RTCConferenceSession, openrtc.StreamObservation, harness.ObservationControl) {
	target, ok := c.Target.(harness.ObservationTarget)
	if !ok {
		panic(c.NotRun("no controlled observation commit/loss fixture"))
	}
	s := joined(ctx, c).Session
	stream, ok := s.Observation().(openrtc.StreamObservation)
	if !ok {
		panic(c.NotRun("binding has no ordered observe projection; Watch alone does not prove ordered delivery"))
	}
	control := target.ObservationControl()
	must(control.Commit(ctx, s, "baseline"))
	return s, stream, control
}

var ObservationContractTests = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{ID: "CT-OBS-001", Title: "registration racing commit has exactly one initial/stream application", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.3:467–477", "RFC A.1:1301–1308"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s, stream, control := observeFixture(ctx, c)
		log := &observationLog{}
		start := make(chan struct{})
		done := make(chan error, 1)
		go func() { <-start; done <- control.Commit(ctx, s, "handoff") }()
		close(start)
		stop := stream.Observe(log.add)
		defer stop()
		select {
		case err := <-done:
			must(err)
		case <-ctx.Done():
			panic(c.NotRun("handoff commit not observed"))
		}
		must(c.WaitFor(ctx, func() bool { return markerCount(log.read(), "handoff") > 0 }, "covered commit in handoff", 0))
		check(c, markerCount(log.read(), "handoff") == 1, "covered change omitted/duplicated across initial snapshot and continuation")
		us := log.read()
		check(c, len(us) > 0 && us[0].Snapshot != nil, "observe lacks initial snapshot")
		detachedLog := &observationLog{}
		detachStop := stream.Observe(detachedLog.add)
		defer detachStop()
		detachedUpdates := detachedLog.read()
		check(c, len(detachedUpdates) > 0 && detachedUpdates[0].Snapshot != nil, "replacement observer lacks initial state")
		saved := detachedUpdates[0].Snapshot.Roster.Entries
		for k := range saved {
			saved[k] = openrtc.RTCParticipant{ID: k, DisplayName: openrtc.KnownValue("consumer-mutated")}
			break
		}
		check(c, markerCount([]openrtc.ObservationUpdate{{Snapshot: ptrSnapshot(s.Observation().Snapshot())}}, "consumer-mutated") == 0, "consumer mutation of initial snapshot corrupted public state")
		before := map[openrtc.ParticipantConnectionID]openrtc.RTCParticipant{}
		for k, v := range saved {
			before[k] = v
		}
		must(control.Commit(ctx, s, "after-handoff"))
		check(c, reflect.DeepEqual(saved, before), "initial snapshot aliases future state")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-OBS-002", Title: "required intermediate changes preserve order and reentrant delivery", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.3:467–477", "RFC §14.3:1149"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s, stream, control := observeFixture(ctx, c)
		a, b := &observationLog{}, &observationLog{}
		var once sync.Once
		stopA := stream.Observe(func(u openrtc.ObservationUpdate) {
			a.add(u)
			if markerCount([]openrtc.ObservationUpdate{u}, "first") > 0 {
				once.Do(func() { must(control.Commit(ctx, s, "reentrant")) })
			}
		})
		defer stopA()
		stopB := stream.Observe(b.add)
		defer stopB()
		must(control.Commit(ctx, s, "first"))
		must(control.Commit(ctx, s, "last"))
		extract := func(us []openrtc.ObservationUpdate) []string {
			var out []string
			for _, u := range us {
				for _, m := range []string{"first", "reentrant", "last"} {
					if markerCount([]openrtc.ObservationUpdate{u}, m) > 0 {
						out = append(out, m)
					}
				}
			}
			return out
		}
		want := []string{"first", "reentrant", "last"}
		check(c, reflect.DeepEqual(extract(a.read()), want) && reflect.DeepEqual(extract(b.read()), want), "subscribers missed/duplicated/reordered an intermediate or reentrant commit")
		check(c, markerCount([]openrtc.ObservationUpdate{{Snapshot: ptrSnapshot(s.Observation().Snapshot())}}, "last") == 1, "public snapshot omits delivered current change")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-OBS-003", Title: "delivery loss precedes continuation and resnapshot establishes fresh handoff", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.3:475–477", "RFC A.1:1301–1308"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s, stream, control := observeFixture(ctx, c)
		log := &observationLog{}
		stop := stream.Observe(log.add)
		defer stop()
		must(control.LoseNext(ctx, s))
		must(control.Commit(ctx, s, "lost"))
		must(control.Commit(ctx, s, "continuation"))
		lost := false
		resynced := false
		for _, u := range log.read() {
			if u.Loss {
				lost = true
				resynced = false
				continue
			}
			if lost && u.Snapshot != nil && u.Current {
				resynced = true
			}
			if markerCount([]openrtc.ObservationUpdate{u}, "continuation") > 0 {
				check(c, lost, "complete continuation hid local delivery loss")
				check(c, !u.Current || resynced, "continuation promoted stale state")
			}
		}
		check(c, lost, "deliberately lost local observation has no Loss notification")
		fresh := &observationLog{}
		start := make(chan struct{})
		done := make(chan error, 1)
		go func() { <-start; done <- control.Commit(ctx, s, "fresh-next") }()
		close(start)
		stopFresh := stream.Observe(fresh.add)
		defer stopFresh()
		select {
		case err := <-done:
			must(err)
		case <-ctx.Done():
			must(c.NotRun("fresh handoff checkpoint unavailable"))
		}
		current := fresh.read()
		check(c, len(current) > 0 && current[0].Snapshot != nil, "replacement observer lacks initial snapshot")
		first := []openrtc.ObservationUpdate{current[0]}
		check(c, markerCount(first, "continuation") == 1 || markerCount(first, "fresh-next") == 1, "replacement initial snapshot is stale")
		check(c, markerCount(current, "fresh-next") == 1, "replacement snapshot/stream handoff has gap/duplicate")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-OBS-004", Title: "repeated removal stops only the selected observer for future stimuli", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.3:469–477", "RFC A.1:1301–1308"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s, stream, control := observeFixture(ctx, c)
		a, b := &observationLog{}, &observationLog{}
		stopA := stream.Observe(a.add)
		defer stopA()
		stopB := stream.Observe(b.add)
		defer stopB()
		stopA()
		stopA()
		must(control.Commit(ctx, s, "post-removal"))
		check(c, markerCount(a.read(), "post-removal") == 0, "removed observer received new covered change")
		check(c, markerCount(b.read(), "post-removal") == 1, "removing A also removed B or B missed covered change")
	})}),
}

func ptrSnapshot(s openrtc.SessionSnapshot) *openrtc.SessionSnapshot { return &s }
