package render

import (
	"fmt"
	"io"

	"github.com/JaydenCJ/covdrift/internal/diff"
)

// Markdown writes a PR-comment-ready summary: a verdict line, a table of
// every file that moved, and the gate breaches as a list. Unchanged files
// are always omitted — PR comments should be short.
func Markdown(w io.Writer, res *diff.Result) {
	verdict := "✅ coverage gate passed"
	if !res.GateOK() {
		verdict = "❌ coverage gate failed"
	}
	fmt.Fprintf(w, "### covdrift: %s\n\n", verdict)
	fmt.Fprintf(w, "**Total:** %s → %s (%s) · %d regressed · %d improved · %d added · %d removed\n\n",
		pct(res.BasePct), pct(res.CurPct), delta(res.TotalDeltaPP),
		res.Regressed, res.Improved, res.Added, res.Removed)

	moved := false
	for _, fd := range res.Files {
		if fd.Status != diff.StatusUnchanged {
			moved = true
			break
		}
	}
	if !moved {
		fmt.Fprintf(w, "No per-file coverage movement.\n")
		return
	}

	fmt.Fprintf(w, "| File | Baseline | Current | Δ | Status |\n")
	fmt.Fprintf(w, "|---|---:|---:|---:|---|\n")
	for _, fd := range res.Files {
		switch fd.Status {
		case diff.StatusUnchanged:
			continue
		case diff.StatusAdded:
			fmt.Fprintf(w, "| `%s` | — | %s | — | added |\n", fd.Path, pct(fd.CurPct))
		case diff.StatusRemoved:
			fmt.Fprintf(w, "| `%s` | %s | — | — | removed |\n", fd.Path, pct(fd.BasePct))
		default:
			status := string(fd.Status)
			if fd.Status == diff.StatusRegressed {
				status = "**regressed**"
				if fd.Exempt {
					status = "regressed (below min-lines)"
				}
			}
			fmt.Fprintf(w, "| `%s` | %s | %s | %s | %s |\n",
				fd.Path, pct(fd.BasePct), pct(fd.CurPct), delta(fd.DeltaPP), status)
		}
	}

	// Lost-line detail per regressed file, the part reviewers act on.
	for _, fd := range res.Files {
		if fd.Status == diff.StatusRegressed && len(fd.LostLines) > 0 {
			fmt.Fprintf(w, "\n`%s` — newly uncovered lines: %s\n",
				fd.Path, diff.FormatRanges(fd.LostLines, 6))
		}
	}

	if !res.GateOK() {
		fmt.Fprintf(w, "\n")
		for _, reason := range res.GateFailures {
			fmt.Fprintf(w, "- ⚠️ %s\n", reason)
		}
	}
}
