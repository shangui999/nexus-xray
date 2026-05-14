package xray

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// XrayConfig Xray 完整配置
type XrayConfig struct {
	Log       *LogConfig       `json:"log,omitempty"`
	API       *APIConfig       `json:"api,omitempty"`
	Stats     *StatsConfig     `json:"stats,omitempty"`
	Policy    *PolicyConfig    `json:"policy,omitempty"`
	Inbounds  []InboundConfig  `json:"inbounds"`
	Outbounds []OutboundConfig `json:"outbounds"`
	Routing   *RoutingConfig   `json:"routing,omitempty"`
}

// LogConfig 日志配置
type LogConfig struct {
	LogLevel string `json:"loglevel"`
}

// APIConfig API 服务配置
type APIConfig struct {
	Tag      string   `json:"tag"`
	Services []string `json:"services"`
}

// StatsConfig 统计配置
type StatsConfig struct{}

// PolicyConfig 策略配置
type PolicyConfig struct {
	Levels map[string]PolicyLevel `json:"levels"`
	System *SystemPolicy          `json:"system"`
}

// PolicyLevel 策略级别
type PolicyLevel struct {
	StatsUserUplink   bool `json:"statsUserUplink"`
	StatsUserDownlink bool `json:"statsUserDownlink"`
}

// SystemPolicy 系统策略
type SystemPolicy struct {
	StatsInboundUplink    bool `json:"statsInboundUplink"`
	StatsInboundDownlink  bool `json:"statsInboundDownlink"`
	StatsOutboundUplink   bool `json:"statsOutboundUplink"`
	StatsOutboundDownlink bool `json:"statsOutboundDownlink"`
}

// InboundConfig 入站配置
type InboundConfig struct {
	Tag            string          `json:"tag"`
	Port           int             `json:"port"`
	Listen         string          `json:"listen,omitempty"`
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings,omitempty"`
}

// OutboundConfig 出站配置
type OutboundConfig struct {
	Tag      string `json:"tag"`
	Protocol string `json:"protocol"`
}

// RoutingConfig 路由配置
type RoutingConfig struct {
	Rules []RoutingRule `json:"rules"`
}

// RoutingRule 路由规则
type RoutingRule struct {
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Type        string   `json:"type"`
}

// ProxyUser 代理用户
type ProxyUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Level int    `json:"level"`
}

// UserTraffic 用户流量
type UserTraffic struct {
	Email    string `json:"email"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// InboundSettings 入站用户列表（用于 AddUser/RemoveUser）
type InboundSettings struct {
	Clients []ProxyUser `json:"clients,omitempty"`
}

// xrayLogWriter 将 Xray 进程输出适配为 zap 日志
type xrayLogWriter struct {
	logger *zap.Logger
}

func (w *xrayLogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n\r")
	if msg != "" {
		w.logger.Debug("xray", zap.String("output", msg))
	}
	return len(p), nil
}

// Manager Xray 进程管理器
type Manager struct {
	mu            sync.RWMutex
	cmd           *exec.Cmd
	configPath    string
	xrayBin       string
	apiAddr       string
	logger        *zap.Logger
	running       bool
	currentConfig *XrayConfig
	processDone   chan struct{}
}

// NewManager 创建管理器
func NewManager(xrayBin, configPath, apiAddr string, logger *zap.Logger) *Manager {
	return &Manager{
		xrayBin:    xrayBin,
		configPath: configPath,
		apiAddr:    apiAddr,
		logger:     logger,
	}
}

// Start 启动 Xray 进程
func (m *Manager) Start(config *XrayConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("xray is already running")
	}

	return m.startInternal(config)
}

func (m *Manager) startInternal(config *XrayConfig) error {
	// 写入配置文件
	if err := m.writeConfig(config); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// 创建命令
	m.cmd = exec.Command(m.xrayBin, "run", "-c", m.configPath)
	m.cmd.Stdout = &xrayLogWriter{logger: m.logger}
	m.cmd.Stderr = &xrayLogWriter{logger: m.logger}

	// 启动进程
	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start xray: %w", err)
	}

	m.running = true
	m.currentConfig = config
	m.processDone = make(chan struct{})

	// 监控进程退出
	go m.monitorProcess()

	m.logger.Info("xray process started", zap.Int("pid", m.cmd.Process.Pid))

	return nil
}

// monitorProcess 监控 Xray 进程状态
func (m *Manager) monitorProcess() {
	err := m.cmd.Wait()

	m.mu.Lock()
	m.running = false
	m.cmd = nil
	m.mu.Unlock()

	if err != nil {
		m.logger.Error("xray process exited with error", zap.Error(err))
	} else {
		m.logger.Info("xray process exited normally")
	}

	close(m.processDone)
}

// Reload 重新加载配置（停止 + 重新启动）
func (m *Manager) Reload(config *XrayConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		if err := m.stopInternal(); err != nil {
			return fmt.Errorf("stop xray: %w", err)
		}
	}

	return m.startInternal(config)
}

// Stop 停止 Xray 进程
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.stopInternal()
}

func (m *Manager) stopInternal() error {
	if !m.running || m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	m.logger.Info("stopping xray process")

	// 发送 SIGTERM
	if err := m.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		m.logger.Warn("failed to send SIGTERM, killing process", zap.Error(err))
		_ = m.cmd.Process.Kill()
	}

	// 等待进程退出（超时 10s）
	select {
	case <-m.processDone:
		m.logger.Info("xray process stopped")
	case <-time.After(10 * time.Second):
		m.logger.Warn("xray process did not stop in 10s, killing")
		_ = m.cmd.Process.Kill()
		<-m.processDone
	}

	return nil
}

// IsRunning 检查 Xray 是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetCurrentConfig 获取当前配置
func (m *Manager) GetCurrentConfig() *XrayConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentConfig
}

// AddUserToInbound 向指定入站添加用户并重载
func (m *Manager) AddUserToInbound(inboundTag string, user ProxyUser) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentConfig == nil {
		return fmt.Errorf("xray not configured")
	}

	found := false
	for i := range m.currentConfig.Inbounds {
		if m.currentConfig.Inbounds[i].Tag == inboundTag {
			var settings InboundSettings
			if len(m.currentConfig.Inbounds[i].Settings) > 0 {
				if err := json.Unmarshal(m.currentConfig.Inbounds[i].Settings, &settings); err != nil {
					return fmt.Errorf("parse inbound settings: %w", err)
				}
			}

			settings.Clients = append(settings.Clients, user)

			newSettings, err := json.Marshal(&settings)
			if err != nil {
				return fmt.Errorf("marshal inbound settings: %w", err)
			}
			m.currentConfig.Inbounds[i].Settings = json.RawMessage(newSettings)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("inbound %s not found", inboundTag)
	}

	if m.running {
		if err := m.stopInternal(); err != nil {
			return fmt.Errorf("stop xray: %w", err)
		}
		return m.startInternal(m.currentConfig)
	}

	return nil
}

// RemoveUserFromInbound 从指定入站移除用户并重载
func (m *Manager) RemoveUserFromInbound(inboundTag string, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentConfig == nil {
		return fmt.Errorf("xray not configured")
	}

	found := false
	for i := range m.currentConfig.Inbounds {
		if m.currentConfig.Inbounds[i].Tag == inboundTag {
			var settings InboundSettings
			if len(m.currentConfig.Inbounds[i].Settings) > 0 {
				if err := json.Unmarshal(m.currentConfig.Inbounds[i].Settings, &settings); err != nil {
					return fmt.Errorf("parse inbound settings: %w", err)
				}
			}

			newClients := make([]ProxyUser, 0, len(settings.Clients))
			for _, c := range settings.Clients {
				if c.Email != email {
					newClients = append(newClients, c)
				}
			}
			settings.Clients = newClients

			newSettings, err := json.Marshal(&settings)
			if err != nil {
				return fmt.Errorf("marshal inbound settings: %w", err)
			}
			m.currentConfig.Inbounds[i].Settings = json.RawMessage(newSettings)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("inbound %s not found", inboundTag)
	}

	if m.running {
		if err := m.stopInternal(); err != nil {
			return fmt.Errorf("stop xray: %w", err)
		}
		return m.startInternal(m.currentConfig)
	}

	return nil
}

// writeConfig 将配置写入文件
func (m *Manager) writeConfig(config *XrayConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	m.logger.Debug("xray config written", zap.String("path", m.configPath))
	return nil
}

// GenerateDefaultConfig 生成默认配置（含 API、Stats 和 Policy 支持）
func GenerateDefaultConfig() *XrayConfig {
	return &XrayConfig{
		Log: &LogConfig{
			LogLevel: "warning",
		},
		API: &APIConfig{
			Tag:      "api",
			Services: []string{"StatsService"},
		},
		Stats: &StatsConfig{},
		Policy: &PolicyConfig{
			Levels: map[string]PolicyLevel{
				"0": {
					StatsUserUplink:   true,
					StatsUserDownlink: true,
				},
			},
			System: &SystemPolicy{
				StatsInboundUplink:    true,
				StatsInboundDownlink:  true,
				StatsOutboundUplink:   true,
				StatsOutboundDownlink: true,
			},
		},
		Inbounds: []InboundConfig{
			{
				Tag:      "api",
				Port:     10085,
				Listen:   "127.0.0.1",
				Protocol: "dokodemo-door",
				Settings: json.RawMessage(`{"address":""}`),
			},
		},
		Outbounds: []OutboundConfig{
			{
				Tag:      "free",
				Protocol: "freedom",
			},
			{
				Tag:      "blackhole",
				Protocol: "blackhole",
			},
		},
		Routing: &RoutingConfig{
			Rules: []RoutingRule{
				{
					InboundTag:  []string{"api"},
					OutboundTag: "api",
					Type:        "field",
				},
			},
		},
	}
}
