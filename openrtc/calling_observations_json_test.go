package openrtc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestAudit1905UnionRoundTrips(t *testing.T) {
	leg := CallReference{Session: "session", Leg: "leg"}
	recipients := MessageRecipients{Kind: "session"}
	cases := []struct {
		name   string
		value  any
		target any
	}{
		{"call", CallOutcome{Status: "unknown", NativeReason: Unknown[string](), ProtocolStatus: Unknown[string](), Error: Unknown[SerializedRTCError](), TerminalCause: Unknown[string]()}, new(CallOutcome)},
		{"transfer", TransferRequest{Kind: "attended", Leg: "original", Consultation: &leg}, new(TransferRequest)},
		{"job", JobOutcome{Status: "accepted", Outputs: []JobOutput{}}, new(JobOutcome)},
		{"request", MessageRequest{Mode: "ephemeral", Recipients: &recipients, Content: MessageContent{Kind: "text"}, Requirements: []CapabilityRequest{}}, new(MessageRequest)},
		{"recipients", MessageRecipients{Kind: "participants", IDs: []ParticipantConnectionID{}}, new(MessageRecipients)},
		{"content", MessageContent{Kind: "binary", Bytes: []byte{1, 2}, MediaType: ""}, new(MessageContent)},
		{"encryption", EncryptionOutcome{Status: "pending", Reason: ""}, new(EncryptionOutcome)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(a, tc.target); err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			var x, y any
			if err = json.Unmarshal(a, &x); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(b, &y); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(x, y) {
				t.Fatalf("%s != %s", a, b)
			}
		})
	}
}
func TestAudit1905RejectsInvalidUnionShapes(t *testing.T) {
	cases := []struct {
		data   string
		target any
	}{
		{`{"status":"bogus","nativeCause":{"status":"unknown"}}`, new(CallOutcome)},
		{`{"kind":"attended"}`, new(TransferRequest)},
		{`{"status":"unknown","reason":"","outputs":[]}`, new(JobOutcome)},
		{`{"mode":"ephemeral","content":{"kind":"text","text":""},"requirements":[]}`, new(MessageRequest)},
		{`{"kind":"participants"}`, new(MessageRecipients)},
		{`{"kind":"text","text":"","bytes":""}`, new(MessageContent)},
		{`{"status":"invalid"}`, new(EncryptionOutcome)},
	}
	for _, c := range cases {
		if err := json.Unmarshal([]byte(c.data), c.target); err == nil {
			t.Errorf("accepted %s", c.data)
		}
	}
}

func TestAudit1905ObservationRoundTrip(t *testing.T) {
	source := ObservationSource{Origin: "native", NativeID: Unknown[ScopedNativeID](), NativeTimestamp: Unknown[Instant](), NativeSequence: Unknown[ScopedSequence](), Evidence: []EvidenceReference{}}
	value := NormalizedObservation{Kind: "cleanup", Generation: "g", Source: source, Status: CleanupStatus{Kind: "complete"}}
	a, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded NormalizedObservation
	if err = json.Unmarshal(a, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded.Status.(CleanupStatus); !ok {
		t.Fatalf("status has type %T", decoded.Status)
	}
	b, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("%s != %s", a, b)
	}
}
