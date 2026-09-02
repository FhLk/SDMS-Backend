# Testing

## Unit tests

The default test command runs the deterministic unit-test suite and does not
require PostgreSQL or Docker:

```bash
go test ./...
```

Run the race detector and write a coverage profile with:

```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

The suite covers configuration, domain validation, topic and user use cases,
HTTP handlers and error mapping, GORM model mapping, repository behavior,
health checks, and application route wiring. Repository unit tests use a
deterministic `database/sql` stub, so they do not connect to an external
database.

## Integration and end-to-end tests

Tests that start PostgreSQL through Testcontainers use the `integration` build
tag. Docker must be running before executing them:

```bash
go test -tags=integration ./internal/modules/topic/repository/postgres ./tests/e2e
```

To run every unit, integration, and end-to-end package together:

```bash
go test -tags=integration ./...
```
