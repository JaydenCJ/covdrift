package parse

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// maxBlockLines rejects a single profile block claiming to span an absurd
// number of lines — a corrupt block would otherwise allocate unbounded
// memory while "parsing" garbage.
const maxBlockLines = 1 << 20

// GoProfile parses a Go coverprofile as written by `go test
// -coverprofile=…`. The format is a `mode:` header followed by one block
// per basic block:
//
//	name.go:startLine.startCol,endLine.endCol numStatements count
//
// Blocks are projected onto lines: every line a block spans is
// instrumented. Overlapping blocks (the same line inside several basic
// blocks) combine as max-covered in `set` mode and as summed counts in
// `count`/`atomic` mode, matching how `go tool cover -func` treats them.
func GoProfile(data []byte) (*model.Report, error) {
	report := model.NewReport(FormatGoProfile)
	mode := ""

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if mode == "" {
			m, ok := strings.CutPrefix(line, "mode:")
			if !ok {
				return nil, fmt.Errorf("goprofile line %d: expected \"mode: set|count|atomic\" header, got %q", lineNo, truncate(line, 40))
			}
			mode = strings.TrimSpace(m)
			if mode != "set" && mode != "count" && mode != "atomic" {
				return nil, fmt.Errorf("goprofile line %d: unknown mode %q", lineNo, mode)
			}
			continue
		}
		if err := parseProfileBlock(report, line, mode); err != nil {
			return nil, fmt.Errorf("goprofile line %d: %v", lineNo, err)
		}
	}
	if mode == "" {
		return nil, fmt.Errorf("goprofile: missing \"mode:\" header")
	}
	return report, nil
}

// parseProfileBlock parses one block line and applies it to the report.
func parseProfileBlock(report *model.Report, line, mode string) error {
	// Rightmost two space-separated fields are numStatements and count;
	// everything before them is "<path>:<range>". Splitting from the
	// right keeps paths containing spaces intact.
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return fmt.Errorf("malformed block %q", truncate(line, 60))
	}
	countStr := fields[len(fields)-1]
	stmtStr := fields[len(fields)-2]
	loc := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(line, countStr), " "))
	loc = strings.TrimSpace(strings.TrimSuffix(loc, stmtStr))

	count, err := strconv.ParseInt(countStr, 10, 64)
	if err != nil || count < 0 {
		return fmt.Errorf("invalid execution count %q", countStr)
	}
	if _, err := strconv.Atoi(stmtStr); err != nil {
		return fmt.Errorf("invalid statement count %q", stmtStr)
	}

	// The path/range separator is the LAST colon: file names may contain
	// colons, but the range never contains one.
	colon := strings.LastIndexByte(loc, ':')
	if colon <= 0 || colon == len(loc)-1 {
		return fmt.Errorf("malformed block location %q", truncate(loc, 60))
	}
	path, rng := loc[:colon], loc[colon+1:]

	startLine, endLine, err := parseBlockRange(rng)
	if err != nil {
		return err
	}
	if endLine-startLine >= maxBlockLines {
		return fmt.Errorf("block spans %d lines, refusing (corrupt profile?)", endLine-startLine+1)
	}

	file := report.File(path)
	for ln := startLine; ln <= endLine; ln++ {
		switch {
		case count == 0:
			file.MarkSeen(ln)
		case mode == "set":
			// set mode is boolean; overlapping covered blocks must not
			// inflate the count past 1.
			if file.Lines[ln] < 1 {
				file.Lines[ln] = 1
			}
		default:
			file.AddHits(ln, count)
		}
	}
	return nil
}

// parseBlockRange parses "startLine.startCol,endLine.endCol".
func parseBlockRange(rng string) (startLine, endLine int, err error) {
	start, end, ok := strings.Cut(rng, ",")
	if !ok {
		return 0, 0, fmt.Errorf("malformed block range %q", rng)
	}
	if startLine, err = linePart(start); err != nil {
		return 0, 0, fmt.Errorf("malformed block range %q: %v", rng, err)
	}
	if endLine, err = linePart(end); err != nil {
		return 0, 0, fmt.Errorf("malformed block range %q: %v", rng, err)
	}
	if endLine < startLine {
		return 0, 0, fmt.Errorf("block range %q ends before it starts", rng)
	}
	return startLine, endLine, nil
}

// linePart extracts the line number from a "line.col" position.
func linePart(pos string) (int, error) {
	lineStr, _, ok := strings.Cut(pos, ".")
	if !ok {
		return 0, fmt.Errorf("position %q missing column", pos)
	}
	n, err := strconv.Atoi(lineStr)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid line number %q", lineStr)
	}
	return n, nil
}
