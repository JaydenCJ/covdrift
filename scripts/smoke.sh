#!/usr/bin/env bash
# End-to-end smoke test for covdrift: builds the binary, fabricates
# coverage reports in all three supported formats, and asserts on the real
# CLI output and exit codes. No network, idempotent, finishes in seconds.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

fail() {
  echo "SMOKE FAIL: $*" >&2
  exit 1
}

BIN="$WORKDIR/covdrift"

echo "1. build"
(cd "$ROOT" && go build -o "$BIN" ./cmd/covdrift) || fail "go build failed"

echo "2. version matches manifest"
"$BIN" --version | grep -qx "covdrift 0.1.0" || fail "--version mismatch"

echo "3. fabricate baseline (lcov) and current (cobertura) reports"
cat > "$WORKDIR/base.info" <<'EOF'
SF:src/a.js
DA:1,2
DA:2,2
DA:3,1
DA:4,0
end_of_record
SF:src/b.js
DA:1,1
DA:2,1
end_of_record
EOF
cat > "$WORKDIR/cur.xml" <<'EOF'
<?xml version="1.0" ?>
<coverage><packages><package name="src"><classes>
<class filename="src/a.js"><lines>
<line number="1" hits="2"/><line number="2" hits="0"/>
<line number="3" hits="0"/><line number="4" hits="0"/>
</lines></class>
<class filename="src/b.js"><lines>
<line number="1" hits="1"/><line number="2" hits="1"/>
</lines></class>
</classes></package></packages></coverage>
EOF

echo "4. cross-format diff detects the regression and exits 1"
set +e
OUT="$("$BIN" diff "$WORKDIR/base.info" "$WORKDIR/cur.xml")"
CODE=$?
set -e
[ "$CODE" -eq 1 ] || fail "regression should exit 1, got $CODE"
echo "$OUT" | grep -q "REGRESS" || fail "missing REGRESS row"
echo "$OUT" | grep -q "src/a.js" || fail "regressed file not named"
echo "$OUT" | grep -q "lost: 2-3" || fail "lost-line ranges missing"
echo "$OUT" | grep -q "gate: FAIL" || fail "gate verdict missing"

echo "5. identical runs pass with exit 0"
"$BIN" diff "$WORKDIR/base.info" "$WORKDIR/base.info" | grep -q "gate: OK" \
  || fail "identical runs should pass"

echo "6. tolerance turns the failure into a pass"
"$BIN" diff --tolerance 60 "$WORKDIR/base.info" "$WORKDIR/cur.xml" >/dev/null \
  || fail "diff should pass at 60pp tolerance"

echo "7. JSON output is machine-readable and carries the verdict"
JSON="$("$BIN" diff --format json "$WORKDIR/base.info" "$WORKDIR/cur.xml" || true)"
echo "$JSON" | grep -q '"tool": "covdrift"' || fail "json envelope missing"
echo "$JSON" | grep -q '"ok": false' || fail "json gate verdict wrong"
echo "$JSON" | grep -q '"regressed": 1' || fail "json regression count wrong"

echo "8. go coverprofile parses and shows normalized numbers"
printf 'mode: set\npkg/a.go:1.1,3.2 2 1\npkg/a.go:5.1,5.9 1 0\n' > "$WORKDIR/go.out"
"$BIN" show "$WORKDIR/go.out" | grep -q "goprofile report, 1 file" \
  || fail "goprofile not detected by show"
"$BIN" show "$WORKDIR/go.out" | grep -q "75.0%" || fail "goprofile percent wrong"

echo "9. strip-prefix reconciles differing build roots"
sed 's|SF:src/|SF:/ci/build/src/|' "$WORKDIR/base.info" > "$WORKDIR/base-abs.info"
"$BIN" diff --strip-prefix /ci/build/ "$WORKDIR/base-abs.info" "$WORKDIR/base.info" \
  | grep -q "0 added" || fail "--strip-prefix did not reconcile paths"

echo "10. usage errors exit 2, unreadable input exits 3"
set +e
"$BIN" diff --format yaml "$WORKDIR/base.info" "$WORKDIR/cur.xml" >/dev/null 2>&1
[ $? -eq 2 ] || fail "bad --format should exit 2"
echo "not coverage" > "$WORKDIR/junk.txt"
"$BIN" diff "$WORKDIR/junk.txt" "$WORKDIR/base.info" >/dev/null 2>&1
[ $? -eq 3 ] || fail "unparseable input should exit 3"
set -e

echo "SMOKE OK"
