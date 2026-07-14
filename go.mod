// covdrift — diffs two coverage reports and fails CI on per-file
// regressions, format-agnostic (lcov, cobertura, go coverprofile).
//
// version:    0.1.0
// author:     JaydenCJ
// license:    MIT
// repository: https://github.com/JaydenCJ/covdrift
// keywords:   coverage, ci, diff, lcov, cobertura, regression, quality-gate
//
// Zero runtime dependencies: standard library only.
module github.com/JaydenCJ/covdrift

go 1.22
