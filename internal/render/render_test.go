// Tests for the three renderers. Renderers are pure functions of the
// Result, so these assert on exact substrings of deterministic output.
package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/JaydenCJ/covdrift/internal/diff"
	"github.com/JaydenCJ/covdrift/internal/model"
)

// scenario builds a diff with one regression (with lost lines), one
// improvement, one added, one removed, and one unchanged file.
func scenario(t *testing.T) *diff.Result {
	t.Helper()
	base := model.NewReport("lcov")
	cur := model.NewReport("cobertura")
	fill := func(r *model.Report, path string, covered, total int) {
		f := r.File(path)
		for ln := 1; ln <= total; ln++ {
			if ln <= covered {
				f.AddHits(ln, 1)
			} else {
				f.MarkSeen(ln)
			}
		}
	}
	fill(base, "src/parser.go", 9, 10)
	fill(cur, "src/parser.go", 5, 10)
	fill(base, "src/util.go", 5, 10)
	fill(cur, "src/util.go", 9, 10)
	fill(cur, "src/new.go", 4, 8)
	fill(base, "src/old.go", 7, 10)
	fill(base, "src/same.go", 3, 4)
	fill(cur, "src/same.go", 3, 4)
	return diff.Compare(base, cur, diff.DefaultOptions())
}

func TestTextReportShowsMovementAndGateVerdict(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, scenario(t), TextOptions{})
	out := buf.String()
	for _, want := range []string{
		"covdrift — baseline vs current",
		"1 regressed · 1 improved · 1 added · 1 removed · 1 unchanged",
		"REGRESS", "src/parser.go", "90.0%", "50.0%", "-40.0pp",
		"improve", "src/util.go", "+40.0pp",
		"added", "src/new.go",
		"removed", "src/old.go",
		"gate: FAIL — 1 breach\n", // singular: never "1 breaches" or "1 breach(es)"
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("text output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "src/same.go") {
		t.Fatalf("unchanged file listed without --all:\n%s", out)
	}
}

func TestTextReportListsLostLineRanges(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, scenario(t), TextOptions{})
	// Lines 6-9 were covered in base and are uncovered now (line 10 was
	// uncovered on both sides).
	if !strings.Contains(buf.String(), "lost: 6-9  (4 lines newly uncovered)") {
		t.Fatalf("lost-line detail missing:\n%s", buf.String())
	}
}

func TestTextAllIncludesUnchangedFiles(t *testing.T) {
	var buf bytes.Buffer
	Text(&buf, scenario(t), TextOptions{All: true})
	if !strings.Contains(buf.String(), "src/same.go") {
		t.Fatalf("--all should list unchanged files")
	}
}

func TestTextGateOKOnCleanDiff(t *testing.T) {
	base := model.NewReport("lcov")
	cur := model.NewReport("lcov")
	base.File("a.go").AddHits(1, 1)
	cur.File("a.go").AddHits(1, 1)
	var buf bytes.Buffer
	Text(&buf, diff.Compare(base, cur, diff.DefaultOptions()), TextOptions{})
	out := buf.String()
	if !strings.Contains(out, "gate: OK") || !strings.Contains(out, "no per-file coverage movement") {
		t.Fatalf("clean diff output wrong:\n%s", out)
	}
}

func TestJSONIsValidAndCarriesTheSchema(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, scenario(t)); err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if doc["tool"] != "covdrift" || doc["schema_version"] != float64(1) {
		t.Fatalf("envelope wrong: %v", doc)
	}
	gate := doc["gate"].(map[string]any)
	if gate["ok"] != false {
		t.Fatalf("gate.ok should be false")
	}
	if _, isArray := gate["failures"].([]any); !isArray {
		t.Fatalf("gate.failures must be an array, never null")
	}
	files := doc["files"].([]any)
	if len(files) != 5 {
		t.Fatalf("got %d files, want 5", len(files))
	}
	first := files[0].(map[string]any)
	if first["path"] != "src/new.go" || first["status"] != "added" {
		t.Fatalf("files not sorted by path: %v", first)
	}
	if first["base"] != nil {
		t.Fatalf("added file must have base: null")
	}
}

func TestJSONLostLinesAreStructured(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, scenario(t)); err != nil {
		t.Fatalf("JSON() error: %v", err)
	}
	if !strings.Contains(buf.String(), `"lost_lines"`) ||
		!strings.Contains(buf.String(), `"start": 6`) {
		t.Fatalf("structured lost lines missing:\n%s", buf.String())
	}
}

func TestMarkdownTableAndVerdict(t *testing.T) {
	var buf bytes.Buffer
	Markdown(&buf, scenario(t))
	out := buf.String()
	for _, want := range []string{
		"### covdrift: ❌ coverage gate failed",
		"| File | Baseline | Current | Δ | Status |",
		"| `src/parser.go` | 90.0% | 50.0% | -40.0pp | **regressed** |",
		"| `src/new.go` | — | 50.0% | — | added |",
		"newly uncovered lines: 6-9",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("markdown missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "src/same.go") {
		t.Fatalf("markdown must omit unchanged files")
	}
}

func TestMarkdownCleanDiffSaysSo(t *testing.T) {
	base := model.NewReport("lcov")
	cur := model.NewReport("lcov")
	base.File("a.go").AddHits(1, 1)
	cur.File("a.go").AddHits(1, 1)
	var buf bytes.Buffer
	Markdown(&buf, diff.Compare(base, cur, diff.DefaultOptions()))
	out := buf.String()
	if !strings.Contains(out, "✅ coverage gate passed") || !strings.Contains(out, "No per-file coverage movement.") {
		t.Fatalf("clean markdown wrong:\n%s", out)
	}
}

func TestShowTextAndJSON(t *testing.T) {
	r := model.NewReport("goprofile")
	f := r.File("pkg/a.go")
	f.AddHits(1, 2)
	f.MarkSeen(2)
	var buf bytes.Buffer
	ShowText(&buf, r)
	out := buf.String()
	if !strings.Contains(out, "goprofile report, 1 file\n") || !strings.Contains(out, "pkg/a.go") ||
		!strings.Contains(out, "total    1/2 lines covered (50.0%)") {
		t.Fatalf("show text wrong:\n%s", out)
	}
	buf.Reset()
	if err := ShowJSON(&buf, r); err != nil {
		t.Fatalf("ShowJSON error: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("show JSON invalid: %v", err)
	}
	if doc["format"] != "goprofile" {
		t.Fatalf("show JSON format wrong: %v", doc)
	}
}
