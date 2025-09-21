package service

import (
	"context"

	"l0-demo/internal/models"
	"l0-demo/internal/repository"
)

//go:generate mockgen -source=service.go -destination=mocks/mock.go

type Order interface {
	GetCachedOrder(uid string) (models.Order, error)
	GetAllCachedOrders() ([]models.Order, error)
	GetAllDbOrders() ([]models.Order, error)
	GetDbOrder(uid string) (models.Order, error)
	PutOrdersFromDbToCache() error
	PutCachedOrder(order models.Order)
	PutDbOrder(order models.Order) error

	HandleMessage(ctx context.Context, payload []byte) error
}

type Service struct {
	repository.OrderCache
	repository.OrderPostgres
}

func NewService(repository *repository.Repository) *Service {
	return &Service{
		OrderCache:    repository.OrderCache,
		OrderPostgres: repository.OrderPostgres,
	}
}
