SHELL := /bin/bash
PROJECT_NAME := yaprac-loyalty-system

RUN_ADDRESS ?= localhost:8080
BASE        ?= http://$(RUN_ADDRESS)

DB_DSN_HOST ?= localhost
DB_DSN      ?= postgres://postgres:postgres@$(DB_DSN_HOST):5432/postgres?sslmode=disable
ACCRUAL     ?= http://localhost:8081
AUTH_SECRET ?= secret

GO        ?= go
PKG       ?= ./...
GOFLAGS   ?=
TEST_FLAGS?= -count=1 -v

USER1_LOGIN    ?= user1
USER1_PASS     ?= password123
ORDER_VALID    ?= 9278923470
WITHDRAW_ORDER ?= 2377225624

DC := docker compose -p $(PROJECT_NAME)

PIDFILE := .tmp/server.pid

.PHONY: up migrate down ps logs build run run-bg stop test e2e e2e-keep clean tests ci tests-local status kill-port \
        t.register t.login t.order t.order-invalid t.orders t.balance t.withdraw t.withdrawals t.auth t.logout

## up: start Postgres and run migrations
up:
	@echo "==> Starting Postgres..."
	$(DC) up -d db
	@echo "==> Running migrations..."
	$(DC) up migrate

## migrate: run migrations manually
migrate:
	$(DC) run --rm migrate

## down: stop everything and remove volumes
down:
	$(DC) down -v

## ps: show compose status
ps:
	$(DC) ps

## logs: tail docker compose logs
logs:
	$(DC) logs -f --tail=200

## build: build server binary
build:
	$(GO) build -o ./cmd/gophermart ./cmd/gophermart

## run: run server in foreground
run:
	RUN_ADDRESS=$(RUN_ADDRESS) DATABASE_URI="$(DB_DSN)" ACCRUAL_SYSTEM_ADDRESS="$(ACCRUAL)" AUTH_SECRET="$(AUTH_SECRET)" \
	$(GO) run ./cmd/gophermart

run-bg:
	mkdir -p .tmp
	@echo "==> Starting server on $(RUN_ADDRESS) with DB=$(DB_DSN)"
	RUN_ADDRESS=$(RUN_ADDRESS) DATABASE_URI="$(DB_DSN)" ACCRUAL_SYSTEM_ADDRESS="$(ACCRUAL)" AUTH_SECRET="$(AUTH_SECRET)" \
	$(GO) run ./cmd/gophermart > .tmp/server.log 2>&1 & echo $$! > .tmp/server.pid
	@sleep 1; echo "PID=$$(cat .tmp/server.pid)"

## stop: stop background server
stop:
	@PORT=$$(echo $(RUN_ADDRESS) | awk -F: '{print $$NF}'); \
	if [ -f $(PIDFILE) ]; then \
		echo "==> Stopping server PID $$(cat $(PIDFILE))"; \
		kill -TERM $$(cat $(PIDFILE)) || true; \
		for i in 1 2 3 4 5; do \
			kill -0 $$(cat $(PIDFILE)) 2>/dev/null || break; \
			sleep 1; \
		done; \
		kill -0 $$(cat $(PIDFILE)) 2>/dev/null && kill -KILL $$(cat $(PIDFILE)) || true; \
		rm -f $(PIDFILE); \
	fi; \
	if lsof -tiTCP:$$PORT -sTCP:LISTEN >/dev/null; then \
		PIDS=$$(lsof -tiTCP:$$PORT -sTCP:LISTEN); \
		echo "==> Forcing stop on port $$PORT (PIDs: $$PIDS)"; \
		kill -TERM $$PIDS || true; sleep 1; \
		lsof -tiTCP:$$PORT -sTCP:LISTEN >/dev/null && kill -KILL $$PIDS || true; \
	fi

status:
	@PORT=$$(echo $(RUN_ADDRESS) | awk -F: '{print $$NF}'); \
	echo "RUN_ADDRESS=$(RUN_ADDRESS)  PORT=$$PORT"; \
	if [ -f $(PIDFILE) ]; then echo "PIDFILE=$(PIDFILE): $$(cat $(PIDFILE))"; else echo "PIDFILE: (none)"; fi; \
	lsof -iTCP:$$PORT -sTCP:LISTEN -nP || echo "Port $$PORT is free"


# Tests
## test: run Go tests
test:
	DATABASE_URI="$(DB_DSN)" $(GO) test $(PKG) $(TEST_FLAGS) $(GOFLAGS)

## e2e: start infra, run server in bg, run e2e script, ALWAYS cleanup containers + volumes
e2e:
	@set -euo pipefail; \
	trap '$(MAKE) stop; $(DC) down -v' EXIT; \
	$(MAKE) stop; \
	$(MAKE) up; \
	$(MAKE) run-bg; \
	if [ -x ./scripts/e2e.sh ]; then bash ./scripts/e2e.sh; else bash ./tests.sh; fi

## e2e-keep: e2e but keep volumes
e2e-keep:
	@set -euo pipefail; \
	trap '$(MAKE) stop; $(DC) down' EXIT; \
	$(MAKE) stop; \
	$(MAKE) up; \
	$(MAKE) run-bg; \
	if [ -x ./scripts/e2e.sh ]; then bash ./scripts/e2e.sh; else bash ./tests.sh; fi

## clean: remove build artifacts
clean:
	rm -rf .tmp .bin

# Aggregators
tests: test e2e
	@echo "✅ All tests passed"

ci:
	@set -euo pipefail; \
	trap '$(MAKE) stop; $(MAKE) down' EXIT; \
	$(MAKE) up; \
	$(MAKE) run-bg; \
	$(MAKE) test; \
	if [ -x ./scripts/e2e.sh ]; then bash ./scripts/e2e.sh; else bash ./tests.sh; fi

tests-local:
	@$(MAKE) test
	@BASE="$(BASE)" bash ./scripts/e2e.sh


# Manual tests

t.register:
	curl -i -X POST "$(BASE)/api/user/register" -H "Content-Type: application/json" \
	 -d "{\"login\":\"$(USER1_LOGIN)\",\"password\":\"$(USER1_PASS)\"}"

t.login:
	@mkdir -p .tmp
	@set -e; HDR=$$(mktemp); \
	curl -s -D $$HDR -o /dev/null \
	  -X POST "$(BASE)/api/user/login" \
	  -H "Content-Type: application/json" \
	  -d "{\"login\":\"$(USER1_LOGIN)\",\"password\":\"$(USER1_PASS)\"}"; \
	TOKEN=$$(awk '/^Authorization:/ {print $$3}' $$HDR | tr -d '\r'); \
	rm -f $$HDR; \
	test -n "$$TOKEN" || { echo "ERROR: token not found"; exit 1; }; \
	echo "Authorization: Bearer $$TOKEN" > .tmp/auth.h; \
	echo "Saved .tmp/auth.h"

t.auth:
	@cat .tmp/auth.h 2>/dev/null || echo "No .tmp/auth.h — run 'make t.login' first"

t.logout:
	@rm -f .tmp/auth.h && echo "Removed .tmp/auth.h" || true

t.order:
	@AUTH=$$(cat .tmp/auth.h); \
	printf "%s" "$(ORDER_VALID)" | curl -i -X POST "$(BASE)/api/user/orders" \
	 -H "$$AUTH" -H "Content-Type: text/plain" --data-binary @-

t.order-invalid:
	@AUTH=$$(cat .tmp/auth.h); \
	printf "%s" "12345" | curl -i -X POST "$(BASE)/api/user/orders" \
	 -H "$$AUTH" -H "Content-Type: text/plain" --data-binary @-

t.orders:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(BASE)/api/user/orders" -H "$$AUTH"

t.balance:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(BASE)/api/user/balance" -H "$$AUTH"

t.withdraw:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X POST "$(BASE)/api/user/balance/withdraw" \
	 -H "$$AUTH" -H "Content-Type: application/json" \
	 -d "{\"order\":\"$(WITHDRAW_ORDER)\",\"sum\": 10.00}"

t.withdrawals:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(BASE)/api/user/withdrawals" -H "$$AUTH"
