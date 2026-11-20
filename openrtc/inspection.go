package openrtc

// ConfigurationOutcome is the safe, retained projection of configuration consumption.
type ConfigurationOutcome = ConfigurationResult
type ConfigurationReceipt struct {
	Outcome ConfigurationOutcome `json:"outcome"`
}
type SchemaDiscovery struct {
	Lifetime  ConfigurationLifetime `json:"lifetime"`
	Documents []SchemaDocument      `json:"documents"`
	Selected  *SchemaReference      `json:"selected,omitempty"`
}
type SchemaReference struct {
	AdapterID AdapterID             `json:"adapterId"`
	SchemaID  string                `json:"schemaId"`
	Revision  string                `json:"revision"`
	Lifetime  ConfigurationLifetime `json:"lifetime"`
}
type OwnerDiscovery struct {
	Revision           Revision                      `json:"revision"`
	Media              Known[MediaControlModel]      `json:"media"`
	Placement          Known[[]PlacementSupport]     `json:"placement"`
	CredentialChannels []CredentialChannelDescriptor `json:"credentialChannels"`
	Extensions         []ExtensionManifest           `json:"extensions"`
	Schemas            []SchemaDiscovery             `json:"schemas"`
	CallingPolicy      *CallingPolicyObservation     `json:"callingPolicy,omitempty"`
}
type OwnerInspectionSnapshot struct {
	Revision      Revision             `json:"revision"`
	Descriptor    ProviderDescriptor   `json:"descriptor"`
	Configuration ConfigurationOutcome `json:"configuration"`
	Discovery     OwnerDiscovery       `json:"discovery"`
	Capabilities  CapabilitySnapshot   `json:"capabilities"`
	Cleanup       CleanupReport        `json:"cleanup"`
}
type SessionInspectionSnapshot struct {
	OwnerInspectionSnapshot
	ControlUnits    map[ControlUnitID]MediaControlUnit `json:"controlUnits"`
	SourceFreshness []SourceFreshness                  `json:"sourceFreshness"`
	Consistency     string                             `json:"consistency"`
}
type MediaControlUnit struct {
	ID              ControlUnitID             `json:"id"`
	Granularity     string                    `json:"granularity"`
	Domain          string                    `json:"domain"`
	Resources       []ResourceRef             `json:"resources"`
	Operations      []string                  `json:"operations"`
	AdapterCommands []AdapterCommandReference `json:"adapterCommands"`
}
type AdapterCommandReference struct {
	AdapterID AdapterID `json:"adapterId"`
	CommandID string    `json:"commandId"`
}
type InitialMediaOutcome struct {
	Operations []InitialMediaOperationOutcome `json:"operations"`
}
type InitialMediaOperationOutcome struct {
	ID        string                    `json:"id"`
	Status    string                    `json:"status"`
	Effects   MediaOperationEffects     `json:"effects"`
	Error     Known[SerializedRTCError] `json:"error"`
	Kind      string                    `json:"kind"`
	Source    Known[CaptureSourceID]    `json:"source,omitzero"`
	Resources *MediaResources           `json:"resources,omitempty"`
}
type OperationInspection struct {
	Dtmf          Known[DtmfResult]            `json:"dtmf"`
	OperationID   OperationID                  `json:"operationId"`
	Configuration Known[ConfigurationReceipt]  `json:"configuration"`
	Effects       Known[MediaOperationEffects] `json:"effects"`
	Negotiation   Known[NegotiationResult]     `json:"negotiation"`
	InitialMedia  Known[InitialMediaOutcome]   `json:"initialMedia"`
}
type OperationInspectionResult struct {
	Status      string               `json:"status"`
	Details     *OperationInspection `json:"details,omitempty"`
	OperationID OperationID          `json:"operationId,omitempty"`
}
type OperationInspectionAccess interface {
	Operation(OperationID) (OperationInspectionResult, error)
}

// InspectionCommon is repeated by the owner and session projections because Go
// interfaces cannot override Snapshot and Watch with covariant result types.
type InspectionCommon interface {
	OperationInspectionAccess
	Capabilities() RTCCapabilities
	Schemas(ConfigurationLifetime) SchemaDiscovery
	ResourceConfiguration(SessionDefinitionID) (ConfigurationOutcome, error)
	Cleanup() CleanupReport
	WatchCleanup(func(CleanupReport)) Unsubscribe
}
type OwnerInspection interface {
	InspectionCommon
	Snapshot() OwnerInspectionSnapshot
	Watch(func(OwnerInspectionSnapshot)) Unsubscribe
}
type SessionInspection interface {
	InspectionCommon
	Snapshot() SessionInspectionSnapshot
	Watch(func(SessionInspectionSnapshot)) Unsubscribe
	PublicationEffects(PublicationID) MediaOperationEffects
}

// InitializationOperation retains inspection even if initialization produces no provider.
type InitializationOperation interface {
	Async[RTCProvider]
	Inspection() OperationInspectionAccess
	Cleanup() CleanupReport
	WatchCleanup(func(CleanupReport)) Unsubscribe
}
