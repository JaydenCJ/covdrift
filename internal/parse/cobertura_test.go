// Tests for the cobertura XML parser, using report shapes from
// coverage.py/pytest-cov (one class per module) and JaCoCo-style exports
// (several classes sharing one source file).
package parse

import (
	"strings"
	"testing"
)

const coberturaHeader = `<?xml version="1.0" ?>
<!DOCTYPE coverage SYSTEM "http://cobertura.sourceforge.net/xml/coverage-04.dtd">
`

func wrapCobertura(classes string) []byte {
	return []byte(coberturaHeader +
		`<coverage line-rate="0.5" branch-rate="0" version="7.4" timestamp="1700000000">` +
		`<sources><source>/build/src</source></sources>` +
		`<packages><package name="app">` + classes + `</package></packages></coverage>`)
}

func TestCoberturaBasicClass(t *testing.T) {
	report, err := Cobertura(wrapCobertura(
		`<classes><class name="app" filename="app/main.py" line-rate="0.667">
			<methods/>
			<lines><line number="1" hits="3"/><line number="2" hits="0"/><line number="4" hits="1"/></lines>
		</class></classes>`))
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	f := report.Files["app/main.py"]
	if f == nil || f.Total() != 3 || f.Covered() != 2 {
		t.Fatalf("app/main.py = %+v, want 2/3 covered", f)
	}
}

func TestCoberturaMultiplePackages(t *testing.T) {
	data := []byte(coberturaHeader + `<coverage><packages>
		<package name="a"><classes><class filename="a/x.py"><lines><line number="1" hits="1"/></lines></class></classes></package>
		<package name="b"><classes><class filename="b/y.py"><lines><line number="1" hits="0"/></lines></class></classes></package>
	</packages></coverage>`)
	report, err := Cobertura(data)
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	if len(report.Files) != 2 {
		t.Fatalf("got %d files, want 2", len(report.Files))
	}
}

func TestCoberturaMergesClassesSharingAFile(t *testing.T) {
	// JaCoCo-converted reports emit one <class> per Java class; inner
	// classes live in the same source file and must merge, not clobber.
	report, err := Cobertura(wrapCobertura(
		`<classes>
			<class name="Outer" filename="src/Outer.java"><lines><line number="3" hits="2"/><line number="9" hits="0"/></lines></class>
			<class name="Outer$Inner" filename="src/Outer.java"><lines><line number="9" hits="1"/><line number="12" hits="0"/></lines></class>
		</classes>`))
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	f := report.Files["src/Outer.java"]
	if f.Total() != 3 {
		t.Fatalf("merged file has %d lines, want 3", f.Total())
	}
	if f.Lines[9] != 1 {
		t.Fatalf("line 9 should be covered after merge, got %d", f.Lines[9])
	}
}

func TestCoberturaIgnoresMethodLevelLines(t *testing.T) {
	// Method lines duplicate the class-level <lines>; reading both would
	// double the hit counts.
	report, err := Cobertura(wrapCobertura(
		`<classes><class filename="a.py">
			<methods><method name="f"><lines><line number="1" hits="7"/></lines></method></methods>
			<lines><line number="1" hits="7"/></lines>
		</class></classes>`))
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	if got := report.Files["a.py"].Lines[1]; got != 7 {
		t.Fatalf("line 1 hits = %d, want 7 (method lines double-counted?)", got)
	}
}

func TestCoberturaBranchAttributesIgnored(t *testing.T) {
	report, err := Cobertura(wrapCobertura(
		`<classes><class filename="a.py"><lines>
			<line number="5" hits="2" branch="true" condition-coverage="50% (1/2)"/>
		</lines></class></classes>`))
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	if report.Files["a.py"].Lines[5] != 2 {
		t.Fatalf("branch attributes broke line parsing")
	}
}

func TestCoberturaEmptyPackagesYieldsEmptyReport(t *testing.T) {
	report, err := Cobertura([]byte(`<coverage><packages/></coverage>`))
	if err != nil {
		t.Fatalf("Cobertura() error: %v", err)
	}
	if len(report.Files) != 0 {
		t.Fatalf("expected empty report")
	}
}

func TestCoberturaRejectsCorruptInput(t *testing.T) {
	cases := map[string][]byte{
		"truncated XML":          []byte(`<coverage><packages>`),
		"class without filename": wrapCobertura(`<classes><class><lines><line number="1" hits="1"/></lines></class></classes>`),
		"line number zero":       wrapCobertura(`<classes><class filename="a.py"><lines><line number="0" hits="1"/></lines></class></classes>`),
		"negative hits":          wrapCobertura(`<classes><class filename="a.py"><lines><line number="1" hits="-1"/></lines></class></classes>`),
	}
	for name, data := range cases {
		if _, err := Cobertura(data); err == nil {
			t.Fatalf("%s must be rejected", name)
		}
	}
	// The wrong root element gets a targeted message naming what was
	// expected, since "you gave me a JaCoCo file" is a common mistake.
	_, err := Cobertura([]byte(`<report><packages/></report>`))
	if err == nil || !strings.Contains(err.Error(), "coverage") {
		t.Fatalf("wrong root should name the expected element, got %v", err)
	}
}
