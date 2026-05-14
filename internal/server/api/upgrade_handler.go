package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shangui999/nexus-xray/internal/common/config"
	"go.uber.org/zap"
)

// UpgradeHandler 处理 Agent 升级相关的 API 请求
type UpgradeHandler struct {
	cfg    *config.ServerConfig
	logger *zap.Logger
	cache  versionCache
}

// NewUpgradeHandler 创建升级处理器
func NewUpgradeHandler(cfg *config.ServerConfig, logger *zap.Logger) *UpgradeHandler {
	return &UpgradeHandler{
		cfg:    cfg,
		logger: logger,
		cache: versionCache{
			checksum: make(map[string]string),
		},
	}
}

// versionCache 缓存 GitHub Releases API 的查询结果
type versionCache struct {
	version   string
	checksum  map[string]string // arch -> sha256
	fetchedAt time.Time
	mu        sync.RWMutex
}

const cacheTTL = 5 * time.Minute

// VersionResponse 版本信息响应
type VersionResponse struct {
	Version        string `json:"version"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	DownloadURL    string `json:"download_url"`
	ReleaseNotes   string `json:"release_notes"`
}

// githubRelease GitHub Releases API 响应结构
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
		Size               int    `json:"size"`
	} `json:"assets"`
}

// GetVersion 返回最新 Agent 版本信息
// GET /api/agent/version
func (h *UpgradeHandler) GetVersion(c *gin.Context) {
	arch := c.DefaultQuery("arch", "amd64")

	// 优先使用配置中的固定版本
	if h.cfg.AgentRelease.CurrentVersion != "" {
		version := h.cfg.AgentRelease.CurrentVersion
		downloadURL := fmt.Sprintf("/api/agent/download?version=%s&arch=%s", version, arch)

		resp := VersionResponse{
			Version:     version,
			DownloadURL: downloadURL,
		}
		Success(c, resp)
		return
	}

	// 自动从 GitHub Releases API 获取最新版本
	version, checksumMap, releaseNotes, err := h.getLatestRelease()
	if err != nil {
		h.logger.Error("failed to fetch latest release from GitHub", zap.Error(err))
		Error(c, http.StatusBadGateway, 502, fmt.Sprintf("failed to fetch latest release: %v", err))
		return
	}

	checksum := ""
	if c, ok := checksumMap[arch]; ok {
		checksum = c
	}

	downloadURL := fmt.Sprintf("/api/agent/download?version=%s&arch=%s", version, arch)

	resp := VersionResponse{
		Version:        version,
		ChecksumSHA256: checksum,
		DownloadURL:    downloadURL,
		ReleaseNotes:   releaseNotes,
	}

	Success(c, resp)
}

// DownloadBinary 下载 Agent 二进制文件（302 重定向到 GitHub Release）
// GET /api/agent/download?os=linux&arch=amd64&version=latest
func (h *UpgradeHandler) DownloadBinary(c *gin.Context) {
	osParam := c.DefaultQuery("os", "linux")
	archParam := c.DefaultQuery("arch", "amd64")
	version := c.DefaultQuery("version", "latest")

	repo := h.getRepo()

	// 如果 version 是 "latest" 且配置了固定版本，使用固定版本
	if version == "latest" && h.cfg.AgentRelease.CurrentVersion != "" {
		version = h.cfg.AgentRelease.CurrentVersion
	}

	var url string
	if version == "latest" {
		url = fmt.Sprintf("https://github.com/%s/releases/latest/download/nexus-xray-agent-%s-%s", repo, osParam, archParam)
	} else {
		// 确保 version 带 v 前缀
		if !strings.HasPrefix(version, "v") {
			version = "v" + version
		}
		url = fmt.Sprintf("https://github.com/%s/releases/download/%s/nexus-xray-agent-%s-%s", repo, version, osParam, archParam)
	}

	h.logger.Info("redirecting agent binary download",
		zap.String("version", version),
		zap.String("os", osParam),
		zap.String("arch", archParam),
		zap.String("url", url),
	)

	c.Redirect(302, url)
}

// getRepo 返回 GitHub 仓库路径
func (h *UpgradeHandler) getRepo() string {
	if h.cfg.AgentRelease.GithubRepo != "" {
		return h.cfg.AgentRelease.GithubRepo
	}
	return "shangui999/nexus-xray"
}

// getLatestRelease 从 GitHub Releases API 获取最新版本（带缓存）
func (h *UpgradeHandler) getLatestRelease() (string, map[string]string, string, error) {
	// 检查缓存
	h.cache.mu.RLock()
	if h.cache.version != "" && time.Since(h.cache.fetchedAt) < cacheTTL {
		version := h.cache.version
		checksum := h.cache.checksum
		h.cache.mu.RUnlock()
		return version, checksum, "", nil
	}
	h.cache.mu.RUnlock()

	// 请求 GitHub API
	repo := h.getRepo()
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	h.logger.Debug("fetching latest release from GitHub", zap.String("url", apiURL))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", nil, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("github api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, "", fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", nil, "", fmt.Errorf("decode github response: %w", err)
	}

	// 从 assets 中解析 checksums.txt
	checksumMap := h.parseChecksums(release.Assets)

	// 更新缓存
	h.cache.mu.Lock()
	h.cache.version = release.TagName
	h.cache.checksum = checksumMap
	h.cache.fetchedAt = time.Now()
	h.cache.mu.Unlock()

	return release.TagName, checksumMap, release.Body, nil
}

// parseChecksums 从 GitHub Release 的 assets 中解析 checksums.txt
func (h *UpgradeHandler) parseChecksums(assets []struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int    `json:"size"`
}) map[string]string {
	checksumMap := make(map[string]string)

	// 查找 checksums.txt asset
	var checksumURL string
	for _, asset := range assets {
		if asset.Name == "checksums.txt" || asset.Name == "sha256sums.txt" {
			checksumURL = asset.BrowserDownloadURL
			break
		}
	}

	if checksumURL == "" {
		return checksumMap
	}

	// 下载 checksums 文件
	resp, err := http.Get(checksumURL)
	if err != nil {
		h.logger.Warn("failed to download checksums file", zap.Error(err))
		return checksumMap
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		h.logger.Warn("checksums file download failed", zap.Int("status", resp.StatusCode))
		return checksumMap
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		h.logger.Warn("failed to read checksums file", zap.Error(err))
		return checksumMap
	}

	// 解析每行，格式: <sha256>  nexus-xray-agent-linux-<arch>
	lines := strings.Split(string(bodyBytes), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			continue
		}
		hash := parts[0]
		filename := parts[1]

		// 从文件名提取 arch: nexus-xray-agent-linux-<arch>
		if strings.HasPrefix(filename, "nexus-xray-agent-linux-") {
			arch := strings.TrimPrefix(filename, "nexus-xray-agent-linux-")
			checksumMap[arch] = hash
		}
	}

	return checksumMap
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
