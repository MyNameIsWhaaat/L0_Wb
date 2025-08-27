package service

import (
	"context"
	"encoding/json"
	"l0-demo/internal/models"
	"time"

	"github.com/jinzhu/gorm"
)

func (s *Service) GetCachedOrder(uid string) (models.Order, error) {
	return s.OrderCache.GetOrder(uid)
}

func (s *Service) GetAllCachedOrders() ([]models.Order, error) {
	return s.OrderCache.GetAllOrders()
}

func (s *Service) GetAllDbOrders() ([]models.Order, error) {
	return s.OrderPostgres.GetAll()
}

func (s *Service) PutOrdersFromDbToCache() error {
	orders, err := s.GetAllDbOrders()
	if err != nil {
		return err
	}
	for i := 0; i < len(orders); i++ {
		s.PutCachedOrder(orders[i])
	}
	return nil
}

func (s *Service) PutCachedOrder(order models.Order) {
	s.OrderCache.PutOrder(order.OrderUid, order)
}

func (s *Service) PutDbOrder(order models.Order) error {
	return s.OrderPostgres.Create(order)
}

func (s *Service) GetDbOrder(uid string) (order models.Order, err error) {
	ord, err := s.OrderPostgres.Get(uid)
	if gorm.IsRecordNotFoundError(err) {
		return models.Order{}, ErrNotFound
	}
	return ord, err
}

func (s *Service) HandleMessage(ctx context.Context, payload []byte) error {
	var ord models.Order
	if err := json.Unmarshal(payload, &ord); err != nil {
		return err
	}
	if ord.DateCreated.IsZero() {
		ord.DateCreated = time.Now().UTC()
	}
	if err := s.OrderPostgres.CreateOrUpdate(ord); err != nil {
		return err
	}
	s.OrderCache.PutOrder(ord.OrderUid, ord)
	return nil
}