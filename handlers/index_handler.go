package handlers

import (
	"call_center_app/config"

	"github.com/gofiber/fiber/v2"
)

func IndexHandler(config *config.YamlConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{})
	}
}
