#!/usr/bin/env bash
# End-to-end smoke test against a live Fizzy instance.
# Requires FIZZY_BASE_URL, FIZZY_TOKEN, FIZZY_ACCOUNT in the environment.
# Uses ./fizzy-cli if present, otherwise `go run ./cmd/fizzy-cli`.
set -euo pipefail

if [ -x "./fizzy-cli" ]; then
  CLI="./fizzy-cli"
else
  CLI="go run ./cmd/fizzy-cli"
fi

: "${FIZZY_BASE_URL:?FIZZY_BASE_URL is required}"
: "${FIZZY_TOKEN:?FIZZY_TOKEN is required}"
: "${FIZZY_ACCOUNT:?FIZZY_ACCOUNT is required}"

pass() { echo "  ok: $*"; }
fail() { echo "  FAIL: $*" >&2; exit 1; }

echo "== auth =="
$CLI auth status >/dev/null || fail "auth status"
pass "auth status"

echo "== boards =="
$CLI board list >/dev/null || fail "board list"
pass "board list"

# Create a throwaway board and exercise the write path, then clean up.
board_loc=$($CLI board create --name "smoke-$$" | awk '{print $NF}')
board_id=$(basename "$board_loc" .json)
[ -n "$board_id" ] || fail "board create returned no id"
pass "board create ($board_id)"

$CLI --json board get "$board_id" >/dev/null || fail "board get --json"
pass "board get --json"

card_loc=$($CLI card create --board-id "$board_id" --title "smoke card" | awk '{print $NF}')
card_num=$(basename "$card_loc" .json)
[ -n "$card_num" ] || fail "card create returned no number"
pass "card create (#$card_num)"

$CLI card get "$card_num" >/dev/null || fail "card get"
$CLI card close "$card_num" >/dev/null || fail "card close"
$CLI card reopen "$card_num" >/dev/null || fail "card reopen"
pass "card close/reopen"

$CLI card delete "$card_num" >/dev/null || fail "card delete"
$CLI board delete "$board_id" >/dev/null || fail "board delete"
pass "cleanup"

echo "All smoke tests passed."
