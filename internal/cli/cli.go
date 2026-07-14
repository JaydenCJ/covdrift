// Package cli implements the covdrift command-line interface. Run takes
// argv and two writers and returns an exit code, so the whole surface is
// testable in-process without building a binary.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/JaydenCJ/covdrift/internal/model"
	"github.com/JaydenCJ/covdrift/internal/parse"
	"github.com/JaydenCJ/covdrift/internal/pathnorm"
	"github.com/JaydenCJ/covdrift/internal/version"
)

// Exit codes. Documented in the README; `diff` uses ExitGate as its
// machine-readable verdict for CI.
const (
	ExitOK      = 0
	ExitGate    = 1
	ExitUsage   = 2
	ExitRuntime = 3
)

// Run dispatches argv and returns the process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitUsage
	}
	switch args[0] {
	case "diff":
		return runDiff(args[1:], stdout, stderr)
	case "show":
		return runShow(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "covdrift %s\n", version.Version)
		return ExitOK
	case "help", "--help", "-h":
		usage(stdout)
		return ExitOK
	default:
		if strings.HasPrefix(args[0], "-") {
			fmt.Fprintf(stderr, "covdrift: unknown flag %q before a subcommand\n\n", args[0])
			usage(stderr)
			return ExitUsage
		}
		// Two bare paths: treat as `diff <baseline> <current>`.
		if len(args) == 2 && !strings.HasPrefix(args[1], "-") {
			return runDiff(args, stdout, stderr)
		}
		fmt.Fprintf(stderr, "covdrift: unknown subcommand %q\n\n", args[0])
		usage(stderr)
		return ExitUsage
	}
}

// multiFlag is a repeatable string flag.
type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

// inputFlags are shared by diff and show: how to read and normalize the
// report files before any comparison happens.
type inputFlags struct {
	inputFormat string
	stripPrefix multiFlag
	include     multiFlag
	exclude     multiFlag
}

func (f *inputFlags) register(fs *flag.FlagSet) {
	fs.StringVar(&f.inputFormat, "input-format", parse.FormatAuto, "input format: auto, lcov, cobertura, or goprofile")
	fs.Var(&f.stripPrefix, "strip-prefix", "path prefix removed from every file (repeatable)")
	fs.Var(&f.include, "include", "only compare files matching this glob (repeatable)")
	fs.Var(&f.exclude, "exclude", "skip files matching this glob, e.g. 'vendor/**' (repeatable)")
}

func (f *inputFlags) validate(stderr io.Writer) int {
	switch f.inputFormat {
	case parse.FormatAuto, parse.FormatLCOV, parse.FormatCobertura, parse.FormatGoProfile:
		return ExitOK
	}
	fmt.Fprintf(stderr, "covdrift: unknown --input-format %q (want auto, lcov, cobertura, or goprofile)\n", f.inputFormat)
	return ExitUsage
}

// load reads, parses, and normalizes one report file.
func (f *inputFlags) load(path string) (*model.Report, error) {
	r, err := parse.File(path, f.inputFormat)
	if err != nil {
		return nil, err
	}
	opts := pathnorm.Options{StripPrefixes: f.stripPrefix, Include: f.include, Exclude: f.exclude}
	return pathnorm.Apply(r, opts), nil
}

// parseInterleaved parses fs against args, allowing flags to appear after
// positional arguments (e.g. `diff base.info cur.xml --format json`). The
// standard library stops at the first non-flag argument; users coming from
// almost any other CLI expect trailing flags to work, so we resume parsing
// after each positional and return the collected positionals.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var positionals []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, args[0])
		args = args[1:]
	}
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `covdrift %s — diff two coverage reports, fail CI on per-file regressions

Usage:
  covdrift diff [flags] <baseline> <current>   compare two runs (exit 1 on regression)
  covdrift show [flags] <report>               print one report, normalized
  covdrift version                             print the version

Inputs are lcov tracefiles, cobertura XML, or Go coverprofiles; the format
is auto-detected per file, so the two sides may use different formats.

Diff flags:
  --format FORMAT        text (default), json, or markdown
  --tolerance PP         allowed per-file drop in percentage points (default 0)
  --total-tolerance PP   also gate the overall delta (default: off)
  --min-new PCT          require new files to be covered at least PCT (default: off)
  --min-lines N          files smaller than N lines never gate (default 0)
  --no-gate              report only; exit 0 even on regressions
  --all                  list unchanged files too

Input flags (diff and show):
  --input-format FORMAT  auto (default), lcov, cobertura, or goprofile
  --strip-prefix PREFIX  remove a path prefix before matching (repeatable)
  --include GLOB         only compare matching files (repeatable)
  --exclude GLOB         skip matching files, e.g. 'vendor/**' (repeatable)

Exit codes: 0 ok · 1 gate breach · 2 usage error · 3 runtime error
`, version.Version)
}
