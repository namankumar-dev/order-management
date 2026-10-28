package tests

import (
	"fmt"
	"math"
	"net/http"
	"testing"

	"order-management/model"
)

type errorEnvelope struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		Details   map[string]string `json:"details"`
		RequestID string            `json:"requestId"`
	} `json:"error"`
}

func expectStatus(t *testing.T, got, want int, body string) {
	t.Helper()
	if got != want {
		t.Fatalf("status = %d, want %d, body = %s", got, want, body)
	}
}

func TestOrderAPIFlow(t *testing.T) {
	env := newEnv(t)
	aliceToken, aliceID := env.customer(t, "alice@example.com")
	bobToken, bobID := env.customer(t, "bob@example.com")
	adminToken := env.admin(t)

	rec := env.serve(http.MethodPost, "/api/v1/orders", "", orderBody(aliceID, 501, 1))
	expectStatus(t, rec.Code, http.StatusUnauthorized, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/orders", aliceToken, orderBody(aliceID, 501, 2))
	expectStatus(t, rec.Code, http.StatusCreated, rec.Body.String())
	var created model.Order
	decode(t, rec, &created)
	if created.Status != model.StatusCreated {
		t.Fatalf("status = %s", created.Status)
	}
	if math.Abs(created.TotalPrice-159.98) > 0.001 {
		t.Fatalf("total = %v", created.TotalPrice)
	}
	if rec.Header().Get("Location") != fmt.Sprintf("/api/v1/orders/%d", created.ID) {
		t.Fatalf("location header = %q", rec.Header().Get("Location"))
	}

	rec = env.serve(http.MethodPost, "/api/v1/orders", aliceToken, orderBody(aliceID, 501, 0))
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())
	var verr errorEnvelope
	decode(t, rec, &verr)
	if verr.Error.Code != "VALIDATION_FAILED" || verr.Error.Details["quantity"] == "" || verr.Error.RequestID == "" {
		t.Fatalf("unexpected validation error: %+v", verr)
	}

	rec = env.serve(http.MethodPost, "/api/v1/orders", aliceToken, map[string]any{"customerId": aliceID, "productId": 501, "quantity": 1, "price": 0})
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/orders", bobToken, orderBody(aliceID, 501, 1))
	expectStatus(t, rec.Code, http.StatusForbidden, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/orders", aliceToken, orderBody(aliceID, 999, 1))
	expectStatus(t, rec.Code, http.StatusNotFound, rec.Body.String())

	rec = env.serve(http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", created.ID), aliceToken, nil)
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())

	rec = env.serve(http.MethodGet, fmt.Sprintf("/api/v1/orders/%d", created.ID), bobToken, nil)
	expectStatus(t, rec.Code, http.StatusNotFound, rec.Body.String())

	rec = env.serve(http.MethodGet, "/api/v1/orders/abc", aliceToken, nil)
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())

	rec = env.serve(http.MethodGet, "/api/v1/orders/9999", adminToken, nil)
	expectStatus(t, rec.Code, http.StatusNotFound, rec.Body.String())

	statusPath := fmt.Sprintf("/api/v1/orders/%d/status", created.ID)
	rec = env.serve(http.MethodPut, statusPath, aliceToken, map[string]string{"status": "CONFIRMED"})
	expectStatus(t, rec.Code, http.StatusForbidden, rec.Body.String())

	rec = env.serve(http.MethodPut, statusPath, adminToken, map[string]string{"status": "CONFIRMED"})
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	var confirmed model.Order
	decode(t, rec, &confirmed)
	if confirmed.Status != model.StatusConfirmed {
		t.Fatalf("status = %s", confirmed.Status)
	}

	rec = env.serve(http.MethodPut, statusPath, adminToken, map[string]string{"status": "BOGUS"})
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())

	rec = env.serve(http.MethodPut, statusPath, aliceToken, map[string]string{"status": "CANCELLED"})
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())

	rec = env.serve(http.MethodPut, statusPath, aliceToken, map[string]string{"status": "CANCELLED"})
	expectStatus(t, rec.Code, http.StatusConflict, rec.Body.String())

	var page model.OrderPage
	rec = env.serve(http.MethodGet, "/api/v1/orders?pageSize=10", aliceToken, nil)
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	decode(t, rec, &page)
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("alice page = %+v", page)
	}

	rec = env.serve(http.MethodGet, "/api/v1/orders", bobToken, nil)
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	decode(t, rec, &page)
	if page.Total != 0 || page.Items == nil {
		t.Fatalf("bob page = %+v", page)
	}

	rec = env.serve(http.MethodGet, "/api/v1/orders?status=NOPE", adminToken, nil)
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())

	rec = env.serve(http.MethodGet, "/api/v1/orders?status=CANCELLED", adminToken, nil)
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	decode(t, rec, &page)
	if page.Total != 1 {
		t.Fatalf("admin page = %+v", page)
	}

	_ = bobID
}

func TestAuthentication(t *testing.T) {
	env := newEnv(t)
	env.customer(t, "carol@example.com")

	rec := env.serve(http.MethodPost, "/api/v1/auth/register", "", map[string]string{"email": "carol@example.com", "password": "StrongPass123"})
	expectStatus(t, rec.Code, http.StatusConflict, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/auth/register", "", map[string]string{"email": "dave@example.com", "password": "short"})
	expectStatus(t, rec.Code, http.StatusBadRequest, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "carol@example.com", "password": "WrongPassword1"})
	expectStatus(t, rec.Code, http.StatusUnauthorized, rec.Body.String())

	rec = env.serve(http.MethodPost, "/api/v1/auth/login", "", map[string]string{"email": "nobody@example.com", "password": "WrongPassword1"})
	expectStatus(t, rec.Code, http.StatusUnauthorized, rec.Body.String())

	rec = env.serve(http.MethodGet, "/api/v1/orders", "not-a-jwt", nil)
	expectStatus(t, rec.Code, http.StatusUnauthorized, rec.Body.String())
}

func TestHealthHeadersAndRouting(t *testing.T) {
	env := newEnv(t)

	for _, path := range []string{"/health/live", "/health/ready"} {
		rec := env.serve(http.MethodGet, path, "", nil)
		expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s missing security header", path)
		}
		if rec.Header().Get("X-Request-ID") == "" {
			t.Fatalf("%s missing request id", path)
		}
	}

	rec := env.serve(http.MethodGet, "/nope", "", nil)
	expectStatus(t, rec.Code, http.StatusNotFound, rec.Body.String())
	var env404 errorEnvelope
	decode(t, rec, &env404)
	if env404.Error.Code != "NOT_FOUND" {
		t.Fatalf("code = %s", env404.Error.Code)
	}

	rec = env.serve(http.MethodDelete, "/api/v1/auth/login", "", nil)
	expectStatus(t, rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
}

func TestGraphQL(t *testing.T) {
	env := newEnv(t)
	token, id := env.customer(t, "erin@example.com")

	mutation := fmt.Sprintf(`mutation { createOrder(customerId: %d, productId: 501, quantity: 3) { id status totalPrice } }`, id)
	rec := env.serve(http.MethodPost, "/graphql", token, map[string]any{"query": mutation})
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	var created struct {
		Data struct {
			CreateOrder struct {
				ID     int    `json:"id"`
				Status string `json:"status"`
			} `json:"createOrder"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	decode(t, rec, &created)
	if len(created.Errors) > 0 || created.Data.CreateOrder.Status != "CREATED" {
		t.Fatalf("unexpected result: %s", rec.Body.String())
	}

	rec = env.serve(http.MethodPost, "/graphql", token, map[string]any{"query": `{ orders { total items { id status } } }`})
	expectStatus(t, rec.Code, http.StatusOK, rec.Body.String())
	var list struct {
		Data struct {
			Orders struct {
				Total int `json:"total"`
			} `json:"orders"`
		} `json:"data"`
		Errors []any `json:"errors"`
	}
	decode(t, rec, &list)
	if len(list.Errors) > 0 || list.Data.Orders.Total != 1 {
		t.Fatalf("unexpected result: %s", rec.Body.String())
	}

	rec = env.serve(http.MethodPost, "/graphql", "", map[string]any{"query": `{ orders { total } }`})
	expectStatus(t, rec.Code, http.StatusUnauthorized, rec.Body.String())
}
