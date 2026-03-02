package whatsapp

import (
	"call_center_app/models"
	"fmt"
	"log"
	"time"
)

func (h *WhatsmeowHandler) ResetStatusIsOnCalling() {
	resetStatusIsOnCallingMutex.Lock()
	defer resetStatusIsOnCallingMutex.Unlock()

	log.Print("Scheduler reset status is_on_calling is being Started")
	tableWaReq := h.YamlCfg.Db.TbWaReq

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Println("❌ Failed to load timezone Asia/Jakarta:", err)
		return
	}

	timeLimit := time.Now().In(loc).Add(-14 * time.Hour)

	var waReqData []models.WaRequest
	if err := h.Database.Table(tableWaReq).
		Where("is_on_calling = ? AND is_done = ? AND updated_at <= ?",
			true,
			false,
			timeLimit,
		).
		Find(&waReqData).Error; err != nil {
		log.Println("Error fetching records:", err)
		return
	}

	phoneNumbers := make(map[string]bool)
	for _, req := range waReqData {
		phoneNumbers[req.PicPhone] = true
	}

	var phoneList []string
	for phone := range phoneNumbers {
		phoneList = append(phoneList, phone)
	}

	if len(phoneList) == 0 {
		fmt.Println("No records to update.")
		return
	}

	if err := h.Database.Table("wa_request").
		Where("x_pic_phone IN ? AND is_done = ?", phoneList, false).
		// Update("is_on_calling", false). // not use this again coz we will track the is oncalling date time
		Updates(map[string]interface{}{
			"counter":                0,
			"temp_cs":                0,
			"is_on_calling":          false,
			"is_done":                false,
			"is_on_calling_datetime": nil,
		}).
		Error; err != nil {
		log.Println("Error updating records:", err)
		return
	}

	log.Printf("Success: Reset is_on_calling for %d records at %v\n", len(phoneList), time.Now().Format("2006-01-02 15:04:05"))
}
