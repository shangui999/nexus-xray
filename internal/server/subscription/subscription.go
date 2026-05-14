package subscription

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/shangui999/nexus-xray/internal/server/model"
)

// NodeConfig 节点配置信息（用于生成链接）
type NodeConfig struct {
	NodeName         string                 `json:"node_name"`
	Address          string                 `json:"address"`           // 原始地址
	SubscriptionAddr string                 `json:"subscription_addr"` // 自定义订阅地址（优先使用）
	IPv6Addr         string                 `json:"ipv6_addr"`         // IPv6 地址
	Port             int                    `json:"port"`
	Protocol         string                 `json:"protocol"`
	UUID             string                 `json:"uuid"` // 用户的 UUID（作为 VLESS id）
	Settings         map[string]interface{} `json:"settings"`
	StreamSettings   map[string]interface{} `json:"stream_settings"`
}

// Generator 订阅链接生成器
type Generator struct {
	hmacSecret    []byte
	serverDomain  string // 订阅服务器域名（CF Tunnel 域名）
}

// NewGenerator 创建生成器
func NewGenerator(secret, domain string) *Generator {
	return &Generator{
		hmacSecret:   []byte(secret),
		serverDomain: domain,
	}
}

// GenerateToken 为用户生成订阅 token（HMAC-SHA256 签名）
// token = base64url(userID + "." + hmac(userID, secret))
func (g *Generator) GenerateToken(userID string) string {
	mac := hmac.New(sha256.New, g.hmacSecret)
	mac.Write([]byte(userID))
	sig := mac.Sum(nil)

	payload := fmt.Sprintf("%s.%x", userID, sig)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

// ValidateToken 验证并解析订阅 token，返回 userID
func (g *Generator) ValidateToken(token string) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("invalid token encoding: %w", err)
	}

	parts := strings.SplitN(string(data), ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}

	userID := parts[0]
	sig := parts[1]

	// Verify HMAC
	mac := hmac.New(sha256.New, g.hmacSecret)
	mac.Write([]byte(userID))
	expectedSig := fmt.Sprintf("%x", mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid token signature")
	}

	return userID, nil
}

// GenerateSubscription 生成用户的完整订阅内容
// 输入：用户信息 + 该用户可用的所有节点/入站
// 输出：base64 编码的多行 URI
func (g *Generator) GenerateSubscription(user *model.User, nodes []NodeConfig) string {
	var lines []string
	for _, node := range nodes {
		// 确定主地址：优先 SubscriptionAddr，否则 Address
		primaryAddr := node.Address
		if node.SubscriptionAddr != "" {
			primaryAddr = node.SubscriptionAddr
		}

		// 生成主链接（IPv4/域名）
		uri := g.generateURI(node, user, primaryAddr, node.NodeName)
		if uri != "" {
			lines = append(lines, uri)
		}

		// 如果有 IPv6，生成额外的 IPv6 链接
		if node.IPv6Addr != "" {
			// IPv6 地址需要用方括号包裹: [2001:db8::1]
			ipv6URI := g.generateURI(node, user, "["+node.IPv6Addr+"]", node.NodeName+" [IPv6]")
			if ipv6URI != "" {
				lines = append(lines, ipv6URI)
			}
		}
	}

	content := strings.Join(lines, "\n")
	return base64.StdEncoding.EncodeToString([]byte(content))
}

// generateURI 生成单个节点 URI，接受地址和名称参数
func (g *Generator) generateURI(node NodeConfig, user *model.User, address, name string) string {
	switch node.Protocol {
	case "vless":
		return g.generateVLESSURIWithAddr(node, user, address, name)
	case "trojan":
		return g.generateTrojanURIWithAddr(node, user, address, name)
	default:
		return g.generateVLESSURIWithAddr(node, user, address, name)
	}
}

// generateVLESSURIWithAddr 生成单个 VLESS 链接（指定地址和名称）
// 格式: vless://uuid@address:port?type=tcp&security=reality&pbk=xxx&fp=chrome&sni=xxx&sid=xxx&spx=%2F#NodeName
func (g *Generator) generateVLESSURIWithAddr(node NodeConfig, user *model.User, address, name string) string {
	vlessUUID := node.UUID
	if vlessUUID == "" {
		vlessUUID = user.VlessUUID
	}

	host := fmt.Sprintf("%s:%d", address, node.Port)
	u := &url.URL{
		Scheme:   "vless",
		User:     url.User(vlessUUID),
		Host:     host,
		Fragment: name,
	}

	query := url.Values{}

	// Extract settings
	if node.Settings != nil {
		if v, ok := node.Settings["type"]; ok {
			query.Set("type", fmt.Sprintf("%v", v))
		}
	}

	// Extract stream settings
	if node.StreamSettings != nil {
		if v, ok := node.StreamSettings["security"]; ok {
			query.Set("security", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["pbk"]; ok {
			query.Set("pbk", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["fp"]; ok {
			query.Set("fp", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["sni"]; ok {
			query.Set("sni", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["sid"]; ok {
			query.Set("sid", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["spx"]; ok {
			query.Set("spx", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["flow"]; ok {
			query.Set("flow", fmt.Sprintf("%v", v))
		}
	}

	// Default values if not set
	if !query.Has("type") {
		query.Set("type", "tcp")
	}
	if !query.Has("security") {
		query.Set("security", "reality")
	}

	u.RawQuery = query.Encode()
	return u.String()
}

// generateTrojanURIWithAddr 生成 Trojan 链接（指定地址和名称）
func (g *Generator) generateTrojanURIWithAddr(node NodeConfig, user *model.User, address, name string) string {
	trojanPassword := node.UUID
	if trojanPassword == "" {
		trojanPassword = user.VlessUUID
	}

	host := fmt.Sprintf("%s:%d", address, node.Port)
	u := &url.URL{
		Scheme:   "trojan",
		User:     url.User(trojanPassword),
		Host:     host,
		Fragment: name,
	}

	query := url.Values{}

	if node.StreamSettings != nil {
		if v, ok := node.StreamSettings["sni"]; ok {
			query.Set("sni", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["security"]; ok {
			query.Set("security", fmt.Sprintf("%v", v))
		}
		if v, ok := node.StreamSettings["type"]; ok {
			query.Set("type", fmt.Sprintf("%v", v))
		}
	}

	if node.Settings != nil {
		if v, ok := node.Settings["type"]; ok {
			query.Set("type", fmt.Sprintf("%v", v))
		}
	}

	if !query.Has("security") {
		query.Set("security", "tls")
	}

	u.RawQuery = query.Encode()
	return u.String()
}
