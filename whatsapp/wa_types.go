package whatsapp

import (
	"call_center_app/config"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

// Global Trigger Channel for Feedback Result
type FeedbackTriggerData struct {
	Config            *config.YamlConfig
	Database          *gorm.DB
	WhatsappClient    *whatsmeow.Client
	StanzaID          string
	OriginalSenderJID string
	GroupWAJID        string
	RequestInWhatsapp string
	PicPhoneNumber    string
	SpkNumber         string
	WoNumber          string
}

// Global Trigger Channel for Update data in ODOO
type UpdatedODOODataTriggerItem struct {
	Config   *config.YamlConfig
	Database *gorm.DB
	TaskID   uint
}

type OdooTaskDataRequestItem struct {
	ID                  int               `json:"id"`
	MerchantName        nullAbleString    `json:"x_merchant"`
	PicMerchant         nullAbleString    `json:"x_pic_merchant"`
	PicPhone            nullAbleString    `json:"x_pic_phone"`
	MerchantAddress     nullAbleString    `json:"partner_street"`
	Description         nullAbleString    `json:"x_title_cimb"` // "description"
	SlaDeadline         nullAbleTime      `json:"x_sla_deadline"`
	CreateDate          nullAbleTime      `json:"create_date"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	PlanDate            nullAbleTime      `json:"planned_date_begin"`
	TimesheetLastStop   nullAbleTime      `json:"timesheet_timer_last_stop"`
	TaskType            nullAbleString    `json:"x_task_type"`
	WorksheetTemplateId nullAbleInterface `json:"worksheet_template_id"`
	TicketTypeId        nullAbleInterface `json:"x_ticket_type2"`
	CompanyId           nullAbleInterface `json:"company_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	HelpdeskTicketId    nullAbleInterface `json:"helpdesk_ticket_id"`
	Mid                 nullAbleString    `json:"x_cimb_master_mid"`
	Tid                 nullAbleString    `json:"x_cimb_master_tid"`
	Source              nullAbleString    `json:"x_source"`
	MessageCC           nullAbleString    `json:"x_message_call"`
	WoNumber            string            `json:"x_no_task"`
	StatusMerchant      nullAbleString    `json:"x_status_merchant"`
	SnEdc               nullAbleInterface `json:"x_studio_edc"`
	EdcType             nullAbleInterface `json:"x_product"`
	WoRemarkTiket       nullAbleString    `json:"x_wo_remark"`
	Longitude           nullAbleString    `json:"x_longitude"`
	Latitude            nullAbleString    `json:"x_latitude"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	ReasonCodeId        nullAbleInterface `json:"x_reason_code_id"`
}

type OdooTicketDataRequestItem struct {
	ID      int               `json:"id"`
	JobId   nullAbleString    `json:"x_merchant"`
	StageId nullAbleInterface `json:"stage_id"`
}

type OdooTicketSolvedPendingDataRequestItem struct {
	ID                  int               `json:"id"`
	TicketSubject       string            `json:"name"`
	MerchantName        nullAbleString    `json:"x_merchant"`
	PicMerchant         nullAbleString    `json:"x_merchant_pic"`
	PicPhone            nullAbleString    `json:"x_merchant_pic_phone"`
	MerchantAddress     nullAbleString    `json:"x_studio_alamat"`
	Description         nullAbleString    `json:"description"`
	SlaDeadline         nullAbleTime      `json:"sla_deadline"`
	CreateDate          nullAbleTime      `json:"create_date"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	TimesheetLastStop   nullAbleTime      `json:"complete_datetime_wo"`
	TaskType            nullAbleString    `json:"x_task_type"`
	WorksheetTemplateId nullAbleInterface `json:"x_worksheet_template_id"`
	TicketTypeId        nullAbleInterface `json:"ticket_type_id"`
	CompanyId           nullAbleInterface `json:"company_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	Mid                 nullAbleString    `json:"x_master_mid"`
	Tid                 nullAbleString    `json:"x_master_tid"`
	Source              nullAbleString    `json:"x_source"`
	JobId               nullAbleString    `json:"x_job_id"`
	WoNumber            string            `json:"x_wo_number_last"`
	StatusMerchant      nullAbleString    `json:"x_status_merchant"`
	SnEdc               nullAbleInterface `json:"x_merchant_sn_edc"`
	EdcType             nullAbleInterface `json:"x_merchant_tipe_edc"`
	WoRemarkTiket       nullAbleString    `json:"x_wo_remark"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	ReasonCode          nullAbleString    `json:"x_reasoncode"`
	TaskId              nullAbleInterface `json:"fsm_task_ids"`
	Latitude            nullAbleFloat     `json:"x_partner_latitude"`
	Longitude           nullAbleFloat     `json:"x_partner_longitude"`
}

type WhatsmeowHandler struct {
	Client     *whatsmeow.Client
	YamlCfg    *config.YamlConfig
	GroupJID   types.JID
	GroupCCJID types.JID
	GroupTAJID types.JID
	Database   *gorm.DB
}

type SchedulerConfig struct {
	Times    []string
	Function func()
	Name     string
}

type EmailAttachment struct {
	FilePath    string
	NewFileName string
}

type odooResPartnerDataItem struct {
	ID                  int               `json:"id"`
	Name                string            `json:"name"`
	Merchant            nullAbleString    `json:"x_merchant"`
	MerchantCode        nullAbleString    `json:"x_merchant_code"`
	MerchantGroupCode   nullAbleString    `json:"x_merchant_group_code"`
	MerchantGroupName   nullAbleString    `json:"x_merchant_group_name"`
	AlamatPengirimanEDC nullAbleString    `json:"x_alamat_pengiriman_edc"`
	ContactPerson       nullAbleString    `json:"x_contact_person"`
	ContactPhone        nullAbleString    `json:"phone"`
	ContactMobile       nullAbleString    `json:"mobile"`
	AlamatPerusahaan    nullAbleString    `json:"contact_address"`
	PicMerchant         nullAbleString    `json:"x_merchant_pic"`
	PicPhone            nullAbleString    `json:"x_merchant_pic_phone"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	SnEdcId             nullAbleInterface `json:"x_studio_sn_edc"`
	TipeEdcId           nullAbleInterface `json:"x_product"`
	SimCardId           nullAbleInterface `json:"x_simcard"`
	SimCardProviderId   nullAbleInterface `json:"x_simcard_provider"`
	MsisdnSimcard       nullAbleString    `json:"x_msisdn_sim_card"`
	Iccid               nullAbleString    `json:"iccid_simcard"`
	Mid                 nullAbleString    `json:"x_cimb_mid"`
	Tid                 nullAbleString    `json:"x_cimb_tid"`
	Longitude           nullAbleFloat     `json:"partner_latitude"`
	Latitude            nullAbleFloat     `json:"partner_longitude"`
	ServicePoint        nullAbleString    `json:"x_service_point"`
	MerchantLastStatus  nullAbleString    `json:"merchant_last_status"`
	TaskIds             nullAbleInterface `json:"task_ids"`
	TicketIds           nullAbleInterface `json:"x_ticket_ids"`
}

type helpdeskTicketPicPhoneDataItem struct {
	ID       int            `json:"id"`
	PicPhone nullAbleString `json:"x_merchant_pic_phone"`
}

type projectTaskPicPhoneDataItem struct {
	ID       int            `json:"id"`
	PicPhone nullAbleString `json:"x_pic_phone"`
}
