package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	db     *gorm.DB
	secret string
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(db *gorm.DB, secret string) *AuthHandler {
	return &AuthHandler{db: db, secret: secret}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type refreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// Login 管理员登录
// POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	var admin model.Admin
	if err := h.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		Error(c, http.StatusUnauthorized, 401, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		Error(c, http.StatusUnauthorized, 401, "invalid credentials")
		return
	}

	accessToken, err := generateToken(admin.ID.String(), admin.Role, "access", 24*time.Hour, h.secret)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate token")
		return
	}

	refreshToken, err := generateToken(admin.ID.String(), admin.Role, "refresh", 7*24*time.Hour, h.secret)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate token")
		return
	}

	Success(c, loginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64((24 * time.Hour).Seconds()),
	})
}

// RefreshToken 刷新令牌
// POST /api/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req refreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	token, err := jwt.ParseWithClaims(req.RefreshToken, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(h.secret), nil
	})

	if err != nil {
		Error(c, http.StatusUnauthorized, 401, "invalid refresh token")
		return
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		Error(c, http.StatusUnauthorized, 401, "invalid refresh token claims")
		return
	}

	if claims.Type != "refresh" {
		Error(c, http.StatusUnauthorized, 401, "not a refresh token")
		return
	}

	accessToken, err := generateToken(claims.Subject, claims.Role, "access", 24*time.Hour, h.secret)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to generate token")
		return
	}

	Success(c, refreshTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64((24 * time.Hour).Seconds()),
	})
}

// generateToken 生成 JWT token
func generateToken(userID, role, tokenType string, expiry time.Duration, secret string) (string, error) {
	claims := CustomClaims{
		Role: role,
		Type: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
