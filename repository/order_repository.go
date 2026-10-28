package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"order-management/apperror"
	"order-management/model"
	"order-management/trace"
)

type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) error
	GetByID(ctx context.Context, id uint) (*model.Order, error)
	List(ctx context.Context, q model.ListQuery, customerID *uint) ([]model.Order, int64, error)
	UpdateStatus(ctx context.Context, id uint, apply func(*model.Order) error) (*model.Order, error)
	Ping(ctx context.Context) error
}

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) Create(ctx context.Context, order *model.Order) error {
	_, err := trace.Run(ctx, "repository.create_order(postgres)", func(ctx context.Context) (struct{}, error) {
		if err := r.db.WithContext(ctx).Create(order).Error; err != nil {
			return struct{}{}, dbError(err)
		}
		return struct{}{}, nil
	})
	return err
}

func (r *orderRepository) GetByID(ctx context.Context, id uint) (*model.Order, error) {
	return trace.Run(ctx, "repository.get_order(postgres)", func(ctx context.Context) (*model.Order, error) {
		var order model.Order
		err := r.db.WithContext(ctx).First(&order, id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("order not found")
		}
		if err != nil {
			return nil, dbError(err)
		}
		return &order, nil
	})
}

type listResult struct {
	items []model.Order
	total int64
}

func (r *orderRepository) List(ctx context.Context, q model.ListQuery, customerID *uint) ([]model.Order, int64, error) {
	res, err := trace.Run(ctx, "repository.list_orders(postgres)", func(ctx context.Context) (listResult, error) {
		scope := func() *gorm.DB {
			tx := r.db.WithContext(ctx).Model(&model.Order{})
			if customerID != nil {
				tx = tx.Where("customer_id = ?", *customerID)
			}
			if q.Status != "" {
				tx = tx.Where("status = ?", q.Status)
			}
			return tx
		}

		var out listResult
		if err := scope().Count(&out.total).Error; err != nil {
			return out, dbError(err)
		}
		if out.total == 0 {
			return out, nil
		}
		err := scope().Order("id DESC").Limit(q.PageSize).Offset(q.Offset()).Find(&out.items).Error
		if err != nil {
			return out, dbError(err)
		}
		return out, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return res.items, res.total, nil
}

func (r *orderRepository) UpdateStatus(ctx context.Context, id uint, apply func(*model.Order) error) (*model.Order, error) {
	return trace.Run(ctx, "repository.update_order_status(postgres)", func(ctx context.Context) (*model.Order, error) {
		var order model.Order
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, id).Error; err != nil {
				return err
			}
			if err := apply(&order); err != nil {
				return err
			}
			return tx.Save(&order).Error
		})
		if err == nil {
			return &order, nil
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperror.NotFound("order not found")
		}
		if _, ok := apperror.As(err); ok {
			return nil, err
		}
		return nil, dbError(err)
	})
}

func (r *orderRepository) Ping(ctx context.Context) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
