package middleware

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

	"almak-back/internal/database"
	"almak-back/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// AuthMiddleware проверяет JWT из заголовка Authorization: Bearer <token>.
func AuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "отсутствует заголовок Authorization"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "неверный формат Authorization"})
			return
		}

		tokenString := parts[1]
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("неподдерживаемый метод подписи")
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "невалидный токен"})
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if tokenType, _ := claims["type"].(string); tokenType != "" && tokenType != "access" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "невалидный токен"})
				return
			}
			id, valid := TokenUserID(claims["sub"])
			if !valid {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "невалидный пользователь"})
				return
			}
			var user models.User
			err := database.DB.Select("id", "login", "name", "role").First(&user, id).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "пользователь не найден"})
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "не удалось проверить пользователя"})
				return
			}
			if !user.Role.Valid() {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "роль пользователя не поддерживается"})
				return
			}
			c.Set("user_id", user.ID)
			c.Set("login", user.Login)
			c.Set("user", user)
		} else {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "невалидный токен"})
			return
		}

		c.Next()
	}
}

func TokenUserID(value any) (uint, bool) {
	n, ok := value.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 || n != math.Trunc(n) || n > 9007199254740991 {
		return 0, false
	}
	id, err := strconv.ParseUint(strconv.FormatFloat(n, 'f', 0, 64), 10, strconv.IntSize)
	return uint(id), err == nil
}

func RequirePermission(permission models.Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("user")
		user, ok := value.(models.User)
		if !exists || !ok || !user.Can(permission) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "недостаточно прав"})
			return
		}
		c.Next()
	}
}
