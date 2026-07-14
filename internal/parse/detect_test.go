// Tests for content-based format detection — the feature that lets the
// two diff sides come from different coverage ecosystems without flags.
package parse

import (
	"strings"
	"testing"
)

func detectOK(t *testing.T, data string) string {
	t.Helper()
	format, err := Detect([]byte(data))
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	return format
}

func TestDetectLCOVBySFLine(t *testing.T) {
	if got := detectOK(t, "SF:src/a.js\nDA:1,1\nend_of_record\n"); got != FormatLCOV {
		t.Fatalf("got %q, want lcov", got)
	}
}

func TestDetectLCOVWithLeadingTestName(t *testing.T) {
	if got := detectOK(t, "TN:unit\nSF:a.js\nend_of_record\n"); got != FormatLCOV {
		t.Fatalf("TN-first lcov not detected: %q", got)
	}
}

func TestDetectCoberturaWithAndWithoutXMLDeclaration(t *testing.T) {
	if got := detectOK(t, "<?xml version=\"1.0\"?>\n<coverage/>\n"); got != FormatCobertura {
		t.Fatalf("got %q, want cobertura", got)
	}
	if got := detectOK(t, "<coverage line-rate=\"1.0\"></coverage>"); got != FormatCobertura {
		t.Fatalf("bare root: got %q, want cobertura", got)
	}
}

func TestDetectGoProfileModeHeader(t *testing.T) {
	for _, mode := range []string{"set", "count", "atomic"} {
		if got := detectOK(t, "mode: "+mode+"\na.go:1.1,1.2 1 1\n"); got != FormatGoProfile {
			t.Fatalf("mode %s: got %q, want goprofile", mode, got)
		}
	}
}

func TestDetectToleratesBOMAndLeadingBlankLines(t *testing.T) {
	if got := detectOK(t, "\xEF\xBB\xBF\n\nmode: set\n"); got != FormatGoProfile {
		t.Fatalf("BOM+blank goprofile: got %q", got)
	}
	if got := detectOK(t, "\n  \n<?xml version=\"1.0\"?><coverage/>"); got != FormatCobertura {
		t.Fatalf("leading-whitespace XML: got %q", got)
	}
}

func TestDetectRejectsEmptyAndUnknownContent(t *testing.T) {
	if _, err := Detect([]byte("")); err == nil {
		t.Fatalf("empty file must not detect")
	}
	if _, err := Detect([]byte("hello world\nnot coverage\n")); err == nil {
		t.Fatalf("prose must not detect")
	}
	// "mode:" appearing later must not trigger goprofile: the header is
	// only valid as the first non-empty line.
	if _, err := Detect([]byte("some junk\nmode: set\n")); err == nil {
		t.Fatalf("late mode header must not detect")
	}
}

func TestDetectScansPastNoiseToFindSF(t *testing.T) {
	// Some emitters put several TN records before the first SF; make
	// sure a long-ish preamble still detects.
	data := strings.Repeat("TN:shard\n", 5) + "SF:a.js\nend_of_record\n"
	if got := detectOK(t, data); got != FormatLCOV {
		t.Fatalf("preamble lcov not detected: %q", got)
	}
}

func TestBytesHonorsExplicitFormatOverAuto(t *testing.T) {
	// Content is valid lcov, but the caller forces goprofile: parsing
	// must fail rather than silently fall back to detection.
	if _, err := Bytes([]byte("SF:a.js\nend_of_record\n"), FormatGoProfile, "x"); err == nil {
		t.Fatalf("explicit --input-format must win over content sniffing")
	}
	if _, err := Bytes([]byte("SF:a.js\nend_of_record\n"), "yaml", "x"); err == nil {
		t.Fatalf("unknown explicit format must be rejected")
	}
}
