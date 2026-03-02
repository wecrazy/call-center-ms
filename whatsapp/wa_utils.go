package whatsapp

import (
	"call_center_app/config"
	"call_center_app/models"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Boostport/mjml-go"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gopkg.in/gomail.v2"
)

type nullAbleTime struct {
	Time  time.Time
	Valid bool
}

type nullAbleString struct {
	String string
	Valid  bool
}

func (ns *nullAbleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ns.String = ""
		ns.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &ns.String); err != nil {
		return err
	}
	ns.Valid = true
	return nil
}

// Nullable Float Type (For ID fields)
type nullAbleFloat struct {
	Float float64
	Valid bool
}

func (nf *nullAbleFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		nf.Float = 0
		nf.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &nf.Float); err != nil {
		return err
	}
	nf.Valid = true
	return nil
}

// Nullable Interface (For arrays or mixed types)
type nullAbleInterface struct {
	Data  interface{}
	Valid bool
}

func (ni *nullAbleInterface) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ni.Data = nil
		ni.Valid = false
		return nil
	}

	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	ni.Data = temp
	ni.Valid = true
	return nil
}

func (ni nullAbleInterface) IsEmpty() bool {
	return !ni.Valid || ni.Data == nil
}

func (ni nullAbleInterface) ToIntSlice() []int {
	if ni.Data == nil || !ni.Valid {
		return []int{}
	}

	// Try to assert the data as a slice of interfaces
	if dataSlice, ok := ni.Data.([]interface{}); ok {
		intSlice := make([]int, len(dataSlice))
		for i, v := range dataSlice {
			// Convert each value to int
			if num, ok := v.(float64); ok {
				intSlice[i] = int(num) // Convert float64 to int
			}
		}
		return intSlice
	}

	// Return empty slice if conversion fails
	return []int{}
}

func (t *OdooTaskDataRequestItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTaskDataRequestItem // Create an alias to avoid recursion
	aux := &struct {
		SlaDeadline         interface{} `json:"x_sla_deadline"`
		CreateDate          interface{} `json:"create_date"`
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		PlanDate            interface{} `json:"planned_date_begin"`
		TimesheetLastStop   interface{} `json:"timesheet_timer_last_stop"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the auxiliary structure
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Function to parse time fields
	parseTimeField := func(value interface{}) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, fmt.Errorf("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	// Parse each time field separately
	var err error

	if t.PlanDate, err = parseTimeField(aux.PlanDate); err != nil {
		return fmt.Errorf("PlanDate: %v", err)
	}

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CreateDate, err = parseTimeField(aux.CreateDate); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.TimesheetLastStop, err = parseTimeField(aux.TimesheetLastStop); err != nil {
		return fmt.Errorf("TimesheetLastStop: %v", err)
	}

	return nil
}

func (t *OdooTicketSolvedPendingDataRequestItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTicketSolvedPendingDataRequestItem // Create an alias to avoid recursion
	aux := &struct {
		SlaDeadline         interface{} `json:"sla_deadline"`
		CreateDate          interface{} `json:"create_date"`
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		TimesheetLastStop   interface{} `json:"complete_datetime_wo"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the auxiliary structure
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Function to parse time fields
	parseTimeField := func(value interface{}) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, fmt.Errorf("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	// Parse each time field separately
	var err error

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CreateDate, err = parseTimeField(aux.CreateDate); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.TimesheetLastStop, err = parseTimeField(aux.TimesheetLastStop); err != nil {
		return fmt.Errorf("TimesheetLastStop: %v", err)
	}

	return nil
}

func containsJID(groupList []string, jid types.JID) bool {
	for _, group := range groupList {
		groupJID := types.NewJID(group, "g.us") // Convert string to types.JID
		if groupJID == jid {
			return true
		}
	}
	return false
}

func isValidRC(rc string, allowedRCs []string) bool {
	lowerRC := strings.ToLower(rc)
	for _, allowed := range allowedRCs {
		lowerAllowed := strings.ToLower(allowed)
		if fuzzy.Match(lowerRC, lowerAllowed) || strings.Contains(lowerRC, lowerAllowed) || strings.HasPrefix(lowerRC, lowerAllowed) {
			return true
		}
	}
	return false
}

func translateWeather(desc string) string {
	if indonesianDesc, found := weatherTranslations[desc]; found {
		return indonesianDesc
	}
	return desc // Default: return original if no translation is found
}

func uniqueSlice(input []string) []string {
	seen := make(map[string]bool) // Map to track unique values
	result := []string{}

	for _, val := range input {
		if !seen[val] { // If not already added
			seen[val] = true
			result = append(result, val) // Add to result slice
		}
	}

	return result
}

func getWeatherEmoji(description string) string {
	emojiMap := map[string]string{
		"clear sky":                    "☀️", // Langit cerah
		"few clouds":                   "🌤️", // Sedikit berawan
		"scattered clouds":             "⛅",  // Berawan tipis
		"broken clouds":                "🌥️", // Berawan tebal
		"overcast clouds":              "☁️", // Mendung
		"light rain":                   "🌦️", // Hujan rintik-rintik
		"moderate rain":                "🌧️", // Hujan sedang
		"heavy intensity rain":         "🌧️", // Hujan lebat
		"very heavy rain":              "⛈️", // Hujan sangat lebat
		"extreme rain":                 "🌊",  // Hujan ekstrem
		"freezing rain":                "❄️", // Hujan beku
		"shower rain":                  "🌦️", // Hujan deras sesekali
		"thunderstorm":                 "⛈️", // Badai petir
		"thunderstorm with light rain": "⛈️",
		"thunderstorm with rain":       "⛈️",
		"thunderstorm with heavy rain": "⛈️",
		"snow":                         "❄️", // Salju
		"light snow":                   "🌨️", // Salju ringan
		"heavy snow":                   "❄️", // Salju lebat
		"mist":                         "🌫️", // Berkabut
		"smoke":                        "💨",  // Berasap
		"haze":                         "🌫️", // Kabut asap
		"fog":                          "🌫️", // Kabut
		"sand":                         "🏜️", // Badai pasir
		"dust":                         "💨",  // Badai debu
		"volcanic ash":                 "🌋",  // Abu vulkanik
		"squalls":                      "💨",  // Angin kencang
		"tornado":                      "🌪️", // Tornado
	}

	// Default emoji if the description is not found
	if emoji, exists := emojiMap[description]; exists {
		return emoji
	}
	return "🌎" // Default emoji if condition is unknown
}

// ConvertMJMLToHTML converts MJML content to HTML using mjml-go
func ConvertMJMLToHTML(mjmlContent string) (string, error) {
	// Correctly call mjml.ToHTML
	output, err := mjml.ToHTML(context.Background(), mjmlContent, mjml.WithMinify(true))
	if err != nil {
		var mjmlError mjml.Error
		if errors.As(err, &mjmlError) {
			// Convert error details to a readable string format
			detailsStr := formatMJMLErrorDetails(mjmlError.Details)
			return "", fmt.Errorf("MJML conversion error: %s - %s", mjmlError.Message, detailsStr)
		}
		return "", err
	}
	return output, nil
}

// Helper function to format MJML error details
func formatMJMLErrorDetails(details []struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	TagName string `json:"tagName"`
}) string {
	var formattedDetails []string
	for _, detail := range details {
		formattedDetails = append(formattedDetails, fmt.Sprintf("Line %d: %s (Tag: %s)", detail.Line, detail.Message, detail.TagName))
	}
	return strings.Join(formattedDetails, "; ")
}

func SendMail(to []string, cc []string, subject string, mjmlBody string, attachments []EmailAttachment) error {
	// Convert MJML to HTML
	htmlBody, err := ConvertMJMLToHTML(mjmlBody)
	if err != nil {
		log.Printf("❌ Failed to convert MJML to HTML: %v", err)
		return err
	}

	config := config.GetConfig().Email

	d := gomail.NewDialer(config.Host, config.Port, config.Username, config.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("\"%s\" <%s>", "Service Report", config.Username))
	m.SetHeader("To", to...)
	if len(cc) > 0 {
		m.SetHeader("Cc", cc...)
	}
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	// Attachments
	if len(attachments) > 0 {
		for _, attachment := range attachments {
			if _, err := os.Stat(attachment.FilePath); err == nil {
				m.Attach(attachment.FilePath, gomail.Rename(attachment.NewFileName))
			} else {
				log.Printf("⚠️ File does not exist: %s", attachment.FilePath)
			}
		}
	}

	// Retry logic for sending emails
	for i := 0; i < config.MaxRetry; i++ {
		err = d.DialAndSend(m)
		if err == nil {
			log.Printf("📧 Email sent successfully on attempt %d!", i+1)
			return nil
		}

		log.Printf("⚠ Attempt %d/%d failed to send email: %v", i+1, config.MaxRetry, err)
		time.Sleep(time.Duration(config.RetryDelay) * time.Second)
	}

	return err
}

func (h *WhatsmeowHandler) sendAttachCannotFUbyCC(jid types.JID, requestType string) {
	var dbData []models.CannotFollowUp
	if err := h.Database.Model(&models.CannotFollowUp{}).Where("request_type = ?", requestType).Find(&dbData).Error; err != nil {
		log.Print(err)
		return
	}

	if len(dbData) == 0 {
		return
	}

	mentions := []string{
		h.YamlCfg.Whatsmeow.WaTetty,
		h.YamlCfg.Whatsmeow.WaBuLina,
	}

	var mentionJIDs []string
	var mentionTags []string
	for _, num := range mentions {
		jid := num + "@s.whatsapp.net"
		mentionJIDs = append(mentionJIDs, jid)
		mentionTags = append(mentionTags, "@"+num)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠ Kami mohon maaf, karena *%d* data %v tidak dapat diFollow up oleh tim Call Center.\n", len(dbData), requestType))
	sb.WriteString("Berikut kami sertakan rincian datanya.")
	caption := sb.String()
	taggedMessage := fmt.Sprintf("%s\n\nCc: %s\n\n~ Regards, Call Center Team *%v*", caption, strings.Join(mentionTags, " "), h.YamlCfg.Default.PT)

	selectedMainDir, err := h.findValidDirectory([]string{
		"public/file/cannot_fu_record",
		"../public/file/cannot_fu_record",
		"../../public/file/cannot_fu_record",
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	excelFileName := fmt.Sprintf("(%v)RincianDataTidakDapatDiFollowUpOlehTimCC.xlsx", time.Now().Format("02Jan2006-15_04_05"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	columns := []struct {
		ColIndex string
		ColTitle string
		ColSize  float64
	}{
		{"A", "Request Type", 35},
		{"B", "WO Number", 35},
		{"C", "Ticket Subject", 55},
		{"D", "MID", 35},
		{"E", "TID", 35},
		{"F", "Task Type", 55},
		{"G", "Ticket Type", 55},
		{"H", "Worksheet Template", 55},
		{"I", "Alasan tidak dapat di Follow Up oleh Tim Call Center", 85},
	}

	f := excelize.NewFile()

	masterSheet := "CannotFollowUp"
	_, err = f.NewSheet(masterSheet)
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	style, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	for _, column := range columns {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(masterSheet, cell, column.ColTitle)
		f.SetColWidth(masterSheet, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(masterSheet, cell, cell, style)
	}

	batchSize := 100
	rowIndex := 2
	totalRecords := len(dbData)
	woDetailServer := fmt.Sprintf("%v:%d", h.YamlCfg.Default.WoDetailServer, h.YamlCfg.Default.WoDetailPort)

	for start := 0; start < totalRecords; start += batchSize {
		end := start + batchSize
		if end > totalRecords {
			end = totalRecords
		}

		currentBatch := dbData[start:end]

		for _, record := range currentBatch {

			var woDetailLink string
			if record.WoNumber != "" {
				woDetailLink = fmt.Sprintf("%v/projectTask/detailWO?wo_number=%v", woDetailServer, record.WoNumber)
				if err := f.SetCellHyperLink(masterSheet, fmt.Sprintf("B%d", rowIndex), woDetailLink, "External"); err != nil {
					h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
					return
				}
				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), record.WoNumber)
			} else {
				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), "N/A")
			}
			f.SetCellValue(masterSheet, fmt.Sprintf("A%d", rowIndex), defaultIfEmpty(record.RequestType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("C%d", rowIndex), defaultIfEmpty(record.TicketSubject, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("D%d", rowIndex), defaultIfEmpty(record.Mid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("E%d", rowIndex), defaultIfEmpty(record.Tid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("F%d", rowIndex), defaultIfEmpty(record.TaskType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("G%d", rowIndex), defaultIfEmpty(record.TicketType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("H%d", rowIndex), defaultIfEmpty(record.WorksheetTemplate, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("I%d", rowIndex), defaultIfEmpty(record.Message, "N/A"))

			// Apply Styles
			for col := 'A'; col <= 'I'; col++ {
				cell := fmt.Sprintf("%c%d", col, rowIndex)
				f.SetCellStyle(masterSheet, cell, cell, style)
			}

			rowIndex++
		}
	}

	err = f.AutoFilter(masterSheet, "A1:I1", []excelize.AutoFilterOptions{})
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	/* PIVOT MIDTID */
	pivotDataRange := masterSheet + "!$A$1:$I$" + fmt.Sprintf("%d", rowIndex-1)
	pivotSheet := "NEED VALID PHONE NUMBER"
	_, err = f.NewSheet(pivotSheet)
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}
	pivotRange := fmt.Sprintf("%v!A5:O200", pivotSheet)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: pivotRange,
		Rows: []excelize.PivotTableField{
			{Data: "MID"},
			{Data: "TID"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Ticket Subject",
				Name:     fmt.Sprintf("Total Try to Get Valid Phone Number from MIDTID @%v", time.Now().Format("02/Jan/2006 15:04:05")),
				Subtotal: "Count",
			},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	f.SetCellValue(pivotSheet, "A1", "Mohon list MIDTID berikut dicantumkan nomor PIC yang valid (terdaftar di Whatsapp) di ODOO Sales/Orders/Customers")

	f.DeleteSheet("Sheet1")

	if err := f.SaveAs(excelFilePath); err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err))
		return
	}

	// Read file contents
	fileData, err := os.ReadFile(excelFilePath)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to read file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal membaca file untuk dikirim: %v.", err))
		return
	}

	// Get file information
	fileInfo, err := os.Stat(excelFilePath)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to get file info: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mendapatkan info file: %v.", err))
		return
	}

	// Upload file to WhatsApp servers
	uploaded, err := h.Client.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to upload file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mengunggah file: %v.", err))
		return
	}

	// Validate if upload was successful
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		log.Printf("[ERROR] JID: %v, upload did not return a valid URL or DirectPath", jid)
		h.sendWhatsAppMessage(jid, "⚠ Gagal mengunggah file, URL tidak valid.")
		return
	}

	fileName := fileInfo.Name()

	// Sending the document message
	_, err = h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Caption:           proto.String(taggedMessage),
			FileName:          proto.String(fileName),
			Mimetype:          proto.String("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			FileSHA256:        uploaded.FileSHA256,
			FileEncSHA256:     uploaded.FileEncSHA256, // Include encryption hash
			FileLength:        &uploaded.FileLength,
			MediaKey:          uploaded.MediaKey,
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()), // Ensure timestamp is included
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentionJIDs, // ✅ Mentions will work!
			},
		},
	})

	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to send file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mengirim file laporan: %v", err))
		return
	}

	// emailAttachments := []EmailAttachment{
	// 	{
	// 		FilePath:    excelFilePath,
	// 		NewFileName: excelFileName,
	// 	},
	// }
	// emailSubject := fmt.Sprintf("Invalid Whatsapp Phone Number from MIDTID List %v", time.Now().Format("02 Jan 2006 @15:04:05"))
	// mjmlTemplate := fmt.Sprintf(`
	// 	<mjml>
	// 	<mj-head>
	// 		<mj-preview>MIDTID LIST INVALID WHATSAPP PHONE NUMBER</mj-preview>
	// 		<mj-style inline="inline">
	// 		.body-section {
	// 			background-color: #ffffff;
	// 			padding: 30px;
	// 			border-radius: 12px;
	// 			box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
	// 		}
	// 		.footer-text {
	// 			color: #6b7280;
	// 			font-size: 12px;
	// 			text-align: center;
	// 			padding-top: 10px;
	// 			border-top: 1px solid #e5e7eb;
	// 		}
	// 		.header-title {
	// 			font-size: 66px;
	// 			font-weight: bold;
	// 			color: #1E293B;
	// 			text-align: left;
	// 		}
	// 		.cta-button {
	// 			background-color: #6D28D9;
	// 			color: #ffffff;
	// 			padding: 12px 24px;
	// 			border-radius: 8px;
	// 			font-size: 16px;
	// 			font-weight: bold;
	// 			text-align: center;
	// 			display: inline-block;
	// 		}
	// 		.email-info {
	// 			color: #374151;
	// 			font-size: 16px;
	// 		}
	// 		</mj-style>
	// 	</mj-head>

	// 	<mj-body background-color="#f8fafc">
	// 		<!-- Main Content -->
	// 		<mj-section css-class="body-section" padding="20px">
	// 		<mj-column>
	// 			<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear All,</mj-text>
	// 			<mj-text font-size="16px" color="#4B5563" line-height="1.6">
	// 			We would like to attach a report listing all MIDTIDs that do not have a valid WhatsApp phone number as of %v.<br/><br/>
	// 			Soon,<br/>
	// 			please input the valid WhatsApp phone number of the PIC merchant in <b>ODOO Sales/Orders/Customers</b>.
	// 			</mj-text>

	// 			<mj-divider border-color="#e5e7eb"></mj-divider>

	// 			<mj-text font-size="16px" color="#374151">
	// 			Best Regards,<br>
	// 			<b><i>%v</i></b>
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 		<!-- Footer -->
	// 		<mj-section>
	// 		<mj-column>
	// 			<mj-text css-class="footer-text">
	// 			⚠ This is an automated email. Please do not reply directly.
	// 			</mj-text>
	// 			<mj-text font-size="12px" color="#6b7280">
	// 			<b>Call Center Team</b><br>
	// 			<!--
	// 			<br>
	// 			<a href="wa.me/%v">
	// 			📞 Support
	// 			</a>
	// 			-->
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 	</mj-body>
	// 	</mjml>
	// 	`,
	// 	time.Now().Format("02 January 2006"),
	// 	h.YamlCfg.Default.PT,
	// 	h.YamlCfg.Whatsmeow.WaBot,
	// )

	// emailTo := h.YamlCfg.Email.To
	// emailCc := h.YamlCfg.Email.Cc

	// err = SendMail(emailTo, emailCc, emailSubject, mjmlTemplate, emailAttachments)
	// if err != nil {
	// 	h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mengirim file laporan ke email: %v", err))
	// 	return
	// }

	log.Printf("[DEBUG] Uploaded File - JID: %v, URL: %s, DirectPath: %s, FileLength: %d",
		jid, uploaded.URL, uploaded.DirectPath, uploaded.FileLength)
	log.Printf("[SUCCESS] File sent successfully - JID: %v, FileName: %s", jid, fileName)
}

func (h *WhatsmeowHandler) sendWhatsAppMessageWithMentions(jid types.JID, mentions []string, message string) {
	var mentionedJIDs []string
	var mentionTags []string

	// Process mentions
	for _, num := range mentions {
		mentionJID := num + "@s.whatsapp.net" // Convert number to WhatsApp JID string
		mentionedJIDs = append(mentionedJIDs, mentionJID)
		mentionTags = append(mentionTags, "@"+num)
	}

	// Append mentions to message
	taggedMessage := message
	if len(mentionTags) > 0 {
		taggedMessage += fmt.Sprintf("\n\nCc: %s\n\n~ Regards, Call Center Team *%v*", strings.Join(mentionTags, " "), h.YamlCfg.Default.PT)
	}

	// Send message with mentions
	_, err := h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: &taggedMessage,
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	})
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to send message with mentions: %v", jid, err)
		return
	}
}

func (h *WhatsmeowHandler) sendAttachWithStanzaCannotFUbyCC(v *events.Message, stanzaID, originalSenderJID, requestType string, mentions []string) {
	var dbData []models.CannotFollowUp
	if err := h.Database.Model(&models.CannotFollowUp{}).Where("request_type = ?", requestType).Find(&dbData).Error; err != nil {
		log.Print(err)
		return
	}

	var mentionJIDs []string
	var mentionTags []string
	for _, num := range mentions {
		jid := num + "@s.whatsapp.net"
		mentionJIDs = append(mentionJIDs, jid)
		mentionTags = append(mentionTags, "@"+num)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠ Kami mohon maaf, karena *%d* data %v tidak dapat diFollow up oleh tim Call Center.\n", len(dbData), requestType))
	sb.WriteString("Berikut kami sertakan rincian datanya.")
	caption := sb.String()
	taggedMessage := fmt.Sprintf("%s\n\nCc: %s\n\n~ Regards, Call Center Team *%v*", caption, strings.Join(mentionTags, " "), h.YamlCfg.Default.PT)

	selectedMainDir, err := h.findValidDirectory([]string{
		"public/file/cannot_fu_record",
		"../public/file/cannot_fu_record",
		"../../public/file/cannot_fu_record",
	})
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	excelFileName := fmt.Sprintf("(%v)RincianDataTidakDapatDiFollowUpOlehTimCC.xlsx", time.Now().Format("02Jan2006-15_04_05"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	columns := []struct {
		ColIndex string
		ColTitle string
		ColSize  float64
	}{
		{"A", "Request Type", 35},
		{"B", "WO Number", 35},
		{"C", "Ticket Subject", 55},
		{"D", "MID", 35},
		{"E", "TID", 35},
		{"F", "Task Type", 55},
		{"G", "Ticket Type", 55},
		{"H", "Worksheet Template", 55},
		{"I", "Alasan tidak dapat di Follow Up oleh Tim Call Center", 85},
	}

	f := excelize.NewFile()

	masterSheet := "CannotFollowUp"
	_, err = f.NewSheet(masterSheet)
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	style, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	for _, column := range columns {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(masterSheet, cell, column.ColTitle)
		f.SetColWidth(masterSheet, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(masterSheet, cell, cell, style)
	}

	batchSize := 100
	rowIndex := 2
	totalRecords := len(dbData)
	woDetailServer := fmt.Sprintf("%v:%d", h.YamlCfg.Default.WoDetailServer, h.YamlCfg.Default.WoDetailPort)

	for start := 0; start < totalRecords; start += batchSize {
		end := start + batchSize
		if end > totalRecords {
			end = totalRecords
		}

		currentBatch := dbData[start:end]

		for _, record := range currentBatch {

			var woDetailLink string
			if record.WoNumber != "" {
				woDetailLink = fmt.Sprintf("%v/projectTask/detailWO?wo_number=%v", woDetailServer, record.WoNumber)
				if err := f.SetCellHyperLink(masterSheet, fmt.Sprintf("B%d", rowIndex), woDetailLink, "External"); err != nil {
					msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
					quotedMsg := &waProto.ContextInfo{
						StanzaID:      &stanzaID,
						Participant:   &originalSenderJID,
						QuotedMessage: v.Message,
					}

					_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
						ExtendedTextMessage: &waProto.ExtendedTextMessage{
							Text:        &msgToSend,
							ContextInfo: quotedMsg,
						},
					})

					if err != nil {
						log.Print(err)
						return
					}
					return
				}
				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), record.WoNumber)
			} else {
				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), "N/A")
			}
			f.SetCellValue(masterSheet, fmt.Sprintf("A%d", rowIndex), defaultIfEmpty(record.RequestType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("C%d", rowIndex), defaultIfEmpty(record.TicketSubject, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("D%d", rowIndex), defaultIfEmpty(record.Mid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("E%d", rowIndex), defaultIfEmpty(record.Tid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("F%d", rowIndex), defaultIfEmpty(record.TaskType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("G%d", rowIndex), defaultIfEmpty(record.TicketType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("H%d", rowIndex), defaultIfEmpty(record.WorksheetTemplate, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("I%d", rowIndex), defaultIfEmpty(record.Message, "N/A"))

			// Apply Styles
			for col := 'A'; col <= 'I'; col++ {
				cell := fmt.Sprintf("%c%d", col, rowIndex)
				f.SetCellStyle(masterSheet, cell, cell, style)
			}

			rowIndex++
		}
	}

	err = f.AutoFilter(masterSheet, "A1:I1", []excelize.AutoFilterOptions{})
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	/* PIVOT MIDTID */
	pivotDataRange := masterSheet + "!$A$1:$I$" + fmt.Sprintf("%d", rowIndex-1)
	pivotSheet := "NEED VALID PHONE NUMBER"
	_, err = f.NewSheet(pivotSheet)
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}
	pivotRange := fmt.Sprintf("%v!A3:O200", pivotSheet)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: pivotRange,
		Rows: []excelize.PivotTableField{
			{Data: "MID"},
			{Data: "TID"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Ticket Subject",
				Name:     fmt.Sprintf("Total Try to Get Valid Phone Number from MIDTID @%v", time.Now().Format("02/Jan/2006 15:04:05")),
				Subtotal: "Count",
			},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	f.SetCellValue(pivotSheet, "A1", "Mohon list MIDTID berikut dicantumkan nomor PIC yang valid (terdaftar di Whatsapp) di ODOO Sales/Orders/Customers")

	f.DeleteSheet("Sheet1")

	if err := f.SaveAs(excelFilePath); err != nil {
		msgToSend := fmt.Sprintf("⚠ Kami mohon maaf, gagal mengirim rincian untuk data %v yang tidak dapat difollow up: %v", requestType, err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	// Read file contents
	fileData, err := os.ReadFile(excelFilePath)
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Gagal membaca file untuk dikirim: %v.", err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	// Get file information
	fileInfo, err := os.Stat(excelFilePath)
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Gagal mendapatkan info file: %v.", err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	// Upload file to WhatsApp servers
	uploaded, err := h.Client.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Gagal mengunggah file: %v.", err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	// Validate if upload was successful
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		msgToSend := "⚠ Gagal mengunggah file, URL tidak valid."
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	fileName := fileInfo.Name()

	// Sending the document message
	_, err = h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Caption:           proto.String(taggedMessage),
			FileName:          proto.String(fileName),
			Mimetype:          proto.String("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			FileSHA256:        uploaded.FileSHA256,
			FileEncSHA256:     uploaded.FileEncSHA256, // Include encryption hash
			FileLength:        &uploaded.FileLength,
			MediaKey:          uploaded.MediaKey,
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()), // Ensure timestamp is included
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentionJIDs, // ✅ Mentions will work!
			},
		},
	})

	if err != nil {
		msgToSend := fmt.Sprintf("⚠ Gagal mengirim file laporan: %v", err)
		quotedMsg := &waProto.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
			ExtendedTextMessage: &waProto.ExtendedTextMessage{
				Text:        &msgToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			log.Print(err)
			return
		}
		return
	}

	// emailAttachments := []EmailAttachment{
	// 	{
	// 		FilePath:    excelFilePath,
	// 		NewFileName: excelFileName,
	// 	},
	// }
	// emailSubject := fmt.Sprintf("Invalid Whatsapp Phone Number from MIDTID List %v", time.Now().Format("02 Jan 2006 @15:04:05"))
	// mjmlTemplate := fmt.Sprintf(`
	// 	<mjml>
	// 	<mj-head>
	// 		<mj-preview>MIDTID LIST INVALID WHATSAPP PHONE NUMBER</mj-preview>
	// 		<mj-style inline="inline">
	// 		.body-section {
	// 			background-color: #ffffff;
	// 			padding: 30px;
	// 			border-radius: 12px;
	// 			box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
	// 		}
	// 		.footer-text {
	// 			color: #6b7280;
	// 			font-size: 12px;
	// 			text-align: center;
	// 			padding-top: 10px;
	// 			border-top: 1px solid #e5e7eb;
	// 		}
	// 		.header-title {
	// 			font-size: 66px;
	// 			font-weight: bold;
	// 			color: #1E293B;
	// 			text-align: left;
	// 		}
	// 		.cta-button {
	// 			background-color: #6D28D9;
	// 			color: #ffffff;
	// 			padding: 12px 24px;
	// 			border-radius: 8px;
	// 			font-size: 16px;
	// 			font-weight: bold;
	// 			text-align: center;
	// 			display: inline-block;
	// 		}
	// 		.email-info {
	// 			color: #374151;
	// 			font-size: 16px;
	// 		}
	// 		</mj-style>
	// 	</mj-head>

	// 	<mj-body background-color="#f8fafc">
	// 		<!-- Main Content -->
	// 		<mj-section css-class="body-section" padding="20px">
	// 		<mj-column>
	// 			<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear All,</mj-text>
	// 			<mj-text font-size="16px" color="#4B5563" line-height="1.6">
	// 			We would like to attach a report listing all MIDTIDs that do not have a valid WhatsApp phone number as of %v.<br/><br/>
	// 			Soon,<br/>
	// 			please input the valid WhatsApp phone number of the PIC merchant in <b>ODOO Sales/Orders/Customers</b>.
	// 			</mj-text>

	// 			<mj-divider border-color="#e5e7eb"></mj-divider>

	// 			<mj-text font-size="16px" color="#374151">
	// 			Best Regards,<br>
	// 			<b><i>%v</i></b>
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 		<!-- Footer -->
	// 		<mj-section>
	// 		<mj-column>
	// 			<mj-text css-class="footer-text">
	// 			⚠ This is an automated email. Please do not reply directly.
	// 			</mj-text>
	// 			<mj-text font-size="12px" color="#6b7280">
	// 			<b>Call Center Team</b><br>
	// 			<!--
	// 			<br>
	// 			<a href="wa.me/%v">
	// 			📞 Support
	// 			</a>
	// 			-->
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 	</mj-body>
	// 	</mjml>
	// 	`,
	// 	time.Now().Format("02 January 2006"),
	// 	h.YamlCfg.Default.PT,
	// 	h.YamlCfg.Whatsmeow.WaBot,
	// )

	// emailTo := h.YamlCfg.Email.To
	// emailCc := h.YamlCfg.Email.Cc

	// err = SendMail(emailTo, emailCc, emailSubject, mjmlTemplate, emailAttachments)
	// if err != nil {
	// 	h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mengirim file laporan ke email: %v", err))
	// 	return
	// }

	log.Printf("[DEBUG] Uploaded File - JID: %v, URL: %s, DirectPath: %s, FileLength: %d",
		v.Info.Chat, uploaded.URL, uploaded.DirectPath, uploaded.FileLength)
	log.Printf("[SUCCESS] File sent successfully - JID: %v, FileName: %s", v.Info.Chat, fileName)
}

// IsAOBRelated returns true if the input string contains "aob" (case-insensitive).
func IsAOBRelated(s string) bool {
	return strings.Contains(strings.ToLower(s), "aob")
}
