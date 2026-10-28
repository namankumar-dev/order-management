# Final demo checklist

1. Show the repository: README, issues, branch protection, a merged pull request with green CI.
2. Show the architecture and package layout.
3. `docker compose up --build`, then `./scripts/demo.sh` and walk through each step: register, login, create, get, list, GraphQL, validation error, authorization failure, admin confirmation.
4. Show logs with `request_id`, and `LOG_LEVEL=debug` spans.
5. Show `/health/ready` and `/metrics` output.
6. Resilience: run the outage section of the demo and point at the 503 responses and the circuit breaker log lines.
7. Concurrency: `make loadtest` and `make race`.
8. CI: show the pipeline stages, a failing gate (for example a deliberately failing test), and the release workflow.
9. AWS: ALB, ECS service with two tasks, RDS, Secrets Manager secret, CloudWatch log group and alarms.
