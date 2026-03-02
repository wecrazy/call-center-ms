package database

import (
	"call_center_app/config"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func InitDB(config *config.YamlConfig) (*gorm.DB, error) {
	dbHost := config.Db.Host
	dbPort := config.Db.Port
	dbUser := config.Db.User
	dbPwd := config.Db.Password
	dbName := config.Db.Name

	if dbUser == "" || dbName == "" || dbHost == "" || dbPort == 0 {
		errMsg := "database environment variables are not fully set"
		log.Fatal(errMsg)
		return nil, errors.New(errMsg)
	}

	var db *gorm.DB
	var err error

	maxRetries := config.Db.MaxRetry
	retryIntervalStr := config.Db.RetryDelay
	retryInterval, parseErr := time.ParseDuration(retryIntervalStr)
	if parseErr != nil {
		log.Fatalf("failed to parse retry interval: %v", parseErr)
		return nil, parseErr
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%v)/?charset=utf8mb4&parseTime=True&loc=Local",
			dbUser, dbPwd, dbHost, dbPort)

		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err == nil {
			// Successful connection
			break
		}

		log.Printf("Failed to connect to MySQL (attempt %d/%d): %v", attempt, maxRetries, err)

		// Wait before the next retry
		if attempt < maxRetries {
			log.Printf("Retrying in %s...", retryIntervalStr)
			time.Sleep(retryInterval)
		}
	}

	if err != nil {
		errMsg := fmt.Sprintf("failed to connect to MySQL after %d attempts: %v", maxRetries, err)
		log.Fatal(errMsg)
		return nil, errors.New(errMsg)
	}

	createDBQuery := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", dbName)
	if err := db.Exec(createDBQuery).Error; err != nil {
		errMsg := fmt.Sprintf("failed to create database: %v", err)
		log.Fatal(errMsg)
		return nil, errors.New(errMsg)
	}

	dsnWithDB := fmt.Sprintf("%s:%s@tcp(%s:%v)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPwd, dbHost, dbPort, dbName)

	db, err = gorm.Open(mysql.Open(dsnWithDB), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "",    // No prefix
			SingularTable: false, // Use plural table names
		},
	})

	if err != nil {
		errMsg := fmt.Sprintf("failed to connect to database after selecting DB: %v", err)
		log.Fatal(errMsg)
		return nil, errors.New(errMsg)
	}

	tables := map[string]interface{}{
		// // UNCOMMENT this soon if needed!!
		// config.Db.TbUser:            &models.CS{},
		// config.Db.TbOdooRC:          &models.OdooReasonCode{},
		// config.Db.TbWaReq:           &models.WaRequest{},
		// config.Db.TbWaMention:       &models.WaMention{},
		// config.Db.TbUnsolvedTicket:  &models.UnsolvedTicketData{},
		// config.Db.TbBasedOnWoNumber: &models.BasedOnWoNumber{},
		// config.Db.TbCannotFollowUp: &models.CannotFollowUp{},

		// // config.Db.TbDataMerchantHmin1:    &models.JOMerchantHmin1{},
		// // config.Db.TbMerchantHmin1CallLog: &models.JOMerchantHmin1CallLog{},
		// // config.Db.TbRole: &models.Role{},
		//////////////////////////
		//     add more table   //
		//////////////////////////
	}

	for tableName, model := range tables {
		if tableName == "" {
			errMsg := "environment variable for table name is not set"
			log.Fatal(errMsg)
			return nil, errors.New(errMsg)
		}
		if err := db.Table(tableName).AutoMigrate(model); err != nil {
			errMsg := fmt.Sprintf("error migrating model for table %s: %v", tableName, err)
			log.Fatal(errMsg)
			return nil, errors.New(errMsg)
		}
	}

	return db, nil
}
