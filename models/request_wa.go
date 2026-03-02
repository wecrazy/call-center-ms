package models

import (
	"call_center_app/config"
	"time"

	"gorm.io/gorm"
)

type WaRequest struct {
	gorm.Model
	ID                  uint       `gorm:"primarykey"`
	Counter             int        `gorm:"type:int;column:counter" json:"counter"`
	RequestType         string     `gorm:"type:varchar(255);column:request_type" json:"request_type"`
	MerchantName        string     `gorm:"type:varchar(255);column:x_merchant" json:"x_merchant"`
	PicMerchant         string     `gorm:"type:varchar(255);column:x_pic_merchant" json:"x_pic_merchant"`
	PicPhone            string     `gorm:"type:varchar(50);column:x_pic_phone" json:"x_pic_phone"`
	MerchantAddress     string     `gorm:"type:text;column:partner_street" json:"partner_street"`
	Description         string     `gorm:"type:text;column:description" json:"description"`
	SlaDeadline         *time.Time `gorm:"column:x_sla_deadline" json:"x_sla_deadline"`
	CreateDate          *time.Time `gorm:"column:create_date" json:"create_date"`
	ReceivedDatetimeSpk *time.Time `gorm:"column:x_received_datetime_spk" json:"x_received_datetime_spk"`
	PlanDate            *time.Time `gorm:"column:planned_date_begin" json:"planned_date_begin"`
	TaskType            string     `gorm:"type:varchar(100);column:x_task_type" json:"x_task_type"`
	CompanyId           int        `gorm:"column:company_id" json:"company_id"`
	CompanyName         string     `gorm:"type:varchar(100);column:company_name" json:"company_name"`
	StageId             int        `gorm:"column:stage_id" json:"stage_id"`
	StageName           string     `gorm:"type:varchar(100);column:stage_name" json:"stage_name"`
	HelpdeskTicketId    int        `gorm:"column:helpdesk_ticket_id" json:"helpdesk_ticket_id"`
	HelpdeskTicketName  string     `gorm:"type:varchar(100);column:helpdesk_ticket_name" json:"helpdesk_ticket_name"`
	Mid                 string     `gorm:"type:varchar(100);column:x_cimb_master_mid" json:"x_cimb_master_mid"`
	Tid                 string     `gorm:"type:varchar(100);column:x_cimb_master_tid" json:"x_cimb_master_tid"`
	Source              string     `gorm:"type:varchar(300);column:x_source" json:"x_source"`
	MessageCC           string     `gorm:"type:text;column:x_message_call" json:"x_message_call"`
	WoNumber            string     `gorm:"type:varchar(50);column:x_no_task" json:"x_no_task"`
	StatusMerchant      string     `gorm:"type:varchar(100);column:x_status_merchant" json:"x_status_merchant"`
	SnEdcId             int        `gorm:"column:x_studio_edc_id" json:"x_studio_edc_id"`
	SnEdc               string     `gorm:"type:varchar(100);column:x_studio_edc" json:"x_studio_edc"`
	EdcTypeId           int        `gorm:"column:x_product_id" json:"x_product_id"`
	EdcType             string     `gorm:"type:varchar(100);column:x_product" json:"x_product"`
	WoRemarkTiket       string     `gorm:"type:text;column:x_wo_remark" json:"x_wo_remark"`
	Longitude           string     `gorm:"type:varchar(50);column:x_longitude" json:"x_longitude"`
	Latitude            string     `gorm:"type:varchar(50);column:x_latitude" json:"x_latitude"`
	TechnicianId        int        `gorm:"column:technician_id" json:"technician_id"`
	TechnicianName      string     `gorm:"type:varchar(100);column:technician_name" json:"technician_name"`
	ReasonCodeId        int        `gorm:"column:reason_code_id" json:"reason_code_id"`
	ReasonCodeName      string     `gorm:"type:varchar(200);column:reason_code_name" json:"reason_code_name"`
	IsOnCalling         bool       `gorm:"type:bool;column:is_on_calling;not null;default:false" json:"is_on_calling"`
	IsDone              bool       `gorm:"type:bool;column:is_done;not null;default:false" json:"is_done"`
	TempCS              int        `gorm:"type:int;column:temp_cs" json:"temp_cs"`
	JobId               string     `gorm:"type:varchar(200);column:x_job_id" json:"x_job_id"`
	// Whatsapp Info
	GroupWaJid        string `gorm:"type:varchar(255);column:group_wa_jid" json:"group_wa_jid"`
	StanzaId          string `gorm:"type:varchar(255);column:stanza_id" json:"stanza_id"`
	OriginalSenderJid string `gorm:"type:varchar(255);column:original_sender_jid" json:"original_sender_jid"`
	// Form input fields
	IsReschedule       bool       `gorm:"type:bool;column:is_reschedule;not null;default:false" json:"is_reschedule"`
	OrderWish          string     `gorm:"type:varchar(255);column:order_wish" json:"order_wish"`
	TargetScheduleDate *time.Time `gorm:"column:target_schedule_date" json:"target_schedule_date"`
	UpdatedToOdoo      bool       `gorm:"type:bool;column:updated_to_odoo;not null;default:false" json:"updated_to_odoo"`
	CallCenterMessage  string     `gorm:"type:text;column:call_center_message" json:"call_center_message"`
	ImgWaPath          string     `gorm:"type:text;column:img_wa_path" json:"img_wa_path"`
	ImgSnEdcPath       string     `gorm:"type:text;column:img_sn_edc_path" json:"img_sn_edc_path"`
	ImgMerchantPath    string     `gorm:"type:text;column:img_merchant_path" json:"img_merchant_path"`
	Keterangan         string     `gorm:"type:text;column:keterangan" json:"keterangan"`
	// Additional fields on v3
	NextFollowUpTo      string     `gorm:"type:text;column:next_follow_up_to" json:"next_follow_up_to"`
	IsFinal             bool       `gorm:"type:bool;column:is_final;not null;default:false" json:"is_final"`
	LastUpdateBy        string     `gorm:"type:varchar(100);column:last_update_by" json:"last_update_by"`
	RequestToCC         string     `gorm:"type:text;column:request_to_cc" json:"request_to_cc"`
	TimesheetLastStop   *time.Time `gorm:"column:timesheet_last_stop" json:"timesheet_last_stop"`
	TicketStageId       int        `gorm:"column:ticket_stage_id" json:"ticket_stage_id"`
	TicketStageName     string     `gorm:"type:varchar(100);column:ticket_stage_name" json:"ticket_stage_name"`
	IsOnCallingDatetime *time.Time `gorm:"column:is_on_calling_datetime" json:"is_on_calling_datetime"`
	IsDoneDatetime      *time.Time `gorm:"column:is_done_datetime" json:"is_done_datetime"`

	// All teams
	MarkDoneByOperational   bool   `gorm:"type:bool;column:mark_done_by_operational;not null;default:false" json:"mark_done_by_operational"`
	RemarkByOperational     string `gorm:"type:text;column:remark_by_operational" json:"remark_by_operational"`
	AttachmentByOperational string `gorm:"type:text;column:attachment_by_operational" json:"attachment_by_operational"`

	MarkDoneByInventory   bool   `gorm:"type:bool;column:mark_done_by_inventory;not null;default:false" json:"mark_done_by_inventory"`
	RemarkByInventory     string `gorm:"type:text;column:remark_by_inventory" json:"remark_by_inventory"`
	AttachmentByInventory string `gorm:"type:text;column:attachment_by_inventory" json:"attachment_by_inventory"`

	MarkDoneByPmo   bool   `gorm:"type:bool;column:mark_done_by_pmo;not null;default:false" json:"mark_done_by_pmo"`
	RemarkByPmo     string `gorm:"type:text;column:remark_by_pmo" json:"remark_by_pmo"`
	AttachmentByPmo string `gorm:"type:text;column:attachment_by_pmo" json:"attachment_by_pmo"`

	MarkDoneByMonitoring   bool   `gorm:"type:bool;column:mark_done_by_monitoring;not null;default:false" json:"mark_done_by_monitoring"`
	RemarkByMonitoring     string `gorm:"type:text;column:remark_by_monitoring" json:"remark_by_monitoring"`
	AttachmentByMonitoring string `gorm:"type:text;column:attachment_by_monitoring" json:"attachment_by_monitoring"`
}

func (WaRequest) TableName() string {
	return config.GetConfig().Db.TbWaReq
}
