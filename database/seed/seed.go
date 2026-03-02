package seed

import (
	"call_center_app/config"
	"call_center_app/models"
	"log"

	// "strings"
	// "golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func UserSeed(db *gorm.DB, config *config.YamlConfig) {
	var userCount int64
	table := config.Db.TbUser
	db.Table(table).Model(&models.CS{}).Count(&userCount)
	if userCount == 0 {
		defPwd := config.Default.Password
		if defPwd == "" {
			log.Print("default password env is not set!")
		}

		// hashPwd, err := bcrypt.GenerateFromPassword([]byte(defPwd), bcrypt.DefaultCost)
		// if err != nil {
		// 	log.Printf("error hashing password %v : %v", defPwd, err)
		// }

		initialUserCallCenters := []models.CS{
			{
				Username: config.Default.UserName,
				Pass:     config.Default.Password,
				IsLogin:  false,
				Email:    config.Default.Email,
				Phone:    config.Default.Phone,
			},
		}

		for _, user := range initialUserCallCenters {
			if user.Pass == "" {
				log.Print("one of the users has an empty email address")
			}
		}

		if err := db.Table(table).Create(&initialUserCallCenters).Error; err != nil {
			log.Printf("error creating initial users: %v", err)
		}
	}
}
