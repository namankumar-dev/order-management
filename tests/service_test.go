package tests

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"order-management/apperror"
	"order-management/client"
	"order-management/model"
	"order-management/service"
)

func statusOf(err error) int {
	if ae, ok := apperror.As(err); ok {
		return ae.Status
	}
	return 0
}

func newService() (service.OrderService, *fakeOrderRepo, *fakeProducts) {
	repo := newFakeOrderRepo()
	products := newFakeProducts()
	return service.NewOrderService(repo, products), repo, products
}

func TestCreateOrderComputesTotals(t *testing.T) {
	svc, _, _ := newService()
	actor := model.Actor{UserID: 7, Role: model.RoleCustomer}

	order, err := svc.Create(context.Background(), actor, model.CreateOrderRequest{CustomerID: 7, ProductID: 502, Quantity: 3})
	if err != nil {
		t.Fatal(err)
	}
	if order.UnitPrice != 34.5 || order.TotalPrice != 103.5 || order.Status != model.StatusCreated {
		t.Fatalf("unexpected order: %+v", order)
	}
}

func TestCreateOrderRules(t *testing.T) {
	ctx := context.Background()
	customer := model.Actor{UserID: 7, Role: model.RoleCustomer}
	admin := model.Actor{UserID: 1, Role: model.RoleAdmin}

	t.Run("customer cannot order for someone else", func(t *testing.T) {
		svc, _, _ := newService()
		_, err := svc.Create(ctx, customer, model.CreateOrderRequest{CustomerID: 8, ProductID: 501, Quantity: 1})
		if statusOf(err) != http.StatusForbidden {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("admin can order for a customer", func(t *testing.T) {
		svc, _, _ := newService()
		if _, err := svc.Create(ctx, admin, model.CreateOrderRequest{CustomerID: 8, ProductID: 501, Quantity: 1}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unknown product", func(t *testing.T) {
		svc, _, _ := newService()
		_, err := svc.Create(ctx, customer, model.CreateOrderRequest{CustomerID: 7, ProductID: 12345, Quantity: 1})
		if statusOf(err) != http.StatusNotFound {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("insufficient stock", func(t *testing.T) {
		svc, repo, _ := newService()
		_, err := svc.Create(ctx, customer, model.CreateOrderRequest{CustomerID: 7, ProductID: 502, Quantity: 4})
		if statusOf(err) != http.StatusConflict {
			t.Fatalf("err = %v", err)
		}
		if repo.count() != 0 {
			t.Fatal("order must not be stored")
		}
	})

	t.Run("product service unavailable", func(t *testing.T) {
		svc, repo, products := newService()
		products.err = client.ErrProductServiceUnavailable
		_, err := svc.Create(ctx, customer, model.CreateOrderRequest{CustomerID: 7, ProductID: 501, Quantity: 1})
		if statusOf(err) != http.StatusServiceUnavailable {
			t.Fatalf("err = %v", err)
		}
		if repo.count() != 0 {
			t.Fatal("order must not be stored")
		}
	})

	t.Run("cancelled context", func(t *testing.T) {
		svc, _, products := newService()
		products.err = context.Canceled
		_, err := svc.Create(ctx, customer, model.CreateOrderRequest{CustomerID: 7, ProductID: 501, Quantity: 1})
		if statusOf(err) != http.StatusServiceUnavailable || !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestStatusTransitions(t *testing.T) {
	ctx := context.Background()
	admin := model.Actor{UserID: 1, Role: model.RoleAdmin}
	owner := model.Actor{UserID: 7, Role: model.RoleCustomer}

	cases := []struct {
		name  string
		setup []model.OrderStatus
		actor model.Actor
		to    model.OrderStatus
		want  int
	}{
		{"created to confirmed by admin", nil, admin, model.StatusConfirmed, 200},
		{"created to cancelled by admin", nil, admin, model.StatusCancelled, 200},
		{"created to cancelled by owner", nil, owner, model.StatusCancelled, 200},
		{"created to confirmed by owner", nil, owner, model.StatusConfirmed, http.StatusForbidden},
		{"confirmed to cancelled by admin", []model.OrderStatus{model.StatusConfirmed}, admin, model.StatusCancelled, 200},
		{"cancelled to confirmed", []model.OrderStatus{model.StatusCancelled}, admin, model.StatusConfirmed, http.StatusConflict},
		{"confirmed to created", []model.OrderStatus{model.StatusConfirmed}, admin, model.StatusCreated, http.StatusConflict},
		{"other customer", nil, model.Actor{UserID: 9, Role: model.RoleCustomer}, model.StatusCancelled, http.StatusNotFound},
		{"invalid status", nil, admin, model.OrderStatus("NOPE"), http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _ := newService()
			order, err := svc.Create(ctx, owner, model.CreateOrderRequest{CustomerID: 7, ProductID: 501, Quantity: 1})
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range tc.setup {
				if _, err := svc.UpdateStatus(ctx, admin, order.ID, s); err != nil {
					t.Fatal(err)
				}
			}
			_, err = svc.UpdateStatus(ctx, tc.actor, order.ID, tc.to)
			got := 200
			if err != nil {
				got = statusOf(err)
			}
			if got != tc.want {
				t.Fatalf("status = %d, want %d (err=%v)", got, tc.want, err)
			}
		})
	}
}

func TestCanTransition(t *testing.T) {
	if !model.CanTransition(model.StatusCreated, model.StatusConfirmed) {
		t.Fatal("created -> confirmed must be allowed")
	}
	if model.CanTransition(model.StatusCancelled, model.StatusCreated) {
		t.Fatal("cancelled -> created must be rejected")
	}
}
