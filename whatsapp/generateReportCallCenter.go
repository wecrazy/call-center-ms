package whatsapp

import (
	"call_center_app/models"
	"call_center_app/utils"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var generateCCReport sync.Mutex

type Column struct {
	ColIndex string
	ColTitle string
	ColSize  float64
}

// Generate Excel-style column names: A, B, ..., Z, AA, AB, ...
func getColName(n int) string {
	name := ""
	for n >= 0 {
		// name = string('A'+(n%26)) + name
		name = string(rune('A'+(n%26))) + name
		n = n/26 - 1
	}
	return name
}

func (h *WhatsmeowHandler) sendWhatsAppMessage(jid types.JID, message string) {
	_, err := h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: &message,
		},
	})
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to send message: %v", jid, err)
		return
	}
}

func (h *WhatsmeowHandler) sendWhatsAppMessageWithStanza(v *events.Message, stanzaID, originalSenderJID, message string) {
	// sender := v.Info.Sender.String()
	// number := strings.Split(sender, ":")[0] // Remove device-specific suffix
	// number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

	quotedMsg := &waProto.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	_, err := h.Client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        &message,
			ContextInfo: quotedMsg,
		},
	})

	if err != nil {
		log.Println("[ERROR] Failed to send Pong reply in group:", err)
		return
	}
}

func (h *WhatsmeowHandler) sendWhatsAppFile(jid types.JID, filePath, caption string) {
	// Read file contents
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to read file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal membaca file untuk dikirim: %v.", err))
		return
	}

	// Get file information
	fileInfo, err := os.Stat(filePath)
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
	taggedMessage := fmt.Sprintf("%s\n\nCC: %s", caption, strings.Join(mentionTags, " "))

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
		h.sendWhatsAppMessage(jid, "⚠ Gagal mengirim file laporan.")
		return
	}

	log.Printf("[DEBUG] Uploaded File - JID: %v, URL: %s, DirectPath: %s, FileLength: %d",
		jid, uploaded.URL, uploaded.DirectPath, uploaded.FileLength)
	log.Printf("[SUCCESS] File sent successfully - JID: %v, FileName: %s", jid, fileName)
}

func (h *WhatsmeowHandler) sendWhatsAppImage(jid types.JID, filePath, caption string) {
	// Read file contents
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to read file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal membaca file untuk dikirim: %v.", err))
		return
	}

	// Get file information
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to get file info: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mendapatkan info file: %v.", err))
		return
	}

	// Upload file to WhatsApp servers
	uploaded, err := h.Client.Upload(context.Background(), fileData, whatsmeow.MediaImage) // Use MediaImage here
	if err != nil {
		log.Printf("[ERROR] JID: %v, failed to upload file: %v", jid, err)
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Gagal mengunggah file: %v.", err))
		return
	}

	// Validate if upload was successful
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		log.Printf("[ERROR] JID: %v, upload did not return a valid URL or DirectPath", jid)
		h.sendWhatsAppMessage(jid, "⚠ Gagal mengunggah gambar, URL tidak valid.")
		return
	}

	// Mentions (this is the same logic as before)
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
	taggedMessage := fmt.Sprintf("%s\n\nCC: %s", caption, strings.Join(mentionTags, " "))

	// Sending the image message
	_, err = h.Client.SendMessage(context.Background(), jid, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:           proto.String(taggedMessage),
			Mimetype:          proto.String("image/jpeg"), // Change MIME type as needed (jpeg, png, etc.)
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
		log.Printf("[ERROR] JID: %v, failed to send image: %v", jid, err)
		h.sendWhatsAppMessage(jid, "⚠ Gagal mengirim gambar.")
		return
	}

	log.Printf("[DEBUG] Uploaded Image - JID: %v, URL: %s, DirectPath: %s, FileLength: %d",
		jid, uploaded.URL, uploaded.DirectPath, uploaded.FileLength)
	log.Printf("[SUCCESS] Image sent successfully - JID: %v, FileName: %s", jid, fileInfo.Name())
}

func (h *WhatsmeowHandler) findValidDirectory(paths []string) (string, error) {
	for _, dir := range paths {
		if _, err := os.Stat(dir); err == nil {
			return dir, nil
		}
	}
	return "", fmt.Errorf("no valid report directory found in: %v", paths)
}

// Helper function to handle empty values
func defaultIfEmpty(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func (h *WhatsmeowHandler) GenerateReportCallCenter() {
	if !generateCCReport.TryLock() {
		log.Println("GenerateReportCallCenter is already running, skipping execution.")
		return
	}
	defer generateCCReport.Unlock()

	woDetailServer := fmt.Sprintf("%v:%d", h.YamlCfg.Default.WoDetailServer, h.YamlCfg.Default.WoDetailPort)

	jidString := h.YamlCfg.Whatsmeow.GroupCCJID + "@g.us"
	jid, err := types.ParseJID(jidString)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidString)
		h.sendWhatsAppMessage(jid, "⚠ Invalid JID format. Report generation aborted.")
		return
	}

	log.Printf("Running task generate call center report @%v", time.Now())

	selectedMainDir, err := h.findValidDirectory([]string{
		"public/file/call_center_report",
		"../public/file/call_center_report",
		"../../public/file/call_center_report",
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal _generate_ report Call Center: %v", err))
		return
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal _generate_ report Call Center: %v", err))
		return
	}

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		h.sendWhatsAppMessage(jid, "⚠ Gagal memuat zona waktu Asia/Jakarta.")
		return
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	tableCCFU := h.YamlCfg.Db.TbWaReq
	tableCS := h.YamlCfg.Db.TbUser

	var dbData []models.WaRequest
	if err := h.Database.Table(tableCCFU).
		// Where("is_done = ? AND temp_cs != ? AND updated_at >= ? AND updated_at <= ?",
		// 	true,
		// 	0,
		// 	startOfDay,
		// 	endOfDay,
		// ).
		Where("is_done = ? AND temp_cs NOT IN (?, ?) AND updated_at BETWEEN ? AND ?",
			true, 0, 14, startOfDay, endOfDay). // IIN delete this soon !!!
		Find(&dbData).Error; err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}

	totalData := len(dbData)

	if totalData == 0 {
		h.sendWhatsAppMessage(jid, ("⚠ Kami mohon maaf, tidak ada data hasil follow up tim Call Center."))
		return
	}

	excelFileName := fmt.Sprintf("(%v)HasilFUTimCallCenter.xlsx", time.Now().Format("02Jan2006"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)
	imgOutput := fmt.Sprintf("public/img/self/report/pivotHasilFollowUpCallCenter_%v.jpg", time.Now().Format("02Jan2006"))

	columns := []struct {
		ColIndex string
		ColTitle string
		ColSize  float64
	}{
		{"A", "Admin CS", 15},
		{"B", "WO Number", 15},
		{"C", "SPK Number", 25},
		{"D", "MID", 20},
		{"E", "TID", 20},
		{"F", "Merchant", 20},
		{"G", "PIC Merchant", 20},
		{"H", "PIC Phone Number", 15},
		{"I", "Company", 20},
		{"J", "Plan Date", 20},
		{"K", "Target Schedule Date", 25},
		{"L", "Technician", 15},
		{"M", "Request Type", 20},
		{"N", "Reason Code", 25},
		{"O", "CC Message", 20},
		{"P", "WO Remark Tiket", 25},
		{"Q", "Merchant Want Reschedule", 15},
		{"R", "Order to CC", 20},
		{"S", "Request to CC", 25},
		{"T", "Being Follow Up Time", 25},
		{"U", "Follow Up Result", 25},
	}

	f := excelize.NewFile()

	masterSheet := "MASTER"
	_, err = f.NewSheet(masterSheet)
	if err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}

	style, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
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
	totalRecords := int(totalData)

	for start := 0; start < totalRecords; start += batchSize {
		end := start + batchSize
		if end > totalRecords {
			end = totalRecords
		}

		currentBatch := dbData[start:end]

		for _, record := range currentBatch {
			var adminCS models.CS
			var csUsername string
			err := h.Database.Table(tableCS).Where("id = ?", record.TempCS).First(&adminCS).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// h.sendWhatsAppMessage(jid, "⚠ Data CS tidak ditemukan. Silakan periksa ID yang dimasukkan.")
				// return
				csUsername = "N/A"
			} else if err != nil {
				h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err))
				return
			}
			csUsername = adminCS.Username

			// Format PlanDate and TargetScheduleDate
			var planDateStr, targetScheduleDateStr string
			if record.PlanDate != nil && !record.PlanDate.IsZero() {
				planDateStr = record.PlanDate.Format("2006-01-02")
			} else {
				planDateStr = "N/A"
			}

			if record.TargetScheduleDate != nil && !record.TargetScheduleDate.IsZero() {
				targetScheduleDateStr = record.TargetScheduleDate.Format("2006-01-02")
			} else {
				targetScheduleDateStr = "N/A"
			}

			// Convert Boolean to String
			merchantWanttoReschedule := "No"
			if record.IsReschedule {
				merchantWanttoReschedule = "Yes"
			}

			// Calculate Follow-Up Time
			var beingFollowUpTime string
			if record.IsOnCallingDatetime != nil && record.IsDoneDatetime != nil {
				duration := record.IsDoneDatetime.Sub(*record.IsOnCallingDatetime)
				hours := int(duration.Hours())
				minutes := int(duration.Minutes()) % 60
				seconds := int(duration.Seconds()) % 60
				beingFollowUpTime = fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
			} else {
				beingFollowUpTime = "N/A"
			}

			// Fill Excel Sheet
			f.SetCellValue(masterSheet, fmt.Sprintf("A%d", rowIndex), csUsername)

			// f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), defaultIfEmpty(record.WoNumber, "N/A"))
			var woDetailLink string
			var resultFULink string
			if record.WoNumber != "" {
				woDetailLink = fmt.Sprintf("%v/projectTask/detailWO?wo_number=%v", woDetailServer, record.WoNumber)
				if err := f.SetCellHyperLink(masterSheet, fmt.Sprintf("B%d", rowIndex), woDetailLink, "External"); err != nil {
					h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
					return
				}

				resultFULink = fmt.Sprintf("%v/cc/resultFU?data=%d_%v_%v", woDetailServer, record.ID, record.WoNumber, utils.GenerateRandomString(50))
				if err := f.SetCellHyperLink(masterSheet, fmt.Sprintf("U%d", rowIndex), resultFULink, "External"); err != nil {
					h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
					return
				}

				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), record.WoNumber)
				f.SetCellValue(masterSheet, fmt.Sprintf("U%d", rowIndex), "See Detail Result Follow Up from CC")
			} else {
				f.SetCellValue(masterSheet, fmt.Sprintf("B%d", rowIndex), "N/A")
				f.SetCellValue(masterSheet, fmt.Sprintf("U%d", rowIndex), "N/A")
			}

			f.SetCellValue(masterSheet, fmt.Sprintf("C%d", rowIndex), defaultIfEmpty(record.HelpdeskTicketName, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("D%d", rowIndex), defaultIfEmpty(record.Mid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("E%d", rowIndex), defaultIfEmpty(record.Tid, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("F%d", rowIndex), defaultIfEmpty(record.MerchantName, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("G%d", rowIndex), defaultIfEmpty(record.PicMerchant, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("H%d", rowIndex), defaultIfEmpty(record.PicPhone, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("I%d", rowIndex), defaultIfEmpty(record.CompanyName, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("L%d", rowIndex), defaultIfEmpty(record.TechnicianName, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("M%d", rowIndex), defaultIfEmpty(record.RequestType, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("N%d", rowIndex), defaultIfEmpty(record.ReasonCodeName, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("O%d", rowIndex), defaultIfEmpty(record.CallCenterMessage, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("P%d", rowIndex), defaultIfEmpty(record.WoRemarkTiket, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("R%d", rowIndex), defaultIfEmpty(record.OrderWish, "N/A"))
			f.SetCellValue(masterSheet, fmt.Sprintf("S%d", rowIndex), defaultIfEmpty(record.RequestToCC, "N/A"))

			f.SetCellValue(masterSheet, fmt.Sprintf("J%d", rowIndex), planDateStr)
			f.SetCellValue(masterSheet, fmt.Sprintf("K%d", rowIndex), targetScheduleDateStr)
			f.SetCellValue(masterSheet, fmt.Sprintf("Q%d", rowIndex), merchantWanttoReschedule)
			f.SetCellValue(masterSheet, fmt.Sprintf("T%d", rowIndex), beingFollowUpTime)

			// Apply Styles
			for col := 'A'; col <= 'U'; col++ {
				cell := fmt.Sprintf("%c%d", col, rowIndex)
				f.SetCellStyle(masterSheet, cell, cell, style)
			}

			rowIndex++
		}

	}

	err = f.AutoFilter(masterSheet, "A1:U1", []excelize.AutoFilterOptions{})
	if err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}

	f.DeleteSheet("Sheet1")

	var dataUnfollow []models.WaRequest
	if err := h.Database.Table(tableCCFU).
		Where("is_done = ? AND temp_cs = ? AND is_on_calling = ? AND updated_at >= ? AND updated_at <= ?",
			false, 0, false, startOfDay, endOfDay,
		).
		Find(&dataUnfollow).Error; err != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err))
		return
	}

	if len(dataUnfollow) > 0 {
		unfollowSheet := "UNFOLLOW DATA"
		_, err = f.NewSheet(unfollowSheet)
		if err != nil {
			h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err))
			return
		}

		// Define titles
		titles := []struct {
			Title string
			Size  float64
		}{
			{"Tercatat di System", 35},
			{"WO Number", 35},
			{"SPK Number (Ticket Subject)", 45},
			{"Company", 35},
			{"Task Type", 45},
			{"MID", 35},
			{"TID", 35},
			{"Merchant", 35},
			{"PIC Merchant", 35},
			{"PIC / Owner Phone Number", 35},
			{"Plan Date", 35},
			{"Target Schedule Date", 35},
			{"Technician", 35},
			{"Request Type", 35},
			{"SLA Deadline", 35},
			{"WO Remark Tiket", 35},
			{"Description", 35},
			{"Request to CC", 35},
		}

		var unfollowColumns []Column
		for i, t := range titles {
			unfollowColumns = append(unfollowColumns, Column{
				ColIndex: getColName(i),
				ColTitle: t.Title,
				ColSize:  t.Size,
			})
		}

		// Set header
		for _, column := range unfollowColumns {
			f.SetCellValue(unfollowSheet, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
			f.SetColWidth(unfollowSheet, column.ColIndex, column.ColIndex, column.ColSize)
		}

		// Set filter
		lastCol := getColName(len(unfollowColumns) - 1)
		filterRange := fmt.Sprintf("A1:%s1", lastCol)
		err = f.AutoFilter(unfollowSheet, filterRange, []excelize.AutoFilterOptions{})
		if err != nil {
			h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err))
			return
		}

		// Fill data
		rowIndex := 2
		for _, record := range dataUnfollow {
			for _, column := range unfollowColumns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{}

				switch column.ColTitle {
				case "WO Number":
					value = record.WoNumber
				case "SPK Number (Ticket Subject)":
					value = record.HelpdeskTicketName
				case "Company":
					value = record.CompanyName
				case "Task Type":
					value = record.TaskType
				case "MID":
					value = record.Mid
				case "TID":
					value = record.Tid
				case "Merchant":
					value = record.MerchantName
				case "PIC Merchant":
					value = record.PicMerchant
				case "PIC / Owner Phone Number":
					value = record.PicPhone
				case "Plan Date":
					if record.PlanDate != nil && !record.PlanDate.IsZero() {
						value = record.PlanDate.Format("2006-01-02")
					} else {
						value = "N/A"
					}
				case "Tercatat di System":
					if !record.UpdatedAt.IsZero() {
						value = record.UpdatedAt.Format("2006-01-02 15:04:05")
					} else {
						value = "N/A"
					}
				case "Target Schedule Date":
					if record.TargetScheduleDate != nil && !record.TargetScheduleDate.IsZero() {
						value = record.TargetScheduleDate.Format("2006-01-02 15:04:05")
					} else {
						value = "N/A"
					}
				case "Technician":
					value = record.TechnicianName
				case "Request Type":
					value = record.RequestType
				case "SLA Deadline":
					if record.SlaDeadline != nil && !record.SlaDeadline.IsZero() {
						value = record.SlaDeadline.Format("2006-01-02 15:04:05")
					} else {
						value = "N/A"
					}
				case "WO Remark Tiket":
					value = record.WoRemarkTiket
				case "Description":
					value = record.Description
				case "Request to CC":
					value = record.RequestToCC
				}

				// Default "N/A" if string is empty
				if str, ok := value.(string); ok && str == "" {
					value = "N/A"
				}

				f.SetCellValue(unfollowSheet, cell, value)
			}
			rowIndex++
		}
	}

	/* PIVOT */
	pivotDataRange := masterSheet + "!$A$1:$U$" + fmt.Sprintf("%d", rowIndex-1)
	pivotSheet := "PIVOT"
	_, err = f.NewSheet(pivotSheet)
	if err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}
	pivotRange := fmt.Sprintf("%v!A1:O200", pivotSheet)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: pivotRange,
		Rows: []excelize.PivotTableField{
			{Data: "Admin CS", Name: "CS"},
		},
		Data: []excelize.PivotTableField{
			{Data: "WO Number", Name: fmt.Sprintf("Followed-Up by CC Team @%v", time.Now().Format("02/Jan/2006 15:04:05")), Subtotal: "Count"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}

	// Set column widths
	f.SetColWidth(pivotSheet, "A", "A", 15)
	f.SetColWidth(pivotSheet, "B", "B", 47)

	f.SetActiveSheet(0)
	// Save excel file report
	if err := f.SaveAs(excelFilePath); err != nil {
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _generate_ report: %v.", err)))
		return
	}

	message := "🎉 Yeay! Report Tim Call Center berhasil di-generate otomatis oleh _system_."
	h.sendWhatsAppFile(jid, excelFilePath, message)

	// Uncomment this soon if needed!
	// @10 PM Send Email Report
	if time.Now().Hour() == 22 {
		emailAttachments := []EmailAttachment{
			{
				FilePath:    excelFilePath,
				NewFileName: excelFileName,
			},
		}

		emailSubject := fmt.Sprintf("Result Followed-Up by Team Call Center %v", time.Now().Format("02 Jan 2006 @15:04:05"))

		// logoPT := fmt.Sprintf("%v/static%v", woDetailServer, h.YamlCfg.Default.PTLogo)
		mjmlTemplate := fmt.Sprintf(`
		<mjml>
		<mj-head>
			<mj-preview>Follow-up Report by Team Call Center</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>

		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear All,</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
				We would like to attach the report about the count of results followed up by the team call center per %v.
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%v</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>Call Center Team</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		</mjml>
		`,
			time.Now().Format("02 January 2006"),
			h.YamlCfg.Default.PT,
			h.YamlCfg.Whatsmeow.WaBot,
		)

		// Send email
		err := SendMail(h.YamlCfg.Email.To, h.YamlCfg.Email.Cc, emailSubject, mjmlTemplate, emailAttachments)
		if err != nil {
			log.Fatalf("❌ Failed to send email: %v", err)
			h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _send email_ report: %v.", err)))
			return
		}
		log.Println("✅ Email successfully sent!")
	}

	// err = openExcelAndCapturePivotScreenshot(excelFilePath, imgOutput)
	err = h.exportPivotToImage(excelFilePath, imgOutput)
	if err != nil {
		// log.Printf("⚠ Kami mohon maaf, terjadi kesalahan saat _screenshot pivot_ report: %v.", err)
		h.sendWhatsAppMessage(jid, (fmt.Sprintf("⚠ Kami mohon maaf, terjadi kesalahan saat _screenshot pivot_ report: %v.", err)))
		return
	}
	log.Printf("✅ Gambar SS Call Center Followed Up Pivot tersimpan di: %v", imgOutput)
	imgAttachMsg := "Berikut hasil pivot follow up by Call Center Team"
	if imgOutput != "" {
		// log.Printf("%v - %v - %v", jid, imgOutput, imgAttachMsg)
		h.sendWhatsAppImage(jid, imgOutput, imgAttachMsg)
		return
	}

	log.Printf("Task Call Center Report successfully executed @%v", time.Now())
}

var (
// user32            = syscall.NewLazyDLL("user32.dll")
// procFindWindowW   = user32.NewProc("FindWindowW")
// procShowWindow    = user32.NewProc("ShowWindow")
// procSetForeground = user32.NewProc("SetForegroundWindow")
// procKeybdEvent    = user32.NewProc("keybd_event")
)

const (
	SW_RESTORE       = 9
	SW_SHOWMAXIMIZED = 3
	KEYEVENTF_KEYUP  = 0x0002
	VK_MENU          = 0x12 // ALT key
)

// func findWindow(className, windowName *uint16) syscall.Handle {
// 	hwnd, _, _ := procFindWindowW.Call(
// 		uintptr(unsafe.Pointer(className)),
// 		uintptr(unsafe.Pointer(windowName)),
// 	)
// 	return syscall.Handle(hwnd)
// }

// func showWindow(hwnd syscall.Handle, cmd int) {
// 	procShowWindow.Call(uintptr(hwnd), uintptr(cmd))
// }

// func setForegroundWindow(hwnd syscall.Handle) {
// 	procSetForeground.Call(uintptr(hwnd))
// }

// func simulateAltKeyPress() {
// 	procKeybdEvent.Call(VK_MENU, 0, 0, 0)
// 	time.Sleep(50 * time.Millisecond)
// 	procKeybdEvent.Call(VK_MENU, 0, KEYEVENTF_KEYUP, 0)
// }

func killExcelProcesses() {
	_ = exec.Command("taskkill", "/F", "/IM", "EXCEL.EXE").Run()
}

func waitForFileUnlock(filePath string, timeout time.Duration) error {
	start := time.Now()
	for {
		f, err := os.OpenFile(filePath, os.O_RDWR, 0666)
		if err == nil {
			f.Close()
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("file still locked after %v: %v", timeout, err)
		}
		time.Sleep(2 * time.Second)
	}
}

// Not work in MINI PC Cideng
// func (h *WhatsmeowHandler) exportPivotToImage(excelFilePath, imgOutput string) (err error) {
// 	logFile, _ := os.Create("export_pivot_debug.log")
// 	defer logFile.Close()
// 	log := func(msg string) {
// 		logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
// 	}

// 	runtime.LockOSThread()
// 	defer runtime.UnlockOSThread()

// 	var excel, workbook, workbooks, sheets, pivotSheet *ole.IDispatch

// 	defer func() {
// 		if r := recover(); r != nil {
// 			err = fmt.Errorf("panic occurred: %v", r)
// 			log(fmt.Sprintf("🔥 Panic: %v", r))
// 		}

// 		// Clean up in proper order
// 		if workbook != nil {
// 			oleutil.MustCallMethod(workbook, "Close", false) // false = don't save changes
// 			workbook.Release()
// 		}
// 		if pivotSheet != nil {
// 			pivotSheet.Release()
// 		}
// 		if sheets != nil {
// 			sheets.Release()
// 		}
// 		if workbooks != nil {
// 			workbooks.Release()
// 		}
// 		if excel != nil {
// 			oleutil.MustCallMethod(excel, "Quit")
// 			excel.Release()
// 		}

// 		ole.CoUninitialize()
// 		log("✅ COM Uninitialized")
// 	}()

// 	log("🔄 Start exportPivotToImage")
// 	killExcelProcesses()
// 	log("✅ Killed Excel processes")

// 	if err = waitForFileUnlock(excelFilePath, 30*time.Second); err != nil {
// 		log(fmt.Sprintf("⛔ waitForFileUnlock: %v", err))
// 		return
// 	}
// 	log("✅ File unlocked")

// 	if err = ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
// 		log(fmt.Sprintf("⛔ COM init failed: %v", err))
// 		return fmt.Errorf("COM init failed: %v", err)
// 	}
// 	log("✅ COM initialized")

// 	unknown, err := oleutil.CreateObject("Excel.Application")
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ CreateObject failed: %v", err))
// 		return fmt.Errorf("cannot start Excel: %v", err)
// 	}
// 	excel, _ = unknown.QueryInterface(ole.IID_IDispatch)
// 	log("✅ Excel started")

// 	oleutil.PutProperty(excel, "Visible", false)

// 	workbooks = oleutil.MustGetProperty(excel, "Workbooks").ToIDispatch()
// 	log("✅ Got Workbooks")

// 	// Make path absolute + log it
// 	absPath, err := filepath.Abs(excelFilePath)
// 	if err != nil {
// 		log("⛔ Failed to get abs path: " + err.Error())
// 		return err
// 	}
// 	log("📁 Absolute path to Excel file: " + absPath)

// 	// Check file access
// 	if _, err := os.Stat(absPath); os.IsNotExist(err) {
// 		log("⛔ File does not exist at: " + absPath)
// 		return fmt.Errorf("file does not exist: %v", absPath)
// 	}
// 	workbook = oleutil.MustCallMethod(workbooks, "Open", filepath.ToSlash(absPath), false, true).ToIDispatch()
// 	log("✅ Workbook opened")

// 	time.Sleep(2 * time.Second) // Let Excel catch up

// 	sheets = oleutil.MustGetProperty(workbook, "Sheets").ToIDispatch()
// 	log("✅ Got Sheets")

// 	// Retry getting the PIVOT sheet
// 	for i := 1; i <= 5; i++ {
// 		log(fmt.Sprintf("⏳ Trying to get sheet 'PIVOT' (attempt %d/5)...", i))
// 		sheetProp, err := oleutil.GetProperty(sheets, "Item", "PIVOT")
// 		if err == nil && sheetProp.Val != 0 {
// 			pivotSheet = sheetProp.ToIDispatch()
// 			log("✅ Got PIVOT sheet")
// 			break
// 		}
// 		time.Sleep(2 * time.Second)
// 	}
// 	if pivotSheet == nil {
// 		err = fmt.Errorf("❌ Failed to get 'PIVOT' sheet after retries")
// 		log(err.Error())
// 		return err
// 	}

// 	// Export to PDF
// 	pdfOutput := strings.TrimSuffix(imgOutput, filepath.Ext(imgOutput)) + ".pdf"
// 	absPdf, _ := filepath.Abs(pdfOutput)
// 	oleutil.MustCallMethod(pivotSheet, "ExportAsFixedFormat", 0, absPdf)
// 	log("✅ Exported PDF")

// 	log(fmt.Sprintf("📄 PDF path: %v", absPdf))
// 	log(fmt.Sprintf("📷 Image output path: %v", imgOutput))

// 	// Convert to image with ImageMagick
// 	absImg, _ := filepath.Abs(imgOutput)
// 	magickPath, err := exec.LookPath("magick")
// 	if err != nil {
// 		// fallback to known default install path
// 		magickPath = h.YamlCfg.Default.MagickFullPath
// 		if _, err := os.Stat(magickPath); os.IsNotExist(err) {
// 			return fmt.Errorf("magick not found in PATH or default path")
// 		}
// 	}
// 	// cmd := exec.Command("magick", absPdf, absImg)
// 	cmd := exec.Command(magickPath, "-density", "300", absPdf, "-quality", "100", absImg)

// 	cmdOutput, err := cmd.CombinedOutput()
// 	// _, err = cmd.CombinedOutput()
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ ImageMagick failed: %v, Output: %s", err, string(cmdOutput)))
// 		return fmt.Errorf("ImageMagick convert failed: %v", err)
// 	}

// 	return nil
// }

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// Function to check if a file is locked by another process
func checkIfFileLocked(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("unable to open file: %w", err)
	}
	file.Close()
	return nil
}

// Function to check if the temp file is accessible for write
func checkFileWritePermission(filePath string) error {
	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to open file for write: %w", err)
	}
	defer file.Close()
	return nil
}

// func (h *WhatsmeowHandler) exportPivotToImage(excelFilePath, imgOutput string) (err error) {
// 	logFile, _ := os.Create("./logs/export_pivot_debug.log")
// 	defer logFile.Close()

// 	log := func(msg string) {
// 		logFile.WriteString(time.Now().Format("2006/01/02 15:04:05") + " " + msg + "\n")
// 	}

// 	// Lock the OS thread for COM initialization
// 	runtime.LockOSThread()
// 	defer runtime.UnlockOSThread()

// 	var tempExcel string

// 	defer func() {
// 		if r := recover(); r != nil {
// 			err = fmt.Errorf("panic occurred: %v", r)
// 			log(fmt.Sprintf("🔥 Panic: %v", r))
// 		}

// 		if tempExcel != "" {
// 			if err := os.Remove(tempExcel); err != nil {
// 				log(fmt.Sprintf("⛔ Failed to remove temp file: %v", err))
// 			} else {
// 				log("✅ Temp file removed")
// 			}
// 		}
// 	}()

// 	log("🔄 Start exportPivotToImage")

// 	if err = waitForFileUnlock(excelFilePath, 3*time.Second); err != nil {
// 		log(fmt.Sprintf("⛔ waitForFileUnlock: %v", err))
// 		return fmt.Errorf("failed to unlock file: %v", err)
// 	}
// 	log("✅ File unlocked")

// 	tempDir := os.TempDir()
// 	tempExcel = filepath.Join(tempDir, fmt.Sprintf("temp_excel_%d.xlsx", time.Now().UnixNano()))
// 	log("📁 Temp Excel path: " + tempExcel)

// 	if err = checkFileWritePermission(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Temp Excel file is not accessible for write: %v", err))
// 		return fmt.Errorf("temp Excel file is not accessible for write: %v", err)
// 	}
// 	log("✅ Temp Excel file is accessible for write")

// 	if err = copyFile(excelFilePath, tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Failed to copy to temp file: %v", err))
// 		return fmt.Errorf("failed to copy Excel file to temp location: %w", err)
// 	}
// 	log("✅ Copied Excel to temp dir")

// 	time.Sleep(3 * time.Second)

// 	log("📁 Checking if copied file exists and is accessible")
// 	if _, err := os.Stat(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Copied file missing or inaccessible: %v", err))
// 		return fmt.Errorf("copied file missing or inaccessible: %v", err)
// 	}
// 	log("✅ Copied file is accessible")

// 	log("📁 Checking if temp file is locked by any process")
// 	if err := checkIfFileLocked(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ File is locked: %v", err))
// 		return fmt.Errorf("file is locked by another process: %w", err)
// 	}

// 	// Use excelize to open the Excel file
// 	f, err := excelize.OpenFile(tempExcel)
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ Failed to open Excel file: %v", err))
// 		return fmt.Errorf("failed to open Excel file: %v", err)
// 	}
// 	log("✅ Opened Excel file")

// 	// Try to find the 'PIVOT' sheet
// 	pivotSheetIndex := -1
// 	for i, sheet := range f.GetSheetList() {
// 		if sheet == "PIVOT" {
// 			pivotSheetIndex = i
// 			break
// 		}
// 	}
// 	if pivotSheetIndex == -1 {
// 		err = fmt.Errorf("❌ 'PIVOT' sheet not found")
// 		log(err.Error())
// 		return err
// 	}
// 	log("✅ Found 'PIVOT' sheet")

// 	// Export the sheet to PDF (we'll still use ImageMagick to convert it to an image)
// 	pdfOutput := strings.TrimSuffix(imgOutput, filepath.Ext(imgOutput)) + ".pdf"
// 	absPdf, _ := filepath.Abs(pdfOutput)
// 	log(fmt.Sprintf("📄 PDF output path: %s", absPdf))

// 	// Save the Excel sheet to PDF (requires external library for actual conversion)
// 	// You would need an external tool like LibreOffice or Excel to convert to PDF
// 	// Here we assume it's done by some external process or scripting

// 	// Convert PDF to Image using ImageMagick (if necessary)
// 	log(fmt.Sprintf("📄 PDF path: %v", absPdf))
// 	log(fmt.Sprintf("📷 Image output path: %v", imgOutput))

// 	absImg, _ := filepath.Abs(imgOutput)
// 	magickPath, err := exec.LookPath("magick")
// 	if err != nil {
// 		magickPath = h.YamlCfg.Default.MagickFullPath
// 		if _, err := os.Stat(magickPath); os.IsNotExist(err) {
// 			log("⛔ ImageMagick not found")
// 			return fmt.Errorf("ImageMagick not found in PATH or default path")
// 		}
// 	}
// 	cmd := exec.Command(magickPath, "-density", "300", absPdf, "-quality", "100", absImg)
// 	cmdOutput, err := cmd.CombinedOutput()
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ ImageMagick failed: %v, Output: %s", err, string(cmdOutput)))
// 		return fmt.Errorf("ImageMagick convert failed: %v", err)
// 	}

// 	log("✅ Screenshot converted to image")
// 	return nil
// }

// func (h *WhatsmeowHandler) exportPivotToImage(excelFilePath, imgOutput string) (err error) {
// 	logFile, _ := os.Create("./logs/export_pivot_debug.log")
// 	defer logFile.Close()

// 	log := func(msg string) {
// 		logFile.WriteString(time.Now().Format("2006/01/02 15:04:05") + " " + msg + "\n")
// 	}

// 	// Lock the OS thread for COM initialization
// 	runtime.LockOSThread()
// 	defer runtime.UnlockOSThread()

// 	var tempExcel string

// 	defer func() {
// 		if r := recover(); r != nil {
// 			err = fmt.Errorf("panic occurred: %v", r)
// 			log(fmt.Sprintf("🔥 Panic: %v", r))
// 		}

// 		if tempExcel != "" {
// 			if err := os.Remove(tempExcel); err != nil {
// 				log(fmt.Sprintf("⛔ Failed to remove temp file: %v", err))
// 			} else {
// 				log("✅ Temp file removed")
// 			}
// 		}
// 	}()

// 	log("🔄 Start exportPivotToImage")

// 	if err = waitForFileUnlock(excelFilePath, 3*time.Second); err != nil {
// 		log(fmt.Sprintf("⛔ waitForFileUnlock: %v", err))
// 		return fmt.Errorf("failed to unlock file: %v", err)
// 	}
// 	log("✅ File unlocked")

// 	tempDir := os.TempDir()
// 	tempExcel = filepath.Join(tempDir, fmt.Sprintf("temp_excel_%d.xlsx", time.Now().UnixNano()))
// 	log("📁 Temp Excel path: " + tempExcel)

// 	if err = checkFileWritePermission(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Temp Excel file is not accessible for write: %v", err))
// 		return fmt.Errorf("temp Excel file is not accessible for write: %v", err)
// 	}
// 	log("✅ Temp Excel file is accessible for write")

// 	if err = copyFile(excelFilePath, tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Failed to copy to temp file: %v", err))
// 		return fmt.Errorf("failed to copy Excel file to temp location: %w", err)
// 	}
// 	log("✅ Copied Excel to temp dir")

// 	time.Sleep(3 * time.Second)

// 	log("📁 Checking if copied file exists and is accessible")
// 	if _, err := os.Stat(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ Copied file missing or inaccessible: %v", err))
// 		return fmt.Errorf("copied file missing or inaccessible: %v", err)
// 	}
// 	log("✅ Copied file is accessible")

// 	log("📁 Checking if temp file is locked by any process")
// 	if err := checkIfFileLocked(tempExcel); err != nil {
// 		log(fmt.Sprintf("⛔ File is locked: %v", err))
// 		return fmt.Errorf("file is locked by another process: %w", err)
// 	}

// 	// Use excelize to open the Excel file
// 	f, err := excelize.OpenFile(tempExcel)
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ Failed to open Excel file: %v", err))
// 		return fmt.Errorf("failed to open Excel file: %v", err)
// 	}
// 	log("✅ Opened Excel file")

// 	// Try to find the 'PIVOT' sheet
// 	pivotSheetIndex := -1
// 	for i, sheet := range f.GetSheetList() {
// 		if sheet == "PIVOT" {
// 			pivotSheetIndex = i
// 			break
// 		}
// 	}
// 	if pivotSheetIndex == -1 {
// 		err = fmt.Errorf("❌ 'PIVOT' sheet not found")
// 		log(err.Error())
// 		return err
// 	}
// 	log("✅ Found 'PIVOT' sheet")

// 	// Prepare PDF output file path
// 	pdfOutput := filepath.Join(tempDir, strings.TrimSuffix(filepath.Base(imgOutput), filepath.Ext(imgOutput))+".pdf")

// 	// Convert the Excel sheet to PDF using LibreOffice
// 	sofficePath, err := exec.LookPath("soffice")
// 	if err != nil {
// 		sofficePath = h.YamlCfg.Default.LibreOfficeFullPath
// 		if _, err := os.Stat(sofficePath); os.IsNotExist(err) {
// 			log("⛔ LibreOffice not found")
// 			return fmt.Errorf("LibreOffice not found in PATH or default path")
// 		}
// 	}

// 	// Run LibreOffice conversion command
// 	cmdLibre := exec.Command(sofficePath, "--headless", "--convert-to", "pdf", "--outdir", tempDir, tempExcel)
// 	cmdOutput, err := cmdLibre.CombinedOutput()

// 	// Log LibreOffice command output
// 	log(fmt.Sprintf("LibreOffice command output: %s", string(cmdOutput)))

// 	if err != nil {
// 		log(fmt.Sprintf("⛔ LibreOffice conversion failed: %v", err))
// 		return fmt.Errorf("LibreOffice conversion failed: %v, Output: %s", err, string(cmdOutput))
// 	}

// 	log(fmt.Sprintf("✅ Excel converted to PDF. PDF Output Path: %s", pdfOutput))

// 	// Wait a bit longer for PDF to be fully created
// 	time.Sleep(2 * time.Second)

// 	// Check if the PDF file actually exists before passing it to ImageMagick
// 	if _, err := os.Stat(pdfOutput); err != nil {
// 		log(fmt.Sprintf("⛔ PDF file does not exist at %s: %v", pdfOutput, err))
// 		return fmt.Errorf("PDF file does not exist at the expected location: %v", err)
// 	}
// 	log("✅ PDF file is accessible, ready for ImageMagick conversion")

// 	// Convert PDF to Image using ImageMagick
// 	absPdf, _ := filepath.Abs(pdfOutput)
// 	absImg, _ := filepath.Abs(imgOutput)
// 	magickPath, err := exec.LookPath("magick")
// 	if err != nil {
// 		magickPath = h.YamlCfg.Default.MagickFullPath
// 		if _, err := os.Stat(magickPath); os.IsNotExist(err) {
// 			log("⛔ ImageMagick not found")
// 			return fmt.Errorf("ImageMagick not found in PATH or default path")
// 		}
// 	}

// 	cmdMagick := exec.Command(magickPath, "-density", "300", absPdf, "-quality", "100", absImg)
// 	cmdOutput, err = cmdMagick.CombinedOutput()
// 	if err != nil {
// 		log(fmt.Sprintf("⛔ ImageMagick failed: %v, Output: %s", err, string(cmdOutput)))
// 		return fmt.Errorf("ImageMagick conversion failed: %v", err)
// 	}

// 	log("✅ Screenshot converted to image")

// 	// Cleanup the temporary files
// 	if err := os.Remove(pdfOutput); err != nil {
// 		log(fmt.Sprintf("⛔ Failed to remove PDF file: %v", err))
// 	} else {
// 		log("✅ PDF file removed")
// 	}

// 	return nil
// }

func (h *WhatsmeowHandler) exportPivotToImage(excelFilePath, imgOutput string) (err error) {
	logFile, _ := os.Create(fmt.Sprintf("./logs/export_pivot_debug_%v.log", time.Now().Format("2006-01-02")))
	defer logFile.Close()

	log := func(msg string) {
		logFile.WriteString(time.Now().Format("2006/01/02 15:04:05") + " " + msg + "\n")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	log("🔄 Start exportPivotToImage")

	absExcelPath, _ := filepath.Abs(excelFilePath)
	absImgOutput, _ := filepath.Abs(imgOutput)

	log("✅ Absolute Excel file path: " + absExcelPath)
	log("✅ Absolute Image output path: " + absImgOutput)

	// Wait for file to be unlocked
	if err = waitForFileUnlock(absExcelPath, 3*time.Second); err != nil {
		log(fmt.Sprintf("⛔ waitForFileUnlock: %v", err))
		return fmt.Errorf("failed to unlock file: %v", err)
	}
	log("✅ File unlocked")

	// PDF output is based on Excel path
	pdfOutput := strings.TrimSuffix(absExcelPath, filepath.Ext(absExcelPath)) + ".pdf"
	log("📄 PDF output path: " + pdfOutput)

	sofficePath, err := exec.LookPath("soffice")
	if err != nil {
		sofficePath = h.YamlCfg.Default.LibreOfficeFullPath
		if _, err := os.Stat(sofficePath); os.IsNotExist(err) {
			log("⛔ LibreOffice not found")
			return fmt.Errorf("LibreOffice not found")
		}
	}

	// Convert Excel → PDF
	cmdLibre := exec.Command(sofficePath, "--headless", "--convert-to", "pdf", "--outdir", filepath.Dir(absExcelPath), absExcelPath)
	cmdOutput, err := cmdLibre.CombinedOutput()
	log(fmt.Sprintf("LibreOffice command output: %s", string(cmdOutput)))
	if err != nil {
		log(fmt.Sprintf("⛔ LibreOffice conversion failed: %v", err))
		return fmt.Errorf("LibreOffice conversion failed: %v", err)
	}
	log("✅ Excel converted to PDF. PDF Output Path: " + pdfOutput)

	time.Sleep(2 * time.Second)

	// Check if PDF exists
	if _, err := os.Stat(pdfOutput); err != nil {
		log(fmt.Sprintf("⛔ PDF file does not exist at %s: %v", pdfOutput, err))
		return fmt.Errorf("PDF does not exist: %v", err)
	}
	log("✅ PDF file is accessible, ready for conversion to PNG")

	// Get total page count using pdftopng
	pageCount, err := h.getPDFPageCount(pdfOutput)
	if err != nil {
		log(fmt.Sprintf("⛔ getPDFPageCount failed: %v", err))
		return err
	}
	lastPageIndex := pageCount - 1
	log(fmt.Sprintf("📄 PDF has %d pages. Will export last page: %d", pageCount, lastPageIndex))

	// Path to pdftopng executable (from xpdf-tools)
	pdftopngPath := h.YamlCfg.Default.PdfToPngFullPath
	if _, err := os.Stat(pdftopngPath); os.IsNotExist(err) {
		log("⛔ pdftopng not found")
		return fmt.Errorf("pdftopng not found")
	}

	// Generate PNG output path
	pngOutput := strings.TrimSuffix(absImgOutput, filepath.Ext(absImgOutput)) + fmt.Sprintf("_page%d.png", lastPageIndex)

	// pdftopng command to convert last page of PDF to PNG
	cmdPdftopng := exec.Command(pdftopngPath, "-f", strconv.Itoa(lastPageIndex+1), "-l", strconv.Itoa(lastPageIndex+1), "-r", "300", pdfOutput, pngOutput)
	cmdOutput, err = cmdPdftopng.CombinedOutput()
	if err != nil {
		log(fmt.Sprintf("⛔ pdftopng failed: %v, Output: %s", err, string(cmdOutput)))
		return fmt.Errorf("pdftopng conversion failed: %v", err)
	}
	log(fmt.Sprintf("✅ PDF last page converted to PNG. PNG Output Path: %s", pngOutput))

	// Improved logic for finding PNG files
	files, err := ioutil.ReadDir(filepath.Dir(pngOutput))
	if err != nil {
		log(fmt.Sprintf("⛔ Error reading directory: %v", err))
		return fmt.Errorf("error reading directory: %v", err)
	}

	// Check if the PNG file exists
	var foundPNG string
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".png") {
			foundPNG = filepath.Join(filepath.Dir(pngOutput), file.Name())
			break
		}
	}

	// If no PNG file found
	if foundPNG == "" {
		log("⛔ No PNG files found")
		return fmt.Errorf("no PNG files found")
	}

	log(fmt.Sprintf("✅ Found PNG file: %s", foundPNG))

	// Rename the PNG file to JPG output
	if err := os.Rename(foundPNG, absImgOutput); err != nil {
		log(fmt.Sprintf("⛔ Failed to rename PNG output: %v", err))
		return fmt.Errorf("failed to rename PNG output: %v", err)
	}

	log("✅ PNG file renamed to JPG successfully.")

	// Defer cleanup of PDF and PNG files
	defer func() {
		if err := os.Remove(pdfOutput); err != nil {
			log(fmt.Sprintf("⛔ Failed to remove PDF file: %v", err))
		} else {
			log(fmt.Sprintf("✅ PDF file removed: %s", pdfOutput))
		}

		// if err := os.Remove(foundPNG); err != nil {
		// 	log(fmt.Sprintf("⛔ Failed to remove PNG file: %v", err))
		// } else {
		// 	log(fmt.Sprintf("✅ PNG file removed: %s", foundPNG))
		// }
	}()

	return nil
}

func (h *WhatsmeowHandler) getPDFPageCount(pdfFilePath string) (int, error) {
	pdfinfoPath := h.YamlCfg.Default.PdfInfoFullPath

	// Check if the pdfinfo executable exists
	if _, err := os.Stat(pdfinfoPath); os.IsNotExist(err) {
		return 0, fmt.Errorf("pdfinfo not found at path: %s", pdfinfoPath)
	}

	// Execute the pdfinfo command to get PDF metadata
	cmd := exec.Command(pdfinfoPath, pdfFilePath)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to execute pdfinfo: %v, Output: %s", err, string(cmdOutput))
	}

	// Parse the page count from the output
	pageCount := 0
	outputLines := strings.Split(string(cmdOutput), "\n")
	for _, line := range outputLines {
		if strings.HasPrefix(line, "Pages:") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				pageCount, err = strconv.Atoi(parts[1])
				if err != nil {
					return 0, fmt.Errorf("failed to parse page count: %v", err)
				}
				break
			}
		}
	}

	if pageCount == 0 {
		return 0, fmt.Errorf("failed to find page count in pdfinfo output")
	}

	return pageCount, nil
}
