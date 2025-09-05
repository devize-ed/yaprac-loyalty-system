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
ACCRUAL_PIDFILE := .tmp/accrual.pid

ACCRUAL_BIN  ?= ./cmd/accrual/accrual_linux_amd64
ACCRUAL_HOST ?= localhost
ACCRUAL_PORT ?= $(shell python3 - <<'PY' 2>/dev/null || echo 8081\nimport socket;s=socket.socket();s.bind(('',0));print(s.getsockname()[1]);s.close()\nPY)
ACCRUAL_DB   ?= postgresql://postgres:postgres@$(DB_DSN_HOST):5432/praktikum?sslmode=disable

.PHONY: up down logs build run run-bg wait-db wait-ready stop status clean \
        test tests e2e e2e-keep \
        t.register t.login t.order t.order-invalid t.orders t.balance t.withdraw t.withdrawals t.auth t.logout \
        mock-gen \
        accrual-db accrual-run wait-accrual stop-accrual

## up: start Postgres
up:
	@echo "==> Starting Postgres..."
	$(DC) up -d db

## down: stop everything and remove volumes
down:
	$(DC) down -v

## logs: tail docker compose logs
logs:
	$(DC) logs -f --tail=200

## build: build server binary
build:
	$(GO) build -o ./cmd/gophermart ./cmd/gophermart

## wait-db: wait until Postgres is ready
wait-db:
	@echo "==> Waiting for Postgres (docker health or pg_isready) ..."
	@for i in $$(seq 1 60); do \
		$(DC) exec -T db bash -lc 'pg_isready -U postgres -d postgres -h localhost -p 5432' >/dev/null 2>&1 && { echo "Postgres is ready"; exit 0; }; \
		sleep 1; \
	done; \
	echo "ERROR: Postgres is not ready"; exit 1

## run: run server in foreground
run:
	RUN_ADDRESS=$(RUN_ADDRESS) DATABASE_URI="$(DB_DSN)" ACCRUAL_SYSTEM_ADDRESS="$(ACCRUAL)" AUTH_SECRET="$(AUTH_SECRET)" \
	$(GO) run ./cmd/gophermart

## run-bg: run server in background
run-bg:
	mkdir -p .tmp
	@echo "==> Starting server on $(RUN_ADDRESS) with DB=$(DB_DSN)"
	@set -e; \
	RUN_ADDRESS=$(RUN_ADDRESS) DATABASE_URI="$(DB_DSN)" ACCRUAL_SYSTEM_ADDRESS="$(ACCRUAL)" AUTH_SECRET="$(AUTH_SECRET)" \
	$(GO) run ./cmd/gophermart > .tmp/server.log 2>&1 & echo $$! > $(PIDFILE); \
	sleep 0.5; \
	if ! kill -0 $$(cat $(PIDFILE)) 2>/dev/null; then \
		echo "ERROR: server exited immediately"; \
		tail -n 200 .tmp/server.log || true; \
		exit 1; \
	fi; \
	echo "PID=$$(cat $(PIDFILE))"

## wait-ready: wait until server is listening on port
wait-ready:
	@PORT=$$(echo $(RUN_ADDRESS) | awk -F: '{print $$NF}'); \
	echo "==> Waiting for server on port $$PORT ..."; \
	for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do \
		lsof -tiTCP:$$PORT -sTCP:LISTEN >/dev/null && { echo "Server is up"; exit 0; }; \
		if [ -f $(PIDFILE) ] && ! kill -0 $$(cat $(PIDFILE)) 2>/div/null; then \
			echo "ERROR: server process died early"; \
			tail -n 200 .tmp/server.log || true; \
			exit 1; \
		fi; \
		sleep 0.5; \
	done; \
	echo "ERROR: server did not start in time"; \
	tail -n 200 .tmp/server.log || true; \
	exit 1

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

# accrual

accrual-db:
	@echo "==> Ensuring DB 'praktikum' exists ..."
	@$(DC) exec -T db bash -lc 'psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = '\''praktikum'\''" | grep -q 1 || createdb -U postgres praktikum'
	@echo "DB 'praktikum' is ready"

accrual-run:
	@mkdir -p .tmp
	@test -x $(ACCRUAL_BIN) || { echo "ERROR: $(ACCRUAL_BIN) is not executable. Try: chmod +x $(ACCRUAL_BIN)"; exit 1; }
	@echo "==> Starting accrual on $(ACCRUAL_HOST):$(ACCRUAL_PORT)"
	@$(ACCRUAL_BIN) \
	  -accrual-host=$(ACCRUAL_HOST) \
	  -accrual-port=$(ACCRUAL_PORT) \
	  -accrual-database-uri="$(ACCRUAL_DB)" \
	  > .tmp/accrual.log 2>&1 & echo $$! > $(ACCRUAL_PIDFILE)
	@sleep 0.3; echo "ACCRUAL_PID=$$(cat $(ACCRUAL_PIDFILE))"

wait-accrual:
	@URL="http://$(ACCRUAL_HOST):$(ACCRUAL_PORT)/api/orders/self-check-$$RANDOM"; \
	echo "==> Waiting for accrual on $$URL ..."; \
	for i in $$(seq 1 60); do \
		code=$$(curl -s -o /dev/null -w "%{http_code}" "$$URL"); \
		if echo "$$code" | grep -qE "200|204"; then echo "Accrual is up"; exit 0; fi; \
		sleep 0.5; \
	done; \
	echo "ERROR: accrual did not start in time"; \
	echo "---- .tmp/accrual.log (last 200 lines) ----"; tail -n 200 .tmp/accrual.log || true; \
	exit 1

stop-accrual:
	@if [ -f $(ACCRUAL_PIDFILE) ]; then \
		echo "==> Stopping accrual PID $$(cat $(ACCRUAL_PIDFILE))"; \
		kill -TERM $$(cat $(ACCRUAL_PIDFILE)) 2>/dev/null || true; \
		for i in 1 2 3 4 5; do \
			kill -0 $$(cat $(ACCRUAL_PIDFILE)) 2>/dev/null || break; \
			sleep 1; \
		done; \
		kill -0 $$(cat $(ACCRUAL_PIDFILE)) 2>/dev/null && kill -KILL $$(cat $(ACCRUAL_PIDFILE)) || true; \
		rm -f $(ACCRUAL_PIDFILE); \
	fi

# Tests

## test: run unit + integration + mock tests
test:
	DATABASE_URI="$(DB_DSN)" $(GO) test $(PKG) $(TEST_FLAGS) $(GOFLAGS); \
	DATABASE_URI="$(DB_DSN)" $(GO) test -tags=integration_tests $(PKG) $(TEST_FLAGS) $(GOFLAGS); \
	DATABASE_URI="$(DB_DSN)" $(GO) test -tags=mock_tests $(PKG) $(TEST_FLAGS) $(GOFLAGS)

e2e:
	@set -euo pipefail; \
	trap '$(MAKE) stop; $(MAKE) stop-accrual; $(DC) down -v' EXIT; \
	$(MAKE) stop; \
	$(MAKE) up; \
	$(MAKE) wait-db; \
	$(MAKE) accrual-db; \
	$(MAKE) accrual-run; \
	$(MAKE) wait-accrual; \
	ACCRUAL="http://$(ACCRUAL_HOST):$(ACCRUAL_PORT)" $(MAKE) run-bg; \
	$(MAKE) wait-ready; \
	if [ -x ./scripts/e2e.sh ]; then BASE="$(BASE)" ACCRUAL="http://$(ACCRUAL_HOST):$(ACCRUAL_PORT)" bash ./scripts/e2e.sh; else BASE="$(BASE)" bash ./tests.sh; fi

e2e-keep:
	@set -euo pipefail; \
	trap '$(MAKE) stop; $(MAKE) stop-accrual; $(DC) down' EXIT; \
	$(MAKE) stop; \
	$(MAKE) up; \
	$(MAKE) wait-db; \
	$(MAKE) accrual-db; \
	$(MAKE) accrual-run; \
	$(MAKE) wait-accrual; \
	ACCRUAL="http://$(ACCRUAL_HOST):$(ACCRUAL_PORT)" $(MAKE) run-bg; \
	$(MAKE) wait-ready; \
	if [ -x ./scripts/e2e.sh ]; then BASE="$(BASE)" ACCRUAL="http://$(ACCRUAL_HOST):$(ACCRUAL_PORT)" bash ./scripts/e2e.sh; else BASE="$(BASE)" bash ./tests.sh; fi

clean:
	rm -rf .tmp .bin

tests: test e2e
	@echo "✅ All tests passed"


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

mock-gen:
	mockery --name=Storage --with-expecter --dir=internal/handlers --output=internal/handlers/mocks --outpkg=mocks && \
	mockery --name=Storage --with-expecter --dir=internal/service/accrual --output=internal/service/accrual/mocks --outpkg=mocks