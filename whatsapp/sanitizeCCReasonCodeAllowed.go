package whatsapp

import (
	"call_center_app/models"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types"
)

var sanitizeCCRCAllowedMutex sync.Mutex

func (h *WhatsmeowHandler) SanitizeCCRCAllowed() {
	if !sanitizeCCRCAllowedMutex.TryLock() {
		log.Println("SanitizeCCRCAllowed is already running, skipping execution.")
		return
	}
	defer sanitizeCCRCAllowedMutex.Unlock()

	taskDoing := "Sanitize Data in Solved Pending Ticket based on Allowed RC for Call Center"

	log.Printf("Running scheduler %v @%v", taskDoing, time.Now())

	jidString := h.YamlCfg.Whatsmeow.GroupTestJID + "@g.us"
	jid, err := types.ParseJID(jidString)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidString)
		h.sendWhatsAppMessage(jid, "⚠ Invalid JID format. Report generation aborted.")
		return
	}

	remarkFilters := []string{
		"%EDC tidak ada di lokasi merchant%",
		"%Merchant - EDC Not in Merchant Location%",
		"%EDC berada di KP Merchant%",
		"%EDC Sudah di Tarik Vendor Lain%",
		"%Merchant tutup permanen%",
		"%Merchant tutup sementara%",
		"%Merchant menolak dilakukannya pekerjaan%",
		"%Merchant - Refused Installation%",
		"%Merchant - Refused Pull Out%",
		"%Merchant - Temporarily Closed%",
		"%Merchant - Permanently Closed/Moved%",
		"%Merchant - Visit Re-Schedule%",
		"%Merchant meminta reschedule kunjungan%",
		"%Merchant - Refuses Work%",
		"%Merchant Refuses Work%",
		"%Merchant - Location Not Ready / Renovation%",
		"%Lokasi Merchant Belum Siap%",
	}

	query := h.Database.Table(h.YamlCfg.Db.TbWaReq).
		Where("request_type = ?", "Ticket Solved Pending Follow Up").
		Where("is_done = ?", false).
		Where("is_on_calling = ?", false).
		Where("temp_cs = ?", 0)

	for _, remark := range remarkFilters {
		query = query.Where("x_wo_remark NOT LIKE ?", remark)
	}

	// result := query.Delete(nil) // Pass nil since we are not deleting a specific struct
	result := query.Delete(&models.WaRequest{})
	if result.Error != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data Ticket Solved Pending follow up Reason Code untuk Call Center: %v", result.Error))
		return
	}

	query2 := h.Database.Table(h.YamlCfg.Db.TbWaReq).
		Where("request_type != ?", "Ticket Solved Pending Follow Up").
		Where("x_wo_remark IS NOT NULL AND x_wo_remark <> ''").
		Where("is_done = ?", false).
		Where("is_on_calling = ?", false).
		Where("temp_cs = ?", 0)

	for _, remark := range remarkFilters {
		query2 = query2.Where("x_wo_remark NOT LIKE ?", remark)
	}
	// result2 := query2.Delete(nil)
	result2 := query2.Delete(&models.WaRequest{})
	if result2.Error != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data selain Ticket Solved Pending follow up Reason Code untuk Call Center: %v", result2.Error))
		return
	}

	query3 := h.Database.Table(h.YamlCfg.Db.TbWaReq).
		Where("technician_name = ?", "Teknisi Pameran").
		Where("x_task_type = ?", "Withdrawal").
		Where("is_done = ?", false).
		Where("is_on_calling = ?", false).
		Where("temp_cs = ?", 0)
	// result3 := query3.Delete(nil)
	result3 := query3.Delete(&models.WaRequest{})
	if result3.Error != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data teknisi pameran SPK Penarikan: %v", result3.Error))
		return
	}

	query4 := h.Database.Table(h.YamlCfg.Db.TbWaReq).
		Where("x_task_type = ?", "Preventive Maintenance").
		Where("stage_name = ?", "New").
		// Where("request_type IN (?, ?)", "Data Planned H+0 Follow Up", "SLA H-2 Follow Up").
		Where("is_done = ?", false).
		Where("is_on_calling = ?", false).
		Where("temp_cs = ?", 0)
	// result4 := query4.Delete(nil)
	result4 := query4.Delete(&models.WaRequest{})
	if result4.Error != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data SPK PM: %v", result4.Error))
		return
	}

	query5 := h.Database.Table(h.YamlCfg.Db.TbWaReq).
		Where("LOWER(description) LIKE ?", "%pingmerchant%").
		Where("is_done = ?", false).
		Where("is_on_calling = ?", false).
		Where("temp_cs = ?", 0)
	result5 := query5.Delete(&models.WaRequest{})
	if result5.Error != nil {
		h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data Ping Merchant: %v", result4.Error))
		return
	}

	// result6 := h.Database.
	// 	Table(models.CannotFollowUp{}.TableName()).
	// 	// Session(&gorm.Session{AllowGlobalUpdate: true}).
	// 	Where("id != 0").
	// 	Delete(nil)
	// 	// Delete(&models.CannotFollowUp{})

	// if result6.Error != nil {
	// 	h.sendWhatsAppMessage(jid, fmt.Sprintf("⚠ Kami mohon maaf, gagal saat melakukan sanitazi data Cannot Follow Up: %v", result5.Error))
	// 	return
	// }

	var sb strings.Builder
	sb.WriteString("🎉 *[INFO]* Sukses melakukan sanitasi (hapus) data:\n\n")
	sb.WriteString(fmt.Sprintf("1) Solved Pending Ticket FU tidak sesuai RC Call Center: %d rows\n", result.RowsAffected))
	sb.WriteString(fmt.Sprintf("2) Non Solved Pending Ticket FU tidak sesuai RC Call Center: %d rows\n", result2.RowsAffected))
	sb.WriteString(fmt.Sprintf("3) Data SPK Penarikan Teknisi Pameran: %d rows\n", result3.RowsAffected))
	sb.WriteString(fmt.Sprintf("4) Data SPK PM New: %d rows\n", result4.RowsAffected))
	sb.WriteString(fmt.Sprintf("5) Ping Merchant: %d rows\n", result5.RowsAffected))
	// sb.WriteString(fmt.Sprintf("6) Cannot Follow Up Data: %d rows\n", result6.RowsAffected))
	msgToSend := sb.String()

	h.sendWhatsAppMessage(jid, msgToSend)
	log.Printf("Scheduler %v successfully executed @%v", taskDoing, time.Now())
}
