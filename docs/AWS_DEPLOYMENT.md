# AWS deployment

Target: ECS Fargate behind an Application Load Balancer, RDS PostgreSQL, Secrets Manager, CloudWatch. Region used in the examples: ap-south-1.

```
Internet -> ALB (443) -> ECS tasks (private subnets) -> RDS PostgreSQL (private subnets)
                              |-> Product Service (ECS, internal DNS)
                              |-> CloudWatch Logs and metrics
                              |-> Secrets Manager
```

## 1. Network

1. Create a VPC with two public subnets (ALB) and two private subnets (ECS, RDS) in different AZs, an internet gateway, and a NAT gateway for outbound access from private subnets.
2. Security groups:
   - `alb-sg`: inbound 80 and 443 from 0.0.0.0/0.
   - `api-sg`: inbound 8080 from `alb-sg` only.
   - `product-sg`: inbound 8081 from `api-sg` only.
   - `rds-sg`: inbound 5432 from `api-sg` only.

## 2. Database

1. Create an RDS PostgreSQL 16 instance in the private subnets with `rds-sg`, Multi-AZ for production, encryption on.
2. Apply the schema once from a bastion or a one-off ECS task: `psql "host=<endpoint> dbname=orderdb user=orderuser sslmode=require" -f db/migrations/001_init.sql`.

## 3. Secrets Manager

Create one secret named `order-management/prod`:

```bash
aws secretsmanager create-secret --name order-management/prod --secret-string '{
  "DB_HOST": "<rds-endpoint>",
  "DB_NAME": "orderdb",
  "DB_USER": "orderuser",
  "DB_PASSWORD": "<generated>",
  "JWT_SECRET": "<at least 32 random characters>",
  "PRODUCT_SERVICE_TOKEN": "<random>",
  "ADMIN_EMAIL": "admin@example.com",
  "ADMIN_PASSWORD": "<at least 12 characters>"
}'
```

The app reads the secret at startup when `SECRETS_NAME` is set. Keys of an RDS managed secret (`username`, `password`, `host`, `port`, `dbname`) are also understood.

## 4. IAM

Task execution role: `AmazonECSTaskExecutionRolePolicy` (pull from ECR, write logs).

Task role `order-api-task-role` with this inline policy:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "arn:aws:secretsmanager:ap-south-1:ACCOUNT_ID:secret:order-management/prod-*"
    }
  ]
}
```

No access keys are stored anywhere. The SDK uses the task role credentials.

## 5. Images

```bash
aws ecr create-repository --repository-name order-api --image-scanning-configuration scanOnPush=true
aws ecr create-repository --repository-name product-service --image-scanning-configuration scanOnPush=true
```

The release workflow builds and pushes `order-api`. Build `product-service` the same way and deploy it as a second ECS service registered in a Cloud Map namespace `order.internal` so the API can reach `http://product-service.order.internal:8081`.

## 6. Load balancer

1. Request an ACM certificate for your domain.
2. Create an internet facing ALB in the public subnets with `alb-sg`.
3. Listener 80 redirects to 443. Listener 443 forwards to a target group.
4. Target group: type `ip`, protocol HTTP, port 8080, health check path `/health/ready`, healthy threshold 2, interval 15 seconds.

## 7. ECS

1. Create cluster `order-cluster`, log group `/ecs/order-api`.
2. Register `deploy/ecs-task-definition.json` after replacing `ACCOUNT_ID`.
3. Create service `order-api` with desired count 2, Fargate, private subnets, `api-sg`, attached to the target group. Set `TRUSTED_PROXIES` to the VPC CIDR so client IPs and `X-Forwarded-Proto` come from the ALB.
4. Add service auto scaling on CPU at 60 percent.

## 8. CloudWatch

- Logs: the `awslogs` driver ships the JSON logs. Query example in Logs Insights:

```
fields @timestamp, request_id, method, path, status, duration
| filter status >= 500
| sort @timestamp desc
```

- Metrics: run the CloudWatch agent or the AWS Distro for OpenTelemetry collector as a sidecar scraping `localhost:9090/metrics`, or scrape with Amazon Managed Prometheus. The metrics port is not exposed through the ALB.
- Alarms: ALB `HTTPCode_Target_5XX_Count`, target group `UnHealthyHostCount`, RDS `CPUUtilization` and `DatabaseConnections`, and a metric filter on log lines containing `circuit breaker state change`.

## 9. CI/CD role

Create a GitHub OIDC identity provider and an IAM role trusted for `repo:<owner>/<repo>:ref:refs/tags/v*`, with permissions to push to ECR, register task definitions, update the ECS service and `iam:PassRole` for the two task roles. Store its ARN as the repository secret `AWS_DEPLOY_ROLE_ARN`.

## 10. Release

```bash
git tag v1.0.0
git push origin v1.0.0
```

The `release` workflow runs tests with the race detector, builds the image tagged `v1.0.0`, pushes to ECR, deploys to ECS, waits for stability and creates the GitHub release.

## 11. Verify

```bash
curl https://api.example.com/health/ready
API=https://api.example.com ./scripts/demo.sh
```
