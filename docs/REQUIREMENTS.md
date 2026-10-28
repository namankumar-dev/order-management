# Requirements

## Business problem

An online retailer needs a backend to accept customer orders, price them from a catalog service, track their lifecycle and let staff confirm or cancel them. The system must be safe to run in production on AWS.

## Actors

- CUSTOMER: registers, logs in, places orders for themselves, views and cancels their own orders.
- ADMIN: views all orders, places orders on behalf of customers, confirms and cancels any order.

## Functional requirements

| ID | Requirement |
|----|-------------|
| FR1 | A user can register with email and password and receives the CUSTOMER role. |
| FR2 | A user can log in and receive a short-lived JWT access token. |
| FR3 | An authenticated user can create an order for a product and quantity. The unit price and stock are read from the Product Service. |
| FR4 | An authenticated user can fetch one order. Customers only see their own orders. |
| FR5 | An authenticated user can list orders with pagination and an optional status filter. Customers only see their own orders. |
| FR6 | Order status changes follow CREATED to CONFIRMED or CANCELLED, and CONFIRMED to CANCELLED. Only ADMIN can confirm. CUSTOMER can cancel their own orders. |
| FR7 | The same order operations are available through GraphQL. |
| FR8 | An initial ADMIN account can be seeded from configuration. |

## Non-functional requirements

| ID | Requirement | How it is met |
|----|-------------|---------------|
| NFR1 | Security | JWT, bcrypt, role checks, parameterized SQL through GORM, strict input validation, unknown JSON fields rejected, body size limit, security headers, HTTPS enforcement in production, no secrets in source. |
| NFR2 | Reliability | Timeout, bounded retries with exponential backoff and jitter, circuit breaker, context cancellation, graceful shutdown. |
| NFR3 | Concurrency safety | Stateless handlers, pooled DB connections, row lock on status updates, tested with the race detector and 100 concurrent requests. |
| NFR4 | Observability | JSON logs with request id, health probes, Prometheus metrics, span logs across controller, service, repository and client. |
| NFR5 | Performance | Connection pooling, indexes on customer_id and status, one count query and one page query per list call. |
| NFR6 | Operability | Twelve factor configuration, Docker image running as non-root, CI gates, tag based release. |
| NFR7 | Maintainability | Layered packages with interfaces at repository and client boundaries so each layer is testable in isolation. |

## Package responsibilities

| Package | Responsibility |
|---------|----------------|
| config | Load and validate configuration from environment and AWS Secrets Manager. |
| model | Domain types, request models, status rules. |
| apperror | Typed errors with HTTP status and public message. |
| validation | Validator setup and translation of validation errors. |
| auth | JWT issue and verification. |
| database | PostgreSQL connection, pooling, optional auto migration. |
| repository | Data access behind interfaces. |
| client | Resilient HTTP client for the Product Service. |
| service | Business rules and authorization decisions. |
| controller | HTTP handlers. |
| graph | GraphQL schema and handler. |
| middleware | Request id, logging, metrics, recovery, security, auth, error handling. |
| router | Route and middleware wiring. |
| trace | Request id propagation and span logging. |
| metrics | Prometheus collectors. |
| product-service | Separate microservice that owns product price and stock. |
