package whatsapp

import (
	"call_center_app/utils"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (h *WhatsmeowHandler) CheckValidWhatsappPhoneNumber(phoneNumber string) bool {
	if phoneNumber == "" {
		return false
	} else {
		if len(phoneNumber) > digitNoTelp {

			sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(phoneNumber)
			if err != nil {
				return false
			}

			result, err := h.Client.IsOnWhatsApp([]string{sanitizedPhoneNumber + "@s.whatsapp.net"})
			if err != nil {
				return false
			} else {
				if len(result) > 0 {
					contact := result[0]
					if !contact.IsIn {
						return false
					} else {
						return true
					}
				} else {
					return false
				}
			}
		} else {
			return false
		}
	}
}

func (h *WhatsmeowHandler) GetODOOPhoneNumberBasedonMIDTID(midtid string) (string, error) {
	if midtid == "" {
		return "", nil
	} else {
		dataMidTid := strings.TrimSpace(midtid)

		// Get data from Customers (res.partner)
		odooModel := "res.partner"
		odooDomain := []interface{}{
			[]interface{}{"name", "=", dataMidTid},
		}
		odooFields := []string{
			"id",
			"task_ids",
			"x_ticket_ids",
			"x_merchant_pic_phone",
			"phone",
			"mobile",
		}
		odooOrder := "id asc"
		odooParams := map[string]interface{}{
			"model":  odooModel,
			"domain": odooDomain,
			"fields": odooFields,
			"order":  odooOrder,
		}

		odooRequest := map[string]interface{}{
			"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
			"params":  odooParams,
		}

		odooPayload, err := json.Marshal(odooRequest)
		if err != nil {
			return "", err
		}

		result, err := getODOOData(h.YamlCfg, string(odooPayload))
		if err != nil {
			return "", err
		}

		resultArray, ok := result.([]interface{})
		if !ok {
			return "", errors.New("failed to assert results as []interface{}")
		}

		if len(resultArray) == 0 {
			return "", errors.New("empty data in odoo while trying to search in res.partner")
		}

		var midtidData odooResPartnerDataItem
		var taskIds []int
		var ticketIds []int
		var validWhatsappPhoneNumber string
		var errorMsg error

		for i, record := range resultArray {
			recordMap, ok := record.(map[string]interface{})
			if !ok {
				return "", fmt.Errorf("MIDTID: %v, got error while trying to get its phone number: [%d] invalid record format in midtid resultArray", dataMidTid, i)
			}

			jsonData, err := json.Marshal(recordMap)
			if err != nil {
				return "", fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in index[%d] %v", dataMidTid, i, err)
			}

			err = json.Unmarshal(jsonData, &midtidData)
			if err != nil {
				return "", fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in index[%d] %v", dataMidTid, i, err)
			}

			taskIds = midtidData.TaskIds.ToIntSlice()
			ticketIds = midtidData.TicketIds.ToIntSlice()

			if midtidData.PicPhone.String != "" {
				sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(midtidData.PicPhone.String)
				if err != nil {
					errorMsg = err
				}
				isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
				if isValidWhatsappPhoneNumber {
					validWhatsappPhoneNumber = sanitizedPhoneNumber
					return validWhatsappPhoneNumber, nil
				} else {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in index[%d] is got invalid phone number or not registered in whatsapp", dataMidTid, i)
				}
			}
		}

		/*
			Get PIC Phone Number from Ticket Ids
		*/
		ticketOdooModel := "helpdesk.ticket"
		ticketOdooDomain := []interface{}{
			[]interface{}{"id", "=", ticketIds},
		}
		ticketOdooFields := []string{
			"id",
			"x_merchant_pic_phone",
		}
		ticketOdooOrder := "id asc"
		ticketOdooParams := map[string]interface{}{
			"model":  ticketOdooModel,
			"domain": ticketOdooDomain,
			"fields": ticketOdooFields,
			"order":  ticketOdooOrder,
		}
		ticketOdooRequest := map[string]interface{}{
			"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
			"params":  ticketOdooParams,
		}

		ticketOdooPayload, err := json.Marshal(ticketOdooRequest)
		if err != nil {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from helpdesk.ticket %v", dataMidTid, err)
		}

		ticketResult, err := getODOOData(h.YamlCfg, string(ticketOdooPayload))
		if err != nil {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from helpdesk.ticket %v", dataMidTid, err)
		}

		ticketResultArray, ok := ticketResult.([]interface{})
		if !ok {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from helpdesk.ticket failed to assert results as []interface{}", dataMidTid)
		}

		if len(ticketResultArray) == 0 {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from helpdesk.ticket coz its empty data in ODOO", dataMidTid)
		}

		var ticketData helpdeskTicketPicPhoneDataItem
		for i, record := range ticketResultArray {
			recordMap, ok := record.(map[string]interface{})
			if !ok {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number: [%d] invalid record format in midtid helpdesk.ticket resultArray", dataMidTid, i)
			}

			jsonData, err := json.Marshal(recordMap)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in helpdesk.ticket index[%d] %v", dataMidTid, i, err)
			}

			err = json.Unmarshal(jsonData, &ticketData)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in helpdesk.ticket index[%d] %v", dataMidTid, i, err)
			}

			if ticketData.PicPhone.String != "" {
				sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(ticketData.PicPhone.String)
				if err != nil {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in helpdesk.ticket index[%d] %v", dataMidTid, i, err)
				}
				isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
				if isValidWhatsappPhoneNumber {
					validWhatsappPhoneNumber = sanitizedPhoneNumber
					// return validWhatsappPhoneNumber, errorData
					return validWhatsappPhoneNumber, nil
				} else {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in helpdesk.ticket index[%d] is not valid phone number or not registered in whatsapp", dataMidTid, i)
				}
			}
		}

		/*
			Get PIC Phone Number from Task Ids
		*/
		taskOdooModel := "project.task"
		taskOdooDomain := []interface{}{
			[]interface{}{"id", "=", taskIds},
		}
		taskOdooFields := []string{
			"id",
			"x_pic_phone",
		}
		taskOdooOrder := "id asc"
		taskOdooParams := map[string]interface{}{
			"model":  taskOdooModel,
			"domain": taskOdooDomain,
			"fields": taskOdooFields,
			"order":  taskOdooOrder,
		}
		taskOdooRequest := map[string]interface{}{
			"jsonrpc": h.YamlCfg.ApiODOO.JSONRPC,
			"params":  taskOdooParams,
		}

		taskOdooPayload, err := json.Marshal(taskOdooRequest)
		if err != nil {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from project.task %v", dataMidTid, err)
		}

		taskResult, err := getODOOData(h.YamlCfg, string(taskOdooPayload))
		if err != nil {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from project.task %v", dataMidTid, err)
		}

		taskResultArray, ok := taskResult.([]interface{})
		if !ok {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from project.task failed to assert results as []interface{}", dataMidTid)
		}

		if len(taskResultArray) == 0 {
			errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number from project.task coz its empty data in ODOO", dataMidTid)
		}

		var taskData projectTaskPicPhoneDataItem
		for i, record := range taskResultArray {
			recordMap, ok := record.(map[string]interface{})
			if !ok {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number: [%d] invalid record format in midtid project.task resultArray", dataMidTid, i)
			}

			jsonData, err := json.Marshal(recordMap)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in project.task index[%d] %v", dataMidTid, i, err)
			}

			err = json.Unmarshal(jsonData, &taskData)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in project.task index[%d] %v", dataMidTid, i, err)
			}

			if taskData.PicPhone.String != "" {
				sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(taskData.PicPhone.String)
				if err != nil {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in project.task index[%d] %v", dataMidTid, i, err)
				}
				isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
				if isValidWhatsappPhoneNumber {
					validWhatsappPhoneNumber = sanitizedPhoneNumber
					// return validWhatsappPhoneNumber, errorData
					return validWhatsappPhoneNumber, nil
				} else {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in project.task index[%d] is not valid phone number or not registered in whatsapp", dataMidTid, i)
				}
			}
		}

		// Check back in res.partner item if contact person phone & mobile is valid
		for i, record := range resultArray {
			recordMap, ok := record.(map[string]interface{})
			if !ok {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number: [%d] invalid record format in midtid resultArray", dataMidTid, i)
				return "", errorMsg
			}

			jsonData, err := json.Marshal(recordMap)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in index[%d] %v", dataMidTid, i, err)
				return "", errorMsg
			}

			err = json.Unmarshal(jsonData, &midtidData)
			if err != nil {
				errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its phone number in index[%d] %v", dataMidTid, i, err)
				return "", errorMsg
			}

			if midtidData.ContactPhone.String != "" {
				sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(midtidData.ContactPhone.String)
				if err != nil {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its contact phone number in index[%d] %v", dataMidTid, i, err)
				}
				isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
				if isValidWhatsappPhoneNumber {
					validWhatsappPhoneNumber = sanitizedPhoneNumber
					// return validWhatsappPhoneNumber, errorData
					return validWhatsappPhoneNumber, nil
				} else {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its contact phone number in index[%d] is got invalid phone number or not registered in whatsapp", dataMidTid, i)
				}
			}

			if midtidData.ContactMobile.String != "" {
				sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(midtidData.ContactMobile.String)
				if err != nil {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its contact mobiel number in index[%d] %v", dataMidTid, i, err)
				}
				isValidWhatsappPhoneNumber := h.CheckValidWhatsappPhoneNumber(sanitizedPhoneNumber)
				if isValidWhatsappPhoneNumber {
					validWhatsappPhoneNumber = sanitizedPhoneNumber
					// return validWhatsappPhoneNumber, errorData
					return validWhatsappPhoneNumber, nil
				} else {
					errorMsg = fmt.Errorf("MIDTID: %v, got error while trying to get its contact phone number in index[%d] is got invalid phone number or not registered in whatsapp", dataMidTid, i)
				}
			}
		}

		itsValidPhoneNumber := h.CheckValidWhatsappPhoneNumber(validWhatsappPhoneNumber)
		if itsValidPhoneNumber {
			return validWhatsappPhoneNumber, nil
		} else {
			// Default
			return "", errorMsg
		}
	}
}
