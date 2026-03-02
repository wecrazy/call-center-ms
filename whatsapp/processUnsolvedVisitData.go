package whatsapp

import (
	"call_center_app/models"
	"call_center_app/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

func (h *WhatsmeowHandler) ProcessUnsolvedVisitData(v *events.Message, stanzaID, originalSenderJID string, doc *waProto.DocumentMessage) {
	maxSize := h.YamlCfg.Default.ExcelUploadedMaxSize * 1024 // KB
	findSheet := "UNSOLVED VISIT"

	if doc.GetFileLength() >= maxSize {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID,
			fmt.Sprintf("⚠ Maaf ukuran file yang Anda upload melebihi maksimal ukuran yang diperbolehkan (%d KB)", maxSize))
		return
	}

	// 1. Download file from WhatsApp
	fileBytes, err := h.Client.Download(doc)
	if err != nil {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Gagal mengunduh file %v: %v", doc.GetFileName(), err))
		return
	}

	// 2. Save it to a temporary file
	fileName := fmt.Sprintf("unsolved_visit_%v.xlsx", utils.GenerateRandomString(20))
	tempPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s_%s", stanzaID, fileName))

	if err := os.WriteFile(tempPath, fileBytes, 0644); err != nil {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Gagal menyimpan file %v sementara: %v", doc.GetFileName(), err))
		return
	}
	log.Printf("[INFO] File saved temporarily at: %s", tempPath)

	// 3. Open Excel file with excelize
	f, err := excelize.OpenFile(tempPath)
	if err != nil {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Gagal membuka file %v: %v", doc.GetFileName(), err))
		_ = os.Remove(tempPath) // Clean up on failure
		return
	}
	defer func() {
		f.Close()
		_ = os.Remove(tempPath) // Always clean up
	}()

	// 4. Read data from the first sheet (or a specific one)
	sheetName := f.GetSheetName(0)
	if sheetName != findSheet {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Sheet %v pada index pertama tidak ditemukan.", findSheet))
		return
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Gagal membaca data dari file %v: %v", doc.GetFileName(), err))
		return
	}

	// 5. Extract 'subject' values (assuming they're in the first column)
	var subjects []string
	for i, row := range rows {
		if i == 0 {
			continue // Skip header
		}
		if len(row) > 0 && strings.TrimSpace(row[0]) != "" {
			subjects = append(subjects, row[0])
		}
	}

	// 6. Respond or process
	if len(subjects) == 0 {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Tidak ada data 'subject' atau spk number ditemukan pada file %v", doc.GetFileName()))
	} else {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("*[INFO]* %d data sementara diproses.", len(subjects)))
		tableName := models.UnsolvedTicketData{}.TableName()
		h.Database.Table(tableName).Where("1=1").Delete(&models.UnsolvedTicketData{})

		var rowsCannotProcess []string
		for i, row := range rows {
			if i == 0 {
				if row[0] != "SUBJECT" {
					msg := "⚠ Pastikan judul kolom A benar, dengan format: *SUBJECT*"
					h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, msg)
					return
				}
				continue
			}

			var subjectSPK string

			if len(row) > 0 {
				subjectSPK = strings.TrimSpace(row[0])
			}

			if subjectSPK == "" {
				rowsCannotProcess = append(rowsCannotProcess, fmt.Sprintf("Baris ke-%d kosong atau tidak memiliki subject SPK", i+1))
			} else {
				dataToInsert := models.UnsolvedTicketData{
					Subject: subjectSPK,
				}
				if err := h.Database.Create(&dataToInsert).Error; err != nil {
					rowsCannotProcess = append(rowsCannotProcess, fmt.Sprintf("Baris ke-%d mengalami masalah saat akan diinput didatabase: %v", i+1, err.Error()))
				}
			}
		}

		if len(rowsCannotProcess) > 0 {
			msgToSend := fmt.Sprintf("Dari total %d data, yang tidak dapat diinput kedalam database sebanyak %d data", len(subjects), len(rowsCannotProcess))
			h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, msgToSend)
		}

		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("*[INFO]* %d data sudah masuk kedalam database, selanjutnya akan diproses ke ODOO.\nMohon bersabar, karena pemrosesan data ini dapat berlangsung lama🙏🏽", len(subjects)))
		err := h.GetDataUnsolvedTicketForFU(v, stanzaID, originalSenderJID)
		if err != nil {
			var sb strings.Builder
			sb.WriteString("⚠ Kami mohon maaf karena ada kendala saat akan memproses data Anda untuk difollow up oleh *Tim Call Center*.\n")
			sb.WriteString(fmt.Sprintf("_Details:_ %v", err.Error()))
			errMsg := sb.String()

			h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, errMsg)
		}
	}
}
