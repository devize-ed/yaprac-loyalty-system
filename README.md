## Yaprac Loyalty System (Gophermart)

## Prerequisites

### System Requirements
- **Go**: Version 1.23.0 or higher (toolchain 1.24.6 recommended)
- **Docker & Docker Compose**: For running PostgreSQL database
- **Make**: For running build and test commands

### Devtools
- **Mockery**: For generating mocks (installed automatically via `make mock-gen`)
- **golangci-lint**: For code linting

## Installation & Setup

**Generate mocks**:
```bash
make mock-gen
```

## Running the Service

### 1. Start Database
Start PostgreSQL using Docker Compose:
```bash
make up
```

### 2. Run the Server

**Run in foreground**:
```bash
make run
```

**Run in background**:
```bash
make run-bg
```

The background mode will capture logs in `.tmp/server.log`

### 4. Stop the Service
Stop background server:
```bash
make stop
```

Stop and clean up containers:
```bash
make down
```

## Configuration

The service can be configured using environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `RUN_ADDRESS` | `localhost:8080` | Server address and port |
| `DATABASE_URI` | `postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable` | PostgreSQL connection string |
| `ACCRUAL_SYSTEM_ADDRESS` | `http://127.0.0.1:65535` | Accrual system address |
| `AUTH_SECRET` | `` | JWT secret key |

Custom configuration:
```bash
RUN_ADDRESS=localhost:9090 DATABASE_URI="postgres://user:pass@localhost:5432/mydb" make run
```

## Testing

**Run all tests**:
```bash
make tests
```

**Run unit tests**:
```bash
go test ./... -v
```

**Run integration tests**:
```bash
go test -tags=integration_tests ./... -v
```

**Run mock tests**:
```bash
go test -tags=mock_tests ./... -v
```

**Run end-to-end tests**:
```bash
make e2e
```

### Manual API Testing

```bash
# Register a user
make t.register

# Login and save auth token
make t.login

# Upload an order
make t.order

# Upload invalid order
make t.order-invalid

# List orders
make t.orders

# Check balance
make t.balance

# Make withdrawal
make t.withdraw

# List withdrawals
make t.withdrawals

# Show current auth token
make t.auth

# Clear auth token
make t.logout
```

## Building

Build the server binary:
```bash
make build
```