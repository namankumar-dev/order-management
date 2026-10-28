package service

import (
	"context"
	"errors"
	"fmt"
	"math"

	"order-management/apperror"
	"order-management/client"
	"order-management/model"
	"order-management/repository"
	"order-management/trace"
)

type OrderService interface {
	Create(ctx context.Context, actor model.Actor, req model.CreateOrderRequest) (*model.Order, error)
	Get(ctx context.Context, actor model.Actor, id uint) (*model.Order, error)
	List(ctx context.Context, actor model.Actor, q model.ListQuery) (*model.OrderPage, error)
	UpdateStatus(ctx context.Context, actor model.Actor, id uint, status model.OrderStatus) (*model.Order, error)
}

type orderService struct {
	repo     repository.OrderRepository
	products client.ProductClient
}

func NewOrderService(repo repository.OrderRepository, products client.ProductClient) OrderService {
	return &orderService{repo: repo, products: products}
}

func (s *orderService) Create(ctx context.Context, actor model.Actor, req model.CreateOrderRequest) (*model.Order, error) {
	return trace.Run(ctx, "service.create_order", func(ctx context.Context) (*model.Order, error) {
		if actor.Role != model.RoleAdmin && req.CustomerID != actor.UserID {
			return nil, apperror.Forbidden("cannot create orders for another customer")
		}

		product, err := s.products.GetProduct(ctx, req.ProductID)
		if err != nil {
			return nil, mapProductError(err)
		}
		if product.Stock < req.Quantity {
			return nil, apperror.Conflict("insufficient stock")
		}

		unit := round2(product.Price)
		order := &model.Order{
			CustomerID: req.CustomerID,
			ProductID:  req.ProductID,
			Quantity:   req.Quantity,
			UnitPrice:  unit,
			TotalPrice: round2(unit * float64(req.Quantity)),
			Status:     model.StatusCreated,
		}
		if err := s.repo.Create(ctx, order); err != nil {
			return nil, err
		}
		return order, nil
	})
}

func (s *orderService) Get(ctx context.Context, actor model.Actor, id uint) (*model.Order, error) {
	return trace.Run(ctx, "service.get_order", func(ctx context.Context) (*model.Order, error) {
		order, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if !canAccess(actor, order) {
			return nil, apperror.NotFound("order not found")
		}
		return order, nil
	})
}

func (s *orderService) List(ctx context.Context, actor model.Actor, q model.ListQuery) (*model.OrderPage, error) {
	return trace.Run(ctx, "service.list_orders", func(ctx context.Context) (*model.OrderPage, error) {
		q.Normalize()
		var customerID *uint
		if actor.Role != model.RoleAdmin {
			id := actor.UserID
			customerID = &id
		}
		items, total, err := s.repo.List(ctx, q, customerID)
		if err != nil {
			return nil, err
		}
		if items == nil {
			items = []model.Order{}
		}
		return &model.OrderPage{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}, nil
	})
}

func (s *orderService) UpdateStatus(ctx context.Context, actor model.Actor, id uint, status model.OrderStatus) (*model.Order, error) {
	return trace.Run(ctx, "service.update_order_status", func(ctx context.Context) (*model.Order, error) {
		if !status.Valid() {
			return nil, apperror.BadRequest("invalid status")
		}
		return s.repo.UpdateStatus(ctx, id, func(order *model.Order) error {
			if !canAccess(actor, order) {
				return apperror.NotFound("order not found")
			}
			if actor.Role != model.RoleAdmin && status != model.StatusCancelled {
				return apperror.Forbidden("customers may only cancel orders")
			}
			if !model.CanTransition(order.Status, status) {
				return apperror.Conflict(fmt.Sprintf("cannot change status from %s to %s", order.Status, status))
			}
			order.Status = status
			return nil
		})
	})
}

func canAccess(actor model.Actor, order *model.Order) bool {
	return actor.Role == model.RoleAdmin || order.CustomerID == actor.UserID
}

func mapProductError(err error) error {
	switch {
	case errors.Is(err, client.ErrProductNotFound):
		return apperror.NotFound("product not found")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return apperror.Unavailable("request was cancelled or timed out").WithCause(err)
	default:
		return apperror.Unavailable("product service is temporarily unavailable").WithCause(err)
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
