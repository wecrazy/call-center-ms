package handlers

import (
	"bytes"
	"call_center_app/config"
	"call_center_app/models"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// func formatIndonesianDate(t time.Time) string {
// 	// Indonesian month names
// 	indonesianMonths := []string{
// 		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
// 		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
// 	}
// 	day := t.Day()
// 	month := indonesianMonths[t.Month()-1]
// 	year := t.Year()

// 	return fmt.Sprintf("%02d %s %d", day, month, year)
// }

func FormSubmitCallTask(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	tableCS := config.Db.TbUser
	tableMerchantsJOHmin1 := config.Db.TbDataMerchantHmin1

	return func(c *fiber.Ctx) error {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
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

		data := c.Query("data")

		// Info data parsed
		var roleCode, idData, idCS string

		dataParsed := strings.Split(data, "_")

		if len(dataParsed) >= 4 {
			roleCode = dataParsed[0]
			idData = dataParsed[1]
			idCS = dataParsed[2]
			// randomStr = dataParsed[3]
		} else {
			return c.SendString("invalid data url!!")
		}

		// Get admin cs data
		var adminCSData models.CS

		intID, err := strconv.Atoi(idCS)
		if err != nil {
			log.Print(err)
			return c.SendString(err.Error())
		}

		if err := db.Table(tableCS).Where("id = ?", uint(intID)).First(&adminCSData).Error; err != nil {
			errMsg := fmt.Sprintf("unauthorized: %v", err)
			log.Print(errMsg)
			return c.SendString(errMsg)
		}

		switch roleCode {
		// Merchant info to call
		case "m":
			var merchantsCallData models.JOMerchantHmin1

			intID, err := strconv.Atoi(idData)
			if err != nil {
				log.Print(err)
				return c.SendString(err.Error())
			}

			// Check if data is exists to call
			if err := db.Table(tableMerchantsJOHmin1).Where("id = ?", uint(intID)).First(&merchantsCallData).Error; err != nil {
				errMsg := fmt.Sprintf("got error while try to get data for being call: %v", err.Error())
				log.Print(errMsg)
				return c.SendString(errMsg)
			}

			// Handle multi merchants same phone number call
			var merchantMultiCallData []models.JOMerchantHmin1
			if err := db.Table(tableMerchantsJOHmin1).
				Where("pic_phone = ? AND on_calling = ? AND is_done = ?", merchantsCallData.PicPhone, false, false).
				Find(&merchantMultiCallData).Error; err != nil {
				errMsg := fmt.Sprintf("got error while try to get data for being call: %v", err.Error())
				log.Print(errMsg)
				return c.SendString(errMsg)
			}

			// // ****************************************************** // //
			// //       For Development Comment if Condition Below       // //
			// // ****************************************************** // //
			// if data != merchantsCallData.WebUrl {
			// 	return c.SendString("unauthorized data web url and data url db is incorrect!")
			// }

			// Update on calling status if web is open
			if len(merchantMultiCallData) == 1 {
				merchantOnCalling := models.JOMerchantHmin1{
					OnCalling: true,
					TempCS:    adminCSData.ID,
				}
				if err := db.Table(tableMerchantsJOHmin1).Where("id = ?", uint(intID)).Updates(&merchantOnCalling).Error; err != nil {
					errMsg := fmt.Sprintf("got error while try to update data for being call: %v", err.Error())
					log.Print(errMsg)
					return c.SendString(errMsg)
				}

				// soon here u set the UI for 1 mechant ID
			} else {
				for _, merchant := range merchantMultiCallData {
					merchantOnCalling := models.JOMerchantHmin1{
						OnCalling: true,
						TempCS:    adminCSData.ID,
					}
					if err := db.Table(tableMerchantsJOHmin1).Where("id = ?", merchant.ID).Updates(&merchantOnCalling).Error; err != nil {
						errMsg := fmt.Sprintf("got error while try to update data for being call: %v", err.Error())
						log.Print(errMsg)
						return c.SendString(errMsg)
					}
				}

				// soon here UI multiple merchant ID
			}

			var textPresentation strings.Builder
			textPresentation.WriteString("<h2>")
			textPresentation.WriteString("Halo, <i>" + greeting + "</i>. ")
			textPresentation.WriteString("Perkenalkan saya <i>" + adminCSData.Username + "</i> dari tim <b>Call Center</b> Manage Service EDC " + merchantsCallData.XSource + "<br><br>")
			textPresentation.WriteString("Apakah saya berbicara dengan Bapak/Ibu <i>" + merchantsCallData.Pic + "</i> dari " + merchantsCallData.Merchant + " ? <br><br>")

			// tomorrow := time.Now().AddDate(0, 0, 1)
			// tomorrowDate := formatIndonesianDate(tomorrow)

			var kunjunganTeknisi string
			switch merchantsCallData.TaskType {
			case "Preventive Maintenance":
				kunjunganTeknisi = "pemeliharaan rutin & pencegahan terhadap EDC agar memastikan EDC dalam kondisi normal"
			case "Corrective Maintenance":
				kunjunganTeknisi = "perbaikan kerusakan atau masalah pada EDC"
			case "Withdrawal":
				kunjunganTeknisi = "penarikan perangkat EDC dari lokasi merchant"
			case "Installation":
				kunjunganTeknisi = "pemasangan perangkat EDC baru (Installation)"
			case "Replacement":
				kunjunganTeknisi = "penggantian perangkat EDC lama dengan yang baru"
			case "Pindah Vendor":
				kunjunganTeknisi = "proses pemindahan layanan ke vendor lain"
			case "Re-Init":
				kunjunganTeknisi = "inisialisasi ulang perangkat EDC"
				// default:
				// 	kunjunganTeknisi = "jenis tugas tidak dikenal"
			}

			textPresentation.WriteString("<br>")
			textPresentation.WriteString("Saya ingin mengonfirmasi jadwal teknisi kami melakukan kunjungan untuk  <i>" + kunjunganTeknisi + "</i> ke tempat Bapak/Ibu. <br><br><br>")
			textPresentation.WriteString("Mohon Bapak/Ibu dapat memastikan tanggal yang sesuai untuk kunjungan tersebut. Kapan waktu yang memungkinkan untuk dilakukan kunjungan Pak/Bu?")
			// textPresentation.WriteString("Ada yang ingin saya konfirmasikan terkait kunjungan <i>" + kunjunganTeknisi + "</i> yang akan dijadwalkan pada: " + tomorrowDate + "<br><br><br>")
			// textPresentation.WriteString("Sesuai dengan catatan di sistem kami, kunjungan ke merchant Bapak/Ibu dijadwalkan pada " + tomorrowDate + ". Kami ingin memastikan apakah jadwal tersebut berkenan untuk Bapak/Ibu? <br><br>")

			textPresentation.WriteString("</h2>")
			finalTextPresentation := textPresentation.String()

			var plannedDate interface{} // Can be nil or a valid date
			if merchantsCallData.PlannedDate != nil {
				plannedDate = merchantsCallData.PlannedDate.Format("2006-01-02")
			} else {
				plannedDate = nil // Keep it nil
			}

			formInput := fmt.Sprintf(`
				<div class="form-group">
					<input
						class="form-control"
						type="hidden"
						name="dataRequest"
						id="dataRequest"
						value="merchant"
					/>
					<input
						class="form-control"
						type="hidden"
						name="idCS"
						id="idCS"
						value="%d"
					/>
					<input
						class="form-control"
						type="hidden"
						name="idJO"
						id="idJO"
						value="%d"
					/>
					<input
						class="form-control"
						type="hidden"
						name="xJobID"
						id="xJobID"
						value="%v"
					/>
					<input
						class="form-control"
						type="hidden"
						name="woNumber"
						id="woNumber"
						value="%s"
					/>
					<input
						class="form-control"
						type="hidden"
						name="taskType"
						id="taskType"
						value="%s"
					/>
				</div>

				<div class="row mb-4 mt-3">
					<!-- Merchant PIC Input -->
					<div class="form-group col-md-4 col-sm-4 col-lg-4">
						<label for="pic" class="form-label">Merchant PIC</label>
						<input
							class="form-control"
							type="text"
							name="pic"
							id="pic"
							value="%s"
						/>
					</div>

					<!-- PIC Phone Number Input -->
					<div class="form-group col-md-4 col-sm-4 col-lg-4">
						<label for="picPhone" class="form-label">PIC Phone Number</label>
						<input
							class="form-control"
							type="text"
							name="picPhone"
							id="picPhone"
							value="%s"
						/>
					</div>

					<!-- Re-Schedule Section -->
					<div class="form-group col-md-4 col-sm-4 col-lg-4">
						<div class="form-check form-switch mb-2">
							<input class="form-check-input" type="checkbox" id="reSchedule" name="reSchedule">
							<label class="form-check-label" for="reSchedule">ReSchedule</label>
						</div>
						<!-- <label for="dateReschedule" class="form-label mt-2">Date Visit Re-Schedule</label> -->
						<input
							readonly
							class="form-control bg-label-secondary text-black"
							type="date"
							name="dateReschedule"
							id="dateReschedule"
							value="%s"
						/>
					</div>
				</div>


				<div class="row mb-4">
					<div class="form-group col-sm-12 col-md-4 col-lg-4">
						<label for="imgWA" class="form-label">WhatsApp Screenshot Image</label>
						<input
							class="form-control"
							type="file"
							name="imgWA"
							id="imgWA"
						/>
					</div>
					
					<div class="form-group col-sm-12 col-md-4 col-lg-4">
						<label for="imgMerchant" class="form-label">Image Merchant</label>
						<input
							class="form-control"
							type="file"
							name="imgMerchant"
							id="imgMerchant"
						/>
					</div>

					<div class="form-group col-sm-12 col-md-4 col-lg-4">
						<label for="imgSNEDC" class="form-label">SN EDC Picture</label>
						<input
							class="form-control"
							type="file"
							name="imgSNEDC"
							id="imgSNEDC"
						/>
					</div>
				</div>

				<div class="form-group col-12">
					<label for="additionalNotes" class="form-label">Additional Notes</label>
					<textarea
						class="form-control"
						name="additionalNotes"
						id="additionalNotes"
					></textarea>
				</div>
			`, adminCSData.ID, merchantsCallData.ID, merchantsCallData.JobID, merchantsCallData.WoNumber, merchantsCallData.TaskType, merchantsCallData.Pic, merchantsCallData.PicPhone, plannedDate)

			return c.Render("merchant_hmin1", fiber.Map{
				"SLADeadline":      merchantsCallData.SlaDeadline.Format("02/Jan/2006 15:04:05"),
				"AdminCSName":      adminCSData.Username,
				"MerchantName":     merchantsCallData.Merchant,
				"MerchantPIC":      merchantsCallData.Pic,
				"MerchantPICPhone": merchantsCallData.PicPhone,
				"MerchantAddress":  &merchantsCallData.MerchantAddress,
				"Description":      &merchantsCallData.Description,
				"TextPresentation": template.HTML(finalTextPresentation),
				"Form":             template.HTML(formInput),
				"TaskType":         merchantsCallData.TaskType,
				"WoNumber":         merchantsCallData.WoNumber,
				"MID":              merchantsCallData.Mid,
				"TID":              merchantsCallData.Tid,
				"SNEDC":            merchantsCallData.SnEdc,
				"TicketNumber":     merchantsCallData.TicketNumber,
			})

		// case "t":
		// var techniciansCallData models.

		case "u":
			tableReqDapur := config.Db.TbReqDapur
			var urgentCallData models.RequestDapur

			intID, err := strconv.Atoi(idData)
			if err != nil {
				log.Print(err)
				return c.SendString(err.Error())
			}

			if err := db.Table(tableReqDapur).Where("id = ?", uint(intID)).First(&urgentCallData).Error; err != nil {
				errMsg := fmt.Sprintf("got error while try to get data urgent for being call: %v", err.Error())
				log.Print(errMsg)
				return c.SendString(errMsg)
			}

			// log.Print("weburl data send to web: ", data)
			// log.Print("urgent data weburl in db: ", *urgentCallData.WebUrl)

			if data != *urgentCallData.WebUrl {
				return c.SendString("unauthorized data in database and data url is incorrect!")
			}

			// Update on calling status if web urgent is open
			if err := db.Table(tableReqDapur).Where("id = ?", uint(intID)).Update("on_calling", true).Error; err != nil {
				errMsg := fmt.Sprintf("got error while try to update on calling status for urgent data: %v", err.Error())
				log.Print(errMsg)
				return c.SendString(errMsg)
			}

			var textPresentation strings.Builder

			textPresentation.WriteString("<h2>")
			textPresentation.WriteString("<br>Halo, " + greeting)
			textPresentation.WriteString(".Perkenalkan saya <i>" + adminCSData.Username + "</i> dari tim <b>Call Center</b> Manage Service EDC<br><br>")
			textPresentation.WriteString("Apakah saya berbicara dengan Bapak/Ibu <i><u>" + urgentCallData.Pic + "</u></i> dari " + urgentCallData.Merchant + " ? <br><br>")

			var kunjunganTeknisi string
			switch urgentCallData.JobOrder {
			case "Preventive Maintenance":
				kunjunganTeknisi = "pemeliharaan rutin & pencegahan terhadap EDC agar memastikan EDC dalam kondisi normal"
			case "Corrective Maintenance":
				kunjunganTeknisi = "perbaikan kerusakan atau masalah pada EDC"
			case "Withdrawal":
				kunjunganTeknisi = "penarikan perangkat EDC dari lokasi merchant"
			case "Installation":
				kunjunganTeknisi = "pemasangan perangkat EDC baru (Installation)"
			case "Replacement":
				kunjunganTeknisi = "penggantian perangkat EDC lama dengan yang baru"
			case "Pindah Vendor":
				kunjunganTeknisi = "proses pemindahan layanan ke vendor lain"
			case "Re-Init":
				kunjunganTeknisi = "inisialisasi ulang perangkat EDC"
				// default:
				// 	kunjunganTeknisi = "jenis tugas tidak dikenal"
			}

			textPresentation.WriteString("Saya ingin mengonfirmasi terkait jadwal kunjungan teknisi kami untuk hari ini, yang dimana untuk <i>" + kunjunganTeknisi + "</i> ke tempat Bapak/Ibu.")
			textPresentation.WriteString("</h2>")

			finalTextPresentation := textPresentation.String()

			return c.Render("request_dapur", fiber.Map{
				"ID":               urgentCallData.ID,
				"AdminCSName":      adminCSData.Username,
				"IDCS":             adminCSData.ID,
				"MerchantName":     urgentCallData.Merchant,
				"MerchantPIC":      urgentCallData.Pic,
				"MerchantPICPhone": urgentCallData.PicPhone,
				"MerchantAddress":  &urgentCallData.MerchantAddress,
				"Description":      &urgentCallData.OrderDetail,
				"TextPresentation": template.HTML(finalTextPresentation),
				"TaskType":         urgentCallData.JobOrder,
				"MID":              urgentCallData.Mid,
				"TID":              urgentCallData.Tid,
			})
		default:
			return c.SendString("unauthorized! you are not allowed to access this page!")
		}
	}
}

func SubmitCallTask(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	tableJOMerchantHMin1 := config.Db.TbDataMerchantHmin1
	tableJOMerchantHmin1CallLog := config.Db.TbMerchantHmin1CallLog
	// tableCS := config.Db.TbUser

	return func(c *fiber.Ctx) error {
		dataRequest := c.FormValue("dataRequest")

		switch dataRequest {
		case "merchant":
			idCS := c.FormValue("idCS")
			joID := c.FormValue("idJO")
			xJobID := c.FormValue("xJobID")
			woNumber := c.FormValue("woNumber")
			taskType := c.FormValue("taskType")
			pic := c.FormValue("pic")
			picPhone := c.FormValue("picPhone")
			reSchedule := c.FormValue("reSchedule")
			dateReschedule := c.FormValue("dateReschedule")
			additionalNotes := c.FormValue("additionalNotes")

			intJOID, err := strconv.Atoi(joID)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"status":  "error",
					"message": err.Error(),
				})
			}

			// var encodedImgWA, encodedImgMerchant, encodedImgSNEDC *string
			var pathImgWA, pathImgMerchant, pathImgSNEdc *string

			imgVars := map[string]**string{
				// "imgWA":       &encodedImgWA,
				// "imgMerchant": &encodedImgMerchant,
				// "imgSNEDC":    &encodedImgSNEDC,
				"imgWA":       &pathImgWA,
				"imgMerchant": &pathImgMerchant,
				"imgSNEDC":    &pathImgSNEdc,
			}

			for fieldName, variablePointer := range imgVars {
				file, err := c.FormFile(fieldName)
				if err != nil {
					continue // Skip if no file uploaded
				}

				if !isValidImage(file) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": fmt.Sprintf("invalid image file type for field %s, must be JPG, JPEG, or PNG", fieldName),
					})
				}

				// **Generate Folder & Filename**
				folderPath := filepath.Join("filestore", time.Now().Format("2006/01/02")) // YYYY/mm/dd
				filename := fmt.Sprintf("%d_%s_%d%s", intJOID, fieldName, time.Now().UnixNano(), filepath.Ext(file.Filename))
				log.Printf("Fullpath img: %v", filename)
				fullFilePath := filepath.Join(folderPath, filename)

				// **Ensure Directory Exists**
				if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": fmt.Sprintf("failed to create directory: %v", err),
					})
				}

				// **Save File**
				if err := c.SaveFile(file, fullFilePath); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": fmt.Sprintf("failed to save image: %v", err),
					})
				}

				// Store file path in the corresponding variable
				path := fullFilePath // Create a new variable to store the path
				*variablePointer = &path
				fmt.Printf("[%v] Saved: %s\n", **variablePointer, fullFilePath)
			}

			var isReschedule bool
			if reSchedule == "on" {
				isReschedule = true
			} else {
				isReschedule = false
			}

			uintID, err := strconv.ParseUint(joID, 10, 32)
			if err != nil {
				errMsg := fmt.Sprintf("error converting to uint: %v", err)
				log.Print(errMsg)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status":  "error",
					"message": errMsg,
				})
			}

			intIDCS, err := strconv.Atoi(idCS)
			if err != nil {
				errMsg := fmt.Sprintf("error converting to int: %v", err)
				log.Print(errMsg)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status":  "error",
					"message": errMsg,
				})
			}

			var dateToReschedule *time.Time
			if dateReschedule != "" {
				parsedDate, err := time.Parse("2006-01-02", dateReschedule)
				if err != nil {
					log.Printf("failed to parse date: %v\n", err)
				} else {
					dateToReschedule = &parsedDate
				}
			}

			var dataCallLogMerchant models.JOMerchantHmin1CallLog

			// Check if the record with given ID exists
			if err := db.Table(tableJOMerchantHmin1CallLog).First(&dataCallLogMerchant, uintID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// **Record not found -> Create new entry**
					newData := models.JOMerchantHmin1CallLog{
						ID:           uint(uintID),
						IdCS:         intIDCS,
						WoNumber:     woNumber,
						TaskType:     &taskType,
						Pic:          pic,
						PicPhone:     picPhone,
						IsReschedule: isReschedule,
						Reschedule:   dateToReschedule,
						// ImgWa:           encodedImgWA,
						// ImgMerchant:     encodedImgMerchant,
						// ImgSnEdc:        encodedImgSNEDC,
						CsNotes:         additionalNotes,
						JoStatus:        "Call By Admin CS",
						JobID:           &xJobID,
						ImgWaPath:       pathImgWA,
						ImgMerchantPath: pathImgMerchant,
						ImgSnEdcPath:    pathImgSNEdc,
					}

					if err := db.Table(tableJOMerchantHmin1CallLog).Create(&newData).Error; err != nil {
						log.Print(err)
						return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
							"status":  "error",
							"message": err.Error(),
						})
					}
					// return c.JSON(fiber.Map{"message": "Record created successfully", "data": newData})
				}

				// **Other errors (DB issues, etc.)**
				log.Print(err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status":  "error",
					"message": err.Error(),
				})
			}

			// **Record found -> Update existing**
			// **Update existing record using struct fields**
			dataCallLogMerchant.IdCS = intIDCS
			dataCallLogMerchant.WoNumber = woNumber
			dataCallLogMerchant.TaskType = &taskType
			dataCallLogMerchant.Pic = pic
			dataCallLogMerchant.PicPhone = picPhone
			dataCallLogMerchant.IsReschedule = isReschedule
			dataCallLogMerchant.Reschedule = dateToReschedule
			// dataCallLogMerchant.ImgWa = encodedImgWA
			// dataCallLogMerchant.ImgMerchant = encodedImgMerchant
			// dataCallLogMerchant.ImgSnEdc = encodedImgSNEDC
			dataCallLogMerchant.CsNotes = additionalNotes
			dataCallLogMerchant.JoStatus = "Call By Admin CS"
			dataCallLogMerchant.JobID = &xJobID
			dataCallLogMerchant.ImgWaPath = pathImgWA
			dataCallLogMerchant.ImgMerchantPath = pathImgMerchant
			dataCallLogMerchant.ImgSnEdcPath = pathImgSNEdc

			if err := db.Table(tableJOMerchantHmin1CallLog).Save(&dataCallLogMerchant).Error; err != nil {
				log.Print(err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status":  "error",
					"message": err.Error(),
				})
			}

			if err := db.Table(tableJOMerchantHMin1).Where("id = ?", uintID).Updates(map[string]interface{}{
				"is_done": true,
				"web_url": "",
			}).Error; err != nil {
				log.Print(err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"status":  "error",
					"message": err.Error(),
				})
			}

			// #############################################################################################################################################################
			// Update data to ODOO
			// var dataCs models.CS
			// if err := db.Table(tableCS).Where("id = ?", idCS).Find(&dataCs).Error; err != nil {
			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			// 		"status":  "error",
			// 		"message": err.Error(),
			// 	})
			// }

			// ODOOModel := "project.task"
			// params := map[string]interface{}{
			// 	"model": ODOOModel,
			// 	"id":    joID,
			// }
			// if isReschedule {
			// 	if dateToReschedule == nil {
			// 		params["planned_date_begin"] = nil
			// 		params["planned_date_end"] = nil
			// 	} else {
			// 		// dateFormatToReschedule := dateToReschedule.Format("2006-01-02")
			// 		// params["planned_date_begin"] = dateFormatToReschedule + " 00:00:00"
			// 		// params["planned_date_end"] = dateFormatToReschedule + " 23:59:59"
			// 		startOfDay := time.Date(dateToReschedule.Year(), dateToReschedule.Month(), dateToReschedule.Day(), 0, 0, 0, 0, dateToReschedule.Location())
			// 		endOfDay := time.Date(dateToReschedule.Year(), dateToReschedule.Month(), dateToReschedule.Day(), 23, 59, 59, 0, dateToReschedule.Location())
			// 		adjustedStartOfDay := startOfDay.Add(-7 * time.Hour)
			// 		adjustedEndOfDay := endOfDay.Add(-7 * time.Hour)
			// 		params["planned_date_begin"] = adjustedStartOfDay.Format("2006-01-02 15:04:05")
			// 		params["planned_date_end"] = adjustedEndOfDay.Format("2006-01-02 15:04:05")
			// 	}
			// }

			// if additionalNotes != "" {
			// 	params["x_message_call"] = fmt.Sprintf("%v ~%v", additionalNotes, dataCs.Username)
			// }

			// if pic != "" && picPhone != "" {
			// 	params["x_pic_merchant"] = pic
			// 	params["x_pic_phone"] = picPhone
			// }

			// rawJSON, err := json.Marshal(map[string]interface{}{
			// 	"jsonrpc": "2.0",
			// 	"params":  params,
			// })

			// if err != nil {
			// 	log.Printf("Failed to marshal JSON: %v", err)
			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			// 		"status":  "error",
			// 		"message": "failed to prepare request for update JO in ODOO!",
			// 	})
			// }

			// ODOOResult, err := api.ODOOUpdateData(config, string(rawJSON))
			// if err != nil {
			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			// 		"status":  "error",
			// 		"message": fmt.Sprintf("failed to Update JO: %v in ODOO", woNumber),
			// 	})
			// }

			// if ODOOResult != "Success update ODOO data" {
			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			// 		"status":  "error",
			// 		"message": fmt.Sprintf("something went wrong while try to update JO: %v in ODOO, %v", woNumber, ODOOResult),
			// 	})
			// }

			// // Update the Call Log Again
			// if err := db.Table(tableJOMerchantHmin1CallLog).Where("id = ?", joID).Update("update_to_odoo", "success").Error; err != nil {
			// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			// 		"status":  "error",
			// 		"message": err.Error(),
			// 	})
			// }
			// #############################################################################################################################################################

			// **Now send a single HTTP request with all files**
			go func() {
				// Create request body
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)

				// Attach images using the provided imgVars map
				for fieldName, variablePointer := range imgVars {
					if variablePointer == nil || *variablePointer == nil {
						continue // Skip if no file was uploaded
					}

					filePath := **variablePointer // Dereference the pointer to get the actual file path
					fileData, err := os.Open(filePath)
					if err != nil {
						log.Printf("Failed to open file: %v\n", err)
						continue
					}
					defer fileData.Close()

					// Create a form file field
					part, err := writer.CreateFormFile(fieldName, filepath.Base(filePath))
					if err != nil {
						log.Printf("Failed to create form file: %v\n", err)
						continue
					}

					// Copy file data into the request
					if _, err = io.Copy(part, fileData); err != nil {
						log.Printf("Failed to copy file data: %v\n", err)
						continue
					}
				}

				// Attach other form fields
				formFields := map[string]string{
					"idJO": joID,
				}

				for key, value := range formFields {
					_ = writer.WriteField(key, value)
				}

				// Finalize body
				if err := writer.Close(); err != nil {
					log.Printf("Failed to close writer: %v\n", err)
					return
				}

				// Create HTTP request
				fileStoreURL := fmt.Sprintf("%v/submit_cc_image", config.Default.FilestoreServer)
				req, err := http.NewRequest("POST", fileStoreURL, body)
				if err != nil {
					log.Printf("Failed to create HTTP request: %v\n", err)
					return
				}
				req.Header.Set("Content-Type", writer.FormDataContentType())
				req.Header.Set("upload", config.Default.TokenVMCCDs)

				// Send request
				client := &http.Client{Timeout: 30 * time.Second}
				// log.Printf("Img File Client: %v", client)
				resp, err := client.Do(req)
				if err != nil {
					log.Printf("Failed to send HTTP request: %v\n", err)
					return
				}
				defer resp.Body.Close()

				log.Printf("Uploaded images - Response: %d %s\n", resp.StatusCode, resp.Status)
			}()

			return c.JSON(fiber.Map{
				"message": "Form submitted successfully",
				// "data": fiber.Map{
				// 	"idCS":            idCS,
				// 	"joID":            joID,
				// 	"woNumber":        woNumber,
				// 	"pic":             pic,
				// 	"picPhone":        picPhone,
				// 	"isReschedule":    isReschedule,
				// 	"dateReschedule":  dateReschedule,
				// 	"additionalNotes": additionalNotes,
				// },
				// "odoo_update": fiber.Map{
				// 	"data": params,
				// },
			})
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "error",
				"message": "unknown data request!",
			})
		}
	}
}

func isValidImage(file *multipart.FileHeader) bool {
	validImageTypes := []string{"image/jpeg", "image/jpg", "image/png"}
	contentType := file.Header.Get("Content-Type")

	// Ensure the file's MIME type is one of the valid types
	for _, validType := range validImageTypes {
		if strings.Contains(contentType, validType) {
			return true
		}
	}

	return false
}

// func processImage(file *multipart.FileHeader, c *fiber.Ctx) (string, error) {
// 	tmpDir := "./tmp"
// 	err := os.MkdirAll(tmpDir, os.ModePerm)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create temp directory: %v", err)
// 	}

// 	tmpFilePath := fmt.Sprintf("%s/%s", tmpDir, file.Filename)

// 	err = c.SaveFile(file, tmpFilePath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to save file: %v", err)
// 	}

// 	fileContent, err := ioutil.ReadFile(tmpFilePath)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to read file content: %v", err)
// 	}

// 	base64Image := base64.StdEncoding.EncodeToString(fileContent)

// 	err = os.Remove(tmpFilePath)
// 	if err != nil {
// 		log.Printf("Failed to remove temp file: %v", err)
// 	}

// 	return base64Image, nil
// }

// func saveOrUpdateMerchantCallLog(db *gorm.DB, config *config.YamlConfig, data models.JOMerchantHmin1CallLog) error {
// 	table := config.Db.TbMerchantHmin1CallLog
// 	if data.CreatedAt == nil {
// 		currentTime := time.Now()
// 		data.CreatedAt = &currentTime
// 	}

// 	currentTime := time.Now()
// 	data.UpdatedAt = &currentTime

// 	if err := db.Table(table).Save(&data).Error; err != nil {
// 		return fmt.Errorf("failed to save data: %v", err)
// 	}
// 	return nil
// }
