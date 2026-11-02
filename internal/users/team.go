package users

import (
	"errors"
	"strings"

	"almak-back/internal/models"
	"gorm.io/gorm"
)

type ManagerAccount struct {
	Login    string `json:"login"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type InitialTeam struct {
	AdminLogin string           `json:"adminLogin"`
	Managers   []ManagerAccount `json:"managers"`
}

// A failure rolls back the entire team setup, including administrator promotion.
func ProvisionTeam(db *gorm.DB, input InitialTeam) ([]models.UserProfile, error) {
	if strings.TrimSpace(input.AdminLogin) == "" || len(input.Managers) != 2 {
		return nil, errors.New("укажите существующего администратора и ровно двух менеджеров")
	}
	seen := map[string]bool{strings.TrimSpace(input.AdminLogin): true}
	for _, manager := range input.Managers {
		login := strings.TrimSpace(manager.Login)
		if login == "" || seen[login] || len(manager.Password) < 12 || len(manager.Password) > 72 {
			return nil, errors.New("проверьте уникальные логины и длину паролей менеджеров")
		}
		seen[login] = true
	}
	profiles := []models.UserProfile{}
	err := db.Transaction(func(tx *gorm.DB) error {
		admin, err := GrantAdmin(tx, input.AdminLogin)
		if err != nil {
			return err
		}
		profiles = append(profiles, admin.Profile())
		for _, manager := range input.Managers {
			user, _, err := Provision(tx, manager.Login, manager.Name, models.RoleManager, []byte(manager.Password))
			if err != nil {
				return err
			}
			profiles = append(profiles, user.Profile())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return profiles, nil
}
