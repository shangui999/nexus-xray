package api

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/shangui999/nexus-xray/internal/common/config"
	"go.uber.org/zap"
)

// UpgradeHandler 处理 Agent 升级相关的 API 请求
type UpgradeHandler struct {
	cfg   *config.ServerConfig
	logger *zap.Logger
}

// NewUpgradeHandler 创建升级处理器
func NewUpgradeHandler(cfg *config.ServerConfig, logger *zap.Logger) *UpgradeHandler {
	return &UpgradeHandler{
		cfg:   cfg,
		logger: logger,
	}
}

// VersionResponse 版本信息响应
type VersionResponse struct {
	Version       string `json:"version"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	DownloadURL   string `json:"download_url"`
	ReleaseNotes  string `json:"release_notes"`
}

// GetVersion 返回最新 Agent 版本信息
// GET /api/agent/version
func (h *UpgradeHandler) GetVersion(c *gin.Context) {
	releaseDir := h.cfg.AgentRelease.Dir
	currentVersion := h.cfg.AgentRelease.CurrentVersion

	if currentVersion == "" {
		Error(c, http.StatusNotFound, 404, "no agent version available")
		return
	}

	// 尝试找到匹配的二进制文件来计算 checksum
	// 查找目录中的 agent 二进制
	checksum := ""
	releaseNotes := ""

	// 尝试读取 SHA256 校验文件
	checksumFile := filepath.Join(releaseDir, fmt.Sprintf("agent-%s.sha256", currentVersion))
	if data, err := os.ReadFile(checksumFile); err == nil {
		checksum = string(data)
	}

	// 尝试读取发布说明文件
	notesFile := filepath.Join(releaseDir, fmt.Sprintf("agent-%s-notes.txt", currentVersion))
	if data, err := os.ReadFile(notesFile); err == nil {
		releaseNotes = string(data)
	}

	// 如果没有 checksum 文件，尝试从二进制文件计算
	if checksum == "" {
		// 尝试找到任意平台的二进制来计算 checksum
		pattern := filepath.Join(releaseDir, fmt.Sprintf("agent-%s-*", currentVersion))
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			f, err := os.Open(matches[0])
			if err == nil {
				h := sha256.New()
				if _, err := io.Copy(h, f); err == nil {
					checksum = fmt.Sprintf("%x", h.Sum(nil))
				}
				f.Close()
			}
		}
	}

	resp := VersionResponse{
		Version:       currentVersion,
		ChecksumSHA256: checksum,
		DownloadURL:   "/api/agent/download",
		ReleaseNotes:  releaseNotes,
	}

	Success(c, resp)
}

// DownloadBinary 下载 Agent 二进制文件
// GET /api/agent/download?os=linux&arch=amd64
func (h *UpgradeHandler) DownloadBinary(c *gin.Context) {
	osParam := c.Query("os")
	archParam := c.Query("arch")

	if osParam == "" {
		osParam = "linux"
	}
	if archParam == "" {
		archParam = "amd64"
	}

	releaseDir := h.cfg.AgentRelease.Dir
	currentVersion := h.cfg.AgentRelease.CurrentVersion

	if currentVersion == "" {
		Error(c, http.StatusNotFound, 404, "no agent version available")
		return
	}

	// 查找对应的二进制文件: agent-<version>-<os>-<arch>
	filename := fmt.Sprintf("agent-%s-%s-%s", currentVersion, osParam, archParam)
	filePath := filepath.Join(releaseDir, filename)

	// 如果精确匹配不存在，尝试不带平台后缀的文件名
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// 尝试通用文件名
		genericPath := filepath.Join(releaseDir, fmt.Sprintf("agent-%s", currentVersion))
		if _, err := os.Stat(genericPath); os.IsNotExist(err) {
			// 尝试 .zip 文件
			zipPath := filepath.Join(releaseDir, fmt.Sprintf("agent-%s-%s-%s.zip", currentVersion, osParam, archParam))
			if _, err := os.Stat(zipPath); os.IsNotExist(err) {
				Error(c, http.StatusNotFound, 404, fmt.Sprintf("agent binary not found for %s/%s", osParam, archParam))
				return
			}
			filePath = zipPath
		} else {
			filePath = genericPath
		}
	}

	h.logger.Info("agent binary download",
		zap.String("version", currentVersion),
		zap.String("os", osParam),
		zap.String("arch", archParam),
		zap.String("file", filePath),
	)

	c.File(filePath)
}

// NodeTokenAuthMiddleware Node Token 认证中间件
// 通过 Authorization: Bearer <node_token> 或 X-Node-Token header 进行认证
func NodeTokenAuthMiddleware(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if validToken == "" {
			// 如果没有配置 token，暂时允许（开发模式）
			c.Next()
			return
		}

		// 从 Authorization header 或 X-Node-Token header 获取 token
		token := c.GetHeader("X-Node-Token")
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" {
				// 去掉 "Bearer " 前缀
				if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
					token = authHeader[7:]
				}
			}
		}

		if token != validToken {
			Error(c, http.StatusUnauthorized, 401, "invalid node token")
			c.Abort()
			return
		}

		c.Set("node_token", token)
		c.Next()
	}
}