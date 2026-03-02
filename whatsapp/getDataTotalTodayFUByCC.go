package whatsapp

import (
	"call_center_app/models"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"go.mau.fi/whatsmeow/types"
)

var getTotalTodayDataFUCCMutex sync.Mutex

func (h *WhatsmeowHandler) GetTotalTodayDataFUCC() {
	if !getTotalTodayDataFUCCMutex.TryLock() {
		log.Println("GetTotalTodayDataFUCC is already running, skipping execution.")
		return
	}
	defer getTotalTodayDataFUCCMutex.Unlock()

	jidString := h.YamlCfg.Whatsmeow.GroupCCJID + "@g.us"
	jid, err := types.ParseJID(jidString)
	if err != nil {
		log.Println("[ERROR] Invalid JID format:", jidString)
		return
	}

	taskDoing := "Get Total Data Today Follow Up by Call Center"
	log.Printf("Running task %v @%v", taskDoing, time.Now())

	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Print(err)
		return
	}

	// Get current time in Jakarta timezone
	now := time.Now().In(loc)
	// Convert to Indonesian date format
	tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	startDate := now.Format("2006-01-02") + " 00:00:00"
	endDate := now.Format("2006-01-02") + " 23:59:59"

	// Count data
	var (
		totalJOData,
		totalPhoneData,
		totalJONotFUData,
		totalPhoneNotFUData,
		totalJOBeingFUData,
		totalPhoneBeingFUData,
		totalJODoneFUData,
		totalPhoneDoneFUData,
		totalPingMerchantData,
		totalPingMerchantDataNotFU,
		totalPMNewData,
		totalPMNewDataNotFU,
		totalPMAllStageData,
		totalPMAllStageDataNotFU int64
	)

	// Define count queries with multiple WHERE conditions
	countQueries := []struct {
		column string
		where  []string
		args   []interface{}
		target *int64
	}{
		/* Total All Data */
		{
			"x_no_task",
			[]string{"updated_at BETWEEN ? AND ? "},
			[]interface{}{startDate, endDate},
			&totalJOData,
		},
		{
			"x_pic_phone",
			[]string{"updated_at BETWEEN ? AND ?"},
			[]interface{}{startDate, endDate},
			&totalPhoneData,
		},
		/* Total Not Followed Up */
		{
			"x_no_task",
			[]string{"temp_cs = ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, false, false, startDate, endDate},
			&totalJONotFUData,
		},
		{
			"x_pic_phone",
			[]string{"temp_cs = ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, false, false, startDate, endDate},
			&totalPhoneNotFUData,
		},
		/* Total Data Being Follow Up */
		{
			"x_no_task",
			[]string{"temp_cs != ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, true, false, startDate, endDate},
			&totalJOBeingFUData,
		},
		{
			"x_pic_phone",
			[]string{"temp_cs != ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, true, false, startDate, endDate},
			&totalPhoneBeingFUData,
		},
		/* Total Data Done Followed Up */
		{
			"x_no_task",
			[]string{"temp_cs != ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, true, true, startDate, endDate},
			&totalJODoneFUData,
		},
		{
			"x_pic_phone",
			[]string{"temp_cs != ? AND is_on_calling = ? AND is_done = ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, true, true, startDate, endDate},
			&totalPhoneDoneFUData,
		},
		/* Total Ping Merchant */
		{
			"x_no_task",
			[]string{"stage_name = ? AND ticket_stage_name = ? AND LOWER(description) LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{"New", "New", "%pingmerchant%", startDate, endDate},
			&totalPingMerchantData,
		},
		{
			"x_no_task",
			[]string{"temp_cs = ? AND is_on_calling = ? AND is_done = ? AND stage_name = ? AND ticket_stage_name = ? AND LOWER(description) LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, false, false, "New", "New", "%pingmerchant%", startDate, endDate},
			&totalPingMerchantDataNotFU,
		},
		/* PM New Data */
		{
			"x_no_task",
			[]string{"stage_name = ? AND ticket_stage_name = ? AND x_task_type LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{"New", "New", "Preventive Maintenance", startDate, endDate},
			&totalPMNewData,
		},
		{
			"x_no_task",
			[]string{"temp_cs = ? AND is_on_calling = ? AND is_done = ? AND stage_name = ? AND ticket_stage_name = ? AND x_task_type LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, false, false, "New", "New", "Preventive Maintenance", startDate, endDate},
			&totalPMNewDataNotFU,
		},
		/* PM All Data */
		{
			"x_no_task",
			[]string{"x_task_type LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{"Preventive Maintenance", startDate, endDate},
			&totalPMAllStageData,
		},
		{
			"x_no_task",
			[]string{"temp_cs = ? AND is_on_calling = ? AND is_done = ? AND x_task_type LIKE ? AND updated_at BETWEEN ? AND ?"},
			[]interface{}{0, false, false, "Preventive Maintenance", startDate, endDate},
			&totalPMAllStageDataNotFU,
		},
	}

	// Execute each query dynamically
	for _, query := range countQueries {
		dbQuery := h.Database.Model(&models.WaRequest{}).Distinct(query.column)

		// Apply WHERE conditions dynamically
		for _, condition := range query.where {
			dbQuery = dbQuery.Where(condition, query.args...) // ✅ Pass all arguments properly
		}

		if err := dbQuery.Count(query.target).Error; err != nil {
			log.Println("Error counting", query.column, ":", err)
		}
	}

	var sb strings.Builder
	formattedDate := tgl.Format(" ", []tanggal.Format{
		tanggal.NamaHari,  // Kamis
		tanggal.Hari,      // 27
		tanggal.NamaBulan, // Maret
		tanggal.Tahun,     // 2025
		tanggal.PukulDenganDetik,
		tanggal.ZonaWaktu,
	})
	sb.WriteString(fmt.Sprintf("🔔 FYI Team, Per _%v_\n\n", formattedDate))
	sb.WriteString(fmt.Sprintf("1️⃣ Total data keseluruhan yang tercatat: *%d* JO tersedia, dengan *%d* nomor yang dapat dihubungi.\n", totalJOData, totalPhoneData))
	sb.WriteString(fmt.Sprintf("2️⃣ Data yang belum difollow up: %d JO, dengan %d nomor yang dapat dihubungi.\n", totalJONotFUData, totalPhoneNotFUData))
	sb.WriteString(fmt.Sprintf("3️⃣ %d JO yang sementara difollow up (belum tersubmit di dashboard), dengan %d nomor yang sementara dihubungi.\n", totalJOBeingFUData, totalPhoneBeingFUData))
	sb.WriteString(fmt.Sprintf("4️⃣ Total yang sudah difollow up: %d JO, dengan %d nomor yang sudah dihubungi.\n", totalJODoneFUData, totalPhoneDoneFUData))
	sb.WriteString(fmt.Sprintf("5️⃣ Total data #PINGMERCHANT: %d JO tersedia, dengan %d JO yang belum difollow up.\n", totalPingMerchantData, totalPingMerchantDataNotFU))
	sb.WriteString(fmt.Sprintf("6️⃣ Total data PM (New): %d JO tersedia, dengan %d JO yang belum difollow up.\n", totalPMNewData, totalPMNewDataNotFU))
	sb.WriteString(fmt.Sprintf("7️⃣ Total data PM (All Stage): %d JO tersedia, dengan %d JO yang belum difollow up.\n", totalPMAllStageData, totalPMAllStageDataNotFU))
	msgToSend := sb.String()
	mentions := []string{
		h.YamlCfg.Whatsmeow.WaTetty,
		h.YamlCfg.Whatsmeow.WaBuLina,
	}

	// h.sendWhatsAppMessage(jid, msgToSend)
	h.sendWhatsAppMessageWithMentions(jid, mentions, msgToSend)

	log.Printf("Task %v successfully executed @%v", taskDoing, time.Now())
}
