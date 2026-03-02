package routes

import (
	"call_center_app/config"
	"call_center_app/handlers"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ServerRoutes(app *fiber.App, config *config.YamlConfig, db *gorm.DB) {
	// Common routes
	app.Get("/", handlers.IndexHandler(config))
	app.Post("/login", handlers.LoginHandler(db, config))

	// ===============================================================================
	// OLD Not use anymore
	// app.Get("/info", handlers.FormSubmitCallTask(db, config))
	// app.Post("/info", handlers.SubmitCallTask(db, config))
	// app.Post("/request-wa", handlers.MarkAsCalled(db, config))
	// app.Get("/urgent", handlers.RequestDapur(config))
	// app.Post("/urgent", handlers.UrgentRequest(db, config))
	// app.Get("/urgentLog", handlers.DetailReqDapur(db, config))
	// ===============================================================================
	// Call Center Dashboard
	app.Get("/request-wa", handlers.FormCallWhatsappRequest(db, config))
	app.Post("/submit-request-wa", handlers.SubmitFormCallWhatsappRequest(db, config))
	app.Post("/submit-edited-jo", handlers.SubmitFormEditedJO(db, config))
	app.Get("/merchant-detail", handlers.ShowMerchantDetailData(db, config))
	app.Get("/get-wa-mentions", handlers.GetWhatsappMention(db, config))
	// Logs
	app.Get("/logs", handlers.RenderLogs())
	app.Get("/get-logs", handlers.GetLogs())
	app.Post("/log-content", handlers.LogContent())
	// Re-Open Link
	app.Post("/re-open-data", handlers.FetchReopenData(db))
	app.Get("/re-open-link", handlers.RenderReopenLink())
	app.Post("/re-open-link", handlers.ReopenLink(db))
	// Link Photo
	app.Get("/proxy-photo", handlers.ProxyPhotoHandler())
	app.Get("/file/:imageID", handlers.GetPhotoInFS())

	api := app.Group("/api")
	api.Get("/getUserList", handlers.GetUserList(db, config))

	app.Get("/myConnection", func(c *fiber.Ctx) error {
		localIP, err := handlers.GetWiFi_IPv4()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to retrieve IP address: %v", err),
			})
		}

		ssid, err := handlers.GetWiFi_SSID()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to get ssid connected: %v", err),
			})
		}
		return c.JSON(fiber.Map{
			"wifi": ssid,
			"ipv4": localIP,
		})
	})
}
