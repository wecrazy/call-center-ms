package models

import (
	"call_center_app/config"
	"time"

	"gorm.io/gorm"
)

type CS struct {
	gorm.Model
	ID        int       `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	Username  string    `gorm:"type:varchar(100);column:username;not null" json:"username"`
	Email     string    `gorm:"type:text;column:email;not null" json:"email"`
	Phone     string    `gorm:"type:varchar(50);column:phone;not null" json:"phone"`
	Pass      string    `gorm:"type:varchar(100);column:pass;not null" json:"pass"`
	IP        string    `gorm:"type:varchar(50);column:ip;default:NULL" json:"ip"`
	LastLogin time.Time `gorm:"type:datetime;column:last_login;default:NULL" json:"last_login"`
	IsLogin   bool      `gorm:"type:bool;column:is_login;not null" json:"is_login"`
}

func (CS) TableName() string {
	return config.GetConfig().Db.TbUser
}
