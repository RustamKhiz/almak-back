package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"almak-back/internal/config"
	"almak-back/internal/database"
	"almak-back/internal/models"
	"almak-back/internal/users"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	action := flag.String("action", "", "create | grant-admin | provision-team")
	login := flag.String("login", "", "unique login; existing login for grant-admin")
	name := flag.String("name", "", "display name (optional)")
	role := flag.String("role", "manager", "admin | manager")
	flag.Parse()
	if (*action != "create" && *action != "grant-admin" && *action != "provision-team") || (*action != "provision-team" && strings.TrimSpace(*login) == "") {
		fail("укажите -action create|grant-admin и -login")
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		fail("не удалось загрузить конфигурацию БД")
	}
	db, err := database.Open(cfg, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fail("не удалось подключиться к БД")
	}
	db.Logger = logger.Default.LogMode(logger.Silent)
	connection, err := db.DB()
	if err != nil {
		fail("не удалось открыть подключение")
	}
	defer connection.Close()
	// Only the users schema is touched. This command never runs order migrations.
	if err = db.AutoMigrate(&models.User{}); err != nil {
		fail("не удалось подготовить таблицу пользователей")
	}
	if *action == "provision-team" {
		var input users.InitialTeam
		decoder := json.NewDecoder(io.LimitReader(os.Stdin, 8193))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&input); err != nil {
			fail("некорректные данные команды пользователей")
		}
		var extra any
		if err = decoder.Decode(&extra); err != io.EOF {
			fail("ожидался один JSON-документ")
		}
		profiles, err := users.ProvisionTeam(db, input)
		if err != nil {
			fail(err.Error())
		}
		_ = json.NewEncoder(os.Stdout).Encode(profiles)
		return
	}
	if *action == "grant-admin" {
		user, err := users.GrantAdmin(db, *login)
		if err != nil {
			fail(err.Error())
		}
		_ = json.NewEncoder(os.Stdout).Encode(user.Profile())
		return
	}
	// Passwords never appear in process arguments or command output.
	password, err := io.ReadAll(io.LimitReader(os.Stdin, 74))
	if err != nil {
		fail("не удалось прочитать пароль из stdin")
	}
	password = []byte(strings.TrimSuffix(strings.TrimSuffix(string(password), "\n"), "\r"))
	user, created, err := users.Provision(db, *login, *name, models.UserRole(*role), password)
	clear(password)
	if err != nil {
		fail(err.Error())
	}
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		User    models.UserProfile `json:"user"`
		Created bool               `json:"created"`
	}{user.Profile(), created})
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
