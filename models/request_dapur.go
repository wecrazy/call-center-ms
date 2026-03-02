package models

import "gorm.io/gorm"

type RequestDapur struct {
	gorm.Model
	AdminCS         int     `gorm:"type:int;column:admin_cs;not null" json:"admin_cs"`
	Merchant        string  `gorm:"type:text;column:merchant;not null" json:"merchant"`
	MerchantAddress *string `gorm:"type:text;column:merchant_address;default:NULL" json:"merchant_address"`
	Pic             string  `gorm:"type:varchar(100);column:pic;not null" json:"pic"`
	PicPhone        string  `gorm:"type:varchar(50);column:pic_phone;not null" json:"pic_phone"`
	Mid             string  `gorm:"type:varchar(100);column:mid;not null" json:"mid"`
	Tid             string  `gorm:"type:varchar(100);column:tid;not null" json:"tid"`
	JobOrder        string  `gorm:"type:varchar(100);column:job_order;not null" json:"job_order"`
	OrderDetail     *string `gorm:"type:text;column:order_detail;default:NULL" json:"order_detail"`
	Counter         int     `gorm:"type:int;column:counter" json:"counter"`
	OnCalling       bool    `gorm:"type:bool;column:on_calling;not null" json:"on_calling"`
	IsDone          bool    `gorm:"type:bool;column:is_done;not null" json:"is_done"`
	WebUrl          *string `gorm:"type:varchar(500);column:web_url;default:NULL" json:"web_url"`
}
