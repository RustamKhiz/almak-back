package users

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"almak-back/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Provision is idempotent: existing accounts never have passwords reset.
func Provision(db *gorm.DB, login, name string, role models.UserRole, password []byte) (models.User, bool, error) {
	login = strings.TrimSpace(login)
	name = strings.TrimSpace(name)
	if login == "" || len(login) > 100 || strings.IndexFunc(login, unicode.IsSpace) >= 0 || len(name) > 200 || !role.Valid() {
		return models.User{}, false, errors.New("некорректные логин, имя или роль")
	}
	var existing models.User
	if err := db.Where("login = ?", login).First(&existing).Error; err == nil {
		if existing.Role != role || existing.Name != name {
			return existing, false, errors.New("аккаунт уже существует с другим именем или ролью; запись не изменена")
		}
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.User{}, false, errors.New("не удалось проверить существующий аккаунт")
	}
	if len(password) < 12 || len(password) > 72 {
		return models.User{}, false, errors.New("пароль должен содержать от 12 до 72 байт")
	}
	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, false, errors.New("не удалось подготовить пароль")
	}
	user := models.User{Login: login, Name: name, Role: role, Password: string(hash)}
	result := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "login"}}, DoNothing: true}).Create(&user)
	if result.Error != nil {
		return models.User{}, false, errors.New("не удалось создать аккаунт")
	}
	if result.RowsAffected == 0 {
		return Provision(db, login, name, role, password)
	}
	return user, true, nil
}

func GrantAdmin(db *gorm.DB, login string) (models.User, error) {
	var user models.User
	if err := db.Where("login = ?", strings.TrimSpace(login)).First(&user).Error; err != nil {
		return user, fmt.Errorf("существующий аккаунт не найден или БД недоступна")
	}
	if err := db.Model(&user).Update("role", models.RoleAdmin).Error; err != nil {
		return user, errors.New("не удалось назначить роль")
	}
	user.Role = models.RoleAdmin
	return user, nil
}
