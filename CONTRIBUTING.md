# Contributing to covdrift

Issues, discussions and pull requests are all welcome.

## Getting started

You need Go ≥1.22; nothing else — the project has zero runtime
dependencies and the tests never touch the network.

```bash
git clone https://github.com/JaydenCJ/covdrift && cd covdrift
go build ./...
go test ./...
bash scripts/smoke.sh
```

`scripts/smoke.sh` builds the binary, fabricates coverage reports in all
three supported formats in a temp dir, and asserts on real CLI output and
exit codes; it must finish by printing `SMOKE OK`.

## Before you open a pull request

1. `gofmt -l .` reports nothing (formatting is enforced).
2. `go vet ./...` passes with no findings.
3. `go test ./...` passes (90 deterministic tests, no network).
4. `bash scripts/smoke.sh` prints `SMOKE OK`.
5. Add tests for behavior changes; keep logic in pure, unit-testable
   modules (parsers, diff, and renderers never touch the filesystem —
   only the CLI layer reads files).

## Ground rules

- Keep dependencies at zero; adding one needs strong justification in
  the PR. covdrift's whole pitch is a single static binary.
- No network calls, ever, and no telemetry — covdrift reads two local
  files and writes to stdout.
- Determinism first: identical inputs must produce byte-identical
  reports, including all orderings, on every machine.
- New input formats get their own file in `internal/parse/`, a detection
  rule with tests, and a section in `docs/formats.md`. Fixtures must
  reproduce what a real emitter writes, not an idealized shape.
- Gate semantics are load-bearing: a change that can turn a passing diff
  into a failing one (or vice versa) needs a test for both directions.
- Code comments and doc comments are written in English.

## Reporting bugs

Include the output of `covdrift version`, the full command you ran, and —
for parse or classification bugs — the smallest slice of the two report
files that reproduces the problem (an `SF:`…`end_of_record` block, one
`<class>` element, or a couple of profile lines). That slice is exactly
what the parser sees.

## Security

Please do not open public issues for security problems; use GitHub's
private vulnerability reporting on this repository instead.
