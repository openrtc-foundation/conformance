// Package inventory contains the frozen obligation inventory for the sole
// normative RFC. A mapping is evidence of an assertion's scope, never a result.
package inventory

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed 00d81f2.json v1-amended.json
var files embed.FS

type Assertion struct {
	ID       string   `json:"id"`
	Version  int      `json:"version"`
	Tests    []string `json:"tests"`
	Coverage string   `json:"coverage"`
}
type Entry struct {
	ID             string      `json:"id"`
	Section        string      `json:"section"`
	Line           int         `json:"line"`
	Clause         string      `json:"clause"`
	Strength       string      `json:"strength"`
	Applicability  string      `json:"applicability"`
	Operations     []string    `json:"operations"`
	Kinds          []string    `json:"kinds"`
	Scopes         []string    `json:"scopes"`
	Profile        string      `json:"profile"`
	EvidenceMethod string      `json:"evidenceMethod"`
	Assertions     []Assertion `json:"assertions"`
	Aliases        []string    `json:"aliases"`
}
type Document struct {
	Revision     string  `json:"revision"`
	BaseRevision string  `json:"baseRevision"`
	SpecSHA256   string  `json:"specSHA256"`
	Source       string  `json:"source"`
	Entries      []Entry `json:"entries"`
}

func Load() (Document, error) { return LoadRevision("v1-amended.json") }
func LoadRevision(name string) (Document, error) {
	var d Document
	b, err := files.ReadFile(name)
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(b, &d)
	return d, err
}

var modal = regexp.MustCompile(`\b(MUST(?: NOT)?|SHOULD(?: NOT)?)\b`)
var heading = regexp.MustCompile(`^#{2,4}\s+(\d+(?:\.\d+)*|A\.\d+)\.?\s`)

func normativeSection(s string) bool {
	for _, prefix := range []string{"4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "A"} {
		if s == prefix || strings.HasPrefix(s, prefix+".") {
			return true
		}
	}
	return false
}

// Validate rejects wrong content revisions, lost modal obligations, invalid
// locators and dangling/duplicate assertions. Missing test execution remains a
// reportable gap rather than an inventory validation failure (§14.3:1119–1149).
func Validate(d Document, source []byte) error {
	sum := sha256.Sum256(source)
	if d.Revision == "" || d.BaseRevision != "00d81f2" || d.Source != "rfcs/openrtc-core.md" || d.SpecSHA256 != hex.EncodeToString(sum[:]) {
		return fmt.Errorf("inventory revision/source/hash does not identify this RFC")
	}
	// Published inventories are frozen. Validate against the identified release
	// as well as modal accounting, so deleting an implicit table obligation or
	// a non-executable IDL definition cannot escape the completeness check.
	name := ""
	switch d.Revision {
	case "00d81f2":
		name = "00d81f2.json"
	case "v1-amended-780927ab":
		name = "v1-amended.json"
	default:
		return fmt.Errorf("unknown inventory revision %q", d.Revision)
	}
	expected, err := LoadRevision(name)
	if err != nil {
		return err
	}
	if expected.SpecSHA256 != d.SpecSHA256 {
		return fmt.Errorf("inventory revision hash mismatch")
	}
	lines := strings.Split(string(source), "\n")
	seen := map[string]bool{}
	assertions := map[string]bool{}
	covered := map[int]int{}
	for _, e := range d.Entries {
		if e.ID == "" || seen[e.ID] {
			return fmt.Errorf("duplicate or empty obligation ID %q", e.ID)
		}
		seen[e.ID] = true
		if e.Line < 1 || e.Line > len(lines) || e.Clause == "" || !strings.Contains(lines[e.Line-1], e.Clause) {
			return fmt.Errorf("%s: absent clause text at line %d", e.ID, e.Line)
		}
		if !normativeSection(e.Section) || e.Applicability == "" || e.EvidenceMethod == "" || len(e.Assertions) == 0 {
			return fmt.Errorf("%s: missing scope/evidence/assertion metadata", e.ID)
		}
		if e.Strength != "definition" && e.Strength != "positive" {
			covered[e.Line]++
		}
		for _, a := range e.Assertions {
			if a.ID == "" || assertions[a.ID] || a.Version < 1 {
				return fmt.Errorf("%s: duplicate or invalid assertion %q", e.ID, a.ID)
			}
			assertions[a.ID] = true
			switch a.Coverage {
			case "tested", "partial":
				if len(a.Tests) == 0 {
					return fmt.Errorf("%s: dangling assertion test mapping", a.ID)
				}
			case "missing", "documentary":
			default:
				return fmt.Errorf("%s: invalid coverage %q", a.ID, a.Coverage)
			}
		}
	}
	byID := map[string]Entry{}
	for _, entry := range d.Entries {
		byID[entry.ID] = entry
	}
	for _, entry := range expected.Entries {
		if !seen[entry.ID] {
			return fmt.Errorf("missing frozen obligation %s", entry.ID)
		}
		actual := byID[entry.ID]
		if actual.Clause != entry.Clause || actual.Section != entry.Section || actual.Line != entry.Line || actual.Strength != entry.Strength {
			return fmt.Errorf("unversioned clause/locator semantics for %s", entry.ID)
		}
	}
	if len(seen) != len(expected.Entries) {
		return fmt.Errorf("unversioned inventory obligation addition")
	}
	section := ""
	for n, line := range lines {
		if m := heading.FindStringSubmatch(line); m != nil {
			section = m[1]
		}
		if normativeSection(section) && len(modal.FindAllString(line, -1)) != covered[n+1] {
			return fmt.Errorf("line %d: missing or duplicate independently inventoried modal obligation", n+1)
		}
	}
	if len(d.Entries) == 0 {
		return fmt.Errorf("empty obligation inventory")
	}
	return nil
}

// ValidateTests checks explicit executable mappings, including obsolete-only
// references. Legacy aliases have no authority to create a current assertion.
func ValidateTests(d Document, tests map[string]bool) error {
	for _, e := range d.Entries {
		for _, a := range e.Assertions {
			for _, id := range a.Tests {
				if !tests[id] {
					return fmt.Errorf("%s: dangling current test %s", a.ID, id)
				}
			}
		}
	}
	return nil
}
