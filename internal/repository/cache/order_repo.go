package cache

import (
	"fmt"

	"l0-demo/internal/models"

	"net/http"
)

type OrderCacheRepo struct {
	cch KV
}

func NewOrderCache(cch KV) *OrderCacheRepo {
	return &OrderCacheRepo{cch: cch}
}

func (o *OrderCacheRepo) PutOrder(uid string, ord models.Order) {
	o.cch.Put(uid, ord)
}

func (o *OrderCacheRepo) GetOrder(uid string) (models.Order, error) {
	v, ok := o.cch.Get(uid)
	if !ok {
		return models.Order{}, NewErrorHandler(fmt.Errorf("order %s not found", uid), http.StatusNotFound)
	}

	ord, ok := v.(models.Order)
	if !ok {
		return models.Order{},
			NewErrorHandler(fmt.Errorf("failed to convert order with uid %s to its struct", uid),
				http.StatusInternalServerError)
	}
	return ord, nil
}

func (o *OrderCacheRepo) GetAllOrders() ([]models.Order, error) {
	snap := o.cch.Snapshot()
	if len(snap) == 0 {
		return []models.Order{}, nil
	}

	orders := make([]models.Order, 0, len(snap))
	for uid, val := range snap {
		ord, ok := val.(models.Order)
		if !ok {
			return nil,
				NewErrorHandler(fmt.Errorf("failed to convert order with uid %s to its struct", uid),
					http.StatusInternalServerError)
		}
		orders = append(orders, ord)
	}
	return orders, nil
}

func (o *OrderCacheRepo) Delete(uid string) {
	o.cch.Delete(uid)
}
