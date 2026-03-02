package whatsapp

import (
	"bytes"
	"call_center_app/config"
	"call_center_app/models"
	"call_center_app/utils"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

//#################################################################
/*
	Send Message Server
	- To Group: g.us
	- To Contact: s.whatsapp.net
*/
//#################################################################

const layoutPlanSchedule = "02/01/2006"

func FeedbackResultfromFUCC() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("[ERROR] Recovered from panic in FeedbackResultfromFUCC:", r)
			}
		}()

		for feedbackData := range TriggerGetFeedbackFromFU { // Keeps listening
			log.Printf("[INFO] Trigger for feedback request from: %v in group: %v", feedbackData.OriginalSenderJID, feedbackData.GroupWAJID)
			processFeedback(feedbackData) // Process received data
		}
	}()
}

func UpdateDatainODOOFromDataRequestWhatsapp() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Println("[ERROR] Recovered from panic in UpdateDatainODOOFromDataRequestWhatsapp:", r)
			}
		}()

		for dataRequest := range TriggerUpdateDatainODOO { // Keeps listening
			log.Printf("[INFO] Trigger for update data in odoo from whatsapp request using id task: %d", dataRequest.TaskID)
			updateDatainODOO(dataRequest)
		}
	}()
}

func parseJSONIDDataCombined(nullableData nullAbleInterface) (int, string, error) {
	if nullableData.IsEmpty() {
		return 0, "", nil // Return default values for empty data
	}

	arrayData, ok := nullableData.Data.([]interface{})
	if !ok || len(arrayData) < 2 {
		return 0, "", errors.New("invalid array data")
	}

	dataIDFloat, ok := arrayData[0].(float64)
	if !ok {
		return 0, "", errors.New("invalid type for data ID; expected float64")
	}
	dataID := int(dataIDFloat)

	dataString, ok := arrayData[1].(string)
	if !ok {
		return 0, "", errors.New("invalid type for data string; expected string")
	}

	return dataID, dataString, nil
}

func StartOdooSessionRefresher(yamlCfg *config.YamlConfig) {
	RefreshOdooSession(yamlCfg)

	go func() {
		for {
			now := time.Now()
			nextRefresh := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, now.Location())

			if now.After(nextRefresh) {
				nextRefresh = nextRefresh.Add(24 * time.Hour)
			}

			durationUntilNextRefresh := time.Until(nextRefresh)
			log.Printf("🕒 Next Odoo session refresh scheduled at: %v", nextRefresh)

			time.Sleep(durationUntilNextRefresh)
			RefreshOdooSession(yamlCfg)
		}
	}()
}

func RefreshOdooSession(yamlCfg *config.YamlConfig) {
	email := yamlCfg.ApiODOO.Login
	password := yamlCfg.ApiODOO.Password

	newCookies, err := getSessionODOO(email, password, yamlCfg)
	if err != nil {
		log.Printf("[ERROR] Failed to refresh Odoo session: %v", err)
		return
	}

	odooSessionMutex.Lock()
	OdooSessionCookies = newCookies
	odooSessionMutex.Unlock()
	log.Println("✅ Odoo session refreshed successfully at", time.Now().Format("2006-01-02 15:04:05"))
}

func getSessionODOO(email string, password string, yamlCfg *config.YamlConfig) ([]*http.Cookie, error) {
	odooConfig := yamlCfg.ApiODOO

	odooDB := odooConfig.Db
	odooJSONRPC := odooConfig.JSONRPC
	urlSession := odooConfig.UrlSession

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, odooJSONRPC, odooDB, email, password)

	maxRetriesStr := odooConfig.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := odooConfig.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	var errMsg string

	reqTimeout, err := time.ParseDuration(odooConfig.SessionTimeout)
	if err != nil {
		errMsg = fmt.Sprintf("invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			errMsg = fmt.Sprintf("[ERROR] error creating request: %v", err)
			return nil, errors.New(errMsg)
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("[WARNING] error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("[WARNING] bad response, status code: %d (attempt %d/%d), response: %v", response.StatusCode, attempts, maxRetries, response)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg = fmt.Sprintf("[ERROR] post request failed with status code: %v", response.StatusCode)
		return nil, errors.New(errMsg)
	}

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		errMsg = fmt.Sprintf("[ERROR] error reading response body: %v", err)
		return nil, errors.New(errMsg)
	}

	cookieODOO := response.Cookies()
	// log.Printf("ODOO session for email: %v, pwd: %v obtained successfully.", email, password)

	return cookieODOO, nil
}

func getODOOData(config *config.YamlConfig, req string) (interface{}, error) {
	urlGetData := config.ApiODOO.UrlGetData

	maxRetriesStr := config.ApiODOO.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		// log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := config.ApiODOO.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlGetData, bytes.NewBufferString(req))
		if err != nil {
			// log.Printf("[ERROR] creating request: %v", err)
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("[ERROR] making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("[ERROR] Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("[ERROR] POST request failed with status code: %v", response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] reading response body: %v", err)
	}

	// log.Print("Response Body:", string(body))

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("[ERROR] parsing JSON response: %v", err)
	}

	// Check for error response from Odoo
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return nil, fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	// Check for the result in JSON response
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		// Log the message and success status if they exist
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			log.Printf("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		return nil, fmt.Errorf("[ERROR] Result field missing in the response!, error with params: %v", bytes.NewBufferString(req))
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		return nil, fmt.Errorf("[ERROR] Cannot find the data you have been request. Unexpected result format or empty result!, error with params: %v", bytes.NewBufferString(req))
	}

	return result, nil
}

// NewWhatsmeowHandler initializes a new instance of WhatsmeowHandler.
func NewWhatsmeowHandler(client *whatsmeow.Client, db *gorm.DB, yamlCfg *config.YamlConfig) *WhatsmeowHandler {
	go StartOdooSessionRefresher(yamlCfg)

	return &WhatsmeowHandler{
		Client:     client,
		YamlCfg:    yamlCfg,
		GroupJID:   types.NewJID(yamlCfg.Whatsmeow.GroupTestJID, "g.us"),
		GroupCCJID: types.NewJID(yamlCfg.Whatsmeow.GroupCCJID, "g.us"),
		GroupTAJID: types.NewJID(yamlCfg.Whatsmeow.GroupTAJID, "g.us"),
		Database:   db,
		// Database: db.Debug(),
	}
}

// HandleEvent processes WhatsApp messages
func (h *WhatsmeowHandler) HandleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		h.processMessage(v)
	}
}

// Goroutine to Continuously Process Feedback
func cleanJID(jid string) string {
	if strings.Contains(jid, ":") {
		jid = strings.Split(jid, ":")[0] // Remove device part
	}
	if !strings.HasSuffix(jid, "@s.whatsapp.net") {
		jid += "@s.whatsapp.net"
	}
	return jid
}

// Send feedback FU CC with PIC to WAG
func processFeedback(data FeedbackTriggerData) {
	table := data.Config.Db.TbWaReq
	tableCs := data.Config.Db.TbUser

	var sb strings.Builder
	var requestorString string
	var quotedMsg *waProto.ContextInfo
	var waReqData models.WaRequest
	var csData models.CS

	var jidString string
	if data.GroupWAJID == "" || data.StanzaID == "" || data.OriginalSenderJID == "" {
		// jidString = data.Config.Whatsmeow.GroupCCJID + "@g.us"
		jidString = data.Config.Whatsmeow.GroupKoordinasiJID + "@g.us"
	} else {
		jidString = cleanJID(data.GroupWAJID)
		requestorString = strings.Split(cleanJID(data.OriginalSenderJID), "@")[0]

		// Set quoted message if all required fields exist
		quotedMsg = &waProto.ContextInfo{
			StanzaID:    &data.StanzaID,
			Participant: &data.OriginalSenderJID,
		}
	}

	// Parse JID
	jid, err := types.ParseJID(jidString)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidString)
		return
	}

	// Database query: Fetch WaRequest data
	query := data.Database.Table(table).
		Where("is_on_calling = ? AND is_done = ? AND request_type = ? AND x_pic_phone = ?",
			true,
			true,
			data.RequestInWhatsapp,
			data.PicPhoneNumber,
		)

	// Add additional filters
	if data.WoNumber != "" {
		query = query.Where("x_no_task = ?", data.WoNumber)
	}

	if data.SpkNumber != "" {
		query = query.Where("helpdesk_ticket_name = ?", data.SpkNumber)
	}

	if data.StanzaID != "" && data.OriginalSenderJID != "" && data.GroupWAJID != "" {
		query = query.Where("stanza_id = ? AND original_sender_jid = ? AND group_wa_jid = ?",
			data.StanzaID,
			data.OriginalSenderJID,
			data.GroupWAJID,
		)
	}

	err = query.First(&waReqData).Error

	// [DEBUG] delete soon if needed!
	sqlQuery := query.Dialector.Explain(query.Statement.SQL.String(), query.Statement.Vars...)

	if err != nil {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("got error while trying to get pic phone: %v in db log data: %v.\nDetails:\n", data.PicPhoneNumber, err))
		sb.WriteString(fmt.Sprintf("- SPK Number: %v\n", data.SpkNumber))
		sb.WriteString(fmt.Sprintf("- WO Number: %v\n", data.WoNumber))
		sb.WriteString(fmt.Sprintf("- Query exec: %v\n", sqlQuery))

		msgToSend := sb.String()

		sendErrorMessageHelper(data, jid, quotedMsg, requestorString, msgToSend)
		return
	}

	// Database query: Fetch CS Data
	if err := data.Database.Table(tableCs).
		Where("id = ?", waReqData.TempCS).
		First(&csData).
		Error; err != nil {
		sendErrorMessageHelper(data, jid, quotedMsg, requestorString, fmt.Sprintf("got error while trying to get CS Data: %v", err))
		return
	}

	// Check if not got feedback yet
	if waReqData.CallCenterMessage != "" && waReqData.WoNumber != "" && waReqData.HelpdeskTicketName != "" {
		isNotGettingFeedbackYet := isNotFeedbackYet(waReqData.CallCenterMessage)
		if isNotGettingFeedbackYet {
			log.Printf("[INFO] WO Number: %v SPK Number: %v, belum dapat feedback dari PIC merchant", waReqData.WoNumber, waReqData.HelpdeskTicketName)
			return
		}
	}

	// Build feedback message
	sb.WriteString(fmt.Sprintf("🎉 *[REQUEST]* %v", waReqData.RequestType))
	if requestorString != "" {
		sb.WriteString(fmt.Sprintf(" dari +%v", requestorString))
	}
	sb.WriteString(fmt.Sprintf(" berhasil di-Follow Up oleh _%v_ Call Center.\n\n", csData.Username))

	if waReqData.MerchantName != "" && waReqData.PicMerchant != "" {
		sb.WriteString(fmt.Sprintf("Merchant: *%v* [%v]\n", waReqData.MerchantName, waReqData.PicMerchant))
	}
	if waReqData.PicPhone != "" {
		sb.WriteString(fmt.Sprintf("Nomor Telepon: *%v*\n", waReqData.PicPhone))
	}
	if waReqData.MerchantAddress != "" {
		sb.WriteString(fmt.Sprintf("Alamat: %v\n", waReqData.MerchantAddress))
	}
	if waReqData.Mid != "" && waReqData.Tid != "" {
		sb.WriteString(fmt.Sprintf("MID: *%v*, TID: *%v*\n", waReqData.Mid, waReqData.Tid))
	}
	if waReqData.WoNumber != "" {
		sb.WriteString(fmt.Sprintf("WO Number: *%v*\n", waReqData.WoNumber))
	}
	if waReqData.HelpdeskTicketName != "" {
		sb.WriteString(fmt.Sprintf("SPK Number: *%v*\n", waReqData.HelpdeskTicketName))
	}
	if waReqData.CallCenterMessage != "" {
		sb.WriteString(fmt.Sprintf("Hasil feedback: %v\n", waReqData.CallCenterMessage))
	}

	// Add link & signature
	sb.WriteString("\n_*Untuk melihat lebih detail hasil FU tim Call Center, dapat dilihat pada link berikut:_\n")
	sb.WriteString(fmt.Sprintf("%v:%v/cc/resultFU?data=%v_%v_%v\n",
		data.Config.Default.WoDetailServer,
		data.Config.Default.WoDetailPort,
		waReqData.ID,
		waReqData.WoNumber,
		utils.GenerateRandomString(50),
	))

	// CC to
	var mentionedJIDs []string
	var mentionedTexts []string
	if waReqData.NextFollowUpTo != "" {
		phoneNumbers := strings.Split(waReqData.NextFollowUpTo, ",") // Split by comma

		for _, phone := range phoneNumbers {
			trimmedPhone := strings.TrimSpace(phone) // Remove extra spaces
			waJID := fmt.Sprintf("%s@s.whatsapp.net", trimmedPhone)
			mentionedJIDs = append(mentionedJIDs, waJID)
			mentionedTexts = append(mentionedTexts, fmt.Sprintf("@%s", trimmedPhone)) // Format for message
		}

		// Construct CC text with mentions
		sb.WriteString("\nCc: ")
		for i, text := range mentionedTexts {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(text)
		}
		sb.WriteString("\n")
	}

	// Always add the closing message
	sb.WriteString(fmt.Sprintf("\n~Regards, Call Center Team *%v*", data.Config.Default.PT))

	// Send message (with or without mentions)
	msgToSend := sb.String()

	// Initialize ContextInfo
	contextInfo := &waProto.ContextInfo{}
	if quotedMsg != nil {
		contextInfo = quotedMsg // Retain the quoted message context
	}

	// Add mentions if available
	if len(mentionedJIDs) > 0 {
		contextInfo.MentionedJID = mentionedJIDs
	}

	// Create message
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &msgToSend,
			ContextInfo: contextInfo, // Ensure ContextInfo is always included
		},
	}

	// Send the message
	_, err = data.WhatsappClient.SendMessage(context.Background(), jid, msg)
	if err != nil {
		log.Printf("[ERROR] JID: %v, Failed to send feedback message: %v", jid, err)
	}

}

func isNotFeedbackYet(msg string) bool {
	// Convert message to lowercase to make it case-insensitive
	msg = strings.ToLower(msg)

	// List of phrases to match
	excludeSentences := []string{
		"ada feedback", "ada feedbck", "difeedback", "dibalas", "dapat feedback", "dapet feedback", "adafeedback",
		"ada respon", "ada response", "respon", "response",
	}

	// Construct regex pattern dynamically to support "belum" OR "tidak"
	pattern := `\b(belum|tidak)\b.*(\b` + strings.Join(excludeSentences, `\b|\b`) + `\b)`

	// Check for matches
	matched, _ := regexp.MatchString(pattern, msg)

	return matched
}

// Helper function to send error messages
func sendErrorMessageHelper(data FeedbackTriggerData, jid types.JID, quotedMsg *waProto.ContextInfo, requestorString, errorMessage string) {
	var sb strings.Builder
	if requestorString != "" {
		sb.WriteString(fmt.Sprintf("Kami mohon maaf kepada *+%v* atas request: _%v_, karena terjadi kesalahan saat mendapatkan hasil feedback tim *Call Center* dengan PIC merchant. 🙏🏻\n",
			requestorString,
			data.RequestInWhatsapp,
		))
	} else {
		sb.WriteString(fmt.Sprintf("Kami mohon maaf atas request: _%v_, karena terjadi kesalahan saat mendapatkan hasil feedback tim *Call Center* dengan PIC merchant. 🙏🏻\n",
			data.RequestInWhatsapp,
		))
	}
	sb.WriteString(fmt.Sprintf("⚠ Error: %v\n", errorMessage))
	sb.WriteString(fmt.Sprintf("\nSilahkan hubungi *IT Support +%v* untuk informasi lebih lanjut.", data.Config.Whatsmeow.WaSupport))

	// Send error message
	msgToSend := sb.String()
	_, err := data.WhatsappClient.SendMessage(context.Background(), jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &msgToSend,
			ContextInfo: quotedMsg,
		},
	})
	if err != nil {
		log.Printf("[ERROR] JID: %v, Failed to send error message: %v", jid, err)
	}
}

// Update data in ODOO
func updateDatainODOO(data UpdatedODOODataTriggerItem) {
	var waReqData models.WaRequest
	var csData models.CS

	if err := data.Database.Table(data.Config.Db.TbWaReq).
		Where("id = ? AND is_done = ? AND keterangan = ?",
			data.TaskID,
			true,
			"Need for being updated to ODOO",
		).
		First(&waReqData).
		Error; err != nil {
		log.Printf("[ERROR] while trying to search data for being updated in ODOO %v", err)
		return
	}

	if err := data.Database.Table(data.Config.Db.TbUser).
		Where("id = ?", waReqData.TempCS).
		First(&csData).
		Error; err != nil {
		log.Printf("[ERROR] while trying to search data for cs user in DB %v", err)
		return
	}

	odooModel := "project.task"
	odooParams := map[string]interface{}{
		"model": odooModel,
		"id":    waReqData.ID,
	}

	if waReqData.IsReschedule {
		if waReqData.PlanDate == nil {
			odooParams["planned_date_begin"] = nil
			odooParams["planned_date_end"] = nil
		} else {
			startOfDay := time.Date(waReqData.PlanDate.Year(), waReqData.PlanDate.Month(), waReqData.PlanDate.Day(), 8, 10, 0, 0, waReqData.PlanDate.Location())
			endOfDay := time.Date(waReqData.PlanDate.Year(), waReqData.PlanDate.Month(), waReqData.PlanDate.Day(), 23, 59, 59, 0, waReqData.PlanDate.Location())
			adjustedStartOfDay := startOfDay.Add(-7 * time.Hour)
			adjustedEndOfDay := endOfDay.Add(-7 * time.Hour)
			odooParams["planned_date_begin"] = adjustedStartOfDay.Format("2006-01-02 15:04:05")
			odooParams["planned_date_end"] = adjustedEndOfDay.Format("2006-01-02 15:04:05")
		}
	}

	if waReqData.CallCenterMessage != "" {
		if waReqData.IsReschedule {
			var plannedDateStr string
			if waReqData.PlanDate != nil {
				plannedDateStr = waReqData.PlanDate.Format("2006-01-02")
			} else {
				plannedDateStr = "N/A"
			}

			odooParams["x_message_call"] = fmt.Sprintf("[RE-SCHEDULE]; Scheduled Date: %v; PIC: %v; PIC Phone Number: %v; Last Call by Call Center: %v; %v ~%v",
				plannedDateStr,
				waReqData.PicMerchant,
				waReqData.PicPhone,
				waReqData.UpdatedAt.Format("2006-01-02 15:04:05"),
				waReqData.CallCenterMessage,
				csData.Username)
		} else {
			odooParams["x_message_call"] = fmt.Sprintf("Last Call by Call Center: %v; %v ~%v",
				waReqData.UpdatedAt.Format("2006-01-02 15:04:05"),
				waReqData.CallCenterMessage,
				csData.Username,
			)
		}
	}

	if waReqData.PicMerchant != "" && waReqData.PicPhone != "" {
		odooParams["x_pic_merchant"] = waReqData.PicMerchant
		odooParams["x_pic_phone"] = waReqData.PicPhone
	}

	payload := map[string]interface{}{
		"jsonrpc": data.Config.ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ERROR] while trying to marshal json payload: %v", err)
		return
	}

	err = updateODOOData(data.Config, string(payloadBytes))
	if err != nil {
		log.Print(err)
		return
	}

	if err := data.Database.Table(data.Config.Db.TbWaReq).
		Where("id = ? AND keterangan = ?",
			data.TaskID,
			"Need for being updated to ODOO",
		).
		Updates(map[string]interface{}{
			"is_done":         true,
			"updated_to_odoo": true,
			"keterangan":      "Success updated to ODOO",
		}).
		Error; err != nil {
		log.Print(err)
		return
	}
}

func updateODOOData(config *config.YamlConfig, req string) error {
	urlUpdateData := config.ApiODOO.UrlUpdateData

	maxRetriesStr := config.ApiODOO.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		return fmt.Errorf("invalid ODOO_MAX_RETRY value: %v", err)
	}

	retryDelayStr := config.ApiODOO.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		return fmt.Errorf("invalid ODOO_RETRY_DELAY value: %v", err)
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlUpdateData, bytes.NewBufferString(req))
		if err != nil {
			return fmt.Errorf("error creating request: %v", err)
		}

		request.Header.Set("Content-Type", "application/json")

		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}

		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		response, err = client.Do(request)
		if err != nil {
			log.Printf("error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return err
		}

		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return err
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("POST request failed with status code: %v", response.StatusCode)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	// log.Print("Response Body:", string(body))

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return fmt.Errorf("error parsing JSON Response: %v", err)
	}

	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	if result, exists := jsonResponse["result"]; exists && result != nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if status, ok := resultMap["status"].(float64); ok {
				if int(status) == 200 {
					if resultMap["success"] == true && resultMap["response"] == true && resultMap["message"] == "Success" {
						// log.Print("ODOO data update successful.")
						return nil
					} else {
						errorMsg := fmt.Sprintf("odoo error: [status]%v; [success: %v]; [response: %v]; [message: %v]",
							status, resultMap["success"], resultMap["response"], resultMap["message"])
						return errors.New(errorMsg)
					}
				} else {
					errorMsg := fmt.Sprintf("odoo error: [%v] %v, from json request: %v",
						status, resultMap["message"], req)
					return errors.New(errorMsg)
				}
			} else {
				errorMsg := fmt.Sprintf("expected status to be float64, but got: %v", resultMap["status"])
				return errors.New(errorMsg)
			}
		} else {
			return fmt.Errorf("result exists but is not a valid map: %v", result)
		}
	} else {
		return errors.New("missing or empty 'result' key in response")
	}
}
