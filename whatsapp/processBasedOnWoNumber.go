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

func (h *WhatsmeowHandler) ProcessSingleWoNumber(v *events.Message, stanzaID, originalSenderJID, woNumber string) {
	if woNumber == "" {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, "Maaf wo number yang Anda masukkan kosong!")
		return
	}

	woNumber = strings.ToUpper(woNumber)
	h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("*[INFO]* Request Anda untuk difollow up oleh tim Call Center Adalah WO Number *%v*. _System_ sementara memproses request tersebut, mohon bersabar.", woNumber))
	// tableName := models.BasedOnWoNumber{}.TableName()
	// h.Database.Table(tableName).Where("1=1").Delete(&models.BasedOnWoNumber{})

	dataToInsert := models.BasedOnWoNumber{
		WoNumber: woNumber,
	}
	if err := h.Database.Create(&dataToInsert).Error; err != nil {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Kami mohon maaf karena ada kendala saat akan memproses data Anda untuk difollow up oleh *Tim Call Center*.\n_Error:_ %v", err.Error()))
		return
	}

	err := h.GetDataBasedOnWONumber(v, stanzaID, originalSenderJID)
	if err != nil {
		var sb strings.Builder
		sb.WriteString("⚠ Kami mohon maaf karena ada kendala saat akan memproses data request Anda untuk difollow up oleh *Tim Call Center*.\n")
		sb.WriteString(fmt.Sprintf("_Details:_ %v", err.Error()))
		errMsg := sb.String()

		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, errMsg)
	}
}

func (h *WhatsmeowHandler) ProcessBasedOnWoNumber(v *events.Message, stanzaID, originalSenderJID string, doc *waProto.DocumentMessage) {
	maxSize := h.YamlCfg.Default.ExcelUploadedMaxSize * 1024 // KB
	findSheet := "NEED FOLLOW UP"

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
	fileName := fmt.Sprintf("based_on_wo_number%v.xlsx", utils.GenerateRandomString(20))
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

	// 5. Extract 'woNumber' values (assuming they're in the first column)
	var woNumbers []string
	for i, row := range rows {
		if i == 0 {
			continue // Skip header
		}
		if len(row) > 0 && strings.TrimSpace(row[0]) != "" {
			woNumbers = append(woNumbers, row[0])
		}
	}

	// 6. Respond or process
	if len(woNumbers) == 0 {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("⚠ Tidak ada data 'WO Number' yang ditemukan pada file %v", doc.GetFileName()))
	} else {
		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("*[INFO]* %d data sementara diproses.", len(woNumbers)))
		// tableName := models.BasedOnWoNumber{}.TableName()
		// h.Database.Table(tableName).Where("1=1").Delete(&models.BasedOnWoNumber{})

		var rowsCannotProcess []string
		for i, row := range rows {
			if i == 0 {
				if row[0] != "WO NUMBER" {
					msg := "⚠ Pastikan judul kolom A benar, dengan format: *WO NUMBER*"
					h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, msg)
					return
				}
				continue
			}

			var woNumberValue string

			if len(row) > 0 {
				woNumberValue = strings.TrimSpace(row[0])
			}

			if woNumberValue == "" {
				rowsCannotProcess = append(rowsCannotProcess, fmt.Sprintf("Baris ke-%d kosong atau tidak memiliki WO Number", i+1))
			} else {
				dataToInsert := models.BasedOnWoNumber{
					WoNumber: woNumberValue,
				}
				if err := h.Database.Create(&dataToInsert).Error; err != nil {
					rowsCannotProcess = append(rowsCannotProcess, fmt.Sprintf("Baris ke-%d mengalami masalah saat akan diinput didatabase: %v", i+1, err.Error()))
				}
			}
		}

		if len(rowsCannotProcess) > 0 {
			msgToSend := fmt.Sprintf("Dari total %d data, yang tidak dapat diinput kedalam database sebanyak %d data", len(woNumbers), len(rowsCannotProcess))
			h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, msgToSend)
		}

		h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, fmt.Sprintf("*[INFO]* %d data sudah masuk kedalam database, selanjutnya akan diproses ke ODOO.\nMohon bersabar, karena pemrosesan data ini dapat berlangsung lama🙏🏽", len(woNumbers)))
		err := h.GetDataBasedOnWONumber(v, stanzaID, originalSenderJID)
		if err != nil {
			var sb strings.Builder
			sb.WriteString("⚠ Kami mohon maaf karena ada kendala saat akan memproses data Anda untuk difollow up oleh *Tim Call Center*.\n")
			sb.WriteString(fmt.Sprintf("_Details:_ %v", err.Error()))
			errMsg := sb.String()

			h.sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, errMsg)
		}
	}
}
