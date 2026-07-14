# Input formats and normalization

covdrift reads three coverage report formats and normalizes each into the
same internal shape: **per file, a map of instrumented line → hit count**.
Everything else — diffing, tolerances, lost-line ranges, rendering — works
on that shape only, which is why the two sides of a diff may come from
different ecosystems.

Format detection is content-based, never extension-based. Each file is
sniffed independently, so `covdrift diff base.info cur.xml` just works.
Force a format with `--input-format` when a file is ambiguous on purpose.

## lcov tracefiles (`lcov`)

Written by lcov/geninfo, and by virtually every JavaScript/TypeScript
runner via an "lcov" reporter (Istanbul/nyc, vitest, jest, c8).

| Directive | Meaning | covdrift behavior |
|---|---|---|
| `SF:<path>` | start of a file record | opens the file |
| `DA:<line>,<count>[,<checksum>]` | line execution count | recorded; checksum ignored |
| `end_of_record` | end of the record | closes the file |
| `TN:`, `VER:`, `FN*`, `BRDA/BRF/BRH`, `LF/LH`, `#…` | test names, functions, branches, summaries | validated position, otherwise ignored |

Duplicate `SF` records for the same path — the result of concatenating
shard outputs with `cat` — merge by summing per-line counts, matching
`lcov --add-tracefile`. A record without `end_of_record` (a truncated
upload) is an error: silently accepting it would understate the baseline
and let a real regression pass the gate. Unknown directives are treated
as corruption for the same reason.

## Cobertura XML (`cobertura`)

Written by coverage.py/pytest-cov, gcovr (`--cobertura`), Istanbul's
cobertura reporter, and most JaCoCo-to-cobertura converters.

covdrift reads `packages > package > classes > class` and each class's
**class-level** `<lines>` list (`number` and `hits` attributes). Lines
nested under `<methods>` duplicate the class-level list and are ignored,
so nothing is double-counted. Multiple `<class>` elements sharing one
`filename` (Java inner classes, Python classes in one module) merge by
summing per-line counts. `branch` / `condition-coverage` attributes are
ignored — covdrift 0.1.0 diffs line coverage only. The `<sources>` prefix
list is not applied automatically; use `--strip-prefix` to reconcile
paths explicitly and predictably.

## Go coverprofile (`goprofile`)

Written by `go test -coverprofile=cover.out`. After the `mode:` header,
each line is a basic block:

```
name.go:startLine.startCol,endLine.endCol numStatements count
```

Blocks are projected onto lines: every line the block spans becomes
instrumented. Overlapping blocks combine as booleans in `set` mode and as
summed counts in `count`/`atomic` mode. The path/range separator is the
*last* colon, so import paths containing colons survive.

## Path normalization

Applied identically to both sides, before diffing:

1. backslashes become forward slashes (Windows emitters),
2. a leading `./` is dropped,
3. the first matching `--strip-prefix` is removed,
4. `--include` / `--exclude` globs filter the remaining paths
   (`*`, `?`, `**`; a pattern without `/` matches base names).

Two input paths that normalize to the same output path merge by summing
per-line counts — exactly what you want when stripping shard-specific
build roots.

## Percentage semantics

A file's coverage is `covered lines / instrumented lines`. A file with
zero instrumented lines is 100% by definition (there is nothing left to
execute). Percentages are rounded to 4 decimal places before comparison,
so the same ratio computed through different divisions (2/3 vs 4/6) never
registers as drift; display rounds to 0.1pp.
