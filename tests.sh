#!/usr/bin/env bash
if set -o pipefail 2>/dev/null; then
  set -euo pipefail
else
  set -eu
fi

# Setup variables
ADDR="${ADDR:-http://localhost:8080}"
USER1_LOGIN="${USER1_LOGIN:-user1}"
USER1_PASS="${USER1_PASS:-pass123}"
USER2_LOGIN="${USER2_LOGIN:-user2}"
USER2_PASS="${USER2_PASS:-pass321}"

# Setup orders
ORDER_VALID="${ORDER_VALID:-9278923470}"
ORDER_INVALID="${ORDER_INVALID:-12345}"
WITHDRAW_ORDER="${WITHDRAW_ORDER:-2377225624}"

# 
need() { command -v "$1" >/dev/null 2>&1 || { echo "ERROR: '$1' is required"; exit 1; }; }
say() { printf "\n\033[1;34m==> %s\033[0m\n" "$*"; }
expect_code() {
  local got="$1" want="$2" msg="${3:-}"
  if [[ "$got" != "$want" ]]; then
    echo "ERROR: expected HTTP $want, got $got ${msg:+($msg)}"
    exit 1
  fi
}

# Check tools
need curl
need awk
need tr

say "Server base: $ADDR"

# Register user1
say "Register user1 ($USER1_LOGIN)"
code=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$ADDR/api/user/register" \
  -H "Content-Type: application/json" \
  -d "{\"login\":\"$USER1_LOGIN\",\"password\":\"$USER1_PASS\"}")
if [[ "$code" == "409" ]]; then
  echo "Login already exists — ok for re-run"
else
  expect_code "$code" "200" "register user1"
fi

# Login user1 & capture token from Authorization header
say "Login user1 and capture token"
HDR1=$(mktemp)
curl -s -D "$HDR1" -o /dev/null \
  -X POST "$ADDR/api/user/login" \
  -H "Content-Type: application/json" \
  -d "{\"login\":\"$USER1_LOGIN\",\"password\":\"$USER1_PASS\"}"
TOKEN=$(awk '/^Authorization:/ {print $3}' "$HDR1" | tr -d '\r')
rm -f "$HDR1"
if [[ -z "${TOKEN:-}" ]]; then
  echo "ERROR: token not found in response headers"
  exit 1
fi
AUTH="Authorization: Bearer $TOKEN"

# Upload new order (202), then re-upload (200)
say "Upload order (first time -> 202)"
code=$(printf "%s" "$ORDER_VALID" | \
curl -s -o /dev/null -w "%{http_code}" -X POST "$ADDR/api/user/orders" \
  -H "$AUTH" -H "Content-Type: text/plain" \
  --data-binary @-)
[[ "$code" == "202" || "$code" == "200" ]] || expect_code "$code" "202 or 200" "upload order first"

say "Re-upload same order (-> 200)"
code=$(printf "%s" "$ORDER_VALID" | \
curl -s -o /dev/null -w "%{http_code}" -X POST "$ADDR/api/user/orders" \
  -H "$AUTH" -H "Content-Type: text/plain" \
  --data-binary @-)
expect_code "$code" "200" "re-upload order"

# Invalid order (422)
say "Upload invalid order (-> 422)"
code=$(printf "%s" "$ORDER_INVALID" | \
curl -s -o /dev/null -w "%{http_code}" -X POST "$ADDR/api/user/orders" \
  -H "$AUTH" -H "Content-Type: text/plain" \
  --data-binary @-)
expect_code "$code" "422" "invalid order should be 422"

# List orders (200 or 204)
say "List orders"
curl -i -s -X GET "$ADDR/api/user/orders" -H "$AUTH" | sed -n '1,15p'

# Balance (200)
say "Get balance"
curl -i -s -X GET "$ADDR/api/user/balance" -H "$AUTH" | sed -n '1,15p'

# Withdraw (200 or 402/422 depending on balance)
say "Withdraw attempt"
code=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST "$ADDR/api/user/balance/withdraw" \
  -H "$AUTH" -H "Content-Type: application/json" \
  -d "{\"order\":\"$WITHDRAW_ORDER\",\"sum\": 10.00}")
case "$code" in
  200) echo "OK: withdrawal accepted";;
  402) echo "OK: insufficient funds (expected until accrual is processed)";;
  422) echo "OK: invalid order format — check validation rules";;
  *)   echo "Unexpected code: $code"; exit 1;;
esac

# Withdrawals list (200 or 204)
say "Withdrawals list"
curl -i -s -X GET "$ADDR/api/user/withdrawals" -H "$AUTH" | sed -n '1,15p'

say "E2E script finished successfully"
