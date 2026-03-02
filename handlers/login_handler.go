package handlers

import (
	"call_center_app/config"
	"call_center_app/models"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func LoginHandler(db *gorm.DB, config *config.YamlConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		table := config.Db.TbUser

		type LoginReq struct {
			Username string `json:"username"`
			Password string `json:"password"`
			SSID     string `json:"ssid"`
			IP       string `json:"ip"`
		}

		// var loginRequest LoginReq
		// if err := c.BodyParser(&loginRequest); err != nil {
		// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 		"status":  "GAGAL",
		// 		"message": "invalid input data",
		// 	})
		// }

		body := c.Body()

		var loginRequest LoginReq
		if err := json.Unmarshal(body, &loginRequest); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": "invalid JSON format",
			})
		}

		if loginRequest.Password == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": "password cannot be empty!",
			})
		}

		// if loginRequest.SSID == "" || loginRequest.IP == "" {
		if loginRequest.IP == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"status": "GAGAL",
				// "message": "missing data of SSID or IP!",
				"message": "missing data of IP!",
			})
		}

		var user models.CS
		if err := db.Table(table).Where("username = ?", loginRequest.Username).First(&user).Error; err != nil {
			if gorm.ErrRecordNotFound == err {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"status":  "GAGAL",
					"message": fmt.Sprintf("username: %v not found!", loginRequest.Username),
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": fmt.Sprintf("database error: %v", err),
			})
		}

		if user.Pass != loginRequest.Password {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": fmt.Sprintf("incorrect password for username: %v", loginRequest.Username),
			})
		}

		if err := db.Table(table).Where("username = ?", loginRequest.Username).First(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"status":  "GAGAL",
					"message": "username not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": fmt.Sprintf("error while finding user: %v", err),
			})
		}

		user.IsLogin = true
		user.IP = loginRequest.IP
		user.LastLogin = time.Now()

		if err := db.Table(table).Where("id = ?", user.ID).Updates(map[string]interface{}{
			"is_login":   user.IsLogin,
			"ip":         user.IP,
			"last_login": user.LastLogin,
		}).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "GAGAL",
				"message": fmt.Sprintf("error while updating user: %v", err),
			})
		}

		log.Printf("[%v] Successfully login!", user.Username)

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "OK",
			"message": fmt.Sprintf("%v successfully login.", loginRequest.Username),
		})
	}
}
