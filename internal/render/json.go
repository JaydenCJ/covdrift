package render

import (
	"encoding/json"
	"io"

	"github.com/JaydenCJ/covdrift/internal/diff"
	"github.com/JaydenCJ/covdrift/internal/version"
)

// jsonEnvelope is the stable machine-readable schema. schema_version is
// bumped on any breaking change so downstream tooling can pin against it.
type jsonEnvelope struct {
	Tool          string     `json:"tool"`
	Version       string     `json:"version"`
	SchemaVersion int        `json:"schema_version"`
	Totals        jsonTotals `json:"totals"`
	Files         []jsonFile `json:"files"`
	Gate          jsonGate   `json:"gate"`
	Counts        jsonCounts `json:"counts"`
}

type jsonTotals struct {
	BaseCovered int     `json:"base_covered"`
	BaseTotal   int     `json:"base_total"`
	BasePct     float64 `json:"base_percent"`
	CurCovered  int     `json:"current_covered"`
	CurTotal    int     `json:"current_total"`
	CurPct      float64 `json:"current_percent"`
	DeltaPP     float64 `json:"delta_pp"`
}

type jsonCounts struct {
	Regressed int `json:"regressed"`
	Improved  int `json:"improved"`
	Unchanged int `json:"unchanged"`
	Added     int `json:"added"`
	Removed   int `json:"removed"`
}

type jsonFile struct {
	Path      string           `json:"path"`
	Status    string           `json:"status"`
	Base      *jsonSide        `json:"base"`
	Current   *jsonSide        `json:"current"`
	DeltaPP   *float64         `json:"delta_pp"`
	Exempt    bool             `json:"exempt,omitempty"`
	LostLines []diff.LineRange `json:"lost_lines,omitempty"`
}

type jsonSide struct {
	Covered int     `json:"covered"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

type jsonGate struct {
	OK       bool     `json:"ok"`
	Failures []string `json:"failures"`
}

// JSON writes the diff result as indented, key-stable JSON.
func JSON(w io.Writer, res *diff.Result) error {
	env := jsonEnvelope{
		Tool:          "covdrift",
		Version:       version.Version,
		SchemaVersion: 1,
		Totals: jsonTotals{
			BaseCovered: res.BaseCovered, BaseTotal: res.BaseTotal, BasePct: res.BasePct,
			CurCovered: res.CurCovered, CurTotal: res.CurTotal, CurPct: res.CurPct,
			DeltaPP: res.TotalDeltaPP,
		},
		Counts: jsonCounts{
			Regressed: res.Regressed, Improved: res.Improved, Unchanged: res.Unchanged,
			Added: res.Added, Removed: res.Removed,
		},
		Gate: jsonGate{OK: res.GateOK(), Failures: emptyNotNull(res.GateFailures)},
	}
	env.Files = make([]jsonFile, 0, len(res.Files))
	for _, fd := range res.Files {
		jf := jsonFile{Path: fd.Path, Status: string(fd.Status), Exempt: fd.Exempt, LostLines: fd.LostLines}
		if fd.Base != nil {
			jf.Base = &jsonSide{Covered: fd.Base.Covered(), Total: fd.Base.Total(), Percent: fd.BasePct}
		}
		if fd.Cur != nil {
			jf.Current = &jsonSide{Covered: fd.Cur.Covered(), Total: fd.Cur.Total(), Percent: fd.CurPct}
		}
		if fd.Base != nil && fd.Cur != nil {
			d := fd.DeltaPP
			jf.DeltaPP = &d
		}
		env.Files = append(env.Files, jf)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

// emptyNotNull keeps `"failures": []` instead of `null` in the JSON.
func emptyNotNull(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
