// Package model defines the normalized coverage report every parser
// produces: per-file, per-line hit counts. Everything downstream — path
// normalization, diffing, rendering — works on this one shape, which is
// what makes covdrift format-agnostic.
package model

import (
	"math"
	"sort"
)

// FileCoverage is the line-level coverage of a single file: a map from
// 1-based line number to hit count. Only instrumented lines appear in the
// map; a count of zero means "instrumented but never executed".
type FileCoverage struct {
	Path  string
	Lines map[int]int64
}

// NewFileCoverage returns an empty coverage record for path.
func NewFileCoverage(path string) *FileCoverage {
	return &FileCoverage{Path: path, Lines: make(map[int]int64)}
}

// AddHits records count executions of line, accumulating on top of any
// previous count with int64 saturation. Accumulation (rather than
// replacement) is what makes merging duplicate records — parallel test
// shards, repeated SF blocks, overlapping profile blocks — correct.
func (f *FileCoverage) AddHits(line int, count int64) {
	f.Lines[line] = saturatingAdd(f.Lines[line], count)
}

// MarkSeen ensures line is present as instrumented even with zero hits.
func (f *FileCoverage) MarkSeen(line int) {
	if _, ok := f.Lines[line]; !ok {
		f.Lines[line] = 0
	}
}

// Total is the number of instrumented lines.
func (f *FileCoverage) Total() int { return len(f.Lines) }

// Covered is the number of instrumented lines with at least one hit.
func (f *FileCoverage) Covered() int {
	n := 0
	for _, c := range f.Lines {
		if c > 0 {
			n++
		}
	}
	return n
}

// Percent is the line coverage percentage in [0, 100]. A file with no
// instrumented lines is fully covered by definition (there is nothing
// left to execute), so it reports 100.
func (f *FileCoverage) Percent() float64 {
	return Percent(f.Covered(), f.Total())
}

// Percent computes covered/total as a percentage, defining 0/0 as 100.
func Percent(covered, total int) float64 {
	if total == 0 {
		return 100
	}
	return float64(covered) / float64(total) * 100
}

// Report is a normalized coverage run: one FileCoverage per source path,
// tagged with the input format it was parsed from.
type Report struct {
	Format string // "lcov", "cobertura", or "goprofile"
	Files  map[string]*FileCoverage
}

// NewReport returns an empty report for the given input format.
func NewReport(format string) *Report {
	return &Report{Format: format, Files: make(map[string]*FileCoverage)}
}

// File returns the coverage record for path, creating it on first use.
func (r *Report) File(path string) *FileCoverage {
	f, ok := r.Files[path]
	if !ok {
		f = NewFileCoverage(path)
		r.Files[path] = f
	}
	return f
}

// Paths returns every file path in the report, sorted, so all downstream
// output is deterministic regardless of map iteration order.
func (r *Report) Paths() []string {
	paths := make([]string, 0, len(r.Files))
	for p := range r.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

// Totals sums covered and instrumented lines across every file.
func (r *Report) Totals() (covered, total int) {
	for _, f := range r.Files {
		covered += f.Covered()
		total += f.Total()
	}
	return covered, total
}

// Merge folds other's per-line hits into f (line-wise saturating sum).
func (f *FileCoverage) Merge(other *FileCoverage) {
	for line, count := range other.Lines {
		if count == 0 {
			f.MarkSeen(line)
			continue
		}
		f.AddHits(line, count)
	}
}

// saturatingAdd adds two non-negative counts without overflowing int64.
// Coverage counters from long-running atomic-mode runs can be enormous;
// covered-ness only needs "greater than zero" to stay true.
func saturatingAdd(a, b int64) int64 {
	if a > math.MaxInt64-b {
		return math.MaxInt64
	}
	return a + b
}
