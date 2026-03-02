package handlers

import (
	"call_center_app/config"
	"call_center_app/models"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func GetUserList(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		table := config.Db.TbUser
		var users []models.CS
		if err := db.Table(table).Where("username NOT IN (?)", []string{"FAIZ"}).Find(&users).Error; err != nil {
			// if err := db.Table(table).Where("username NOT IN (?)", []string{"Wegil", "FAIZ"}).Find(&users).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fmt.Sprintf("unable to fetch users: %v", err),
			})
		}
		return c.JSON(fiber.Map{
			"users": users,
		})
	}
}
