// In-process integration tests for the CLI: Run(argv) with real files on
// disk, asserting exit codes and output — the same surface CI scripts see.
package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run invokes the CLI in-process and captures both streams.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = Run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// write drops a fixture file into the test's temp dir.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const lcovBase = `SF:src/a.js
DA:1,3
DA:2,1
DA:3,1
DA:4,0
end_of_record
SF:src/b.js
DA:1,1
DA:2,1
end_of_record
`

// lcovRegressed drops src/a.js from 3/4 to 1/4 covered.
const lcovRegressed = `SF:src/a.js
DA:1,3
DA:2,0
DA:3,0
DA:4,0
end_of_record
SF:src/b.js
DA:1,1
DA:2,1
end_of_record
`

func TestVersionSubcommandAndFlagForms(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		code, out, _ := run(t, arg)
		if code != ExitOK || out != "covdrift 0.1.0\n" {
			t.Fatalf("%s: code=%d out=%q", arg, code, out)
		}
	}
}

func TestTopLevelUsageSurface(t *testing.T) {
	code, out, _ := run(t, "help")
	if code != ExitOK || !strings.Contains(out, "covdrift diff") || !strings.Contains(out, "Exit codes") {
		t.Fatalf("help: code=%d out=%q", code, out)
	}
	code, _, errOut := run(t)
	if code != ExitUsage || !strings.Contains(errOut, "Usage") {
		t.Fatalf("no args: code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = run(t, "frobnicate")
	if code != ExitUsage || !strings.Contains(errOut, "unknown subcommand") {
		t.Fatalf("unknown subcommand: code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = run(t, "--tolerance", "5")
	if code != ExitUsage || !strings.Contains(errOut, "unknown flag") {
		t.Fatalf("flag before subcommand: code=%d stderr=%q", code, errOut)
	}
}

func TestDiffFailsOnRegressionWithExitOne(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, out, _ := run(t, "diff", base, cur)
	if code != ExitGate {
		t.Fatalf("regression must exit 1, got %d\n%s", code, out)
	}
	if !strings.Contains(out, "gate: FAIL") || !strings.Contains(out, "src/a.js") {
		t.Fatalf("output should show the failing file:\n%s", out)
	}
}

func TestDiffPassesOnIdenticalRuns(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovBase)
	code, out, _ := run(t, "diff", base, cur)
	if code != ExitOK || !strings.Contains(out, "gate: OK") {
		t.Fatalf("identical runs must pass: code=%d\n%s", code, out)
	}
}

func TestTwoBarePathsDefaultToDiff(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, out, _ := run(t, base, cur)
	if code != ExitGate || !strings.Contains(out, "covdrift — baseline vs current") {
		t.Fatalf("bare paths should run diff: code=%d\n%s", code, out)
	}
}

func TestDiffToleranceAllowsBoundedDrops(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	// a.js drops 50pp: still fails at 10pp tolerance, passes at 50pp.
	code, _, _ := run(t, "diff", "--tolerance", "10", base, cur)
	if code != ExitGate {
		t.Fatalf("50pp drop must fail 10pp tolerance, got %d", code)
	}
	code, _, _ = run(t, "diff", "--tolerance", "50", base, cur)
	if code != ExitOK {
		t.Fatalf("50pp drop must pass 50pp tolerance, got %d", code)
	}
}

func TestDiffNoGateReportsButExitsZero(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, out, _ := run(t, "diff", "--no-gate", base, cur)
	if code != ExitOK {
		t.Fatalf("--no-gate must exit 0, got %d", code)
	}
	if !strings.Contains(out, "gate: FAIL") {
		t.Fatalf("--no-gate still reports the breach:\n%s", out)
	}
}

func TestDiffJSONOutputIsMachineReadable(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, out, _ := run(t, "diff", "--format", "json", base, cur)
	if code != ExitGate {
		t.Fatalf("json format must not change the exit code, got %d", code)
	}
	var doc struct {
		Gate struct {
			OK bool `json:"ok"`
		} `json:"gate"`
		Counts struct {
			Regressed int `json:"regressed"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc.Gate.OK || doc.Counts.Regressed != 1 {
		t.Fatalf("json verdict wrong: %+v", doc)
	}
}

func TestDiffAcrossDifferentFormats(t *testing.T) {
	// Baseline from lcov, current from a Go coverprofile: the normalized
	// model makes them directly comparable. Same file, same coverage.
	dir := t.TempDir()
	base := write(t, dir, "base.info", "SF:pkg/a.go\nDA:1,1\nDA:2,1\nDA:3,0\nend_of_record\n")
	cur := write(t, dir, "cur.out", "mode: set\npkg/a.go:1.1,2.5 2 1\npkg/a.go:3.1,3.9 1 0\n")
	code, out, _ := run(t, "diff", base, cur)
	if code != ExitOK {
		t.Fatalf("cross-format identical coverage must pass: %d\n%s", code, out)
	}
	if !strings.Contains(out, "0 regressed") {
		t.Fatalf("expected no regressions:\n%s", out)
	}
}

func TestDiffStripPrefixReconcilesPaths(t *testing.T) {
	// The baseline was produced under a CI build root; --strip-prefix
	// makes it line up with the current run's relative paths.
	dir := t.TempDir()
	base := write(t, dir, "base.info", "SF:/build/repo/src/a.js\nDA:1,1\nend_of_record\n")
	cur := write(t, dir, "cur.info", "SF:src/a.js\nDA:1,1\nend_of_record\n")
	code, out, _ := run(t, "diff", "--strip-prefix", "/build/repo/", base, cur)
	if code != ExitOK {
		t.Fatalf("stripped paths must match: %d\n%s", code, out)
	}
	if !strings.Contains(out, "1 compared") || !strings.Contains(out, "0 added") {
		t.Fatalf("paths did not reconcile:\n%s", out)
	}
}

func TestDiffExcludeRemovesFilesFromTheGate(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, _, _ := run(t, "diff", "--exclude", "src/a.js", base, cur)
	if code != ExitOK {
		t.Fatalf("excluding the regressed file must pass, got %d", code)
	}
}

// Trailing flags matter because that is what Quickstart readers type first:
// `covdrift diff base cur --format markdown`. Go's flag package alone would
// stop parsing at the first positional and reject this.
func TestDiffAcceptsFlagsAfterPositionalPaths(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cur := write(t, dir, "cur.info", lcovRegressed)
	code, out, _ := run(t, "diff", base, cur, "--format", "markdown")
	if code != ExitGate || !strings.Contains(out, "coverage gate failed") {
		t.Fatalf("trailing --format must parse: code=%d out=%q", code, out)
	}
	// Interleaved: flag, positional, positional, flag.
	code, _, _ = run(t, "diff", "--tolerance", "80", base, cur, "--no-gate")
	if code != ExitOK {
		t.Fatalf("interleaved flags must parse, got %d", code)
	}
}

func TestShowAcceptsFlagsAfterPositionalPath(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	code, out, _ := run(t, "show", base, "--format", "json")
	if code != ExitOK || !strings.Contains(out, "\"files\"") {
		t.Fatalf("trailing --format must parse: code=%d out=%q", code, out)
	}
}

func TestDiffUsageErrors(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	cases := [][]string{
		{"diff", base},                                  // one path
		{"diff", "--format", "yaml", base, base},        // bad format
		{"diff", "--input-format", "junit", base, base}, // bad input format
		{"diff", "--tolerance", "-1", base, base},       // negative tolerance
		{"diff", "--min-lines", "-5", base, base},       // negative min-lines
	}
	for _, args := range cases {
		if code, _, _ := run(t, args...); code != ExitUsage {
			t.Fatalf("%v should exit %d (usage)", args, ExitUsage)
		}
	}
}

func TestDiffRuntimeErrors(t *testing.T) {
	dir := t.TempDir()
	base := write(t, dir, "base.info", lcovBase)
	bad := write(t, dir, "bad.txt", "this is not a coverage report\n")
	if code, _, errOut := run(t, "diff", base, filepath.Join(dir, "missing.info")); code != ExitRuntime || !strings.Contains(errOut, "current") {
		t.Fatalf("missing file must exit 3 and name the side, got %d", code)
	}
	if code, _, _ := run(t, "diff", bad, base); code != ExitRuntime {
		t.Fatalf("unparseable baseline must exit 3, got %d", code)
	}
}

func TestShowNormalizesASingleReport(t *testing.T) {
	dir := t.TempDir()
	report := write(t, dir, "cov.xml", `<?xml version="1.0"?>
<coverage><packages><package name="p"><classes>
<class filename="app/x.py"><lines><line number="1" hits="2"/><line number="2" hits="0"/></lines></class>
</classes></package></packages></coverage>`)
	code, out, _ := run(t, "show", report)
	if code != ExitOK {
		t.Fatalf("show failed: %d", code)
	}
	if !strings.Contains(out, "cobertura report, 1 file\n") || !strings.Contains(out, "app/x.py") {
		t.Fatalf("show output wrong:\n%s", out)
	}
	code, out, _ = run(t, "show", "--format", "json", report)
	if code != ExitOK || !strings.Contains(out, `"format": "cobertura"`) {
		t.Fatalf("show json wrong: %d\n%s", code, out)
	}
}

func TestShowUsageAndRuntimeErrors(t *testing.T) {
	dir := t.TempDir()
	report := write(t, dir, "cov.info", lcovBase)
	if code, _, _ := run(t, "show"); code != ExitUsage {
		t.Fatalf("show without a path must exit 2")
	}
	if code, _, _ := run(t, "show", "--format", "markdown", report); code != ExitUsage {
		t.Fatalf("show --format markdown is not supported, must exit 2")
	}
	if code, _, _ := run(t, "show", filepath.Join(dir, "nope.info")); code != ExitRuntime {
		t.Fatalf("show on a missing file must exit 3")
	}
}
