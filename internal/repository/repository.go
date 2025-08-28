package repository

import (
	"l0-demo/internal/models"
	"l0-demo/internal/repository/postgres"
	"l0-demo/internal/repository/cache"

	"github.com/jinzhu/gorm"
)

type OrderPostgres interface {
    Create(ord models.Order) error                    
    CreateOrUpdate(ord models.Order) error            
    Get(uid string) (models.Order, error)
    GetAll() ([]models.Order, error)
}

type OrderCache interface {
	PutOrder(uid string, order models.Order)
	GetOrder(uid string) (models.Order, error)
	GetAllOrders() ([]models.Order, error)
}

type Repository struct {
	OrderPostgres
	OrderCache
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		OrderPostgres: postgres.NewOrderPostgres(db),
		OrderCache:    cache.NewOrderCache(cache.NewCache()),
	}
}