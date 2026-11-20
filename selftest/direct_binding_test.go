package selftest

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// directImplementation has only actual application operations and observation
// state. It has no production Registry/Provider/Adapter, native identifiers or
// inspection ledger. Its unusual state syntax is projected by fixture metadata.
type directImplementation struct {
	mu            sync.Mutex
	serial        int
	integerStates bool
	defect        string
}
type directClient struct{ available bool }
type directSession struct {
	id        string
	state     any
	observers map[int]func(harness.PublicSessionFacts)
	next      int
}
type directTarget struct {
	*FakeTarget
	implementation *directImplementation
}

func (t *directTarget) CreateRegistry() openrtc.RTCAdapterRegistry {
	panic("direct implementation has no production registry")
}
func (t *directTarget) CreateAdapter() openrtc.RTCAdapter {
	panic("direct implementation has no production adapter")
}
func (t *directTarget) PublicBinding() harness.PublicBindingMapping {
	a := t.implementation
	state := func(closed bool) any {
		if a.integerStates {
			if closed {
				return 9
			}
			return 3
		}
		if closed {
			return "off-air"
		}
		return "on-air"
	}
	project := func(s *directSession) harness.PublicSessionFacts {
		terminal := s.state == state(true)
		return harness.PublicSessionFacts{Incarnation: s.id, Connected: !terminal, Terminal: terminal, Reason: map[bool]string{true: "left", false: ""}[terminal], Coverage: "local-participation"}
	}
	return harness.PublicBindingMapping{
		Documentation: "selftest/direct binding mapping v1; literal on-air/off-air or 3/9 state projection",
		Operations:    map[string]string{"createClient": "newEndpoint", "join": "enterRoom", "capabilities": "features", "snapshot": "current", "observe": "listen", "unsubscribe": "removeListener", "leave": "exitRoom", "dispose": "closeEndpoint"},
		Descriptor:    openrtc.ProviderDescriptor{AdapterID: "direct-binding-fixture", Artifact: "direct-selftest"},
		CreateClient:  func(context.Context) (any, error) { return &directClient{available: a.defect != "not-ready"}, nil },
		Ready:         func(h any) (bool, error) { a.mu.Lock(); defer a.mu.Unlock(); return h.(*directClient).available, nil },
		Join: func(_ context.Context, h any) (any, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			if !h.(*directClient).available {
				return nil, fmt.Errorf("endpoint closed")
			}
			a.serial++
			s := &directSession{id: fmt.Sprintf("incarnation-%d", a.serial), state: state(false), observers: map[int]func(harness.PublicSessionFacts){}}
			if a.defect == "join-terminal" {
				s.state = state(true)
			}
			return s, nil
		},
		Capabilities: func(any) ([]openrtc.CapabilityDescriptor, error) { return []openrtc.CapabilityDescriptor{}, nil },
		Snapshot: func(h any) (harness.PublicSessionFacts, error) {
			a.mu.Lock()
			defer a.mu.Unlock()
			return project(h.(*directSession)), nil
		},
		Observe: func(h any, callback func(harness.PublicSessionFacts)) (func() error, error) {
			a.mu.Lock()
			s := h.(*directSession)
			s.next++
			id := s.next
			s.observers[id] = callback
			initial := project(s)
			a.mu.Unlock()
			callback(initial)
			return func() error {
				a.mu.Lock()
				defer a.mu.Unlock()
				if a.defect != "removal-ignored" {
					delete(s.observers, id)
				}
				return nil
			}, nil
		},
		Leave: func(_ context.Context, h any) error {
			a.mu.Lock()
			s := h.(*directSession)
			if s.state == state(true) {
				a.mu.Unlock()
				return nil
			}
			if a.defect == "leave-not-terminal" {
				a.mu.Unlock()
				return nil
			}
			s.state = state(true)
			v := project(s)
			callbacks := make([]func(harness.PublicSessionFacts), 0, len(s.observers))
			for _, f := range s.observers {
				callbacks = append(callbacks, f)
			}
			a.mu.Unlock()
			for _, f := range callbacks {
				f(v)
			}
			return nil
		},
		Dispose: func(_ context.Context, h any) error {
			a.mu.Lock()
			defer a.mu.Unlock()
			h.(*directClient).available = false
			return nil
		},
	}
}

// RFC §§2:13, 5:27, 7.0:45–65, 14.2:1109 and A.0:1174: syntax and host
// architecture are not requirements; actual public postconditions remain so.
func TestDirectBindingEquivalentCoreWithoutProductionHost(t *testing.T) {
	for _, integers := range []bool{false, true} {
		t.Run(fmt.Sprintf("integer-states-%v", integers), func(t *testing.T) {
			target := &directTarget{MakeFakeTarget(FakeDefects{}), &directImplementation{integerStates: integers}}
			tc := harness.NewTestContext(target, harness.TargetDescriptor(target))
			test := harness.PublicBindingTest()
			if err := test.Run(context.Background(), tc); err != nil {
				t.Fatal(err)
			}
			if failures := tc.Cleanup(context.Background()); len(failures) > 0 {
				t.Fatal(failures)
			}
		})
	}
}

// RFC §§7.0:45–65,14.2:1109; A.0:1174: equivalence does not waive public postconditions.
func TestDirectBindingOracleRejectsDefectiveProjection(t *testing.T) {
	for _, defect := range []string{"not-ready", "join-terminal", "leave-not-terminal", "removal-ignored"} {
		t.Run(defect, func(t *testing.T) {
			target := &directTarget{MakeFakeTarget(FakeDefects{}), &directImplementation{defect: defect}}
			tc := harness.NewTestContext(target, harness.TargetDescriptor(target))
			if err := harness.PublicBindingTest().Run(context.Background(), tc); err == nil {
				t.Fatal("defective direct implementation passed public oracle")
			}
			tc.Cleanup(context.Background())
		})
	}
}

// RFC §14.2:1109 and A.0:1174: even the runner/probe must not require
// production factories from a direct implementation with an equivalent map.
func TestDirectBindingRunnerUsesNoProductionFactories(t *testing.T) {
	target := &directTarget{MakeFakeTarget(FakeDefects{}), &directImplementation{}}
	report := harness.RunConformance(context.Background(), target, []harness.ConformanceTest{harness.PublicBindingTest()}, harness.RunOptions{})
	for _, result := range report.Results {
		if result.TestID == "CT-PUBLIC-001" {
			if result.Status != harness.Pass {
				t.Fatalf("mapped public test: %s %s", result.Status, result.Reason)
			}
			return
		}
	}
	t.Fatal("direct public assertion omitted")
}
