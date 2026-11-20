package selftest

import (
	"encoding/json"
	"github.com/openrtc/conformance/openrtc"
)

func unknownEffects() openrtc.MediaOperationEffects {
	v := openrtc.MediaEffectValues{Capture: openrtc.Unknown[string](), Outbound: openrtc.Unknown[string](), Publication: openrtc.Unknown[string](), Receive: openrtc.Unknown[string](), ReacquisitionRequired: openrtc.Unknown[string](), Recording: openrtc.Unknown[string](), Playback: openrtc.Unknown[string]()}
	return openrtc.MediaOperationEffects{Guaranteed: v, Observed: v, Collateral: []openrtc.MediaEffectField{}}
}

type fakeOwnerInspection struct {
	owner      any
	caps       *fakeCapabilities
	session    *fakeSession
	descriptor openrtc.ProviderDescriptor
}

func (i fakeOwnerInspection) Operation(id openrtc.OperationID) (openrtc.OperationInspectionResult, error) {
	s := storeFor(i.owner)
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.records[id]; ok {
		if ownerDefects(i.owner).MediaInspectionWrongID {
			r.OperationID = "incorrect-operation"
		}
		data, err := json.Marshal(r)
		if err != nil {
			return openrtc.OperationInspectionResult{}, err
		}
		var immutable openrtc.OperationInspection
		if err = json.Unmarshal(data, &immutable); err != nil {
			return openrtc.OperationInspectionResult{}, err
		}
		return openrtc.OperationInspectionResult{Status: "retained", Details: &immutable}, nil
	}
	if s.evicted[id] {
		if ownerDefects(i.owner).InspectionEvictionError {
			return openrtc.OperationInspectionResult{}, fail("invalid-state", "inspect", "lost evicted ID", failureExtras{})
		}
		return openrtc.OperationInspectionResult{Status: "evicted", OperationID: id}, nil
	}
	if ownerDefects(i.owner).ForeignInspectionAccepted {
		return openrtc.OperationInspectionResult{Status: "evicted", OperationID: id}, nil
	}
	return openrtc.OperationInspectionResult{}, fail("invalid-state", "inspect", "operation ID was never issued by this owner", failureExtras{})
}
func (i fakeOwnerInspection) Capabilities() openrtc.RTCCapabilities { return i.caps }
func (i fakeOwnerInspection) Schemas(l openrtc.ConfigurationLifetime) openrtc.SchemaDiscovery {
	discovery := openrtc.SchemaDiscovery{Lifetime: l, Documents: []openrtc.SchemaDocument{}}
	for _, schema := range (bindingAdapter{}).Schemas(l) {
		discovery.Documents = append(discovery.Documents, openrtc.SchemaDocument{AdapterID: schema.AdapterID(), SchemaID: schema.SchemaID(), Revision: schema.Revision(), Lifetime: l, JSONSchema: schema.JSONSchema().Document, Bindings: schema.Bindings(), Commands: []openrtc.AdapterCommandDescriptor{}})
		discovery.Selected = &openrtc.SchemaReference{AdapterID: schema.AdapterID(), SchemaID: schema.SchemaID(), Revision: schema.Revision(), Lifetime: l}
	}
	return discovery
}
func (i fakeOwnerInspection) ResourceConfiguration(id openrtc.SessionDefinitionID) (openrtc.ConfigurationOutcome, error) {
	store := storeFor(i.owner)
	store.mu.Lock()
	defer store.mu.Unlock()
	if value, ok := store.resources[id]; ok {
		return value, nil
	}
	return openrtc.ConfigurationOutcome{}, fail("invalid-state", "inspect", "unknown definition", failureExtras{})
}
func (i fakeOwnerInspection) Cleanup() openrtc.CleanupReport {
	if i.session != nil {
		return detached(openrtc.CleanupReport{Status: i.session.Observation().Snapshot().Cleanup, Generation: openrtc.KnownValue(i.session.generation)})
	}
	if c, ok := i.owner.(*fakeClient); ok {
		c.mu.Lock()
		sessions := append([]*fakeSession(nil), c.sessionOrder...)
		c.mu.Unlock()
		for _, s := range sessions {
			r := s.Inspection().Cleanup()
			if r.Status.Kind != "complete" {
				return r
			}
		}
	}
	if p, ok := i.owner.(*fakeProvider); ok {
		p.mu.Lock()
		clients := append([]*fakeClient(nil), p.clients...)
		p.mu.Unlock()
		for _, c := range clients {
			r := c.Inspection().Cleanup()
			if r.Status.Kind != "complete" {
				return r
			}
		}
	}
	store := storeFor(i.owner)
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.cleanup != nil {
		return detached(*store.cleanup)
	}
	return completeCleanup()
}
func (i fakeOwnerInspection) WatchCleanup(f func(openrtc.CleanupReport)) openrtc.Unsubscribe {
	if f == nil {
		panic(fail("invalid-option", "watch-cleanup", "nil listener", failureExtras{}))
	}
	if ownerDisposed(i.owner) && ownerDefects(i.owner).CleanupWatchCloses {
		panic(fail("invalid-state", "watch-cleanup", "disposed", failureExtras{}))
	}
	st := storeFor(i.owner)
	st.mu.Lock()
	st.nextWatcher++
	id := st.nextWatcher
	st.cleanupWatchers[id] = func() { f(i.Cleanup()) }
	st.mu.Unlock()
	isolated(func() { f(i.Cleanup()) })
	return func() { st.mu.Lock(); delete(st.cleanupWatchers, id); st.mu.Unlock() }
}
func (i fakeOwnerInspection) Snapshot() openrtc.OwnerInspectionSnapshot {
	revision := i.caps.Snapshot().Revision
	media := openrtc.NotApplicable[openrtc.MediaControlModel]("owner has no media")
	if i.session != nil {
		revision = i.session.Observation().Snapshot().Revision
		media = openrtc.KnownValue(i.session.media.Model())
	}
	store := storeFor(i.owner)
	store.mu.Lock()
	configuration := emptyConfigurationResult(nil)
	if store.configuration != nil {
		configuration = *store.configuration
	}
	if i.session == nil {
		revision = store.revision
	}
	store.mu.Unlock()
	descriptor := i.descriptor
	if i.session != nil && i.session.defects.LeakInspectionCanary {
		i.session.media.mu.Lock()
		if len(i.session.media.canaries) > 0 {
			descriptor.SDKArtifact = i.session.media.canaries[0]
		}
		i.session.media.mu.Unlock()
	}
	return detached(openrtc.OwnerInspectionSnapshot{Revision: revision, Descriptor: descriptor, Configuration: configuration, Capabilities: i.caps.Snapshot(), Cleanup: i.Cleanup(), Discovery: openrtc.OwnerDiscovery{Revision: revision, Media: media, Placement: openrtc.KnownValue([]openrtc.PlacementSupport{}), Schemas: []openrtc.SchemaDiscovery{}, CredentialChannels: i.credentialChannels(), Extensions: []openrtc.ExtensionManifest{}}})
}
func (i fakeOwnerInspection) Watch(f func(openrtc.OwnerInspectionSnapshot)) openrtc.Unsubscribe {
	validateWatch(i.owner, f == nil)
	store := storeFor(i.owner)
	store.mu.Lock()
	store.nextWatcher++
	key := store.nextWatcher
	store.watchers[key] = func() { f(i.Snapshot()) }
	store.mu.Unlock()
	isolated(func() { f(i.Snapshot()) })
	unsubscribe := i.caps.OnChanged(func(openrtc.CapabilitySnapshot) { f(i.Snapshot()) })
	return func() { store.mu.Lock(); delete(store.watchers, key); store.mu.Unlock(); unsubscribe() }
}

type fakeSessionInspection struct{ fakeOwnerInspection }

func (i fakeSessionInspection) Snapshot() openrtc.SessionInspectionSnapshot {
	return openrtc.SessionInspectionSnapshot{OwnerInspectionSnapshot: i.fakeOwnerInspection.Snapshot(), ControlUnits: map[openrtc.ControlUnitID]openrtc.MediaControlUnit{}, SourceFreshness: []openrtc.SourceFreshness{}, Consistency: "best-effort"}
}
func (i fakeSessionInspection) Watch(f func(openrtc.SessionInspectionSnapshot)) openrtc.Unsubscribe {
	validateWatch(i.owner, f == nil)
	return i.session.Observation().Watch(func(openrtc.SessionSnapshot) { f(i.Snapshot()) })
}
func (i fakeSessionInspection) PublicationEffects(id openrtc.PublicationID) openrtc.MediaOperationEffects {
	m := i.session.media
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.effects[id]; ok {
		return detached(e)
	}
	panic(fail("invalid-state", "inspect", "unknown publication", failureExtras{}))
}
func (s *fakeSession) Inspection() openrtc.SessionInspection {
	return fakeSessionInspection{fakeOwnerInspection{owner: s, caps: s.capabilities, session: s, descriptor: s.client.providerDescriptor()}}
}
func (c *fakeClient) providerDescriptor() openrtc.ProviderDescriptor {
	return (bindingAdapter{}).Descriptor()
}
func (c *fakeClient) Inspection() openrtc.OwnerInspection {
	return fakeOwnerInspection{owner: c, caps: c.capabilities, descriptor: c.providerDescriptor()}
}
func (p *fakeProvider) Inspection() openrtc.OwnerInspection {
	return fakeOwnerInspection{owner: p, caps: p.capabilities, descriptor: p.descriptor}
}
func (a *fakeAdministration) Inspection() openrtc.OwnerInspection {
	return fakeOwnerInspection{owner: a, caps: a.capabilities, descriptor: (bindingAdapter{}).Descriptor()}
}

func fakeRecoveryPolicy() openrtc.RecoveryPolicy {
	return openrtc.RecoveryPolicy{Mechanism: "native-resume", Planes: []openrtc.PlacementPlane{openrtc.PlacementSignaling}, Continuity: map[string]openrtc.RecoveryContinuity{}, Control: "fixed", RetryBudget: openrtc.Unknown[uint64](), Deadline: openrtc.Unknown[openrtc.Duration](), Placement: []openrtc.RecoveryPlacement{}}
}
