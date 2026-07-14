// Package render turns diff results into terminal text, stable JSON, and
// PR-ready Markdown. Renderers are pure functions of the Result, so
// identical diffs produce byte-identical output on every machine.
package render

import (
	"fmt"
	"io"

	"github.com/JaydenCJ/covdrift/internal/diff"
)

// TextOptions tune the human-readable renderer.
type TextOptions struct {
	// All lists unchanged files too; by default only movement is shown.
	All bool
	// MaxRanges caps how many lost-line ranges print per file.
	MaxRanges int
}

// Text writes the human-readable diff report.
func Text(w io.Writer, res *diff.Result, opts TextOptions) {
	if opts.MaxRanges == 0 {
		opts.MaxRanges = 6
	}
	fmt.Fprintf(w, "covdrift — baseline vs current\n\n")
	fmt.Fprintf(w, "total    %s → %s  %s  (%d/%d → %d/%d lines)\n",
		pct(res.BasePct), pct(res.CurPct), delta(res.TotalDeltaPP),
		res.BaseCovered, res.BaseTotal, res.CurCovered, res.CurTotal)
	fmt.Fprintf(w, "files    %d compared · %d regressed · %d improved · %d added · %d removed · %d unchanged\n",
		len(res.Files), res.Regressed, res.Improved, res.Added, res.Removed, res.Unchanged)

	shown := 0
	header := false
	writeHeader := func() {
		if !header {
			fmt.Fprintf(w, "\n%-9s %7s %7s %9s  %s\n", "status", "base", "cur", "delta", "file")
			header = true
		}
	}
	for _, fd := range res.Files {
		if fd.Status == diff.StatusUnchanged && !opts.All {
			continue
		}
		writeHeader()
		shown++
		switch fd.Status {
		case diff.StatusRegressed:
			label := "REGRESS"
			if fd.Exempt {
				label = "regress*"
			}
			fmt.Fprintf(w, "%-9s %7s %7s %9s  %s\n", label, pct(fd.BasePct), pct(fd.CurPct), delta(fd.DeltaPP), fd.Path)
			if len(fd.LostLines) > 0 {
				fmt.Fprintf(w, "%-9s   lost: %s  (%d lines newly uncovered)\n",
					"", diff.FormatRanges(fd.LostLines, opts.MaxRanges), diff.TotalLines(fd.LostLines))
			}
		case diff.StatusImproved:
			fmt.Fprintf(w, "%-9s %7s %7s %9s  %s\n", "improve", pct(fd.BasePct), pct(fd.CurPct), delta(fd.DeltaPP), fd.Path)
		case diff.StatusAdded:
			fmt.Fprintf(w, "%-9s %7s %7s %9s  %s\n", "added", "—", pct(fd.CurPct), "—", fd.Path)
		case diff.StatusRemoved:
			fmt.Fprintf(w, "%-9s %7s %7s %9s  %s\n", "removed", pct(fd.BasePct), "—", "—", fd.Path)
		default:
			fmt.Fprintf(w, "%-9s %7s %7s %9s  %s\n", "same", pct(fd.BasePct), pct(fd.CurPct), delta(fd.DeltaPP), fd.Path)
		}
	}
	if shown == 0 {
		fmt.Fprintf(w, "\nno per-file coverage movement\n")
	}
	if exempt := countExempt(res); exempt > 0 {
		fmt.Fprintf(w, "\n* %d regressed %s below --min-lines: reported, not gated\n",
			exempt, plural(exempt, "file", "files"))
	}

	fmt.Fprintln(w)
	if res.GateOK() {
		fmt.Fprintf(w, "gate: OK\n")
		return
	}
	fmt.Fprintf(w, "gate: FAIL — %d %s\n",
		len(res.GateFailures), plural(len(res.GateFailures), "breach", "breaches"))
	for _, reason := range res.GateFailures {
		fmt.Fprintf(w, "  · %s\n", reason)
	}
}

// countExempt counts regressed-but-exempt files for the footnote.
func countExempt(res *diff.Result) int {
	n := 0
	for _, fd := range res.Files {
		if fd.Exempt {
			n++
		}
	}
	return n
}

// plural picks the singular form for exactly one and the plural form
// otherwise, so output never reads "1 breach(es)".
func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// pct renders a percentage with one decimal.
func pct(p float64) string { return fmt.Sprintf("%.1f%%", p) }

// delta renders a signed percentage-point delta; exact zero prints flat.
func delta(pp float64) string {
	if pp == 0 {
		return "0.0pp"
	}
	return fmt.Sprintf("%+.1fpp", pp)
}
