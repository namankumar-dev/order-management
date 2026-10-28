package model

import "time"

type OrderStatus string

const (
	StatusCreated   OrderStatus = "CREATED"
	StatusConfirmed OrderStatus = "CONFIRMED"
	StatusCancelled OrderStatus = "CANCELLED"
)

func (s OrderStatus) Valid() bool {
	switch s {
	case StatusCreated, StatusConfirmed, StatusCancelled:
		return true
	}
	return false
}

var transitions = map[OrderStatus][]OrderStatus{
	StatusCreated:   {StatusConfirmed, StatusCancelled},
	StatusConfirmed: {StatusCancelled},
}

func CanTransition(from, to OrderStatus) bool {
	for _, allowed := range transitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

type Order struct {
	ID         uint        `gorm:"primaryKey" json:"id"`
	CustomerID uint        `gorm:"not null;index" json:"customerId"`
	ProductID  uint        `gorm:"not null" json:"productId"`
	Quantity   int         `gorm:"not null" json:"quantity"`
	UnitPrice  float64     `gorm:"type:numeric(12,2);not null" json:"unitPrice"`
	TotalPrice float64     `gorm:"type:numeric(12,2);not null" json:"totalPrice"`
	Status     OrderStatus `gorm:"type:varchar(20);not null;index" json:"status"`
	CreatedAt  time.Time   `json:"createdAt"`
	UpdatedAt  time.Time   `json:"updatedAt"`
}

type CreateOrderRequest struct {
	CustomerID uint `json:"customerId" binding:"required,gt=0"`
	ProductID  uint `json:"productId" binding:"required,gt=0"`
	Quantity   int  `json:"quantity" binding:"required,gte=1,lte=1000"`
}

type UpdateStatusRequest struct {
	Status OrderStatus `json:"status" binding:"required,oneof=CREATED CONFIRMED CANCELLED"`
}

type ListQuery struct {
	Page     int    `form:"page" json:"page" binding:"omitempty,gte=1,lte=100000"`
	PageSize int    `form:"pageSize" json:"pageSize" binding:"omitempty,gte=1,lte=100"`
	Status   string `form:"status" json:"status" binding:"omitempty,oneof=CREATED CONFIRMED CANCELLED"`
}

func (q *ListQuery) Normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
}

func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.PageSize
}

type OrderPage struct {
	Items    []Order `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
}
