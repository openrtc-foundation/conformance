package capability

import "testing"

func TestParseObservedAt(t *testing.T) {
	for _, instant := range []string{"2024-03-10", "2024-03-10T12:30Z", "2024-03-10T12:30:45.123Z", "2024-03-10T12:30:45", "Sun, 10 Mar 2024 12:30:45 GMT"} {
		if _, err := parseObservedAt(instant); err != nil {
			t.Errorf("parseObservedAt(%q): %v", instant, err)
		}
	}
	if _, err := parseObservedAt("not an instant"); err == nil {
		t.Fatal("accepted an invalid instant")
	}
}
