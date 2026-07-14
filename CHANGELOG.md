# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-13

### Added

- Format-agnostic report parsing with content-based detection: lcov
  tracefiles (duplicate-shard merging, checksum tolerance, strict
  truncation errors), cobertura XML (class merging, method-line
  deduplication), and Go coverprofiles (`set`/`count`/`atomic` block
  projection with correct overlap semantics).
- `diff` subcommand comparing every file against its own baseline:
  regressed / improved / unchanged / added / removed classification,
  per-file `--tolerance` in percentage points, optional
  `--total-tolerance` budget, `--min-new` floor for new files, and
  `--min-lines` exemption for tiny files.
- Lost-line analysis for every regressed file: the exact lines that were
  covered in the baseline and are uncovered now, grouped into ranges.
- Path normalization so paths from different machines and formats line
  up: slash canonicalization, repeatable `--strip-prefix`, and
  `--include`/`--exclude` globs (`*`, `?`, `**`, bare-name patterns).
- Three output formats: aligned terminal text, stable JSON
  (`schema_version: 1`), and PR-comment-ready Markdown; CI-friendly exit
  codes (0 ok, 1 gate breach, 2 usage, 3 runtime) plus `--no-gate` for
  report-only runs.
- `show` subcommand printing one report normalized — the exact numbers
  `diff` compares — in text or JSON, doubling as a format validator.
- Runnable examples (`examples/pr-gate.sh`, a cross-format lcov/cobertura
  fixture pair) and a format reference (`docs/formats.md`).
- 90 deterministic offline tests (unit + in-process CLI integration) and
  `scripts/smoke.sh`.

[0.1.0]: https://github.com/JaydenCJ/covdrift/releases/tag/v0.1.0
