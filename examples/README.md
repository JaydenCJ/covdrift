# covdrift examples

Two coverage fixtures and one runnable gate script, all offline and
self-contained.

## baseline.info + current.xml

A deliberately mixed-format pair: the baseline is an lcov tracefile, the
current run is cobertura XML — the situation you hit when the main branch
pipeline and a migrated PR pipeline use different reporters. The pair
contains one regression (with lost lines), one improvement, one new file,
and one unchanged file.

```bash
covdrift diff examples/baseline.info examples/current.xml
covdrift diff --format markdown examples/baseline.info examples/current.xml
covdrift show examples/current.xml
```

The first command exits 1: `src/parser.js` dropped from 91.7% to 58.3%
and lines 5-8 are newly uncovered.

## pr-gate.sh

Shows the shape of a real CI gate: per-file tolerance of 0.5pp, small
files exempted with `--min-lines`, vendored code excluded, and a Markdown
summary written on failure for the PR conversation.

```bash
bash examples/pr-gate.sh; echo "exit: $?"
```

Both fixtures are static files, so every command above produces identical
output on every machine.
