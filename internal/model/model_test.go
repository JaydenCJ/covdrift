// Tests for the normalized coverage model: percentage math, merging, and
// the saturation behavior that keeps huge atomic-mode counters safe.
package model

import (
	"math"
	"testing"
)

func TestPercentDefinesEmptyFileAsFullyCovered(t *testing.T) {
	// A file with zero instrumented lines has nothing left to cover; if
	// this returned 0 instead, deleting all code from a file would read
	// as a 100-point regression.
	f := NewFileCoverage("a.go")
	if got := f.Percent(); got != 100 {
		t.Fatalf("empty file Percent() = %v, want 100", got)
	}
}

func TestCoveredCountsOnlyHitLines(t *testing.T) {
	f := NewFileCoverage("a.go")
	f.AddHits(1, 3)
	f.MarkSeen(2)
	f.AddHits(3, 1)
	if f.Total() != 3 || f.Covered() != 2 {
		t.Fatalf("total=%d covered=%d, want 3/2", f.Total(), f.Covered())
	}
	if got := f.Percent(); math.Abs(got-66.666) > 0.01 {
		t.Fatalf("Percent() = %v, want ~66.67", got)
	}
	// MarkSeen on an already-covered line must not zero its count.
	f.MarkSeen(1)
	if f.Lines[1] != 3 {
		t.Fatalf("MarkSeen clobbered an existing count: %d", f.Lines[1])
	}
}

func TestMergeSumsHitsAndPreservesZeroHitLines(t *testing.T) {
	a := NewFileCoverage("a.go")
	a.AddHits(1, 2)
	a.MarkSeen(9)
	b := NewFileCoverage("a.go")
	b.AddHits(1, 3)
	b.MarkSeen(2)
	a.Merge(b)
	if a.Lines[1] != 5 {
		t.Fatalf("merged line 1 = %d, want 5", a.Lines[1])
	}
	if _, ok := a.Lines[2]; !ok {
		t.Fatalf("zero-hit line 2 lost in merge")
	}
	if a.Total() != 3 {
		t.Fatalf("merged total = %d, want 3", a.Total())
	}
}

func TestAddHitsSaturatesInsteadOfOverflowing(t *testing.T) {
	f := NewFileCoverage("a.go")
	f.AddHits(1, math.MaxInt64-1)
	f.AddHits(1, math.MaxInt64-1)
	if f.Lines[1] != math.MaxInt64 {
		t.Fatalf("expected saturation at MaxInt64, got %d", f.Lines[1])
	}
	if f.Covered() != 1 {
		t.Fatalf("saturated line must still count as covered")
	}
}

func TestReportPathsAreSorted(t *testing.T) {
	r := NewReport("lcov")
	for _, p := range []string{"z.go", "a.go", "m/a.go"} {
		r.File(p)
	}
	paths := r.Paths()
	want := []string{"a.go", "m/a.go", "z.go"}
	for i, p := range want {
		if paths[i] != p {
			t.Fatalf("Paths() = %v, want %v", paths, want)
		}
	}
}

func TestReportTotalsSumAcrossFiles(t *testing.T) {
	r := NewReport("lcov")
	f1 := r.File("a.go")
	f1.AddHits(1, 1)
	f1.MarkSeen(2)
	f2 := r.File("b.go")
	f2.AddHits(1, 4)
	covered, total := r.Totals()
	if covered != 2 || total != 3 {
		t.Fatalf("Totals() = %d/%d, want 2/3", covered, total)
	}
}
