# API contract

Base URL: `/api/v1`. All bodies are JSON. Protected endpoints need `Authorization: Bearer <token>`.

## Error format

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "request validation failed",
    "details": { "quantity": "must be at least 1" },
    "requestId": "9d6f6a5e-3c1e-4d5c-8a6b-0d2f7e0a1c11"
  }
}
```

| Status | Code | Meaning |
|--------|------|---------|
| 400 | BAD_REQUEST, VALIDATION_FAILED | Malformed body or invalid fields. |
| 401 | UNAUTHORIZED | Missing, invalid or expired token, or bad credentials. |
| 403 | FORBIDDEN | Authenticated but not allowed. |
| 404 | NOT_FOUND | Resource missing or not visible to the caller. |
| 405 | METHOD_NOT_ALLOWED | Wrong HTTP method. |
| 409 | CONFLICT | Duplicate email, insufficient stock, invalid status transition. |
| 500 | INTERNAL_ERROR | Unexpected failure. Details are only in logs. |
| 503 | SERVICE_UNAVAILABLE | Product Service unavailable, circuit open, or request cancelled. |

## Auth

### POST /auth/register

```json
{ "email": "alice@example.com", "password": "StrongPass123" }
```

201:

```json
{ "id": 1, "email": "alice@example.com", "role": "CUSTOMER", "createdAt": "2026-10-26T10:00:00Z" }
```

Password must be 8 to 72 characters.

### POST /auth/login

```json
{ "email": "alice@example.com", "password": "StrongPass123" }
```

200:

```json
{ "accessToken": "eyJ...", "tokenType": "Bearer", "expiresIn": 900 }
```

## Orders

### POST /orders

Roles: CUSTOMER (own customerId only), ADMIN.

```json
{ "customerId": 1, "productId": 501, "quantity": 2 }
```

201 with `Location: /api/v1/orders/42`:

```json
{
  "id": 42,
  "customerId": 1,
  "productId": 501,
  "quantity": 2,
  "unitPrice": 79.99,
  "totalPrice": 159.98,
  "status": "CREATED",
  "createdAt": "2026-10-26T10:01:00Z",
  "updatedAt": "2026-10-26T10:01:00Z"
}
```

Rules: quantity 1 to 1000, product must exist, stock must cover the quantity.

### GET /orders/{id}

200 with the order, 404 if missing or owned by someone else.

### GET /orders

Query: `page` (default 1), `pageSize` (default 20, max 100), `status` (CREATED, CONFIRMED, CANCELLED).

```json
{ "items": [], "total": 0, "page": 1, "pageSize": 20 }
```

### PUT /orders/{id}/status

```json
{ "status": "CONFIRMED" }
```

| From | To | Who |
|------|----|-----|
| CREATED | CONFIRMED | ADMIN |
| CREATED | CANCELLED | ADMIN, owner |
| CONFIRMED | CANCELLED | ADMIN, owner |

## Health

- `GET /health/live` returns 200 while the process runs.
- `GET /health/ready` returns 200 when PostgreSQL answers within 2 seconds, otherwise 503.

## Metrics

Served on `METRICS_PORT` (default 9090) at `/metrics`, not on the public port: `http_requests_total`, `http_request_duration_seconds`, `http_errors_total`, `db_errors_total`, `product_service_calls_total`.

## GraphQL

`POST /graphql` with a bearer token and body `{ "query": "...", "variables": {}, "operationName": "" }`.

```graphql
type Order {
  id: Int!
  customerId: Int!
  productId: Int!
  quantity: Int!
  unitPrice: Float!
  totalPrice: Float!
  status: String!
  createdAt: String!
  updatedAt: String!
}

type OrderPage {
  items: [Order!]!
  total: Int!
  page: Int!
  pageSize: Int!
}

type Query {
  order(id: Int!): Order
  orders(page: Int = 1, pageSize: Int = 20, status: String): OrderPage!
}

type Mutation {
  createOrder(customerId: Int!, productId: Int!, quantity: Int!): Order!
  updateOrderStatus(id: Int!, status: String!): Order!
}
```

GraphQL calls the same service layer as REST, so authorization and validation rules are identical. REST is the primary interface. GraphQL is useful for clients that need to shape responses, for example dashboards.

## Product Service (internal)

Requires header `X-Internal-Token`.

- `GET /products/{id}` returns `{ "id": 501, "name": "...", "price": 79.99, "stock": 500 }` or 404.
- `POST /chaos/on` and `POST /chaos/off` toggle simulated outages for the resilience demo.
