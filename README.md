# Order Management System

Cloud-native order management backend in Go: Gin REST API, GraphQL, PostgreSQL with GORM, JWT authentication, role based authorization, a separate Product microservice, resilience patterns, observability, CI/CD and AWS deployment.

## Architecture

```
Client -> ALB -> Go API -> Controller -> Service -> Repository -> PostgreSQL (RDS)
                   |            |           |
                   |            |           +-> Product Service (timeout, retry, backoff, circuit breaker)
                   |            +-> Middleware: request id, logging, metrics, recovery, security headers, auth, errors
                   +-> /graphql (same service layer)

Logs -> CloudWatch    Metrics (:9090) -> CloudWatch / Prometheus    Secrets -> Secrets Manager
```

## Quick start

Requirements: Go 1.22+, Docker.

```bash
cp .env.example .env
```

Edit `.env` and set `DB_PASSWORD`, `JWT_SECRET` (32+ characters), `PRODUCT_SERVICE_TOKEN` and `ADMIN_PASSWORD` (12+ characters). Then:

```bash
go mod tidy
(cd product-service && go mod tidy)
docker compose up --build
```

In another terminal:

```bash
./scripts/demo.sh
```

Run locally without Docker for the API: start PostgreSQL, apply `db/migrations/001_init.sql` (or set `AUTO_MIGRATE=true`), run the product service with `INTERNAL_TOKEN=<token> go run ./product-service`, then `go run .`.

## Endpoints

| Method | Path | Access |
|--------|------|--------|
| POST | /api/v1/auth/register | public |
| POST | /api/v1/auth/login | public |
| POST | /api/v1/orders | CUSTOMER (own), ADMIN |
| GET | /api/v1/orders | CUSTOMER (own), ADMIN (all) |
| GET | /api/v1/orders/{id} | owner, ADMIN |
| PUT | /api/v1/orders/{id}/status | ADMIN, owner may cancel |
| POST | /graphql | authenticated |
| GET | /health/live, /health/ready | public |
| GET | :9090/metrics | internal port |

Full contract: [docs/API.md](docs/API.md). Requirements: [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md). AWS: [docs/AWS_DEPLOYMENT.md](docs/AWS_DEPLOYMENT.md).

## Configuration

All settings come from environment variables or from the AWS Secrets Manager secret named in `SECRETS_NAME`. Secret values take precedence. See `.env.example` for every variable. The service refuses to start if required values are missing, the JWT secret is shorter than 32 characters, or production uses `DB_SSLMODE=disable`.

## Testing

```bash
make race
make cover
make loadtest
```

Covered: business rules, API behavior and authorization, validation errors, GraphQL, configuration, 100 concurrent order requests, concurrent status updates, retry, backoff bounds, circuit breaker, per attempt timeout and context cancellation. Repositories use fakes in tests, so the suite needs no database.

## Observability

- Logs: JSON with `request_id`, `method`, `path`, `status`, `duration`, `error`. Set `LOG_FORMAT=text` for key=value output and `LOG_LEVEL=debug` to see span lines that show the path Controller > Service > Repository(postgres) and the Product Service call.
- Health: `/health/live`, `/health/ready` (checks PostgreSQL).
- Metrics: request count, latency histogram, 5xx count, database errors, product service call outcomes.
- Tracing: the request id is propagated through the context and to the Product Service in `X-Request-ID`. Spans are logged. For full distributed tracing swap `trace.Start` for OpenTelemetry with the AWS X-Ray exporter.

## Resilience demo

```bash
export PRODUCT_SERVICE_TOKEN=<token>
./scripts/demo.sh
```

The script switches the Product Service into outage mode, shows order creation returning 503 after bounded retries, then 503 immediately once the circuit opens, and restores the service. Retries are limited by `PRODUCT_MAX_RETRIES` (default 2, maximum 5).

## Security summary

bcrypt password hashing, signed JWT with expiry and algorithm pinning, role checks in the service layer, parameterized queries through GORM, row locking for status updates, unknown JSON fields rejected, 1 MB body limit, security headers, HSTS and HTTPS enforcement in production, generic error messages with details only in logs, non-root distroless image, no secrets in the repository, `govulncheck`, `gosec` and Trivy in CI.

## Git workflow

- `main`: always releasable, protected, requires a pull request and passing CI.
- `develop`: integration branch.
- `feature/<issue>-<name>`, `fix/<issue>-<name>`: short lived branches from `develop`.
- `hotfix/<name>`: from `main`, merged to `main` and `develop`.
- Commits follow Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`).
- Releases are semantic version tags `vMAJOR.MINOR.PATCH` on `main`. Pushing a tag triggers the release workflow.

Create the initial issues with `./scripts/create_issues.sh` (needs the GitHub CLI).

## Known limitations

- Stock is checked but not reserved. A production system would reserve stock through the Product Service or events.
- The Product Service keeps its catalog in memory for the demo.
- Login has no rate limiting. Put AWS WAF in front of the ALB or add a limiter.
- GraphQL has a query size limit but no depth limit.
