package parse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// LCOV parses an lcov tracefile (the `.info` format written by lcov,
// geninfo, and virtually every JS/TS coverage reporter via
// `--reporter=lcov`). Only the line-coverage records matter to covdrift:
//
//	SF:<path>            start of a per-file record
//	DA:<line>,<count>    execution count for one instrumented line
//	end_of_record        end of the current record
//
// Function (FN/FNDA/FNF/FNH), branch (BRDA/BRF/BRH), summary (LF/LH), and
// test-name (TN) lines are validated as belonging inside a record where
// required, but otherwise ignored. Duplicate SF records for the same path
// (common when shards are concatenated with `cat`) are merged by summing
// per-line counts, matching `lcov --add-tracefile` semantics.
func LCOV(data []byte) (*model.Report, error) {
	report := model.NewReport(FormatLCOV)
	var current *model.FileCoverage

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "SF:"):
			if current != nil {
				return nil, fmt.Errorf("lcov line %d: SF record opened before previous end_of_record", lineNo)
			}
			path := strings.TrimSpace(line[len("SF:"):])
			if path == "" {
				return nil, fmt.Errorf("lcov line %d: SF record with empty path", lineNo)
			}
			current = report.File(path)
		case line == "end_of_record":
			if current == nil {
				return nil, fmt.Errorf("lcov line %d: end_of_record without an open SF record", lineNo)
			}
			current = nil
		case strings.HasPrefix(line, "DA:"):
			if current == nil {
				return nil, fmt.Errorf("lcov line %d: DA outside of an SF record", lineNo)
			}
			ln, count, err := parseDA(line[len("DA:"):])
			if err != nil {
				return nil, fmt.Errorf("lcov line %d: %v", lineNo, err)
			}
			if count == 0 {
				current.MarkSeen(ln)
			} else {
				current.AddHits(ln, count)
			}
		default:
			// TN:, FN*, BRDA/BRF/BRH, LF/LH, VER: — recognized lcov
			// noise; anything else is a corrupt or truncated file.
			if !knownLCOVPrefix(line) {
				return nil, fmt.Errorf("lcov line %d: unrecognized directive %q", lineNo, truncate(line, 40))
			}
		}
	}
	// A trailing record without end_of_record (truncated upload) is an
	// error: silently accepting it would understate the baseline and let
	// a real regression pass the gate.
	if current != nil {
		return nil, fmt.Errorf("lcov: unterminated SF record for %q (missing end_of_record)", current.Path)
	}
	return report, nil
}

// parseDA parses the payload of a DA line: "<line>,<count>[,<checksum>]".
func parseDA(payload string) (line int, count int64, err error) {
	parts := strings.Split(payload, ",")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, 0, fmt.Errorf("malformed DA payload %q", truncate(payload, 40))
	}
	line, err = strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || line < 1 {
		return 0, 0, fmt.Errorf("invalid DA line number %q", parts[0])
	}
	count, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || count < 0 {
		return 0, 0, fmt.Errorf("invalid DA hit count %q", parts[1])
	}
	return line, count, nil
}

// knownLCOVPrefix whitelists the lcov directives covdrift ignores.
func knownLCOVPrefix(line string) bool {
	for _, p := range []string{"TN:", "VER:", "FN:", "FNDA:", "FNF:", "FNH:", "BRDA:", "BRF:", "BRH:", "LF:", "LH:", "#"} {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// truncate shortens s for error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
