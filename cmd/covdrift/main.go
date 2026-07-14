// Command covdrift diffs two coverage reports — lcov, cobertura, or Go
// coverprofile — and fails CI on per-file regressions instead of absolute
// thresholds.
package main

import (
	"os"

	"github.com/JaydenCJ/covdrift/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
