package models

import (
	"time"
)

type JOMerchantHmin1CallLog struct {
	ID              uint       `gorm:"type:int;column:id" json:"id"`
	IsReschedule    bool       `gorm:"type:bool;column:is_reschedule;default:NULL" json:"is_reschedule"`
	Reschedule      *time.Time `gorm:"type:datetime;column:reschedule" json:"reschedule"`
	WoNumber        string     `gorm:"type:varchar(100);column:wonumber;not null" json:"wonumber"`
	ImgWa           *string    `gorm:"type:longtext;column:img_wa;default:NULL" json:"img_wa"`
	ImgMerchant     *string    `gorm:"type:longtext;column:img_merchant;default:NULL" json:"img_merchant"`
	ImgSnEdc        *string    `gorm:"type:longtext;column:img_sn_edc;default:NULL" json:"img_sn_edc"`
	CsNotes         string     `gorm:"type:text;column:cs_notes" json:"cs_notes"`
	JoStatus        string     `gorm:"type:varchar(250);column:jo_status;default:NULL" json:"jo_status"`
	UpdateToOdoo    string     `gorm:"type:varchar(250);column:update_to_odoo;default:NULL" json:"update_to_odoo"`
	UpdateToMti     string     `gorm:"type:varchar(250);column:update_to_mti;default:NULL" json:"update_to_mti"`
	Pic             string     `gorm:"type:varchar(100);column:pic;" json:"pic"`
	PicPhone        string     `gorm:"type:varchar(50);column:pic_phone;" json:"pic_phone"`
	IdCS            int        `gorm:"type:int;column:id_cs" json:"id_cs"`
	CreatedAt       *time.Time `gorm:"type:datetime;column:created_at" json:"created_at"`
	UpdatedAt       *time.Time `gorm:"type:datetime;column:updated_at" json:"updated_at"`
	TaskType        *string    `gorm:"type:varchar(100);column:task_type;default:NULL" json:"task_type"`
	JobID           *string    `gorm:"type:text;column:x_job_id;default:NULL" json:"x_job_id"`
	ImgWaPath       *string    `gorm:"type:varchar(1000);column:img_wa_path;default:NULL" json:"img_wa_path"`
	ImgMerchantPath *string    `gorm:"type:varchar(1000);column:img_merchant_path;default:NULL" json:"img_merchant_path"`
	ImgSnEdcPath    *string    `gorm:"type:varchar(1000);column:img_sn_edc_path;default:NULL" json:"img_sn_edc_path"`
}
