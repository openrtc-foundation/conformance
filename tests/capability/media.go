package capability

import (
	"context"
	"fmt"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"math"
	"slices"
	"time"
)

type abort struct{ err error }

func must(e error) {
	if e != nil {
		panic(abort{e})
	}
}
func value[T any](v T, e error) T                    { must(e); return v }
func check(c *harness.TestContext, b bool, s string) { must(c.Check(b, s)) }
func run(f func(context.Context, *harness.TestContext)) func(context.Context, *harness.TestContext) (err error) {
	return func(ctx context.Context, c *harness.TestContext) (err error) {
		defer func() {
			if v := recover(); v != nil {
				if a, ok := v.(abort); ok {
					err = a.err
				} else {
					panic(v)
				}
			}
		}()
		f(ctx, c)
		return nil
	}
}
func within[T any](ctx context.Context, c *harness.TestContext, label string, f func(context.Context) (T, error)) T {
	return value(harness.WithinValue(ctx, c, label, 0, f))
}
func joined(ctx context.Context, c *harness.TestContext) openrtc.RTCConferenceSession {
	return value(c.Joined(ctx, harness.JoinedOptions{})).Session
}
func capture(ctx context.Context, c *harness.TestContext, s openrtc.RTCConferenceSession, own string) openrtc.CaptureSource {
	r := c.Capture(ctx, func(ctx context.Context) (any, error) {
		return s.Media().Capture(ctx, openrtc.CaptureRequest{Kinds: []openrtc.MediaKind{"audio"}, Ownership: own}).Result(ctx)
	})
	if !r.OK {
		var failure *openrtc.RTCFailure
		switch e := r.Error.(type) {
		case *openrtc.RTCFailure:
			failure = e
		case *openrtc.RTCOperationError:
			if e != nil {
				failure = e.RTCFailure
			}
		case *openrtc.RTCObservationError:
			if e != nil {
				failure = e.RTCFailure
			}
		}
		if failure != nil && failure.Category == "media" {
			must(c.NotRun(fmt.Sprintf("capture failed in this environment (%s); media devices are a fixture prerequisite", failure.Code)))
		}
		_, e := c.ExpectError(r.Error, harness.ExpectedFailure{}, "capture")
		must(e)
		must(c.Fail("capture rejected"))
	}
	source := r.Value.(openrtc.CaptureSource)
	c.Track("capture source", func(ctx context.Context) error {
		if source.Snapshot().State != "stopped" {
			_, e := source.Stop(ctx).Result(ctx)
			return e
		}
		return nil
	})
	return source
}
func localPublication(s openrtc.RTCConferenceSession) *openrtc.TrackPublication {
	for _, p := range s.Observation().Snapshot().Publications {
		if p.Source.Status == "known" {
			return &p
		}
	}
	return nil
}
func dial(ctx context.Context, c *harness.TestContext) (openrtc.RTCClient, openrtc.RTCCallSession) {
	target, ok := c.Target.(harness.CallingTarget)
	if !ok {
		must(c.NotRun("fixture provides no call account/destination"))
	}
	d := target.Calling()
	client := value(c.Client(ctx, harness.ClientOptions{}))
	call := openrtc.ClientExtension(client, openrtc.CallingKey).Dial(ctx, openrtc.DialOptions{Account: d.Account, Destination: d.Destination})
	c.Track("call session", func(ctx context.Context) error {
		_, e := cleanupAfter(ctx, call.Inspection(), call.Dispose(ctx))
		return e
	})
	a := call.Attempt()
	if a.Status == "known" && a.Value != nil {
		within(ctx, c, "dial setup", func(arg0 context.Context) (openrtc.CallSetupResult, error) {
			return (*a.Value).Result(arg0).Result(arg0)
		})
	}
	return client, call
}
func test1(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	before := c.NativeCounts()
	n := len(s.Observation().Snapshot().Publications)
	src := capture(ctx, c, s, "application")
	check(c, slices.Contains([]openrtc.CaptureState{"acquiring", "active"}, src.Snapshot().State), fmt.Sprintf("capture state '%s'", src.Snapshot().State))
	check(c, src.Snapshot().Ownership == "application", "ownership differs from the request (MED-006)")
	check(c, len(s.Observation().Snapshot().Publications) == n, "capture created a publication (MED-001)")
	after := c.NativeCounts()
	if before != nil && after != nil {
		check(c, after.Publications == before.Publications, "capture published natively")
	}
}

// RFC §7.4:310–321, §8.1:448–451: a successful mute retains P and its source.
func test2(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	src := capture(ctx, c, s, "session")
	pubs := value(openrtc.SessionExtension(s, openrtc.IndependentPublicationKey).Publish(ctx, src.Snapshot().ID, openrtc.PublishOptions{Kinds: src.Snapshot().Kinds}).Result(ctx))
	check(c, len(pubs) > 0, "publish produced no target")
	p := pubs[0]
	source := src.Snapshot().ID
	value(s.Media().SetMuted(ctx, p.ID, true, openrtc.OperationOptions{}).Result(ctx))
	v, ok := s.Observation().Snapshot().Publications[p.ID]
	check(c, ok && v.State == "published" && v.Source.Value != nil && *v.Source.Value == source, "mute removed or replaced target/source")
	check(c, src.Snapshot().ID == source && src.Snapshot().State != "stopped", "successful mute ended retained source")
	check(c, v.LocalMuteIntent.Status == "known" && v.LocalMuteIntent.Value != nil && *v.LocalMuteIntent.Value, "mute intent not established")
	value(s.Media().SetMuted(ctx, p.ID, false, openrtc.OperationOptions{}).Result(ctx))
	v, ok = s.Observation().Snapshot().Publications[p.ID]
	check(c, ok && v.LocalMuteIntent.Status == "known" && v.LocalMuteIntent.Value != nil && !*v.LocalMuteIntent.Value, "unmute intent not established")
	c.Partial("independent output evidence is required for claimed content suppression boundary (RFC §14.2:1108–1115)")
}

func test3(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	src := capture(ctx, c, s, "application")
	within(ctx, c, "disable", func(ctx context.Context) (openrtc.Void, error) {
		return s.Media().SetDisabled(ctx, src.Snapshot().ID, true).Result(ctx)
	})
	check(c, src.Snapshot().State == "disabled", fmt.Sprintf("state '%s' after disable", src.Snapshot().State))
	within(ctx, c, "re-enable", func(ctx context.Context) (openrtc.Void, error) {
		return s.Media().SetDisabled(ctx, src.Snapshot().ID, false).Result(ctx)
	})
	check(c, src.Snapshot().State == "active", fmt.Sprintf("state '%s' after re-enable", src.Snapshot().State))
	within(ctx, c, "stop", func(arg0 context.Context) (openrtc.Void, error) { return src.Stop(arg0).Result(arg0) })
	check(c, src.Snapshot().State == "stopped", fmt.Sprintf("state '%s' after stop", src.Snapshot().State))
	_, e := c.Rejects(ctx, func(ctx context.Context) (any, error) {
		return s.Media().SetDisabled(ctx, src.Snapshot().ID, false).Result(ctx)
	}, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"invalid-state"}}, "disable after stop")
	must(e)
	c.Partial("production resuming/ceasing is judged from reported state; no output observer (D-20)")
}
func test4(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	a := capture(ctx, c, s, "session")
	b := capture(ctx, c, s, "application")
	within(ctx, c, "dispose", func(arg0 context.Context) (openrtc.CleanupReport, error) {
		return cleanupAfter(arg0, s.Inspection(), s.Dispose(arg0))
	})
	must(c.Settle(ctx))
	check(c, a.Snapshot().State == "stopped", fmt.Sprintf("session-owned capture is '%s' after dispose (MED-117)", a.Snapshot().State))
	check(c, b.Snapshot().State != "stopped", "application-owned capture was stopped by session disposal (MED-006)")
	within(ctx, c, "owner stop", func(arg0 context.Context) (openrtc.Void, error) { return b.Stop(arg0).Result(arg0) })
	check(c, b.Snapshot().State == "stopped", "owner could not stop its own capture")
}
func test5(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	ctrl := openrtc.SessionExtension(s, openrtc.IndependentPublicationKey)
	src := capture(ctx, c, s, "application")
	publications := within(ctx, c, "publish", func(ctx context.Context) ([]openrtc.TrackPublication, error) {
		return ctrl.Publish(ctx, src.Snapshot().ID, openrtc.PublishOptions{Kinds: []openrtc.MediaKind{"audio"}}).Result(ctx)
	})
	check(c, len(publications) > 0, "publish returned no publications")
	for _, p := range publications {
		check(c, slices.Contains([]openrtc.PublicationState{"publishing", "published"}, p.State), fmt.Sprintf("publication state '%s'", p.State))
		check(c, effect(s.Inspection().PublicationEffects(p.ID).Guaranteed.Capture) != "acquired", "publish silently acquired a device (MED-001)")
		e := mediaEffects(ctx, c, s.Inspection(), "unpublish", ctrl.Unpublish(ctx, p.ID, openrtc.UnpublishOptions{Capture: "retain"}))
		check(c, effect(e.Guaranteed.Capture) == "retained" || effect(e.Guaranteed.Capture) == "unchanged", "unpublish(retain) released the capture")
		check(c, src.Snapshot().State == "active", fmt.Sprintf("source is '%s' after unpublish(retain)", src.Snapshot().State))
		check(c, effect(e.Guaranteed.Publication) == "removed", fmt.Sprintf("unpublish reported publication '%s'", effect(e.Observed.Publication)))
	}
}
func test6(ctx context.Context, c *harness.TestContext) {
	target := value(c.Conference(ctx))
	remote := value(c.Remote(ctx, target, "sender"))
	s := value(c.Joined(ctx, harness.JoinedOptions{Target: &target})).Session
	must(remote.Publish(ctx, []string{"audio"}))
	find := func() *openrtc.TrackPublication {
		for _, p := range s.Observation().Snapshot().Publications {
			if p.Source.Status != "known" {
				return &p
			}
		}
		return nil
	}
	must(c.WaitFor(ctx, func() bool { return find() != nil }, "remote publication in the snapshot", 0))
	p := find()
	if p == nil {
		must(c.Unknown("remote publication vanished"))
	}
	ctrl := openrtc.SessionExtension(s, openrtc.ManualSubscriptionKey)
	sub := within(ctx, c, "subscribe", func(ctx context.Context) (openrtc.TrackSubscription, error) {
		return ctrl.Subscribe(ctx, p.ID, []openrtc.MediaKind{"audio"}).Result(ctx)
	})
	check(c, sub.State == "subscribed" && slices.Contains(sub.Kinds, openrtc.MediaKind("audio")), "subscribe did not establish requested selection (RFC §11.2:654–665)")
	check(c, sub.Authority == "client", fmt.Sprintf("authority '%s' for a manual subscription", sub.Authority))
	check(c, sub.Source.Kind == "publication" && sub.Source.Publication == p.ID, "subscription names another publication")
	e := mediaEffects(ctx, c, s.Inspection(), "unsubscribe", ctrl.Unsubscribe(ctx, sub.ID))
	check(c, effect(e.Observed.Receive) != "selected", fmt.Sprintf("unsubscribe reported receive '%s'", effect(e.Observed.Receive)))
	if effect(e.Observed.Receive) == "unknown" {
		c.Partial("unsubscribe reported receive 'unknown'; cessation not observed")
	}
	found := false
	for _, v := range s.Observation().Snapshot().Publications {
		if v.ID == p.ID {
			found = true
			break
		}
	}
	check(c, found, "unsubscribing removed the remote publication (MED-103)")
}
func test7(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	stats := openrtc.SessionExtension(s, openrtc.StatisticsKey)
	sample := within(ctx, c, "sample", func(ctx context.Context) (openrtc.StatsSample, error) {
		return stats.Sample(ctx, openrtc.StatisticsTarget{Kind: "session"}).Result(ctx)
	})
	check(c, sample.Target.Kind == "session", "sample target differs from the request")
	check(c, slices.Contains([]string{"complete", "partial", "unknown"}, sample.Coverage), "invalid coverage")
	// RFC §11.10:999 permits explicitly unknown collection time.
	if sample.CollectedAt != "" {
		at, e := parseObservedAt(string(sample.CollectedAt))
		check(c, e == nil, "known collection time invalid")
		check(c, !at.After(time.Now().Add(5*time.Second)), "collection time in future")
	}
	check(c, sample.Source != "", "statistics omit measurement source/scope")
	for _, m := range sample.Measurements {
		if m.Value.Status == "known" && m.Value.Value != nil {
			v := *m.Value.Value
			check(c, !math.IsNaN(v) && !math.IsInf(v, 0), fmt.Sprintf("%s: non-finite value", m.Metric))
		}
	}
}

// Date.parse accepts ISO dates and date-times without a zone as well as RFC 3339.
// An unzoned date is UTC; an unzoned date-time is in the local time zone.
func parseObservedAt(s string) (time.Time, error) {
	if at, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return at, nil
	}
	for _, layout := range []string{"2006-01-02", "2006-01", "2006", "2006-01-02T15:04Z07:00", "2006-01-02T15:04:05.999999999-0700", "2006-01-02T15:04-0700", time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC} {
		if at, err := time.Parse(layout, s); err == nil {
			return at, nil
		}
	}
	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04"} {
		if at, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return at, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid instant %q", s)
}
func test8(ctx context.Context, c *harness.TestContext) {
	client := value(c.Client(ctx, harness.ClientOptions{}))
	before := c.NativeCounts()
	devices := within(ctx, c, "enumerate", func(arg0 context.Context) (openrtc.DeviceInventory, error) {
		return openrtc.ClientExtension(client, openrtc.DevicesKey).Enumerate(arg0).Result(arg0)
	})
	ids := map[openrtc.DeviceID]bool{}
	for _, d := range devices.Entries {
		check(c, !ids[d.ID], fmt.Sprintf("duplicate device id %s", d.ID))
		ids[d.ID] = true
	}
	after := c.NativeCounts()
	if before != nil && after != nil {
		check(c, after.Captures == before.Captures, "enumeration captured media (MED-118)")
	}
}
func test9(ctx context.Context, c *harness.TestContext) {
	s := joined(ctx, c)
	rec := openrtc.SessionExtension(s, openrtc.RecordingKey)
	job := within(ctx, c, "start", func(ctx context.Context) (openrtc.MediaJob, error) {
		return rec.Start(ctx, openrtc.RecordingRequest{Target: instanceID(c, s)}).Result(ctx)
	})
	check(c, len(job.Snapshot().ID) > 0, "job has no id")
	check(c, job.Snapshot().Status == "running", fmt.Sprintf("a freshly started job reports '%s' (EXT-108)", job.Snapshot().Status))
	again := within(ctx, c, "get", func(ctx context.Context) (openrtc.MediaJob, error) {
		return rec.Get(ctx, job.Snapshot().ID).Result(ctx)
	})
	check(c, again.Snapshot().ID == job.Snapshot().ID, "get returned another job")
	out := within(ctx, c, "stop", func(arg0 context.Context) (openrtc.JobStopReceipt, error) { return job.Stop(arg0).Result(arg0) })
	check(c, slices.Contains([]string{"accepted", "confirmed", "failed", "unknown"}, out.Status), "invalid stop outcome")
}
func test10(ctx context.Context, c *harness.TestContext) {
	ctrl := value(c.RequireControl("authority")).(func() harness.AuthorityControl)
	s := joined(ctx, c)
	rec := openrtc.SessionExtension(s, openrtc.RecordingKey)
	authority := ctrl()
	must(c.Within(ctx, "revoke", 0, func(ctx context.Context) error { return authority.Revoke(ctx, "recording.server") }))
	c.Track("restore authority", func(ctx context.Context) error { return authority.Restore(ctx, "recording.server") })
	_, e := c.Rejects(ctx, func(ctx context.Context) (any, error) {
		return rec.Start(ctx, openrtc.RecordingRequest{Target: instanceID(c, s)}).Result(ctx)
	}, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"authorization-denied"}}, "recording without authority")
	must(e)
}

func effect(v openrtc.Known[string]) string {
	if v.Status == "known" && v.Value != nil {
		return *v.Value
	}
	return "unknown"
}

func mediaEffects(ctx context.Context, c *harness.TestContext, inspection openrtc.SessionInspection, label string, op openrtc.Async[openrtc.Void]) openrtc.MediaOperationEffects {
	within(ctx, c, label, op.Result)
	record := value(inspection.Operation(op.OperationID()))
	if record.Status != "retained" || record.Details == nil || record.Details.Effects.Status != "known" || record.Details.Effects.Value == nil {
		must(c.Unknown(label + ": operation effects unavailable"))
	}
	return *record.Details.Effects.Value
}

func cleanupAfter(ctx context.Context, inspection openrtc.InspectionCommon, operation openrtc.Async[openrtc.Void]) (openrtc.CleanupReport, error) {
	_, err := operation.Result(ctx)
	return inspection.Cleanup(), err
}
