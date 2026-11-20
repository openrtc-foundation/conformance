package capability

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

type profileObservationLog struct {
	mu      sync.Mutex
	updates []harness.ProfileObservationUpdate
}

func (l *profileObservationLog) add(u harness.ProfileObservationUpdate) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.updates = append(l.updates, u)
}
func (l *profileObservationLog) read() []harness.ProfileObservationUpdate {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]harness.ProfileObservationUpdate(nil), l.updates...)
}
func profileMarkers(us []harness.ProfileObservationUpdate) []string {
	out := []string{}
	for _, u := range us {
		if u.Marker != "" {
			out = append(out, u.Marker)
		}
	}
	return out
}
func profileMarkerCount(us []harness.ProfileObservationUpdate, marker string) int {
	n := 0
	for _, u := range us {
		if u.Marker == marker {
			n++
		}
		if u.Snapshot != nil && u.Snapshot.Facts["marker"] == marker {
			n++
		}
	}
	return n
}
func profileStreamProfiles(c *harness.TestContext) []string {
	claim, ok := c.Target.(harness.ClaimTarget)
	if !ok {
		must(c.NotRun("profile applicability claim declaration unavailable"))
	}
	profiles := []string{}
	add := func(name string) {
		if name != "" && !slices.Contains(profiles, name) {
			profiles = append(profiles, name)
		}
	}
	for _, p := range claim.ClaimDeclaration().Profiles {
		add(p.Name)
	}
	// Advertised defining operations carry their profile obligations even when
	// the claim omits a redundant profile label (§14.1:1092).
	for _, o := range claim.ClaimDeclaration().Operations {
		if o.Support != "supported" && o.Support != "conditional" {
			if o.Support == "unknown" {
				c.Partial("profile observation applicability unresolved for " + o.Operation)
			}
			continue
		}
		switch o.Operation {
		case "dial", "accept", "reject", "hangup", "setHold", "sendDigits", "startReceiving", "stopReceiving", "transfer", "captureCall", "publishCall":
			add("calling")
		case "subscribe", "unsubscribeMedia", "unsubscribeSubscription":
			add("manual-subscription")
		case "shareSource", "revokeSourceGrant", "replaceSource":
			add("source-replacement-and-sharing")
		case "captureScreen":
			add("screen-sharing")
		case "sendMessage", "observeMessages":
			add("data-messaging")
		case "startRecording", "stopRecording", "getRecording", "startIndependentRecording":
			add("recording")
		case "assessPlacement", "applyPlacement", "observePlacement":
			add("placement")
		case "configureEncryption", "observeEncryption":
			add("end-to-end-encryption")
		case "createConference", "endConference", "removeParticipant", "mintCredential":
			add("server-administration")
		case "listDevices", "selectDevice", "getStatistics":
			add("devices-and-statistics")
		case "renewCredential":
			add("credential-renewal")
		}
	}
	return profiles
}
func profileInitial(c *harness.TestContext, f harness.ProfileObservationFixture, us []harness.ProfileObservationUpdate) {
	check(c, len(us) > 0 && us[0].Snapshot != nil, "profile observe omitted initial covered state")
	initial := us[0].Snapshot
	check(c, initial.Scope == f.Scope && slices.Contains([]string{"complete", "incomplete", "unknown"}, initial.Completeness), "profile initial state omitted explicit scope/completeness")
	if initial.Completeness != "complete" {
		c.Partial("profile initial coverage remains " + initial.Completeness + ": " + f.Profile)
	}
}
func relatedProfileFacts(c *harness.TestContext, updates []harness.ProfileObservationUpdate) {
	for _, u := range updates {
		if u.Marker != "" {
			check(c, u.Related["related"] == u.Marker+"/related", "one indivisible profile observation lost its related established fact")
		}
	}
}
func profileObserve(ctx context.Context, c *harness.TestContext, f harness.ProfileObservationFixture, log *profileObservationLog) openrtc.Unsubscribe {
	return value(f.Observe(ctx, log.add))
}

var ProfileObservationTests = []harness.ConformanceTest{
	harness.DefineTest(harness.ConformanceTest{ID: "CT-PROFILE-OBS-001", Title: "each claimed profile preserves initial coverage, ordered changes, loss/resnapshot and independent removal", Suite: harness.Capability, Profile: harness.Common, Requirements: []string{"RFC §7.0:63", "RFC §8.3:467–477", "RFC §14.1:1088–1098", "RFC A.3:1335–1339/1425"}, Observable: "actual public profile observation projection; no required event names, sequence fields or internal machinery", Run: run(func(ctx context.Context, c *harness.TestContext) {
		profiles := profileStreamProfiles(c)
		if len(profiles) == 0 {
			must(c.Exclude("no profile observation behavior in the scoped claim", "explicit claim profiles and supported defining operations contain no profile stream"))
		}
		target, ok := c.Target.(harness.ProfileObservationTarget)
		if !ok {
			must(c.NotRun("actual profile event/snapshot/removal and controlled loss mappings unavailable"))
		}
		for _, profile := range profiles {
			f, err := target.ProfileObservation(ctx, profile)
			if err != nil {
				c.Partial("profile observation mapping missing for " + profile + ": " + err.Error())
				continue
			}
			if f.Profile != profile || f.Scope == "" || f.PublicRepresentation == "" || f.Documentation == "" || f.Capability == nil || f.Observe == nil || f.Snapshot == nil || f.Commit == nil || f.LoseNext == nil {
				c.Partial("profile observation mapping incomplete for " + profile)
				continue
			}
			scoped := value(f.Capability(ctx))
			if scoped.Scope.Resource != f.Scope || scoped.Scope.Profile != profile {
				c.Partial("public profile capability does not establish exact requested scope: " + profile)
				continue
			}
			switch scoped.Support.Level {
			case "unsupported", "not-applicable":
				check(c, false, "claimed profile's required scoped observation is unsupported: "+profile)
			case "supported":
			default:
				c.Partial("scoped profile observation applicability unresolved: " + profile)
				continue
			}
			// A fixture marker represents controlled established public facts. It is
			// never required in a real binding's event shape or diagnostic metadata.
			must(f.Commit(ctx, "baseline"))
			a, b := &profileObservationLog{}, &profileObservationLog{}
			var once sync.Once
			stopA := value(f.Observe(ctx, func(u harness.ProfileObservationUpdate) {
				a.add(u)
				if u.Marker == "first" {
					once.Do(func() { must(f.Commit(ctx, "reentrant")) })
				}
			}))
			defer stopA()
			stopB := profileObserve(ctx, c, f, b)
			defer stopB()
			profileInitial(c, f, a.read())
			profileInitial(c, f, b.read())
			initial := a.read()[0].Snapshot
			saved := map[string]string{}
			for k, v := range initial.Facts {
				saved[k] = v
			}
			must(f.Commit(ctx, "first"))
			must(f.Commit(ctx, "last"))
			want := []string{"first", "reentrant", "last"}
			check(c, reflect.DeepEqual(profileMarkers(a.read()), want) && reflect.DeepEqual(profileMarkers(b.read()), want), "profile subscribers omitted, duplicated or reordered required/reentrant changes: "+profile)
			relatedProfileFacts(c, a.read())
			relatedProfileFacts(c, b.read())
			check(c, reflect.DeepEqual(initial.Facts, saved), "profile initial snapshot aliases later state")
			current := value(f.Snapshot(ctx))
			check(c, current.Scope == f.Scope, "profile snapshot changed coverage scope")
			if f.Stateful && current.Completeness != "complete" {
				c.Partial("current profile state remains " + current.Completeness + ": " + profile)
			}
			if f.Stateful && current.Completeness == "complete" {
				check(c, current.Facts["marker"] == "last" && current.Facts["related"] == "last/related", "profile snapshot omits already delivered established related facts")
			}
			// Mutating a consumer's detached copy must not mutate provider facts.
			if initial.Facts != nil {
				initial.Facts["consumer"] = "mutated"
				check(c, value(f.Snapshot(ctx)).Facts["consumer"] != "mutated", "profile initial state exposes mutable implementation storage")
			}
			must(f.LoseNext(ctx))
			must(f.Commit(ctx, "lost"))
			must(f.Commit(ctx, "after-loss"))
			loss := false
			fresh := false
			for _, u := range b.read() {
				if u.Loss {
					loss = true
					fresh = false
					continue
				}
				if loss && u.Snapshot != nil && u.Current {
					fresh = true
				}
				if u.Marker == "after-loss" {
					check(c, loss, "profile stream hid deliberate local delivery loss")
					check(c, !u.Current || fresh, "profile continuation falsely presented stale state as current")
				}
			}
			check(c, loss, "profile stream omitted local delivery loss")
			replacement := &profileObservationLog{}
			var stopFresh openrtc.Unsubscribe
			if f.Stateful {
				begin := make(chan struct{})
				done := make(chan error, 1)
				go func() {
					select {
					case <-begin:
						done <- f.Commit(ctx, "fresh-next")
					case <-ctx.Done():
						done <- ctx.Err()
					}
				}()
				close(begin)
				stopFresh = profileObserve(ctx, c, f, replacement)
				select {
				case err := <-done:
					must(err)
				case <-ctx.Done():
					must(c.NotRun("profile fresh snapshot handoff checkpoint unavailable"))
				}
				profileInitial(c, f, replacement.read())
				us := replacement.read()
				if us[0].Snapshot.Completeness == "complete" {
					check(c, us[0].Snapshot.Facts["marker"] == "after-loss" || us[0].Snapshot.Facts["marker"] == "fresh-next", "profile resnapshot retained stale facts")
					check(c, profileMarkerCount(us, "fresh-next") == 1, "fresh profile snapshot/stream handoff has gap or duplicate")
				} else {
					c.Partial("profile resnapshot coverage does not establish a complete fresh handoff: " + profile)
				}
			} else {
				// Ephemeral payloads must not be replayed into a retained snapshot. Test
				// continuation using a stimulus after completed fresh registration.
				stopFresh = profileObserve(ctx, c, f, replacement)
				profileInitial(c, f, replacement.read())
				must(f.Commit(ctx, "fresh-next"))
				check(c, profileMarkerCount(replacement.read(), "fresh-next") == 1, "transient profile stream lost fresh continuation")
			}
			defer stopFresh()
			stopA()
			stopA()
			must(f.Commit(ctx, "post-removal"))
			check(c, profileMarkerCount(a.read(), "post-removal") == 0, "removed profile observer received a new covered change")
			check(c, profileMarkerCount(b.read(), "post-removal") == 1 && profileMarkerCount(replacement.read(), "post-removal") == 1, "profile removal also removed another independent observer")
		}
	})}),
}

func init() { CapabilityTests = append(CapabilityTests, ProfileObservationTests...) }
