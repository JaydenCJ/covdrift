// Tests for path normalization and glob filtering — the machinery that
// makes an absolute lcov path from one CI machine line up with a relative
// cobertura path from another.
package pathnorm

import (
	"testing"

	"github.com/JaydenCJ/covdrift/internal/model"
)

func report(paths ...string) *model.Report {
	r := model.NewReport("lcov")
	for _, p := range paths {
		f := r.File(p)
		f.AddHits(1, 1)
	}
	return r
}

func TestNormalizeStripsPrefix(t *testing.T) {
	got := Normalize("/build/repo/src/a.go", []string{"/build/repo/"})
	if got != "src/a.go" {
		t.Fatalf("got %q, want src/a.go", got)
	}
	// Prefix without trailing slash also works; the slash is trimmed.
	got = Normalize("/build/repo/src/a.go", []string{"/build/repo"})
	if got != "src/a.go" {
		t.Fatalf("no-trailing-slash prefix: got %q", got)
	}
}

func TestNormalizeFirstMatchingPrefixWins(t *testing.T) {
	got := Normalize("/ci/work/src/a.go", []string{"/other/", "/ci/work/", "/ci/"})
	if got != "src/a.go" {
		t.Fatalf("got %q, want src/a.go (first match, not longest)", got)
	}
}

func TestNormalizeCanonicalizesWindowsPaths(t *testing.T) {
	got := Normalize(`src\pkg\a.go`, nil)
	if got != "src/pkg/a.go" {
		t.Fatalf("backslashes not converted: %q", got)
	}
	got = Normalize(`build\repo\src\a.go`, []string{`build\repo`})
	if got != "src/a.go" {
		t.Fatalf("backslash prefix not stripped: %q", got)
	}
	// A leading "./" (common in relative lcov output) is also dropped.
	if got := Normalize("./src/a.go", nil); got != "src/a.go" {
		t.Fatalf("leading ./ not dropped: %q", got)
	}
}

func TestApplyMergesPathsThatCollideAfterStripping(t *testing.T) {
	// Two shards instrumented the same file under different build roots;
	// after stripping both roots the counts must merge, not clobber.
	r := model.NewReport("lcov")
	r.File("/shard1/src/a.go").AddHits(1, 2)
	r.File("/shard2/src/a.go").AddHits(1, 3)
	out := Apply(r, Options{StripPrefixes: []string{"/shard1/", "/shard2/"}})
	if len(out.Files) != 1 {
		t.Fatalf("got %d files, want 1 merged: %v", len(out.Files), out.Paths())
	}
	if out.Files["src/a.go"].Lines[1] != 5 {
		t.Fatalf("merged hits = %d, want 5", out.Files["src/a.go"].Lines[1])
	}
}

func TestApplyIncludeKeepsOnlyMatches(t *testing.T) {
	out := Apply(report("src/a.go", "src/b.py", "docs/c.md"), Options{Include: []string{"src/**"}})
	if len(out.Files) != 2 {
		t.Fatalf("include kept %v", out.Paths())
	}
}

func TestApplyExcludeWinsOverInclude(t *testing.T) {
	out := Apply(report("src/a.go", "src/gen/b.go"), Options{
		Include: []string{"src/**"},
		Exclude: []string{"src/gen/**"},
	})
	if len(out.Files) != 1 || out.Files["src/a.go"] == nil {
		t.Fatalf("exclude did not win: %v", out.Paths())
	}
}

func TestGlobDoubleStarMatchesZeroOrMoreSegments(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/**", "src/a.go", true},
		{"src/**", "src/deep/nest/a.go", true},
		{"a/**/b.go", "a/b.go", true}, // ** may match zero segments
		{"a/**/b.go", "a/x/y/b.go", true},
		{"src/**", "other/a.go", false},
		{"**/vendor/**", "x/vendor/y/z.go", true},
	}
	for _, c := range cases {
		if got := Match(c.pattern, c.path); got != c.want {
			t.Fatalf("Match(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestGlobStarAndQuestionStayWithinSegment(t *testing.T) {
	if Match("src/*.go", "src/deep/a.go") {
		t.Fatalf("* must not cross a slash")
	}
	if !Match("src/*.go", "src/main.go") {
		t.Fatalf("* should match within segment")
	}
	if !Match("src/a?.go", "src/ab.go") || Match("src/a?.go", "src/abc.go") {
		t.Fatalf("? must match exactly one character")
	}
}

func TestGlobBareNameMatchesBasenameAnywhere(t *testing.T) {
	if !Match("*_test.go", "deep/pkg/foo_test.go") {
		t.Fatalf("bare pattern should match basename in any directory")
	}
	if Match("*_test.go", "deep/pkg/foo.go") {
		t.Fatalf("non-matching basename matched")
	}
}

func TestGlobBacktracking(t *testing.T) {
	// The classic star-backtracking trap: the first '*' must be able to
	// re-expand when the tail fails to match.
	if !Match("a*b*c", "aXbXbYc") {
		t.Fatalf("backtracking match failed")
	}
	if Match("a*b*c", "aXbXbY") {
		t.Fatalf("matched despite missing trailing c")
	}
}
