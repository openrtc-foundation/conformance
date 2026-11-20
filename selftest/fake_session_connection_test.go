package selftest

import (
	"context"
	"reflect"
	"testing"

	"github.com/openrtc/conformance/openrtc"
)

func TestSessionCleanupLifecycle(t *testing.T) {
	w := newWorld()
	s := newFakeSession(&fakeClient{}, openrtc.ConferenceTarget{}, w, NoDefects)
	s.nativeJoined = true
	s.state = openrtc.ConnectionState("connected")
	s.addressValue, s.identityKey = "room", "identity"
	w.sessions[s] = struct{}{}
	w.held[s] = struct{}{}
	w.activeIdentities[s.identityKey] = struct{}{}
	var changes []string
	s.Observation().On("connectionChanged", func(e openrtc.EventEnvelope[any]) { changes = append(changes, "connection") })
	s.Observation().On("cleanupChanged", func(e openrtc.EventEnvelope[any]) { changes = append(changes, "cleanup") })
	report, err := cleanupAfter(context.Background(), s.Inspection(), s.Leave(context.Background()))
	if err != nil || report.Status.Kind != "complete" || s.state != "ended" {
		t.Fatalf("cleanup: %+v, %v, state %+v", report, err, s.state)
	}
	if !reflect.DeepEqual(changes, []string{"connection", "cleanup", "connection", "cleanup"}) {
		t.Fatalf("event order: %v", changes)
	}
	if _, err = s.Dispose(context.Background()).Result(context.Background()); err != nil || w.counts.Leaves != 1 || len(w.sessions) != 0 || len(w.held) != 0 || len(w.activeIdentities) != 0 {
		t.Fatalf("repeated cleanup: %v, leaves=%d", err, w.counts.Leaves)
	}
}
