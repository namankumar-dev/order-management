#!/usr/bin/env bash
set -euo pipefail

create() {
  gh issue create --title "$1" --body "$2" --label "$3"
}

gh label create feature --color 0E8A16 --force
gh label create security --color B60205 --force
gh label create infra --color 1D76DB --force
gh label create quality --color 5319E7 --force
gh label create docs --color 0075CA --force

create "Document requirements and API contract" "Functional and non-functional requirements, REST endpoints, request and response models." docs
create "Project structure and package responsibilities" "Clean layout: config, model, controller, service, repository, middleware, database." feature
create "Gin REST API for orders" "Create, get, list and update order status endpoints." feature
create "JWT authentication" "Register and login with bcrypt hashing and signed access tokens." security
create "Role based authorization" "ADMIN and CUSTOMER roles with ownership checks." security
create "Externalized configuration and AWS Secrets Manager" "Environment variables, validation, secrets loaded at startup." infra
create "Request validation and centralized errors" "Validator rules, uniform error envelope, request id in errors." quality
create "Unit and API tests" "Service, API, concurrency, resilience and config tests with race detector." quality
create "GraphQL endpoint" "Queries and mutations for orders sharing the REST service layer." feature
create "Product microservice and resilient client" "Separate product service, timeout, retry with backoff, circuit breaker, context cancellation." feature
create "Observability" "Structured logs, health endpoints, Prometheus metrics, span tracing." infra
create "CI pipeline with lint and security scanning" "Build, vet, race tests, golangci-lint, govulncheck, gosec, trivy." infra
create "Versioned release and AWS deployment" "Tag driven release to ECR and ECS behind an ALB." infra
create "Final demo preparation" "Rehearse the demo script and capture screenshots." docs
