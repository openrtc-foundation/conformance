package selftest

import (
	"encoding/json"
	"github.com/openrtc/conformance/openrtc"
	"strings"
	"testing"
)

func TestObservationDelivery(t *testing.T) {
	o := newFakeObservation(openrtc.SessionSnapshot{}, FakeDefects{})
	seen := 0
	o.On("connectionChanged", func(e openrtc.EventEnvelope[any]) {
		if e.Sequence == 1 && (o.Snapshot().Revision != 1 || e.Payload != "value") {
			t.Error("event was not committed first")
		}
		panic("listener failure")
	})
	o.Once("connectionChanged", func(e openrtc.EventEnvelope[any]) { seen++ })
	o.commit(openrtc.SessionSnapshot{Revision: 1}, []observationEvent{{"connectionChanged", "value"}})
	o.commit(openrtc.SessionSnapshot{Revision: 2}, []observationEvent{{"connectionChanged", "next"}})
	if seen != 1 {
		t.Fatalf("once listener called %d times", seen)
	}
}
func TestFailureSerializationDefect(t *testing.T) {
	for _, leak := range []bool{false, true} {
		e := fail(openrtc.ProviderFailure, "join", "failure", failureExtras{original: "secret-token", leakOriginal: leak})
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "secret-token") != leak {
			t.Fatalf("leak=%v: %s", leak, b)
		}
	}
}
func TestDescriptor(t *testing.T) {
	d := descriptorFor("devices.enumerate", 4)
	if d.Support.Level != "supported" || d.Revision != 4 || d.Scope.AdapterID != adapterID {
		t.Fatalf("unexpected descriptor: %+v", d)
	}
}
