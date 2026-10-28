package controller

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"order-management/model"
	"order-management/service"
	"order-management/trace"
)

type OrderController struct {
	svc service.OrderService
}

func NewOrderController(svc service.OrderService) *OrderController {
	return &OrderController{svc: svc}
}

func (oc *OrderController) Create(c *gin.Context) {
	actor, ok := actorOf(c)
	if !ok {
		return
	}
	var req model.CreateOrderRequest
	if !bindJSON(c, &req) {
		return
	}
	order, err := trace.Run(c.Request.Context(), "controller.create_order", func(ctx context.Context) (*model.Order, error) {
		return oc.svc.Create(ctx, actor, req)
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Location", fmt.Sprintf("/api/v1/orders/%d", order.ID))
	c.JSON(http.StatusCreated, order)
}

func (oc *OrderController) Get(c *gin.Context) {
	actor, ok := actorOf(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	order, err := trace.Run(c.Request.Context(), "controller.get_order", func(ctx context.Context) (*model.Order, error) {
		return oc.svc.Get(ctx, actor, id)
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

func (oc *OrderController) List(c *gin.Context) {
	actor, ok := actorOf(c)
	if !ok {
		return
	}
	var q model.ListQuery
	if !bindQuery(c, &q) {
		return
	}
	page, err := trace.Run(c.Request.Context(), "controller.list_orders", func(ctx context.Context) (*model.OrderPage, error) {
		return oc.svc.List(ctx, actor, q)
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (oc *OrderController) UpdateStatus(c *gin.Context) {
	actor, ok := actorOf(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.UpdateStatusRequest
	if !bindJSON(c, &req) {
		return
	}
	order, err := trace.Run(c.Request.Context(), "controller.update_order_status", func(ctx context.Context) (*model.Order, error) {
		return oc.svc.UpdateStatus(ctx, actor, id, req.Status)
	})
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}
