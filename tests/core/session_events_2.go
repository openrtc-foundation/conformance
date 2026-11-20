package core

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

func eventConnection(e openrtc.EventEnvelope[any]) openrtc.StateChange[openrtc.ConnectionSnapshot] {
	switch p := e.Payload.(type) {
	case openrtc.StateChange[openrtc.ConnectionSnapshot]:
		return p
	case *openrtc.StateChange[openrtc.ConnectionSnapshot]:
		return *p
	}
	return openrtc.StateChange[openrtc.ConnectionSnapshot]{}
}

var sessionEventTests2 = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-002", Title: "the snapshot is committed before listeners run", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "STM-008", "STM-108"}, Observable: "inside every listener, snapshot().revision >= envelope.revision, and equal revisions show the state the event reports", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		rec := c.Record(s)
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		events := rec.Events()
		check(c, len(events) > 0, "no events observed while leaving")
		for _, e := range events {
			check(c, e.SnapshotAtDelivery.Revision >= e.Envelope.Revision, fmt.Sprintf("listener saw snapshot r%v older than event r%v (STM-008)", e.SnapshotAtDelivery.Revision, e.Envelope.Revision))
			if e.Key == "connectionChanged" && e.SnapshotAtDelivery.Revision == e.Envelope.Revision {
				check(c, e.SnapshotAtDelivery.Connection.State == eventConnection(e.Envelope).Current.State, "snapshot state differs from the state announced by the event at the same revision")
			}
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-003", Title: "a throwing listener does not stop delivery to others or fail the operation", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-006", "CORE-109", "STM-108"}, Observable: "a second listener still receives every connection event, and join still resolves", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		var mu sync.Mutex
		var received []string
		a := s.Observation().On("connectionChanged", func(openrtc.EventEnvelope[any]) { panic("listener failure (conformance)") })
		b := s.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) {
			mu.Lock()
			defer mu.Unlock()
			received = append(received, string(eventConnection(e).Current.State))
		})
		c.Track("listeners", func(context.Context) error { a(); b(); return nil })
		awaitJoin(ctx, c, s.Join(opts(c)), "join with a throwing listener")
		mu.Lock()
		seen := append([]string(nil), received...)
		mu.Unlock()
		check(c, slices.Contains(seen, "connected"), fmt.Sprintf("second listener saw [%s] but not 'connected' (CORE-006)", strings.Join(seen, ", ")))
		c.Note("a conforming implementation must not let the throw escape as an unhandled exception; the runner cannot observe that")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-004", Title: "on() does not replay history", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-110", "CORE-112", "STM-108"}, Observable: "a listener registered after join receives nothing until a new commit, then receives the new events", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		var mu sync.Mutex
		var late []string
		stop := s.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) {
			mu.Lock()
			defer mu.Unlock()
			late = append(late, string(eventConnection(e).Current.State))
		})
		c.Track("late listener", func(context.Context) error { stop(); return nil })
		must(c.Settle(ctx))
		mu.Lock()
		before := append([]string(nil), late...)
		mu.Unlock()
		check(c, len(before) == 0, fmt.Sprintf("on() replayed [%s] (CORE-110)", strings.Join(before, ", ")))
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		mu.Lock()
		n := len(late)
		mu.Unlock()
		check(c, n > 0, "listener received nothing for the leave that followed")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-005", Title: "once() fires at most once, even when the handler causes further commits", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-110", "CORE-007"}, Observable: "a once-listener whose handler calls leave() again, while a first leave() is already running, is invoked exactly once", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		var count atomic.Int32
		stop := s.Observation().Once("connectionChanged", func(openrtc.EventEnvelope[any]) { count.Add(1); go cleanupAfter(ctx, s.Inspection(), s.Leave(ctx)) })
		c.Track("once listener", func(context.Context) error { stop(); return nil })
		go cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(s.Connection().Snapshot().State) }, "terminal after leave", 0))
		must(c.Settle(ctx))
		check(c, count.Load() == 1, fmt.Sprintf("once-listener invoked %d times (CORE-110)", count.Load()))
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-006", Title: "unsubscribe is idempotent; off removes by name and listener identity", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-007", "CORE-110"}, Observable: "no callback after unsubscribe/off returns; repeated unsubscribe and off of an absent listener do not throw; two registrations of one listener are both removed by off", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := joined(ctx, c).Session
		var calls atomic.Int32
		listener := func(openrtc.EventEnvelope[any]) { calls.Add(1) }
		u := s.Observation().On("connectionChanged", listener)
		u()
		u()
		s.Observation().Off("connectionChanged", func(openrtc.EventEnvelope[any]) {})
		s.Observation().On("connectionChanged", listener)
		s.Observation().On("connectionChanged", listener)
		s.Observation().Off("connectionChanged", listener)
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		check(c, calls.Load() == 0, fmt.Sprintf("%d callback(s) after unsubscribe/off (CORE-007, CORE-110)", calls.Load()))
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-007", Title: "observations caused inside a listener queue after the current dispatch", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-109", "CORE-007", "STM-108"}, Observable: "listener A calls leave() on 'connected'; listener B still sees 'connected' before 'leaving', in strictly increasing sequence", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		type entry struct {
			kind     string
			sequence uint64
		}
		var mu sync.Mutex
		var seen []entry
		var triggered atomic.Bool
		a := s.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) {
			if eventConnection(e).Current.State == "connected" && triggered.CompareAndSwap(false, true) {
				go cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
			}
		})
		b := s.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) {
			mu.Lock()
			defer mu.Unlock()
			seen = append(seen, entry{string(eventConnection(e).Current.State), e.Sequence})
		})
		c.Track("listeners", func(context.Context) error { a(); b(); return nil })
		attempt := s.Join(opts(c))
		c.Capture(ctx, func(ctx context.Context) (any, error) { return attempt.Result(ctx).Result(ctx) })
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(s.Connection().Snapshot().State) }, "terminal state", 0))
		mu.Lock()
		copySeen := append([]entry(nil), seen...)
		mu.Unlock()
		kinds := []string{}
		for _, v := range copySeen {
			kinds = append(kinds, v.kind)
		}
		check(c, triggered.Load(), "the connected observation never reached listener A")
		x, y := slices.Index(kinds, "connected"), slices.Index(kinds, "leaving")
		check(c, x >= 0 && (y < 0 || x < y), fmt.Sprintf("listener B saw [%s]: reentrant observation overtook the current dispatch (CORE-109)", strings.Join(kinds, ", ")))
		for i := 1; i < len(copySeen); i++ {
			check(c, copySeen[i].sequence > copySeen[i-1].sequence, "sequence not strictly increasing under reentrancy")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-008", Title: "watch delivers the current snapshot then newer revisions without a gap", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-009", "STM-108", "SES-108", "CORE-112"}, Observable: "first delivery is not older than the snapshot at registration; revisions strictly increase; the last delivery equals the final snapshot; none after unwatch", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		before := s.Observation().Snapshot().Revision
		var mu sync.Mutex
		var delivered []openrtc.SessionSnapshot
		u := s.Observation().Watch(func(v openrtc.SessionSnapshot) { mu.Lock(); defer mu.Unlock(); delivered = append(delivered, v) })
		c.Track("watch", func(context.Context) error { u(); return nil })
		awaitJoin(ctx, c, s.Join(opts(c)), "join")
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		mu.Lock()
		snapshots := append([]openrtc.SessionSnapshot(nil), delivered...)
		mu.Unlock()
		check(c, len(snapshots) > 0, "watch delivered no initial snapshot (STM-009)")
		check(c, snapshots[0].Revision >= before, "initial snapshot older than the registration")
		for i := 1; i < len(snapshots); i++ {
			check(c, snapshots[i].Revision > snapshots[i-1].Revision, "watch revisions are not strictly increasing")
		}
		check(c, snapshots[len(snapshots)-1].Revision == s.Observation().Snapshot().Revision, "last watched revision differs from the final snapshot (handoff gap)")
		mu.Lock()
		count := len(delivered)
		mu.Unlock()
		u()
		mu.Lock()
		n := len(delivered)
		mu.Unlock()
		check(c, n == count, "delivery after unwatch")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CON-001", Title: "connection transitions follow the STM-101 table and chain continuously", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-101", "STM-001", "STM-117"}, Observable: "every connectionChanged edge is listed in the table; each previous equals the prior current; ended/failed are final", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		rec := c.Record(s)
		awaitJoin(ctx, c, s.Join(opts(c)), "join")
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(s.Connection().Snapshot().State) }, "terminal state", 0))
		must(c.Settle(ctx))
		changes := connectionChanges(rec.Events())
		check(c, len(changes) >= 2, "fewer than two connection changes for join and leave")
		prior := "idle"
		for _, change := range changes {
			next := string(change.Current.State)
			if change.Previous.Status == "known" {
				check(c, string(change.Previous.Value.State) == prior, fmt.Sprintf("previous '%s' does not chain from '%s'", change.Previous.Value.State, prior))
			}
			check(c, slices.Contains(helpers.LegalConnectionEdges[prior], next), fmt.Sprintf("illegal edge %s -> %s (STM-101)", prior, next))
			prior = next
		}
		check(c, prior == "ended" || prior == "failed", fmt.Sprintf("final observed state '%s' is not terminal after leave", prior))
		check(c, slices.Contains(helpers.ConnectionKinds(rec.Events()), "connected"), "the generation never reported connected")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CON-002", Title: "a connected session captures and publishes nothing unless asked", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-002", "SES-107", "MED-001"}, Observable: "with captureDefaults set and no initialMedia, no capture or publication exists after join", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cfg := value(c.ClientConfig(harness.ClientOptions{}))
		echo := true
		video := true
		cfg.CaptureDefaults = &openrtc.CaptureDefaults{Audio: &echo, Video: &video}
		native := c.NativeCounts()
		cl := value(c.Client(ctx, harness.ClientOptions{Config: &cfg}))
		s := value(c.Joined(ctx, harness.JoinedOptions{Client: cl})).Session
		for _, p := range s.Observation().Snapshot().Publications {
			check(c, p.Source.Status != "known", "a local publication exists without initialMedia (SES-107)")
		}
		check(c, len(s.Observation().Snapshot().Publications) == 0, "media.publications is non-empty")
		after := c.NativeCounts()
		if native != nil && after != nil {
			check(c, after.Captures == native.Captures, "capture started from defaults alone")
			check(c, after.Publications == native.Publications, "publication started unasked")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CON-003", Title: "a terminal generation never reconnects; rejoin needs a fresh handle", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-001", "SES-002", "SES-117", "STM-117"}, Observable: "after leave: terminal state, unchanged generation, no later connection events, join rejects; a fresh handle gets a new generation", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		target := value(c.Conference(ctx))
		first := c.Open(cl, target)
		rec := c.Record(first)
		awaitJoin(ctx, c, first.Join(opts(c)), "join")
		_, err := cleanupAfter(ctx, first.Inspection(), first.Leave(ctx))
		must(err)
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(first.Connection().Snapshot().State) }, "terminal state", 0))
		generation := first.Generation()
		count := len(rec.Of("connectionChanged"))
		must(c.Settle(ctx))
		check(c, first.Generation() == generation, "generation changed after terminal")
		check(c, len(rec.Of("connectionChanged")) == count, "connection events after the terminal state (STM-001)")
		rejectsJoin(ctx, c, first, "invalid-state", "join on a terminal handle")
		second := c.Open(cl, target)
		awaitJoin(ctx, c, second.Join(opts(c)), "fresh join")
		check(c, second.Generation() != generation && second.ID() != first.ID(), "fresh handle did not receive a new generation and id")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-CON-004", Title: "transport loss recovers inside one generation without implying continuity", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"STM-003", "STM-101", "SES-116", "SES-118"}, Observable: "connection reports recovering with planes and a known cause/continuity enum, then connected on the same generation; ended/failed only if recovery truly failed", Needs: []harness.FixtureNeed{"transport-control"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		control := value(c.RequireControl("interruptTransport")).(func(context.Context, openrtc.RTCSession) (harness.TransportInterruption, error))
		s := joined(ctx, c).Session
		rec := c.Record(s)
		generation := s.Generation()
		interruption := value(harness.WithinValue(ctx, c, "interrupt transport", 0, func(ctx context.Context) (harness.TransportInterruption, error) { return control(ctx, s) }))
		event := value(helpers.NextEvent(ctx, c, rec.Events, "connectionChanged", "recovering state", func(e harness.RecordedEvent) bool {
			return eventConnection(e.Envelope).Current.State == "recovering"
		}, 0))
		connection := eventConnection(event.Envelope).Current
		state := connection.State
		check(c, state == "recovering" && connection.Recovery.Status == "known" && connection.Recovery.Value != nil, "unreachable")
		r := connection.Recovery.Value
		check(c, len(r.Planes) > 0, "recovery names no affected plane (STM-003)")
		check(c, slices.Contains([]string{"transport-failure", "planned-move", "admission-change", "unknown"}, r.Cause), "invalid recovery cause")
		check(c, slices.Contains([]string{"resumed", "rejoined", "unknown"}, r.Continuity), "invalid continuity value")
		check(c, event.Envelope.Generation == generation, "recovery changed generation")
		must(c.Within(ctx, "restore transport", 0, interruption.Restore))
		must(c.WaitFor(ctx, func() bool {
			state := s.Connection().Snapshot().State
			return state == "connected" || helpers.IsTerminal(state)
		}, "recovery outcome", 0))
		check(c, s.Generation() == generation, "generation changed during recovery (STM-001)")
		if state := s.Connection().Snapshot().State; state != "connected" {
			c.Note(fmt.Sprintf("recovery ended in '%s'; not asserted as failure", state))
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-ID-001", Title: "identity layers stay separate: display names and ids are not identity", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §4:23; §8.3:477", "RFC §4:23", "RFC §6:33–35", "RFC §8.2:461–463", "CORE-004", "CORE-005", "CORE-107", "SES-110"}, Observable: "a remote's display name never appears as its application identity; participant ids are scoped; the local application identity is not rewritten", Needs: []harness.FixtureNeed{"remote-endpoint"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		target := value(c.Conference(ctx))
		remote := value(c.Remote(ctx, target, "display-name-impostor"))
		identity := c.FreshIdentity("local")
		s := value(c.Joined(ctx, harness.JoinedOptions{Target: &target, Identity: &identity})).Session
		must(c.WaitFor(ctx, func() bool { return len(s.Observation().Snapshot().Roster.Entries) > 0 }, "roster to list someone", 0))
		snapshot := s.Observation().Snapshot()
		checkRoster(c, snapshot, "identity roster")
		for _, e := range snapshot.Roster.Entries {
			if e.Identity.Application.Status == "known" {
				app := e.Identity.Application.Value
				check(c, app.Subject != "display-name-impostor" || remote.Identity().Subject == app.Subject, "a display name was reported as an application identity (CORE-005)")
				check(c, app.Subject != identity.Subject || app.Namespace == identity.Namespace, "local identity rewritten (SES-110)")
			}
			check(c, string(e.ID) != remote.Identity().Subject || len(e.Identity.MembershipIDs) > 0, "participant id is the bare application subject without scoped native ids (CORE-107)")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-ID-002", Title: "duplicate application identity follows the advertised collision policy", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §8.2:461–463; §14.3:1134", "RFC §8.2:461–463", "RFC §14.3:1134", "SES-007", "SES-110", "CORE-005"}, Observable: "two clients with one identity: reject-new rejects the second; replace-existing ends the first; allow-multiple keeps both; unknown is reported unknown", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		capability := c.DescriptorFor(joined(ctx, c).Session.Inspection().Capabilities().Snapshot(), "session.conference")
		if capability == nil {
			must(c.Unknown("conference collision policy missing"))
		}
		policy := capability.IdentityCollision
		if policy == "unknown" {
			c.Note("unknown collision policy does not establish replacement authority or permit automatic retry (RFC §8.2:461–463)")
		}
		if policy == "not-applicable" {
			must(c.NotApplicable("collision policy not applicable"))
		}
		identity := c.FreshIdentity("duplicate")
		target := value(c.Conference(ctx))
		provider := value(c.Provider(ctx, nil))
		aClient := value(c.Client(ctx, harness.ClientOptions{Provider: provider, Identity: &identity}))
		bClient := value(c.Client(ctx, harness.ClientOptions{Provider: provider, Identity: &identity}))
		a := c.Open(aClient, target)
		awaitJoin(ctx, c, a.Join(opts(c)), "first join")
		b := c.Open(bClient, target)
		second := c.Capture(ctx, func(ctx context.Context) (any, error) { return b.Join(opts(c)).Result(ctx).Result(ctx) })
		switch policy {
		case "reject-new":
			check(c, !second.OK, "reject-new advertised but the second endpoint was admitted")
			_, err := c.ExpectError(second.Error, harness.ExpectedFailure{}, "second endpoint")
			must(err)
			check(c, a.Connection().Snapshot().State == "connected", "first endpoint lost on reject-new")
		case "replace-existing":
			check(c, second.OK, fmt.Sprintf("replace-existing advertised but second join failed: %v", second.Error))
			must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(a.Connection().Snapshot().State) }, "first endpoint replaced", 0))
		case "unknown":
			if second.OK {
				check(c, a.ID() != b.ID() && a.Generation() != b.Generation(), "unknown collision policy collapsed separate incarnations")
			}
			c.Partial("collision policy unknown: independent submission/retry and replacement-authority evidence unavailable; neither success nor rejection proves policy")
		default:
			check(c, second.OK, fmt.Sprintf("allow-multiple advertised but second join failed: %v", second.Error))
			check(c, a.Connection().Snapshot().State == "connected", "first endpoint dropped under allow-multiple")
		}
	})}),
}

// SessionEventsTests contains all session-event tests in source order.
var SessionEventsTests = append(append([]harness.ConformanceTest{}, SessionEventTests...), sessionEventTests2...)
