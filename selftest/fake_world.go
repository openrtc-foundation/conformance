package selftest

import (
	"fmt"
	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
)

var NoDefects = FakeDefects{}

const adapterID openrtc.AdapterID = "harness-fake"
const schemaID = "harness-fake/client-init"

var hostSpec = openrtc.SpecVersion{Major: 0, Minor: 1, Patch: 0}
var category = map[openrtc.ErrorCode]openrtc.ErrorCategory{"unsupported-option": "configuration", "invalid-option": "configuration", "unsupported-capability": "capability", "capability-pending": "capability", "placement-unsatisfied": "placement", "invalid-state": "state", "operation-conflict": "state", "cancelled": "cancellation", "authentication-failed": "authentication", "authorization-denied": "authorization", "credential-failed": "credential", "credential-expired": "credential", "credential-revoked": "credential", "network-failure": "network", "operation-timeout": "network", "media-failure": "media", "resource-not-found": "resource", "resource-conflict": "resource", "quota-exceeded": "quota", "service-unavailable": "service", "protocol-failure": "protocol", "internal-failure": "internal", "provider-failure": "unknown", "cleanup-failed": "resource", "incompatible-adapter": "configuration", "adapter-contract-violation": "internal"}
var counter atomic.Uint64

func newID(prefix string) string { return fmt.Sprintf("%s-%d", prefix, counter.Add(1)) }

type failureExtras struct {
	optionPath         []string
	lifetime           *openrtc.ExtensionLifetime
	cause              error
	original           any
	kind, retryability string
	category           openrtc.ErrorCategory
	leakOriginal       bool
}

func fail(code openrtc.ErrorCode, operation, message string, extras failureExtras) openrtc.RTCError {
	if extras.leakOriginal {
		message = fmt.Sprintf("unsafe native mutant: %#v", extras.original)
	}
	p := openrtc.Unknown[openrtc.ProviderError]()
	if extras.original != nil {
		p = openrtc.KnownValue(openrtc.ProviderError{Original: extras.original, Provenance: openrtc.EvidenceReference{Kind: "inference", Source: "harness self-test", Artifact: "harness-fake", Version: openrtc.Unknown[string](), InspectedAt: "1970-01-01T00:00:00.000Z", Gaps: []string{"harness self-test"}}, Related: []openrtc.ProviderError{}, OriginalAvailability: "available"})
	}
	c := extras.category
	if c == "" {
		c = category[code]
	}
	retry := extras.retryability
	if retry == "" {
		retry = "never"
	}
	reasons := []string{"unknown"}
	reason := map[openrtc.ErrorCode]string{"credential-failed": "credential-acquisition-failed", "credential-expired": "credential-expired", "credential-revoked": "credential-revoked", "authentication-failed": "authentication-failed", "authorization-denied": "authorization-denied", "operation-conflict": "operation-conflict", "resource-not-found": "resource-not-found", "resource-conflict": "resource-conflict", "quota-exceeded": "quota-or-rate-limit", "network-failure": "network-failure", "service-unavailable": "service-unavailable", "media-failure": "media-failure", "protocol-failure": "protocol-failure", "cleanup-failed": "cleanup-failure"}[code]
	if reason != "" {
		reasons = []string{reason}
	}
	d := openrtc.RTCErrorDetails{Reasons: reasons, Outcome: "unknown", ID: openrtc.ErrorID(newID("err")), Code: code, Category: c, Operation: openrtc.OperationRef{Name: operation, Attempt: openrtc.Unknown[openrtc.AttemptID](), Generation: openrtc.Unknown[openrtc.GenerationID]()}, AdapterID: openrtc.KnownValue(adapterID), Resource: openrtc.Unknown[openrtc.ResourceRef](), Retryability: retry, RetryAfter: openrtc.Unknown[openrtc.Duration](), RecoveryAction: openrtc.Unknown[openrtc.RecoveryAction](), ProviderError: p, OptionPath: extras.optionPath, Lifetime: extras.lifetime, Cause: extras.cause}
	if extras.kind == "observation" {
		e := openrtc.NewRTCObservationError(message, d)
		return e
	}
	e := openrtc.NewRTCOperationError(message, d)
	return e
}
func emptyConfigurationResult(placement []openrtc.PlacementObservation) openrtc.ConfigurationResult {
	if placement == nil {
		placement = []openrtc.PlacementObservation{}
	}
	return openrtc.ConfigurationResult{Fields: []openrtc.ConfigurationFieldResult{}, Placement: placement, Schemas: []openrtc.SchemaReference{}, Effects: unknownEffects()}
}

type remote struct {
	identity  openrtc.ApplicationIdentity
	connected bool
}
type worldGate struct {
	reached   *harness.Deferred[struct{}]
	outcome   *harness.Deferred[any]
	reachedCh chan struct{}
}
type worldFailure struct {
	operation string
	failure   any
}
type world struct {
	pendingAsync        map[*fakeSession]*worldGate
	createGate          *worldGate
	holdCleanup         bool
	cleanupAcknowledges bool
	allocations         map[string]bool
	canarySubject       openrtc.ApplicationIdentity
	logs                []string

	canaries        []string
	fallbackEffects int

	mediaEffects harness.MediaFixtureEffects

	fault            *lifecycleFault
	resources        harness.LifecycleResources
	pendingCleanup   map[any]bool
	clock            func() openrtc.Instant
	mu               sync.Mutex
	counts           harness.NativeEffectCounts
	remotes          map[string][]*remote
	activeIdentities map[string]struct{}
	gate             *worldGate
	nextFailure      *worldFailure
	admissionMode    string
	held             map[*fakeSession]struct{}
	sessions         map[*fakeSession]struct{}
	tokenCounter     int
}

func newWorld() *world {
	return &world{pendingAsync: map[*fakeSession]*worldGate{}, allocations: map[string]bool{}, pendingCleanup: map[any]bool{}, clock: func() openrtc.Instant { return "1970-01-01T00:00:00.000Z" }, remotes: map[string][]*remote{}, activeIdentities: map[string]struct{}{}, admissionMode: "admit", held: map[*fakeSession]struct{}{}, sessions: map[*fakeSession]struct{}{}}
}
func (w *world) remotesAt(address string) []*remote {
	list, ok := w.remotes[address]
	if !ok {
		list = []*remote{}
		w.remotes[address] = list
	}
	return list
}

type registration struct {
	name         string
	listener     func(openrtc.EventEnvelope[any])
	active, once bool
}
type watcher struct {
	listener func(openrtc.SessionSnapshot)
	active   bool
}
type observationEvent struct {
	key     string
	payload any
}
type fakeObservation struct {
	streams       []*streamSubscriber
	streamDelayed map[*streamSubscriber]openrtc.ObservationUpdate
	loseStream    bool
	streamDefect  string
	closed        bool
	clock         func() openrtc.Instant
	mu            sync.Mutex
	registrations []*registration
	watchers      []*watcher
	queue         []func()
	dispatching   bool
	sequence      uint64
	lastByKey     map[string]openrtc.EventEnvelope[any]
	ownerID       openrtc.LocalOwnerID
	current       openrtc.SessionSnapshot
	defects       FakeDefects
}

var _ openrtc.SessionObservation = (*fakeObservation)(nil)

func newFakeObservation(initial openrtc.SessionSnapshot, defects FakeDefects) *fakeObservation {
	return &fakeObservation{clock: func() openrtc.Instant { return "1970-01-01T00:00:00.000Z" }, current: initial, defects: defects, ownerID: openrtc.LocalOwnerID(newID("owner")), lastByKey: map[string]openrtc.EventEnvelope[any]{}}
}
func (o *fakeObservation) Snapshot() openrtc.SessionSnapshot {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.defects.SnapshotAliases {
		return o.current
	}
	return detached(o.current)
}
func (o *fakeObservation) On(name string, listener func(openrtc.EventEnvelope[any])) openrtc.Unsubscribe {
	return o.register(name, listener, false)
}
func (o *fakeObservation) Once(name string, listener func(openrtc.EventEnvelope[any])) openrtc.Unsubscribe {
	return o.register(name, listener, true)
}
func (o *fakeObservation) Off(name string, listener func(openrtc.EventEnvelope[any])) {
	o.validateEvent(name, listener)
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, r := range o.registrations {
		if r.name == name && reflect.ValueOf(r.listener).Pointer() == reflect.ValueOf(listener).Pointer() {
			r.active = false
		}
	}
}
func (o *fakeObservation) Watch(listener func(openrtc.SessionSnapshot)) openrtc.Unsubscribe {
	o.mu.Lock()
	if listener == nil && !o.defects.WatchAcceptsNil {
		o.mu.Unlock()
		panic(fail("invalid-option", "watch", "nil listener", failureExtras{}))
	}
	if o.closed && !o.defects.WatchAfterDispose {
		o.mu.Unlock()
		panic(fail("invalid-state", "watch", "disposed owner", failureExtras{}))
	}
	w := &watcher{listener: listener, active: true}
	o.watchers = append(o.watchers, w)
	current := detached(o.current)
	if !o.defects.WatchSkipsBootstrap {
		o.queue = append(o.queue, func() { isolated(func() { listener(current) }) })
	}
	o.mu.Unlock()
	o.drain()
	return func() { o.mu.Lock(); w.active = false; o.mu.Unlock() }
}
func (o *fakeObservation) register(name string, listener func(openrtc.EventEnvelope[any]), once bool) openrtc.Unsubscribe {
	o.validateEvent(name, listener)
	o.mu.Lock()
	if listener == nil {
		o.mu.Unlock()
		panic(fail("invalid-option", "on", "nil listener", failureExtras{}))
	}
	if o.closed {
		o.mu.Unlock()
		panic(fail("invalid-state", "on", "disposed owner", failureExtras{}))
	}
	o.mu.Unlock()
	r := &registration{name: name, listener: listener, active: true, once: once}
	o.mu.Lock()
	o.registrations = append(o.registrations, r)
	last, ok := o.lastByKey[name]
	replay := o.defects.ReplayOnSubscribe
	o.mu.Unlock()
	if replay && ok {
		listener(last)
	}
	return func() { o.mu.Lock(); r.active = false; o.mu.Unlock() }
}

// commit publishes a snapshot and delivers events from one transaction.
func (o *fakeObservation) commit(next openrtc.SessionSnapshot, events []observationEvent) {
	txn := openrtc.TransactionID(newID("txn"))
	o.mu.Lock()
	o.queueStream(next)
	if !o.defects.ListenersBeforeCommit {
		o.current = next
	}
	for _, event := range events {
		o.sequence++
		e := openrtc.EventEnvelope[any]{EventID: openrtc.LocalEventID(newID("evt")), OwnerID: o.ownerID, Generation: next.Generation, Sequence: o.sequence, Revision: next.Revision, ObservedAt: o.clock(), Origin: "local-operation", NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), NativeTimestamp: openrtc.Unknown[openrtc.Instant](), NativeSequence: openrtc.Unknown[openrtc.ScopedSequence](), Transaction: txn, Payload: event.payload}
		key := event.key
		o.queue = append(o.queue, func() { o.deliver(key, e) })
	}
	o.queue = append(o.queue, func() {
		o.mu.Lock()
		ws := append([]*watcher(nil), o.watchers...)
		o.mu.Unlock()
		for _, w := range ws {
			o.mu.Lock()
			active := w.active
			o.mu.Unlock()
			if active {
				isolated(func() { w.listener(detached(next)) })
			}
		}
	})
	o.mu.Unlock()
	o.drain()
	if o.defects.ListenersBeforeCommit {
		o.mu.Lock()
		o.current = next
		o.mu.Unlock()
	}
}
func (o *fakeObservation) deliver(key string, e openrtc.EventEnvelope[any]) {
	o.mu.Lock()
	o.lastByKey[key] = e
	rs := append([]*registration(nil), o.registrations...)
	o.mu.Unlock()
	for _, r := range rs {
		o.mu.Lock()
		active := r.active && r.name == key
		if active && r.once {
			r.active = false
		}
		o.mu.Unlock()
		if !active {
			continue
		}
		if o.defects.ThrowingListenerBlocks {
			r.listener(e)
		} else {
			func() { defer func() { _ = recover() }(); r.listener(e) }()
		}
	}
}
func (o *fakeObservation) drain() {
	o.mu.Lock()
	if o.dispatching {
		o.mu.Unlock()
		return
	}
	o.dispatching = true
	o.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			o.mu.Lock()
			o.dispatching = false
			o.mu.Unlock()
			panic(r)
		}
	}()
	for {
		o.mu.Lock()
		if len(o.queue) == 0 {
			o.dispatching = false
			o.mu.Unlock()
			return
		}
		task := o.queue[0]
		o.queue = o.queue[1:]
		o.mu.Unlock()
		task()
	}
}

var supported = []openrtc.PortableCapabilityID{"session.conference", "media.statistics", "devices.enumerate", "messaging.ephemeral"}

func descriptorFor(id openrtc.PortableCapabilityID, revision openrtc.Revision) openrtc.CapabilityDescriptor {
	level, explanation := "unsupported", "not implemented by the in-memory fake"
	for _, v := range supported {
		if v == id {
			level, explanation = "supported", "implemented by the in-memory fake"
			break
		}
	}
	return openrtc.CapabilityDescriptor{ID: openrtc.CapabilityID(id), Revision: revision, Support: openrtc.SupportDescription{Level: level, Reason: explanation, Subset: []string{}}, Scope: openrtc.CapabilityDescriptorScope{AdapterID: adapterID, Artifact: "none", Platform: "node", Profile: "conference", Authority: "client", Resource: "client"}, Guarantees: []openrtc.GuaranteeDescriptor{}, Constraints: []openrtc.CapabilityConstraint{}, Prerequisites: []openrtc.CapabilityPrerequisite{}, Details: openrtc.CapabilityDetails{Kind: "operation"}, IdentityCollision: "reject-new", Availability: openrtc.CapabilityAvailability{Status: "available", Reason: "synthetic discovery", CheckedAt: openrtc.Unknown[openrtc.Instant](), ExpiresAt: openrtc.Unknown[openrtc.Instant]()}, Authorization: openrtc.CapabilityAuthorization{Requirement: "none-defined-by-layer", Authorities: []openrtc.AuthorizationRequirement{}, Decision: "not-evaluated"}, Evidence: []openrtc.EvidenceReference{{Kind: "design-proposal", Source: "harness self-test", Artifact: "harness-fake", Version: openrtc.Unknown[string](), InspectedAt: "1970-01-01T00:00:00.000Z", Gaps: []string{"harness self-test only"}}}}
}

func (o *fakeObservation) validateEvent(name string, listener func(openrtc.EventEnvelope[any])) {
	if o.defects.EmitterAcceptsInvalid {
		return
	}
	if listener == nil || !slices.Contains([]string{"connectionChanged", "admissionChanged", "participantChanged", "publicationChanged", "subscriptionChanged", "credentialChanged", "capabilitiesChanged", "cleanupChanged", "snapshotChanged", "error"}, name) {
		panic(fail("invalid-option", "observe", "invalid event or listener", failureExtras{}))
	}
}
