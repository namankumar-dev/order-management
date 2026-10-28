package graph

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/graphql-go/graphql"

	"order-management/apperror"
	"order-management/middleware"
	"order-management/model"
	"order-management/service"
	"order-management/validation"
)

type actorKey struct{}

func WithActor(ctx context.Context, actor model.Actor) context.Context {
	return context.WithValue(ctx, actorKey{}, actor)
}

func actorFrom(ctx context.Context) (model.Actor, error) {
	actor, ok := ctx.Value(actorKey{}).(model.Actor)
	if !ok {
		return model.Actor{}, errors.New("authentication required")
	}
	return actor, nil
}

func publicError(err error) error {
	if ae, ok := apperror.As(err); ok {
		return errors.New(ae.Message)
	}
	return errors.New("internal server error")
}

func validationError(err error) error {
	ae := validation.Translate(err)
	return fmt.Errorf("%s: %v", ae.Message, ae.Details)
}

func asOrder(src interface{}) (model.Order, bool) {
	switch v := src.(type) {
	case model.Order:
		return v, true
	case *model.Order:
		if v != nil {
			return *v, true
		}
	}
	return model.Order{}, false
}

func orderField(t graphql.Output, fn func(model.Order) interface{}) *graphql.Field {
	return &graphql.Field{
		Type: t,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			order, ok := asOrder(p.Source)
			if !ok {
				return nil, errors.New("invalid order source")
			}
			return fn(order), nil
		},
	}
}

func pageField(t graphql.Output, fn func(*model.OrderPage) interface{}) *graphql.Field {
	return &graphql.Field{
		Type: t,
		Resolve: func(p graphql.ResolveParams) (interface{}, error) {
			page, ok := p.Source.(*model.OrderPage)
			if !ok || page == nil {
				return nil, errors.New("invalid page source")
			}
			return fn(page), nil
		},
	}
}

func NewSchema(svc service.OrderService) (graphql.Schema, error) {
	nonNullInt := graphql.NewNonNull(graphql.Int)
	nonNullFloat := graphql.NewNonNull(graphql.Float)
	nonNullString := graphql.NewNonNull(graphql.String)

	orderType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Order",
		Fields: graphql.Fields{
			"id":         orderField(nonNullInt, func(o model.Order) interface{} { return int(o.ID) }),
			"customerId": orderField(nonNullInt, func(o model.Order) interface{} { return int(o.CustomerID) }),
			"productId":  orderField(nonNullInt, func(o model.Order) interface{} { return int(o.ProductID) }),
			"quantity":   orderField(nonNullInt, func(o model.Order) interface{} { return o.Quantity }),
			"unitPrice":  orderField(nonNullFloat, func(o model.Order) interface{} { return o.UnitPrice }),
			"totalPrice": orderField(nonNullFloat, func(o model.Order) interface{} { return o.TotalPrice }),
			"status":     orderField(nonNullString, func(o model.Order) interface{} { return string(o.Status) }),
			"createdAt": orderField(nonNullString, func(o model.Order) interface{} {
				return o.CreatedAt.UTC().Format(time.RFC3339)
			}),
			"updatedAt": orderField(nonNullString, func(o model.Order) interface{} {
				return o.UpdatedAt.UTC().Format(time.RFC3339)
			}),
		},
	})

	pageType := graphql.NewObject(graphql.ObjectConfig{
		Name: "OrderPage",
		Fields: graphql.Fields{
			"items": pageField(
				graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(orderType))),
				func(p *model.OrderPage) interface{} { return p.Items },
			),
			"total":    pageField(nonNullInt, func(p *model.OrderPage) interface{} { return int(p.Total) }),
			"page":     pageField(nonNullInt, func(p *model.OrderPage) interface{} { return p.Page }),
			"pageSize": pageField(nonNullInt, func(p *model.OrderPage) interface{} { return p.PageSize }),
		},
	})

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"order": &graphql.Field{
				Type: orderType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: nonNullInt},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					actor, err := actorFrom(p.Context)
					if err != nil {
						return nil, err
					}
					id, _ := p.Args["id"].(int)
					if id <= 0 {
						return nil, errors.New("id must be a positive integer")
					}
					order, err := svc.Get(p.Context, actor, uint(id))
					if err != nil {
						return nil, publicError(err)
					}
					return order, nil
				},
			},
			"orders": &graphql.Field{
				Type: graphql.NewNonNull(pageType),
				Args: graphql.FieldConfigArgument{
					"page":     &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 1},
					"pageSize": &graphql.ArgumentConfig{Type: graphql.Int, DefaultValue: 20},
					"status":   &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					actor, err := actorFrom(p.Context)
					if err != nil {
						return nil, err
					}
					var q model.ListQuery
					q.Page, _ = p.Args["page"].(int)
					q.PageSize, _ = p.Args["pageSize"].(int)
					q.Status, _ = p.Args["status"].(string)
					if err := binding.Validator.ValidateStruct(&q); err != nil {
						return nil, validationError(err)
					}
					page, err := svc.List(p.Context, actor, q)
					if err != nil {
						return nil, publicError(err)
					}
					return page, nil
				},
			},
		},
	})

	mutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createOrder": &graphql.Field{
				Type: graphql.NewNonNull(orderType),
				Args: graphql.FieldConfigArgument{
					"customerId": &graphql.ArgumentConfig{Type: nonNullInt},
					"productId":  &graphql.ArgumentConfig{Type: nonNullInt},
					"quantity":   &graphql.ArgumentConfig{Type: nonNullInt},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					actor, err := actorFrom(p.Context)
					if err != nil {
						return nil, err
					}
					customerID, _ := p.Args["customerId"].(int)
					productID, _ := p.Args["productId"].(int)
					quantity, _ := p.Args["quantity"].(int)
					if customerID <= 0 || productID <= 0 {
						return nil, errors.New("customerId and productId must be positive integers")
					}
					req := model.CreateOrderRequest{
						CustomerID: uint(customerID),
						ProductID:  uint(productID),
						Quantity:   quantity,
					}
					if err := binding.Validator.ValidateStruct(&req); err != nil {
						return nil, validationError(err)
					}
					order, err := svc.Create(p.Context, actor, req)
					if err != nil {
						return nil, publicError(err)
					}
					return order, nil
				},
			},
			"updateOrderStatus": &graphql.Field{
				Type: graphql.NewNonNull(orderType),
				Args: graphql.FieldConfigArgument{
					"id":     &graphql.ArgumentConfig{Type: nonNullInt},
					"status": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					actor, err := actorFrom(p.Context)
					if err != nil {
						return nil, err
					}
					id, _ := p.Args["id"].(int)
					status, _ := p.Args["status"].(string)
					if id <= 0 {
						return nil, errors.New("id must be a positive integer")
					}
					order, err := svc.UpdateStatus(p.Context, actor, uint(id), model.OrderStatus(status))
					if err != nil {
						return nil, publicError(err)
					}
					return order, nil
				},
			},
		},
	})

	return graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
}

type request struct {
	Query         string                 `json:"query" binding:"required,max=10000"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operationName" binding:"max=100"`
}

func Handler(schema graphql.Schema) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, ok := middleware.Actor(c)
		if !ok {
			_ = c.Error(apperror.Unauthorized("authentication required"))
			return
		}
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			_ = c.Error(validation.Translate(err))
			return
		}
		result := graphql.Do(graphql.Params{
			Schema:         schema,
			RequestString:  req.Query,
			VariableValues: req.Variables,
			OperationName:  req.OperationName,
			Context:        WithActor(c.Request.Context(), actor),
		})
		c.JSON(http.StatusOK, result)
	}
}
