package models

import (
	"call_center_app/config"

	"gorm.io/gorm"
)

type WaMention struct {
	gorm.Model
	ContactName  string `gorm:"type:varchar(300);column:contact_name" json:"contact_name"`
	ContactPhone string `gorm:"type:varchar(30);column:contact_phone" json:"contact_phone"`
	Category     string `gorm:"type:varchar(300);column:category" json:"category"`
	Keterangan   string `gorm:"type:text;column:keterangan" json:"keterangan"`
}

func (WaMention) TableName() string {
	return config.GetConfig().Db.TbWaMention
}
