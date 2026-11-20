package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// These optional fixture mappings invoke actual public operations and qualify
// independent observation boundaries. They are test inputs, not production APIs.
// Missing facilities/evidence leave their applicable assertions incomplete.
type MessageScenarioTarget interface {
	MessageScenario(context.Context) (MessageScenario, error)
}
type MessageRecipients struct {
	Kind               string
	IDs                []string
	Scope              string
	MembershipBoundary string
}
type MessageEvent struct {
	Kind       string
	Sender     openrtc.Known[string]
	Recipients MessageRecipients
	Payload    []byte
}
type MessageSubmission struct {
	State      string
	Recipients MessageRecipients
}
type MessageScenario struct {
	Documentation         string
	Modes                 []MessageRecipients
	MaximumPayloadOctets  openrtc.Known[int]
	Send                  func(context.Context, MessageRecipients, []byte) (MessageSubmission, error)
	Observe               func(func(MessageEvent)) (func() error, error)
	PeerSend              func(context.Context, MessageRecipients, []byte) error
	LoseDelivery          func(context.Context) error
	SetOnline             func(context.Context, bool) error
	Checkpoint            func(context.Context) error
	RecipientPayloads     func() [][]byte
	DefaultSurfaces       func() []any
	SubmissionCount       func() int
	NextSubmissionUnknown func()
}
type DeviceScenarioTarget interface {
	DeviceScenario(context.Context) (DeviceScenario, error)
}
type DeviceScenario struct {
	Documentation    string
	Manager          openrtc.MediaDeviceManager
	Kind             openrtc.MediaKind
	Selection        openrtc.DeviceSelection
	Effects          NativeEffectObserver
	Statistics       openrtc.MediaStatistics
	StatisticsTarget openrtc.StatisticsTarget
	// CachedSample repeats the same provider sample rather than initiating a
	// fresh collection. nil means cached-time evidence is unavailable.
	CachedSample func(context.Context) (openrtc.StatsSample, error)
}
type RenewalScenarioTarget interface {
	RenewalScenario(context.Context) (RenewalScenario, error)
}
type RenewalResult struct {
	State        string
	Boundary     string
	Acknowledged bool
}
type RenewalScenario struct {
	Documentation string
	Renew         func(context.Context, string) (RenewalResult, error)
	// Candidate identifiers are fixture labels for separate authorized secret
	// channel inputs; they never expose secret material in public observations.
	First, Second, Mismatched string
	SetAcknowledgementState   func(string)
	HoldNext                  func() JoinGate
	Installed                 func() openrtc.Known[string]
	SubmissionCount           func() int
	PublicConnected           func() openrtc.Known[bool]
	FailNext                  func()
	Checkpoint                func(context.Context) error
}
type AdministrationScenarioTarget interface {
	AdministrationScenario(context.Context) (AdministrationScenario, error)
}
type ConferenceFacts struct{ ID, Owner, State string }
type AdministrationRelease struct {
	Scope       string
	RemoteState openrtc.Known[string]
}
type CredentialFacts struct {
	ID                                      string
	Audience, Subject, Destination, Purpose string
	AuthorizedChannel                       bool
}
type AdministrationScenario struct {
	Documentation   string
	Server          string
	Create          func(context.Context, string) (ConferenceFacts, error)
	End             func(context.Context, string, string) (AdministrationRelease, error)
	Remove          func(context.Context, string, string, string) (AdministrationRelease, error)
	Mint            func(context.Context, string, CredentialFacts) (CredentialFacts, error)
	Populate        func(context.Context, string) (string, error)
	Conference      func(string) openrtc.Known[string]
	Participant     func(string) openrtc.Known[string]
	DefaultSurfaces func() []any
	Checkpoint      func(context.Context) error
}
type VersionScenarioTarget interface {
	VersionScenario(context.Context) (VersionScenario, error)
}
type VersionResult struct {
	Operation string
	Resource  string
	State     string
}
type VersionScenario struct {
	Documentation      string
	Profile, Namespace string
	SupportedVersion   int
	Execute            func(context.Context, int, string) (VersionResult, error)
	Effects            NativeEffectObserver
}
