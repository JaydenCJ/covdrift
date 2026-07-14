// Tests for the Go coverprofile parser. Block shapes mirror what
// `go test -coverprofile` actually writes, including the overlapping
// blocks that make naive line projection double-count.
package parse

import (
	"testing"
)

func TestGoProfileSetMode(t *testing.T) {
	input := "mode: set\n" +
		"example.test/pkg/a.go:3.10,5.2 2 1\n" +
		"example.test/pkg/a.go:7.2,7.20 1 0\n"
	report, err := GoProfile([]byte(input))
	if err != nil {
		t.Fatalf("GoProfile() error: %v", err)
	}
	f := report.Files["example.test/pkg/a.go"]
	if f == nil {
		t.Fatalf("file missing: %v", report.Paths())
	}
	// Block 1 spans lines 3-5, block 2 line 7; line 6 is not instrumented.
	if f.Total() != 4 {
		t.Fatalf("instrumented lines = %d, want 4 (3,4,5,7)", f.Total())
	}
	if f.Covered() != 3 {
		t.Fatalf("covered = %d, want 3", f.Covered())
	}
	if _, instrumented := f.Lines[6]; instrumented {
		t.Fatalf("line 6 is between blocks and must not be instrumented")
	}
}

func TestGoProfileSetModeOverlapStaysBoolean(t *testing.T) {
	// The same line inside two covered basic blocks: set mode is a
	// boolean and must not report count 2.
	input := "mode: set\na.go:1.1,3.2 1 1\na.go:3.4,4.2 1 1\n"
	report, err := GoProfile([]byte(input))
	if err != nil {
		t.Fatalf("GoProfile() error: %v", err)
	}
	if got := report.Files["a.go"].Lines[3]; got != 1 {
		t.Fatalf("set-mode overlap line 3 = %d, want 1", got)
	}
}

func TestGoProfileCountAndAtomicModesSumOverlaps(t *testing.T) {
	for _, mode := range []string{"count", "atomic"} {
		input := "mode: " + mode + "\na.go:1.1,2.2 1 3\na.go:2.4,3.2 1 4\n"
		report, err := GoProfile([]byte(input))
		if err != nil {
			t.Fatalf("GoProfile(%s) error: %v", mode, err)
		}
		f := report.Files["a.go"]
		if f.Lines[1] != 3 || f.Lines[2] != 7 || f.Lines[3] != 4 {
			t.Fatalf("%s-mode sums wrong: %v", mode, f.Lines)
		}
	}
}

func TestGoProfileZeroCountBlockIsInstrumentedNotCovered(t *testing.T) {
	report, err := GoProfile([]byte("mode: count\na.go:10.1,12.2 2 0\n"))
	if err != nil {
		t.Fatalf("GoProfile() error: %v", err)
	}
	f := report.Files["a.go"]
	if f.Total() != 3 || f.Covered() != 0 {
		t.Fatalf("got %d/%d, want 0 of 3 covered", f.Covered(), f.Total())
	}
}

func TestGoProfilePathWithColons(t *testing.T) {
	// Module paths can contain colons (custom import path tricks, or a
	// Windows drive letter in older toolchains); the range starts after
	// the LAST colon.
	report, err := GoProfile([]byte("mode: set\nhost:8080/pkg/a.go:1.1,1.5 1 1\n"))
	if err != nil {
		t.Fatalf("GoProfile() error: %v", err)
	}
	if _, ok := report.Files["host:8080/pkg/a.go"]; !ok {
		t.Fatalf("path with colon mangled: %v", report.Paths())
	}
}

func TestGoProfileHeaderErrors(t *testing.T) {
	cases := map[string]string{
		"missing mode header": "a.go:1.1,1.5 1 1\n",
		"empty profile":       "",
		"unknown mode":        "mode: fancy\n",
	}
	for name, input := range cases {
		if _, err := GoProfile([]byte(input)); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
}

func TestGoProfileMalformedBlocksAreErrors(t *testing.T) {
	for _, block := range []string{
		"a.go:1.1,1.5 1",         // missing count
		"a.go:1.1 1 1",           // range missing end
		"a.go 1 1",               // no colon separator
		"a.go:1.1,1.5 x 1",       // bad statement count
		"a.go:1.1,1.5 1 -3",      // negative count
		"a.go:0.1,1.5 1 1",       // line 0
		"a.go:5.1,2.10 1 1",      // range ends before it starts
		"a.go:1.1,2000000.1 1 1", // absurd span
	} {
		if _, err := GoProfile([]byte("mode: set\n" + block + "\n")); err == nil {
			t.Fatalf("block %q must be rejected", block)
		}
	}
}

func TestGoProfileOnlyModeHeaderYieldsEmptyReport(t *testing.T) {
	report, err := GoProfile([]byte("mode: count\n"))
	if err != nil {
		t.Fatalf("header-only profile should parse: %v", err)
	}
	if len(report.Files) != 0 {
		t.Fatalf("expected empty report")
	}
}
