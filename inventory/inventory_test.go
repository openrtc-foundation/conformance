package inventory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func currentSource(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("../../../../spec", "rfcs/openrtc-core.md"))
	if e != nil {
		t.Fatal(e)
	}
	return b
}

// RFC §14.3:1119–1149: exact clauses and every independently testable
// obligation remain inventoried even where no executable/service fixture exists.
func TestCurrentInventoryAgainstSoleRFC(t *testing.T) {
	d, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	if e = Validate(d, currentSource(t)); e != nil {
		t.Fatal(e)
	}
	if len(d.Entries) < 600 {
		t.Fatalf("lost normative inventory: %d entries", len(d.Entries))
	}
	positives := map[string]bool{}
	for _, e := range d.Entries {
		if e.Strength == "positive" {
			positives[e.Operations[0]] = true
		}
	}
	for _, op := range []string{"createClient", "join", "leave", "dispose", "snapshot", "observe", "dial", "accept", "configureEncryption", "selectDevice", "renewCredential"} {
		if !positives[op] {
			t.Errorf("missing mandatory positive %s", op)
		}
	}
}
func copyCurrent(t *testing.T) Document {
	t.Helper()
	d, e := Load()
	if e != nil {
		t.Fatal(e)
	}
	return d
}

// RFC §§14.1–14.3:1090–1149: seeded missing obligations and invalid
// evidence mappings cannot be mistaken for complete inventory coverage.
func TestInventoryRejectsDefects(t *testing.T) {
	for _, name := range []string{"wrong-revision", "absent-quote", "missing-obligation", "duplicate-obligation", "dangling-assertion", "missing-non-executable", "missing-implicit-definition", "partial-without-test"} {
		t.Run(name, func(t *testing.T) {
			d := copyCurrent(t)
			switch name {
			case "wrong-revision":
				d.SpecSHA256 = strings.Repeat("0", 64)
			case "absent-quote":
				d.Entries[0].Clause = "invented clause"
			case "missing-obligation":
				d.Entries = d.Entries[1:]
			case "duplicate-obligation":
				d.Entries = append(d.Entries, d.Entries[0])
			case "dangling-assertion":
				d.Entries[0].Assertions = nil
			case "missing-non-executable":
				for i, e := range d.Entries {
					if e.EvidenceMethod == "binding-documentation" && e.Strength != "definition" {
						d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
						break
					}
				}
			case "missing-implicit-definition":
				for i, e := range d.Entries {
					if e.Strength == "definition" && strings.HasPrefix(e.Clause, "async ") {
						d.Entries = append(d.Entries[:i], d.Entries[i+1:]...)
						break
					}
				}
			case "partial-without-test":
				d.Entries[0].Assertions[0].Coverage = "partial"
				d.Entries[0].Assertions[0].Tests = nil
			}
			if e := Validate(d, currentSource(t)); e == nil {
				t.Fatal("defective inventory accepted")
			}
		})
	}
}

// RFC §14.3:1119–1149: current executable references cannot dangle.
func TestDanglingMappedTest(t *testing.T) {
	d := copyCurrent(t)
	tests := map[string]bool{}
	for _, entry := range d.Entries {
		for _, assertion := range entry.Assertions {
			for _, id := range assertion.Tests {
				tests[id] = true
			}
		}
	}
	if e := ValidateTests(d, tests); e != nil {
		t.Fatal("valid mapping rejected:", e)
	}
	d.Entries[0].Assertions[0].Tests = []string{"CT-ABSENT-001"}
	d.Entries[0].Assertions[0].Coverage = "partial"
	if e := ValidateTests(d, tests); e == nil || !strings.Contains(e.Error(), "CT-ABSENT-001") {
		t.Fatal("dangling current assertion not identified:", e)
	}
}
