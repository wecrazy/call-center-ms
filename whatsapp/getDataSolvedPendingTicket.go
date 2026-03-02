package whatsapp

import (
	"call_center_app/models"
	"call_center_app/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

var getDataSolvedPendingMutex sync.Mutex

func (h *WhatsmeowHandler) GetDataSolvedPendingforFU() {
	if !getDataSolvedPendingMutex.TryLock() {
		log.Println("GetDataSolvedPendingforFU is already running, skipping execution.")
		return
	}
	defer getDataSolvedPendingMutex.Unlock()

	jidString := h.YamlCfg.Whatsmeow.GroupCCJID + "@g.us"
	jid, err := types.ParseJID(jidString)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidString)
		return
	}

	jidStringInvent := h.YamlCfg.Whatsmeow.GroupInventoryOprsRMMetlandJID + "@g.us"
	jidInvent, err := types.ParseJID(jidStringInvent)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidStringInvent)
		return
	}

	log.Printf("Running scheduler Get Data Solved Pending Ticket for Followed Up by CC @%v", time.Now())

	// Running scheduler get data Ticket Solved Pending

	orderWish := "Re-Confirm"
	requestType := "Ticket Solved Pending Follow Up"
	lastUpdateBy := "System"
	requestToCC := "Konfirmasi ke PIC merchant terkait perangkat EDC apakah masih di lokasi merchant / tidak. Serta konfirmasi terkait Remark yang ada"

	odooModel := "helpdesk.ticket"
	odooOrder := "id desc"
	companyAllowed := h.YamlCfg.ApiODOO.CompanyAllowed
	stageIncluded := []string{
		"Solved Pending",
	}
	technicianExcluded := []int{
		3046, // "Teknisi Pameran",
		5,    // "Tes Dev Mfjr",
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		fmt.Println("Error loading timezone:", err)
		return
	}
	now := time.Now().In(loc)
	currentYear, currentMonth, _ := now.Date()

	start := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, loc)
	end := time.Date(currentYear, currentMonth+1, 1, 0, 0, 0, -1, loc) // Subtract 1 nanosecond
	// prevMonth := currentMonth - 1
	// prevYear := currentYear
	// if prevMonth == 0 { // If it's January, set previous month to December of last year
	// 	prevMonth = 12
	// 	prevYear--
	// }

	// // First day of the previous month
	// start := time.Date(prevYear, prevMonth, 1, 0, 0, 0, 0, loc)
	// // Last day of the previous month (midnight of the first day of the current month - 1 nanosecond)
	// end := time.Date(currentYear, currentMonth, 1, 0, 0, 0, -1, loc)

	start = start.Add(-7 * time.Hour)
	end = end.Add(-7 * time.Hour)

	odooFields := []string{
		"id",
		"name",
		"x_merchant",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_studio_alamat",
		"description",
		"sla_deadline",
		"create_date",
		"x_received_datetime_spk",
		"complete_datetime_wo",
		"x_task_type",
		"company_id",
		"stage_id",
		"x_master_mid",
		"x_master_tid",
		"x_source",
		"x_job_id",
		"x_wo_number_last",
		"x_status_merchant",
		"x_merchant_sn_edc",
		"x_merchant_tipe_edc",
		"x_wo_remark",
		"technician_id",
		"x_reasoncode",
		"fsm_task_ids",
		"x_partner_latitude",
		"x_partner_longitude",
		"x_worksheet_template_id",
		"ticket_type_id",
	}

	payload := map[string]interface{}{
		"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
		"params": map[string]interface{}{
			"model": odooModel,
			"domain": []interface{}{
				"|", []interface{}{"x_wo_remark", "ilike", "EDC tidak ada di lokasi merchant"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - EDC Not in Merchant Location"},
				"|", []interface{}{"x_wo_remark", "ilike", "EDC berada di KP Merchant"},
				"|", []interface{}{"x_wo_remark", "ilike", "EDC Sudah di Tarik Vendor Lain"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant tutup permanen"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant tutup sementara"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant menolak dilakukannya pekerjaan"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Refused Installation"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Refused Pull Out"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Temporarily Closed"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Permanently Closed/Moved"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Visit Re-Schedule"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant meminta reschedule kunjungan"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Refuses Work"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant Refuses Work"},
				"|", []interface{}{"x_wo_remark", "ilike", "Merchant - Location Not Ready / Renovation"},
				"|", []interface{}{"x_wo_remark", "ilike", "Lokasi Merchant Belum Siap"},
				[]interface{}{"active", "=", true},
				[]interface{}{"x_wo_number_last", "!=", false},
				[]interface{}{"x_reasoncode", "!=", false},
				[]interface{}{"company_id", "=", companyAllowed},
				[]interface{}{"stage_id", "=", stageIncluded},
				[]interface{}{"technician_id", "!=", technicianExcluded},
				[]interface{}{"fsm_task_ids", "!=", false},
				[]interface{}{"x_task_type", "!=", "Preventive Maintenance"},
				// SOON u dont need this again if want to get not current year & current month
				[]interface{}{"complete_datetime_wo", ">=", start.Format("2006-01-02 15:04:05")},
				[]interface{}{"complete_datetime_wo", "<=", end.Format("2006-01-02 15:04:05")},
			},
			"fields": odooFields,
			"order":  odooOrder,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Print(err)
		return
	}

	result, err := getODOOData(h.YamlCfg, string(payloadBytes))
	if err != nil {
		log.Print(err)
		return
	}

	resultArray, ok := result.([]interface{})
	if !ok {
		log.Print("error: failed to assert results as []interface{}")
		return
	}

	if len(resultArray) == 0 {
		return
	}

	totalDataGet := 0
	totalDataBeingInsert := 0
	totalDataBeingUpdate := 0
	totalDataCannotBeFU := 0

	for i, record := range resultArray {
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			log.Printf("[%v] invalid record format in resultArray", i)
			continue
		}

		var odooData OdooTicketSolvedPendingDataRequestItem
		jsonData, err := json.Marshal(recordMap)

		if err != nil {
			log.Printf("failed to marshal recordMap: %v", err)
			continue
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			log.Printf("failed to unmarshal into odooData struct: %v", err)
			continue
		}

		companyId, companyName, err := parseJSONIDDataCombined(odooData.CompanyId)
		if err != nil {
			log.Print(err)
			continue
		}

		ticketStageId, ticketStageName, err := parseJSONIDDataCombined(odooData.StageId)
		if err != nil {
			log.Print(err)
			continue
		}

		snEdcId, snEdcName, err := parseJSONIDDataCombined(odooData.SnEdc)
		if err != nil {
			log.Print(err)
			continue
		}

		edcTypeId, edcTypeName, err := parseJSONIDDataCombined(odooData.EdcType)
		if err != nil {
			log.Print(err)
			continue
		}

		technicianId, technicianName, err := parseJSONIDDataCombined(odooData.TechnicianId)
		if err != nil {
			log.Print(err)
			continue
		}

		worksheetTemplateId, worksheetTemplate, err := parseJSONIDDataCombined(odooData.WorksheetTemplateId)
		if err != nil {
			log.Print(err)
			continue
		}

		ticketTypeId, ticketType, err := parseJSONIDDataCombined(odooData.TicketTypeId)
		if err != nil {
			log.Print(err)
			continue
		}

		_ = worksheetTemplateId
		_ = ticketTypeId

		var validPhoneNumber string
		var gotErrorMsg string
		if odooData.PicPhone.String != "" {
			sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(odooData.PicPhone.String)
			if err != nil {
				gotErrorMsg = err.Error()
			}

			isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
			if isValidWhatsappPhoneNumber {
				validPhoneNumber = sanitizedPhoneNumber
			} else {
				gotErrorMsg = fmt.Sprintf("%v is not being registered in Whatsapp", odooData.PicPhone.String)
			}
		} else {
			gotErrorMsg = "empty pic phone number"
		}

		// (1st) check in all MIDTID Ticket & Task Data
		if validPhoneNumber == "" {
			if odooData.Mid.String != "" && odooData.Tid.String != "" {
				phoneNumberFromMIDTID, err := h.GetODOOPhoneNumberBasedonMIDTID(odooData.Mid.String + odooData.Tid.String)
				if err != nil {
					gotErrorMsg = err.Error()
				}
				validPhoneNumber = phoneNumberFromMIDTID
			} else {
				gotErrorMsg = "empty mid & tid"
				continue
			}
		}

		// (2nd) check if valid phone number still empty then continue looping
		if validPhoneNumber == "" {
			if len(gotErrorMsg) == 0 {
				gotErrorMsg = "pic phone number or owner phone number is not found!"
			}
		}

		if len(gotErrorMsg) > 0 {
			cannotFUData := models.CannotFollowUp{
				RequestType:       requestType,
				WoNumber:          odooData.WoNumber,
				TicketSubject:     odooData.TicketSubject,
				Mid:               odooData.Mid.String,
				Tid:               odooData.Tid.String,
				TaskType:          odooData.TaskType.String,
				WorksheetTemplate: worksheetTemplate,
				TicketType:        ticketType,
				Message:           gotErrorMsg,
			}
			if err := h.Database.
				Create(&cannotFUData).
				Error; err != nil {
				log.Print(err)
			}

			continue
		}

		var slaDeadline, createDate, receivedDatetimeSpk, timesheetLastStop *time.Time
		if !odooData.SlaDeadline.Time.IsZero() {
			slaDeadline = &odooData.SlaDeadline.Time
		}
		if !odooData.CreateDate.Time.IsZero() {
			createDate = &odooData.CreateDate.Time
		}
		if !odooData.ReceivedDatetimeSpk.Time.IsZero() {
			receivedDatetimeSpk = &odooData.ReceivedDatetimeSpk.Time
		}
		if !odooData.TimesheetLastStop.Time.IsZero() {
			timesheetLastStop = &odooData.TimesheetLastStop.Time
		}

		// nil value
		var dateTimeFormatPlanSchedule *time.Time = nil

		var bankVendor string
		odooTicketSource := strings.ToUpper(odooData.Source.String)
		if strings.Contains(odooTicketSource, "BMRI") {
			bankVendor = strings.ReplaceAll(odooTicketSource, "BMRI", "MANDIRI")
		} else {
			bankVendor = odooTicketSource
		}

		ticketLatitudeStr := "0.0"
		ticketLongitudeStr := "0.0"
		if odooData.Latitude.Float != 0 {
			ticketLatitudeStr = strconv.FormatFloat(odooData.Latitude.Float, 'g', -1, 64)
		}
		if odooData.Longitude.Float != 0 {
			ticketLongitudeStr = strconv.FormatFloat(odooData.Longitude.Float, 'g', -1, 64)
		}

		var ticketTaskId uint
		if odooData.TaskId.Valid {
			if taskIDs, ok := odooData.TaskId.Data.([]interface{}); ok && len(taskIDs) > 0 {
				if firstTaskID, ok := taskIDs[0].(float64); ok {
					ticketTaskId = uint(firstTaskID)
				}
			}
		}

		containAOB := IsAOBRelated(odooData.TicketSubject)
		if containAOB {
			continue
		}

		dataToDB := models.WaRequest{
			ID:                      ticketTaskId,
			Counter:                 0,
			RequestType:             requestType,
			MerchantName:            odooData.MerchantName.String,
			PicMerchant:             odooData.PicMerchant.String,
			PicPhone:                validPhoneNumber,
			MerchantAddress:         odooData.MerchantAddress.String,
			Description:             odooData.Description.String,
			SlaDeadline:             slaDeadline,
			CreateDate:              createDate,
			ReceivedDatetimeSpk:     receivedDatetimeSpk,
			PlanDate:                nil,
			TimesheetLastStop:       timesheetLastStop,
			TaskType:                odooData.TaskType.String,
			CompanyId:               companyId,
			CompanyName:             companyName,
			StageId:                 0,
			StageName:               "",
			HelpdeskTicketId:        odooData.ID,
			HelpdeskTicketName:      odooData.TicketSubject,
			Mid:                     odooData.Mid.String,
			Tid:                     odooData.Tid.String,
			Source:                  bankVendor,
			MessageCC:               "",
			WoNumber:                odooData.WoNumber,
			StatusMerchant:          odooData.StatusMerchant.String,
			SnEdcId:                 snEdcId,
			SnEdc:                   snEdcName,
			EdcTypeId:               edcTypeId,
			EdcType:                 edcTypeName,
			WoRemarkTiket:           odooData.WoRemarkTiket.String,
			Latitude:                ticketLatitudeStr,
			Longitude:               ticketLongitudeStr,
			TechnicianId:            technicianId,
			TechnicianName:          technicianName,
			ReasonCodeId:            0,
			ReasonCodeName:          odooData.ReasonCode.String,
			IsOnCalling:             false,
			IsDone:                  false,
			TempCS:                  0,
			UpdatedToOdoo:           false,
			OrderWish:               orderWish,
			TargetScheduleDate:      dateTimeFormatPlanSchedule,
			Keterangan:              "",
			IsFinal:                 false,
			LastUpdateBy:            lastUpdateBy,
			RequestToCC:             requestToCC,
			JobId:                   odooData.JobId.String,
			TicketStageId:           ticketStageId,
			TicketStageName:         ticketStageName,
			NextFollowUpTo:          "",
			IsOnCallingDatetime:     nil,
			IsDoneDatetime:          nil,
			GroupWaJid:              "",
			StanzaId:                "",
			OriginalSenderJid:       "",
			ImgWaPath:               "",
			ImgSnEdcPath:            "",
			ImgMerchantPath:         "",
			MarkDoneByOperational:   false,
			MarkDoneByInventory:     false,
			MarkDoneByPmo:           false,
			MarkDoneByMonitoring:    false,
			RemarkByOperational:     "",
			RemarkByInventory:       "",
			RemarkByPmo:             "",
			RemarkByMonitoring:      "",
			AttachmentByOperational: "",
			AttachmentByInventory:   "",
			AttachmentByPmo:         "",
			AttachmentByMonitoring:  "",
		}

		var existingRequest models.WaRequest
		err = h.Database.Table(h.YamlCfg.Db.TbWaReq).
			Where("helpdesk_ticket_id = ? AND x_no_task = ?", odooData.ID, odooData.WoNumber).
			First(&existingRequest).
			Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := h.Database.Table(h.YamlCfg.Db.TbWaReq).Create(&dataToDB).Error; err != nil {
					log.Print(err)
				} else {
					totalDataBeingInsert++
				}
			} else {
				log.Print(err)
			}
		} else {
			// Record exists, update it
			if err := h.Database.Table(h.YamlCfg.Db.TbWaReq).
				Where("id = ? AND is_on_calling = ? AND is_done = ? AND temp_cs = ?",
					existingRequest.ID,
					false,
					false,
					0,
				).
				Updates(map[string]interface{}{
					"counter":                   dataToDB.Counter,
					"request_type":              requestType,
					"x_merchant":                dataToDB.MerchantName,
					"x_pic_merchant":            dataToDB.PicMerchant,
					"x_pic_phone":               dataToDB.PicPhone,
					"partner_street":            dataToDB.MerchantAddress,
					"description":               dataToDB.Description,
					"x_sla_deadline":            slaDeadline,
					"create_date":               createDate,
					"x_received_datetime_spk":   receivedDatetimeSpk,
					"planned_date_begin":        nil,
					"timesheet_last_stop":       timesheetLastStop,
					"x_task_type":               dataToDB.TaskType,
					"company_id":                dataToDB.CompanyId,
					"company_name":              dataToDB.CompanyName,
					"stage_id":                  dataToDB.StageId,
					"stage_name":                dataToDB.StageName,
					"helpdesk_ticket_id":        dataToDB.HelpdeskTicketId,
					"helpdesk_ticket_name":      dataToDB.HelpdeskTicketName,
					"x_cimb_master_mid":         dataToDB.Mid,
					"x_cimb_master_tid":         dataToDB.Tid,
					"x_source":                  dataToDB.Source,
					"x_message_call":            dataToDB.MessageCC,
					"x_no_task":                 dataToDB.WoNumber,
					"x_status_merchant":         dataToDB.StatusMerchant,
					"x_studio_edc_id":           dataToDB.SnEdcId,
					"x_studio_edc":              dataToDB.SnEdc,
					"x_product_id":              dataToDB.EdcTypeId,
					"x_product":                 dataToDB.EdcType,
					"x_wo_remark":               dataToDB.WoRemarkTiket,
					"x_longitude":               dataToDB.Longitude,
					"x_latitude":                dataToDB.Latitude,
					"technician_id":             dataToDB.TechnicianId,
					"technician_name":           dataToDB.TechnicianName,
					"reason_code_id":            dataToDB.ReasonCodeId,
					"reason_code_name":          dataToDB.ReasonCodeName,
					"is_on_calling":             dataToDB.IsOnCalling,
					"is_done":                   dataToDB.IsDone,
					"temp_cs":                   dataToDB.TempCS,
					"updated_to_odoo":           dataToDB.UpdatedToOdoo,
					"order_wish":                orderWish,
					"target_schedule_date":      dateTimeFormatPlanSchedule,
					"keterangan":                "",
					"is_final":                  dataToDB.IsFinal,
					"x_job_id":                  dataToDB.JobId,
					"last_update_by":            dataToDB.LastUpdateBy,
					"request_to_cc":             dataToDB.RequestToCC,
					"ticket_stage_id":           dataToDB.TicketStageId,
					"ticket_stage_name":         dataToDB.TicketStageName,
					"next_follow_up_to":         dataToDB.NextFollowUpTo,
					"is_on_calling_datetime":    nil,
					"is_done_datetime":          nil,
					"group_wa_jid":              dataToDB.GroupWaJid,
					"stanza_id":                 dataToDB.StanzaId,
					"original_sender_jid":       dataToDB.OriginalSenderJid,
					"img_wa_path":               "",
					"img_sn_edc_path":           "",
					"img_merchant_path":         "",
					"mark_done_by_operational":  dataToDB.MarkDoneByOperational,
					"remark_by_operational":     dataToDB.RemarkByOperational,
					"attachment_by_operational": dataToDB.AttachmentByOperational,
					"mark_done_by_inventory":    dataToDB.MarkDoneByInventory,
					"remark_by_inventory":       dataToDB.RemarkByInventory,
					"attachment_by_inventory":   dataToDB.AttachmentByInventory,
					"mark_done_by_pmo":          dataToDB.MarkDoneByPmo,
					"remark_by_pmo":             dataToDB.RemarkByPmo,
					"attachment_by_pmo":         dataToDB.AttachmentByPmo,
					"mark_done_by_monitoring":   dataToDB.MarkDoneByMonitoring,
					"remark_by_monitoring":      dataToDB.RemarkByMonitoring,
					"attachment_by_monitoring":  dataToDB.AttachmentByMonitoring,
				}).
				Error; err != nil {
				log.Print(err)
			} else {
				totalDataBeingUpdate++
			}
		}
	} // .end of looping odoo ticket map data

	totalDataGet = len(resultArray)
	var totalDataCannotBeFUInt64 int64
	h.Database.Model(&models.CannotFollowUp{}).Where("request_type = ?", requestType).Count(&totalDataCannotBeFUInt64)
	totalDataCannotBeFU = int(totalDataCannotBeFUInt64)

	if totalDataGet == 0 && totalDataBeingInsert == 0 && totalDataBeingUpdate == 0 && totalDataCannotBeFU == 0 {
		var sb strings.Builder
		sb.WriteString("⚠ Kami mohon maaf, karena tidak adanya data Ticket Solved Pending yang dapat di-follow up oleh tim *Call Center* pada hari ini, karena terdapat masalah pada _system_.\n")
		sb.WriteString(fmt.Sprintf("\nUntuk info lebih lanjut, silahkan hubungi *IT Support +%v* terkait masalah ini!", h.YamlCfg.Whatsmeow.WaSupport))
		sb.WriteString(fmt.Sprintf("\n\n~Regards, Call Center Team *%v*", h.YamlCfg.Default.PT))
		msgToSend := sb.String()
		_, err = h.Client.SendMessage(context.Background(), jid, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text: &msgToSend,
			},
		})
		if err != nil {
			log.Printf("[ERROR] JID: %v, got error: %v", jid, err)
			return
		}
		return
	}

	var sb strings.Builder
	sb.WriteString("*[INFO]* Berikut hasil tarikan _system_ terkait data Ticket Solved Pending untuk selanjutnya di-follow up oleh tim *Call Center*:\n")
	sb.WriteString(fmt.Sprintf(
		"\nTotal Data Tarikan Ticket Solved Pending: %d\nTotal Data Baru yang Diinput ke Database: %d\nTotal Data yang Diupdate di Database: %d\nTotal Data Invalid / Tidak Bisa di Follow Up: %d",
		totalDataGet, totalDataBeingInsert, totalDataBeingUpdate, totalDataCannotBeFU,
	))
	msgToSend := sb.String()
	_, err = h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: &msgToSend,
		},
	})
	if err != nil {
		log.Printf("[ERROR] JID: %v, got error: %v", jid, err)
		return
	}

	if totalDataCannotBeFU > 0 {
		// var sb strings.Builder
		// sb.WriteString(fmt.Sprintf("⚠ Kami mohon maaf, karena %d data Ticket Solved Pending berikut tidak dapat di-follow up oleh tim *Call Center*:\n", totalDataCannotBeFU))
		// for _, record := range cannotFURecords {
		// 	sb.WriteString(fmt.Sprintf("   - %v\n", record))
		// }
		// sb.WriteString(fmt.Sprintf("\n\n~Regards, Call Center Team *%v*", h.YamlCfg.Default.PT))
		// msgToSend := sb.String()
		// _, err = h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		// 	ExtendedTextMessage: &waProto.ExtendedTextMessage{
		// 		Text: &msgToSend,
		// 	},
		// })
		// if err != nil {
		// 	log.Printf("[ERROR] JID: %v, got error: %v", jid, err)
		// 	return
		// }
		// h.sendAttachCannotFUbyCC(jid, requestType, cannotFURecords)
		h.sendAttachCannotFUbyCC(jidInvent, requestType)
	}

	log.Printf("Scheduler Get Data Solved Pending Ticket for FU by Call Center successfully executed @%v", time.Now())
}
