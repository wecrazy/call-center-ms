package models

import (
	"call_center_app/config"

	"gorm.io/gorm"
)

type UnsolvedTicketData struct {
	gorm.Model
	Subject string `gorm:"type:varchar(500);column:subject" json:"subject"`
}

func (UnsolvedTicketData) TableName() string {
	return config.GetConfig().Db.TbUnsolvedTicket
}
