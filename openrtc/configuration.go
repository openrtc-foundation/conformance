package openrtc

// ConfigurationLifetime identifies one of the five portable binding lifetimes (CFG-001).
type ConfigurationLifetime string

const (
	ProviderInit    ConfigurationLifetime = "provider-init"
	ClientInit      ConfigurationLifetime = "client-init"
	SessionCreate   ConfigurationLifetime = "session-create"
	ParticipantJoin ConfigurationLifetime = "participant-join"
	Runtime         ConfigurationLifetime = "runtime"
)

// JSONValue is JSON data used at the schema validation boundary (CFG-003).
type JSONValue any
type JSONObject map[string]JSONValue

type JSONSchemaDocument struct {
	Dialect  string     `json:"dialect"`
	Document JSONObject `json:"document"`
}

// ValidatedExtensionPayload is produced by an adapter's validator (CFG-002–003).
type ValidatedExtensionPayload struct {
	AdapterID      AdapterID         `json:"-"`
	Lifetime       ExtensionLifetime `json:"lifetime"`
	SchemaID       string            `json:"schemaId"`
	SchemaRevision string            `json:"revision"`
	Payload        any               `json:"payload"`
}

// ProviderExtensionLifetimeMap holds adapter-validated payloads by lifetime (CFG-001–003).
type ProviderExtensionLifetimeMap map[ExtensionLifetime]ValidatedExtensionPayload

// ProviderExtensionTypes is the adapter namespace registry (CFG-003).
type ProviderExtensionTypes map[AdapterID]ProviderExtensionLifetimeMap

// ProviderExtensions contains adapter-keyed native configuration (CFG-002–003).
type ProviderExtensions map[AdapterID]ValidatedExtensionPayload
type ValidatedExtensionCommand struct {
	SchemaID string    `json:"schemaId"`
	Revision string    `json:"revision"`
	Command  string    `json:"command"`
	Payload  JSONValue `json:"payload"`
}

// DefaultDescription records whether a default is known (CFG-005).
type DefaultDescription struct {
	Source string           `json:"source"`
	Value  Known[JSONValue] `json:"value"`
}

// ConfigurationBinding describes lifetime, native binding and activation (CFG-001, CFG-004–005).
type ConfigurationBinding struct {
	Path            OptionPath            `json:"path"`
	Lifetime        ConfigurationLifetime `json:"lifetime"`
	Authority       AuthorityPlane        `json:"authority"`
	NativeOwner     string                `json:"nativeOwner"`
	Required        Known[bool]           `json:"required"`
	DefaultValue    DefaultDescription    `json:"defaultValue"`
	UpdateMechanism string                `json:"updateMechanism"`
	EffectiveAfter  string                `json:"effectiveAfter"`
	Mutability      string                `json:"mutability"`
	Evidence        []EvidenceReference   `json:"evidence"`
}

// ConfigurationSchema validates native data at the adapter boundary (CFG-003).
type ConfigurationSchema interface {
	AdapterID() AdapterID
	SchemaID() string
	Revision() string
	Lifetime() ExtensionLifetime
	JSONSchema() JSONSchemaDocument
	Bindings() []ConfigurationBinding
	Validate(input any) (ValidatedExtensionPayload, error)
}

type ConfigurationFieldResult struct {
	Path       OptionPath       `json:"path"`
	Requested  Known[JSONValue] `json:"requested"`
	Accepted   Known[JSONValue] `json:"accepted"`
	Effective  Known[JSONValue] `json:"effective"`
	Activation string           `json:"activation"`
	Selection  string           `json:"selection"`
	Reason     string           `json:"reason"`
}

type ConfigurationResult struct {
	Schemas   []SchemaReference          `json:"schemas"`
	Fields    []ConfigurationFieldResult `json:"fields"`
	Placement []PlacementObservation     `json:"placement"`
	Effects   MediaOperationEffects      `json:"effects"`
}

type DiagnosticsPolicy struct {
	Level string `json:"level"`
}
type DiagnosticsPatch = DiagnosticsPolicy

// PlacementPlane separates independent placement planes (CFG-006).
type PlacementPlane string

const (
	PlacementSignaling           PlacementPlane = "signaling"
	PlacementMedia               PlacementPlane = "media"
	PlacementRecordingProcessing PlacementPlane = "recording-processing"
	PlacementRecordingStorage    PlacementPlane = "recording-storage"
	PlacementDeployment          PlacementPlane = "deployment"
	PlacementData                PlacementPlane = "data"
)

type RegionRef struct {
	Namespace string `json:"namespace"`
	Code      string `json:"code"`
}
type PlacementBoundary struct {
	Resources []string `json:"resources"`
	Transit   string   `json:"transit"`
}
type PlacementRequirement struct {
	Strength string            `json:"strength"`
	Regions  []RegionRef       `json:"regions"`
	Boundary PlacementBoundary `json:"boundary"`
}
type PlacementConstraint = PlacementRequirement

type PlacementPolicy struct {
	SignalingRegion           *PlacementConstraint `json:"signalingRegion,omitempty"`
	MediaRegion               *PlacementConstraint `json:"mediaRegion,omitempty"`
	RecordingProcessingRegion *PlacementConstraint `json:"recordingProcessingRegion,omitempty"`
	RecordingStorageRegion    *PlacementConstraint `json:"recordingStorageRegion,omitempty"`
	DeploymentRegion          *PlacementConstraint `json:"deploymentRegion,omitempty"`
	DataRegion                *PlacementConstraint `json:"dataRegion,omitempty"`
}
type PlacementObservation struct {
	Selection   string              `json:"selection"`
	Reason      string              `json:"reason"`
	Plane       PlacementPlane      `json:"plane"`
	Requested   PlacementConstraint `json:"requested"`
	Enforcement string              `json:"enforcement"`
	Effective   Known[[]RegionRef]  `json:"effective"`
	Observed    Known[[]RegionRef]  `json:"observed"`
	Authority   AuthorityPlane      `json:"authority"`
	Evidence    []EvidenceReference `json:"evidence"`
}

// OperationEffects records independent cross-domain effects (CFG-004, ERR-001).
type OperationEffects struct {
	Media     []MediaOperationEffects `json:"media"`
	Resources []ResourceEffect        `json:"resources"`
}
type MediaOperationEffects struct {
	Guaranteed MediaEffectValues  `json:"guaranteed"`
	Observed   MediaEffectValues  `json:"observed"`
	Collateral []MediaEffectField `json:"collateral"`
}
type RecordingEffect string
type ResourceEffect struct {
	Resource ResourceRef `json:"resource"`
	Effect   string      `json:"effect"`
}

// Pre-effect acceptance equivalents: RFC §7.1:86–88, A.1:1211–1216.
type EffectTarget struct {
	Resource    *ResourceRef     `json:"resource,omitempty"`
	Scope       *CapabilityScope `json:"scope,omitempty"`
	Description string           `json:"description,omitempty"`
}
type Effect struct {
	Target    EffectTarget `json:"target"`
	Action    string       `json:"action"`
	Certainty string       `json:"certainty"`
}
type EffectAcceptance = Effect
type Condition struct {
	Name       string            `json:"name"`
	Scope      CapabilityScope   `json:"scope"`
	Parameters map[string]string `json:"parameters"`
}
type OperationOptions struct {
	Conditions      []Condition        `json:"conditions"`
	AcceptedEffects []EffectAcceptance `json:"acceptedEffects"`
	Deadline        *Instant           `json:"deadline,omitempty"`
}
