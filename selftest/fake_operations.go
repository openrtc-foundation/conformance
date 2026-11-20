package selftest

import (
	"github.com/openrtc/conformance/openrtc"
	"sync"
)

type operationStore struct {
	cleanupWatchers map[uint64]func()
	revision        openrtc.Revision
	cleanup         *openrtc.CleanupReport
	definitions     map[openrtc.SessionDefinitionID]openrtc.SessionDefinition
	resources       map[openrtc.SessionDefinitionID]openrtc.ConfigurationOutcome
	watchers        map[uint64]func()
	nextWatcher     uint64
	configuration   *openrtc.ConfigurationOutcome
	mu              sync.Mutex
	records         map[openrtc.OperationID]openrtc.OperationInspection
	evicted         map[openrtc.OperationID]bool
}

var operationStores sync.Map

func storeFor(owner any) *operationStore {
	switch v := owner.(type) {
	case *fakeMedia:
		owner = v.session
	case fakeConnection:
		owner = v.session
	case fakeStatistics:
		owner = v.owner
	}
	v, _ := operationStores.LoadOrStore(owner, &operationStore{cleanupWatchers: map[uint64]func(){}, definitions: map[openrtc.SessionDefinitionID]openrtc.SessionDefinition{}, resources: map[openrtc.SessionDefinitionID]openrtc.ConfigurationOutcome{}, watchers: map[uint64]func(){}, records: map[openrtc.OperationID]openrtc.OperationInspection{}, evicted: map[openrtc.OperationID]bool{}})
	return v.(*operationStore)
}

type operationObserver struct{ owner any }

func (o operationObserver) OperationStarted(id openrtc.OperationID) {
	s := storeFor(o.owner)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[id] = openrtc.OperationInspection{OperationID: id, Dtmf: openrtc.Unknown[openrtc.DtmfResult](), Configuration: openrtc.Unknown[openrtc.ConfigurationReceipt](), Effects: openrtc.Unknown[openrtc.MediaOperationEffects](), Negotiation: openrtc.Unknown[openrtc.NegotiationResult](), InitialMedia: openrtc.Unknown[openrtc.InitialMediaOutcome]()}
}
func (o operationObserver) OperationSettled(id openrtc.OperationID, value any, err error) {
	o.settle(id, value, err, nil)
}
func (o operationObserver) settle(id openrtc.OperationID, value any, err error, effects *openrtc.MediaOperationEffects) {
	s := storeFor(o.owner)
	s.mu.Lock()
	r := s.records[id]
	r.Dtmf = openrtc.NotApplicable[openrtc.DtmfResult]("operation does not send DTMF")
	r.Configuration = openrtc.NotApplicable[openrtc.ConfigurationReceipt]("operation does not consume configuration")
	r.Effects = openrtc.NotApplicable[openrtc.MediaOperationEffects]("operation has no media effects")
	r.Negotiation = openrtc.NotApplicable[openrtc.NegotiationResult]("operation does not negotiate capabilities")
	r.InitialMedia = openrtc.NotApplicable[openrtc.InitialMediaOutcome]("operation does not set up media")
	switch v := value.(type) {
	case openrtc.ConfigurationResult:
		if err == nil {
			s.configuration = &v
			r.Configuration = openrtc.KnownValue(openrtc.ConfigurationReceipt{Outcome: v})
		} else {
			r.Configuration = openrtc.Unknown[openrtc.ConfigurationReceipt]()
		}
	case openrtc.CleanupReport:
		if err == nil {
			s.cleanup = &v
		}
	case openrtc.SessionDefinition:
		if err != nil {
			r.Configuration = openrtc.Unknown[openrtc.ConfigurationReceipt]()
			break
		}
		s.definitions[v.ID] = v
		s.resources[v.ID] = emptyConfigurationResult(nil)
		r.Configuration = openrtc.KnownValue(openrtc.ConfigurationReceipt{Outcome: emptyConfigurationResult(nil)})
	case openrtc.RTCClient, openrtc.RTCProvider:
		r.Configuration = openrtc.KnownValue(openrtc.ConfigurationReceipt{Outcome: emptyConfigurationResult(nil)})
	case openrtc.NegotiationResult:
		r.Negotiation = openrtc.KnownValue(v)
	case openrtc.MediaOperationEffects:
		r.Effects = openrtc.KnownValue(v)
	case openrtc.JoinResult:
		if err != nil {
			r.Configuration = openrtc.Unknown[openrtc.ConfigurationReceipt]()
			r.InitialMedia = openrtc.Unknown[openrtc.InitialMediaOutcome]()
			break
		}
		if s.configuration != nil {
			r.Configuration = openrtc.KnownValue(openrtc.ConfigurationReceipt{Outcome: *s.configuration})
		}
		r.InitialMedia = openrtc.KnownValue(v.InitialMedia)
	}
	if effects != nil {
		r.Effects = openrtc.KnownValue(*effects)
	}
	if ownerDefects(o.owner).RetentionDropsIndependent {
		s.records = map[openrtc.OperationID]openrtc.OperationInspection{}
	}
	s.records[id] = r
	s.revision++
	callbacks := make([]func(), 0, len(s.watchers))
	for _, f := range s.watchers {
		callbacks = append(callbacks, f)
	}
	s.mu.Unlock()
	notifyCleanup(o.owner)
	if provider, ok := value.(openrtc.RTCProvider); ok && err == nil {
		target := storeFor(provider)
		target.mu.Lock()
		target.records[id] = r
		target.mu.Unlock()
	}
	if session := operationSession(o.owner); session != nil {
		session.apply(func() {}, nil)
	}
	for _, f := range callbacks {
		if !ownerDefects(o.owner).InspectionWatchSkipsCommit {
			isolated(f)
		}
	}
}
func startOperation[T any](owner any, work func() (T, error)) openrtc.Async[T] {
	return openrtc.StartOperation(operationObserver{owner}, work)
}

func operationSession(owner any) *fakeSession {
	switch v := owner.(type) {
	case *fakeSession:
		return v
	case *fakeMedia:
		return v.session
	case fakeConnection:
		return v.session
	case fakeStatistics:
		return v.owner
	}
	return nil
}

type initializationOperation struct {
	openrtc.Async[openrtc.RTCProvider]
	inspection fakeOwnerInspection
}

func newInitialization(owner any, result openrtc.Async[openrtc.RTCProvider]) openrtc.InitializationOperation {
	return initializationOperation{Async: result, inspection: fakeOwnerInspection{owner: owner, caps: newFakeCapabilities(NoDefects), descriptor: (bindingAdapter{}).Descriptor()}}
}
func (i initializationOperation) Inspection() openrtc.OperationInspectionAccess { return i.inspection }
func (i initializationOperation) Cleanup() openrtc.CleanupReport                { return i.inspection.Cleanup() }
func (i initializationOperation) WatchCleanup(f func(openrtc.CleanupReport)) openrtc.Unsubscribe {
	return i.inspection.WatchCleanup(f)
}
