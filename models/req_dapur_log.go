package models

import "gorm.io/gorm"

type RequestDapurLog struct {
	gorm.Model
	ID          uint    `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	AdminCS     int     `gorm:"type:int;column:admin_cs;not null" json:"admin_cs"`
	Merchant    string  `gorm:"type:text;column:merchant;not null" json:"merchant"`
	Pic         string  `gorm:"type:varchar(100);column:pic;not null" json:"pic"`
	PicPhone    string  `gorm:"type:varchar(50);column:pic_phone;not null" json:"pic_phone"`
	Mid         string  `gorm:"type:varchar(100);column:mid;not null" json:"mid"`
	Tid         string  `gorm:"type:varchar(100);column:tid;not null" json:"tid"`
	JobOrder    string  `gorm:"type:varchar(100);column:job_order;not null" json:"job_order"`
	OrderDetail *string `gorm:"type:text;column:order_detail;default:NULL" json:"order_detail"`
	// Log
	KondisiEdc            string  `gorm:"type:text;column:kondisi_edc;not null" json:"kondisi_edc"`
	AdaptorEdc            string  `gorm:"type:text;column:adaptor_edc;not null" json:"adaptor_edc"`
	KondisiAdaptorEdc     string  `gorm:"type:text;column:kondisi_adaptor_edc;not null" json:"kondisi_adaptor_edc"`
	SinyalEdc             string  `gorm:"type:text;column:sinyal_edc;not null" json:"sinyal_edc"`
	BarSinyalEdc          int     `gorm:"type:int;column:bar_sinyal_edc;not null" json:"bar_sinyal_edc"`
	ReqTambahStockThermal bool    `gorm:"type:bool;column:req_tambah_stock_thermal;not null" json:"req_tambah_stock_thermal"`
	AddStockThermal       int     `gorm:"type:int;column:add_stock_thermal;not null" json:"add_stock_thermal"`
	AdditionalNotes       *string `gorm:"type:text;column:additional_notes;default:NULL" json:"additional_notes"`
}
