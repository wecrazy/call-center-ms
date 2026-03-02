package models

import (
	"time"

	"gorm.io/gorm"
)

type JOMerchantHmin1 struct {
	gorm.Model
	ID              uint       `gorm:"type:int;column:id" json:"id"`
	Counter         int        `gorm:"type:int;column:counter" json:"counter"`
	PlannedDate     *time.Time `gorm:"type:datetime;column:planned_date" json:"planned_date"`
	Reschedule      string     `gorm:"type:varchar(255);column:reschedule;default:NULL" json:"reschedule"`
	SlaDeadline     *time.Time `gorm:"type:datetime;column:sla_deadline" json:"sla_deadline"`
	WoNumber        string     `gorm:"type:varchar(100);column:wonumber;not null" json:"wonumber"`
	Stage           string     `gorm:"type:varchar(50);column:stage;not null" json:"stage"`
	TicketID        int        `gorm:"type:int;column:ticket_id;not null" json:"ticket_id"`
	TicketNumber    string     `gorm:"type:varchar(100);column:ticket_number;not null" json:"ticket_number"`
	TaskType        string     `gorm:"type:varchar(100);column:task_type;not null" json:"task_type"`
	Merchant        string     `gorm:"type:text;column:merchant;not null" json:"merchant"`
	MerchantAddress string     `gorm:"type:varchar(255);column:merchant_address;default:NULL" json:"merchant_address"`
	Pic             string     `gorm:"type:varchar(100);column:pic;not null" json:"pic"`
	PicInContact    string     `gorm:"type:text;column:pic_in_contact" json:"pic_in_contact"`
	PicPhone        string     `gorm:"type:varchar(50);column:pic_phone;not null" json:"pic_phone"`
	Description     *string    `gorm:"type:text;column:description;default:NULL" json:"description"`
	Technician      string     `gorm:"type:varchar(100);column:technician;not null" json:"technician"`
	WebUrl          string     `gorm:"type:varchar(500);column:web_url" json:"web_url"`
	OnCalling       bool       `gorm:"type:bool;column:on_calling;not null" json:"on_calling"`
	IsDone          bool       `gorm:"type:bool;column:is_done;not null" json:"is_done"`
	TempCS          int        `gorm:"type:int;column:temp_cs" json:"temp_cs"`
	Mid             string     `gorm:"type:text;column:mid" json:"mid"`
	Tid             string     `gorm:"type:text;column:tid" json:"tid"`
	SnEdc           string     `gorm:"type:text;column:sn_edc" json:"sn_edc"`
	XSource         string     `gorm:"type:text;column:x_source" json:"x_source"`
	JobID           *string    `gorm:"type:text;column:x_job_id;default:NULL" json:"x_job_id"`
	// CsNotes         string    `gorm:"type:text;column:cs_notes" json:"cs_notes"`
	// ImgWa           string    `gorm:"type:varchar(255);column:img_wa" json:"img_wa"`
	// ImgMerchant     string    `gorm:"type:varchar(255);column:img_merchant" json:"img_merchant"`
	// ImgSnEdc        string    `gorm:"type:varchar(255);column:img_sn_edc" json:"img_sn_edc"`
	// JoStatus        string    `gorm:"type:varchar(50);column:jo_status" json:"jo_status"`
	// UpdateToOdoo    string    `gorm:"type:varchar(50);column:update_to_odoo" json:"update_to_odoo"`
	// UpdateToMti     string    `gorm:"type:varchar(50);column:update_to_mti" json:"update_to_mti"`
	// UrlForCS        string    `gorm:"type:text;column:url_for_cs" json:"url_for_cs"`
}
