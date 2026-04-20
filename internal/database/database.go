package database

import (
	"errors"
	"fmt"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func Connect(cfg config.Config) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		cfg.DBHost,
		cfg.DBUser,
		cfg.DBPass,
		cfg.DBName,
		cfg.DBPort,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return err
	}

	if db.Migrator().HasTable("doors") && !db.Migrator().HasTable("interior_doors") {
		if err = db.Migrator().RenameTable("doors", "interior_doors"); err != nil {
			return err
		}
	}

	if err = db.AutoMigrate(&models.User{}, &models.Order{}, &models.OrderPayment{}, &models.InteriorDoor{}, &models.EntranceDoor{}, &models.Molding{}, &models.Extension{}, &models.Capital{}, &models.Hardware{}, &models.Paneling{}); err != nil {
		return err
	}

	if db.Migrator().HasColumn(&models.Order{}, "count") {
		if err = db.Migrator().DropColumn(&models.Order{}, "count"); err != nil {
			return err
		}
	}
	if db.Migrator().HasColumn(&models.InteriorDoor{}, "type") {
		if err = db.Migrator().DropColumn(&models.InteriorDoor{}, "type"); err != nil {
			return err
		}
	}
	if err = ensureOrderPaymentsBackfilled(db); err != nil {
		return err
	}
	if err = ensureDefaultUser(db); err != nil {
		return err
	}
	DB = db
	return nil
}

func ensureDefaultUser(db *gorm.DB) error {
	if err := db.Where("login = ?", "admin").Delete(&models.User{}).Error; err != nil {
		return err
	}
	const defaultLogin = "almak"
	const defaultPassword = "almak05"
	hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var user models.User
	err = db.Where("login = ?", defaultLogin).First(&user).Error
	if err == nil {
		return db.Model(&user).Update("password", string(hash)).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&models.User{Login: defaultLogin, Password: string(hash)}).Error
}

func ensureOrderPaymentsBackfilled(db *gorm.DB) error {
	var orders []models.Order
	if err := db.Find(&orders).Error; err != nil {
		return err
	}

	for _, order := range orders {
		if order.Prepayment <= 0 {
			continue
		}

		var count int64
		if err := db.Model(&models.OrderPayment{}).Where("order_id = ?", order.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		payment := models.OrderPayment{
			OrderID:   order.ID,
			Amount:    order.Prepayment,
			Comment:   "Первоначальный взнос",
			CreatedAt: time.Now().UTC(),
		}
		if err := db.Create(&payment).Error; err != nil {
			return err
		}
	}

	return nil
}
