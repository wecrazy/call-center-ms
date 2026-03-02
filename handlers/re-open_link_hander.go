package handlers

import (
	"call_center_app/config"
	"call_center_app/models"
	"call_center_app/utils"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func RenderReopenLink() fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Render("reopen_link", fiber.Map{})
	}
}

func ReopenLink(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {

		yamlCfg := config.GetConfig()

		type Req struct {
			ID string `json:"id"`
		}

		var payload Req
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid payload, error: " + err.Error(),
			})
		}

		if payload.ID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "ID is required",
			})
		}

		uintID, err := strconv.Atoi(payload.ID)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid ID format, error: " + err.Error(),
			})
		}

		var dbData models.WaRequest
		if err := db.First(&dbData, uintID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Data not found, error: " + err.Error(),
			})
		}

		sanitizedPhoneNumber, err := utils.SanitizePhoneNumber(dbData.PicPhone)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid phone number format, error: " + err.Error(),
			})
		}

		if err := db.Model(&models.WaRequest{}).
			Where("id = ?", uintID).
			Updates(map[string]interface{}{
				"x_pic_phone": sanitizedPhoneNumber,
				"is_done":     false,
			}).
			Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed generate new link, error: " + err.Error(),
			})
		}

		webURL := fmt.Sprintf("%s_%s_%s_%d",
			"whatsapp",
			sanitizedPhoneNumber,
			utils.GenerateRandomString(100),
			dbData.TempCS,
		)
		link := fmt.Sprintf("http://%s:%s/request-wa?data=%s", yamlCfg.Default.HostServer, yamlCfg.App.Port, webURL)

		return c.JSON(fiber.Map{
			"link": link,
		})
	}
}

func FetchReopenData(db *gorm.DB) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type DataTableReq struct {
			Draw           string `form:"draw"`
			Start          int    `form:"start"`
			Length         int    `form:"length"`
			SearchPhone    string `form:"phone"`
			SearchMerchant string `form:"merchant"`
			Order          []struct {
				Column int    `form:"column"`
				Dir    string `form:"dir"`
			} `form:"order"`
		}

		req := new(DataTableReq)
		if err := c.BodyParser(req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request", "details": err.Error()})
		}

		var total int64
		var filtered int64
		var results []models.WaRequest

		query := db.Model(&models.WaRequest{})
		query = query.Where("is_on_calling = ?", true)
		query = query.Where("is_done = ?", true)
		query = query.Where("temp_cs != ?", 0)
		if err := query.Count(&total).Error; err != nil {
			fmt.Println("Error in count query:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count total records"})
		}

		// Apply filters if provided
		if req.SearchPhone != "" && req.SearchMerchant != "" {
			query = query.Where(
				db.Where("x_pic_phone LIKE ?", "%"+req.SearchPhone+"%").
					Or("x_merchant LIKE ?", "%"+req.SearchMerchant+"%"),
			)
		} else if req.SearchPhone != "" {
			query = query.Where("x_pic_phone LIKE ?", "%"+req.SearchPhone+"%")
		} else if req.SearchMerchant != "" {
			query = query.Where("x_merchant LIKE ?", "%"+req.SearchMerchant+"%")
		}

		// Get filtered records count
		if err := query.Count(&filtered).Error; err != nil {
			fmt.Println("Error in filtered count query:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to count filtered records"})
		}

		// Apply ordering if provided
		if len(req.Order) > 0 {
			// Determine the sorting column and direction
			orderColumn := req.Order[0].Column
			orderDirection := req.Order[0].Dir

			var orderableColumns = map[int]string{
				0: "id",
				1: "x_pic_phone",
				2: "x_merchant",
				3: "x_no_task",
				4: "helpdesk_ticket_name",
				5: "temp_cs",
				6: "updated_at",
				7: "call_center_message",
			}
			if col, ok := orderableColumns[orderColumn]; ok {
				if strings.ToLower(orderDirection) == "desc" {
					query = query.Order(col + " DESC")
				} else {
					query = query.Order(col + " ASC")
				}
			}

		}

		// Fetch filtered and paginated results
		if err := query.Offset(req.Start).Limit(req.Length).Find(&results).Error; err != nil {
			fmt.Println("Error fetching data:", err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch data"})
		}

		// Prepare data for response
		data := [][]string{}
		for _, r := range results {
			var adminCS string = "N/A"
			var tempCSData models.CS
			if err := db.Model(&models.CS{}).Where("id = ?", r.TempCS).First(&tempCSData).Error; err == nil {
				adminCS = tempCSData.Username
			} else if err != gorm.ErrRecordNotFound {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": fmt.Sprintf("Failed to fetch CS data: %v", err)})
			}

			var lastUpdate string = "N/A"
			if !r.UpdatedAt.IsZero() {
				lastUpdate = r.UpdatedAt.Format("2006-01-02 15:04:05")
			}

			data = append(data, []string{
				strconv.Itoa(int(r.ID)),
				r.PicPhone,
				r.MerchantName,
				r.WoNumber,
				r.HelpdeskTicketName,
				adminCS,
				lastUpdate,
				r.CallCenterMessage,
			})
		}

		// Return the data in the DataTable format
		return c.JSON(fiber.Map{
			"draw":            req.Draw,
			"recordsTotal":    total,
			"recordsFiltered": filtered,
			"data":            data,
		})
	}
}
