package models

type Item struct {
	OrderRefer  string `json:"-" gorm:"type:varchar(19);index"`
	ChrtId      int    `json:"chrt_id"      validate:"required"`
	TrackNumber string `json:"track_number" validate:"required,len=14"`
	Price       int    `json:"price"        validate:"gt=0"`
	Rid         string `json:"rid"          validate:"required,len=21"`
	Name        string `json:"name"         validate:"required"`
	Sale        int    `json:"sale"         validate:"gt=0"`
	Size        string `json:"size"         validate:"required"`
	TotalPrice  int    `json:"total_price"  validate:"gt=0"`
	NmId        int    `json:"nm_id"        validate:"required"`
	Brand       string `json:"brand"        validate:"required"`
	Status      int    `json:"status"       validate:"gte=0,lte=999"`
}
