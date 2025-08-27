SHELL := /bin/bash
PROJECT_NAME := yaprac-loyalty-system

RUN_ADDRESS ?= localhost:8080
DB_DSN_HOST ?= localhost
DB_DSN      ?= postgres://postgres:postgres@$(DB_DSN_HOST):5432/postgres?sslmode=disable
ACCRUAL     ?= http://localhost:8081
AUTH_SECRET ?= secret

GO ?= go

.PHONY: up migrate down ps logs build run run-bg stop test e2e clean

## up: start Postgres and run migrations (uses docker compose services `db` and `migrate`)
up:
	@echo "==> Starting Postgres..."
	docker compose up -d db
	@echo "==> Running migrations..."
	docker compose up migrate

## migrate: run migrations manually
migrate:
	docker compose run --rm migrate

## down: stop everything and remove volumes
down:
	docker compose down -v

## ps: show compose status
ps:
	docker compose ps

## logs: tail docker compose logs
logs:
	docker compose logs -f --tail=200

## build: build server binary
build:
	mkdir -p .bin
	$(GO) build -o ./.bin/gophermart ./cmd/gophermart

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
	@if [ -f .tmp/server.pid ]; then \
		echo "==> Stopping server PID $$(cat .tmp/server.pid)"; \
		kill -TERM $$(cat .tmp/server.pid) || true; \
		rm -f .tmp/server.pid; \
	else \
		echo "No PID file at .tmp/server.pid"; \
	fi

## test: run Go tests
test:
	DATABASE_URI="$(DB_DSN)" $(GO) test ./... -count=1 -v

## e2e: start e2e tests, run server in bg, execute curl, then stop server
e2e: up run-bg
	@bash ./tests.sh || { RC=$$?; $(MAKE) stop; exit $$RC; }
	@$(MAKE) stop

## clean: remove build artifacts
clean:
	rm -rf .tmp .bin	


######## MANUAL TESTS ########
.PHONY: t.register t.login t.order t.orders t.balance t.withdraw t.withdrawals

## t.register: register a new user
t.register:
	curl -i -X POST "$(RUN_ADDRESS)/api/user/register" -H "Content-Type: application/json" \
	 -d "{\"login\":\"$(USER1_LOGIN)\",\"password\":\"$(USER1_PASS)\"}"

## t.login: login a user and save the token
t.login:
	@mkdir -p .tmp
	@set -e; HDR=$$(mktemp); \
	curl -s -D $$HDR -o /dev/null \
	  -X POST "$(RUN_ADDRESS)/api/user/login" \
	  -H "Content-Type: application/json" \
	  -d "{\"login\":\"$(USER1_LOGIN)\",\"password\":\"$(USER1_PASS)\"}"; \
	TOKEN=$$(awk '/^Authorization:/ {print $$3}' $$HDR | tr -d '\r'); \
	rm -f $$HDR; \
	test -n "$$TOKEN" || { echo "ERROR: token not found"; exit 1; }; \
	echo "Authorization: Bearer $$TOKEN" > .tmp/auth.h; \
	echo "Saved .tmp/auth.h"

## t.order: upload a new order
t.order:
	@AUTH=$$(cat .tmp/auth.h); \
	printf "%s" "$(ORDER_VALID)" | curl -i -X POST "$(RUN_ADDRESS)/api/user/orders" \
	 -H "$$AUTH" -H "Content-Type: text/plain" --data-binary @-

## t.orders: get all orders
t.orders:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(RUN_ADDRESS)/api/user/orders" -H "$$AUTH"

## t.balance: get the balance
t.balance:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(RUN_ADDRESS)/api/user/balance" -H "$$AUTH"

## t.withdraw: withdraw money from the balance
t.withdraw:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X POST "$(RUN_ADDRESS)/api/user/balance/withdraw" \
	 -H "$$AUTH" -H "Content-Type: application/json" \
	 -d "{\"order\":\"$(WITHDRAW_ORDER)\",\"sum\": 10.00}"

## t.withdrawals: get all withdrawals
t.withdrawals:
	@AUTH=$$(cat .tmp/auth.h); curl -i -X GET "$(RUN_ADDRESS)/api/user/balance/withdrawals" -H "$$AUTH"