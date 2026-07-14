package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/JaydenCJ/covdrift/internal/render"
)

// runShow implements `covdrift show <report>`: parse one file, print the
// normalized per-file numbers that diff would compare. It doubles as a
// format validator — exit 3 means covdrift cannot read the file.
func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var in inputFlags
	in.register(fs)
	format := fs.String("format", "text", "output format: text or json")
	rest, err := parseInterleaved(fs, args)
	if err != nil {
		return ExitUsage
	}
	if code := in.validate(stderr); code != ExitOK {
		return code
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "covdrift: unknown --format %q (want text or json)\n", *format)
		return ExitUsage
	}
	if len(rest) != 1 {
		fmt.Fprintf(stderr, "covdrift show: expected exactly one report path, got %d\n", len(rest))
		return ExitUsage
	}

	report, err := in.load(rest[0])
	if err != nil {
		fmt.Fprintf(stderr, "covdrift: %v\n", err)
		return ExitRuntime
	}
	if *format == "json" {
		if err := render.ShowJSON(stdout, report); err != nil {
			fmt.Fprintf(stderr, "covdrift: %v\n", err)
			return ExitRuntime
		}
		return ExitOK
	}
	render.ShowText(stdout, report)
	return ExitOK
}
