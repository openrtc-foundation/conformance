// Command matrix validates and renders the current, versioned RFC inventory.
package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/openrtc/conformance/harness"
	"github.com/openrtc/conformance/inventory"
	"github.com/openrtc/conformance/tests/capability"
	"github.com/openrtc/conformance/tests/core"
)

type testRecord struct {
	ID         string              `json:"id"`
	Title      string              `json:"title"`
	Suite      harness.SuiteName   `json:"suite"`
	Profile    harness.ProfileName `json:"profile"`
	Reqs       []string            `json:"requirements"`
	Observable string              `json:"observable"`
	Needs      []string            `json:"needs"`
}

func specFiles(spec string) ([]string, error) {
	path := filepath.Join(spec, "rfcs/openrtc-core.md")
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("read sole normative RFC (set OPENRTC_SPEC): %w", err)
	}
	return []string{path}, nil
}
func extract(spec string) (inventory.Document, error) {
	d, err := inventory.Load()
	if err != nil {
		return d, err
	}
	files, err := specFiles(spec)
	if err != nil {
		return d, err
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		return d, err
	}
	return d, inventory.Validate(d, b)
}
func records(tests []harness.ConformanceTest) []testRecord {
	out := make([]testRecord, 0, len(tests))
	for _, t := range tests {
		needs := make([]string, 0, len(t.Needs))
		for _, n := range t.Needs {
			needs = append(needs, string(n))
		}
		out = append(out, testRecord{t.ID, t.Title, t.Suite, t.Profile, t.Requirements, t.Observable, needs})
	}
	return out
}
func allRecords() []testRecord {
	all := append([]harness.ConformanceTest{}, core.CoreTests...)
	all = append(all, capability.CapabilityTests...)
	return records(all)
}
func readJSON(path string, v any) error {
	b, e := os.ReadFile(path)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, v)
}
func writeJSON(path string, v any) error {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// errorCodes reads typed, literal portable constants. Profile-specific codes
// must contain a namespace separator and cannot enlarge the core vocabulary.
func errorCodes(path string) (map[string]bool, error) {
	f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if e != nil {
		return nil, e
	}
	out := map[string]bool{}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, s := range g.Specs {
			v := s.(*ast.ValueSpec)
			typ, ok := v.Type.(*ast.Ident)
			if !ok || typ.Name != "ErrorCode" {
				continue
			}
			for _, x := range v.Values {
				if lit, ok := x.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					value, e := strconv.Unquote(lit.Value)
					if e != nil {
						return nil, e
					}
					out[value] = true
				}
			}
		}
	}
	return out, nil
}

var catalogCode = regexp.MustCompile("(?m)^\\| `([a-z-]+)`\\s*\\|")

func compareCodes(goPath, mdPath string) ([]string, []string, error) {
	a, e := errorCodes(goPath)
	if e != nil {
		return nil, nil, e
	}
	b, e := os.ReadFile(mdPath)
	if e != nil {
		return nil, nil, e
	}
	text := string(b)
	start := strings.Index(text, "## 9. Error Handling")
	end := strings.Index(text, "## 10.")
	if start < 0 || end < start {
		return nil, nil, fmt.Errorf("missing sole RFC section 9 error table")
	}
	catalog := map[string]bool{}
	table := text[start:end]
	// Restrict extraction to the first table, excluding outcome/assessment tables.
	p := strings.Index(table, "The error MUST")
	if p >= 0 {
		table = table[:p]
	}
	for _, m := range catalogCode.FindAllStringSubmatch(table, -1) {
		catalog[m[1]] = true
	}
	if len(catalog) != 10 {
		return nil, nil, fmt.Errorf("section 9 must define exactly ten core error codes; got %d", len(catalog))
	}
	var left, right []string
	for _, k := range keys(a) {
		if !catalog[k] && !strings.Contains(k, ":") && !strings.Contains(k, "/") {
			left = append(left, k)
		}
	}
	for _, k := range keys(catalog) {
		if !a[k] {
			right = append(right, k)
		}
	}
	return left, right, nil
}
func testIDs(tests []testRecord) (map[string]bool, error) {
	out := map[string]bool{}
	for _, t := range tests {
		if t.ID == "" || out[t.ID] {
			return nil, fmt.Errorf("duplicate or empty test ID %q", t.ID)
		}
		out[t.ID] = true
	}
	return out, nil
}
func esc(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "|", `\|`), "\n", " ") }
func matrix(w io.Writer, d inventory.Document, tests []testRecord) error {
	ids, err := testIDs(tests)
	if err != nil {
		return err
	}
	if err = inventory.ValidateTests(d, ids); err != nil {
		return err
	}
	fmt.Fprint(w, "# Conformance assertion matrix\n\nStatus: **harness self-tested only**. No provider execution is represented by this inventory.\n\n")
	fmt.Fprintf(w, "Sole normative source: `%s`. Base revision: `%s`. Amended inventory: `%s`. RFC SHA-256: `%s`.\n\n", d.Source, d.BaseRevision, d.Revision, d.SpecSHA256)
	fmt.Fprintf(w, "%d obligation entries; %d executable catalog definitions. Modal obligations are inventoried separately, including multiple obligations in one sentence. Appendix definitions, mandatory operation-table rows and §14.3 scenarios have separate entries.\n\n", len(d.Entries), len(tests))
	fmt.Fprint(w, "Coverage describes the assertion mapping, never a passing execution. `partial` requires additional evidence for the rest of the clause; `missing` has no mapped executable assertion; `documentary` requires artifact, binding or independent evidence. A citation or historical alias alone does not establish coverage. SHOULD entries require an explicit decision and justification.\n\n")
	fmt.Fprint(w, "Final statuses are `pass`, `fail`, `incomplete`, and `not-applicable`. Missing fixtures, unrun assertions and unknown evidence are incomplete. Exclusions require the assertion, reason and evidence. Optional positive media tests apply only to the exact supported operation, kind, combination and owner scope; negative, security and cleanup obligations remain applicable. Claimed profiles require their mandatory operation positives.\n\n")
	fmt.Fprint(w, "| Obligation / assertion version | RFC section:line | Applicability | Evidence | Mapping | Tests / gaps |\n|---|---|---|---|---|---|\n")
	for _, e := range d.Entries {
		for _, a := range e.Assertions {
			detail := strings.Join(a.Tests, ", ")
			if detail == "" {
				detail = "No execution evidence; applicable claim remains incomplete"
			}
			fmt.Fprintf(w, "| %s / %s v%d | §%s:%d | %s | %s | %s | %s |\n", esc(e.ID), esc(a.ID), a.Version, e.Section, e.Line, esc(e.Applicability), esc(e.EvidenceMethod), esc(a.Coverage), esc(detail))
		}
	}
	fmt.Fprint(w, "\n## Executable catalog\n\nLegacy CT identifiers are retained as aliases to revised assertions; historical passes cannot be reused for changed assertions. Legacy CORE/MED/etc citations are background crosswalk labels, not normative documents. Registry, descriptor, native-schema, Once/Off and inspection-ledger promises are bridge-specific and cannot make a universal RFC claim fail. A direct binding may use a harness-only adapter mapping.\n\n| ID | Suite | Label | Current references / historical aliases | Observable oracle |\n|---|---|---|---|---|\n")
	for _, t := range tests {
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", esc(t.ID), t.Suite, t.Profile, esc(strings.Join(t.Reqs, ", ")), esc(t.Observable))
	}
	fmt.Fprint(w, "\n## Evidence limits\n\nEvery unmapped applicable assertion is incomplete, including unimplemented profile-positive, protection, stream, authorization and published-artifact checks. Local operation settlement is separate from remote cleanup. A native request acknowledgment cannot prove release; unknown remote state remains unknown. A fake cannot establish a real service boundary or artifact audit. Public representation equivalence requires a documented mapping to actual implementation operations and facts.\n")
	return nil
}
func moduleRoot() (string, error) {
	dir, e := os.Getwd()
	if e != nil {
		return "", e
	}
	for {
		if _, e = os.Stat(filepath.Join(dir, "go.mod")); e == nil {
			return dir, nil
		}
		p := filepath.Dir(dir)
		if p == dir {
			return "", fmt.Errorf("cannot locate Go module")
		}
		dir = p
	}
}
func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: matrix {extract-requirements OUT.json|test-coverage INVENTORY.json TESTS.json|check-requirement-ids INVENTORY.json|check-error-codes|generate-test-matrix INVENTORY.json TESTS.json OUT.md}")
	}
	root, e := moduleRoot()
	if e != nil {
		return e
	}
	spec := os.Getenv("OPENRTC_SPEC")
	if spec == "" {
		spec = filepath.Clean(filepath.Join(root, "../../../spec"))
	}
	need := func(n int) error {
		if len(args) != n+1 {
			return fmt.Errorf("%s: expected %d arguments", args[0], n)
		}
		return nil
	}
	load := func(path string) (inventory.Document, error) {
		var d inventory.Document
		if e := readJSON(path, &d); e != nil {
			return d, e
		}
		sf, e := specFiles(spec)
		if e != nil {
			return d, e
		}
		b, e := os.ReadFile(sf[0])
		if e != nil {
			return d, e
		}
		return d, inventory.Validate(d, b)
	}
	switch args[0] {
	case "extract-requirements":
		if e = need(1); e != nil {
			return e
		}
		d, e := extract(spec)
		if e != nil {
			return e
		}
		if e = writeJSON(args[1], d); e != nil {
			return e
		}
		fmt.Fprintf(out, "%s: %d current obligations; RFC %s\n", d.Revision, len(d.Entries), d.SpecSHA256)
	case "test-coverage":
		if e = need(2); e != nil {
			return e
		}
		d, e := load(args[1])
		if e != nil {
			return e
		}
		tests := allRecords()
		ids, e := testIDs(tests)
		if e != nil {
			return e
		}
		if e = inventory.ValidateTests(d, ids); e != nil {
			return e
		}
		if e = writeJSON(args[2], tests); e != nil {
			return e
		}
		fmt.Fprintf(out, "%d tests; %d obligations; mapping is not execution evidence\n", len(tests), len(d.Entries))
	case "check-requirement-ids":
		if e = need(1); e != nil {
			return e
		}
		d, e := load(args[1])
		if e != nil {
			return e
		}
		ids, e := testIDs(allRecords())
		if e != nil {
			return e
		}
		if e = inventory.ValidateTests(d, ids); e != nil {
			return e
		}
		fmt.Fprintf(out, "%s: %d exact clauses; no duplicate/dangling current assertions; historical IDs are aliases only\n", d.Revision, len(d.Entries))
	case "check-error-codes":
		if e = need(0); e != nil {
			return e
		}
		a, b, e := compareCodes(filepath.Join(root, "openrtc/errors.go"), filepath.Join(spec, "rfcs/openrtc-core.md"))
		if e != nil {
			return e
		}
		fmt.Fprintf(out, "in Go only: %v\nin RFC §9 only: %v\n", a, b)
		if len(a)+len(b) > 0 {
			return fmt.Errorf("core error code catalogs differ")
		}
	case "generate-test-matrix":
		if e = need(3); e != nil {
			return e
		}
		d, e := load(args[1])
		if e != nil {
			return e
		}
		var tests []testRecord
		if e = readJSON(args[2], &tests); e != nil {
			return e
		}
		var b strings.Builder
		if e = matrix(&b, d, tests); e != nil {
			return e
		}
		if e = os.WriteFile(args[3], []byte(b.String()), 0644); e != nil {
			return e
		}
		fmt.Fprintf(out, "%d lines\n", strings.Count(b.String(), "\n"))
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
	return nil
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "matrix:", err)
		os.Exit(1)
	}
}
