package handlers

import (
	"bytes"
	"call_center_app/config"
	"call_center_app/models"
	"call_center_app/whatsapp"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/freetype"
	"golang.org/x/image/font"
	"gorm.io/gorm"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	// waLog "go.mau.fi/whatsmeow/util/log"
	// waProto "go.mau.fi/whatsmeow/binary/proto"
)

// type FiberLoggerAdapter struct {
// 	module string
// }

// // Implementing all methods required by waLog.Logger
// func (f FiberLoggerAdapter) Debugf(format string, v ...interface{}) {
// 	log.Printf("[DEBUG] "+format, v...)
// }
// func (f FiberLoggerAdapter) Infof(format string, v ...interface{}) {
// 	log.Printf("[INFO] "+format, v...)
// }
// func (f FiberLoggerAdapter) Warnf(format string, v ...interface{}) {
// 	log.Printf("[WARN] "+format, v...)
// }
// func (f FiberLoggerAdapter) Errorf(format string, v ...interface{}) {
// 	log.Printf("[ERROR] "+format, v...)
// }

// // Sub() returns a new FiberLoggerAdapter with a different module name
// func (f FiberLoggerAdapter) Sub(module string) waLog.Logger {
// 	return FiberLoggerAdapter{module: f.module + "." + module}
// }

var Client *whatsmeow.Client

func StartWhatsAppClient(yamlCfg *config.YamlConfig) *whatsmeow.Client {
	// dbLog := FiberLoggerAdapter{}
	// clientLog := FiberLoggerAdapter{}
	dbLog := waLog.Stdout("Database", "ERROR", true) // DEBUG, WARN, INFO, ERROR
	clientLog := waLog.Stdout("Client", "ERROR", true)

	container, err := sqlstore.New(yamlCfg.Whatsmeow.SqlDriver, fmt.Sprintf("file:%s?_foreign_keys=on", yamlCfg.Whatsmeow.SqlSource), dbLog)
	if err != nil {
		log.Printf("failed to create database container: %v", err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		log.Printf("failed to get first device: %v", err)
	}

	Client = whatsmeow.NewClient(deviceStore, clientLog)

	if Client.Store.ID == nil {
		qrChan, _ := Client.GetQRChannel(context.Background())
		err = Client.Connect()
		if err != nil {
			log.Printf("failed to connect WhatApp client: %v", err)
		}

		for evt := range qrChan {
			if evt.Event == "code" {
				fmt.Println("Scan this QR code to login:")

				// Generate and display QR Code
				qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
				saveQRCodeToFile(evt.Code)
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		err = Client.Connect()
		if err != nil {
			log.Printf("failed to connect WhatApp client: %v", err)
		}
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		Client.Disconnect()
	}()

	return Client
}

func saveQRCodeToFile(code string) {
	filePath := "whatsapp/qrcode.txt"

	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("Failed to create QR code file: %v", err)
		return
	}
	defer file.Close()

	qrterminal.GenerateHalfBlock(code, qrterminal.L, file)

	fmt.Println("QR Code saved to:", filePath)
}

func WaWhatsmeow(db *gorm.DB, yamlCfg *config.YamlConfig) {
	Client = StartWhatsAppClient(yamlCfg)
	if Client == nil {
		log.Print("failed to start WhatsApp client")
	}

	handler := whatsapp.NewWhatsmeowHandler(Client, db, yamlCfg)

	// // Initialize
	handler.GetGroupInfo()
	// handler.GetAllowedReasonCode()

	// Every 1 Hour Schedulers
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			handler.GetGroupInfo()
			handler.GetAllowedReasonCode()
			// handler.ResetStatusIsOnCalling() // didnt use this anymore
		}
	}()

	Client.AddEventHandler(handler.HandleEvent)

	scheduler := gocron.NewScheduler(time.Local)
	schedulers := []whatsapp.SchedulerConfig{
		{
			Times:    config.GetConfig().Scheduler.GetDataSLAHmin2,
			Function: handler.GetDataSLAHmin2forFU,
			Name:     "SLA H-2",
		},
		{
			Times:    config.GetConfig().Scheduler.GetSolvedPendingTicket,
			Function: handler.GetDataSolvedPendingforFU,
			Name:     "Solved Pending Ticket",
		},
		{
			Times:    config.GetConfig().Scheduler.GenerateCallCenterReport,
			Function: handler.GenerateReportCallCenter,
			Name:     "Generate Report Call Center",
		},
		{
			Times:    config.GetConfig().Scheduler.SanitizeCCReasonCodeAllowed,
			Function: handler.SanitizeCCRCAllowed,
			Name:     "Sanitize Solved Pending Ticket FU to Call Center",
		},
		{
			Times:    config.GetConfig().Scheduler.CheckMrOliverReport,
			Function: handler.CheckMrOliverReportIsExists,
			Name:     "Check Mr. Oliver Report",
		},
		{
			Times:    config.GetConfig().Scheduler.GetDataPlannedHPlus0,
			Function: handler.GetDataPlannedHPlus0,
			Name:     "Data Planned H+0",
		},
		{
			Times:    config.GetConfig().Scheduler.ResetStatusIsOncalling,
			Function: handler.ResetStatusIsOnCalling,
			Name:     "Reset Status Is On Calling (Updated At below 14 Hours)",
		},
		// Add more schedulers here...
	}

	for _, schedConfig := range schedulers {
		for _, timeStr := range schedConfig.Times {
			_, err := scheduler.Every(1).Day().At(timeStr).Do(schedConfig.Function)
			if err != nil {
				log.Printf("Error scheduling job %s at %s: %v", schedConfig.Name, timeStr, err)
			}
		}
	}
	scheduler.StartAsync()

	select {} // Keeps the program running
}

type RequestData struct {
	PhoneNumber string `json:"phone_number"`
	TempCS      string `json:"temp_cs"`
}

type FormRequest struct {
	ID              string                `form:"id"`
	Pic             string                `form:"pic"`
	PicPhone        string                `form:"picPhone"`
	ReSchedule      bool                  `form:"reSchedule"`
	DateReschedule  string                `form:"dateReschedule"`
	TargetSchedule  string                `form:"targetSchedule"`
	AdditionalNotes string                `form:"additionalNotes"`
	ImgWA           *multipart.FileHeader `form:"imgWA"`
	ImgMerchant     *multipart.FileHeader `form:"imgMerchant"`
	ImgSNEDC        *multipart.FileHeader `form:"imgSNEDC"`
}

func processInfo(data []string) string {
	sort.Strings(data)
	data = slices.Compact(data)
	return strings.Join(data, " ")
}

func removeDuplicates(input []string) string {
	uniqueMap := make(map[string]bool)
	var uniqueList []string

	for _, item := range input {
		if !uniqueMap[item] { // If not already added
			uniqueMap[item] = true
			uniqueList = append(uniqueList, item)
		}
	}
	return strings.Join(uniqueList, ", ")
}

// Struct for parsing geolocation API response
type GeoLocation struct {
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	City    string  `json:"city"`
	Region  string  `json:"regionName"`
	Country string  `json:"country"`
	Query   string  `json:"query"` // Public IP Address
}

// Function to get GPS coordinates using an IP Geolocation API
func getGPSCoordinates() (float64, float64, string, error) {
	resp, err := http.Get("http://ip-api.com/json/") // Public IP-based geolocation
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	// Parse JSON response
	var geoData GeoLocation
	if err := json.NewDecoder(resp.Body).Decode(&geoData); err != nil {
		return 0, 0, "", err
	}

	// Build full address
	address := fmt.Sprintf("%s, %s, %s", geoData.City, geoData.Region, geoData.Country)

	return geoData.Lat, geoData.Lon, address, nil
}

// Load font and return freetype context
func loadFontContext(fontPath string, img draw.Image, fontSize float64) (*freetype.Context, error) {
	fontBytes, err := ioutil.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read font file: %w", err)
	}

	parsedFont, err := freetype.ParseFont(fontBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse font: %w", err)
	}

	ctx := freetype.NewContext()
	ctx.SetDPI(72)
	ctx.SetFont(parsedFont)
	ctx.SetFontSize(fontSize)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(color.White))
	ctx.SetHinting(font.HintingFull)

	return ctx, nil
}

// Function to build multi-line text overlay
func buildOverlayText() string {
	// Get DateTime
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Get GPS Coordinates
	// lat, lon, _, err := getGPSCoordinates()
	// if err != nil {
	// 	log.Print(err)
	// 	// address = "Unknown Location"
	// }

	// Build overlay text
	var sb strings.Builder
	sb.WriteString(timestamp)
	// sb.WriteString(fmt.Sprintf("\nLat: %.6f", lat))
	// sb.WriteString(fmt.Sprintf("\nLon: %.6f", lon))
	// // Kantor Cideng
	// sb.WriteString("\nJl. Persatuan Guru No.25 RT.8/RW.5")
	// sb.WriteString("\nPetojo Sel.")
	// sb.WriteString("\nKecamatan Gambir")
	// sb.WriteString("\nKota Jakarta Pusat")
	// sb.WriteString("\nDaerah Khusus Ibukota Jakarta, 10160")

	overlayText := sb.String()
	return overlayText
}

// Function to add text with black outline dynamically positioned
// func addTextToImage(img image.Image, overlayText string) image.Image {
// 	rgba := image.NewRGBA(img.Bounds())
// 	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

// 	// Get image dimensions
// 	imgWidth := img.Bounds().Dx()
// 	imgHeight := img.Bounds().Dy()

// 	// Dynamically adjust font size based on image size
// 	scaleFactor := float64(imgWidth) / 500.0
// 	fontSize := int(13 * scaleFactor)
// 	if fontSize < 13 {
// 		fontSize = 13 // Set minimum size
// 	} else if fontSize > 50 {
// 		fontSize = 50 // Set max limit
// 	}

// 	// Split text into lines and calculate height
// 	lines := strings.Split(overlayText, "\n")
// 	lineHeight := fontSize + 8 // Add spacing for better readability
// 	textBlockHeight := len(lines) * lineHeight

// 	// Define bottom-left padding
// 	paddingX := 20 // Left padding
// 	paddingY := 20 // Bottom padding

// 	// Calculate text starting position (bottom-left)
// 	x := paddingX
// 	y := imgHeight - textBlockHeight - paddingY

// 	// Define text colors
// 	white := image.NewUniform(color.RGBA{255, 255, 255, 255}) // White text
// 	black := image.NewUniform(color.RGBA{0, 0, 0, 255})       // Black outline

// 	// Function to draw text with an outline
// 	drawText := func(dx, dy int, col *image.Uniform) {
// 		d := &font.Drawer{
// 			Dst:  rgba,
// 			Src:  col,
// 			Face: basicfont.Face7x13,
// 			Dot:  fixed.Point26_6{X: fixed.I(x + dx), Y: fixed.I(y + dy)},
// 		}
// 		for _, line := range lines {
// 			d.DrawString(line)
// 			d.Dot.Y += fixed.I(lineHeight) // Move down for each line
// 		}
// 	}

// 	// Draw black outline (shadows around the text)
// 	offsets := []struct{ dx, dy int }{
// 		{-1, -1}, {1, -1}, {-1, 1}, {1, 1}, {0, -1}, {0, 1}, {-1, 0}, {1, 0},
// 	}
// 	for _, o := range offsets {
// 		drawText(o.dx, o.dy, black)
// 	}

// 	// Draw white text on top
// 	drawText(0, 0, white)

// 	return rgba
// }

func addTextToImage(img image.Image, overlayText string, fontPath string) (image.Image, error) {
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	// imgWidth := img.Bounds().Dx()
	imgHeight := img.Bounds().Dy()

	fontSize := float64(imgHeight) * 0.035 // 3.5% of height

	ctx, err := loadFontContext(fontPath, rgba, fontSize)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(overlayText, "\n")
	x := 20
	yStart := imgHeight - (int(fontSize)+10)*len(lines) - 20

	pt := freetype.Pt(x, yStart)
	for _, line := range lines {
		_, err := ctx.DrawString(line, pt)
		if err != nil {
			return nil, err
		}
		pt.Y += ctx.PointToFixed(fontSize + 8)
	}

	return rgba, nil
}

// Function to convert an image to Base64 after adding text
// func fileToBase64(file *multipart.FileHeader) (string, error) {
// 	openedFile, err := file.Open()
// 	if err != nil {
// 		return "", err
// 	}
// 	defer openedFile.Close()

// 	// Decode the image
// 	img, format, err := image.Decode(openedFile)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Ensure format is only JPG, JPEG, or PNG
// 	if format != "jpeg" && format != "png" {
// 		return "", fmt.Errorf("unsupported image format: %s", format)
// 	}

// 	// Get overlay text (datetime, GPS, address)
// 	overlayText := buildOverlayText()

// 	// Add text to the image
// 	imgWithText := addTextToImage(img, overlayText)

// 	// Encode image with text back to bytes
// 	buf := new(bytes.Buffer)
// 	if format == "jpeg" {
// 		err = jpeg.Encode(buf, imgWithText, nil)
// 	} else {
// 		err = png.Encode(buf, imgWithText)
// 	}
// 	if err != nil {
// 		return "", err
// 	}

// 	// Convert modified image to Base64
// 	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
// }

func fileToBase64(file *multipart.FileHeader, isPhotoUsingTimestamp bool) (string, error) {
	yamlCfg := config.GetConfig()

	openedFile, err := file.Open()
	if err != nil {
		return "", err
	}
	defer openedFile.Close()

	img, format, err := image.Decode(openedFile)
	if err != nil {
		return "", fmt.Errorf("unable to decode image: %w", err)
	}

	if format != "jpeg" && format != "png" {
		return "", fmt.Errorf("unsupported format: %s", format)
	}

	var imgToEncode image.Image = img

	if isPhotoUsingTimestamp {
		fontPath := yamlCfg.Default.FontTTFFullPath
		overlayText := buildOverlayText()

		imgWithText, err := addTextToImage(img, overlayText, fontPath)
		if err != nil {
			return "", err
		}
		imgToEncode = imgWithText
	}

	buf := new(bytes.Buffer)
	switch format {
	case "jpeg":
		err = jpeg.Encode(buf, imgToEncode, nil)
	case "png":
		err = png.Encode(buf, imgToEncode)
	}
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func FormCallWhatsappRequest(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	tableCS := config.Db.TbUser
	tableWaReqData := config.Db.TbWaReq

	return func(c *fiber.Ctx) error {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
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
		linkWODetail := fmt.Sprintf("%v:%v/projectTask/detailWO?wo_number=", config.Default.WoDetailServer, config.Default.WoDetailPort)

		var uniqCode, picPhone, randomStr, idCS string
		dataParsed := strings.Split(data, "_")

		if len(dataParsed) >= 4 {
			uniqCode = dataParsed[0]
			picPhone = dataParsed[1]
			randomStr = dataParsed[2]
			idCS = dataParsed[3]
		} else {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": "Invalid data URL !!",
				"ErrorCode":    fiber.StatusBadRequest,
			})
		}
		_ = randomStr

		var adminCSData models.CS
		intID, err := strconv.Atoi(idCS)
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		if err := db.Table(tableCS).Where("id = ?", uint(intID)).First(&adminCSData).Error; err != nil {
			errMsg := fmt.Sprintf("unauthorized: %v", err)
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": errMsg,
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		switch uniqCode {
		case "whatsapp":
			var waReqData []models.WaRequest
			if err := db.Table(tableWaReqData).
				// Where("x_pic_phone = ? AND is_on_calling  = ? AND is_done = ?", picPhone, true, false). // delete this SOON !!!!!
				// Where("x_pic_phone = ? AND is_on_calling  = ? AND is_done = ?",
				// 	picPhone,
				// 	false,
				// 	false,
				// ).
				Where("x_pic_phone = ? AND is_done = ?",
					picPhone,
					false,
				).
				Find(&waReqData).
				Error; err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}

			if len(waReqData) > 0 {
				// Set pic phone is on calling
				if err := db.Table(tableWaReqData).
					Where("x_pic_phone = ?", picPhone).
					Updates(map[string]interface{}{
						"is_on_calling":          true,
						"is_on_calling_datetime": now,
						"temp_cs":                idCS,
					}).
					Error; err != nil {
					return c.Render("error_page", fiber.Map{
						"ErrorMessage": err.Error(),
						"ErrorCode":    fiber.StatusInternalServerError,
					})
				}

				var reqType []string
				var picAndpicphoneInfo []string
				var snedcAndedctypeInfo []string
				var merchantAndstatusmerchantInfo []string
				var jenisKunjunganInfo []string
				var merchantName []string

				// Variables for Greetings
				var vendorSouce string
				var picSingleName string
				var picMerchants []string

				// Process map data
				for _, data := range waReqData {
					reqType = append(reqType, data.RequestType)
					picAndpicphoneInfo = append(picAndpicphoneInfo, fmt.Sprintf("<p class='mb-0'>%v (0%v)</p>", data.PicMerchant, data.PicPhone))
					snedcAndedctypeInfo = append(snedcAndedctypeInfo, fmt.Sprintf("<p class='mb-0'><span class='fw-bold'>%v</span> #%v</p>", data.EdcType, data.SnEdc))

					if data.MerchantName != "" {
						merchantName = append(merchantName, data.MerchantName)
					}

					var geoLocation string
					if data.Longitude != "0.0" && data.Latitude != "0.0" {
						geoLocation = fmt.Sprintf("<a href='https://www.google.com/maps?q=%v,%v' target='_blank'>%v</a>", data.Latitude, data.Longitude, data.MerchantName)
					} else {
						geoLocation = data.MerchantName
					}

					merchantAndstatusmerchantInfo = append(merchantAndstatusmerchantInfo, fmt.Sprintf("<p class='mb-0'><span class='fw-bold'>%v</span> - %v</p>", geoLocation, data.TechnicianName))

					if data.Source != "" {
						if strings.HasPrefix(strings.ToLower(data.Source), "bmri") {
							data.Source = strings.Replace(strings.ToLower(data.Source), "bmri", "MANDIRI", 1)
						}
						vendorSouce = data.Source
					}

					if data.PicMerchant != "" {
						picSingleName = strings.ToUpper(data.PicMerchant)
					}

					if data.MerchantName != "" {
						picMerchants = append(picMerchants, data.MerchantName)
					}

					switch strings.ToLower(data.TaskType) {
					case "preventive maintenance":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "pemeliharaan rutin & pencegahan terhadap EDC agar memastikan EDC dalam kondisi normal")
					case "corrective maintenance":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "perbaikan kerusakan atau masalah pada EDC")
					case "withdrawal":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "penarikan perangkat EDC dari lokasi merchant")
					case "installation":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "pemasangan perangkat EDC baru (Installation)")
					case "replacement":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "penggantian perangkat EDC lama dengan yang baru")
					case "pindah vendor":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "proses pemindahan layanan ke vendor lain")
					case "re-init":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "inisialisasi ulang perangkat EDC")
					case "roll out":
						jenisKunjunganInfo = append(jenisKunjunganInfo, "distribusi atau implementasi perangkat EDC ke lokasi baru")
					default:
						jenisKunjunganInfo = append(jenisKunjunganInfo, data.TaskType)
					}
				}

				infoforPIC := processInfo(picAndpicphoneInfo)
				infoforEDC := processInfo(snedcAndedctypeInfo)
				infoforMerchant := processInfo(merchantAndstatusmerchantInfo)

				reqTypeOutput := strings.Join(reqType, ", ")
				merchantNameOutput := strings.Join(merchantName, ", ")
				picMerchants = slices.Compact(picMerchants)
				listofMerchantsforsinglePIC := strings.Join(picMerchants, ", ")
				listofJenisKunjunganforsinglePIC := removeDuplicates(jenisKunjunganInfo)

				cardHeaderInfo := template.HTML(fmt.Sprintf(`
				<div class="card-widget-separator-wrapper bg-label-info text-secondary">
					<div class="card-body card-widget-separator">
						<div class="row gy-4 gy-sm-1">
							<div class="col-sm-6 col-lg-2 mb-3">
								<div class="d-flex justify-content-between align-items-start card-widget-1 border-end pb-3 pb-sm-0">
									<div>
										<h3 class="mb-1">%v</h3>
										<p class="mb-0">CC Team</p>
									</div>
									<span class="badge bg-info rounded p-2 me-sm-4">
										<i class="bx bx-user bx-sm"></i>
									</span>
								</div>
								<hr class="d-none d-sm-block d-lg-none me-4" />
							</div>
							<div class="col-sm-6 col-lg-4 mb-3">
								<div class="d-flex justify-content-between align-items-start card-widget-2 border-end pb-3 pb-sm-0">
									<div>
										<h3 class="mb-1">Info PIC</h3>
										%v
									</div>
									<span class="badge bg-info rounded p-2 me-lg-4">
										<i class="bx bxs-user-voice bx-sm"></i>
									</span>
								</div>
								<hr class="d-none d-sm-block d-lg-none" />
							</div>
							<div class="col-sm-6 col-lg-6 mb-3">
								<div class="d-flex justify-content-between align-items-start border-end pb-3 pb-sm-0 card-widget-3">
									<div>
										<h3 class="mb-1">EDC di Lokasi Merchant</h3>
										%v
									</div>
									<span class="badge bg-info rounded p-2 me-sm-4">
										<i class="bx bx-mobile-vibration bx-sm"></i>
									</span>
								</div>
							</div>
							<div class="col-sm-6 col-lg-12 mb-3">
								<div class="d-flex justify-content-between align-items-start">
									<div>
										<h3 class="mb-1">Info Merchant</h3>
										%v
									</div>
									<span class="badge bg-info rounded p-2">
										<i class="bx bxs-store bx-sm"></i>
									</span>
								</div>
							</div>
						</div>
					</div>
				</div>
				`,
					adminCSData.Username,
					infoforPIC,
					infoforEDC,
					infoforMerchant,
				))

				textPresentation := template.HTML(fmt.Sprintf(`
					<h2>
						Halo, %v. Perkenalkan saya <i>%v</i> dari tim Call Center Manage Service EDC %v.
						<br><br> Apakah benar saya berbicara dengan Bapak/Ibu <b>%v</b> dari <i>%v</i> ?
						<br><br>
						Saya ingin mengkonfirmasi, terkait kunjungan teknisi kami ke tempat Bapak/Ibu untuk: <i>%v</i>.
						<br><br> Mohon Bapak/Ibu memastikan tanggal dan waktu kunjungan, agar teknisi kami dapat berkunjung ke tempat Bapak/Ibu. Kira-kira kapan waktu yang tersedia untuk kunjungan tersebut, Pak/Bu?
					</h2>
				`,
					greeting,
					adminCSData.Username,
					vendorSouce,
					picSingleName,
					listofMerchantsforsinglePIC,
					listofJenisKunjunganforsinglePIC,
				))

				// Return Pretty formatted Details Data
				// for i := range waReqData {
				// 	if waReqData[i].PlanDate != nil {
				// 		formattedDateStr := waReqData[i].PlanDate.Format("02/01/2006")
				// 		newTime, err := time.Parse("02/01/2006", formattedDateStr)
				// 		if err == nil {
				// 			waReqData[i].PlanDate = &newTime
				// 		}
				// 	}

				// 	// if waReqData[i].TargetScheduleDate != nil {
				// 	// 	formattedDateStr := waReqData[i].TargetScheduleDate.Format("02/01/2006")
				// 	// 	newTime, err := time.Parse("02/01/2006", formattedDateStr)
				// 	// 	if err == nil {
				// 	// 		waReqData[i].TargetScheduleDate = &newTime // Assign new time.Time pointer
				// 	// 	}
				// 	// }
				// }

				// endpoint get RC by company
				getRCByCompany := fmt.Sprintf("%v:%v/here/listReason",
					config.Default.FilestoreMWServer,
					config.Default.FilestoreMWTAPort,
				)

				return c.Render("whatsapp_request", fiber.Map{
					"Title":            picPhone + " | " + merchantNameOutput + " | [REQUEST] " + reqTypeOutput,
					"CardHeaderInfo":   cardHeaderInfo,
					"TextPresentation": textPresentation,
					"PhoneNumber":      picPhone,
					"TempCS":           intID,
					"DetailsData":      waReqData,
					"LinkWODetail":     linkWODetail,
					"EndpointGetRC":    getRCByCompany,
				})
			} else {
				// Check if its being check by another CS Admin
				var checkedData models.WaRequest
				today := time.Now().Truncate(24 * time.Hour)
				if err := db.Table(tableWaReqData).Where("x_pic_phone = ? AND updated_at >= ?", picPhone, today).First(&checkedData).Error; err != nil {
					return c.Render("error_page", fiber.Map{
						"ErrorMessage": fmt.Sprintf("Maaf, data tidak dapat diproses! Nomor PIC mungkin sudah dihubungin atau datanya telah hilang. ~Details: %v", err.Error()),
						"ErrorCode":    fiber.StatusBadRequest,
					})
				}

				var lastUpdatedBY string
				if checkedData.TempCS != 0 {
					var dataCS models.CS
					if err := db.Table(tableCS).Where("id = ?", uint(checkedData.TempCS)).First(&dataCS).Error; err != nil {
						return c.Render("error_page", fiber.Map{
							"ErrorMessage": fmt.Sprintf("Maaf, data tidak dapat diproses! Nomor PIC mungkin sudah dihubungin atau datanya telah hilang. ~Detail CS Error: %v", err.Error()),
							"ErrorCode":    fiber.StatusBadRequest,
						})
					}
					lastUpdatedBY = dataCS.Username
				} else {
					lastUpdatedBY = checkedData.LastUpdateBy
				}

				var statusData string
				if checkedData.IsOnCalling {
					if checkedData.IsDone {
						statusData = "sudah difollow up"
					} else {
						statusData = "sementara difollow up"
					}
				} else {
					statusData = "direset"
				}

				pesan := fmt.Sprintf("Maaf, data dengan nomor telepon (%v) - %v [%v] mungkin %v oleh %v. Terakhir update pada: %v",
					checkedData.PicPhone,
					checkedData.PicMerchant,
					checkedData.MerchantName,
					statusData,
					lastUpdatedBY,
					checkedData.UpdatedAt.Format("02 Jan 2006 15:04:05"),
				)
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": pesan,
					"ErrorCode":    fiber.StatusBadRequest,
				})
			} // .end of case 'whatsapp'
		default:
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": "Request TIDAK DITEMUKAN !!!",
				"ErrorCode":    fiber.StatusBadRequest,
			})
		}
	}
}

// func MarkAsCalled(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
// 	tableWaReqData := config.Db.TbWaReq
// 	return func(c *fiber.Ctx) error {
// 		var requestData RequestData
// 		if err := c.BodyParser(&requestData); err != nil {
// 			log.Println("Error parsing JSON:", err)
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"message": "Invalid request data!",
// 			})
// 		}

// 		if requestData.PhoneNumber == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"message": "Nomor telepon tidak boleh kosong!",
// 			})
// 		}

// 		if requestData.TempCS == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"message": "Temp CS tidak boleh kosong!",
// 			})
// 		}

// 		var waReqData models.WaRequest
// 		if err := db.Table(tableWaReqData).
// 			Where("is_on_calling = ? AND is_done = ? AND x_pic_phone = ? AND temp_cs = ?",
// 				true,
// 				false,
// 				requestData.PhoneNumber,
// 				requestData.TempCS,
// 			).
// 			First(&waReqData).
// 			Error; err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"message": err.Error(),
// 			})
// 		}

// 		if err := db.Table(tableWaReqData).
// 			Where("is_on_calling = ? AND is_done = ? AND x_pic_phone = ? AND temp_cs = ?",
// 				true,
// 				false,
// 				requestData.PhoneNumber,
// 				requestData.TempCS,
// 			).
// 			Update("is_done", true).
// 			Error; err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"message": err.Error(),
// 			})
// 		}

// 		if waReqData.StanzaId != "" && waReqData.OriginalSenderJid != "" && waReqData.GroupWaJid != "" {
// 			// Trigger feedback result to requestor
// 			whatsapp.TriggerGetFeedbackFromFU <- whatsapp.FeedbackTriggerData{
// 				Config:            config,
// 				Database:          db,
// 				WhatsappClient:    Client,
// 				RequestInWhatsapp: waReqData.RequestType,
// 				PicPhoneNumber:    waReqData.PicPhone,
// 				StanzaID:          waReqData.StanzaId,
// 				OriginalSenderJID: waReqData.OriginalSenderJid,
// 				GroupWAJID:        waReqData.GroupWaJid,
// 			}
// 		}

// 		return c.SendStatus(fiber.StatusOK)
// 	}
// }

func SubmitFormCallWhatsappRequest(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	table := config.Db.TbWaReq
	tableCs := config.Db.TbUser

	return func(c *fiber.Ctx) error {
		idJO := c.FormValue("id")
		idCS := c.FormValue("idCS")
		orderWish := c.FormValue("orderWish")
		woNumber := c.FormValue("woNumber")
		pic := c.FormValue("pic")
		picPhone := c.FormValue("picPhone")
		reSchedule := c.FormValue("isReschedule")
		dateReschedule := c.FormValue("dateReschedule")
		additionalNotes := c.FormValue("additionalNotes")
		nextFollowUpTo := c.FormValue("nextFollowUpTo")

		intIDJo, err := strconv.Atoi(idJO)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		intIDCs, err := strconv.Atoi(idCS)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		var pathImgWA, pathImgMerchant, pathImgSNEdc *string

		imgVars := map[string]**string{
			"imgWA":       &pathImgWA,
			"imgMerchant": &pathImgMerchant,
			"imgSNEDC":    &pathImgSNEdc,
		}

		for fieldName, variablePointer := range imgVars {
			file, err := c.FormFile(fieldName)
			if err != nil {
				log.Printf("No file uploaded for %s, skipping...", fieldName)
				continue // Skip if no file uploaded
			}

			// Validate image format
			if !isValidImage(file) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": fmt.Sprintf("Invalid image file type for field %s, must be JPG, JPEG, or PNG", fieldName),
				})
			}

			// Generate Folder & Filename
			folderPath := filepath.Join("filestore", time.Now().Format("2006/01/02")) // YYYY/mm/dd
			filename := fmt.Sprintf("%d_%s_%d%s", intIDJo, fieldName, time.Now().UnixNano(), filepath.Ext(file.Filename))
			fullFilePath := filepath.Join(folderPath, filename)

			log.Printf("🔹 Saving image: %s", fullFilePath)

			// ✅ **Ensure Directory Exists With Correct Permissions**
			if err := os.MkdirAll(folderPath, 0755); err != nil { // Force 777 for testing
				log.Printf("❌ Failed to create directory: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to create directory: %v", err),
				})
			}

			// ✅ **Manually Open and Copy File Instead of c.SaveFile**
			srcFile, err := file.Open()
			if err != nil {
				log.Printf("❌ Failed to open uploaded file: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to open uploaded file: %v", err),
				})
			}
			defer srcFile.Close()

			// ✅ **Create the Destination File Manually**
			dstFile, err := os.Create(fullFilePath)
			if err != nil {
				log.Printf("❌ Failed to create destination file: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to create destination file: %v", err),
				})
			}
			defer dstFile.Close()

			// ✅ **Copy File Data**
			if _, err := io.Copy(dstFile, srcFile); err != nil {
				log.Printf("❌ Failed to copy file data: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to copy file data: %v", err),
				})
			}

			// ✅ **Ensure the Path is Properly Assigned**
			*variablePointer = &fullFilePath

			log.Printf("✅ Successfully saved: %s", fullFilePath)
		}

		var isReschedule bool
		if reSchedule == "on" {
			isReschedule = true
		} else {
			isReschedule = false
		}

		var dateToReschedule *time.Time
		if dateReschedule != "" {
			parsedDate, err := time.Parse("2006-01-02", dateReschedule)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to parse date: %v", err),
				})
			} else {
				dateToReschedule = &parsedDate
			}
		}

		var csData models.CS
		if err := db.Table(tableCs).
			Where("id = ?", intIDCs).
			First(&csData).
			Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check the exists call center: %v", err),
			})
		}

		var dataWaReq models.WaRequest

		if woNumber != "" {
			if err := db.Table(table).
				// Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber). // i think here u add the condition to check if its already done or not ?
				Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber).
				First(&dataWaReq).
				Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to check the exists wo: %v", err),
				})
			}
		} else {
			if err := db.Table(table).
				// Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber). // i think here u add the condition to check if its already done or not ?
				// Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber). // not use again coz use non odoo data
				Where("id = ?", uint(intIDJo)).
				First(&dataWaReq).
				Error; err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": fmt.Sprintf("failed to check the exists wo: %v", err),
				})
			}
		}

		// Update the data
		dataWaReq.PicMerchant = pic
		dataWaReq.PicPhone = picPhone
		dataWaReq.IsReschedule = isReschedule
		dataWaReq.PlanDate = dateToReschedule
		dataWaReq.CallCenterMessage = additionalNotes
		if strings.ToLower(dataWaReq.OrderWish) == "re-confirm" {
			dataWaReq.Keterangan = "Only need confirmation to PIC merchant"
		} else {
			dataWaReq.Keterangan = "Need for being updated to ODOO"
		}
		dataWaReq.IsDone = true
		dataWaReq.IsOnCalling = true
		dataWaReq.TempCS = intIDCs
		if pathImgWA != nil {
			dataWaReq.ImgWaPath = *pathImgWA
		} else {
			dataWaReq.ImgWaPath = ""
		}
		if pathImgMerchant != nil {
			dataWaReq.ImgMerchantPath = *pathImgMerchant
		} else {
			dataWaReq.ImgMerchantPath = ""
		}
		if pathImgSNEdc != nil {
			dataWaReq.ImgSnEdcPath = *pathImgSNEdc
		} else {
			dataWaReq.ImgSnEdcPath = ""
		}
		dataWaReq.LastUpdateBy = csData.Username
		dataWaReq.NextFollowUpTo = nextFollowUpTo

		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": err.Error(),
			})
		}

		// Get current time in Jakarta timezone
		now := time.Now().In(loc)
		dataWaReq.IsDoneDatetime = &now

		if err := db.Table(table).Save(dataWaReq).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": err.Error(),
			})
		}

		// Trigger for save the img files to filestore server -> running on background
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
				"id": idJO,
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
			fileStoreURL := fmt.Sprintf("%v/submit_cc_req_image", config.Default.FilestoreServer)
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

		// ********************************************************************************
		// switch strings.ToLower(orderWish) {
		// case "merchant confirmation", "merchant confirmation & technician will visit":
		// }
		// ********************************************************************************
		// Check if data is ODOO or Non ODOO
		_ = orderWish
		if woNumber != "" {
			// Trigger for updated the data in ODOO
			whatsapp.TriggerUpdateDatainODOO <- whatsapp.UpdatedODOODataTriggerItem{
				Config:   config,
				Database: db,
				TaskID:   uint(intIDJo),
			}
		}

		// Trigger to send feedback in WAG
		var feedbackWONumber string
		var feedbackSPKNumber string
		if dataWaReq.WoNumber != "" {
			feedbackWONumber = dataWaReq.WoNumber
		}
		if dataWaReq.HelpdeskTicketName != "" {
			feedbackSPKNumber = dataWaReq.HelpdeskTicketName
		}

		whatsapp.TriggerGetFeedbackFromFU <- whatsapp.FeedbackTriggerData{
			Config:            config,
			Database:          db,
			WhatsappClient:    Client,
			StanzaID:          dataWaReq.StanzaId,
			OriginalSenderJID: dataWaReq.OriginalSenderJid,
			GroupWAJID:        dataWaReq.GroupWaJid,
			RequestInWhatsapp: dataWaReq.RequestType,
			PicPhoneNumber:    dataWaReq.PicPhone,
			WoNumber:          feedbackWONumber,
			SpkNumber:         feedbackSPKNumber,
		}

		return c.JSON(fiber.Map{
			"message": "Form submitted successfully",
		})
	}

}

func JoAlreadyVisitByTechnician(idJO int) bool {
	config := config.GetConfig()

	if idJO == 0 {
		return false
	}

	odooModel := "project.task"
	odooDomain := []interface{}{
		[]interface{}{"id", "=", idJO},
	}

	odooFields := []string{
		"id",
		"stage_id",
		"timesheet_timer_last_stop",
	}
	odooOrder := "id asc"
	odooParams := map[string]interface{}{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	odooRequest := map[string]interface{}{
		"jsonrpc": config.ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	odooPayload, err := json.Marshal(odooRequest)
	if err != nil {
		log.Print(err)
		return false
	}

	result, err := getDatafromODOO(&config, string(odooPayload))
	if err != nil {
		log.Print(err)
		return false
	}

	resultArray, ok := result.([]interface{})
	if !ok {
		return false
	}

	if len(resultArray) == 0 {
		return false
	}

	for _, record := range resultArray {
		var odooData MidtidTaskItem
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			return false
		}

		jsonData, err := json.Marshal(recordMap)
		if err != nil {
			log.Print(err)
			return false
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			log.Print(err)
			return false
		}

		stageId, stage, err := parseJSONIDDataCombined(odooData.StageId)
		if err != nil {
			log.Print(err)
			return false
		}

		var timesheetLastStop *time.Time
		if !odooData.TimesheetLastStop.Time.IsZero() {
			adjustedTime := odooData.TimesheetLastStop.Time.Add(7 * time.Hour)
			timesheetLastStop = &adjustedTime
		}

		if stageId == 5 && stage == "Done" && timesheetLastStop != nil {
			return true
		}
	}

	// Default
	return false
}

func SubmitFormEditedJO(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	table := config.Db.TbWaReq
	tableCs := config.Db.TbUser

	return func(c *fiber.Ctx) error {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": err.Error(),
			})
		}

		timesheetTimerFirstStart := time.Now().In(loc).Add(-7 * time.Hour).Format("2006-01-02 15:04:05")

		idJO := c.FormValue("edit-id")
		idCS := c.FormValue("edit-idCS")
		idRC := c.FormValue("edit-reason_code")
		idStage := c.FormValue("edit-stage")
		woNumber := c.FormValue("edit-woNumber")
		// company := c.FormValue("edit-company")
		woRemark := c.FormValue("edit-wo_remark")
		additionalNotes := c.FormValue("edit-additionalNotes")
		// pic := c.FormValue("pic")
		// picPhone := c.FormValue("picPhone")
		// Status Merchant
		merchantStatus := c.FormValue("edit-merchant_status")
		statusAlamatMerchant := c.FormValue("edit-status_alamat_merchant")
		requestMerchant := c.FormValue("edit-request_merchant")
		priorityEdc := c.FormValue("edit-priority_edc")
		// Kondisi & Kelengkapan EDC
		adaptor := c.FormValue("edit-adaptor")
		statusEdc := c.FormValue("edit-status_edc")
		kondisiEdc := c.FormValue("edit-kondisi_edc")
		kondisiDetailEdc := c.FormValue("edit-kondisi_detail_edc")
		signalBarJaringan := c.FormValue("edit-signal_bar_jaringan")
		signalTypeJaringan := c.FormValue("edit-signal_type")
		usePhotoTimestamp := c.FormValue("photoTimestamp")

		var isPhotoUsingTimestamp bool
		if usePhotoTimestamp == "on" {
			isPhotoUsingTimestamp = true
		} else {
			isPhotoUsingTimestamp = false
		}

		intIDJo, err := strconv.Atoi(idJO)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		intIDCs, err := strconv.Atoi(idCS)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		intIDRC, err := strconv.Atoi(idRC)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		intIDStage, err := strconv.Atoi(idStage)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// List of required fields
		requiredFields := map[string]string{
			"merchantStatus":       merchantStatus,
			"statusAlamatMerchant": statusAlamatMerchant,
			"requestMerchant":      requestMerchant,
			"priorityEdc":          priorityEdc,
			"adaptor":              adaptor,
			"statusEdc":            statusEdc,
			"kondisiEdc":           kondisiEdc,
			"kondisiDetailEdc":     kondisiDetailEdc,
			"signalBarJaringan":    signalBarJaringan,
			"signalTypeJaringan":   signalTypeJaringan,
		}

		// Check if any required field is empty
		for field, value := range requiredFields {
			if value == "" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"message": field + " cannot be empty",
				})
			}
		}

		var csData models.CS
		if err := db.Table(tableCs).
			Where("id = ?", intIDCs).
			First(&csData).
			Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check the exists call center cs: %v", err),
			})
		}

		var dataWaReq models.WaRequest
		if err := db.Table(table).
			Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber).
			First(&dataWaReq).
			Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("failed to check the exists wo: %v", err),
			})
		}

		if dataWaReq.HelpdeskTicketId == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf("Ticket ID not found for WO Number: %v", woNumber),
			})
		}

		technicianAlreadyVisit := JoAlreadyVisitByTechnician(intIDJo)
		linkWODetail := fmt.Sprintf("%v:%v/projectTask/detailWO?wo_number=%v", config.Default.WoDetailServer, config.Default.WoDetailPort, woNumber)
		if technicianAlreadyVisit {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": fmt.Sprintf(
					"Kami mohon maaf, karena JO dengan WO Number: %v dan Ticket Subject: %v mungkin sudah dikunjungi oleh Teknisi %v. Coba buka detail WO Number untuk melihat detail dari kunjungan teknisi tersebut. <br> Atau klik tombol berikut <button class='btn mt-2 btn-sm btn-secondary w-100' onclick=\"window.open('%v', '_blank')\">Detail WO Number</button>",
					woNumber, dataWaReq.HelpdeskTicketName, dataWaReq.TechnicianName, linkWODetail,
				),
			})
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("Last update by Call Center: %v", time.Now().Format("2006-01-02 15:04:05")))
		if additionalNotes != "" {
			sb.WriteString(fmt.Sprintf("; Notes: %v", additionalNotes))
		}
		sb.WriteString(fmt.Sprintf(" ~%v", csData.Username))
		ccMsg := sb.String()

		// Declare 11 Base64 string variables
		var base64FotoEDC, base64FotoBAST, base64FotoCeklis, base64FotoToko string
		var base64FotoTransaksi, base64FotoPIC, base64FotoSetting, base64FotoThermal string
		var base64FotoTraining, base64TandaTanganPIC, base64TandaTanganTeknisi string

		// List of image fields mapped to corresponding variables
		imageFields := map[string]*string{
			"x_foto_edc":             &base64FotoEDC,
			"x_foto_bast":            &base64FotoBAST,
			"x_foto_ceklis":          &base64FotoCeklis,
			"x_foto_toko":            &base64FotoToko,
			"x_foto_transaksi":       &base64FotoTransaksi,
			"x_foto_pic":             &base64FotoPIC,
			"x_foto_setting":         &base64FotoSetting,
			"x_foto_thermal":         &base64FotoThermal,
			"x_foto_training":        &base64FotoTraining,
			"x_tanda_tangan_pic":     &base64TandaTanganPIC,
			"x_tanda_tangan_teknisi": &base64TandaTanganTeknisi,
		}

		// Loop through each image field
		for fieldName, base64Var := range imageFields {
			file, err := c.FormFile(fieldName)

			if err == nil { // File is attached
				if !isValidImage(file) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"message": fmt.Sprintf("Invalid image file type for field %s, must be JPG, JPEG, or PNG", fieldName),
					})
				}

				// Convert to Base64
				base64Str, err := fileToBase64(file, isPhotoUsingTimestamp)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"message": fmt.Sprintf("Failed to process file %s got error: %s", fieldName, err.Error()),
					})
				}

				// Assign Base64 string to corresponding variable
				*base64Var = base64Str
			}
		}

		// Set the technician first to Call Center in Helpdesk.Ticket
		odooParams := map[string]interface{}{
			"model":         "helpdesk.ticket",
			"id":            dataWaReq.HelpdeskTicketId,
			"technician_id": 3476, // Call Center
		}
		odooRequest := map[string]interface{}{
			"jsonrpc": config.ApiODOO.JSONRPC,
			"params":  odooParams,
		}

		odooJsonData, err := json.Marshal(odooRequest)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		newODOOCookies, err := getODOOCookies(config.ApiODOO.Login, config.ApiODOO.Password, config)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		err = updateDataToODOO(config, newODOOCookies, string(odooJsonData))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// *************** Send to MW FS Kukuh
		timesheetTimerLastStop := time.Now().In(loc).Add(-7 * time.Hour).Format("2006-01-02 15:04:05")

		params := map[string]interface{}{
			"model": "project.task",
			"id":    idJO,
		}

		params["stage_id"] = intIDStage
		params["x_reason_code_id"] = intIDRC
		params["timesheet_timer_first_start"] = timesheetTimerFirstStart
		params["timesheet_timer_last_stop"] = timesheetTimerLastStop
		params["x_message_call"] = ccMsg
		params["x_keterangan"] = woRemark
		// Status Merchant
		params["x_status_merchant"] = merchantStatus
		params["x_status_alamat_merchant"] = statusAlamatMerchant
		params["x_request_merchant"] = requestMerchant
		params["x_priority_edc"] = priorityEdc
		// Kondisi & Kelengkapan EDC
		params["x_adaptor_edc"] = adaptor
		params["x_status_edc"] = statusEdc
		params["x_condition_edc"] = kondisiEdc
		params["x_kondisi_detail_edc"] = kondisiDetailEdc
		params["x_kondisi_jaringan"] = signalBarJaringan
		params["x_signal_type"] = signalTypeJaringan

		imageFieldsCheck := map[string]string{
			"x_foto_edc":             base64FotoEDC,
			"x_foto_bast":            base64FotoBAST,
			"x_foto_ceklis":          base64FotoCeklis,
			"x_foto_toko":            base64FotoToko,
			"x_foto_transaksi":       base64FotoTransaksi,
			"x_foto_pic":             base64FotoPIC,
			"x_foto_setting":         base64FotoSetting,
			"x_foto_thermal":         base64FotoThermal,
			"x_foto_training":        base64FotoTraining,
			"x_tanda_tangan_pic":     base64TandaTanganPIC,
			"x_tanda_tangan_teknisi": base64TandaTanganTeknisi,
		}

		// Loop through image fields and append only non-empty values
		for field, base64Value := range imageFieldsCheck {
			if base64Value != "" {
				params[field] = base64Value
			}
		}

		request := map[string]interface{}{
			"params": params,
		}

		// Convert params to JSON
		jsonData, err := json.Marshal(request)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		endpointMWFSKukuh := fmt.Sprintf("%v:%v/odoo/update/full", config.Default.FilestoreMWServer, config.Default.FilestoreMWPort)
		req, err := http.NewRequest("POST", endpointMWFSKukuh, bytes.NewBuffer(jsonData))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", config.Default.HeaderAuthFSMWKukuh)

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}
		defer resp.Body.Close()

		// Read the response body
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": err.Error(),
			})
		}

		// // Debug json params comment soon if needed
		// log.Printf("JSON Params Send to Kukuh: %s", jsonData)
		// log.Printf("Response from Kukuh: %s", string(bodyBytes))

		var responseMap map[string]string
		jsonErr := json.Unmarshal(bodyBytes, &responseMap)

		if jsonErr == nil {
			// Successfully decoded as JSON map
			if resp.StatusCode == 200 && responseMap["success"] == "success" {
				// Update status in DB
				if err := db.Table(table).
					Where("id = ? AND x_no_task = ?", uint(intIDJo), woNumber).
					Updates(map[string]interface{}{
						"is_done_datetime":    time.Now(),
						"last_update_by":      csData.Username,
						"keterangan":          "Success updated to ODOO",
						"call_center_message": ccMsg,
						"updated_to_odoo":     true,
						"is_done":             true,
					}).Error; err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"message": err.Error(),
					})
				}

				// i think SOON in here u must trigger to update via WA ??

				return c.Status(fiber.StatusOK).JSON(fiber.Map{
					"message": fmt.Sprintf("WO Number: %v yang telah diubah berhasil untuk disubmit di ODOO!", woNumber),
					"idJO":    idJO,
					"notes":   additionalNotes,
					"rc":      intIDRC,
					"stage":   intIDStage,
					"data": fiber.Map{
						"wo_remark":              woRemark,
						"merchant_status":        merchantStatus,
						"status_alamat_merchant": statusAlamatMerchant,
						"request_merchant":       requestMerchant,
						"priority_edc":           priorityEdc,
						"adaptor":                adaptor,
						"status_edc":             statusEdc,
						"kondisi_edc":            kondisiEdc,
						"kondisi_detail_edc":     kondisiDetailEdc,
						"signal_bar_jaringan":    signalBarJaringan,
						"signal_type_jaringan":   signalTypeJaringan,
					},
					"images": fiber.Map{
						"x_foto_edc":             base64FotoEDC,
						"x_foto_bast":            base64FotoBAST,
						"x_foto_ceklis":          base64FotoCeklis,
						"x_foto_toko":            base64FotoToko,
						"x_foto_transaksi":       base64FotoTransaksi,
						"x_foto_pic":             base64FotoPIC,
						"x_foto_setting":         base64FotoSetting,
						"x_foto_thermal":         base64FotoThermal,
						"x_foto_training":        base64FotoTraining,
						"x_tanda_tangan_pic":     base64TandaTanganPIC,
						"x_tanda_tangan_teknisi": base64TandaTanganTeknisi,
					},
				})
			} else {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"message": fmt.Sprintf("Maaf, Anda mendapatkan error code: %d dengan pesan: %v", resp.StatusCode, responseMap),
				})
			}
		}

		// If JSON decoding fails, treat it as a raw string response
		responseString := string(bodyBytes)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": fmt.Sprintf("Maaf, Anda mendapatkan error code: %d dengan pesan: %s", resp.StatusCode, responseString),
		})
	}
}

func updateDataToODOO(config *config.YamlConfig, odooSessionCookies []*http.Cookie, req string) error {
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

		for _, cookie := range odooSessionCookies {
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
			return errors.New("got error here1")
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
			return errors.New("got error here2")
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

func getODOOCookies(email string, password string, yamlCfg *config.YamlConfig) ([]*http.Cookie, error) {
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

func GetWhatsappMention(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		search := c.Query("search") // Get search term from Select2

		var mentions []models.WaMention

		// Fetch data with optional search filter
		query := db.Table(config.Db.TbWaMention)
		if search != "" {
			query = query.Where("LOWER(contact_name) LIKE LOWER(?)", "%"+search+"%") // For MySQL/SQLite
			// If using PostgreSQL: query = query.Where("contact_name ILIKE ?", "%"+search+"%")
		}

		result := query.Find(&mentions)
		if result.Error != nil {
			fmt.Println("DB Error:", result.Error)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch data",
			})
		}

		// If no data found, return empty JSON
		if len(mentions) == 0 {
			return c.JSON([]fiber.Map{})
		}

		// Format response for Select2
		groupedData := make(map[string][]fiber.Map)
		for _, mention := range mentions {
			groupedData[mention.Category] = append(groupedData[mention.Category], fiber.Map{
				"id":   mention.ContactPhone, // ID is the contact phone
				"text": mention.ContactName,  // Display name
			})
		}

		// Convert grouped data to the required format
		var formattedResults []fiber.Map
		for category, items := range groupedData {
			formattedResults = append(formattedResults, fiber.Map{
				"text":     category, // Category as optgroup
				"children": items,    // Contacts under that category
			})
		}

		return c.JSON(formattedResults) // Return JSON response
	}
}

func ShowMerchantDetailData(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data := c.Query("data")

		if data == "" {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": "Data not found",
				"ErrorCode":    fiber.ErrBadRequest,
			})
		}

		data = strings.TrimSpace(data)
		decodedData, err := url.QueryUnescape(data)
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		// Get data from Customers (res.partner)
		odooModel := "res.partner"
		odooDomain := []interface{}{
			[]interface{}{"name", "=", decodedData},
		}
		odooFields := []string{
			"id",
			"name",
			"task_ids",
			"x_ticket_ids",
			"technician_id",
			"x_merchant",
			"x_merchant_code",
			"x_merchant_group_code",
			"x_merchant_group_name",
			"x_alamat_pengiriman_edc",
			"x_contact_person",
			"phone",
			"mobile",
			"contact_address",
			"x_merchant_pic",
			"x_merchant_pic_phone",
			"x_studio_sn_edc",
			"x_product",
			"x_simcard",
			"x_simcard_provider",
			"x_msisdn_sim_card",
			"iccid_simcard",
			"x_cimb_mid",
			"x_cimb_tid",
			"partner_latitude",
			"partner_longitude",
			"x_service_point",
			"merchant_last_status",
		}
		odooOrder := "id asc"
		odooParams := map[string]interface{}{
			"model":  odooModel,
			"domain": odooDomain,
			"fields": odooFields,
			"order":  odooOrder,
		}

		odooRequest := map[string]interface{}{
			"jsonrpc": config.ApiODOO.JSONRPC,
			"params":  odooParams,
		}

		odooPayload, err := json.Marshal(odooRequest)
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		result, err := getDatafromODOO(config, string(odooPayload))
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		resultArray, ok := result.([]interface{})
		if !ok {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": "failed to assert results as []interface{}",
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		if len(resultArray) != 1 {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": "Empty ODOO data for current MIDTID",
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		var midtidData OdooResPartnerItem
		var technicianName, snEdc, tipeEdc, simCard, simCardProvider string
		var totalTask, totalTicket int
		var taskIds []int
		var ticketIds []int
		for i, record := range resultArray {
			recordMap, ok := record.(map[string]interface{})
			if !ok {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": fmt.Sprintf("[%v] invalid record format in midtid resultArray", i),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}

			jsonData, err := json.Marshal(recordMap)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}

			err = json.Unmarshal(jsonData, &midtidData)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": fmt.Sprintf("failed to unmarshal into midtidData struct: %v", err),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}

			_, technicianName, err = parseJSONIDDataCombined(midtidData.TechnicianId)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}
			_, snEdc, err = parseJSONIDDataCombined(midtidData.SnEdcId)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}
			_, tipeEdc, err = parseJSONIDDataCombined(midtidData.TipeEdcId)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}
			_, simCard, err = parseJSONIDDataCombined(midtidData.SimCardId)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}
			_, simCardProvider, err = parseJSONIDDataCombined(midtidData.SimCardProviderId)
			if err != nil {
				return c.Render("error_page", fiber.Map{
					"ErrorMessage": err.Error(),
					"ErrorCode":    fiber.StatusInternalServerError,
				})
			}

			totalTask = len(midtidData.TaskIds.ToIntSlice())
			totalTicket = len(midtidData.TicketIds.ToIntSlice())
			taskIds = midtidData.TaskIds.ToIntSlice()
			ticketIds = midtidData.TicketIds.ToIntSlice()
		}

		// Get Task Details for MIDTID
		dtTaskData, err := getMIDTIDTaskDetailData(config, midtidData.Name, taskIds)
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		// Get Ticket Details for MIDTID
		dtTicketData, err := getMIDTIDTicketDetailData(config, midtidData.Name, ticketIds)
		if err != nil {
			return c.Render("error_page", fiber.Map{
				"ErrorMessage": err.Error(),
				"ErrorCode":    fiber.StatusInternalServerError,
			})
		}

		return c.Render("midtid_detail", fiber.Map{
			"Title": fmt.Sprintf("MIDTID: %v, Merchant: %v Details @%v",
				midtidData.Name,
				midtidData.Merchant.String,
				time.Now().Format("15:04:05 02, January 2006"),
			),
			"Technician":      technicianName,
			"SnEdc":           snEdc,
			"TipeEdc":         tipeEdc,
			"SimCard":         simCard,
			"SimCardProvider": simCardProvider,
			"TotalTask":       totalTask,
			"TotalTicket":     totalTicket,
			"Data":            midtidData,
			"DtTask":          dtTaskData,
			"DtTicket":        dtTicketData,
		})
	}
}

func getMIDTIDTaskDetailData(config *config.YamlConfig, midtid string, ids []int) ([]DtTask, error) {
	if len(ids) == 0 {
		return nil, errors.New("empty task ids")
	}

	odooModel := "project.task"
	odooDomain := []interface{}{
		[]interface{}{"id", "=", ids},
		[]interface{}{"x_no_task", "!=", false},
	}
	odooFields := []string{
		"id",
		"x_no_task",
		"helpdesk_ticket_id",
		"stage_id",
		"technician_id",
		"write_uid",
		"x_task_type",
		"x_pic_merchant",
		"x_pic_phone",
		"x_longitude",
		"x_latitude",
		"x_received_datetime_spk",
		"timesheet_timer_last_stop",
		"x_message_call",
		"x_reason_code_id",
		"x_wo_remark",
		"x_contact_person",
		"partner_phone",
		"partner_mobile",
		"x_cimb_master_mid",
		"x_cimb_master_tid",
	}
	odooOrder := "id asc"
	odooParams := map[string]interface{}{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	odooRequest := map[string]interface{}{
		"jsonrpc": config.ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	odooPayload, err := json.Marshal(odooRequest)
	if err != nil {
		return nil, err
	}

	result, err := getDatafromODOO(config, string(odooPayload))
	if err != nil {
		return nil, err
	}

	resultArray, ok := result.([]interface{})
	if !ok {
		return nil, errors.New("failed to assert results as []interface{}")
	}

	if len(resultArray) == 0 {
		return nil, fmt.Errorf("empty ODOO data for Detail Task of MIDTID: %v", midtid)
	}

	var taskData []DtTask
	for i, record := range resultArray {
		var odooData MidtidTaskItem
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("[%v] invalid record format in midtid detail task resultArray", i)
		}

		jsonData, err := json.Marshal(recordMap)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			return nil, err
		}

		var spkNumber string
		_, ticketNumber, err := parseJSONIDDataCombined(odooData.HelpdeskTicketId)
		if err != nil {
			return nil, err
		} else {
			re := regexp.MustCompile(`\s*\(.*?\)`)
			spkNumber = re.ReplaceAllString(ticketNumber, "")
		}

		_, technician, err := parseJSONIDDataCombined(odooData.TechnicianId)
		if err != nil {
			return nil, err
		}

		_, stage, err := parseJSONIDDataCombined(odooData.StageId)
		if err != nil {
			return nil, err
		}

		_, lastUpdateBy, err := parseJSONIDDataCombined(odooData.WriteUid)
		if err != nil {
			return nil, err
		}

		_, reasonCode, err := parseJSONIDDataCombined(odooData.ReasonCodeId)
		if err != nil {
			return nil, err
		}

		var receivedDatetimeSpk, timesheetLastStop *time.Time
		if !odooData.ReceivedDatetimeSpk.Time.IsZero() {
			adjustedTime := odooData.ReceivedDatetimeSpk.Time.Add(7 * time.Hour)
			receivedDatetimeSpk = &adjustedTime
		}
		if !odooData.TimesheetLastStop.Time.IsZero() {
			adjustedTime := odooData.TimesheetLastStop.Time.Add(7 * time.Hour)
			timesheetLastStop = &adjustedTime
		}

		linkPhoto := fmt.Sprintf("%v:%d/images?value=%d&tid=%v",
			config.Default.FilestoreMWServer,
			config.Default.FilestoreMWPhotosPort,
			odooData.ID,
			odooData.Tid.String,
		)

		dataForDtTask := DtTask{
			ID:                  odooData.ID,
			WoNumber:            odooData.WoNumber,
			SpkNumber:           spkNumber,
			TaskType:            odooData.TaskType.String,
			ReasonCode:          reasonCode,
			Stage:               stage,
			Technician:          technician,
			PicMerchant:         odooData.PicMerchant.String,
			PicPhone:            odooData.PicPhone.String,
			ContactPerson:       odooData.ContactPerson.String,
			ContactPhone:        odooData.ContactPhone.String,
			ContactMobile:       odooData.ContactMobile.String,
			ReceivedDatetimeSpk: receivedDatetimeSpk,
			TimesheetLastStop:   timesheetLastStop,
			Longitude:           odooData.Longitude.String,
			Latitude:            odooData.Latitude.String,
			WoRemarkTiket:       odooData.WoRemarkTiket.String,
			CallCenterMessage:   odooData.CallCenterMessage.String,
			LastUpdateBy:        lastUpdateBy,
			LinkPhoto:           linkPhoto,
			Tid:                 odooData.Tid.String,
		}

		taskData = append(taskData, dataForDtTask)
	}

	return taskData, nil
}

func getMIDTIDTicketDetailData(config *config.YamlConfig, midtid string, ids []int) ([]DtTicket, error) {
	if len(ids) == 0 {
		return nil, errors.New("empty ticket ids")
	}

	odooModel := "helpdesk.ticket"
	odooDomain := []interface{}{
		[]interface{}{"id", "=", ids},
		[]interface{}{"name", "!=", false},
	}
	odooFields := []string{
		"id",
		"stage_id",
		"name",
		"technician_id",
		"ticket_type_id",
		"x_worksheet_template_id",
		"x_received_datetime_spk",
		"x_sla_deadline",
		"complete_datetime_wo",
		"close_date",
		"write_uid",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_contact_person",
		"x_merchant_phone",
	}
	odooOrder := "id asc"
	odooParams := map[string]interface{}{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	odooRequest := map[string]interface{}{
		"jsonrpc": config.ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	odooPayload, err := json.Marshal(odooRequest)
	if err != nil {
		return nil, err
	}

	result, err := getDatafromODOO(config, string(odooPayload))
	if err != nil {
		return nil, err
	}

	resultArray, ok := result.([]interface{})
	if !ok {
		return nil, errors.New("failed to assert results as []interface{}")
	}

	if len(resultArray) == 0 {
		return nil, fmt.Errorf("empty ODOO data for Detail Ticket of MIDTID: %v", midtid)
	}

	var ticketData []DtTicket
	for i, record := range resultArray {
		var odooData MidtidTicketItem
		recordMap, ok := record.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("[%v] invalid record format in midtid detail ticket resultArray", i)
		}

		jsonData, err := json.Marshal(recordMap)
		if err != nil {
			return nil, err
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			return nil, err
		}

		_, ticketType, err := parseJSONIDDataCombined(odooData.TicketTypeId)
		if err != nil {
			return nil, err
		}

		_, worksheetTemplate, err := parseJSONIDDataCombined(odooData.WorksheetTemplateId)
		if err != nil {
			return nil, err
		}

		_, technician, err := parseJSONIDDataCombined(odooData.TechnicianId)
		if err != nil {
			return nil, err
		}

		_, stage, err := parseJSONIDDataCombined(odooData.StageId)
		if err != nil {
			return nil, err
		}

		_, lastUpdateBy, err := parseJSONIDDataCombined(odooData.WriteUid)
		if err != nil {
			return nil, err
		}

		var receivedDatetimeSpk, slaDeadline, completeDatetimeWo, closeDate *time.Time
		if !odooData.ReceivedDatetimeSpk.Time.IsZero() {
			adjustedTime := odooData.ReceivedDatetimeSpk.Time.Add(7 * time.Hour)
			receivedDatetimeSpk = &adjustedTime
		}
		if !odooData.SlaDeadline.Time.IsZero() {
			adjustedTime := odooData.SlaDeadline.Time.Add(7 * time.Hour)
			slaDeadline = &adjustedTime
		}
		if !odooData.CompleteDatetimeWo.Time.IsZero() {
			adjustedTime := odooData.CompleteDatetimeWo.Time.Add(7 * time.Hour)
			completeDatetimeWo = &adjustedTime
		}
		if !odooData.CloseDate.Time.IsZero() {
			adjustedTime := odooData.CloseDate.Time.Add(7 * time.Hour)
			closeDate = &adjustedTime
		}

		dataforDtTicket := DtTicket{
			ID:                  odooData.ID,
			SpkNumber:           odooData.Name,
			TicketType:          ticketType,
			WorksheetTemplate:   worksheetTemplate,
			Stage:               stage,
			Technician:          technician,
			ReceivedDatetimeSpk: receivedDatetimeSpk,
			SlaDeadline:         slaDeadline,
			CompleteDatetimeWo:  completeDatetimeWo,
			CloseDate:           closeDate,
			LastUpdateBy:        lastUpdateBy,
			PicMerchant:         odooData.PicMerchant.String,
			PicPhone:            odooData.PicPhone.String,
			ContactPerson:       odooData.ContactPerson.String,
			ContactPhone:        odooData.ContactPhone.String,
		}

		ticketData = append(ticketData, dataforDtTicket)
	}

	return ticketData, nil
}

type OdooResPartnerItem struct {
	ID                  int               `json:"id"`
	Name                string            `json:"name"`
	Merchant            nullAbleString    `json:"x_merchant"`
	MerchantCode        nullAbleString    `json:"x_merchant_code"`
	MerchantGroupCode   nullAbleString    `json:"x_merchant_group_code"`
	MerchantGroupName   nullAbleString    `json:"x_merchant_group_name"`
	AlamatPengirimanEDC nullAbleString    `json:"x_alamat_pengiriman_edc"`
	ContactPerson       nullAbleString    `json:"x_contact_person"`
	ContactPhone        nullAbleString    `json:"phone"`
	ContactMobile       nullAbleString    `json:"mobile"`
	AlamatPerusahaan    nullAbleString    `json:"contact_address"`
	PicMerchant         nullAbleString    `json:"x_merchant_pic"`
	PicPhone            nullAbleString    `json:"x_merchant_pic_phone"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	SnEdcId             nullAbleInterface `json:"x_studio_sn_edc"`
	TipeEdcId           nullAbleInterface `json:"x_product"`
	SimCardId           nullAbleInterface `json:"x_simcard"`
	SimCardProviderId   nullAbleInterface `json:"x_simcard_provider"`
	MsisdnSimcard       nullAbleString    `json:"x_msisdn_sim_card"`
	Iccid               nullAbleString    `json:"iccid_simcard"`
	Mid                 nullAbleString    `json:"x_cimb_mid"`
	Tid                 nullAbleString    `json:"x_cimb_tid"`
	Longitude           nullAbleFloat     `json:"partner_latitude"`
	Latitude            nullAbleFloat     `json:"partner_longitude"`
	ServicePoint        nullAbleString    `json:"x_service_point"`
	MerchantLastStatus  nullAbleString    `json:"merchant_last_status"`
	TaskIds             nullAbleInterface `json:"task_ids"`
	TicketIds           nullAbleInterface `json:"x_ticket_ids"`
}

type MidtidTaskItem struct {
	ID                  int               `json:"id"`
	WoNumber            string            `json:"x_no_task"`
	HelpdeskTicketId    nullAbleInterface `json:"helpdesk_ticket_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	WriteUid            nullAbleInterface `json:"write_uid"`
	ReasonCodeId        nullAbleInterface `json:"x_reason_code_id"`
	TaskType            nullAbleString    `json:"x_task_type"`
	PicMerchant         nullAbleString    `json:"x_pic_merchant"`
	PicPhone            nullAbleString    `json:"x_pic_phone"`
	ContactPerson       nullAbleString    `json:"x_contact_person"`
	ContactPhone        nullAbleString    `json:"partner_phone"`
	ContactMobile       nullAbleString    `json:"partner_mobile"`
	Longitude           nullAbleString    `json:"x_longitude"`
	Latitude            nullAbleString    `json:"x_latitude"`
	WoRemarkTiket       nullAbleString    `json:"x_wo_remark"`
	CallCenterMessage   nullAbleString    `json:"x_message_call"`
	Mid                 nullAbleString    `json:"x_cimb_master_mid"`
	Tid                 nullAbleString    `json:"x_cimb_master_tid"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	TimesheetLastStop   nullAbleTime      `json:"timesheet_timer_last_stop"`
}

type MidtidTicketItem struct {
	ID                  int               `json:"id"`
	Name                string            `json:"name"`
	TicketTypeId        nullAbleInterface `json:"ticket_type_id"`
	WorksheetTemplateId nullAbleInterface `json:"x_worksheet_template_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	WriteUid            nullAbleInterface `json:"write_uid"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	SlaDeadline         nullAbleTime      `json:"x_sla_deadline"`
	CompleteDatetimeWo  nullAbleTime      `json:"complete_datetime_wo"`
	CloseDate           nullAbleTime      `json:"close_date"`
	PicMerchant         nullAbleString    `json:"x_merchant_pic"`
	PicPhone            nullAbleString    `json:"x_merchant_pic_phone"`
	ContactPerson       nullAbleString    `json:"x_contact_person"`
	ContactPhone        nullAbleString    `json:"x_merchant_phone"`
}

func (t *MidtidTaskItem) UnmarshalJSON(data []byte) error {
	type Alias MidtidTaskItem // Create an alias to avoid recursion
	aux := &struct {
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
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

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.TimesheetLastStop, err = parseTimeField(aux.TimesheetLastStop); err != nil {
		return fmt.Errorf("TimesheetLastStop: %v", err)
	}

	return nil
}

func (t *MidtidTicketItem) UnmarshalJSON(data []byte) error {
	type Alias MidtidTicketItem // Create an alias to avoid recursion
	aux := &struct {
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		SlaDeadline         interface{} `json:"x_sla_deadline"`
		CompleteDatetimeWo  interface{} `json:"complete_datetime_wo"`
		CloseDate           interface{} `json:"close_date"`
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

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CompleteDatetimeWo, err = parseTimeField(aux.CompleteDatetimeWo); err != nil {
		return fmt.Errorf("CompleteDatetimeWo: %v", err)
	}

	if t.CloseDate, err = parseTimeField(aux.CloseDate); err != nil {
		return fmt.Errorf("CloseDate: %v", err)
	}

	return nil
}

// Task Detail Data
type DtTask struct {
	ID                  int
	WoNumber            string
	SpkNumber           string
	TaskType            string
	Stage               string
	ReasonCode          string
	Technician          string
	PicMerchant         string
	PicPhone            string
	ContactPerson       string
	ContactPhone        string
	ContactMobile       string
	ReceivedDatetimeSpk *time.Time
	TimesheetLastStop   *time.Time
	Longitude           string
	Latitude            string
	WoRemarkTiket       string
	CallCenterMessage   string
	LastUpdateBy        string
	LinkPhoto           string
	Tid                 string
}

// Ticket Detail Data
type DtTicket struct {
	ID                  int
	SpkNumber           string
	TicketType          string
	WorksheetTemplate   string
	Stage               string
	Technician          string
	ReceivedDatetimeSpk *time.Time
	SlaDeadline         *time.Time
	CompleteDatetimeWo  *time.Time
	CloseDate           *time.Time
	PicMerchant         string
	PicPhone            string
	ContactPerson       string
	ContactPhone        string
	LastUpdateBy        string
}

// ************************************************************************************************
// NULLABLE
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

// ************************************************************************************************

func getDatafromODOO(config *config.YamlConfig, req string) (interface{}, error) {
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

		newODOOCookies, err := getODOOCookies(config.ApiODOO.Login, config.ApiODOO.Password, config)
		if err != nil {
			return nil, err
		}

		for _, cookie := range newODOOCookies {
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
