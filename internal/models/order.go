package models

import (
	"time"
)

type Order struct {
	OrderUid          string    `json:"order_uid"        validate:"required,len=19" gorm:"primary_key;unique"`
	TrackNumber       string    `json:"track_number"     validate:"required,len=14"`
	Entry             string    `json:"entry"            validate:"required,len=4"`
	Locale            string    `json:"locale"           validate:"oneof=ru en"`
	InternalSignature string    `json:"internal_signature"`
	CustomerId        string    `json:"customer_id"      validate:"required,len=4"`
	DeliveryService   string    `json:"delivery_service" validate:"required,len=5"`
	ShardKey          string    `json:"shardkey"`
	SmId              int       `json:"sm_id"            validate:"gte=0,lte=100"`
	DateCreated       time.Time `json:"date_created"     validate:"required"`
	OofShard          string    `json:"oof_shard"        validate:"required,max=2"`
	Delivery          *Delivery `json:"delivery"         validate:"required" gorm:"foreignkey:OrderRefer;association_foreignkey:OrderUid"`
	Payment           *Payment  `json:"payment"          validate:"required" gorm:"foreignkey:OrderRefer;association_foreignkey:OrderUid"`
	Items             []Item    `json:"items"            validate:"required,min=1,dive" gorm:"foreignkey:OrderRefer;association_foreignkey:OrderUid"`
}
