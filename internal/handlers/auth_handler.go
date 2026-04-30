package handlers

import (
	"net/http"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	Config config.Config
}

type loginRequest struct {
	Login    string `json:"login" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func NewAuthHandler(cfg config.Config) *AuthHandler {
	return &AuthHandler{Config: cfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	var user models.User
	if err := database.DB.Where("login = ?", req.Login).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный логин или пароль"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "неверный логин или пароль"})
		return
	}

	tokenString, refreshTokenString, err := h.issueTokenPair(user.ID, user.Login)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось выпустить токен"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "refreshToken": refreshTokenString})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(h.Config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невалидный refresh token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невалидный refresh token"})
		return
	}

	userIDFloat, ok := claims["sub"].(float64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невалидный refresh token"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDFloat)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь не найден"})
		return
	}

	tokenString, refreshTokenString, err := h.issueTokenPair(user.ID, user.Login)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось выпустить токен"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "refreshToken": refreshTokenString})
}

func (h *AuthHandler) issueTokenPair(userID uint, login string) (string, string, error) {
	tokenString, err := h.issueToken(jwt.MapClaims{
		"sub":   userID,
		"login": login,
		"type":  "access",
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	})
	if err != nil {
		return "", "", err
	}

	refreshTokenString, err := h.issueToken(jwt.MapClaims{
		"sub":   userID,
		"login": login,
		"type":  "refresh",
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	})
	if err != nil {
		return "", "", err
	}

	return tokenString, refreshTokenString, nil
}

func (h *AuthHandler) issueToken(claims jwt.MapClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.Config.JWTSecret))
}
