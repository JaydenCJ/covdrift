// Tests for the diff engine — classification, tolerances, gating, and
// lost-line computation. Getting these wrong either blocks innocent PRs
// (false regression) or waves real coverage loss through (false pass);
// both failure modes are the exact problem covdrift exists to fix.
package diff

import (
	"strings"
	"testing"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// file builds a FileCoverage where covered lines are 1..covered and
// uncovered instrumented lines continue up to total.
func file(r *model.Report, path string, covered, total int) *model.FileCoverage {
	f := r.File(path)
	for ln := 1; ln <= total; ln++ {
		if ln <= covered {
			f.AddHits(ln, 1)
		} else {
			f.MarkSeen(ln)
		}
	}
	return f
}

func twoReports() (*model.Report, *model.Report) {
	return model.NewReport("lcov"), model.NewReport("lcov")
}

func TestDropBeyondZeroToleranceRegressesAndFailsGate(t *testing.T) {
	base, cur := twoReports()
	file(base, "a.go", 9, 10) // 90%
	file(cur, "a.go", 7, 10)  // 70%
	res := Compare(base, cur, DefaultOptions())
	if res.Regressed != 1 || res.Files[0].Status != StatusRegressed {
		t.Fatalf("expected regression, got %+v", res.Files[0])
	}
	if res.GateOK() {
		t.Fatalf("gate must fail on regression at zero tolerance")
	}
	if !strings.Contains(res.GateFailures[0], "a.go") || !strings.Contains(res.GateFailures[0], "-20.0pp") {
		t.Fatalf("gate reason should name file and delta: %q", res.GateFailures[0])
	}
}

func TestDropWithinToleranceIsUnchanged(t *testing.T) {
	base, cur := twoReports()
	file(base, "a.go", 90, 100) // 90%
	file(cur, "a.go", 89, 100)  // 89%: a 1pp drop
	res := Compare(base, cur, Options{TolerancePP: 1.5, TotalTolerancePP: -1, MinNewPct: -1})
	if res.Files[0].Status != StatusUnchanged {
		t.Fatalf("1pp drop under 1.5pp tolerance should be unchanged, got %s", res.Files[0].Status)
	}
	if !res.GateOK() {
		t.Fatalf("gate must pass: %v", res.GateFailures)
	}
}

func TestDropExactlyAtToleranceBoundaryPasses(t *testing.T) {
	// tolerance is inclusive: a drop of exactly N pp is allowed.
	base, cur := twoReports()
	file(base, "a.go", 90, 100)
	file(cur, "a.go", 88, 100) // exactly -2pp
	res := Compare(base, cur, Options{TolerancePP: 2, TotalTolerancePP: -1, MinNewPct: -1})
	if res.Files[0].Status == StatusRegressed {
		t.Fatalf("drop exactly at tolerance must not regress")
	}
}

func TestImprovementIsReportedAndPasses(t *testing.T) {
	base, cur := twoReports()
	file(base, "a.go", 5, 10)
	file(cur, "a.go", 8, 10)
	res := Compare(base, cur, DefaultOptions())
	if res.Improved != 1 || res.Files[0].Status != StatusImproved {
		t.Fatalf("expected improvement, got %+v", res.Files[0])
	}
	if !res.GateOK() {
		t.Fatalf("improvement must never fail the gate")
	}
}

func TestIdenticalRunsAreUnchanged(t *testing.T) {
	base, cur := twoReports()
	file(base, "a.go", 1, 3) // 33.333…% — exercises float stability
	file(cur, "a.go", 1, 3)
	res := Compare(base, cur, DefaultOptions())
	if res.Files[0].Status != StatusUnchanged || res.Files[0].DeltaPP != 0 {
		t.Fatalf("identical thirds must diff to exactly zero, got %+v", res.Files[0])
	}
}

func TestSameRatioDifferentLineCountsIsUnchanged(t *testing.T) {
	// 2/3 and 4/6 are the same percentage computed through different
	// divisions; float noise must not turn this into a regression.
	base, cur := twoReports()
	file(base, "a.go", 2, 3)
	file(cur, "a.go", 4, 6)
	res := Compare(base, cur, DefaultOptions())
	if res.Files[0].Status != StatusUnchanged {
		t.Fatalf("equal ratios classified as %s (delta %v)", res.Files[0].Status, res.Files[0].DeltaPP)
	}
}

func TestAddedFileWithoutMinNewPasses(t *testing.T) {
	base, cur := twoReports()
	file(cur, "new.go", 0, 10) // brand new, 0% covered
	res := Compare(base, cur, DefaultOptions())
	if res.Added != 1 || res.Files[0].Status != StatusAdded {
		t.Fatalf("expected added file, got %+v", res.Files[0])
	}
	if !res.GateOK() {
		t.Fatalf("added files do not gate unless --min-new is set")
	}
}

func TestMinNewGatesUncoveredNewFiles(t *testing.T) {
	base, cur := twoReports()
	file(cur, "new.go", 3, 10) // 30%
	opts := DefaultOptions()
	opts.MinNewPct = 50
	res := Compare(base, cur, opts)
	if res.GateOK() {
		t.Fatalf("new file at 30%% must fail --min-new 50")
	}
	if !strings.Contains(res.GateFailures[0], "new.go") {
		t.Fatalf("failure should name the file: %q", res.GateFailures[0])
	}
	// At or above the bar it passes.
	cur2 := model.NewReport("lcov")
	file(cur2, "new.go", 5, 10)
	if res := Compare(base, cur2, opts); !res.GateOK() {
		t.Fatalf("50%% new file must pass --min-new 50: %v", res.GateFailures)
	}
}

func TestRemovedFileIsInformationalOnly(t *testing.T) {
	base, cur := twoReports()
	file(base, "old.go", 9, 10)
	res := Compare(base, cur, DefaultOptions())
	if res.Removed != 1 || res.Files[0].Status != StatusRemoved {
		t.Fatalf("expected removed file, got %+v", res.Files[0])
	}
	if !res.GateOK() {
		t.Fatalf("deleting a file must not fail the gate")
	}
}

func TestMinLinesExemptsTinyFilesFromGating(t *testing.T) {
	// A 4-line file dropping one covered line swings 25pp; --min-lines
	// keeps such noise out of the gate while still reporting it.
	base, cur := twoReports()
	file(base, "tiny.go", 4, 4)
	file(cur, "tiny.go", 3, 4)
	opts := DefaultOptions()
	opts.MinLines = 10
	res := Compare(base, cur, opts)
	if res.Files[0].Status != StatusRegressed || !res.Files[0].Exempt {
		t.Fatalf("tiny file should be regressed-but-exempt, got %+v", res.Files[0])
	}
	if !res.GateOK() {
		t.Fatalf("exempt regression must not fail the gate: %v", res.GateFailures)
	}
}

func TestTotalToleranceGatesOverallDrift(t *testing.T) {
	// Each file drops within its per-file tolerance, but the aggregate
	// drop exceeds the total budget.
	base, cur := twoReports()
	file(base, "a.go", 100, 100)
	file(base, "b.go", 100, 100)
	file(cur, "a.go", 96, 100)
	file(cur, "b.go", 96, 100)
	opts := Options{TolerancePP: 5, TotalTolerancePP: 3, MinNewPct: -1}
	res := Compare(base, cur, opts)
	if res.Regressed != 0 {
		t.Fatalf("per-file drops are within tolerance")
	}
	if res.GateOK() {
		t.Fatalf("-4pp total must breach a 3pp total tolerance")
	}
	if !strings.Contains(res.GateFailures[0], "total:") {
		t.Fatalf("failure should be attributed to the total: %q", res.GateFailures[0])
	}
}

func TestLostLinesPointAtNewlyUncoveredCode(t *testing.T) {
	base, cur := twoReports()
	b := base.File("a.go")
	c := cur.File("a.go")
	for ln := 1; ln <= 10; ln++ {
		b.AddHits(ln, 1)
	}
	// Current run: lines 3-5 and 9 lost coverage; line 10 was deleted
	// (not instrumented any more) and must NOT appear as lost.
	for ln := 1; ln <= 9; ln++ {
		if ln >= 3 && ln <= 5 || ln == 9 {
			c.MarkSeen(ln)
		} else {
			c.AddHits(ln, 1)
		}
	}
	res := Compare(base, cur, DefaultOptions())
	fd := res.Files[0]
	if fd.Status != StatusRegressed {
		t.Fatalf("expected regression, got %s", fd.Status)
	}
	if len(fd.LostLines) != 2 || fd.LostLines[0] != (LineRange{3, 5}) || fd.LostLines[1] != (LineRange{9, 9}) {
		t.Fatalf("lost lines = %v, want [3-5, 9]", fd.LostLines)
	}
	if TotalLines(fd.LostLines) != 4 {
		t.Fatalf("lost line count = %d, want 4", TotalLines(fd.LostLines))
	}
}

func TestEmptyBaselineTreatsEverythingAsAdded(t *testing.T) {
	base, cur := twoReports()
	file(cur, "a.go", 1, 2)
	file(cur, "b.go", 2, 2)
	res := Compare(base, cur, DefaultOptions())
	if res.Added != 2 || !res.GateOK() {
		t.Fatalf("first run against an empty baseline must pass: %+v", res)
	}
}

func TestFilesAreSortedByPath(t *testing.T) {
	base, cur := twoReports()
	file(base, "z.go", 1, 1)
	file(cur, "z.go", 1, 1)
	file(cur, "a.go", 1, 1)
	file(base, "m.go", 1, 1)
	res := Compare(base, cur, DefaultOptions())
	want := []string{"a.go", "m.go", "z.go"}
	for i, p := range want {
		if res.Files[i].Path != p {
			t.Fatalf("order = %v", res.Files)
		}
	}
}

func TestRangesGroupsAndFormats(t *testing.T) {
	rs := Ranges([]int{9, 1, 2, 3, 5, 5, 12})
	want := []LineRange{{1, 3}, {5, 5}, {9, 9}, {12, 12}}
	if len(rs) != len(want) {
		t.Fatalf("Ranges = %v, want %v", rs, want)
	}
	for i := range want {
		if rs[i] != want[i] {
			t.Fatalf("Ranges = %v, want %v", rs, want)
		}
	}
	if got := FormatRanges(rs, 2); got != "1-3, 5 (+2 more ranges)" {
		t.Fatalf("FormatRanges = %q", got)
	}
	if got := FormatRanges(rs, 0); got != "1-3, 5, 9, 12" {
		t.Fatalf("unlimited FormatRanges = %q", got)
	}
	if got := FormatRanges(nil, 3); got != "" {
		t.Fatalf("empty FormatRanges = %q", got)
	}
}

func TestTotalsAggregateBothSides(t *testing.T) {
	base, cur := twoReports()
	file(base, "a.go", 8, 10)
	file(base, "b.go", 2, 10)
	file(cur, "a.go", 8, 10)
	file(cur, "b.go", 6, 10)
	res := Compare(base, cur, DefaultOptions())
	if res.BaseCovered != 10 || res.BaseTotal != 20 || res.CurCovered != 14 || res.CurTotal != 20 {
		t.Fatalf("totals wrong: %+v", res)
	}
	if res.BasePct != 50 || res.CurPct != 70 || res.TotalDeltaPP != 20 {
		t.Fatalf("total percentages wrong: %v → %v (%v)", res.BasePct, res.CurPct, res.TotalDeltaPP)
	}
}
