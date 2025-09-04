## Yaprac Loyalty System (Gophermart)

### Mock generate
```bash
make mock-gen
```

### Run service
1) Start Postgres:
```bash
make up
```

2) Run the server:
```bash
make run
```

Run in background and capture logs in `.tmp/server.log`:
```bash
make run-bg
```

3) Stop the background server and clean containers:
```bash
make stop
make down
```

### Testing
- Unit tests:
```bash
make test
```

- End‑to‑end tests:
```bash
make test e2e
```

### Build
```bash
make build
```
