package database

import (
	"errors"
	"fmt"

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

	if err = db.AutoMigrate(&models.User{}, &models.Order{}, &models.Door{}); err != nil {
		return err
	}

	if err = ensureDefaultAdmin(db); err != nil {
		return err
	}

	DB = db
	return nil
}

// ensureDefaultAdmin создаёт пользователя admin/admin при первом запуске.
func ensureDefaultAdmin(db *gorm.DB) error {
	var user models.User
	err := db.Where("login = ?", "admin").First(&user).Error
	if err == nil {
		return nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.User{
		Login:    "admin",
		Password: string(hash),
	}

	return db.Create(&admin).Error
}
