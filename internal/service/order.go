package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"l0-demo/internal/models"

	"github.com/go-playground/validator/v10"
	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

func humanizeValidationErrors(errs validator.ValidationErrors) string {
	var b strings.Builder
	for _, fe := range errs {
		if fe.Param() != "" {
			fmt.Fprintf(&b, "%s: %s=%s; ", fe.Namespace(), fe.Tag(), fe.Param())
		} else {
			fmt.Fprintf(&b, "%s: %s; ", fe.Namespace(), fe.Tag())
		}
	}
	s := b.String()
	if len(s) > 2 {
		s = s[:len(s)-2]
	}
	return s
}

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
	for _, o := range orders {
		if err := s.v.Struct(o); err != nil {
			logrus.WithError(err).WithField("uid", o.OrderUid).Warn("skip invalid order from DB")
			continue
		}
		s.PutCachedOrder(o)
	}
	return nil
}

func (s *Service) PutCachedOrder(order models.Order) {
	s.OrderCache.PutOrder(order.OrderUid, order)
}

func (s *Service) PutDbOrder(order models.Order) error {
	if err := s.v.Struct(order); err != nil {
		if verrs, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("validation failed: %s", humanizeValidationErrors(verrs))
		}
		return fmt.Errorf("validation error: %w", err)
	}
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
