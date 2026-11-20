// Package core defines mandatory core conformance tests.
package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync/atomic"
	"time"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

var origins = []string{"native", "reconciliation", "local-operation", "native-replay"}
var admissions = []openrtc.AdmissionStatus{"unknown", "pending", "held", "admitted", "denied", "not-applicable"}

func connectionChanges(events []harness.RecordedEvent) []openrtc.StateChange[openrtc.ConnectionSnapshot] {
	var changes []openrtc.StateChange[openrtc.ConnectionSnapshot]
	for _, event := range events {
		if event.Key == "connectionChanged" {
			switch v := event.Envelope.Payload.(type) {
			case openrtc.StateChange[openrtc.ConnectionSnapshot]:
				changes = append(changes, v)
			case *openrtc.StateChange[openrtc.ConnectionSnapshot]:
				changes = append(changes, *v)
			}
		}
	}
	return changes
}

// Roster invariants that must hold for every snapshot; SES-109, CORE-004.
func checkRoster(c *harness.TestContext, snapshot openrtc.SessionSnapshot, label string) {
	r := snapshot.Roster
	check(c, r.ObservedCount == uint64(len(r.Entries)), fmt.Sprintf("%s: observedCount %v != entries %d (SES-109)", label, r.ObservedCount, len(r.Entries)))
	check(c, slices.Contains([]string{"complete", "partial", "unknown", "not-applicable"}, r.Completeness), label+": invalid roster completeness")
	check(c, len(r.CoverageReason) > 0, label+": empty coverageReason (SES-109)")
	if r.Completeness == "complete" && r.ReportedCount.Status == "known" {
		check(c, r.ReportedCount.Value != nil && *r.ReportedCount.Value == r.ObservedCount, label+": complete roster but reportedCount differs from observedCount")
	}
	ids := map[openrtc.ParticipantConnectionID]bool{}
	for _, entry := range r.Entries {
		check(c, !ids[entry.ID], fmt.Sprintf("%s: duplicate participant id %s", label, entry.ID))
		ids[entry.ID] = true
		check(c, slices.Contains([]openrtc.ParticipantPresence{"discovered", "present", "recovering", "departed", "unknown"}, entry.Presence), label+": invalid presence")
		for _, n := range entry.Identity.MembershipIDs {
			check(c, n.AdapterID == c.Descriptor.AdapterID && len(n.Namespace) > 0, label+": native membership id lacks adapter/namespace scope (CORE-005)")
		}
	}
}

// testAbort carries normal harness failures through the compact test bodies.
type testAbort struct{ err error }

func must(err error) {
	if err != nil {
		panic(testAbort{err})
	}
}
func value[T any](v T, err error) T                         { must(err); return v }
func check(c *harness.TestContext, ok bool, message string) { must(c.Check(ok, message)) }
func run(body func(context.Context, *harness.TestContext)) func(context.Context, *harness.TestContext) (err error) {
	return func(ctx context.Context, c *harness.TestContext) (err error) {
		defer func() {
			if v := recover(); v != nil {
				if a, ok := v.(testAbort); ok {
					err = a.err
				} else {
					panic(v)
				}
			}
		}()
		body(ctx, c)
		return nil
	}
}
func opts(c *harness.TestContext) openrtc.JoinSessionOptions {
	return c.JoinOptions(harness.JoinOptionsOverride{})
}
func client(ctx context.Context, c *harness.TestContext) openrtc.RTCClient {
	return value(c.Client(ctx, harness.ClientOptions{}))
}
func joined(ctx context.Context, c *harness.TestContext) harness.JoinedResult {
	return value(c.Joined(ctx, harness.JoinedOptions{}))
}
func awaitJoin(ctx context.Context, c *harness.TestContext, a openrtc.JoinAttempt, label string) openrtc.JoinResult {
	return value(harness.WithinValue(ctx, c, label, 0, func(arg0 context.Context) (openrtc.JoinResult, error) { return a.Result(arg0).Result(arg0) }))
}
func rejectsJoin(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, code openrtc.ErrorCode, label string) {
	_, err := c.Rejects(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) }, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{code}}, label)
	must(err)
}

type brokerFunc func(context.Context, openrtc.BrokerSessionRequest) (openrtc.ResolvedSessionTarget, error)

func (f brokerFunc) Resolve(ctx context.Context, r openrtc.BrokerSessionRequest) openrtc.Async[openrtc.ResolvedSessionTarget] {
	return openrtc.StartOperation(f, func() (openrtc.ResolvedSessionTarget, error) {
		return f(ctx, r)

	})
}

// SessionEventTests defines tests in the original order without executing them.
var SessionEventTests = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-001", Title: "opening a local session provisions, joins, requests credentials and captures nothing", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-001", "SES-104", "CORE-113"}, Observable: "after openSession: idle connection, unknown instance, no credential callback; native counters unchanged", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		target := value(c.Conference(ctx))
		before := c.NativeCounts()
		calls := len(c.Spy.Calls())
		s := c.Open(cl, target)
		check(c, s.Kind() == "conference", "openSession did not return a conference session")
		check(c, s.Connection().Snapshot().State == "idle", fmt.Sprintf("initial connection state is '%s', expected idle (STM-101)", s.Connection().Snapshot().State))
		inst := s.Connection().Snapshot().Instance
		check(c, inst.Status != "known" || inst.Value.Status != "active", "opening a session reported an active instance (SES-104)")
		// Credential providers are asynchronous in Go; allow dispatched calls to
		// become observable before checking that opening requested none.
		must(c.Settle(ctx))
		check(c, len(c.Spy.Calls()) == calls, "credentials requested at open (SES-104)")
		check(c, cl.Sessions()[s.ID()] == s, "client.sessions does not list it")
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, reflect.DeepEqual(before, after), "native effects at open")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-002", Title: "one setup attempt per handle: concurrent join conflicts, later join is invalid-state", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–131", "RFC §8.4:479–483", "SES-002", "SES-105", "STM-012", "SES-117"}, Observable: "second join while pending rejects operation-conflict and the first completes; join after settlement rejects invalid-state; a fresh handle joins", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		target := value(c.Conference(ctx))
		s := c.Open(cl, target)
		gate := held(c)
		defer gate.Resolve(struct{}{})
		first := s.Join(opts(c))
		awaitHeld(ctx, c, gate, "setup did not request credentials while held")
		duplicate := s.Join(opts(c))
		// Invoke while the first is held, then release the fixture gate so an
		// alias-to-first defect is observably successful rather than hanging a waiter.
		gate.Resolve(struct{}{})
		_, duplicateErr := c.Rejects(ctx, func(ctx context.Context) (any, error) { return duplicate.Result(ctx).Result(ctx) }, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{openrtc.InvalidState}}, "concurrent join")
		must(duplicateErr)
		awaitJoin(ctx, c, first, "first join")
		check(c, first.Generation() == s.Generation(), "first join was replaced by the duplicate (SES-105)")
		rejectsJoin(ctx, c, s, "invalid-state", "join after settlement")
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		rejectsJoin(ctx, c, s, "invalid-state", "rejoin on a left handle")
		fresh := c.Open(cl, target)
		awaitJoin(ctx, c, fresh.Join(opts(c)), "fresh handle join")
		check(c, fresh.Generation() != first.Generation(), "fresh handle reused the old generation")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-003", Title: "join result reports connection, admission and snapshot consistently", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-106", "STM-002", "STM-101", "STM-102", "STM-103"}, Observable: "Join result admission and the committed snapshot agree with the session; signaling connected does not imply admitted or active media", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		j := joined(ctx, c)
		r := j.Result
		check(c, j.Session.Observation().Snapshot().Generation == j.Session.Generation(), "generation mismatch")
		check(c, r.Connection.State == "connected", fmt.Sprintf("join resolved while state is '%s' (STM-101)", r.Connection.State))
		check(c, r.Admission == "admitted", "successful join did not establish admission (RFC §7.2:117–131)")
		check(c, slices.Contains([]openrtc.MediaReadiness{"unknown", "inactive", "starting", "active", "degraded", "not-applicable"}, r.Connection.MediaReadiness), "invalid mediaReadiness value (STM-103)")
		check(c, j.Session.Observation().Snapshot().Connection.Admission == r.Admission, "JoinResult.admission differs from snapshot admission (SES-106)")
		check(c, j.Session.Observation().Snapshot().Generation == j.Session.Generation(), "snapshot generation mismatch")
		if r.Admission != "admitted" {
			check(c, r.Connection.MediaReadiness != "active", "media reported active although admission is not admitted (STM-002)")
		}
		check(c, j.Session.Inspection().Snapshot().Configuration.Fields != nil, "configuration result missing")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-004", Title: "initial snapshot includes participants who were present before the join", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-108", "SES-109", "CORE-004", "ADP-005"}, Observable: "The committed session snapshot lists the pre-existing remote endpoint, or declares the roster partial/unknown; no join event is awaited", Needs: []harness.FixtureNeed{"remote-endpoint"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		target := value(c.Conference(ctx))
		remote := value(c.Remote(ctx, target, "early"))
		j := value(c.Joined(ctx, harness.JoinedOptions{Target: &target}))
		checkRoster(c, j.Session.Observation().Snapshot(), "initial snapshot")
		r := j.Session.Observation().Snapshot().Roster
		found := false
		for _, e := range r.Entries {
			if e.Identity.Application.Status == "known" && e.Identity.Application.Value.Subject == remote.Identity().Subject {
				found = true
			}
		}
		if r.Completeness == "complete" {
			check(c, found, "complete initial roster lacks the pre-existing endpoint (SES-108)")
		} else {
			c.Note(fmt.Sprintf("roster is '%s'; presence of the remote is %t", r.Completeness, found))
			if !found {
				must(c.Unknown("roster is not complete and the endpoint was not listed"))
			}
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-005", Title: "roster counts, scope and presence follow the SES-109 invariants", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-109", "CORE-004", "CORE-005", "STM-104"}, Observable: "observedCount equals entries; native membership IDs carry adapter scope; participant IDs are unique", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		j := joined(ctx, c)
		checkRoster(c, j.Session.Observation().Snapshot(), "post-join snapshot")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-006", Title: "held admission stays held, then admission is observed on the same generation", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §7.2:117–169", "RFC §8.5:487–500", "RFC §8.6:506–510", "SES-106", "STM-002", "STM-102"}, Observable: "RFC §7.2:117–131: held join remains pending; admit yields admitted success on the same generation", Needs: []harness.FixtureNeed{"admission-control"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		control := value(c.RequireControl("admission")).(func() harness.AdmissionControl)
		a := control()
		a.Hold()
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		rec := c.Record(s)
		o := opts(c)
		o.Completion = "admission-pending"
		attempt := s.Join(o)
		must(lifecycleFaults(c).AwaitJoinMilestone(ctx, s))
		waiter, stop := context.WithCancel(ctx)
		stop()
		_, err := attempt.Result(ctx).Result(waiter)
		check(c, err == context.Canceled, "held join resolved before admission (RFC §7.2:117–131)")
		check(c, s.Connection().Snapshot().Admission != "admitted", "held admission presented admitted")
		a.Admit()
		r := awaitJoin(ctx, c, attempt, "admitted join")
		check(c, r.Admission == "admitted", "successful join did not establish admission")
		event := value(helpers.NextEvent(ctx, c, rec.Events, "admissionChanged", "admission", func(e harness.RecordedEvent) bool {
			switch p := e.Envelope.Payload.(type) {
			case openrtc.StateChange[openrtc.AdmissionStatus]:
				return p.Current == "admitted"
			case *openrtc.StateChange[openrtc.AdmissionStatus]:
				return p.Current == "admitted"
			}
			return false
		}, 0))
		check(c, event.Envelope.Generation == s.Generation(), "admission was observed on a different generation (SES-106)")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-007", Title: "denied admission rejects a pending attempt and is reported as denied", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-106", "STM-102", "STM-101"}, Observable: "join rejects authorization-denied, the generation becomes terminal, and the snapshot reports admission denied", Needs: []harness.FixtureNeed{"admission-control"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		control := value(c.RequireControl("admission")).(func() harness.AdmissionControl)
		control().Deny()
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		out := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
		check(c, !out.OK, "a denied join resolved successfully")
		_, err := c.ExpectError(out.Error, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"authorization-denied"}}, "denied join")
		must(err)
		must(c.WaitFor(ctx, func() bool { return helpers.IsTerminal(s.Connection().Snapshot().State) }, "terminal state after denial", 0))
		admission := s.Observation().Snapshot().Connection.Admission
		check(c, admission == "denied", fmt.Sprintf("admission is '%s' after denial, expected denied (STM-102)", admission))
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-008", Title: "each offered provisioning mode joins or rejects unsupported-capability before effects", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-003", "SES-100", "SES-105"}, Observable: "per mode: join succeeds, or rejects unsupported-capability with no credential callback", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		factory, ok := c.Target.(harness.ConformanceTarget)
		if !ok {
			must(c.NotRun("fixture does not implement the conformance target contract"))
		}
		exercised := 0
		for _, mode := range []harness.ProvisioningModeName{harness.ExplicitCreate, harness.CreateOnJoin, harness.ExternalProvisioned, harness.BackendBrokered} {
			target := value(factory.ConferenceTarget(ctx, mode))
			if target == nil {
				c.Note(fmt.Sprintf("mode %s: fixture cannot produce a target", mode))
				continue
			}
			exercised++
			cl := client(ctx, c)
			s := c.Open(cl, *target)
			calls := len(c.Spy.Calls())
			out := c.Capture(ctx, func(ctx context.Context) (any, error) { return s.Join(opts(c)).Result(ctx).Result(ctx) })
			if out.OK {
				c.Note(fmt.Sprintf("mode %s: joined", mode))
				continue
			}
			failure := value(c.ExpectError(out.Error, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"unsupported-capability"}}, string(mode)))
			check(c, len(c.Spy.Calls()) == calls, fmt.Sprintf("mode %s: unsupported mode rejected only after credentials were requested (SES-100)", mode))
			c.Note(fmt.Sprintf("mode %s: rejected %s", mode, failure.Code))
		}
		if exercised == 0 {
			must(c.NotRun("fixture produced no provisioning mode"))
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-009", Title: "explicit create reports allocation, not a running instance, and open allocates nothing", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-004", "SES-101", "SES-103"}, Observable: "SessionDefinition.allocation is resource or reservation; a session opened on it has no active instance; open() adds no provisioning call", Needs: []harness.FixtureNeed{"admin", "native-effects"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		native := value(c.RequireNative())
		admin := value(c.Administration(ctx))
		before := native.Counts()
		name := fmt.Sprintf("conformance-%d", time.Now().UnixMilli())
		def := value(harness.WithinValue(ctx, c, "create", 0, func(ctx context.Context) (openrtc.SessionDefinition, error) {
			return admin.Provisioner().Create(ctx, openrtc.CreateSessionOptions{Name: &name}).Result(ctx)
		}))
		check(c, def.Allocation == "resource" || def.Allocation == "reservation", "create did not state resource or reservation (SES-004)")
		check(c, def.Address.AdapterID == c.Descriptor.AdapterID, "definition address is not scoped to the adapter (CORE-005)")
		after := native.Counts()
		check(c, after.ProvisioningCalls == before.ProvisioningCalls+1, "create did not perform exactly one provisioning call")
		cl := client(ctx, c)
		s := c.Open(cl, openrtc.ConferenceTarget{Kind: "conference", Provisioning: openrtc.SessionProvisioning{Mode: "explicit-create", Definition: &def}})
		inst := s.Connection().Snapshot().Instance
		check(c, inst.Status != "known" || inst.Value.Status != "active", "an allocated definition was presented as an active instance (SES-004)")
		check(c, native.Counts().ProvisioningCalls == after.ProvisioningCalls, "openSession provisioned (SES-101)")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-010", Title: "broker: nested broker, foreign scope and failure are rejected without retry or fallback", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §9:544–570", "SES-102", "SES-103", "SES-006", "ERR-001"}, Observable: "join rejects with RTCError; broker called once on failure; original cause retained; no admission", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		request := openrtc.BrokerSessionRequest{Application: openrtc.ApplicationIdentity{Namespace: "conformance", Subject: "broker"}, Resource: "conformance"}
		brokered := func(b openrtc.SessionBroker) openrtc.ConferenceTarget {
			return openrtc.ConferenceTarget{Kind: "conference", Provisioning: openrtc.SessionProvisioning{Mode: "backend-brokered", Broker: b, Request: &request}}
		}
		factory, ok := c.Target.(harness.ConformanceTarget)
		if !ok {
			must(c.NotRun("fixture does not implement the conformance target contract"))
		}
		template := value(factory.ConferenceTarget(ctx, harness.ExternalProvisioned))
		sentinel := errors.New("broker transport failure (conformance sentinel)")
		var calls atomic.Int32
		failing := brokerFunc(func(context.Context, openrtc.BrokerSessionRequest) (openrtc.ResolvedSessionTarget, error) {
			calls.Add(1)
			return openrtc.ResolvedSessionTarget{}, sentinel
		})
		before := c.NativeCounts()
		capture := func(b openrtc.SessionBroker) harness.Captured[any] {
			return c.Capture(ctx, func(ctx context.Context) (any, error) {
				return c.Open(cl, brokered(b)).Join(opts(c)).Result(ctx).Result(ctx)
			})
		}
		failure := capture(failing)
		check(c, !failure.OK, "join succeeded although the broker failed")
		e := value(c.ExpectError(failure.Error, harness.ExpectedFailure{}, "broker failure"))
		if calls.Load() == 0 && e.Code == openrtc.Unsupported {
			must(c.NotApplicable("adapter does not support the backend-brokered mode"))
		}
		check(c, calls.Load() == 1, fmt.Sprintf("broker invoked %d times; automatic retry forbidden (SES-103)", calls.Load()))
		check(c, e.Cause == nil && (e.ProviderError.Value == nil || e.ProviderError.Value.Original == nil), "broker default failure leaked native original (RFC §9:544)")
		nested := brokerFunc(func(context.Context, openrtc.BrokerSessionRequest) (openrtc.ResolvedSessionTarget, error) {
			return openrtc.ResolvedSessionTarget{Mode: "backend-brokered", Broker: failing, Request: &request}, nil
		})
		nf := capture(nested)
		check(c, !nf.OK, "nested broker resolution was accepted (SES-102)")
		_, err := c.ExpectError(nf.Error, harness.ExpectedFailure{}, "nested broker")
		must(err)
		if template != nil && template.Provisioning.Mode == "external-provisioned" && template.Provisioning.Address != nil {
			address := *template.Provisioning.Address
			address.AdapterID = helpers.ForeignAdapter
			unknown := openrtc.Known[openrtc.SessionDefinitionDescriptor]{Status: "unknown"}
			foreign := brokerFunc(func(context.Context, openrtc.BrokerSessionRequest) (openrtc.ResolvedSessionTarget, error) {
				return openrtc.ResolvedSessionTarget{Mode: "external-provisioned", Address: &address, KnownDefinition: &unknown}, nil
			})
			f := capture(foreign)
			check(c, !f.OK, "broker result for a foreign adapter was accepted (SES-102)")
			_, err = c.ExpectError(f.Error, harness.ExpectedFailure{}, "foreign-scope broker result")
			must(err)
		} else {
			c.Partial("foreign-scope broker check skipped: fixture has no external-provisioned target")
		}
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.Admissions == before.Admissions, "admission after broker failure (SES-102)")
		}
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-SES-011", Title: "leave affects only local participation", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"SES-005", "SES-119", "CORE-153"}, Observable: "after the local session leaves, a remote endpoint is still connected and the instance is not reported ended", Needs: []harness.FixtureNeed{"remote-endpoint"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		target := value(c.Conference(ctx))
		remote := value(c.Remote(ctx, target, "stays"))
		j := value(c.Joined(ctx, harness.JoinedOptions{Target: &target}))
		_, err := harness.WithinValue(ctx, c, "leave", 0, func(arg0 context.Context) (openrtc.CleanupReport, error) {
			return cleanupAfter(arg0, j.Session.Inspection(), j.Session.Leave(arg0))
		})
		must(err)
		must(c.Settle(ctx))
		check(c, remote.IsConnected(), "remote endpoint was disconnected by the local leave (SES-005)")
		inst := j.Session.Connection().Snapshot().Instance
		check(c, inst.Status != "known" || inst.Value.Status != "ended", "local leave reported the instance ended (SES-119)")
	})}),
	harness.DefineTest(harness.ConformanceTest{ID: "CT-EVT-001", Title: "envelopes use fixed keys with strictly increasing sequence and consistent revisions", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CORE-006", "CORE-108", "CORE-109", "CORE-111", "STM-108"}, Observable: "recorded envelopes: one owner, the session generation, strictly increasing sequence, non-decreasing revision, shared transactions share a revision, valid origins", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		s := c.Open(cl, value(c.Conference(ctx)))
		rec := c.Record(s)
		awaitJoin(ctx, c, s.Join(opts(c)), "join")
		_, err := cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
		must(err)
		must(c.Settle(ctx))
		events := rec.Events()
		check(c, len(events) > 0, "no events were delivered for a full join and leave")
		found := false
		for _, e := range events {
			if e.Key == "connectionChanged" {
				found = true
			}
		}
		check(c, found, "no connectionChanged event for a join (CORE-108)")
		owners := map[openrtc.LocalOwnerID]bool{}
		ids := map[openrtc.LocalEventID]bool{}
		transactions := map[openrtc.TransactionID]openrtc.Revision{}
		var lastSeq uint64
		haveSequence := false
		var lastRev openrtc.Revision
		for _, event := range events {
			e := event.Envelope
			check(c, slices.Contains(harness.SessionEventKeys, event.Key), fmt.Sprintf("unexpected key %s", event.Key))
			check(c, !haveSequence || e.Sequence > lastSeq, fmt.Sprintf("sequence %v not strictly increasing after %v (CORE-109)", e.Sequence, lastSeq))
			lastSeq = e.Sequence
			haveSequence = true
			check(c, e.Revision >= lastRev, "revision decreased (CORE-109)")
			lastRev = e.Revision
			owners[e.OwnerID] = true
			check(c, !ids[e.EventID], "duplicate eventId")
			ids[e.EventID] = true
			check(c, slices.Contains(origins, e.Origin), fmt.Sprintf("invalid origin %s", e.Origin))
			check(c, e.Generation == s.Generation(), "envelope generation differs from the session generation")
			if rev, ok := transactions[e.Transaction]; ok {
				check(c, rev == e.Revision, "one transaction spans two revisions (CORE-109)")
			}
			transactions[e.Transaction] = e.Revision
			if e.NativeID.Status == "known" {
				check(c, e.NativeID.Value.Value != string(e.EventID), "local eventId presented as native ID (CORE-111)")
			}
		}
		check(c, len(owners) == 1, "events carried more than one ownerId")
	})}),
}
