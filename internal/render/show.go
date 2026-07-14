package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/JaydenCJ/covdrift/internal/model"
	"github.com/JaydenCJ/covdrift/internal/version"
)

// ShowText writes the normalized view of a single report — the exact
// per-file numbers `diff` would compare — so users can debug path and
// format questions ("what does covdrift think my cobertura file says?").
func ShowText(w io.Writer, r *model.Report) {
	covered, total := r.Totals()
	fmt.Fprintf(w, "covdrift show — %s report, %d %s\n\n", r.Format, len(r.Files), plural(len(r.Files), "file", "files"))
	fmt.Fprintf(w, "%9s %9s %8s  %s\n", "covered", "lines", "pct", "file")
	for _, path := range r.Paths() {
		f := r.Files[path]
		fmt.Fprintf(w, "%9d %9d %7.1f%%  %s\n", f.Covered(), f.Total(), f.Percent(), path)
	}
	fmt.Fprintf(w, "\ntotal    %d/%d lines covered (%.1f%%)\n", covered, total, model.Percent(covered, total))
}

// showEnvelope is the JSON schema for a single normalized report.
type showEnvelope struct {
	Tool          string         `json:"tool"`
	Version       string         `json:"version"`
	SchemaVersion int            `json:"schema_version"`
	Format        string         `json:"format"`
	Totals        jsonSide       `json:"totals"`
	Files         []showFileJSON `json:"files"`
}

type showFileJSON struct {
	Path    string  `json:"path"`
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

// ShowJSON writes the normalized single-report view as stable JSON.
func ShowJSON(w io.Writer, r *model.Report) error {
	covered, total := r.Totals()
	env := showEnvelope{
		Tool:          "covdrift",
		Version:       version.Version,
		SchemaVersion: 1,
		Format:        r.Format,
		Totals:        jsonSide{Covered: covered, Total: total, Percent: round1(model.Percent(covered, total))},
		Files:         make([]showFileJSON, 0, len(r.Files)),
	}
	for _, path := range r.Paths() {
		f := r.Files[path]
		env.Files = append(env.Files, showFileJSON{
			Path: path, Covered: f.Covered(), Total: f.Total(), Percent: round1(f.Percent()),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// round1 rounds to one decimal place for display-stable JSON.
func round1(p float64) float64 {
	return float64(int(p*10+0.5)) / 10
}
