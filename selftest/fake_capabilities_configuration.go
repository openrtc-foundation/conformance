package selftest

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/helpers"
	"github.com/openrtc/conformance/openrtc"
	"reflect"
	"strings"
	"sync"
)

type fakeCapabilities struct {
	captureCondition string

	revision  openrtc.Revision
	overrides map[string]string

	mu        sync.Mutex
	listeners map[uint64]func(openrtc.CapabilitySnapshot)
	next      uint64
	defects   FakeDefects
}

func newFakeCapabilities(d FakeDefects) *fakeCapabilities {
	return &fakeCapabilities{defects: d, listeners: map[uint64]func(openrtc.CapabilitySnapshot){}}
}
func (c *fakeCapabilities) Snapshot() openrtc.CapabilitySnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	revision := c.revision
	if revision == 0 {
		revision = 1
	}
	ds := make([]openrtc.CapabilityDescriptor, 0, len(helpers.PortableCapabilities))
	for _, id := range helpers.PortableCapabilities {
		d := descriptorFor(openrtc.PortableCapabilityID(id), revision)
		if support, ok := c.overrides[id]; ok {
			d.Support.Level = support
		}
		if id == "media.capture" && (c.captureCondition == "unknown-with-channel" || c.captureCondition == "denied-with-pending") {
			status := "unknown"
			if c.captureCondition == "denied-with-pending" {
				status = "pending"
				d.Authorization.Decision = "denied"
			}
			d.Prerequisites = []openrtc.CapabilityPrerequisite{{Kind: "service-evaluation", Target: "capture", Status: status, Reason: "fixture-controlled assessment", ResolutionChannel: "fixture:reevaluate", EvaluateAt: "next-operation"}}
		}
		ds = append(ds, d)
	}
	return openrtc.CapabilitySnapshot{Revision: revision, Scope: openrtc.CapabilityScope{AdapterID: adapterID, Artifact: "none", Platform: "node", Profile: "conference", Authority: "client", Resource: "client"}, Entries: ds, AdapterEntries: []openrtc.AdapterCapabilityDescriptor{}}
}
func (c *fakeCapabilities) Negotiate(_ context.Context, requests []openrtc.CapabilityRequest) openrtc.Async[openrtc.NegotiationResult] {
	return startOperation(c, func() (openrtc.NegotiationResult, error) {
		s := c.Snapshot()
		result := openrtc.NegotiationResult{Entries: make([]openrtc.NegotiationEntry, 0, len(requests))}
		for _, r := range requests {
			var d *openrtc.CapabilityDescriptor
			for i := range s.Entries {
				if s.Entries[i].ID == r.ID {
					d = &s.Entries[i]
					break
				}
			}
			e := openrtc.NegotiationEntry{Request: r, DescriptorRevision: openrtc.Unknown[openrtc.Revision](), Accepted: openrtc.Unknown[openrtc.CapabilityRequest](), Pending: []openrtc.CapabilityPrerequisite{}, Alternatives: []openrtc.CapabilityRequest{}, Reason: "capability assessed by the fake"}
			if d != nil && d.Support.Level == "supported" || c.defects.AcceptUnknownRequired && r.Strength == "required" {
				if d == nil {
					v := descriptorFor("session.conference", 1)
					d = &v
				}
				e.Status = "accepted"
				e.DescriptorRevision = openrtc.KnownValue(d.Revision)
				e.Accepted = openrtc.KnownValue(r)
			} else if r.Strength == "preferred" {
				e.Status = "not-selected"
				e.Reason = "capability not available in the fake"
			} else {
				e.Status = "rejected"
				e.Reason = "unsupported-capability: not implemented by the fake"
			}
			result.Entries = append(result.Entries, e)
		}
		return result, nil

	})
}
func (c *fakeCapabilities) OnChanged(f func(openrtc.CapabilitySnapshot)) openrtc.Unsubscribe {
	c.mu.Lock()
	c.next++
	id := c.next
	c.listeners[id] = f
	c.mu.Unlock()
	return func() { c.mu.Lock(); delete(c.listeners, id); c.mu.Unlock() }
}

var allowedKeys = map[openrtc.ExtensionLifetime][]string{"provider-init": {"applicationId", "diagnostics", "placement", "extensions"}, "client-init": {"identity", "credentials", "endpoint", "captureDefaults", "requirements", "placement", "extensions"}, "participant-join": {"completion", "encryption", "recovery", "participant", "initialMedia", "receive", "placement", "requirements", "signal", "extensions"}, "session-create": {"name", "metadata", "policy", "placement", "requirements", "extensions"}}
var planes = []string{"signalingRegion", "mediaRegion", "recordingProcessingRegion", "recordingStorageRegion", "deploymentRegion", "dataRegion"}

type validationOutcome struct {
	placement []openrtc.PlacementObservation
}

func objectMap(v any) map[string]any {
	rv := reflect.ValueOf(v)
	for rv.IsValid() && (rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer) {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return nil
	}
	m := map[string]any{}
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return nil
		}
		iter := rv.MapRange()
		for iter.Next() {
			m[iter.Key().String()] = iter.Value().Interface()
		}
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			parts := strings.Split(field.Tag.Get("json"), ",")
			name := parts[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}
			if len(parts) > 1 && parts[1] == "omitempty" && rv.Field(i).IsZero() {
				continue
			}
			m[name] = rv.Field(i).Interface()
		}
	default:
		return nil
	}
	return m
}
func validationError(code openrtc.ErrorCode, op, msg string, path []string, life openrtc.ExtensionLifetime) error {
	return fail(code, op, msg, failureExtras{optionPath: path, lifetime: &life})
}
func validateHolder(life openrtc.ExtensionLifetime, holder any, defects FakeDefects, op string) (validationOutcome, error) {
	record := objectMap(holder)
	out := validationOutcome{placement: []openrtc.PlacementObservation{}}
	if !defects.IgnoreUnknownOptions {
		for k := range record {
			found := false
			for _, allowed := range allowedKeys[life] {
				if k == allowed {
					found = true
					break
				}
			}
			if !found {
				return out, validationError("unsupported-option", op, fmt.Sprintf("field '%s' does not belong to %s", k, life), []string{k}, life)
			}
		}
		if err := validateExtensions(record["extensions"], life, op); err != nil {
			return out, err
		}
	}
	placement := objectMap(record["placement"])
	for _, plane := range planes {
		value, ok := placement[plane]
		if !ok || value == nil {
			continue
		}
		b, _ := json.Marshal(value)
		var constraint openrtc.PlacementConstraint
		if err := json.Unmarshal(b, &constraint); err != nil {
			return out, err
		}
		path := []string{"placement", plane}
		if len(constraint.Regions) == 0 {
			return out, validationError("invalid-option", op, "region list must not be empty", path, life)
		}
		for _, region := range constraint.Regions {
			if region.Namespace != string(adapterID) {
				return out, validationError("invalid-option", op, "region namespace is not this adapter's", path, life)
			}
		}
		if constraint.Strength == "requirement" && !defects.PlacementDowngrade {
			return out, validationError("placement-unsatisfied", op, "the fake cannot enforce any region", path, life)
		}
		p := openrtc.PlacementDeployment
		switch plane {
		case "signalingRegion":
			p = openrtc.PlacementSignaling
		case "mediaRegion":
			p = openrtc.PlacementMedia
		case "dataRegion":
			p = openrtc.PlacementData
		}
		out.placement = append(out.placement, openrtc.PlacementObservation{Plane: p, Requested: constraint, Enforcement: "not-enforced", Effective: openrtc.Unknown[[]openrtc.RegionRef](), Observed: openrtc.Unknown[[]openrtc.RegionRef](), Authority: "client", Evidence: []openrtc.EvidenceReference{}})
	}
	return out, nil
}
func validateExtensions(extensions any, life openrtc.ExtensionLifetime, op string) error {
	for k, v := range objectMap(extensions) {
		if k != string(adapterID) {
			return validationError("unsupported-option", op, fmt.Sprintf("unknown adapter namespace '%s'", k), []string{k}, life)
		}
		m := objectMap(v)
		if m == nil || fmt.Sprint(m["lifetime"]) != string(life) {
			return validationError("invalid-option", op, "payload was validated for another lifetime", []string{k}, life)
		}
		if m["schemaId"] != schemaID {
			return validationError("unsupported-option", op, fmt.Sprintf("unknown schema '%v'", m["schemaId"]), []string{k, "schemaId"}, life)
		}
	}
	return nil
}

type fakeSchema struct{}

var _ openrtc.ConfigurationSchema = (*fakeSchema)(nil)

func buildSchema() openrtc.ConfigurationSchema         { return fakeSchema{} }
func (fakeSchema) AdapterID() openrtc.AdapterID        { return adapterID }
func (fakeSchema) SchemaID() string                    { return schemaID }
func (fakeSchema) Revision() string                    { return "1" }
func (fakeSchema) Lifetime() openrtc.ExtensionLifetime { return "client-init" }
func (fakeSchema) JSONSchema() openrtc.JSONSchemaDocument {
	return openrtc.JSONSchemaDocument{Dialect: "https://json-schema.org/draft/2020-12/schema", Document: openrtc.JSONObject{"type": "object"}}
}
func (fakeSchema) Bindings() []openrtc.ConfigurationBinding {
	return []openrtc.ConfigurationBinding{{Path: []string{"fakeOption"}, Lifetime: openrtc.ClientInit, Authority: "client", NativeOwner: "harness-fake", Required: openrtc.KnownValue(false), DefaultValue: openrtc.DefaultDescription{Source: "unspecified", Value: openrtc.Unknown[openrtc.JSONValue]()}, UpdateMechanism: "unknown", EffectiveAfter: "unknown", Mutability: "fixed", Evidence: []openrtc.EvidenceReference{}}}
}
func (fakeSchema) Validate(input any) (openrtc.ValidatedExtensionPayload, error) {
	record := objectMap(input)
	if record == nil {
		record = map[string]any{}
	}
	for k := range record {
		if k != "fakeOption" {
			return openrtc.ValidatedExtensionPayload{}, validationError("unsupported-option", "configure", fmt.Sprintf("field '%s' is not declared", k), []string{k}, "client-init")
		}
	}
	return openrtc.ValidatedExtensionPayload{AdapterID: adapterID, Lifetime: "client-init", SchemaID: schemaID, SchemaRevision: "1", Payload: record}, nil
}

type fakeAttempt struct {
	deadlineExpired bool
	result          openrtc.Async[openrtc.JoinResult]
	mu              sync.Mutex
	id              openrtc.AttemptID
	generation      openrtc.GenerationID
	done            *harness.Deferred[openrtc.JoinResult]
	abort           context.CancelFunc
	abortContext    context.Context
	fenced          bool
	cancelHandler   func(context.Context) (openrtc.CancellationReport, error)
}

func newFakeAttempt(g openrtc.GenerationID, owner ...any) *fakeAttempt {
	ctx, cancel := context.WithCancel(context.Background())
	a := &fakeAttempt{id: openrtc.AttemptID(newID("attempt")), generation: g, done: harness.NewDeferred[openrtc.JoinResult](), abort: cancel, abortContext: ctx, cancelHandler: func(context.Context) (openrtc.CancellationReport, error) {
		return openrtc.CancellationReport{}, fmt.Errorf("unset")
	}}
	retainedOwner := any(a)
	if len(owner) > 0 {
		retainedOwner = owner[0]
	}
	a.result = startOperation(retainedOwner, func() (openrtc.JoinResult, error) {
		v, reason, rejected, err := a.done.Await(context.Background())
		if err != nil {
			return v, err
		}
		if rejected {
			if e, ok := reason.(error); ok {
				return v, e
			}
			return v, fmt.Errorf("%v", reason)
		}
		return v, nil
	})
	return a
}

var _ openrtc.JoinAttempt = (*fakeAttempt)(nil)

func (a *fakeAttempt) ID() openrtc.AttemptID                                    { return a.id }
func (a *fakeAttempt) Generation() openrtc.GenerationID                         { return a.generation }
func (a *fakeAttempt) Result(context.Context) openrtc.Async[openrtc.JoinResult] { return a.result }
func (a *fakeAttempt) Cancel(ctx context.Context) openrtc.Async[openrtc.CancellationReport] {
	return startOperation(a, func() (openrtc.CancellationReport, error) {
		return a.cancelHandler(ctx)

	})
}

type fakeClient struct {
	provider           *fakeProvider
	disposed           bool
	defects            FakeDefects
	identity           openrtc.ApplicationIdentity
	credentialProvider openrtc.CredentialProvider
	id                 openrtc.ClientID
	world              *world
	capabilities       *fakeCapabilities
	mu                 sync.Mutex
	sessionMap         map[openrtc.LocalSessionID]openrtc.RTCSession
	sessionOrder       []*fakeSession
	disposeOnce        sync.Once
	disposeReport      openrtc.CleanupReport
	disposeErr         error
}

type fakeSession struct {
	milestoneReached                              chan struct{}
	admissionReady                                chan struct{}
	completion                                    string
	joinMu                                        sync.Mutex
	workerDone                                    chan struct{}
	joinValidation                                validationOutcome
	nativeParticipants                            map[string]openrtc.RTCParticipant
	nativeCurrent                                 map[string]string
	instanceID                                    openrtc.SessionInstanceID
	credentialRevision                            uint64
	client                                        *fakeClient
	resolvedAddress                               *openrtc.SessionAddress
	id                                            openrtc.LocalSessionID
	generation                                    openrtc.GenerationID
	target                                        openrtc.ConferenceTarget
	capabilities                                  *fakeCapabilities
	observation                                   *fakeObservation
	recovery                                      openrtc.Known[openrtc.RecoveryStatus]
	state                                         openrtc.ConnectionState
	admission                                     openrtc.AdmissionStatus
	cleanup                                       openrtc.CleanupStatus
	credentials                                   openrtc.CredentialStatus
	roster                                        []openrtc.RTCParticipant
	revision                                      openrtc.Revision
	attempt                                       *fakeAttempt
	settled, disposed, nativeJoined               bool
	addressValue, identityKey, lastCredentialText string
	leaveCount                                    int
	cleanupMu                                     sync.Mutex
	stateMu                                       sync.Mutex
	cleanupRunMu                                  sync.Mutex
	cleanupPromise                                *harness.Deferred[openrtc.CleanupReport]
	media                                         *fakeMedia
	world                                         *world
	defects                                       FakeDefects
}

func initialCredentials() openrtc.CredentialStatus {
	return openrtc.CredentialStatus{State: "empty", Validity: "unknown", Metadata: openrtc.Unknown[openrtc.CredentialMetadata](), Lifecycle: openrtc.CredentialLifecycle{ReplacementMode: "application-live", AdmissionExpiry: "unknown", LiveExpiry: "unknown", ReconnectExpiry: "unknown", RevocationEffect: "unknown", ReplacementAcknowledgment: openrtc.Unknown[openrtc.CredentialPhase]()}, Replacement: openrtc.Unknown[openrtc.CredentialResult](), RenewalError: openrtc.Unknown[openrtc.RTCError]()}
}
func (s *fakeSession) Instance() openrtc.Known[openrtc.SessionInstance] {
	s.stateMu.Lock()
	joined := s.nativeJoined
	s.stateMu.Unlock()
	if !joined {
		return openrtc.Unknown[openrtc.SessionInstance]()
	}
	return s.instanceView()
}
func (s *fakeSession) unsupported(operation string) openrtc.RTCError {
	return fail("unsupported-capability", operation, "not implemented by the fake", failureExtras{})
}
func (s *fakeSession) buildSnapshot() openrtc.SessionSnapshot {
	n := uint64(len(s.roster))
	credentials := s.credentialViews()
	roster := map[openrtc.ParticipantConnectionID]openrtc.RTCParticipant{}
	for _, participant := range s.roster {
		roster[participant.ID] = participant
	}
	snapshot := openrtc.SessionSnapshot{ID: s.id, Kind: "conference", Sources: map[openrtc.CaptureSourceID]openrtc.CaptureSourceView{}, ReceiveSources: map[openrtc.ReceiveSourceID]openrtc.ReceiveSourceView{}, Tracks: map[openrtc.TrackID]openrtc.TrackView{}, Renders: map[openrtc.RenderBindingID]openrtc.RenderBindingView{}, Placement: []openrtc.PlacementObservation{}, ActiveSpeakers: openrtc.Unknown[openrtc.ActiveSpeakerObservation](), NetworkQuality: openrtc.Unknown[[]openrtc.NetworkQualityObservation](), Revision: s.revision, Generation: s.generation, Connection: openrtc.ConnectionSnapshot{Generation: s.generation, State: s.state, Admission: s.admission, MediaReadiness: "inactive", Attempt: openrtc.Unknown[openrtc.AttemptID](), Instance: s.instanceView(), RecoveryPolicy: fakeRecoveryPolicy(), Recovery: s.recovery, NativeState: openrtc.Unknown[string](), NativeReason: openrtc.Unknown[string]()}, Roster: openrtc.RosterView{Entries: roster, Completeness: "complete", ObservedCount: n, ReportedCount: openrtc.KnownValue(n), CoverageReason: "the fake lists every endpoint it created"}, Publications: map[openrtc.PublicationID]openrtc.TrackPublication{}, Subscriptions: map[openrtc.SubscriptionID]openrtc.TrackSubscription{}, Credentials: credentials, Cleanup: s.cleanup, Consistency: "best-effort", SourceFreshness: openrtc.Unknown[openrtc.Instant]()}
	if s.media != nil {
		s.media.project(&snapshot)
	}
	return snapshot
}

type sessionMutationEvent struct {
	key     string
	payload func(openrtc.SessionSnapshot, openrtc.SessionSnapshot) any
}

// apply applies a change and emits the events of that one transaction.
func (s *fakeSession) apply(mutate func(), events []sessionMutationEvent) {
	s.stateMu.Lock()
	previous := s.observation.Snapshot()
	mutate()
	s.revision++
	next := s.buildSnapshot()
	s.stateMu.Unlock()
	all := make([]observationEvent, 0, len(events)+1)
	for _, e := range events {
		all = append(all, observationEvent{key: e.key, payload: e.payload(previous, next)})
	}
	all = append(all, observationEvent{key: "snapshotChanged", payload: openrtc.StateChange[openrtc.SessionSnapshot]{Previous: openrtc.KnownValue(previous), Current: next}})
	s.observation.commit(next, all)
}
func (s *fakeSession) setState(next openrtc.ConnectionState) {
	s.apply(func() { s.state = next }, []sessionMutationEvent{{key: "connectionChanged", payload: func(prev, cur openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.ConnectionSnapshot]{Previous: openrtc.KnownValue(prev.Connection), Current: cur.Connection}
	}}})
}
