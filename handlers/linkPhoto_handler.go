package handlers

import (
	"call_center_app/config"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/proxy"
)

func ProxyPhotoHandler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		value := c.Query("value")
		tid := c.Query("tid")

		// If either 'value' or 'tid' is missing, return a 400 Bad Request response
		if value == "" || tid == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Missing 'value' or 'tid' parameters")
		}

		linkPhoto := fmt.Sprintf("%s:%d/images?value=%s&tid=%s",
			config.GetConfig().Default.FilestoreMWServer,
			config.GetConfig().Default.FilestoreMWPhotosPort,
			value,
			tid,
		)

		return proxy.Forward(linkPhoto)(c)
	}
}

func GetPhotoInFS() fiber.Handler {
	return func(c *fiber.Ctx) error {
		imageID := c.Params("imageID")

		photoInFS := fmt.Sprintf("%s:%d/file/%s",
			config.GetConfig().Default.FilestoreMWServer,
			config.GetConfig().Default.FilestoreMWPhotosPort,
			imageID,
		)

		return proxy.Forward(photoInFS)(c)
	}
}
