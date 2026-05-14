package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerConfig 是 Server 端的完整配置结构
type ServerConfig struct {
	Server        ServerConfigSection `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Log           LogConfig           `yaml:"log"`
	AgentRelease  AgentReleaseConfig  `yaml:"agent_release"`
}

// AgentReleaseConfig 定义 Agent 二进制发布相关配置
type AgentReleaseConfig struct {
	Dir            string `yaml:"dir"`
	CurrentVersion string `yaml:"current_version"`
}

// ServerConfigSection 定义 Server 的 HTTP/gRPC 端口和 JWT 密钥
type ServerConfigSection struct {
	HTTPPort    int    `yaml:"http_port"`
	GRPCPort    int    `yaml:"grpc_port"`
	JWTSecret   string `yaml:"jwt_secret"`
	ExternalAddr string `yaml:"external_addr"` // 对外 gRPC 地址（Agent 连接用）
}

// DatabaseConfig 定义 PostgreSQL 连接参数
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
	SSLMode  string `yaml:"sslmode"`
}

// DSN 返回 PostgreSQL 连接字符串
func (d *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.DBName, d.SSLMode,
	)
}

// AgentConfig 是 Agent 端的完整配置结构
type AgentConfig struct {
	Agent AgentConfigSection `yaml:"agent"`
	Xray  XrayConfig         `yaml:"xray"`
	Log   LogConfig          `yaml:"log"`
}

// AgentConfigSection 定义 Agent 连接 Server 所需的参数
type AgentConfigSection struct {
	NodeID           string        `yaml:"node_id"`
	ServerAddr       string        `yaml:"server_addr"`
	CertDir          string        `yaml:"cert_dir"`
	StatsInterval    time.Duration `yaml:"stats_interval"`
	NodeToken        string        `yaml:"node_token"`
	UpdateInterval   time.Duration `yaml:"update_interval"`
	HTTPAddr         string        `yaml:"http_addr"` // Server HTTP 地址（用于下载升级包）
}

// XrayConfig 定义 Xray 核心相关配置
type XrayConfig struct {
	LogLevel string `yaml:"log_level"`
}

// LogConfig 定义日志级别和格式
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// LoadServerConfig 从 YAML 文件加载 Server 配置
func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &cfg, nil
}

// LoadAgentConfig 从 YAML 文件加载 Agent 配置
func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &cfg, nil
}
