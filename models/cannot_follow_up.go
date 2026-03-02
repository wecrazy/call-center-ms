package models

import (
	"call_center_app/config"

	"gorm.io/gorm"
)

type CannotFollowUp struct {
	gorm.Model
	RequestType       string `gorm:"type:text;column:request_type" json:"request_type"`
	WoNumber          string `gorm:"type:text;column:wo_number" json:"wo_number"`
	TicketSubject     string `gorm:"type:text;column:ticket_subject" json:"ticket_subject"`
	Mid               string `gorm:"type:text;column:mid" json:"mid"`
	Tid               string `gorm:"type:text;column:tid" json:"tid"`
	TaskType          string `gorm:"type:text;column:task_type" json:"task_type"`
	WorksheetTemplate string `gorm:"type:text;column:worksheet_template" json:"worksheet_template"`
	TicketType        string `gorm:"type:text;column:ticket_type" json:"ticket_type"`
	Message           string `gorm:"type:text;column:message" json:"message"`
}

func (CannotFollowUp) TableName() string {
	return config.GetConfig().Db.TbCannotFollowUp
}
