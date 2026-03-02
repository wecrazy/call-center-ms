package models

import "gorm.io/gorm"

type OdooReasonCode struct {
	gorm.Model
	Name       string `gorm:"type:varchar(255);column:x_name;not null" json:"x_name"`
	ReasonCode string `gorm:"type:varchar(50);column:x_reason_code;not null" json:"x_reason_code"`
	// Dependency  string `gorm:"type:varchar(100);column:x_dependency;not null" json:"x_dependency"`
	Pending     bool   `gorm:"column:x_pending" json:"x_pending"`
	CompanyID   int    `gorm:"column:company_id;not null" json:"company_id"`
	CompanyName string `gorm:"type:varchar(255);column:company_name;not null" json:"company_name"`
}
