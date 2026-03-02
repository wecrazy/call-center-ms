package goroutine

import (
	"call_center_app/config"
	"call_center_app/handlers"
	"call_center_app/models"
	"call_center_app/tasks"
	"call_center_app/utils"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/go-co-op/gocron"
	"gorm.io/gorm"
)

func CSCall(db *gorm.DB, config *config.YamlConfig) {
	scheduler := gocron.NewScheduler(time.Local)

	scheduler.Every(config.GoRoutine.CallWAPICMerchant).Seconds().Do(func() {
		csCallProcess(db, config)
	})

	scheduler.StartAsync()
}

func csCallProcess(db *gorm.DB, config *config.YamlConfig) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		log.Print(err)
		return
	}

	// Get current time in Jakarta timezone
	now := time.Now().In(loc)
	hour := now.Hour()

	// Greeting logic (ensuring correct 24-hour format)
	var greeting string
	if hour >= 0 && hour < 4 {
		greeting = "Selamat Dini Hari" // 00:00 - 03:59
	} else if hour >= 4 && hour < 12 {
		greeting = "Selamat Pagi" // 04:00 - 11:59
	} else if hour >= 12 && hour < 15 {
		greeting = "Selamat Siang" // 12:00 - 14:59
	} else if hour >= 15 && hour < 17 {
		greeting = "Selamat Sore" // 15:00 - 16:59
	} else if hour >= 17 && hour < 19 {
		greeting = "Selamat Petang" // 17:00 - 18:59
	} else {
		greeting = "Selamat Malam" // 19:00 - 23:59
	}

	tableCS := config.Db.TbUser
	tableWaReq := config.Db.TbWaReq

	var csRecords []models.CS
	// if err := db.Table(tableCS).Where("is_login = ?", true).Find(&csRecords).Error; err != nil {
	// [DEBUG] delete SOON !!
	if err := db.Table(tableCS).
		// DELETE SOON !!
		// Where("is_login = ? AND id IN (?, ?)",
		// 	true,
		// 	15, // WIWI
		// 	13, // AZZRA
		// ).
		Where("is_login = ?",
			true,
		).
		Find(&csRecords).
		Error; err != nil {
		log.Printf("error fetching CS records: %v", err)
		return
	}

	totalAdminCS := len(csRecords)

	var waReqData []models.WaRequest
	totalWaReqData := 0

	subQuery := db.Table(tableWaReq).
		Select("MIN(id) AS id").
		Where("temp_cs = ? AND is_on_calling = ? AND is_done = ? AND is_final = ? AND (keterangan = '' OR keterangan IS NULL)",
			0,
			false,
			false,
			false,
		).
		Group("x_pic_phone")

	today := time.Now().Truncate(24 * time.Hour) // delete this soon !!
	if err := db.Table(tableWaReq).
		Joins("JOIN (?) AS grouped ON grouped.id = wa_request.id", subQuery).
		Where("wa_request.x_pic_phone REGEXP ?", "^\\+|^08|^6|^8").
		Where("wa_request.x_sla_deadline >= ?", today). // delete this soon !!
		Order("wa_request.counter ASC").
		Limit(totalAdminCS).
		Find(&waReqData).
		Error; err != nil {
		log.Print(err)
		return
	}

	totalWaReqData = len(waReqData)

	for i, cs := range csRecords {
		fmt.Printf("Start processing data for cs: %v\n", cs.Username)

		myIP, err := handlers.GetWiFi_IPv4()
		if err != nil {
			myIP = handlers.GetWIFIIP()
		}
		if myIP == "" {
			myIP = config.Default.HostServer
		}
		//++++++++++++++++++++++++++++ Whatsapp Request +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
		if i < totalWaReqData {
			fmt.Printf("[%v] REQUEST Follow Up: %v by %v, merchant: %v, pic: %v, phone: %v\n",
				i,
				waReqData[i].RequestType,
				cs.Username,
				waReqData[i].MerchantName,
				waReqData[i].PicMerchant,
				waReqData[i].PicPhone,
			)

			randomStr := utils.GenerateRandomString(100)
			uniqCode := "whatsapp"
			waReqURL := fmt.Sprintf("%s_%s_%s_%d", uniqCode, waReqData[i].PicPhone, randomStr, cs.ID)
			// soon fix the url only use id maybe ?

			if err := db.Table(tableWaReq).
				Where("id = ?", waReqData[i].ID).
				Updates(map[string]interface{}{
					"counter": waReqData[i].Counter + 1,
				}).Error; err != nil {
				log.Print(err)
				continue
			}

			// Directly call the PIC merchant
			// go tasks.CallWA(
			// 	config,
			// 	cs.IP,
			// 	"("+waReqData[i].RequestType+")",
			// 	waReqData[i].PicMerchant+" - "+waReqData[i].MerchantName,
			// 	waReqData[i].PicPhone,
			// 	waReqData[i].Tid,
			// 	fmt.Sprintf("http://%s:%s/request-wa?data=%s", myIP, config.App.Port, waReqURL),
			// )

			// Automatic send chat message in Whatsapp
			var kunjunganTeknisi string
			if waReqData[i].Description != "" {
				isItPingMerchant := isPingMerchant(waReqData[i].Description)
				if isItPingMerchant {
					kunjunganTeknisi = getKunjunganTeknisi("default")
				} else {
					kunjunganTeknisi = getKunjunganTeknisi(waReqData[i].TaskType)
					// DELETE SOON COZ WANT TO FU PING MERCHANT
					continue
				}
			} else {
				kunjunganTeknisi = getKunjunganTeknisi(waReqData[i].TaskType)
				// DELETE SOON COZ WANT TO FU PING MERCHANT
				continue
			}

			var chatMsg string
			var textPresentation strings.Builder

			textPresentation.WriteString(fmt.Sprintf("Halo, %v. Perkenalkan saya %v dari tim Call Center Manage Service EDC",
				greeting,
				cs.Username,
			))
			if waReqData[i].Source != "" {
				textPresentation.WriteString(fmt.Sprintf(" %v", strings.ToUpper(waReqData[i].Source)))
			}
			textPresentation.WriteString(".\n")
			textPresentation.WriteString(fmt.Sprintf("Apakah benar saya berbicara dengan Bapak/Ibu *%v* dari:\n",
				strings.ToUpper(strings.TrimSpace(waReqData[i].PicMerchant))))
			if waReqData[i].MerchantName != "" {
				textPresentation.WriteString(fmt.Sprintf("Merchant: *%v*\n",
					strings.ToUpper(strings.TrimSpace(waReqData[i].MerchantName))))
			}
			if waReqData[i].MerchantAddress != "" {
				textPresentation.WriteString(fmt.Sprintf("Alamat: %v\n",
					strings.TrimSpace(waReqData[i].MerchantAddress)))
			}
			if waReqData[i].SnEdc != "" {
				textPresentation.WriteString(fmt.Sprintf("SN EDC: *%v*\n",
					strings.TrimSpace(waReqData[i].SnEdc)))
			}
			textPresentation.WriteString("\n\n")

			if waReqData[i].TaskType == "Preventive Maintenance" && waReqData[i].StageName == "New" {
				textPresentation.WriteString("Mohon Bapak/Ibu dapat menginformasikan hari dan jam operasional yang memungkinkan untuk dilakukan pengecekan EDC, agar dapat kami sampaikan kepada tim yang akan melakukan kunjungan pengecekan EDC.")
			} else {
				textPresentation.WriteString(fmt.Sprintf("Saya ingin mengkonfirmasi jadwal teknisi kami melakukan kunjungan untuk _%v_ di tempat Bapak/Ibu.\n\n",
					kunjunganTeknisi))
				textPresentation.WriteString("Mohon Bapak/Ibu dapat memastikan tanggal yang sesuai untuk kunjungan tersebut. Kapan waktu yang memungkinan untuk dilakukan kunjungannya Pak/Bu ?")
			}

			chatMsg = textPresentation.String()

			if chatMsg == "" {
				continue
			}

			// Send msg chat to PIC merchant
			go tasks.ChatWA(
				config,
				cs.IP,
				"("+waReqData[i].RequestType+")",
				waReqData[i].PicMerchant+" - "+waReqData[i].MerchantName,
				waReqData[i].PicPhone,
				chatMsg,
				waReqData[i].Tid,
				fmt.Sprintf("http://%s:%s/request-wa?data=%s", myIP, config.App.Port, waReqURL),
			)
		}
	}
}

func getKunjunganTeknisi(taskType string) string {
	switch strings.ToLower(taskType) {
	case "preventive maintenance":
		return "pemeliharaan rutin & pencegahan terhadap EDC agar memastikan EDC dalam kondisi normal"
	case "corrective maintenance":
		return "perbaikan kerusakan atau masalah pada EDC"
	case "withdrawal":
		return "penarikan perangkat EDC dari lokasi merchant"
	case "installation":
		return "pemasangan perangkat EDC baru (Installation)"
	case "replacement":
		return "penggantian perangkat EDC lama dengan yang baru"
	case "pindah vendor":
		return "proses pemindahan layanan ke vendor lain"
	case "re-init":
		return "inisialisasi ulang perangkat EDC"
	case "roll out":
		return "distribusi dan implementasi perangkat EDC baru ke berbagai lokasi"
	default:
		return "memastikan perangkat EDC dalam keadaan normal"
	}
}

func isPingMerchant(taskType string) bool {
	// Convert to lowercase for case-insensitive matching
	normalized := strings.ToLower(taskType)
	// Regular expression to match "ping" first and "merchant" second, allowing extra characters before/after
	re := regexp.MustCompile(`.*ping.*merchant.*`)
	// Match the regex
	return re.MatchString(normalized)
}
