package postgres

import (
	"l0-demo/internal/models"

	"github.com/jinzhu/gorm"
)

type OrderPostgresRepo struct {
	db *gorm.DB
}

func NewOrderPostgres(db *gorm.DB) *OrderPostgresRepo {
	return &OrderPostgresRepo{db: db}
}

func (r *OrderPostgresRepo) Create(o models.Order) error {
	if o.Delivery != nil {
		o.Delivery.OrderRefer = o.OrderUid
	}
	if o.Payment != nil {
		o.Payment.OrderRefer = o.OrderUid
	}
	for i := range o.Items {
		o.Items[i].OrderRefer = o.OrderUid
	}

	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&o).Error
	})
}

func (r *OrderPostgresRepo) CreateOrUpdate(o models.Order) error {
	if o.Delivery != nil {
		o.Delivery.OrderRefer = o.OrderUid
	}
	if o.Payment != nil {
		o.Payment.OrderRefer = o.OrderUid
	}
	for i := range o.Items {
		o.Items[i].OrderRefer = o.OrderUid
	}

	return r.db.
		Set("gorm:association_autocreate", false).
		Set("gorm:association_autoupdate", false).
		Transaction(func(tx *gorm.DB) error {

			var count int
			if err := tx.Model(&models.Order{}).
				Where("order_uid = ?", o.OrderUid).
				Count(&count).Error; err != nil {
				return err
			}

			if count == 0 {
				hdr := models.Order{
					OrderUid:          o.OrderUid,
					TrackNumber:       o.TrackNumber,
					Entry:             o.Entry,
					Locale:            o.Locale,
					InternalSignature: o.InternalSignature,
					CustomerId:        o.CustomerId,
					DeliveryService:   o.DeliveryService,
					ShardKey:          o.ShardKey,
					SmId:              o.SmId,
					DateCreated:       o.DateCreated,
					OofShard:          o.OofShard,
				}
				if err := tx.Create(&hdr).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&models.Order{}).
					Where("order_uid = ?", o.OrderUid).
					Updates(map[string]interface{}{
						"track_number":       o.TrackNumber,
						"entry":              o.Entry,
						"locale":             o.Locale,
						"internal_signature": o.InternalSignature,
						"customer_id":        o.CustomerId,
						"delivery_service":   o.DeliveryService,
						"shard_key":          o.ShardKey,
						"sm_id":              o.SmId,
						"date_created":       o.DateCreated,
						"oof_shard":          o.OofShard,
					}).Error; err != nil {
					return err
				}
			}

			if o.Delivery != nil {
				var d models.Delivery
				err := tx.Where("order_refer = ?", o.OrderUid).First(&d).Error
				switch {
				case gorm.IsRecordNotFoundError(err):
					if err := tx.Model(&models.Delivery{}).Create(o.Delivery).Error; err != nil {
						return err
					}
				case err != nil:
					return err
				default:
					if err := tx.Model(&d).Updates(o.Delivery).Error; err != nil {
						return err
					}
				}
			}

			if o.Payment != nil {
				var p models.Payment
				err := tx.Where("order_refer = ?", o.OrderUid).First(&p).Error
				switch {
				case gorm.IsRecordNotFoundError(err):
					if err := tx.Model(&models.Payment{}).Create(o.Payment).Error; err != nil {
						return err
					}
				case err != nil:
					return err
				default:
					if err := tx.Model(&p).Updates(o.Payment).Error; err != nil {
						return err
					}
				}
			}

			if err := tx.Where("order_refer = ?", o.OrderUid).Delete(models.Item{}).Error; err != nil {
				return err
			}
			if len(o.Items) > 0 {
				for i := range o.Items {
					if err := tx.Model(&models.Item{}).Create(&o.Items[i]).Error; err != nil {
						return err
					}
				}
			}

			return nil
		})
}

func (r *OrderPostgresRepo) Get(uid string) (models.Order, error) {
	var o models.Order
	q := r.db.Preload("Delivery").
		Preload("Payment").
		Preload("Items").
		Where("order_uid = ?", uid).
		First(&o)
	return o, q.Error
}

func (r *OrderPostgresRepo) GetAll() ([]models.Order, error) {
	var out []models.Order
	q := r.db.Preload("Delivery").
		Preload("Payment").
		Preload("Items").
		Find(&out)
	return out, q.Error
}
