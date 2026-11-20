package main

import (
	"github.com/openrtc/conformance/inventory"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func put(t *testing.T, p, s string) {
	t.Helper()
	if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, []byte(s), 0644); e != nil {
		t.Fatal(e)
	}
}
func actualSpec(t *testing.T) string {
	t.Helper()
	root, e := moduleRoot()
	if e != nil {
		t.Fatal(e)
	}
	return filepath.Clean(filepath.Join(root, "../../../spec"))
}

// RFC §§14.1–14.3:1090–1149: sole normative source, exact revision and
// assertion gaps. Historical requirement labels cannot establish coverage.
func TestCurrentPipelineAndDeterministicMatrix(t *testing.T) {
	t.Setenv("OPENRTC_SPEC", actualSpec(t))
	dir := t.TempDir()
	req, tests, doc := filepath.Join(dir, "req.json"), filepath.Join(dir, "tests.json"), filepath.Join(dir, "matrix.md")
	var output strings.Builder
	for _, args := range [][]string{{"extract-requirements", req}, {"test-coverage", req, tests}, {"check-requirement-ids", req}, {"generate-test-matrix", req, tests, doc}} {
		if e := run(args, &output); e != nil {
			t.Fatalf("%v: %v", args, e)
		}
	}
	first, e := os.ReadFile(doc)
	if e != nil {
		t.Fatal(e)
	}
	if e = run([]string{"generate-test-matrix", req, tests, doc}, &output); e != nil {
		t.Fatal(e)
	}
	second, e := os.ReadFile(doc)
	if e != nil {
		t.Fatal(e)
	}
	if string(first) != string(second) {
		t.Fatal("matrix regeneration is nondeterministic")
	}
	for _, s := range []string{"rfcs/openrtc-core.md", "780927ab", "incomplete", "No execution evidence", "CT-LIFE-028"} {
		if !strings.Contains(string(first), s) {
			t.Errorf("missing %s", s)
		}
	}
	if strings.Contains(string(first), "61.08%") {
		t.Fatal("obsolete citation coverage survived")
	}
}

// RFC §§14.1:1090,14.3:1119–1149: only the identified sole normative RFC contributes obligations.
func TestSoleNormativeRFC(t *testing.T) {
	dir := t.TempDir()
	put(t, filepath.Join(dir, "rfcs/0001-architecture.md"), "obsolete")
	if _, e := specFiles(dir); e == nil {
		t.Fatal("obsolete-only specification accepted")
	}
	put(t, filepath.Join(dir, "rfcs/openrtc-core.md"), "current")
	files, e := specFiles(dir)
	if e != nil || len(files) != 1 || !strings.HasSuffix(files[0], "openrtc-core.md") {
		t.Fatalf("%v %v", files, e)
	}
	if _, e = extract(dir); e == nil {
		t.Fatal("wrong RFC content hash accepted")
	}
}

// RFC §9:529–543: ten core codes only; profile codes are namespaced.
func TestCodeTableIsRestrictedToSectionNine(t *testing.T) {
	dir := t.TempDir()
	goFile, md := filepath.Join(dir, "errors.go"), filepath.Join(dir, "rfc.md")
	codes := []string{"invalid-argument", "invalid-state", "unsupported", "permission-denied", "authentication-failed", "cancelled", "timeout", "unavailable", "internal", "unknown"}
	var g, b strings.Builder
	g.WriteString("package p\ntype ErrorCode string\nconst (\n")
	b.WriteString("## 9. Error Handling\n")
	for i, c := range codes {
		g.WriteString("C" + string(rune('A'+i)) + " ErrorCode = \"" + c + "\"\n")
		b.WriteString("| `" + c + "` | definition |\n")
	}
	g.WriteString("Profile ErrorCode = \"vendor:condition\"\n)\n")
	b.WriteString("The error MUST be safe.\n| `partial` | not a code |\n## 10. Extensions\n| `old-code` | obsolete |\n")
	put(t, goFile, g.String())
	put(t, md, b.String())
	a, z, e := compareCodes(goFile, md)
	if e != nil || len(a)+len(z) > 0 {
		t.Fatalf("%v %v %v", a, z, e)
	}
	put(t, goFile, strings.Replace(g.String(), "vendor:condition", "unqualified-extra", 1))
	a, _, e = compareCodes(goFile, md)
	if e != nil || len(a) != 1 {
		t.Fatalf("unqualified code accepted: %v %v", a, e)
	}
}

// RFC §14.3:1119–1149: current assertion mappings must identify executable tests.
func TestMatrixRejectsDanglingCurrentMapping(t *testing.T) {
	d, e := inventory.Load()
	if e != nil {
		t.Fatal(e)
	}
	d.Entries[0].Assertions[0].Tests = []string{"CT-ABSENT-001"}
	var b strings.Builder
	if e = matrix(&b, d, nil); e == nil {
		t.Fatal("dangling test accepted")
	}
}
