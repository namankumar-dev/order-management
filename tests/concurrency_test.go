package tests

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"order-management/model"
)

func TestHundredConcurrentOrderRequests(t *testing.T) {
	env := newEnv(t)
	token, id := env.customer(t, "load@example.com")

	const n = 100
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rec := env.serve(http.MethodPost, "/api/v1/orders", token, orderBody(id, 501, 1))
			codes[i] = rec.Code
		}(i)
	}
	close(start)
	wg.Wait()

	for i, code := range codes {
		if code != http.StatusCreated {
			t.Fatalf("request %d returned %d", i, code)
		}
	}
	if env.orders.count() != n {
		t.Fatalf("stored %d orders, want %d", env.orders.count(), n)
	}
}

func TestConcurrentStatusUpdatesApplyOnce(t *testing.T) {
	env := newEnv(t)
	token, id := env.customer(t, "race@example.com")
	adminToken := env.admin(t)

	rec := env.serve(http.MethodPost, "/api/v1/orders", token, orderBody(id, 501, 1))
	expectStatus(t, rec.Code, http.StatusCreated, rec.Body.String())
	var order model.Order
	decode(t, rec, &order)

	const n = 50
	codes := make([]int, n)
	var wg sync.WaitGroup
	path := fmt.Sprintf("/api/v1/orders/%d/status", order.ID)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i] = env.serve(http.MethodPut, path, adminToken, map[string]string{"status": "CONFIRMED"}).Code
		}(i)
	}
	wg.Wait()

	ok, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusConflict:
			conflict++
		}
	}
	if ok != 1 || conflict != n-1 {
		t.Fatalf("ok=%d conflict=%d", ok, conflict)
	}
}
