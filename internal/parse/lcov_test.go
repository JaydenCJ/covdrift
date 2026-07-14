// Tests for the lcov tracefile parser. The fixtures reproduce the exact
// output shapes of real emitters (geninfo, Istanbul/nyc, vitest --coverage)
// including the noise directives they interleave with line records.
package parse

import (
	"strings"
	"testing"
)

func TestLCOVBasicRecord(t *testing.T) {
	report, err := LCOV([]byte("TN:\nSF:src/app.js\nDA:1,5\nDA:2,0\nDA:3,1\nLF:3\nLH:2\nend_of_record\n"))
	if err != nil {
		t.Fatalf("LCOV() error: %v", err)
	}
	f, ok := report.Files["src/app.js"]
	if !ok {
		t.Fatalf("file src/app.js missing: %v", report.Paths())
	}
	if f.Total() != 3 || f.Covered() != 2 {
		t.Fatalf("got %d/%d, want covered 2 of 3", f.Covered(), f.Total())
	}
	if f.Lines[1] != 5 {
		t.Fatalf("line 1 hits = %d, want 5", f.Lines[1])
	}
}

func TestLCOVMultipleRecords(t *testing.T) {
	report, err := LCOV([]byte("SF:a.js\nDA:1,1\nend_of_record\nSF:b.js\nDA:1,0\nend_of_record\n"))
	if err != nil {
		t.Fatalf("LCOV() error: %v", err)
	}
	if len(report.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(report.Files))
	}
	if report.Files["b.js"].Covered() != 0 {
		t.Fatalf("b.js should be uncovered")
	}
}

func TestLCOVDuplicateSFRecordsMergeBySumming(t *testing.T) {
	// Two shards concatenated with `cat shard1.info shard2.info` — the
	// standard way parallel CI jobs combine lcov output. Counts must sum
	// and a line covered by either shard must count as covered.
	input := "SF:a.js\nDA:1,2\nDA:2,0\nend_of_record\nSF:a.js\nDA:1,3\nDA:2,1\nend_of_record\n"
	report, err := LCOV([]byte(input))
	if err != nil {
		t.Fatalf("LCOV() error: %v", err)
	}
	f := report.Files["a.js"]
	if f.Lines[1] != 5 || f.Lines[2] != 1 {
		t.Fatalf("merged counts = %d,%d, want 5,1", f.Lines[1], f.Lines[2])
	}
}

func TestLCOVChecksumFieldTolerated(t *testing.T) {
	// geninfo --checksum appends an MD5 to each DA line.
	report, err := LCOV([]byte("SF:a.c\nDA:10,4,uNhr1yjBWQ6HBBZCVELzMQ\nend_of_record\n"))
	if err != nil {
		t.Fatalf("LCOV() error: %v", err)
	}
	if report.Files["a.c"].Lines[10] != 4 {
		t.Fatalf("checksummed DA not parsed")
	}
}

func TestLCOVIgnoresFunctionAndBranchDirectives(t *testing.T) {
	input := strings.Join([]string{
		"TN:unit", "SF:a.c",
		"FN:3,helper", "FNDA:2,helper", "FNF:1", "FNH:1",
		"BRDA:5,0,0,1", "BRDA:5,0,1,-", "BRF:2", "BRH:1",
		"DA:3,2", "LF:1", "LH:1", "end_of_record", "",
	}, "\n")
	report, err := LCOV([]byte(input))
	if err != nil {
		t.Fatalf("LCOV() error: %v", err)
	}
	f := report.Files["a.c"]
	if f.Total() != 1 || f.Lines[3] != 2 {
		t.Fatalf("only DA lines should be instrumented, got %v", f.Lines)
	}
}

func TestLCOVWindowsLineEndings(t *testing.T) {
	report, err := LCOV([]byte("SF:a.js\r\nDA:1,1\r\nend_of_record\r\n"))
	if err != nil {
		t.Fatalf("CRLF input rejected: %v", err)
	}
	if report.Files["a.js"].Lines[1] != 1 {
		t.Fatalf("CRLF DA not parsed")
	}
}

func TestLCOVStructuralErrorsAreRejected(t *testing.T) {
	// A truncated or corrupt tracefile must fail loudly: silently
	// accepting it would understate the baseline and let a real
	// regression pass the gate.
	cases := map[string]string{
		"DA outside a record":   "DA:1,1\n",
		"missing end_of_record": "SF:a.js\nDA:1,1\n",
		"SF inside open record": "SF:a.js\nSF:b.js\nend_of_record\n",
		"stray end_of_record":   "end_of_record\n",
		"unknown directive":     "SF:a.js\nXX:nonsense\nend_of_record\n",
		"SF with empty path":    "SF:\nend_of_record\n",
	}
	for name, input := range cases {
		if _, err := LCOV([]byte(input)); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestLCOVMalformedDAIsAnError(t *testing.T) {
	for _, payload := range []string{"DA:x,1", "DA:1", "DA:1,-2", "DA:0,1", "DA:1,z"} {
		if _, err := LCOV([]byte("SF:a.js\n" + payload + "\nend_of_record\n")); err == nil {
			t.Fatalf("payload %q must be rejected", payload)
		}
	}
}

func TestLCOVEmptyInputYieldsEmptyReport(t *testing.T) {
	report, err := LCOV([]byte("\n\n"))
	if err != nil {
		t.Fatalf("blank input should parse: %v", err)
	}
	if len(report.Files) != 0 {
		t.Fatalf("expected empty report")
	}
}
