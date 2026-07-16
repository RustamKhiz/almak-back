package handlers

import (
	"net/http"
	"strings"
	"time"

	"almak-back/internal/config"
	"almak-back/internal/database"
	"almak-back/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	refreshCookieName   = "almak_refresh_token"
	refreshCookieMaxAge = 30 * 24 * 60 * 60
)

type AuthHandler struct {
	Config config.Config
}

type loginRequest struct {
	Login     string `json:"login" binding:"required"`
	Password  string `json:"password" binding:"required"`
	UseCookie bool   `json:"useCookie"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func NewAuthHandler(cfg config.Config) *AuthHandler {
	return &AuthHandler{Config: cfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	h.login(c, false)
}

func (h *AuthHandler) DesktopLogin(c *gin.Context) {
	h.login(c, true)
}

func (h *AuthHandler) login(c *gin.Context, desktopEndpoint bool) {
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

	if desktopEndpoint || !req.UseCookie {
		c.JSON(http.StatusOK, gin.H{"token": tokenString, "refreshToken": refreshTokenString})
		return
	}

	setRefreshCookie(c, refreshTokenString)
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		h.refresh(c, req.RefreshToken, true)
		return
	}

	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil || refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh token отсутствует"})
		return
	}

	h.refresh(c, refreshToken, false)
}

func (h *AuthHandler) DesktopRefresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректное тело запроса"})
		return
	}

	h.refresh(c, req.RefreshToken, true)
}

func (h *AuthHandler) refresh(c *gin.Context, refreshToken string, desktop bool) {
	token, err := jwt.Parse(refreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(h.Config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		rejectInvalidRefresh(c, desktop, "невалидный refresh token")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || claims["type"] != "refresh" {
		rejectInvalidRefresh(c, desktop, "невалидный refresh token")
		return
	}

	userIDFloat, ok := claims["sub"].(float64)
	if !ok {
		rejectInvalidRefresh(c, desktop, "невалидный refresh token")
		return
	}

	var user models.User
	if err := database.DB.First(&user, uint(userIDFloat)).Error; err != nil {
		rejectInvalidRefresh(c, desktop, "пользователь не найден")
		return
	}

	tokenString, refreshTokenString, err := h.issueTokenPair(user.ID, user.Login)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "не удалось выпустить токен"})
		return
	}

	if desktop {
		c.JSON(http.StatusOK, gin.H{"token": tokenString, "refreshToken": refreshTokenString})
		return
	}

	setRefreshCookie(c, refreshTokenString)
	c.JSON(http.StatusOK, gin.H{"token": tokenString})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	clearRefreshCookie(c)
	c.Status(http.StatusNoContent)
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

func setRefreshCookie(c *gin.Context, refreshToken string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, refreshToken, refreshCookieMaxAge, "/", "", isSecureRequest(c), true)
}

func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(refreshCookieName, "", -1, "/", "", isSecureRequest(c), true)
}

func rejectInvalidRefresh(c *gin.Context, desktop bool, message string) {
	if !desktop {
		clearRefreshCookie(c)
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": message})
}

func isSecureRequest(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}

	forwardedProto := strings.TrimSpace(strings.Split(c.GetHeader("X-Forwarded-Proto"), ",")[0])
	if strings.EqualFold(forwardedProto, "https") {
		return true
	}

	host := strings.ToLower(strings.Split(c.Request.Host, ":")[0])
	return host != "localhost" && host != "127.0.0.1" && host != "[::1]"
}
