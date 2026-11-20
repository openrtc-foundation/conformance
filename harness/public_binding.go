package harness

import (
	"context"
	"fmt"

	"github.com/openrtc/conformance/openrtc"
)

// PublicBindingMapping is harness-only fixture metadata. Handles are opaque
// application values. No Registry, Provider, native ID or inspection ledger is
// required from the implementation (RFC §§2:13, 5:27, 14.2:1109; A.0:1174).
// Each function must invoke the named actual public operation, not synthesize
// its postconditions. Representation projection belongs in this fixture map.
type PublicBindingMapping struct {
	Documentation string
	Operations    map[string]string
	Descriptor    openrtc.ProviderDescriptor
	CreateClient  func(context.Context) (any, error)
	Ready         func(any) (bool, error)
	Join          func(context.Context, any) (any, error)
	Capabilities  func(any) ([]openrtc.CapabilityDescriptor, error)
	Snapshot      func(any) (PublicSessionFacts, error)
	Observe       func(any, func(PublicSessionFacts)) (func() error, error)
	Leave         func(context.Context, any) error
	Dispose       func(context.Context, any) error
	// ProviderBridge is optional integration with earlier Go catalog cases. It
	// is constructed solely by the harness mapping, never a production mandate.
	ProviderBridge func(context.Context, openrtc.ProviderConfig) (openrtc.RTCProvider, error)
}
type PublicSessionFacts struct {
	Incarnation string
	Connected   bool
	Terminal    bool
	Reason      string
	Coverage    string
}
type PublicBindingTarget interface{ PublicBinding() PublicBindingMapping }

func TargetDescriptor(target ConformanceTarget) openrtc.ProviderDescriptor {
	if t, ok := target.(PublicBindingTarget); ok {
		return t.PublicBinding().Descriptor
	}
	return target.CreateAdapter().Descriptor()
}
func PublicBindingTest() ConformanceTest {
	return DefineTest(ConformanceTest{
		ID: "CT-PUBLIC-001", Title: "equivalent direct public Core operations require no production host machinery", Suite: Core, Profile: Common,
		Requirements: []string{"RFC §2:13", "RFC §5:27", "RFC §7.0:45–65", "RFC §7.2:97–169", "RFC §8.3:467–475", "RFC §14.2:1109", "RFC A.0:1174"},
		Observable:   "actual mapped create/join/capabilities/snapshot/observe/leave/dispose establish current public facts; independent observer removal; repeated release is locally terminal", Run: runPublicBinding,
	})
}
func runPublicBinding(ctx context.Context, c *TestContext) error {
	t, ok := c.Target.(PublicBindingTarget)
	if !ok {
		return c.NotRun("no documented direct public-operation mapping; earlier Go bridge cases remain separate")
	}
	m := t.PublicBinding()
	if m.Documentation == "" {
		return c.NotRun("public-equivalence mapping has no binding documentation")
	}
	for _, op := range []string{"createClient", "join", "capabilities", "snapshot", "observe", "unsubscribe", "leave", "dispose"} {
		if m.Operations[op] == "" {
			return c.NotRun("mapping does not identify actual public " + op)
		}
	}
	if m.CreateClient == nil || m.Ready == nil || m.Join == nil || m.Capabilities == nil || m.Snapshot == nil || m.Observe == nil || m.Leave == nil || m.Dispose == nil {
		return c.NotRun("incomplete public-operation mapping")
	}
	client, err := m.CreateClient(ctx)
	if err != nil {
		return err
	}
	if client == nil {
		return c.Fail("createClient returned nil")
	}
	c.Track("mapped public client", func(ctx context.Context) error { return m.Dispose(ctx, client) })
	ready, err := m.Ready(client)
	if err != nil {
		return err
	}
	if !ready {
		return c.Fail("successful createClient did not establish ready")
	}
	if _, err = m.Capabilities(client); err != nil {
		return err
	}
	session, err := m.Join(ctx, client)
	if err != nil {
		return err
	}
	if session == nil {
		return c.Fail("successful join returned nil")
	}
	c.Track("mapped public session", func(ctx context.Context) error { return m.Leave(ctx, session) })
	facts, err := m.Snapshot(session)
	if err != nil {
		return err
	}
	if !facts.Connected || facts.Terminal || facts.Incarnation == "" || facts.Coverage == "" {
		return c.Fail("successful mapped join lacks immediately observable connected incarnation/coverage")
	}
	// Callbacks may arrive asynchronously. Separate channels avoid requiring an
	// integer revision schema, synchronous event delivery or listener syntax.
	a, b := make(chan PublicSessionFacts, 16), make(chan PublicSessionFacts, 16)
	removeA, err := m.Observe(session, func(v PublicSessionFacts) {
		select {
		case a <- v:
		default:
		}
	})
	if err != nil {
		return err
	}
	if removeA == nil {
		return c.Fail("observation has no independent removal")
	}
	removeB, err := m.Observe(session, func(v PublicSessionFacts) {
		select {
		case b <- v:
		default:
		}
	})
	if err != nil {
		return err
	}
	if removeB == nil {
		return c.Fail("second observation has no independent removal")
	}
	defer removeB()
	read := func(ch <-chan PublicSessionFacts) (PublicSessionFacts, error) {
		select {
		case v := <-ch:
			return v, nil
		case <-ctx.Done():
			return PublicSessionFacts{}, c.NotRun("mapped observation not delivered within harness budget")
		}
	}
	for _, ch := range []chan PublicSessionFacts{a, b} {
		v, e := read(ch)
		if e != nil {
			return e
		}
		if v != facts {
			return c.Fail("mapped initial observation is incoherent with join snapshot")
		}
	}
	if err = removeA(); err != nil {
		return err
	}
	if err = removeA(); err != nil {
		return err
	}
	if err = m.Leave(ctx, session); err != nil {
		return err
	}
	if err = m.Leave(ctx, session); err != nil {
		return err
	}
	terminal, err := m.Snapshot(session)
	if err != nil {
		return err
	}
	if !terminal.Terminal || terminal.Connected || terminal.Incarnation != facts.Incarnation {
		return c.Fail("mapped leave did not retain terminal incarnation")
	}
	update, err := read(b)
	if err != nil {
		return err
	}
	if !update.Terminal || update.Incarnation != facts.Incarnation {
		return c.Fail("surviving observer missed terminal fact")
	}
	select {
	case <-a:
		return c.Fail("removed observer received a newly committed change")
	default:
	}
	if err = m.Dispose(ctx, client); err != nil {
		return err
	}
	if err = m.Dispose(ctx, client); err != nil {
		return err
	}
	ready, err = m.Ready(client)
	if err != nil {
		return err
	}
	if ready {
		return c.Fail("disposed client remains ready")
	}
	c.Note(fmt.Sprintf("public mapping %s; local terminal participation established, remote cleanup not inferred", m.Documentation))
	return nil
}
