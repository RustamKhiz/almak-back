package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port      string
	DBHost    string
	DBPort    string
	DBUser    string
	DBPass    string
	DBName    string
	JWTSecret string
	FrontendOrigins []string
}

func LoadConfig() (Config, error) {
	wd, _ := os.Getwd()

	// Пытаемся явно загрузить .env из текущей директории
	if err := godotenv.Overload(wd + string(os.PathSeparator) + ".env"); err != nil {
		return Config{}, fmt.Errorf("не удалось загрузить .env из %s: %w", wd, err)
	}

	cfg := Config{
		Port:      getEnv("PORT", "8080"),
		DBHost:    os.Getenv("DB_HOST"),
		DBPort:    os.Getenv("DB_PORT"),
		DBUser:    os.Getenv("DB_USER"),
		DBPass:    os.Getenv("DB_PASSWORD"),
		DBName:    os.Getenv("DB_NAME"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		FrontendOrigins: getFrontendOrigins(),
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getFrontendOrigins() []string {
	raw := os.Getenv("FRONTEND_ORIGINS")
	if strings.TrimSpace(raw) == "" {
		return []string{
			"http://localhost:4200",
			"http://109.196.100.71",
		}
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	if len(origins) == 0 {
		return []string{"http://localhost:4200"}
	}

	return origins
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func validate(cfg Config) error {
	if cfg.DBHost == "" || cfg.DBPort == "" || cfg.DBUser == "" || cfg.DBName == "" || cfg.JWTSecret == "" {
		return fmt.Errorf("не заданы обязательные переменные окружения: DB_HOST, DB_PORT, DB_USER, DB_NAME, JWT_SECRET")
	}
	return nil
}
