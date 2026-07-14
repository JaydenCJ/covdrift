// Package parse turns coverage report files — lcov tracefiles, cobertura
// XML, and Go coverprofiles — into the normalized model.Report. Format
// detection is content-based, never extension-based, because CI systems
// name these files inconsistently (`coverage.out`, `lcov.info`, `cov.xml`,
// or nothing recognizable at all).
package parse

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/JaydenCJ/covdrift/internal/model"
)

// Supported input format identifiers.
const (
	FormatLCOV      = "lcov"
	FormatCobertura = "cobertura"
	FormatGoProfile = "goprofile"
	FormatAuto      = "auto"
)

// File reads path and parses it as format ("auto" sniffs the content).
func File(path, format string) (*model.Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Bytes(data, format, path)
}

// Bytes parses raw report content. name is used only in error messages.
func Bytes(data []byte, format, name string) (*model.Report, error) {
	if format == FormatAuto || format == "" {
		detected, err := Detect(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		format = detected
	}
	var (
		report *model.Report
		err    error
	)
	switch format {
	case FormatLCOV:
		report, err = LCOV(data)
	case FormatCobertura:
		report, err = Cobertura(data)
	case FormatGoProfile:
		report, err = GoProfile(data)
	default:
		return nil, fmt.Errorf("unknown input format %q (want auto, lcov, cobertura, or goprofile)", format)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return report, nil
}

// Detect sniffs which coverage format data is in. It only looks at
// structural markers near the top of the file, so multi-megabyte reports
// are detected without a full scan.
func Detect(data []byte) (string, error) {
	// Tolerate a UTF-8 BOM and leading blank lines.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 {
		return "", fmt.Errorf("cannot detect coverage format: file is empty")
	}
	if trimmed[0] == '<' {
		// XML declaration, DOCTYPE, or a bare <coverage> root.
		return FormatCobertura, nil
	}
	// Inspect the first handful of non-empty lines for lcov / goprofile
	// markers. 64 lines is generous: both formats declare themselves on
	// line one in every real emitter we have seen.
	seen := 0
	for _, line := range strings.Split(string(head(trimmed, 64*1024)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if seen == 0 && isGoProfileMode(line) {
			return FormatGoProfile, nil
		}
		if strings.HasPrefix(line, "SF:") || strings.HasPrefix(line, "TN:") || line == "end_of_record" {
			return FormatLCOV, nil
		}
		seen++
		if seen >= 64 {
			break
		}
	}
	return "", fmt.Errorf("cannot detect coverage format: no lcov, cobertura, or Go coverprofile markers found")
}

// isGoProfileMode reports whether line is a Go coverprofile mode header.
func isGoProfileMode(line string) bool {
	mode, ok := strings.CutPrefix(line, "mode:")
	if !ok {
		return false
	}
	switch strings.TrimSpace(mode) {
	case "set", "count", "atomic":
		return true
	}
	return false
}

// head returns at most n leading bytes of data, cut at a line boundary so
// detection never inspects a half line.
func head(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	cut := bytes.LastIndexByte(data[:n], '\n')
	if cut < 0 {
		return data[:n]
	}
	return data[:cut]
}
