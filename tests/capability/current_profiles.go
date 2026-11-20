package capability

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

func currentProfileTest(id, title, profile, capability string, refs []string, body func(context.Context, *harness.TestContext)) harness.ConformanceTest {
	operation := ""
	switch {
	case strings.HasPrefix(id, "CT-MSG-"):
		operation = "sendMessage"
		if id == "CT-MSG-002" {
			operation = "observeMessages"
		}
	case id == "CT-DEVICE-001":
		operation = "listDevices"
	case id == "CT-DEVICE-002":
		operation = "selectDevice"
	case id == "CT-DEVICE-003":
		operation = "getStatistics"
	case strings.HasPrefix(id, "CT-RENEW-"):
		operation = "renewCredential"
	case id == "CT-ADMIN-001":
		operation = "createConference"
	case id == "CT-ADMIN-002":
		operation = "endConference"
	case id == "CT-ADMIN-003":
		operation = "removeParticipant"
	case id == "CT-ADMIN-004":
		operation = "mintCredential"
	case id == "CT-PLC-PROFILE-001" || id == "CT-COMB-001":
		operation = "assessPlacement"
	case strings.HasPrefix(id, "CT-PLC-PROFILE-"):
		operation = "applyPlacement"
	}
	label := harness.ProfileName(profile)
	mandatory := profile
	if strings.HasPrefix(id, "CT-VERSION-") {
		label = harness.Common
		mandatory = ""
	}
	return harness.DefineTest(harness.ConformanceTest{ID: id, Title: title, Suite: harness.Capability, Profile: label, Requirements: refs, Observable: title, Capability: &harness.CapabilityDependence{ID: capability, Operation: operation, MandatoryProfile: mandatory}, Run: run(body)})
}
func messages(ctx context.Context, c *harness.TestContext) harness.MessageScenario {
	t, ok := c.Target.(harness.MessageScenarioTarget)
	if !ok {
		must(c.NotRun("missing actual sendMessage/observeMessages mapping and controlled peer"))
	}
	f := value(t.MessageScenario(ctx))
	check(c, f.Documentation != "" && f.Send != nil && f.Observe != nil, "invalid messaging public-operation mapping")
	return f
}
func devices(ctx context.Context, c *harness.TestContext) harness.DeviceScenario {
	t, ok := c.Target.(harness.DeviceScenarioTarget)
	if !ok {
		must(c.NotRun("missing device/statistics public-operation fixture"))
	}
	f := value(t.DeviceScenario(ctx))
	check(c, f.Documentation != "", "device mapping has no public scope documentation")
	return f
}
func renewal(ctx context.Context, c *harness.TestContext) harness.RenewalScenario {
	t, ok := c.Target.(harness.RenewalScenarioTarget)
	if !ok {
		must(c.NotRun("missing renewCredential public-operation mapping"))
	}
	f := value(t.RenewalScenario(ctx))
	check(c, f.Documentation != "" && f.Renew != nil, "invalid renewal operation mapping")
	return f
}
func administration(ctx context.Context, c *harness.TestContext) harness.AdministrationScenario {
	t, ok := c.Target.(harness.AdministrationScenarioTarget)
	if !ok {
		must(c.NotRun("missing actual trusted-server administration operation mapping"))
	}
	f := value(t.AdministrationScenario(ctx))
	check(c, f.Documentation != "" && f.Server != "", "administration mapping lacks actual server authority scope")
	return f
}
func requireCheckpoint(ctx context.Context, c *harness.TestContext, f func(context.Context) error) {
	if f == nil {
		must(c.NotRun("independent observation checkpoint unavailable"))
	}
	must(f(ctx))
}
func knownString(c *harness.TestContext, v openrtc.Known[string], expected, label string) {
	if v.Status != "known" || v.Value == nil {
		must(c.NotRun(label + ": independent observation unknown"))
	}
	check(c, *v.Value == expected, label+": established value violates postcondition")
}

var CurrentProfileTests = []harness.ConformanceTest{
	currentProfileTest("CT-MSG-001", "send each claimed recipient mode positively without inventing delivery", "data-messaging", "messaging.ephemeral", []string{"RFC §7.0:55/65", "RFC §11.5:729–779", "RFC §14.3:1133"}, func(ctx context.Context, c *harness.TestContext) {
		f := messages(ctx, c)
		check(c, len(f.Modes) > 0, "claimed messaging profile has no positive recipient scope")
		for index, mode := range f.Modes {
			check(c, mode.Kind == "direct" || mode.Kind == "broadcast", "undefined recipient scope")
			if mode.Kind == "direct" {
				check(c, len(mode.IDs) > 0, "direct recipient set empty")
			} else {
				check(c, mode.Scope != "" && mode.MembershipBoundary != "", "broadcast scope lacks membership boundary")
			}
			payload := []byte{byte(index + 1)}
			r := value(f.Send(ctx, mode, payload))
			check(c, slices.Contains([]string{"submitted", "delivered", "unknown"}, r.State), "valid authorized positive send failed or returned invalid state")
			if r.State == "unknown" {
				must(c.NotRun("positive send submission remains unknown; cannot establish defining operation positive"))
			}
			check(c, reflect.DeepEqual(r.Recipients, mode), "send changed exact recipient scope")
			if r.State == "delivered" {
				if f.RecipientPayloads == nil {
					must(c.NotRun("claimed delivered has no independent delivery evidence"))
				}
				found := false
				for _, p := range f.RecipientPayloads() {
					found = found || bytes.Equal(p, payload)
				}
				check(c, found, "delivery claim has no recipient evidence")
			}
		}
	}),
	currentProfileTest("CT-MSG-002", "incoming messages disclose sender/scope and independent removal", "data-messaging", "messaging.ephemeral", []string{"RFC §11.5:758–777", "RFC §8.3:469–477"}, func(ctx context.Context, c *harness.TestContext) {
		f := messages(ctx, c)
		if f.PeerSend == nil {
			must(c.NotRun("controlled peer sender unavailable"))
		}
		check(c, len(f.Modes) > 0, "no supported recipient scope")
		events := make(chan harness.MessageEvent, 8)
		stop := value(f.Observe(func(e harness.MessageEvent) {
			select {
			case events <- e:
			default:
			}
		}))
		check(c, stop != nil, "observeMessages returned no removal")
		payload := []byte("inbound-identity")
		must(f.PeerSend(ctx, f.Modes[0], payload))
		var event harness.MessageEvent
		select {
		case event = <-events:
		case <-ctx.Done():
			must(c.NotRun("incoming message not observed within budget"))
		}
		check(c, event.Kind == "message" && bytes.Equal(event.Payload, payload), "observed message content missing or changed")
		check(c, event.Sender.Status == "known" || event.Sender.Status == "unknown", "sender identity omitted or invalid")
		if event.Sender.Status == "known" {
			check(c, event.Sender.Value != nil && *event.Sender.Value != "", "known sender lacks identity")
		}
		check(c, reflect.DeepEqual(event.Recipients, f.Modes[0]), "incoming recipient scope differs from actual scope")
		if f.LoseDelivery == nil {
			must(c.NotRun("message loss stimulus unavailable"))
		}
		must(f.LoseDelivery(ctx))
		select {
		case loss := <-events:
			check(c, loss.Kind == "loss", "message delivery loss hidden or misrepresented")
		case <-ctx.Done():
			must(c.Fail("controlled local message loss not observed"))
		}
		must(stop())
		must(stop())
		must(f.PeerSend(ctx, f.Modes[0], []byte("after-removal")))
		requireCheckpoint(ctx, c, f.Checkpoint)
		select {
		case <-events:
			must(c.Fail("removed message observer received later covered stimulus"))
		default:
		}
	}),
	currentProfileTest("CT-MSG-003", "reject payload-octet overflow before any submission without truncation", "data-messaging", "messaging.ephemeral", []string{"RFC §11.5:741–747", "RFC §9:529–570"}, func(ctx context.Context, c *harness.TestContext) {
		f := messages(ctx, c)
		if f.MaximumPayloadOctets.Status != "known" || f.MaximumPayloadOctets.Value == nil || f.SubmissionCount == nil {
			must(c.NotRun("message limit or independent submission boundary unknown"))
		}
		check(c, *f.MaximumPayloadOctets.Value > 0 && *f.MaximumPayloadOctets.Value < 1<<24, "invalid fixture payload limit")
		check(c, len(f.Modes) > 0, "no supported message scope")
		before := f.SubmissionCount()
		_, err := f.Send(ctx, f.Modes[0], bytes.Repeat([]byte{0xc3}, *f.MaximumPayloadOctets.Value+1))
		_, e := c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"invalid-argument"}}, "payload octet overflow")
		must(e)
		check(c, f.SubmissionCount() == before, "oversized payload sent or truncated")
	}),
	currentProfileTest("CT-MSG-004", "offline and unknown sends are not replayed and payloads stay transient", "data-messaging", "messaging.ephemeral", []string{"RFC §11.5:754/777", "RFC §9:570", "RFC §14.3:1133"}, func(ctx context.Context, c *harness.TestContext) {
		f := messages(ctx, c)
		if f.SetOnline == nil || f.RecipientPayloads == nil || f.SubmissionCount == nil || f.NextSubmissionUnknown == nil || f.DefaultSurfaces == nil {
			must(c.NotRun("offline, unknown-submission or default-surface evidence unavailable"))
		}
		check(c, len(f.Modes) > 0, "no supported message scope")
		payload := []byte("transient-secret-message-5af3")
		must(f.SetOnline(ctx, false))
		_, _ = f.Send(ctx, f.Modes[0], payload)
		must(f.SetOnline(ctx, true))
		requireCheckpoint(ctx, c, f.Checkpoint)
		for _, p := range f.RecipientPayloads() {
			check(c, !bytes.Equal(p, payload), "queued offline payload replayed on reconnect")
		}
		f.NextSubmissionUnknown()
		before := f.SubmissionCount()
		result, err := f.Send(ctx, f.Modes[0], []byte("unknown-effect"))
		if err == nil {
			check(c, result.State == "unknown", "unknown submission promoted to established success")
		} else {
			failure, e := c.ExpectError(err, harness.ExpectedFailure{}, "unknown submission")
			must(e)
			check(c, failure.Outcome == "unknown", "unknown effect converted to none")
		}
		requireCheckpoint(ctx, c, f.Checkpoint)
		check(c, f.SubmissionCount() == before+1, "automatic duplication-prone retry after unknown submission")
		surfaces := f.DefaultSurfaces()
		if len(surfaces) == 0 {
			must(c.NotRun("no observed default surfaces; cannot prove payload privacy"))
		}
		for _, surface := range surfaces {
			check(c, harness.FindLeak(surface, []string{string(payload), "unknown-effect"}) == nil, "payload retained in snapshot/diagnostics/default logs")
		}
	}),
	currentProfileTest("CT-DEVICE-001", "listDevices reports scoped permission-qualified completeness without capture", "devices-and-statistics", "devices.enumerate", []string{"RFC §11.10:963–975/1005", "RFC §7.0:61/65"}, func(ctx context.Context, c *harness.TestContext) {
		f := devices(ctx, c)
		if f.Manager == nil || f.Effects == nil {
			must(c.NotRun("device list or acquisition evidence missing"))
		}
		before := f.Effects.Counts()
		r := value(f.Manager.Enumerate(ctx).Result(ctx))
		snap := f.Manager.Snapshot()
		check(c, slices.Contains([]string{"complete", "incomplete", "unknown", "partial"}, r.Coverage), "device list omits completeness")
		permission, ok := snap.Permissions[f.Kind]
		check(c, ok && (permission.Status == "known" || permission.Status == "unknown"), "permission-dependent state omitted")
		ids := map[openrtc.DeviceID]bool{}
		for _, d := range r.Entries {
			check(c, d.ID != "" && !ids[d.ID] && d.Kind == f.Kind, "device id missing, duplicated or wrong kind")
			ids[d.ID] = true
		}
		check(c, snap.Freshness.Owner != "", "device identifier owner scope omitted")
		after := f.Effects.Counts()
		check(c, after.Captures == before.Captures && after.Publications == before.Publications, "listDevices acquired or published media")
	}),
	currentProfileTest("CT-DEVICE-002", "selectDevice executes the authorized scoped selection without acquisition", "devices-and-statistics", "devices.select", []string{"RFC §11.10:977–989", "RFC §7.1:86–88"}, func(ctx context.Context, c *harness.TestContext) {
		f := devices(ctx, c)
		if f.Manager == nil || f.Effects == nil {
			must(c.NotRun("device selection or independent acquisition boundary missing"))
		}
		check(c, f.Selection.Device != "", "fixture has no eligible authorized device selection")
		before := f.Effects.Counts()
		value(f.Manager.Select(ctx, f.Selection).Result(ctx))
		snap := f.Manager.Snapshot()
		found := false
		for _, s := range snap.Selections {
			found = found || reflect.DeepEqual(s, f.Selection)
		}
		check(c, found, "successful selectDevice did not establish exact selected device/target")
		after := f.Effects.Counts()
		check(c, after.Captures == before.Captures && after.Publications == before.Publications, "isolated device selection silently captured or published")
	}),
	currentProfileTest("CT-DEVICE-003", "statistics qualify scope interval unit and preserve cached collection time", "devices-and-statistics", "media.statistics", []string{"RFC §11.10:995–1003", "RFC §4:23"}, func(ctx context.Context, c *harness.TestContext) {
		f := devices(ctx, c)
		if f.Statistics == nil {
			must(c.NotRun("actual statistics operation unavailable"))
		}
		first := value(f.Statistics.Sample(ctx, f.StatisticsTarget).Result(ctx))
		check(c, reflect.DeepEqual(first.Target, f.StatisticsTarget) && first.Coverage != "", "statistics target/coverage missing")
		check(c, len(first.Measurements) > 0, "fixture promised a measurement but returned none")
		for _, m := range first.Measurements {
			check(c, m.Metric != "" && reflect.DeepEqual(m.Target, f.StatisticsTarget), "measurement name/scope missing")
			check(c, m.Value.Status == "known" && m.Value.Value != nil || m.Value.Status == "unknown", "measurement omitted or coerced to zero")
			check(c, m.Interval.Status == "known" && m.Interval.Value != nil && m.Interval.Value.Milliseconds >= 0, "successful statistics omitted the required measurement interval")
			check(c, m.Unit != "", "measurement unit missing")
		}
		if f.CachedSample == nil {
			must(c.NotRun("cached-sample collection time evidence missing"))
		}
		second := value(f.CachedSample(ctx))
		check(c, second.CollectedAt == first.CollectedAt, "cached statistics collection time rewritten")
		c.Note("empty collectedAt represents documented unknown; no universal metric sign assumption")
	}),
	currentProfileTest("CT-RENEW-001", "renewCredential applies only after applicable acknowledgment", "credential-renewal", "credential.renewal", []string{"RFC §11.11:1009–1036", "RFC §7.0:62/65"}, func(ctx context.Context, c *harness.TestContext) {
		f := renewal(ctx, c)
		r := value(f.Renew(ctx, f.First))
		check(c, r.State == "applied" && r.Acknowledged && r.Boundary != "", "valid supported renewal did not establish applicable acknowledgment")
		if f.Installed == nil {
			must(c.NotRun("independent renewal installation evidence unavailable"))
		}
		knownString(c, f.Installed(), f.First, "current acknowledged replacement")
	}),
	currentProfileTest("CT-RENEW-002", "known credential mismatch fails before submission and failure alone retains participation", "credential-renewal", "credential.renewal", []string{"RFC §11.11:1015/1024–1036", "RFC §6:33"}, func(ctx context.Context, c *harness.TestContext) {
		f := renewal(ctx, c)
		if f.SubmissionCount == nil || f.PublicConnected == nil || f.FailNext == nil {
			must(c.NotRun("renewal mismatch/submission/connection evidence unavailable"))
		}
		before := f.SubmissionCount()
		_, err := f.Renew(ctx, f.Mismatched)
		_, e := c.ExpectError(err, harness.ExpectedFailure{}, "known replacement scope mismatch")
		must(e)
		check(c, f.SubmissionCount() == before, "known mismatched credential submitted")
		f.FailNext()
		_, err = f.Renew(ctx, f.First)
		_, e = c.ExpectError(err, harness.ExpectedFailure{}, "renewal failure")
		must(e)
		connected := f.PublicConnected()
		if connected.Status != "known" || connected.Value == nil {
			must(c.NotRun("post-renewal participation evidence unknown"))
		}
		check(c, *connected.Value, "renewal failure invented disconnection")
	}),
	currentProfileTest("CT-RENEW-003", "a late overtaken renewal cannot replace the acknowledged newer credential", "credential-renewal", "credential.renewal", []string{"RFC §11.11:1020/1036", "RFC §14.3:1142"}, func(ctx context.Context, c *harness.TestContext) {
		f := renewal(ctx, c)
		if f.HoldNext == nil || f.Installed == nil {
			must(c.NotRun("controlled renewal completion ordering unavailable"))
		}
		g := f.HoldNext()
		defer g.ReleaseSuccess()
		done := make(chan error, 1)
		go func() { _, e := f.Renew(ctx, f.First); done <- e }()
		select {
		case <-g.Reached():
		case <-ctx.Done():
			must(c.NotRun("renewal checkpoint unavailable"))
		}
		r := value(f.Renew(ctx, f.Second))
		check(c, r.State == "applied" && r.Acknowledged, "newer credential not acknowledged")
		knownString(c, f.Installed(), f.Second, "newer installed credential")
		g.ReleaseSuccess()
		select {
		case <-done:
		case <-ctx.Done():
			must(c.NotRun("late renewal did not settle within budget"))
		}
		requireCheckpoint(ctx, c, f.Checkpoint)
		knownString(c, f.Installed(), f.Second, "late completion overwrote newer credential")
	}),
	currentProfileTest("CT-RENEW-004", "unacknowledged deferred or unknown replacement cannot claim applied", "credential-renewal", "credential.renewal", []string{"RFC §11.11:1017–1036", "RFC §14.2:1102–1107"}, func(ctx context.Context, c *harness.TestContext) {
		f := renewal(ctx, c)
		if f.SetAcknowledgementState == nil || f.Installed == nil {
			must(c.NotRun("controlled unacknowledged-renewal result evidence unavailable"))
		}
		before := f.Installed()
		if before.Status != "known" || before.Value == nil {
			must(c.NotRun("prior installed replacement version unknown"))
		}
		for _, state := range []string{"deferred", "unknown"} {
			f.SetAcknowledgementState(state)
			r := value(f.Renew(ctx, f.First))
			check(c, !r.Acknowledged && slices.Contains([]string{"deferred", "unknown"}, r.State), "unacknowledged replacement falsely reported applied")
			if r.State == "deferred" {
				check(c, r.Boundary != "", "deferred replacement omits pending boundary")
			}
			installed := f.Installed()
			if installed.Status != "known" || installed.Value == nil {
				c.Partial("unacknowledged replacement installation evidence unknown")
			} else {
				check(c, *installed.Value == *before.Value, "replacement installed before applicable acknowledgment")
			}
		}
	}),
	currentProfileTest("CT-ADMIN-001", "createConference establishes active server-owned conference", "server-administration", "administration.conference", []string{"RFC §11.9:913–923", "RFC §7.0:60/65"}, func(ctx context.Context, c *harness.TestContext) {
		f := administration(ctx, c)
		if f.Create == nil || f.Conference == nil {
			must(c.NotRun("conference provisioning or independent state evidence missing"))
		}
		r := value(f.Create(ctx, f.Server))
		check(c, r.ID != "" && r.Owner == f.Server && r.State == "active", "createConference did not establish active server-owned incarnation")
		knownString(c, f.Conference(r.ID), "active", "provisioned conference")
	}),
	currentProfileTest("CT-ADMIN-002", "endConference targets one incarnation and repeated end remains safe", "server-administration", "administration.conference", []string{"RFC §11.9:925–935/955–960", "RFC §8.6:506–510"}, func(ctx context.Context, c *harness.TestContext) {
		f := administration(ctx, c)
		if f.Create == nil || f.End == nil || f.Conference == nil {
			must(c.NotRun("endConference or independent scope observation unavailable"))
		}
		a := value(f.Create(ctx, f.Server))
		b := value(f.Create(ctx, f.Server))
		check(c, a.ID != b.ID, "distinct conference acquisitions reused one identity")
		for i := 0; i < 2; i++ {
			r := value(f.End(ctx, f.Server, a.ID))
			check(c, r.Scope == a.ID, "endConference affected wrong conference")
			check(c, r.RemoteState.Status == "known" && r.RemoteState.Value != nil || r.RemoteState.Status == "unknown", "remote end state omitted")
			if r.RemoteState.Status == "known" {
				knownString(c, f.Conference(a.ID), *r.RemoteState.Value, "remote end evidence")
				if *r.RemoteState.Value != "ended" {
					c.Partial("end accepted locally; remote conference termination is not established")
				}
			} else {
				observed := f.Conference(a.ID)
				if observed.Status != "known" || observed.Value == nil || *observed.Value != "ended" {
					c.Partial("local end acceptance does not establish remote termination")
				}
			}
		}
		knownString(c, f.Conference(b.ID), "active", "separately owned conference survived")
	}),
	currentProfileTest("CT-ADMIN-003", "removeParticipant removes selected presence without claiming another effect", "server-administration", "administration.conference", []string{"RFC §11.9:937–947", "RFC §6:35"}, func(ctx context.Context, c *harness.TestContext) {
		f := administration(ctx, c)
		if f.Create == nil || f.Populate == nil || f.Remove == nil || f.Participant == nil {
			must(c.NotRun("participant administration fixture unavailable"))
		}
		a := value(f.Create(ctx, f.Server))
		p := value(f.Populate(ctx, a.ID))
		q := value(f.Populate(ctx, a.ID))
		check(c, p != "" && q != "" && p != q, "fixture lacks distinct current presences")
		r := value(f.Remove(ctx, f.Server, a.ID, p))
		check(c, r.Scope == p, "removeParticipant returned wrong presence effect")
		knownString(c, f.Participant(p), "departed", "selected presence removal")
		knownString(c, f.Participant(q), "present", "other presence survived")
	}),
	currentProfileTest("CT-ADMIN-004", "mintCredential uses authorized secret channel and client credentials do not escalate", "server-administration", "administration.conference", []string{"RFC §11.9:949–960", "RFC §12.1:1044", "RFC §6:33"}, func(ctx context.Context, c *harness.TestContext) {
		f := administration(ctx, c)
		if f.Mint == nil || f.Create == nil || f.DefaultSurfaces == nil {
			must(c.NotRun("credential issuance/secret-channel observation unavailable"))
		}
		a := value(f.Create(ctx, f.Server))
		request := harness.CredentialFacts{Audience: "test-provider", Subject: "authorized-recipient", Destination: a.ID, Purpose: "join"}
		r := value(f.Mint(ctx, f.Server, request))
		check(c, r.ID != "" && r.AuthorizedChannel && r.Audience == request.Audience && r.Subject == request.Subject && r.Destination == request.Destination && r.Purpose == request.Purpose, "mintCredential lost authorized channel or requested claims")
		_, err := f.Create(ctx, "client-credential:"+r.ID)
		_, e := c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"permission-denied"}}, "client credential administration")
		must(e)
		surfaces := f.DefaultSurfaces()
		if len(surfaces) == 0 {
			must(c.NotRun("no observed default surfaces; cannot prove payload privacy"))
		}
		for _, surface := range surfaces {
			check(c, harness.FindLeak(surface, []string{r.ID}) == nil, "issued credential leaked outside authorized recipient")
		}
	}),
	currentProfileTest("CT-VERSION-001", "incompatible profile version and foreign namespace reject before effects", "versioned-operation", "profile.versioned-operation", []string{"RFC §8.8:525", "RFC §10:574–580", "RFC §12.4:1072", "RFC §14.3:1138"}, func(ctx context.Context, c *harness.TestContext) {
		t, ok := c.Target.(harness.VersionScenarioTarget)
		if !ok {
			must(c.NotRun("actual versioned defining operation mapping unavailable"))
		}
		f := value(t.VersionScenario(ctx))
		if f.Execute == nil || f.Effects == nil {
			must(c.NotRun("version request effect boundary unavailable"))
		}
		before := f.Effects.Counts()
		for _, request := range []struct {
			version   int
			namespace string
		}{{f.SupportedVersion + 100, f.Namespace}, {f.SupportedVersion, "foreign.example/unsupported"}} {
			_, err := f.Execute(ctx, request.version, request.namespace)
			_, e := c.ExpectError(err, harness.ExpectedFailure{Codes: []openrtc.ErrorCode{"invalid-argument", "unsupported"}}, "version/namespace rejection")
			must(e)
			check(c, f.Effects.Counts() == before, "incompatible version or foreign namespace caused effects")
		}
	}),
	currentProfileTest("CT-VERSION-002", "supported version executes the defining operation positively", "versioned-operation", "profile.versioned-operation", []string{"RFC §7.0:65", "RFC §8.8:525", "RFC §10:574–580", "RFC §14.3:1138"}, func(ctx context.Context, c *harness.TestContext) {
		t, ok := c.Target.(harness.VersionScenarioTarget)
		if !ok {
			must(c.NotRun("actual versioned operation mapping unavailable"))
		}
		f := value(t.VersionScenario(ctx))
		check(c, f.Documentation != "" && f.Execute != nil && f.Profile != "" && f.Namespace != "", "version mapping lacks actual defining operation")
		r := value(f.Execute(ctx, f.SupportedVersion, f.Namespace))
		check(c, r.Operation != "" && r.Resource != "" && r.State == "active", "supported profile version did not execute defining positive operation")
		check(c, !strings.Contains(r.Operation, "accessor"), "accessor acquisition cannot establish profile operation success")
	}),
}

func init() { CapabilityTests = append(CapabilityTests, CurrentProfileTests...) }
