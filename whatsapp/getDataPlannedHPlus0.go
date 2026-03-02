package whatsapp

import (
	"call_center_app/models"
	"call_center_app/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

var getDataPlannedHPlus0Mutex sync.Mutex

func (h *WhatsmeowHandler) GetDataPlannedHPlus0() {
	if !getDataPlannedHPlus0Mutex.TryLock() {
		log.Println("GetDataPlannedHPlus0 is already running, skipping execution.")
		return
	}
	defer getDataPlannedHPlus0Mutex.Unlock()

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

	taskDoing := "Get Data Planned H+0"
	log.Printf("Running scheduler %v for Followed Up by CC @%v", taskDoing, time.Now())

	// nil value
	var dateTimeFormatPlanSchedule *time.Time = nil
	orderWish := "Merchant Confirmation"
	requestType := "Data Planned H+0 Follow Up"
	lastUpdateBy := "System"
	requestToCC := "Konfirmasi terlebih dahulu apakah teknisi sudah melakukan kunjungan / belum ke lokasi merchant. Juga, konfirmasi ke PIC merchant terkait apakah perangkat EDC yang digunakan masih dilokasi merchant / tidak, serta perhatikan Description yang tertera."

	// Running scheduler get data Planned H+0
	odooModel := "project.task"
	odooOrder := "id desc"
	companyAllowed := h.YamlCfg.ApiODOO.CompanyAllowed
	stageExcluded := []string{
		"Done",
		"Verified",
		"Cancel",
	}
	technicianExcluded := []int{
		3046, // "Teknisi Pameran",
		5,    // "Tes Dev Mfjr",
	}

	odooFields := []string{
		"id",
		"x_merchant",
		"x_pic_merchant",
		"x_pic_phone",
		"partner_street",
		"x_title_cimb", // "description"
		"x_sla_deadline",
		"create_date",
		"x_task_type",
		"company_id",
		"stage_id",
		"helpdesk_ticket_id",
		"x_cimb_master_tid",
		"x_cimb_master_mid",
		"x_source",
		"x_message_call",
		"x_no_task",
		"x_status_merchant",
		"x_studio_edc",
		"x_product",
		"x_wo_remark",
		"x_latitude",
		"x_longitude",
		"technician_id",
		"x_received_datetime_spk",
		"planned_date_begin",
		"x_reason_code_id",
		"timesheet_timer_last_stop",
		"worksheet_template_id",
		"x_ticket_type2",
	}

	loc, _ := time.LoadLocation("Asia/Jakarta") // Set timezone to Asia/Jakarta
	now := time.Now().In(loc)

	startParam := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).Add(-7 * time.Hour)
	endParam := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc).Add(-7 * time.Hour)

	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"company_id", "=", companyAllowed},
		[]interface{}{"stage_id", "!=", stageExcluded},
		[]interface{}{"technician_id", "!=", technicianExcluded},
		[]interface{}{"timesheet_timer_last_stop", "=", false},
		[]interface{}{"planned_date_begin", ">=", startParam},
		[]interface{}{"planned_date_begin", "<=", endParam},
		[]interface{}{"x_task_type", "!=", "Preventive Maintenance"},
	}

	odooParams := map[string]interface{}{
		"domain": domain,
		"model":  odooModel,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]interface{}{
		"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
		"params":  odooParams,
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

		var odooData OdooTaskDataRequestItem
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

		var cleanedTicketNumber string

		ticketID, ticketNumber, err := parseJSONIDDataCombined(odooData.HelpdeskTicketId)
		if err != nil {
			log.Print(err)
			continue
		} else {
			re := regexp.MustCompile(`\s*\(.*?\)`)
			cleanedTicketNumber = re.ReplaceAllString(ticketNumber, "")
		}

		if cleanedTicketNumber == "" {
			continue
		}

		containAOB := IsAOBRelated(cleanedTicketNumber)
		if containAOB {
			continue
		}

		companyId, companyName, err := parseJSONIDDataCombined(odooData.CompanyId)
		if err != nil {
			log.Print(err)
			continue
		}

		stageId, stageName, err := parseJSONIDDataCombined(odooData.StageId)
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

		var reasonCodeIDValue int
		var reasonCodeNameValue string

		reasonCodeId, reasonCodeName, err := parseJSONIDDataCombined(odooData.ReasonCodeId)
		if err != nil {
			log.Printf("Cannot processing WO Number: %v, SPK Number: %v cause not have a valid ReasonCode ID!",
				odooData.WoNumber,
				cleanedTicketNumber,
			)
			// cannotFURecords = append(cannotFURecords,
			// 	fmt.Sprintf("WO Number: *%v*, SPK Number: *%v* got error: _%v_.",
			// 		odooData.WoNumber,
			// 		cleanedTicketNumber,
			// 		err,
			// 	),
			// )
			// continue
		}
		reasonCodeIDValue = reasonCodeId
		reasonCodeNameValue = reasonCodeName

		if odooData.Mid.String == "" || odooData.Tid.String == "" {
			log.Println("Empty mid & tid")
			continue
		}

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
				TicketSubject:     cleanedTicketNumber,
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

		var slaDeadline, createDate, receivedDatetimeSpk, planDate, timesheetLastStop *time.Time
		if !odooData.SlaDeadline.Time.IsZero() {
			slaDeadline = &odooData.SlaDeadline.Time
		}
		if !odooData.CreateDate.Time.IsZero() {
			createDate = &odooData.CreateDate.Time
		}
		if !odooData.ReceivedDatetimeSpk.Time.IsZero() {
			receivedDatetimeSpk = &odooData.ReceivedDatetimeSpk.Time
		}
		if !odooData.PlanDate.Time.IsZero() {
			planDate = &odooData.PlanDate.Time
		}
		if !odooData.TimesheetLastStop.Time.IsZero() {
			timesheetLastStop = &odooData.TimesheetLastStop.Time
		}

		/* Get helpdesk.ticket data*/
		odooModel := "helpdesk.ticket"
		odooFields := []string{
			"id",
			"x_job_id",
			"stage_id",
		}
		domain := []interface{}{
			[]interface{}{"active", "=", true},
			[]interface{}{"company_id", "=", companyAllowed},
			[]interface{}{"id", "=", ticketID},
		}

		odooParams := map[string]interface{}{
			"domain": domain,
			"model":  odooModel,
			"fields": odooFields,
			"order":  odooOrder,
		}

		payload := map[string]interface{}{
			"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
			"params":  odooParams,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Print(err)
			continue
		}

		ticketResult, err := getODOOData(h.YamlCfg, string(payloadBytes))
		if err != nil {
			log.Print(err)
			continue
		}

		ticketResultArray, ok := ticketResult.([]interface{})
		if !ok {
			log.Println("failed to assert results as []interface{}")
			continue
		}

		var bankVendor string
		odooTaskSource := strings.ToUpper(odooData.Source.String)
		if strings.Contains(odooTaskSource, "BMRI") {
			bankVendor = strings.ReplaceAll(odooTaskSource, "BMRI", "MANDIRI")
		} else {
			bankVendor = odooTaskSource
		}

		var ticketJobId string
		var ticketStageId int
		var ticketStageName string

		for _, ticketRecord := range ticketResultArray {
			ticketRecordMap, ok := ticketRecord.(map[string]interface{})
			if !ok {
				log.Println("invalid record format in tiket result array")
				continue
			}

			var odooTicketData OdooTicketDataRequestItem
			jsonDataTicket, err := json.Marshal(ticketRecordMap)
			if err != nil {
				log.Print(err)
				continue
			}

			err = json.Unmarshal(jsonDataTicket, &odooTicketData)
			if err != nil {
				log.Print(err)
				continue
			}

			ticketJobId = odooTicketData.JobId.String
			ticketStageId, ticketStageName, err = parseJSONIDDataCombined(odooData.StageId)
			if err != nil {
				log.Print(err)
				continue
			}
		} // .end of looping ticket data map

		dataToDB := models.WaRequest{
			ID:                      uint(odooData.ID),
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
			PlanDate:                planDate,
			TimesheetLastStop:       timesheetLastStop,
			TaskType:                odooData.TaskType.String,
			CompanyId:               companyId,
			CompanyName:             companyName,
			StageId:                 stageId,
			StageName:               stageName,
			HelpdeskTicketId:        ticketID,
			HelpdeskTicketName:      cleanedTicketNumber,
			Mid:                     odooData.Mid.String,
			Tid:                     odooData.Tid.String,
			Source:                  bankVendor,
			MessageCC:               odooData.MessageCC.String,
			WoNumber:                odooData.WoNumber,
			StatusMerchant:          odooData.StatusMerchant.String,
			SnEdcId:                 snEdcId,
			SnEdc:                   snEdcName,
			EdcTypeId:               edcTypeId,
			EdcType:                 edcTypeName,
			WoRemarkTiket:           odooData.WoRemarkTiket.String,
			Latitude:                odooData.Latitude.String,
			Longitude:               odooData.Longitude.String,
			TechnicianId:            technicianId,
			TechnicianName:          technicianName,
			ReasonCodeId:            reasonCodeIDValue,
			ReasonCodeName:          reasonCodeNameValue,
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
			JobId:                   ticketJobId,
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
			Where("helpdesk_ticket_id = ? AND x_no_task = ?",
				dataToDB.HelpdeskTicketId,
				dataToDB.WoNumber,
			).
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
					"planned_date_begin":        planDate,
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
	} // .end of looping odoo map data

	totalDataGet = len(resultArray)
	var totalDataCannotBeFUInt64 int64
	h.Database.Model(&models.CannotFollowUp{}).Where("request_type = ?", requestType).Count(&totalDataCannotBeFUInt64)
	totalDataCannotBeFU = int(totalDataCannotBeFUInt64)

	if totalDataGet == 0 && totalDataBeingInsert == 0 && totalDataBeingUpdate == 0 && totalDataCannotBeFU == 0 {
		var sb strings.Builder
		sb.WriteString("⚠ Kami mohon maaf, karena tidak adanya data Planned H+0 yang dapat di-follow up oleh tim *Call Center* pada hari ini, karena terdapat masalah pada _system_.\n")
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
	sb.WriteString("*[INFO]* Berikut hasil tarikan _system_ terkait data Planned H+0 untuk selanjutnya di-follow up oleh tim *Call Center*:\n")
	sb.WriteString(fmt.Sprintf(
		"\nTotal Data Tarikan Planned H+0: %d\nTotal Data Baru yang Diinput ke Database: %d\nTotal Data yang Diupdate di Database: %d\nTotal Data Invalid / Tidak Bisa di Follow Up: %d",
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
		// sb.WriteString(fmt.Sprintf("⚠ Kami mohon maaf, karena %d data Planned H+0 berikut tidak dapat di-follow up oleh tim *Call Center*:\n", totalDataCannotBeFU))
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

	log.Printf("Scheduler %v for Followed Up by Call Center successfully executed @%v", taskDoing, time.Now())
}
