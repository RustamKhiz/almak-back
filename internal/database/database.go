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

	if db.Migrator().HasTable("doors") && !db.Migrator().HasTable("interior_doors") {
		if err = db.Migrator().RenameTable("doors", "interior_doors"); err != nil {
			return err
		}
	}

	if err = db.AutoMigrate(&models.User{}, &models.Order{}, &models.InteriorDoor{}, &models.EntranceDoor{}, &models.Molding{}); err != nil {
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

	if db.Migrator().HasColumn(&models.InteriorDoor{}, "color") {
		if err = db.Migrator().DropColumn(&models.InteriorDoor{}, "color"); err != nil {
			return err
		}
	}

	if err = ensureDefaultUser(db); err != nil {
		return err
	}

	DB = db
	return nil
}

// ensureDefaultUser удаляет legacy-пользователя admin и гарантирует пользователя almak/almak05.
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

	defaultUser := models.User{
		Login:    defaultLogin,
		Password: string(hash),
	}

	return db.Create(&defaultUser).Error
}
