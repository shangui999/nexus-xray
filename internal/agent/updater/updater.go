package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"go.uber.org/zap"
)

// Version 当前 Agent 版本（编译时注入）
var Version = "dev"

// VersionInfo 服务端返回的版本信息
type VersionInfo struct {
	Version       string `json:"version"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	DownloadURL   string `json:"download_url"`
	ReleaseNotes  string `json:"release_notes"`
}

// Updater Agent 自动升级器
type Updater struct {
	serverAddr     string
	httpAddr       string
	nodeToken      string
	checkInterval  time.Duration
	currentVersion string
	httpClient     *http.Client
	logger         *zap.Logger
	stopCh         chan struct{}
}

// NewUpdater 创建升级器
func NewUpdater(httpAddr, nodeToken, currentVersion string, interval time.Duration, logger *zap.Logger) *Updater {
	return &Updater{
		serverAddr:     httpAddr,
		httpAddr:       httpAddr,
		nodeToken:      nodeToken,
		checkInterval:  interval,
		currentVersion: currentVersion,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		logger:         logger,
		stopCh:         make(chan struct{}),
	}
}

// Start 启动周期性版本检查
func (u *Updater) Start(ctx context.Context) {
	// 启动时立即检查一次
	if err := u.checkAndUpdate(); err != nil {
		u.logger.Error("initial update check failed", zap.Error(err))
	}

	ticker := time.NewTicker(u.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := u.checkAndUpdate(); err != nil {
				u.logger.Error("update check failed", zap.Error(err))
			}
		case <-ctx.Done():
			u.logger.Info("updater stopped by context")
			return
		case <-u.stopCh:
			u.logger.Info("updater stopped")
			return
		}
	}
}

// Stop 停止检查
func (u *Updater) Stop() {
	close(u.stopCh)
}

// checkAndUpdate 检查并执行升级
func (u *Updater) checkAndUpdate() error {
	// 1. 获取最新版本信息
	versionInfo, err := u.checkVersion()
	if err != nil {
		return fmt.Errorf("check version: %w", err)
	}

	u.logger.Debug("version check result",
		zap.String("current", u.currentVersion),
		zap.String("latest", versionInfo.Version),
	)

	// 2. 比较版本号，如果相同则跳过
	if !compareVersions(u.currentVersion, versionInfo.Version) {
		u.logger.Info("agent is up to date", zap.String("version", u.currentVersion))
		return nil
	}

	u.logger.Info("new version available",
		zap.String("current", u.currentVersion),
		zap.String("latest", versionInfo.Version),
		zap.String("release_notes", versionInfo.ReleaseNotes),
	)

	// 3. 下载新二进制到临时文件
	tmpPath, err := u.downloadBinary(versionInfo.DownloadURL)
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}

	// 4. 校验 SHA256
	if versionInfo.ChecksumSHA256 != "" {
		if err := u.verifyChecksum(tmpPath, versionInfo.ChecksumSHA256); err != nil {
			// 校验失败，删除临时文件
			os.Remove(tmpPath)
			return fmt.Errorf("verify checksum: %w", err)
		}
	}

	// 5. 替换当前二进制并重启
	if err := u.applyUpdate(tmpPath); err != nil {
		return fmt.Errorf("apply update: %w", err)
	}

	return nil
}

// checkVersion 从 Server 获取最新版本信息
func (u *Updater) checkVersion() (*VersionInfo, error) {
	url := fmt.Sprintf("%s/api/agent/version", u.httpAddr)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 添加认证 header
	if u.nodeToken != "" {
		req.Header.Set("X-Node-Token", u.nodeToken)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应，支持统一 API 格式 {"code":0,"message":"success","data":{...}}
	var rawResp struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    VersionInfo `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if rawResp.Code != 0 {
		return nil, fmt.Errorf("server error: %s", rawResp.Message)
	}

	return &rawResp.Data, nil
}

// downloadBinary 下载新版本二进制
func (u *Updater) downloadBinary(downloadURL string) (string, error) {
	// 构建完整下载 URL
	url := fmt.Sprintf("%s%s?os=%s&arch=%s", u.httpAddr, downloadURL, runtime.GOOS, runtime.GOARCH)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	// 添加认证 header
	if u.nodeToken != "" {
		req.Header.Set("X-Node-Token", u.nodeToken)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))
	}

	// 创建临时文件
	tmpDir := os.TempDir()
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("agent-new-%d", time.Now().UnixNano()))

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write to temp file: %w", err)
	}

	f.Close()

	u.logger.Info("binary downloaded", zap.String("path", tmpPath), zap.Int64("size", resp.ContentLength))
	return tmpPath, nil
}

// verifyChecksum 校验文件 SHA256
func (u *Updater) verifyChecksum(filePath, expected string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("compute hash: %w", err)
	}

	actual := fmt.Sprintf("%x", h.Sum(nil))
	if actual != expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expected, actual)
	}

	u.logger.Info("checksum verified", zap.String("sha256", actual))
	return nil
}

// applyUpdate 应用升级
func (u *Updater) applyUpdate(newBinaryPath string) error {
	// 1. 获取当前可执行文件路径
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get current executable path: %w", err)
	}

	// 解析符号链接
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("resolve symlink: %w", err)
	}

	u.logger.Info("applying update",
		zap.String("current", currentExe),
		zap.String("new", newBinaryPath),
	)

	// 2. 备份当前文件
	backupPath := currentExe + ".bak"
	if err := os.Rename(currentExe, backupPath); err != nil {
		return fmt.Errorf("backup current binary: %w", err)
	}

	// 3. 移动新文件到当前位置
	if err := os.Rename(newBinaryPath, currentExe); err != nil {
		// 回滚：恢复备份
		if rollbackErr := os.Rename(backupPath, currentExe); rollbackErr != nil {
			u.logger.Error("rollback failed", zap.Error(rollbackErr))
		}
		return fmt.Errorf("move new binary: %w", err)
	}

	// 4. 设置执行权限
	if err := os.Chmod(currentExe, 0755); err != nil {
		u.logger.Warn("failed to set executable permission", zap.Error(err))
	}

	u.logger.Info("update applied, restarting agent")

	// 5. 重启：使用 systemctl（更可靠）
	cmd := exec.Command("systemctl", "restart", "xray-manager-agent")
	if err := cmd.Start(); err != nil {
		// 如果 systemctl 不可用，尝试直接替换进程
		u.logger.Warn("systemctl restart failed, trying direct restart", zap.Error(err))

		// 作为 fallback，通知升级已完成，需要手动重启
		u.logger.Info("update applied successfully, manual restart required")
		return nil
	}

	u.logger.Info("systemctl restart initiated")
	return nil
}

// compareVersions 比较版本号，返回 true 表示需要升级（latest > current）
func compareVersions(current, latest string) bool {
	// 简单字符串比较：如果不同且 latest 不为空，则认为需要升级
	// "dev" 版本总是尝试升级
	if current == "dev" && latest != "" {
		return true
	}
	if current != latest && latest != "" {
		return true
	}
	return false
}