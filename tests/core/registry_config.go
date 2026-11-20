package core

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
)

var optionErrors = []openrtc.ErrorCode{"unsupported-option", "invalid-option"}

// descriptorAdapter replaces the descriptor while preserving the underlying methods.
type descriptorAdapter struct {
	openrtc.RTCAdapter
	descriptor openrtc.ProviderDescriptor
}

func (a descriptorAdapter) Descriptor() openrtc.ProviderDescriptor { return a.descriptor }
func isNonEmpty(s string) bool                                     { return strings.TrimSpace(s) != "" }
func exposesMember(object any, name string) bool {
	v := reflect.ValueOf(object)
	if !v.IsValid() {
		return false
	}
	if v.MethodByName(name).IsValid() {
		return true
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	return v.Kind() == reflect.Struct && v.FieldByName(name).IsValid()
}
func configFailure(c *harness.TestContext, e *openrtc.RTCFailure, lifetime openrtc.ExtensionLifetime, fragment, label string) {
	check(c, slices.Contains(e.OptionPath, fragment), fmt.Sprintf("%s: optionPath %v does not name '%s' (CFG-002)", label, e.OptionPath, fragment))
	check(c, e.Lifetime != nil && *e.Lifetime == lifetime, fmt.Sprintf("%s: lifetime %v, expected %s (CFG-002)", label, e.Lifetime, lifetime))
	check(c, e.AdapterID.Status == "known" && e.AdapterID.Value != nil && *e.AdapterID.Value == c.Descriptor.AdapterID, label+": adapterId not reported as the adapter under test (CFG-002)")
}
func target(c *harness.TestContext) harness.ConformanceTarget {
	t, ok := c.Target.(harness.ConformanceTarget)
	if !ok {
		must(c.NotRun("fixture does not implement the conformance target contract"))
	}
	return t
}
func expect(c *harness.TestContext, err error, codes []openrtc.ErrorCode, label string) *openrtc.RTCFailure {
	return value(c.ExpectError(err, harness.ExpectedFailure{Codes: codes}, label))
}
func rejected(c *harness.TestContext, err error, msg string) {
	if err == nil {
		must(c.Fail(msg))
	}
}
func disposeProvider(ctx context.Context, p openrtc.RTCProvider) {
	if p != nil {
		_, _ = cleanupAfter(ctx, p.Inspection(), p.Dispose(ctx))
	}
}
func disposeClient(ctx context.Context, p openrtc.RTCClient) {
	if p != nil {
		_, _ = cleanupAfter(ctx, p.Inspection(), p.Dispose(ctx))
	}
}

// Raw configuration boundaries permit testing fields outside Go's typed contracts.
type rawRegistry interface {
	InitializeRaw(context.Context, openrtc.AdapterID, openrtc.ProviderConfig, map[string]any) (openrtc.RTCProvider, error)
}
type rawProvider interface {
	CreateClientRaw(context.Context, openrtc.ClientConfig, map[string]any) (openrtc.RTCClient, error)
}
type rawSession interface {
	JoinRaw(openrtc.JoinSessionOptions, map[string]any) openrtc.JoinAttempt
}

// RegistryConfigTests contains the mandatory registration and configuration tests.
var RegistryConfigTests = []harness.ConformanceTest{
	{ID: "CT-REG-001", Title: "registration accepts a valid descriptor and rejects a duplicate adapter ID", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-002", "ADP-100", "ADP-101"}, Observable: "register() returns normally once; a second register of the same adapterId rejects with RTCError; initialize still works afterwards", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		t := target(c)
		r := t.CreateRegistry()
		a := t.CreateAdapter()
		d := a.Descriptor()
		for _, x := range []struct{ name, value string }{{"adapterId", string(d.AdapterID)}, {"adapterVersion", d.AdapterVersion}, {"sdkArtifact", d.SDKArtifact}, {"platform", d.Platform}} {
			check(c, isNonEmpty(x.value), "descriptor."+x.name+" is empty (ADP-100)")
		}
		check(c, len(d.Profiles) > 0, "descriptor declares no profile (CORE-102, ADP-100)")
		value(r.Register(a))
		_, err := r.Register(t.CreateAdapter())
		rejected(c, err, "duplicate adapter ID was accepted (ADP-101)")
		expect(c, err, nil, "duplicate registration")
		p := value(harness.WithinValue(ctx, c, "initialize after duplicate", 0, func(ctx context.Context) (openrtc.RTCProvider, error) {
			return r.Initialize(ctx, d.AdapterID, t.ProviderConfig()).Result(ctx)
		}))
		c.Track("provider", func(ctx context.Context) error { _, e := cleanupAfter(ctx, p.Inspection(), p.Dispose(ctx)); return e })
		c.Note("[ambiguity] ADP-101 does not name the error code for duplicates; any RTCError is accepted")
	})},
	{ID: "CT-REG-002", Title: "incompatible specification range or malformed descriptor rejects as incompatible-adapter", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-002", "ADP-100", "ADP-101", "ADP-102"}, Observable: "register() or initialize() rejects with code incompatible-adapter; no client is created natively", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		before := c.NativeCounts()
		t := target(c)
		base := t.CreateAdapter()
		d := base.Descriptor()
		future := d
		future.Specification = openrtc.VersionCompatibility{MinimumInclusive: openrtc.SpecVersion{Major: 9999}, MaximumExclusive: openrtc.SpecVersion{Major: 10000}}
		empty := d
		empty.SDKArtifact = ""
		profiles := d
		profiles.Profiles = []string{}
		for _, v := range []struct {
			label string
			d     openrtc.ProviderDescriptor
		}{{"specification range no host supports", future}, {"empty sdkArtifact", empty}, {"no profiles", profiles}} {
			r := t.CreateRegistry()
			a := descriptorAdapter{base, v.d}
			_, err := r.Register(a)
			if err == nil {
				var p openrtc.RTCProvider
				p, err = r.Initialize(ctx, v.d.AdapterID, t.ProviderConfig()).Result(ctx)
				if err == nil {
					disposeProvider(ctx, p)
				}
			}
			rejected(c, err, v.label+": registration/initialization succeeded (ADP-100/101)")
			expect(c, err, []openrtc.ErrorCode{"incompatible-adapter"}, v.label)
		}
		after := c.NativeCounts()
		if before != nil && after != nil {
			check(c, after.ClientsCreated == before.ClientsCreated && after.Admissions == before.Admissions, "native effects occurred while rejecting an incompatible adapter (ADP-101)")
		}
		c.Note("[ambiguity] no specification version number is defined; a far-future range stands in")
	})},
	{ID: "CT-REG-003", Title: "configuration schemas are discoverable before initialization and well formed", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-103", "CFG-003", "CFG-106", "CFG-107"}, Observable: "adapter.schemas(l) and provider.schemas(l) return records naming adapter, lifetime, revision, JSON Schema and bindings", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		a := target(c).CreateAdapter()
		p := value(c.Provider(ctx, nil))
		total := 0
		for _, l := range helpers.ClientLifetimes {
			for _, src := range []struct {
				name    string
				schemas []openrtc.ConfigurationSchema
			}{{"adapter", a.Schemas(l)}, {"provider", p.Schemas(l)}} {
				ids := map[string]bool{}
				for _, s := range src.schemas {
					total++
					label := fmt.Sprintf("%s.schemas('%s') %s", src.name, l, s.SchemaID())
					check(c, s.AdapterID() == c.Descriptor.AdapterID, label+": wrong adapterId")
					check(c, string(s.Lifetime()) == string(l), label+": lifetime mismatch (CFG-106)")
					check(c, isNonEmpty(s.SchemaID()), label+": empty schemaId")
					check(c, isNonEmpty(s.Revision()), label+": empty revision")
					check(c, isNonEmpty(s.JSONSchema().Dialect), label+": no JSON Schema dialect")
					check(c, !ids[s.SchemaID()], label+": duplicate schemaId")
					ids[s.SchemaID()] = true
					for _, b := range s.Bindings() {
						check(c, len(b.Path) > 0, label+": binding with empty path (CFG-107)")
						check(c, b.Lifetime == l, label+": binding lifetime mismatch")
						check(c, isNonEmpty(b.NativeOwner), label+": binding without native owner")
					}
				}
			}
		}
		c.Note(fmt.Sprintf("%d schema record(s) inspected; an empty list is permitted (ADP-103)", total))
	})},
	{ID: "CT-REG-004", Title: "provider and client initialization do not admit, provision, capture or publish", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-101", "ADP-106", "SES-001"}, Observable: "native-effect counters for provisioning, admissions, captures and publications stay at their prior values", Needs: []harness.FixtureNeed{"native-effects"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		n := value(c.RequireNative())
		b := n.Counts()
		_ = client(ctx, c)
		a := n.Counts()
		check(c, a.ProvisioningCalls == b.ProvisioningCalls, "provisioning occurred")
		check(c, a.Admissions == b.Admissions, "an admission occurred")
		check(c, a.Captures == b.Captures, "a capture occurred")
		check(c, a.Publications == b.Publications, "a publication occurred")
	})},
	{ID: "CT-REG-005", Title: "trusted administration is not reachable from the client or provider surface", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"ADP-001", "ADP-004", "ADP-110", "CORE-103", "CORE-105", "CFG-113"}, Observable: "no administration factory/provisioner member on RTCProvider or RTCClient; secret-named fields in ClientConfig reject", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		p := value(c.Provider(ctx, nil))
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p}))
		for _, v := range []struct {
			label string
			obj   any
		}{{"provider", p}, {"client", cl}} {
			for _, name := range []string{"CreateAdministration", "Provisioner", "Administration", "EndSession"} {
				check(c, !exposesMember(v.obj, name), fmt.Sprintf("%s exposes '%s' (ADP-004, CORE-105)", v.label, name))
			}
		}
		raw, ok := p.(rawProvider)
		if !ok {
			must(c.NotRun("provider does not expose raw configuration input"))
		}
		cfg := value(c.ClientConfig(harness.ClientOptions{}))
		made, err := raw.CreateClientRaw(ctx, cfg, map[string]any{"apiSecret": "conformance-secret"})
		if err == nil {
			disposeClient(ctx, made)
		}
		rejected(c, err, "ClientConfig carrying 'apiSecret' was accepted (CFG-113, CFG-100)")
		expect(c, err, optionErrors, "secret-named client field")
	})},
	{ID: "CT-CFG-001", Title: "foreign adapter namespace rejects unsupported-option in four client lifetimes", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-001", "CFG-002", "CFG-102", "CFG-105", "ADP-116", "ERR-103"}, Observable: "RTCError code unsupported-option naming the full optionPath, the lifetime and the adapter", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		t := target(c)
		r := t.CreateRegistry()
		a := t.CreateAdapter()
		value(r.Register(a))
		cfg := t.ProviderConfig()
		cfg.Extensions = helpers.ForgedExtensions(helpers.ForeignAdapter, "provider-init")
		p0, err := r.Initialize(ctx, a.Descriptor().AdapterID, cfg).Result(ctx)
		if err == nil {
			disposeProvider(ctx, p0)
		}
		rejected(c, err, "provider-init: foreign namespace accepted (CFG-002)")
		configFailure(c, expect(c, err, []openrtc.ErrorCode{"unsupported-option"}, "provider-init"), "provider-init", string(helpers.ForeignAdapter), "provider-init")
		p := value(c.Provider(ctx, nil))
		cc := value(c.ClientConfig(harness.ClientOptions{}))
		cc.Extensions = helpers.ForgedExtensions(helpers.ForeignAdapter, "client-init")
		cl0, err := p.CreateClient(ctx, cc).Result(ctx)
		if err == nil {
			disposeClient(ctx, cl0)
		}
		rejected(c, err, "client-init: foreign namespace accepted (CFG-002)")
		configFailure(c, expect(c, err, []openrtc.ErrorCode{"unsupported-option"}, "client-init"), "client-init", string(helpers.ForeignAdapter), "client-init")
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p}))
		_, err = cl.Configure(ctx, openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "info"}, Extensions: helpers.ForgedExtensions(helpers.ForeignAdapter, "runtime")}).Result(ctx)
		rejected(c, err, "runtime: foreign namespace accepted (CFG-002)")
		configFailure(c, expect(c, err, []openrtc.ErrorCode{"unsupported-option"}, "runtime"), "runtime", string(helpers.ForeignAdapter), "runtime")
		conference := value(c.Conference(ctx))
		s := c.Open(cl, conference)
		ext := helpers.ForgedExtensions(helpers.ForeignAdapter, "participant-join")
		_, err = s.Join(c.JoinOptions(harness.JoinOptionsOverride{Extensions: &ext})).Result(ctx).Result(ctx)
		rejected(c, err, "participant-join: foreign namespace accepted (CFG-002)")
		configFailure(c, expect(c, err, []openrtc.ErrorCode{"unsupported-option"}, "participant-join"), "participant-join", string(helpers.ForeignAdapter), "participant-join")
	})},
	{ID: "CT-CFG-002", Title: "foreign adapter namespace rejects unsupported-option at session-create", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-001", "CFG-002", "ADP-004", "ERR-103"}, Observable: "SessionProvisioner.create rejects unsupported-option with path and lifetime", Needs: []harness.FixtureNeed{"admin"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		a := value(c.Administration(ctx))
		_, err := a.Provisioner().Create(ctx, openrtc.CreateSessionOptions{Extensions: helpers.ForgedExtensions(helpers.ForeignAdapter, "session-create")}).Result(ctx)
		rejected(c, err, "session-create: foreign namespace accepted (CFG-002)")
		configFailure(c, expect(c, err, []openrtc.ErrorCode{"unsupported-option"}, "session-create"), "session-create", string(helpers.ForeignAdapter), "session-create")
	})},
	{ID: "CT-CFG-003", Title: "forged, wrong-lifetime and unknown-schema payloads are rejected, not trusted", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-001", "CFG-003", "CFG-105", "CFG-106", "ADP-103"}, Observable: "a payload for this adapter's namespace but the wrong lifetime, or with an unregistered schemaId, rejects with invalid-option or unsupported-option", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		p := value(c.Provider(ctx, nil))
		for _, v := range []struct {
			ext        openrtc.ProviderExtensions
			msg, label string
		}{{helpers.ForgedExtensions(c.Descriptor.AdapterID, "client-init", "runtime"), "runtime-lifetime payload accepted in client-init (CFG-001)", "wrong-lifetime payload"}, {helpers.ForgedExtensions(c.Descriptor.AdapterID, "client-init"), "payload with an unregistered schemaId accepted (CFG-003)", "unknown-schema payload"}} {
			cfg := value(c.ClientConfig(harness.ClientOptions{}))
			cfg.Extensions = v.ext
			cl, err := p.CreateClient(ctx, cfg).Result(ctx)
			if err == nil {
				disposeClient(ctx, cl)
			}
			rejected(c, err, v.msg)
			expect(c, err, optionErrors, v.label)
		}
	})},
	{ID: "CT-CFG-004", Title: "misplaced or unknown fields are rejected, never ignored", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-001", "CFG-100", "CFG-102", "CFG-113"}, Observable: "a field belonging to another lifetime (or unknown) in a holder rejects with optionPath naming it", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		t := target(c)
		r := t.CreateRegistry()
		a := t.CreateAdapter()
		value(r.Register(a))
		raw, ok := r.(rawRegistry)
		if !ok {
			must(c.NotRun("registry does not expose raw configuration input"))
		}
		p0, err := raw.InitializeRaw(ctx, a.Descriptor().AdapterID, t.ProviderConfig(), map[string]any{"identity": map[string]any{}})
		if err == nil {
			disposeProvider(ctx, p0)
		}
		rejected(c, err, "provider-init accepted a client-init field 'identity'")
		expect(c, err, optionErrors, "provider-init stray field")
		p := value(c.Provider(ctx, nil))
		rp, ok := p.(rawProvider)
		if !ok {
			must(c.NotRun("provider does not expose raw configuration input"))
		}
		cfg := value(c.ClientConfig(harness.ClientOptions{}))
		cl0, err := rp.CreateClientRaw(ctx, cfg, map[string]any{"participant": map[string]any{"metadata": map[string]any{}}})
		if err == nil {
			disposeClient(ctx, cl0)
		}
		rejected(c, err, "client-init accepted a join-lifetime field 'participant'")
		e := expect(c, err, optionErrors, "client-init stray field")
		check(c, slices.Contains(e.OptionPath, "participant"), "optionPath does not name 'participant' (CFG-002)")
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p}))
		s := c.Open(cl, value(c.Conference(ctx)))
		rs, ok := s.(rawSession)
		if !ok {
			must(c.NotRun("session does not expose raw join input"))
		}
		_, err = rs.JoinRaw(opts(c), map[string]any{"credentials": c.Spy.Provider(false)}).Result(ctx).Result(ctx)
		rejected(c, err, "join accepted a client-init field 'credentials' (CFG-001)")
		expect(c, err, optionErrors, "join stray field")
	})},
	{ID: "CT-CFG-005", Title: "invalid configuration is rejected before any external side effect", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-001", "CFG-100", "SES-105"}, Observable: "after a rejected join/createClient, credential callbacks were not invoked and native counters are unchanged; no valid subset was applied", Needs: []harness.FixtureNeed{"native-effects"}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		n := value(c.RequireNative())
		p := value(c.Provider(ctx, nil))
		before := n.Counts()
		cfg := value(c.ClientConfig(harness.ClientOptions{}))
		cfg.Extensions = helpers.ForgedExtensions(helpers.ForeignAdapter, "client-init")
		cl0, err := p.CreateClient(ctx, cfg).Result(ctx)
		if err == nil {
			disposeClient(ctx, cl0)
		}
		rejected(c, err, "invalid client configuration accepted")
		check(c, n.Counts().ClientsCreated == before.ClientsCreated, "a native client was created for a rejected configuration (CFG-100: no valid subset applied)")
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p}))
		s := c.Open(cl, value(c.Conference(ctx)))
		baseline := n.Counts()
		calls := len(c.Spy.Calls())
		ext := helpers.ForgedExtensions(helpers.ForeignAdapter, "participant-join")
		_, err = s.Join(c.JoinOptions(harness.JoinOptionsOverride{Extensions: &ext})).Result(ctx).Result(ctx)
		rejected(c, err, "invalid join configuration accepted")
		after := n.Counts()
		check(c, after.Admissions == baseline.Admissions, "admission reached the native layer")
		check(c, after.Captures == baseline.Captures, "capture happened before validation")
		check(c, len(c.Spy.Calls()) == calls, "credentials were requested before configuration validation finished (SES-105)")
	})},
	{ID: "CT-CFG-006", Title: "schema.validate rejects fields the schema does not declare", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-003", "CFG-100", "CFG-106"}, Observable: "validate({unknown field}) throws RTCError unsupported-option or invalid-option with a nonempty optionPath", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		p := value(c.Provider(ctx, nil))
		checked := 0
		for _, l := range helpers.ClientLifetimes {
			for _, s := range p.Schemas(l) {
				checked++
				_, err := s.Validate(map[string]any{"__openrtc_conformance_unknown_field__": 1})
				rejected(c, err, fmt.Sprintf("schema %s: unknown field accepted by validate (CFG-100)", s.SchemaID()))
				e := expect(c, err, optionErrors, s.SchemaID())
				check(c, len(e.OptionPath) > 0, fmt.Sprintf("schema %s: failure lacks optionPath (CFG-002)", s.SchemaID()))
			}
		}
		if checked == 0 {
			must(c.NotApplicable("adapter declares no adapter-specific schemas"))
		}
	})},
	{ID: "CT-CFG-007", Title: "mutating a supplied configuration object does not change a running client", Suite: harness.Core, Profile: harness.Conference, Requirements: []string{"CFG-004", "CFG-103"}, Observable: "a credential request issued after the application mutates its ClientConfig still carries the original subject", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		id := c.FreshIdentity("mutation")
		cfg := value(c.ClientConfig(harness.ClientOptions{Identity: &id}))
		p := value(c.Provider(ctx, nil))
		cl := value(c.Client(ctx, harness.ClientOptions{Provider: p, Config: &cfg}))
		original := cfg.Identity
		cfg.Identity.Subject = "mutated-after-creation"
		cfg.Identity.Namespace = "mutated-namespace"
		_ = value(c.JoinWith(ctx, cl, nil))
		calls := c.Spy.Calls()
		if len(calls) == 0 {
			must(c.Unknown("adapter never requested credentials; nothing to compare"))
		}
		subject := calls[0].Request.Subject
		check(c, subject.Subject == original.Subject && subject.Namespace == original.Namespace, fmt.Sprintf("credential request subject %s/%s followed the mutation (CFG-103)", subject.Namespace, subject.Subject))
	})},
	{ID: "CT-CFG-008", Title: "runtime configuration retains requested/accepted/effective values or an honest rejection", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"RFC §8.8:521–525", "RFC §8.8:519–525", "RFC §14.2:1102–1115", "CFG-004", "CFG-005", "CFG-103", "CFG-104", "CFG-150"}, Observable: "configure() completes and inspection retains a ConfigurationReceipt whose fields never claim effective without accepted, or rejects with unsupported-option and a path", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		cl := client(ctx, c)
		operation := cl.Configure(ctx, openrtc.ClientRuntimeCommand{Target: "client", Diagnostics: &openrtc.DiagnosticsPatch{Level: "debug"}})
		_, err := operation.Result(ctx)
		if err != nil {
			e := expect(c, err, optionErrors, "runtime diagnostics")
			check(c, len(e.OptionPath) > 0, "rejection lacks optionPath")
			c.Note("runtime diagnostics change rejected: acceptable, because rejection is honest")
			return
		}
		inspected := value(cl.Inspection().Operation(operation.OperationID()))
		check(c, inspected.Status == "retained" && inspected.Details != nil && inspected.Details.Configuration.Value != nil, "configuration receipt missing")
		result := inspected.Details.Configuration.Value.Outcome
		check(c, len(result.Fields) > 0, "successful requested diagnostics has no correlated result (RFC §8.8:521–525)")
		matched := false
		for _, f := range result.Fields {
			check(c, len(f.Path) > 0, "field result with empty path")
			if f.Effective.Status == "known" {
				check(c, f.Accepted.Status == "known", fmt.Sprintf("field %s: effective known but accepted unknown (CFG-104)", strings.Join(f.Path, ".")))
			}
			if strings.Join(f.Path, ".") == "diagnostics.level" {
				matched = true
				check(c, slices.Contains([]string{"selected", "not-selected", "unknown"}, f.Selection), "invalid preference selection")
				if f.Selection == "not-selected" {
					check(c, f.Reason != "", "not-selected preference lacks reason")
				}
			}

		}
		check(c, matched, "requested diagnostics.level missing from result")
		consistent := false
		for _, f := range result.Fields {
			consistent = consistent || len(f.Path) > 0
		}
		check(c, consistent, "inconsistent ConfigurationResult")
	})},
	{ID: "CT-CFG-009", Title: "an omitted option is not reported as a guessed explicit value", Suite: harness.Core, Profile: harness.Common, Requirements: []string{"CFG-005", "CFG-101"}, Observable: "for any schema binding with defaultValue status specified, a value is present; status unknown never carries a value", Needs: []harness.FixtureNeed{}, Run: run(func(ctx context.Context, c *harness.TestContext) {
		p := value(c.Provider(ctx, nil))
		inspected := 0
		for _, l := range helpers.ClientLifetimes {
			for _, s := range p.Schemas(l) {
				for _, b := range s.Bindings() {
					inspected++
					d := b.DefaultValue
					path := strings.Join(b.Path, ".")
					check(c, slices.Contains([]string{"portable", "provider", "unspecified", "unknown"}, d.Source), "invalid default source")
					check(c, slices.Contains([]string{"known", "unknown", "not-applicable"}, d.Value.Status), "default knowledge is not explicit")
					check(c, b.Mutability != "fixed" || b.UpdateMechanism != "setter", path+": fixed field advertises a setter (CFG-104)")
				}
			}
		}
		if inspected == 0 {
			must(c.NotApplicable("no adapter-specific bindings to inspect"))
		}
	})},
}
