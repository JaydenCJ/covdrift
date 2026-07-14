// Package pathnorm reconciles the file paths of two coverage reports so
// they can be diffed. Different formats — and different CI machines —
// disagree on paths for the same source file: lcov often writes absolute
// build paths, cobertura writes package-relative paths, Go writes module
// paths. Normalization (slash canonicalization, prefix stripping, glob
// filtering) is applied identically to both sides so files line up.
package pathnorm

import (
	"strings"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// Options control path normalization and filtering.
type Options struct {
	// StripPrefixes are removed from the front of every path; the first
	// matching prefix wins. "/build/repo/" turns "/build/repo/src/a.go"
	// into "src/a.go".
	StripPrefixes []string
	// Include, when non-empty, keeps only paths matching at least one
	// glob. Exclude then removes matching paths; exclude wins over
	// include.
	Include []string
	Exclude []string
}

// Apply returns a copy of report with normalized, filtered paths. Two
// input paths that normalize to the same output path are merged (their
// per-line counts are summed), which is exactly what should happen when
// stripping shard-specific build prefixes.
func Apply(report *model.Report, opts Options) *model.Report {
	out := model.NewReport(report.Format)
	for _, path := range report.Paths() {
		normalized := Normalize(path, opts.StripPrefixes)
		if !selected(normalized, opts) {
			continue
		}
		out.File(normalized).Merge(report.Files[path])
	}
	return out
}

// Normalize canonicalizes one path: backslashes become slashes (Windows
// emitters), a leading "./" is dropped, and the first matching strip
// prefix is removed.
func Normalize(path string, stripPrefixes []string) string {
	p := strings.ReplaceAll(path, `\`, "/")
	p = strings.TrimPrefix(p, "./")
	for _, prefix := range stripPrefixes {
		prefix = strings.ReplaceAll(prefix, `\`, "/")
		if rest, ok := strings.CutPrefix(p, prefix); ok {
			p = strings.TrimPrefix(rest, "/")
			break
		}
	}
	return p
}

// selected applies include/exclude globs to a normalized path.
func selected(path string, opts Options) bool {
	if len(opts.Include) > 0 {
		keep := false
		for _, g := range opts.Include {
			if Match(g, path) {
				keep = true
				break
			}
		}
		if !keep {
			return false
		}
	}
	for _, g := range opts.Exclude {
		if Match(g, path) {
			return false
		}
	}
	return true
}
