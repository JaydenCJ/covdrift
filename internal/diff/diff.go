// Package diff compares two normalized coverage reports file by file and
// decides whether the change constitutes a regression. This is the core
// idea of covdrift: instead of an absolute threshold ("the repo must stay
// above 80%") that punishes whichever PR happens to be open when someone
// else's debt lands, each file is compared against *its own* baseline.
package diff

import (
	"fmt"
	"math"
	"sort"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// Status classifies one file's coverage movement between two runs.
type Status string

const (
	StatusRegressed Status = "regressed" // coverage dropped beyond tolerance
	StatusImproved  Status = "improved"  // coverage went up
	StatusUnchanged Status = "unchanged" // within tolerance / identical
	StatusAdded     Status = "added"     // present only in the current run
	StatusRemoved   Status = "removed"   // present only in the baseline
)

// Options tune classification and gating.
type Options struct {
	// TolerancePP is the per-file drop, in percentage points, that is
	// still acceptable. 0 (the default) means any measurable drop fails.
	TolerancePP float64
	// TotalTolerancePP, when >= 0, additionally gates the overall
	// coverage delta. Negative disables the total gate.
	TotalTolerancePP float64
	// MinNewPct, when >= 0, requires every file that is new in the
	// current run to be covered at least this much. Negative disables it.
	MinNewPct float64
	// MinLines exempts small files from gating: a file whose larger side
	// has fewer instrumented lines than MinLines can swing wildly from a
	// single line and never fails the gate (it is still reported).
	MinLines int
}

// DefaultOptions returns the zero-tolerance defaults.
func DefaultOptions() Options {
	return Options{TolerancePP: 0, TotalTolerancePP: -1, MinNewPct: -1, MinLines: 0}
}

// FileDelta is the comparison result for one file path.
type FileDelta struct {
	Path   string
	Status Status
	// Base/Cur are nil for added/removed files respectively.
	Base, Cur *model.FileCoverage
	// BasePct/CurPct are the rounded percentages (see roundPct); DeltaPP
	// is their difference in percentage points.
	BasePct, CurPct, DeltaPP float64
	// Exempt marks a regressed file that does not gate because it is
	// smaller than Options.MinLines.
	Exempt bool
	// LostLines are the lines that were covered in the baseline but are
	// instrumented-and-uncovered in the current run — the exact places a
	// reviewer should look. Only populated for regressed files.
	LostLines []LineRange
}

// Result is the full diff of two coverage runs.
type Result struct {
	Files []FileDelta // sorted by path

	Regressed, Improved, Unchanged, Added, Removed int

	BaseCovered, BaseTotal int
	CurCovered, CurTotal   int
	BasePct, CurPct        float64
	TotalDeltaPP           float64

	// GateFailures holds one human-readable reason per gate breach; the
	// gate passes iff it is empty.
	GateFailures []string
}

// GateOK reports whether the diff passes every configured gate.
func (r *Result) GateOK() bool { return len(r.GateFailures) == 0 }

// Compare diffs current against base under opts.
func Compare(base, current *model.Report, opts Options) *Result {
	res := &Result{}

	paths := unionPaths(base, current)
	for _, path := range paths {
		b, hasBase := base.Files[path]
		c, hasCur := current.Files[path]
		fd := FileDelta{Path: path}
		switch {
		case hasBase && hasCur:
			fd.Base, fd.Cur = b, c
			fd.BasePct = roundPct(b.Percent())
			fd.CurPct = roundPct(c.Percent())
			fd.DeltaPP = fd.CurPct - fd.BasePct
			switch {
			case fd.DeltaPP < -(opts.TolerancePP + epsilon):
				fd.Status = StatusRegressed
				fd.LostLines = lostLines(b, c)
				if biggerSide(b, c) < opts.MinLines {
					fd.Exempt = true
				} else {
					res.GateFailures = append(res.GateFailures, fmt.Sprintf(
						"%s: %.1f%% → %.1f%% (%+.1fpp, tolerance %.1fpp)",
						path, fd.BasePct, fd.CurPct, fd.DeltaPP, opts.TolerancePP))
				}
			case fd.DeltaPP > epsilon:
				fd.Status = StatusImproved
			default:
				fd.Status = StatusUnchanged
			}
		case hasCur:
			fd.Cur = c
			fd.CurPct = roundPct(c.Percent())
			fd.Status = StatusAdded
			if opts.MinNewPct >= 0 && fd.CurPct < opts.MinNewPct-epsilon && biggerSide(nil, c) >= opts.MinLines {
				res.GateFailures = append(res.GateFailures, fmt.Sprintf(
					"%s: new file covered %.1f%%, below --min-new %.1f%%",
					path, fd.CurPct, opts.MinNewPct))
			}
		default:
			fd.Base = b
			fd.BasePct = roundPct(b.Percent())
			fd.Status = StatusRemoved
		}
		res.Files = append(res.Files, fd)
		switch fd.Status {
		case StatusRegressed:
			res.Regressed++
		case StatusImproved:
			res.Improved++
		case StatusUnchanged:
			res.Unchanged++
		case StatusAdded:
			res.Added++
		case StatusRemoved:
			res.Removed++
		}
	}

	res.BaseCovered, res.BaseTotal = base.Totals()
	res.CurCovered, res.CurTotal = current.Totals()
	res.BasePct = roundPct(model.Percent(res.BaseCovered, res.BaseTotal))
	res.CurPct = roundPct(model.Percent(res.CurCovered, res.CurTotal))
	res.TotalDeltaPP = res.CurPct - res.BasePct

	if opts.TotalTolerancePP >= 0 && res.TotalDeltaPP < -(opts.TotalTolerancePP+epsilon) {
		res.GateFailures = append(res.GateFailures, fmt.Sprintf(
			"total: %.1f%% → %.1f%% (%+.1fpp, total tolerance %.1fpp)",
			res.BasePct, res.CurPct, res.TotalDeltaPP, opts.TotalTolerancePP))
	}
	return res
}

// epsilon absorbs float noise so that "the same fraction computed twice"
// never counts as a drop. Percentages are additionally rounded to 4
// decimal places before comparison, so classification is stable.
const epsilon = 1e-9

// roundPct rounds a percentage to 4 decimal places — far finer than any
// display, coarse enough to make binary float division deterministic.
func roundPct(p float64) float64 {
	return math.Round(p*1e4) / 1e4
}

// biggerSide is the larger instrumented-line count of the two sides.
func biggerSide(b, c *model.FileCoverage) int {
	n := 0
	if b != nil {
		n = b.Total()
	}
	if c != nil && c.Total() > n {
		n = c.Total()
	}
	return n
}

// lostLines finds lines covered in the baseline that are instrumented but
// uncovered now. Lines that vanished from instrumentation entirely
// (deleted code) are not "lost" — there is nothing left to cover.
func lostLines(base, cur *model.FileCoverage) []LineRange {
	var lines []int
	for ln, hits := range base.Lines {
		if hits <= 0 {
			continue
		}
		curHits, instrumented := cur.Lines[ln]
		if instrumented && curHits == 0 {
			lines = append(lines, ln)
		}
	}
	return Ranges(lines)
}

// unionPaths returns the sorted union of both reports' paths.
func unionPaths(a, b *model.Report) []string {
	seen := make(map[string]bool, len(a.Files)+len(b.Files))
	var paths []string
	for p := range a.Files {
		seen[p] = true
		paths = append(paths, p)
	}
	for p := range b.Files {
		if !seen[p] {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}
