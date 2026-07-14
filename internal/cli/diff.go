package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/JaydenCJ/covdrift/internal/diff"
	"github.com/JaydenCJ/covdrift/internal/render"
)

// runDiff implements `covdrift diff <baseline> <current>`.
func runDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var in inputFlags
	in.register(fs)
	format := fs.String("format", "text", "output format: text, json, or markdown")
	tolerance := fs.Float64("tolerance", 0, "allowed per-file drop in percentage points")
	totalTolerance := fs.Float64("total-tolerance", -1, "also gate the overall delta (percentage points)")
	minNew := fs.Float64("min-new", -1, "require new files to be covered at least this percent")
	minLines := fs.Int("min-lines", 0, "files with fewer instrumented lines never gate")
	noGate := fs.Bool("no-gate", false, "report only; exit 0 even on regressions")
	all := fs.Bool("all", false, "list unchanged files too")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return ExitUsage
	}
	if code := in.validate(stderr); code != ExitOK {
		return code
	}
	if *format != "text" && *format != "json" && *format != "markdown" {
		fmt.Fprintf(stderr, "covdrift: unknown --format %q (want text, json, or markdown)\n", *format)
		return ExitUsage
	}
	if *tolerance < 0 {
		fmt.Fprintf(stderr, "covdrift: --tolerance must be >= 0\n")
		return ExitUsage
	}
	if *minLines < 0 {
		fmt.Fprintf(stderr, "covdrift: --min-lines must be >= 0\n")
		return ExitUsage
	}
	if len(rest) != 2 {
		fmt.Fprintf(stderr, "covdrift diff: expected exactly two report paths (baseline, current), got %d\n", len(rest))
		return ExitUsage
	}

	base, err := in.load(rest[0])
	if err != nil {
		fmt.Fprintf(stderr, "covdrift: baseline: %v\n", err)
		return ExitRuntime
	}
	current, err := in.load(rest[1])
	if err != nil {
		fmt.Fprintf(stderr, "covdrift: current: %v\n", err)
		return ExitRuntime
	}

	opts := diff.Options{
		TolerancePP:      *tolerance,
		TotalTolerancePP: *totalTolerance,
		MinNewPct:        *minNew,
		MinLines:         *minLines,
	}
	res := diff.Compare(base, current, opts)

	switch *format {
	case "json":
		if err := render.JSON(stdout, res); err != nil {
			fmt.Fprintf(stderr, "covdrift: %v\n", err)
			return ExitRuntime
		}
	case "markdown":
		render.Markdown(stdout, res)
	default:
		render.Text(stdout, res, render.TextOptions{All: *all})
	}

	if !res.GateOK() && !*noGate {
		return ExitGate
	}
	return ExitOK
}
