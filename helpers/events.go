package helpers

import (
	"context"
	"fmt"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/openrtc"
)

// ConnectionKinds returns the connection state kinds seen in recorded events.
func ConnectionKinds(events []harness.RecordedEvent) []string {
	out := []string{}
	for _, event := range events {
		if event.Key != "connectionChanged" {
			continue
		}
		switch p := event.Envelope.Payload.(type) {
		case openrtc.StateChange[openrtc.ConnectionSnapshot]:
			out = append(out, string(p.Current.State))
		case *openrtc.StateChange[openrtc.ConnectionSnapshot]:
			out = append(out, string(p.Current.State))
		case map[string]any:
			if c, ok := p["current"].(map[string]any); ok {
				if s, ok := c["state"].(map[string]any); ok {
					if k, ok := s["kind"].(string); ok {
						out = append(out, k)
					}
				}
			}
		}
	}
	return out
}

// Request constructs a minimal capability request tied to a session.
func Request(_ *harness.TestContext, session openrtc.RTCSession, id openrtc.CapabilityID, strength ...string) openrtc.CapabilityRequest {
	r := RequestAt(id, strength...)
	r.Scope.Resource = "session"
	r.Scope.SessionKind = session.Kind()
	return r
}

// RequestAt constructs a capability request at client scope.
func RequestAt(id openrtc.CapabilityID, strength ...string) openrtc.CapabilityRequest {
	s := "required"
	if len(strength) > 0 {
		s = strength[0]
	}
	request := openrtc.CapabilityRequest{Kind: "portable", ID: id, Strength: s, Scope: openrtc.CapabilityScope{Resource: "client", Authority: "client", Profile: "common"}, Guarantees: []openrtc.GuaranteeRequirement{}, Constraints: []openrtc.RequestedConstraint{}}
	if namespace, name, ok := strings.Cut(string(id), "/"); ok && strings.HasPrefix(namespace, "adapter:") {
		request.Kind = "adapter"
		request.ID = ""
		request.Adapter = &openrtc.AdapterCapabilityReference{Kind: "adapter", AdapterID: openrtc.AdapterID(strings.TrimPrefix(namespace, "adapter:")), Name: name}
		request.SchemaID = "conformance-probe"
		request.SchemaRevision = "0"
		request.Payload = map[string]any{}
	}
	return request
}

// NextEvent waits for the first matching event at or after the requested order.
// events supplies a fresh snapshot on each poll.
func NextEvent(ctx context.Context, tc *harness.TestContext, events func() []harness.RecordedEvent, key, label string, predicate func(harness.RecordedEvent) bool, after int) (harness.RecordedEvent, error) {
	var found *harness.RecordedEvent
	err := tc.WaitFor(ctx, func() bool {
		for _, e := range events() {
			if e.Order >= after && e.Key == key && (predicate == nil || predicate(e)) {
				v := e
				found = &v
				return true
			}
		}
		return false
	}, label, 0)
	if err != nil {
		return harness.RecordedEvent{}, err
	}
	if found == nil {
		return harness.RecordedEvent{}, tc.Fail(fmt.Sprintf("event not found: %s", label))
	}
	return *found, nil
}
