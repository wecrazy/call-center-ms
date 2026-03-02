package models

import (
	"call_center_app/config"

	"gorm.io/gorm"
)

type BasedOnWoNumber struct {
	gorm.Model
	WoNumber string `gorm:"type:varchar(500);column:wo_number" json:"wo_number"`
}

func (BasedOnWoNumber) TableName() string {
	return config.GetConfig().Db.TbBasedOnWoNumber
}
