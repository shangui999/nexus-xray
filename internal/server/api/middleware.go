package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// CORSMiddleware 允许前端跨域请求
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// LoggerMiddleware 请求日志中间件
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		logger.Info("HTTP request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

// CustomClaims 自定义 JWT Claims，包含 role 和 token type
type CustomClaims struct {
	Role string `json:"role"`
	Type string `json:"type,omitempty"` // access / refresh
	jwt.RegisteredClaims
}

// JWTAuthMiddleware JWT 认证中间件，验证 Bearer token 并将 user_id 和 role 存入 Context
func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			Error(c, http.StatusUnauthorized, 401, "authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			Error(c, http.StatusUnauthorized, 401, "invalid authorization header format")
			c.Abort()
			return
		}

		tokenStr := parts[1]
		token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(secret), nil
		})

		if err != nil {
			Error(c, http.StatusUnauthorized, 401, "invalid token")
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*CustomClaims)
		if !ok || !token.Valid {
			Error(c, http.StatusUnauthorized, 401, "invalid token claims")
			c.Abort()
			return
		}

		// 确保 access token，不接受 refresh token
		if claims.Type == "refresh" {
			Error(c, http.StatusUnauthorized, 401, "refresh token cannot be used for authentication")
			c.Abort()
			return
		}

		c.Set("user_id", claims.Subject)
		c.Set("role", claims.Role)
		c.Next()
	}
}
