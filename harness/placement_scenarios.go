package harness

import (
	"context"
	"github.com/openrtc/conformance/openrtc"
)

// PlacementScenario is independent fixture input and evidence. It does not add
// runtime options or production instrumentation requirements (§§11.7,14.2).
type PlacementScenarioTarget interface {
	PlacementScenario(context.Context) (PlacementScenario, error)
}
type PlacementScenario struct {
	Documentation                        string
	Scope, Activity, Direction, Boundary string
	Regions                              []openrtc.RegionRef
	Control                              openrtc.CurrentPlacementControl
	Effects                              NativeEffectObserver
	Snapshot                             func() openrtc.CurrentPlacementResult
	SetRequiredAssessment                func(string)
	SetCombinationAssessment             func(string)
	Recover                              func(context.Context) error
	WithdrawVerification                 func(context.Context) error
	Checkpoint                           func(context.Context) error
	OutsideEffects                       func() int
	SessionIncarnation                   func() string
	RuntimeApplySupported                bool
}
