package postgres

import (
	"l0-demo/internal/models"

	"github.com/jinzhu/gorm"
	"github.com/sirupsen/logrus"
)

type OrderPostgresRepo struct {
	db *gorm.DB
}

func NewOrderPostgres(db *gorm.DB) *OrderPostgresRepo {
	return &OrderPostgresRepo{db: db}
}

func (o *OrderPostgresRepo) Create(ord models.Order) error {
	err := o.db.Transaction(func(tx *gorm.DB) error {
		if err := o.db.Create(&ord).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logrus.Print(err)
		return err
	}
	return nil
}

func (o *OrderPostgresRepo) GetAll() ([]models.Order, error) {
	var orders []models.Order
	err := o.db.Transaction(func(tx *gorm.DB) error {
		if err := o.db.
			Model(&models.Order{}).
			Preload("Delivery").
			Preload("Payment").
			Preload("Items").
			Find(&orders).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logrus.Fatal(err)
		return nil, err
	}
	return orders, nil
}

func (o *OrderPostgresRepo) Get(uid string) (models.Order, error) {
	var order models.Order
	err := o.db.Transaction(func(tx *gorm.DB) error {
		if err := o.db.
			Model(&models.Order{}).
			Preload("Delivery").
			Preload("Payment").
			Preload("Items").
			Where("order_uid = ?", uid).
			First(&order).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logrus.Error(err)
		return models.Order{}, err
	}
	return order, nil
}

func (o *OrderPostgresRepo) CreateOrUpdate(ord models.Order) error {
	// гарантируем FK в дочках
	ord.Delivery.OrderRefer = ord.OrderUid
	ord.Payment.OrderRefer  = ord.OrderUid
	for i := range ord.Items {
		ord.Items[i].OrderRefer = ord.OrderUid
	}

	return o.db.Transaction(func(tx *gorm.DB) error {
		// есть ли такой заказ?
		var existing models.Order
		err := tx.Where("order_uid = ?", ord.OrderUid).First(&existing).Error
		if gorm.IsRecordNotFoundError(err) {
			// не найден — обычная вставка со связями
			return tx.Create(&ord).Error
		}
		if err != nil {
			return err
		}

		// найден — обновим корневую запись
		if err := tx.Model(&models.Order{}).
			Where("order_uid = ?", ord.OrderUid).
			Updates(map[string]interface{}{
				"track_number":       ord.TrackNumber,
				"entry":              ord.Entry,
				"locale":             ord.Locale,
				"internal_signature": ord.InternalSignature,
				"customer_id":        ord.CustomerId,
				"delivery_service":   ord.DeliveryService,
				"shard_key":           ord.ShardKey,
				"sm_id":              ord.SmId,
				"date_created":       ord.DateCreated,
				"oof_shard":          ord.OofShard,
			}).Error; err != nil {
			return err
		}

		// 1:1 — обновления
		if err := tx.Model(&models.Delivery{}).
			Where("order_refer = ?", ord.OrderUid).
			Updates(ord.Delivery).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Payment{}).
			Where("order_refer = ?", ord.OrderUid).
			Updates(ord.Payment).Error; err != nil {
			return err
		}

		// 1:N items — заменяем список: удаляем старые → вставляем новые
		if err := tx.Where("order_refer = ?", ord.OrderUid).Delete(&models.Item{}).Error; err != nil {
			return err
		}
		if len(ord.Items) > 0 {
			if err := tx.Create(&ord.Items).Error; err != nil {
				return err
			}
		}
		return nil
	})
}