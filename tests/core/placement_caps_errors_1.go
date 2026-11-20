package core

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

var unsupportedLike = []string{"unsupported", "unknown", "not-applicable"}

type extensionCapabilities struct {
	key openrtc.ExtensionKey
	ids []openrtc.PortableCapabilityID
}

// Session-scoped extension key to the portable capability that qualifies it; extensions.md scope table.
var sessionExtensions = []extensionCapabilities{
	{"independent-publication", []openrtc.PortableCapabilityID{"media.publish.independent"}}, {"manual-subscription", []openrtc.PortableCapabilityID{"media.subscribe.manual"}}, {"statistics", []openrtc.PortableCapabilityID{"media.statistics"}}, {"recording", []openrtc.PortableCapabilityID{"recording.server"}}, {"messaging", []openrtc.PortableCapabilityID{"messaging.ephemeral"}}, {"screen-sharing", []openrtc.PortableCapabilityID{"media.screen-share"}}, {"encryption", []openrtc.PortableCapabilityID{"encryption.media"}}, {"session-administration", []openrtc.PortableCapabilityID{"session.end"}},
}
var clientExtensions = []extensionCapabilities{{"calling", []openrtc.PortableCapabilityID{"calling.dial", "calling.accept"}}, {"devices", []openrtc.PortableCapabilityID{"devices.enumerate"}}}

func strongest(descriptors []openrtc.CapabilityDescriptor, ids []openrtc.PortableCapabilityID) string {
	for _, level := range []string{"supported", "partial", "unknown", "unsupported", "not-applicable"} {
		for _, d := range descriptors {
			if slices.Contains(ids, openrtc.PortableCapabilityID(d.ID)) && d.Support.Level == level {
				return level
			}
		}
	}
	return "absent"
}

// acquire probes an extension key, returning whether it was acquired or the thrown value.
func acquire(action func() any) (acquired bool, thrown any) {
	defer func() {
		if v := recover(); v != nil {
			acquired = false
			thrown = v
		}
	}()
	result := action()
	if result == nil {
		return false, nil
	}
	v := reflect.ValueOf(result)
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Map || v.Kind() == reflect.Interface || v.Kind() == reflect.Func || v.Kind() == reflect.Slice) && v.IsNil() {
		return false, nil
	}
	return true, nil
}

type rawClientConfiguration interface {
	ConfigureRaw(context.Context, openrtc.ClientRuntimeCommand, map[string]any) (openrtc.ConfigurationResult, error)
}

func placementOptions(c *harness.TestContext, p *openrtc.PlacementPolicy) openrtc.JoinSessionOptions {
	return c.JoinOptions(harness.JoinOptionsOverride{Placement: &p})
}

var placementCapsErrorsTests1 = []harness.ConformanceTest{
	{ID: "CT-PLC-001", Title: "an unenforceable placement requirement fails closed before any effect, per plane", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CFG-006", "CFG-007", "CFG-110", "ERR-103", "CFG-109"}, Observable: "provider-init, client-init and join each reject placement-unsatisfied for every plane; no credential callback and no admission follows", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		t := target(c)
		id := c.Descriptor.AdapterID
		region := helpers.SentinelRegion(id)
		r := t.CreateRegistry()
		value(r.Register(t.CreateAdapter()))
		p := value(c.Provider(ctx, nil))
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p}))
		dst := value(c.Conference(ctx))
		before := c.NativeCounts()
		for _, plane := range helpers.PlacementKeys {
			placement := helpers.PolicyFor(plane, helpers.Constraint("requirement", []openrtc.RegionRef{region}))
			calls := len(c.Spy.Calls())
			cfg := t.ProviderConfig()
			cfg.Placement = &placement
			created, err := harness.WithinValue(ctx, c, "captured action", 0, func(ctx context.Context) (openrtc.RTCProvider, error) { return r.Initialize(ctx, id, cfg).Result(ctx) })
			if err == nil {
				disposeProvider(ctx, created)
			}
			rejected(c, err, fmt.Sprintf("provider-init %s: unenforceable requirement accepted (CFG-007)", plane))
			expect(c, err, []openrtc.ErrorCode{"placement-unsatisfied"}, "provider-init "+plane)
			cc := value(c.ClientConfig(harness.ClientOptions{}))
			cc.Placement = &placement
			createdClient, err := harness.WithinValue(ctx, c, "captured action", 0, func(ctx context.Context) (openrtc.RTCClient, error) { return p.CreateClient(ctx, cc).Result(ctx) })
			if err == nil {
				disposeClient(ctx, createdClient)
			}
			rejected(c, err, fmt.Sprintf("client-init %s: unenforceable requirement accepted (CFG-007)", plane))
			expect(c, err, []openrtc.ErrorCode{"placement-unsatisfied"}, "client-init "+plane)
			s := c.Open(cl, dst)
			_, err = harness.WithinValue(ctx, c, "captured action", 0, func(arg0 context.Context) (openrtc.JoinResult, error) {
				return s.Join(placementOptions(c, &placement)).Result(arg0).Result(arg0)
			})
			rejected(c, err, fmt.Sprintf("join %s: unenforceable requirement accepted (CFG-007)", plane))
			e := expect(c, err, []openrtc.ErrorCode{"placement-unsatisfied"}, "join "+plane)
			check(c, e.OptionPath == nil || slices.Contains(e.OptionPath, plane), fmt.Sprintf("join %s: optionPath %v names a different plane", plane, e.OptionPath))
			check(c, len(c.Spy.Calls()) == calls, fmt.Sprintf("join %s: credentials requested before the placement requirement failed (CFG-110)", plane))
		}
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.Admissions == before.Admissions, "an admission occurred despite the failed requirement")
			check(c, after.Captures == before.Captures, "a capture occurred despite the failed requirement")
		}
		c.Note("[ambiguity] ERR-103 requires plane/regions/strength/boundary/enforcement in the error; RTCError has only optionPath, so only the plane is checked")
	})},
	{ID: "CT-PLC-002", Title: "an unenforceable placement preference is reported, never rejected as unsatisfied or verified", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CFG-005", "CFG-102", "CFG-111", "CFG-112", "ERR-103"}, Observable: "join resolves with a placement observation that is not verified, or rejects unsupported-option/invalid-option; placement-unsatisfied is a failure", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		region := helpers.SentinelRegion(c.Descriptor.AdapterID)
		cl := client(ctx, c)
		dst := value(c.Conference(ctx))
		for _, plane := range []string{"signalingRegion", "mediaRegion"} {
			p := helpers.PolicyFor(plane, helpers.Constraint("preference", []openrtc.RegionRef{region}))
			s := c.Open(cl, dst)
			_, err := harness.WithinValue(ctx, c, "captured action", 0, func(arg0 context.Context) (openrtc.JoinResult, error) {
				return s.Join(placementOptions(c, &p)).Result(arg0).Result(arg0)
			})
			if err != nil {
				expect(c, err, optionErrors, "preference "+plane)
				c.Note(plane + ": preference rejected as an unsupported option")
				continue
			}
			for _, o := range s.Inspection().Snapshot().Configuration.Placement {
				check(c, o.Enforcement != "verified" || len(o.Evidence) > 0, plane+": 'verified' placement without evidence (CFG-112)")
				check(c, o.Requested.Strength != "preference" || o.Enforcement != "verified", plane+": a sentinel preference was reported as verified (CFG-112)")
			}
			_, err = cleanupAfter(ctx, s.Inspection(), s.Leave(ctx))
			must(err)
		}
	})},
	{ID: "CT-PLC-003", Title: "empty or foreign-namespace region lists are rejected, not silently accepted", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CFG-108", "CFG-006", "CFG-001"}, Observable: "join with an empty region list or a foreign namespace rejects with an RTCError", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		dst := value(c.Conference(ctx))
		for _, v := range []struct {
			label   string
			regions []openrtc.RegionRef
		}{{"empty region list", []openrtc.RegionRef{}}, {"foreign namespace", []openrtc.RegionRef{{Namespace: string(helpers.ForeignAdapter), Code: "x"}}}} {
			p := helpers.PolicyFor("mediaRegion", helpers.Constraint("requirement", v.regions))
			s := c.Open(cl, dst)
			_, err := harness.WithinValue(ctx, c, "captured action", 0, func(arg0 context.Context) (openrtc.JoinResult, error) {
				return s.Join(placementOptions(c, &p)).Result(arg0).Result(arg0)
			})
			rejected(c, err, v.label+": accepted (CFG-108)")
			expect(c, err, []openrtc.ErrorCode{"invalid-option", "placement-unsatisfied", "unsupported-option"}, v.label)
		}
		c.Note("[ambiguity] CFG-108 names no error code; invalid-option is the natural choice")
	})},
	{ID: "CT-PLC-004", Title: "'verified' enforcement names its evidence and boundary", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"RFC §11.7:830–873", "RFC §14.2:1102–1115", "CFG-112", "CFG-108", "CFG-111"}, Observable: "for an enforceable requirement supplied by the fixture, join succeeds and any 'verified' observation carries evidence and a boundary", Needs: []harness.FixtureNeed{"enforceable-placement"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		control := c.Controls().EnforceablePlacement
		if control == nil {
			must(c.NotRun("fixture names no enforceable region"))
		}
		p := control()
		if p == nil {
			must(c.NotRun("fixture names no enforceable region"))
		}
		s := c.Open(client(ctx, c), value(c.Conference(ctx)))
		value(harness.WithinValue(ctx, c, "join", 0, func(arg0 context.Context) (openrtc.JoinResult, error) {
			return s.Join(placementOptions(c, p)).Result(arg0).Result(arg0)
		}))
		observations := s.Inspection().Snapshot().Configuration.Placement
		requested := []struct {
			plane      openrtc.PlacementPlane
			constraint *openrtc.PlacementConstraint
		}{
			{openrtc.PlacementSignaling, p.SignalingRegion}, {openrtc.PlacementMedia, p.MediaRegion},
			{openrtc.PlacementRecordingProcessing, p.RecordingProcessingRegion}, {openrtc.PlacementRecordingStorage, p.RecordingStorageRegion},
			{openrtc.PlacementDeployment, p.DeploymentRegion}, {openrtc.PlacementData, p.DataRegion},
		}
		requests := 0
		for _, request := range requested {
			if request.constraint == nil {
				continue
			}
			requests++
			matched := false
			for _, o := range observations {
				if o.Plane != request.plane || !reflect.DeepEqual(o.Requested, *request.constraint) {
					continue
				}
				matched = true
				if o.Requested.Strength == "preference" {
					check(c, o.Selection == "selected" || o.Selection == "not-selected" || o.Selection == "unknown", "unqualified requested preference result")
					if o.Selection == "not-selected" {
						check(c, o.Reason != "", "nonselected requested preference lacks reason")
					}
					continue
				}
				if o.Enforcement == "unknown" {
					must(c.Unknown("exact required placement enforcement unavailable"))
				}
				check(c, o.Enforcement == "verified" && len(o.Evidence) > 0, "required exact placement lacks established enforcement/evidence")
				check(c, len(o.Requested.Boundary.Resources) > 0, "verified placement lacks exact resource boundary")
				check(c, o.Effective.Status == "known" && o.Effective.Value != nil && len(*o.Effective.Value) > 0, "required effective regions unestablished")
				for _, region := range *o.Effective.Value {
					check(c, slices.Contains(request.constraint.Regions, region), "effective region outside exact requested set/namespace")
				}
			}
			check(c, matched, "missing correlated observation for exact placement request")
		}
		check(c, requests > 0, "enforceable-placement fixture contains no request")
	})},
	{ID: "CT-PLC-005", Title: "legacy Go runtime-command boundary rejects an unrecognized placement field", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.8:519–525", "RFC §11.7:830–873", "CFG-125", "CFG-103", "CFG-109"}, Observable: "client.configure with a stray placement field rejects unsupported-option or invalid-option naming placement", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		raw, ok := cl.(rawClientConfiguration)
		if !ok {
			must(c.NotRun("client does not expose raw runtime configuration"))
		}
		p := helpers.PolicyFor("mediaRegion", helpers.Constraint("preference", []openrtc.RegionRef{helpers.SentinelRegion(c.Descriptor.AdapterID)}))
		command := openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "info"}}
		_, err := harness.WithinValue(ctx, c, "captured action", 0, func(ctx context.Context) (openrtc.ConfigurationResult, error) {
			return raw.ConfigureRaw(ctx, command, map[string]any{"placement": p})
		})
		rejected(c, err, "a runtime command carrying placement was accepted (CFG-125)")
		e := expect(c, err, optionErrors, "runtime placement")
		check(c, slices.Contains(e.OptionPath, "placement"), "optionPath does not name 'placement' (CFG-002)")
	})},
	{ID: "CT-CAP-001", Title: "capability descriptors are complete, scoped and use only registered identifiers", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CAP-100", "CAP-101", "CAP-102", "CAP-103", "CAP-104", "CAP-105", "CAP-106", "CAP-107", "ADP-105"}, Observable: "each descriptor has explanation, scope for this adapter, valid availability/authorization values, evidence for 'unsupported', and an ID that is portable or in this adapter's namespace", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		p := value(c.Provider(ctx, nil))
		first := p.Capabilities().Snapshot()
		second := p.Capabilities().Snapshot()
		check(c, !math.IsInf(float64(first.Revision), 0) && math.Trunc(float64(first.Revision)) == float64(first.Revision) && second.Revision >= first.Revision, "revision not monotonic")
		seen := map[openrtc.CapabilityID]bool{}
		for _, d := range first.Entries {
			label := fmt.Sprintf("descriptor %s", d.ID)
			check(c, !seen[d.ID], label+": duplicate")
			seen[d.ID] = true
			check(c, slices.Contains(helpers.PortableCapabilities, string(d.ID)) || strings.HasPrefix(string(d.ID), fmt.Sprintf("adapter:%s/", c.Descriptor.AdapterID)), label+": neither portable nor in this adapter's namespace (CAP-107)")
			check(c, len(d.Support.Reason) > 0, label+": empty support explanation (CAP-101)")
			if d.Support.Level == "partial" {
				check(c, len(d.Support.Subset) > 0, label+": partial support names no subset (CAP-001)")
			}
			if d.Support.Level == "unsupported" {
				check(c, len(d.Evidence) > 0, label+": 'unsupported' without evidence (CAP-101)")
			}
			check(c, d.Scope.AdapterID == c.Descriptor.AdapterID, label+": scoped to another adapter")
			check(c, slices.Contains([]string{"available", "unavailable", "pending", "unknown"}, d.Availability.Status), label+": invalid runtime state")
			a := d.Authorization
			if a.Decision == "allowed" && a.Requirement == "required" {
				allowed := len(a.Authorities) > 0
				for _, authority := range a.Authorities {
					allowed = allowed && authority.Decision == "allowed"
				}
				check(c, allowed, label+": allowed although a required authority is not allowed (CAP-105)")
			}
			if a.Requirement == "unknown" {
				check(c, a.Decision != "allowed", label+": unknown requirement but allowed (CAP-105)")
			}
			for _, g := range d.Guarantees {
				check(c, slices.Contains([]string{"enforced", "upper-bound", "preference", "best-effort", "unknown"}, g.Level) && slices.Contains([]string{"reject", "propose-alternative", "unknown"}, g.Unmet) && len(g.Evidence) > 0, fmt.Sprintf("%s: guarantee %s lacks a valid level, unmet behavior or evidence (CAP-102)", label, g.ID))
			}
			for _, x := range d.Constraints {
				check(c, slices.Contains([]string{"send", "receive", "both", "not-applicable"}, x.Direction) && slices.Contains([]string{"fixed", "mutable", "unknown"}, x.Mutability) && len(x.Unit) > 0, fmt.Sprintf("%s: constraint %s lacks unit, direction or mutability (CAP-103)", label, x.ID))
			}
			for _, e := range d.Evidence {
				check(c, slices.Contains([]string{"fact(verified)", "inference", "design-proposal"}, e.Kind), label+": invalid evidence label (CAP-106)")
			}
		}
		c.Note(fmt.Sprintf("%d descriptor(s) inspected", len(first.Entries)))
	})},
	{ID: "CT-CAP-002", Title: "required unrecognized capabilities fail; preferred ones are reported not-selected", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CAP-002", "CAP-003", "CAP-107", "CAP-108"}, Observable: "required foreign capability: rejected entry; preferred: not-selected with a reason; every request is correlated and keeps its strength", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		id := openrtc.CapabilityID(fmt.Sprintf("adapter:%s/probe", helpers.ForeignAdapter))
		requests := []openrtc.CapabilityRequest{helpers.RequestAt(id, "required"), helpers.RequestAt(id, "preferred")}
		result := value(harness.WithinValue(ctx, c, "negotiate", 0, func(ctx context.Context) (openrtc.NegotiationResult, error) {
			return cl.Capabilities().Negotiate(ctx, requests).Result(ctx)
		}))
		check(c, len(result.Entries) == 2, "entries do not correlate one-to-one with requests (CAP-108)")
		check(c, negotiationStatus(result) == "rejected", fmt.Sprintf("aggregate '%s' despite a required unknown capability", negotiationStatus(result)))
		first, second := result.Entries[0], result.Entries[1]
		check(c, first.Status == "rejected", fmt.Sprintf("required entry is '%s', expected rejected", first.Status))
		check(c, second.Status == "not-selected", fmt.Sprintf("preferred entry is '%s', expected not-selected", second.Status))
		if second.Status == "not-selected" {
			check(c, len(second.Reason) > 0, "not-selected without a reason (CAP-003)")
		}
		check(c, first.Request.Strength == "required", "a required request was rewritten (CAP-108)")
		check(c, second.Request.Strength == "preferred", "a preferred request was rewritten (CAP-108)")
	})},
	{ID: "CT-CAP-003", Title: "required capabilities advertised unsupported, unknown or not-applicable are never accepted", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CAP-002", "CAP-101", "CAP-109", "CORE-002", "ADP-117", "ERR-109"}, Observable: "for every such descriptor, negotiate(required) has a rejected entry with a reason, or pending only with stated prerequisites for 'unknown'", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		checked := 0
		for _, d := range cl.Capabilities().Snapshot().Entries {
			if !slices.Contains(unsupportedLike, d.Support.Level) {
				continue
			}
			checked++
			result := value(harness.WithinValue(ctx, c, "negotiate "+string(d.ID), 0, func(ctx context.Context) (openrtc.NegotiationResult, error) {
				return cl.Capabilities().Negotiate(ctx, []openrtc.CapabilityRequest{helpers.RequestAt(d.ID)}).Result(ctx)
			}))
			check(c, negotiationStatus(result) != "accepted", fmt.Sprintf("%s (%s) was accepted (CAP-002)", d.ID, d.Support.Level))
			check(c, len(result.Entries) > 0 && len(result.Entries[0].Reason) > 0, "rejected entry lacks a reason")
			if negotiationStatus(result) == "pending" {
				check(c, d.Support.Level == "unknown" && len(result.Entries) > 0 && result.Entries[0].Status == "pending" && len(result.Entries[0].Pending) > 0, fmt.Sprintf("%s: pending without a stated deferred prerequisite (CAP-109)", d.ID))
			}
		}
		if checked == 0 {
			must(c.NotApplicable("no descriptor is advertised unsupported/unknown/not-applicable"))
		}
	})},
}

func negotiationStatus(result openrtc.NegotiationResult) string {
	status := "accepted"
	for _, entry := range result.Entries {
		if entry.Request.Strength != "required" {
			continue
		}
		if entry.Status == "rejected" {
			return "rejected"
		}
		if entry.Status == "pending" {
			status = "pending"
		}
	}
	return status
}
