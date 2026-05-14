package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"github.com/shangui999/nexus-xray/internal/server/subscription"
	"gorm.io/gorm"
)

// SubscriptionHandler 订阅处理器
type SubscriptionHandler struct {
	db        *gorm.DB
	secret    string
	generator *subscription.Generator
}

// NewSubscriptionHandler 创建订阅处理器
func NewSubscriptionHandler(db *gorm.DB, secret string) *SubscriptionHandler {
	return &SubscriptionHandler{db: db, secret: secret, generator: subscription.NewGenerator(secret, "")}
}

// GetSubscription 获取订阅配置
// GET /api/subscription/:token
// token 是 HMAC-SHA256 签名的用户标识，格式: base64(userID.hmacSignature)
func (h *SubscriptionHandler) GetSubscription(c *gin.Context) {
	token := c.Param("token")

	// 解码 token
	decoded, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		// 尝试 RawURLEncoding（无 padding）
		decoded, err = base64.RawURLEncoding.DecodeString(token)
		if err != nil {
			Error(c, http.StatusBadRequest, 400, "invalid subscription token")
			return
		}
	}

	parts := strings.SplitN(string(decoded), ".", 2)
	if len(parts) != 2 {
		Error(c, http.StatusBadRequest, 400, "invalid subscription token format")
		return
	}

	userIDStr := parts[0]
	signature := parts[1]

	// 验证 HMAC 签名
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(userIDStr))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		Error(c, http.StatusUnauthorized, 401, "invalid subscription token signature")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user ID in token")
		return
	}

	// 查找用户
	var user model.User
	if err := h.db.Preload("Plan").First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "user not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get user")
		return
	}

	// 检查用户状态：过期或超额返回空
	if user.State != "active" {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
		return
	}
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
		return
	}
	if user.QuotaBytes > 0 && user.UsedBytes >= user.QuotaBytes {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
		return
	}

	// 获取所有已启用的入站配置（关联在线节点）
	var inbounds []model.Inbound
	if err := h.db.Preload("Node").Where("enabled = ?", true).Find(&inbounds).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to get inbounds")
		return
	}

	// 构建节点配置列表
	var nodeConfigs []subscription.NodeConfig
	for _, inbound := range inbounds {
		if inbound.Node == nil || inbound.Node.Status != "online" {
			continue
		}

		var settings map[string]interface{}
		if inbound.Settings != nil {
			settings = make(map[string]interface{})
			_ = json.Unmarshal(inbound.Settings, &settings)
		}

		var streamSettings map[string]interface{}
		if inbound.StreamSettings != nil {
			streamSettings = make(map[string]interface{})
			_ = json.Unmarshal(inbound.StreamSettings, &streamSettings)
		}

		nodeConfigs = append(nodeConfigs, subscription.NodeConfig{
			NodeName:         inbound.Node.Name,
			Address:          inbound.Node.Address,
			SubscriptionAddr: inbound.Node.SubscriptionAddr,
			IPv6Addr:         inbound.Node.IPv6Addr,
			Port:             inbound.Port,
			Protocol:         inbound.Protocol,
			UUID:             user.VlessUUID,
			Settings:         settings,
			StreamSettings:   streamSettings,
		})
	}

	if len(nodeConfigs) == 0 {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(""))
		return
	}

	// 构建 subscription-userinfo 头
	var expireUnix int64
	if user.ExpiresAt != nil {
		expireUnix = user.ExpiresAt.Unix()
	}
	userinfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		0, user.UsedBytes, user.QuotaBytes, expireUnix)

	// 检测格式：优先使用 query 参数，否则通过 User-Agent 自动检测
	format := c.Query("format")
	if format == "" {
		ua := strings.ToLower(c.GetHeader("User-Agent"))
		if strings.Contains(ua, "clash") || strings.Contains(ua, "stash") || strings.Contains(ua, "mihomo") {
			format = "clash"
		} else {
			format = "base64"
		}
	}

	switch format {
	case "clash":
		data, err := h.generator.GenerateClashConfig(&user, nodeConfigs)
		if err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to generate clash config")
			return
		}
		c.Header("Content-Type", "text/yaml; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=clash-config.yaml")
		c.Header("subscription-userinfo", userinfo)
		c.String(http.StatusOK, string(data))

	default: // base64
		encoded := h.generator.GenerateSubscription(&user, nodeConfigs)
		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.Header("subscription-userinfo", userinfo)
		c.String(http.StatusOK, encoded)
	}
}

// GenerateSubscriptionToken 生成订阅 token（供外部调用）
func GenerateSubscriptionToken(userID string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(userID))
	signature := hex.EncodeToString(mac.Sum(nil))

	payload := userID + "." + signature
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}
