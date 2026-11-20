package selftest

import (
	"context"

	"github.com/openrtc/conformance/openrtc"
)

func (s *fakeSession) setAdmission(next openrtc.AdmissionStatus) {
	s.apply(func() { s.admission = next }, []sessionMutationEvent{{key: "admissionChanged", payload: func(prev, cur openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.AdmissionStatus]{Previous: openrtc.KnownValue(prev.Connection.Admission), Current: cur.Connection.Admission}
	}}})
}
func (s *fakeSession) setCleanup(next openrtc.CleanupStatus) {
	s.apply(func() { s.cleanup = next }, []sessionMutationEvent{{key: "cleanupChanged", payload: func(prev, cur openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.CleanupStatus]{Previous: openrtc.KnownValue(prev.Cleanup), Current: cur.Cleanup}
	}}})
	notifyCleanup(s)
	if s.client != nil {
		notifyCleanup(s.client)
		if s.client.provider != nil {
			notifyCleanup(s.client.provider)
		}
	}
}
func (s *fakeSession) credentialRequest(phase openrtc.CredentialPhase, reason string) openrtc.CredentialRequest {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	address := s.resolvedAddress
	if address == nil {
		address = &openrtc.SessionAddress{AdapterID: adapterID, Namespace: "fake", Value: s.addressValue, Deployment: openrtc.Unknown[openrtc.DeploymentID]()}
	}
	return openrtc.CredentialRequest{AdapterID: adapterID, Audience: "participant", Subject: s.client.identity, Scope: openrtc.CredentialScope{Kind: "session-admission", Address: address, Actions: []openrtc.CapabilityReference{{Kind: "portable", ID: "session.conference"}}}, Phase: phase, Reason: reason}
}
func (s *fakeSession) requestCredential(ctx context.Context, phase openrtc.CredentialPhase, reason string, renew bool) (openrtc.Credential, error) {
	s.stateMu.Lock()
	s.credentialRevision++
	s.stateMu.Unlock()
	request := s.credentialRequest(phase, reason)
	var input openrtc.CredentialInput
	var err error
	if provider, ok := s.client.credentialProvider.(openrtc.CredentialRenewer); renew && ok && s.credentials.Metadata.Status == "known" {
		input, err = provider.Renew(ctx, *s.credentials.Metadata.Value, request).Result(ctx)
	} else {
		input, err = s.client.credentialProvider.Get(ctx, request).Result(ctx)
	}
	credential := openrtc.Credential{Material: input.Material, Metadata: openrtc.CredentialMetadata{ID: openrtc.CredentialID(newID("credential")), Audience: input.Metadata.Audience, Scope: input.Metadata.Scope, Subject: input.Metadata.Subject, IssuedAt: input.Metadata.IssuedAt, ExpiresAt: input.Metadata.ExpiresAt}}
	if err == nil {
		credential.Material.Use(func(bytes []byte) any { s.lastCredentialText = string(bytes); return nil })
	}
	return credential, err
}
func (s *fakeSession) resolveTarget(ctx context.Context) error {
	p := s.target.Provisioning
	var address *openrtc.SessionAddress
	if p.Mode == "backend-brokered" {
		resolved, err := p.Broker.Resolve(ctx, *p.Request).Result(ctx)
		if err != nil {
			return fail("provider-failure", "join", "the session broker failed", failureExtras{cause: err, original: err})
		}
		if resolved.Mode == "backend-brokered" {
			return fail("invalid-option", "join", "a broker may not resolve to another broker", failureExtras{})
		}
		if resolved.Mode == "explicit-create" {
			address = &resolved.Definition.Address
		} else {
			address = resolved.Address
		}
	} else if p.Mode == "explicit-create" {
		address = &p.Definition.Address
	} else {
		address = p.Address
	}
	if address == nil || address.AdapterID != adapterID {
		return fail("invalid-option", "join", "the resolved address belongs to another adapter", failureExtras{})
	}
	s.stateMu.Lock()
	s.resolvedAddress = address
	s.addressValue = address.Value
	s.stateMu.Unlock()
	return nil
}
func (s *fakeSession) participantFor(remote *remote) openrtc.RTCParticipant {
	return openrtc.RTCParticipant{NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), DisplayName: openrtc.Unknown[string](), Metadata: openrtc.Unknown[openrtc.ParticipantMetadata](), ID: openrtc.ParticipantConnectionID(newID("participant")), Identity: openrtc.ParticipantIdentity{Application: openrtc.KnownValue(remote.identity), Provider: openrtc.Unknown[openrtc.ProviderIdentity](), DeviceID: openrtc.Unknown[openrtc.ScopedNativeID](), MembershipIDs: []openrtc.ScopedNativeID{{Value: "member-" + remote.identity.Subject, AdapterID: adapterID, Namespace: "fake-membership", Deployment: openrtc.Unknown[openrtc.DeploymentID](), Origin: "adapter", Lifetime: "generation", Generation: openrtc.KnownValue(s.generation)}}}, Presence: "present", NativeRole: openrtc.Unknown[string](), Publications: []openrtc.PublicationID{}}
}

// admitHeld is the test control for admitting a held session.
func (s *fakeSession) admitHeld() { s.setAdmission("admitted"); close(s.admissionReady) }

// interrupt is the test control for transport loss and restoration.
func (s *fakeSession) interrupt() func(context.Context) error {
	s.apply(func() {
		s.state = "recovering"
		s.recovery = openrtc.KnownValue(openrtc.RecoveryStatus{Planes: []string{"signaling"}, NativeState: openrtc.Unknown[string](), NativeReason: openrtc.Unknown[string](), Cause: "transport-failure", Continuity: "unknown"})
	}, []sessionMutationEvent{{key: "connectionChanged", payload: func(prev, cur openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.ConnectionSnapshot]{Previous: openrtc.KnownValue(prev.Connection), Current: cur.Connection}
	}}})
	return func(context.Context) error { s.setState(openrtc.ConnectionState("connected")); return nil }
}
func (s *fakeSession) Configure(_ context.Context, command openrtc.SessionRuntimeCommand) openrtc.Async[openrtc.Void] {
	return openrtc.Discard(startOperation(s, func() (openrtc.ConfigurationResult, error) {
		if s.disposed {
			return openrtc.ConfigurationResult{}, fail("invalid-state", "configure", "the session was disposed", failureExtras{})
		}
		if command.Extensions != nil {
			if err := validateExtensions(command.Extensions, "runtime", "configure"); err != nil {
				return openrtc.ConfigurationResult{}, err
			}
		}
		return emptyConfigurationResult(nil), nil

	}))
}

// renewCredentials is the test control for one credential renewal.
func (s *fakeSession) renewCredentials(ctx context.Context) {
	credential, err := s.requestCredential(ctx, "replacement", "expiring", true)
	event := sessionMutationEvent{key: "credentialChanged", payload: func(prev, cur openrtc.SessionSnapshot) any {
		return openrtc.StateChange[openrtc.CredentialView]{Previous: openrtc.KnownValue(prev.Credentials[0]), Current: cur.Credentials[0]}
	}}
	if err == nil {
		s.apply(func() {
			s.credentials.State = "valid"
			s.credentials.Metadata = openrtc.KnownValue(credential.Metadata)
			s.credentials.Replacement = openrtc.KnownValue(openrtc.CredentialResult{Kind: "applied-live", Metadata: &credential.Metadata})
		}, []sessionMutationEvent{event})
		return
	}
	observation := fail("credential-failed", "credential-renew", "credential renewal failed", failureExtras{kind: "observation", cause: err, original: err})
	s.apply(func() { s.credentials.RenewalError = openrtc.KnownValue(observation) }, []sessionMutationEvent{event, {key: "error", payload: func(_, _ openrtc.SessionSnapshot) any { return observation }}})
	if s.defects.RenewalFailureDisconnects {
		s.setState(openrtc.ConnectionState("ended"))
	}
}

// credentialViews is called under the session state lock while building a commit.
func (s *fakeSession) credentialViews() []openrtc.CredentialView {
	application := "unknown"
	if s.credentials.Metadata.Status == "known" {
		application = "acknowledged"
	}
	if s.credentials.Replacement.Value != nil && s.credentials.Replacement.Value.Kind == "cached-for-reconnect" {
		application = "cached-for-reconnect"
	}
	failure := openrtc.NotApplicable[openrtc.SerializedRTCError]("no credential failure")
	if s.credentials.RenewalError.Value != nil {
		failure = openrtc.KnownValue((*s.credentials.RenewalError.Value).ToJSON())
	}
	view := openrtc.CredentialView{Owner: openrtc.LocalOwnerID(s.id), Channel: "admission", RequestRevision: s.credentialRevision, Metadata: s.credentials.Metadata, State: s.credentials.State, Validity: s.credentials.Validity, Application: application, Error: failure}
	if s.defects.CredentialLeak {
		view.LeakedForSelfTest = s.lastCredentialText
	}
	return []openrtc.CredentialView{view}
}
func (*fakeProvider) Credentials() openrtc.CredentialSnapshot {
	return openrtc.CredentialSnapshot{Channels: []openrtc.CredentialView{}}
}
func (*fakeClient) Credentials() openrtc.CredentialSnapshot {
	return openrtc.CredentialSnapshot{Channels: []openrtc.CredentialView{}}
}
func (*fakeAdministration) Credentials() openrtc.CredentialSnapshot {
	return openrtc.CredentialSnapshot{Channels: []openrtc.CredentialView{}}
}
func (i fakeOwnerInspection) credentialChannels() []openrtc.CredentialChannelDescriptor {
	if i.session == nil {
		return []openrtc.CredentialChannelDescriptor{}
	}
	s := i.session
	return []openrtc.CredentialChannelDescriptor{{Owner: openrtc.LocalOwnerID(s.id), Channel: "admission", Audience: "participant", Scope: s.credentialRequest("admission", "initial").Scope, Lifecycle: initialCredentials().Lifecycle}}
}

func (s *fakeSession) instanceView() openrtc.Known[openrtc.SessionInstance] {
	if !s.nativeJoined {
		return openrtc.Unknown[openrtc.SessionInstance]()
	}
	address := openrtc.SessionAddress{}
	if s.resolvedAddress != nil {
		address = *s.resolvedAddress
	}
	return openrtc.KnownValue(openrtc.SessionInstance{ID: s.instanceID, NativeID: openrtc.Unknown[openrtc.ScopedNativeID](), Address: address, DefinitionID: openrtc.Unknown[openrtc.SessionDefinitionID](), Status: "active"})
}
func (p *fakeProvider) ExportDefinition(id openrtc.SessionDefinitionID) (openrtc.SessionDefinitionDescriptor, error) {
	store := storeFor(p)
	store.mu.Lock()
	defer store.mu.Unlock()
	definition, ok := store.definitions[id]
	if !ok {
		return openrtc.SessionDefinitionDescriptor{}, fail("invalid-state", "export-definition", "unknown definition", failureExtras{})
	}
	return openrtc.SessionDefinitionDescriptor{NativeID: definition.NativeID, Address: definition.Address, Allocation: definition.Allocation, Policy: definition.Policy}, nil
}
func (p *fakeProvider) ImportDefinition(_ context.Context, descriptor openrtc.SessionDefinitionDescriptor) openrtc.Async[openrtc.SessionDefinition] {
	return startOperation(p, func() (openrtc.SessionDefinition, error) {
		if descriptor.Address.AdapterID != adapterID {
			return openrtc.SessionDefinition{}, fail("invalid-option", "import-definition", "foreign definition", failureExtras{})
		}
		return openrtc.SessionDefinition{ID: openrtc.SessionDefinitionID(newID("definition")), NativeID: descriptor.NativeID, Address: descriptor.Address, Allocation: descriptor.Allocation, Policy: descriptor.Policy}, nil
	})
}
