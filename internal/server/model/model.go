package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSON 类型用于 GORM 的 JSONB 字段，底层基于 json.RawMessage
type JSON json.RawMessage

// Value 实现 driver.Valuer 接口，将 JSON 值写入数据库
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan 实现 sql.Scanner 接口，从数据库读取 JSON 值
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return json.Unmarshal([]byte(string(value.([]byte))), j)
	}
	*j = append((*j)[0:0], bytes...)
	return nil
}

// Admin 管理员
type Admin struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Username     string    `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"size:256;not null" json:"-"`
	Role         string    `gorm:"size:32;default:admin" json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BeforeCreate 自动生成 UUID
func (a *Admin) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Node 节点
type Node struct {
	ID               uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Name             string         `gorm:"size:128;not null" json:"name"`
	Address          string         `gorm:"size:256" json:"address"`
	SubscriptionAddr string         `gorm:"size:256" json:"subscription_addr"` // 自定义订阅地址（域名或IP，覆盖 Address）
	IPv6Addr         string         `gorm:"size:256" json:"ipv6_addr"`          // IPv6 地址
	Status           string         `gorm:"size:32;default:offline" json:"status"` // online/offline/error
	LastHeartbeat    *time.Time     `json:"last_heartbeat"`
	Config           JSON           `gorm:"type:jsonb" json:"config"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 自动生成 UUID
func (n *Node) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// User 订阅用户
type User struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Email          string         `gorm:"size:256;uniqueIndex" json:"email"`
	Username       string         `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash   string         `gorm:"size:256" json:"-"`
	VlessUUID      string         `gorm:"size:36;uniqueIndex" json:"vless_uuid"`
	QuotaBytes     int64          `gorm:"default:0" json:"quota_bytes"`
	UsedBytes      int64          `gorm:"default:0" json:"used_bytes"`
	ExpiresAt      *time.Time     `json:"expires_at"`
	State          string         `gorm:"size:32;default:active" json:"state"` // active/suspended/expired
	TrafficRate    float64        `gorm:"default:1.0" json:"traffic_rate"`
	MaxConnections int            `gorm:"default:3" json:"max_connections"`
	PlanID         *uuid.UUID     `gorm:"type:uuid" json:"plan_id"`
	Plan           *Plan          `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 自动生成 UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Plan 套餐
type Plan struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	QuotaBytes     int64     `gorm:"not null" json:"quota_bytes"`
	DurationDays   int       `gorm:"not null" json:"duration_days"`
	MaxConnections int       `gorm:"default:3" json:"max_connections"`
	TrafficRate    float64   `gorm:"default:1.0" json:"traffic_rate"`
	PriceCents     int       `gorm:"not null" json:"price_cents"`
	IsActive       bool      `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// BeforeCreate 自动生成 UUID
func (p *Plan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// Inbound 入站配置
type Inbound struct {
	ID             uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	NodeID         uuid.UUID      `gorm:"type:uuid;not null" json:"node_id"`
	Node           *Node          `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	Protocol       string         `gorm:"size:32;not null" json:"protocol"` // vless/trojan/shadowsocks
	Port           int            `gorm:"not null" json:"port"`
	Settings       JSON           `gorm:"type:jsonb;not null" json:"settings"`
	StreamSettings JSON           `gorm:"type:jsonb" json:"stream_settings"`
	Tag            string         `gorm:"size:128;uniqueIndex;not null" json:"tag"`
	Enabled        bool           `gorm:"default:true" json:"enabled"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// BeforeCreate 自动生成 UUID
func (i *Inbound) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// TrafficLog 流量记录
type TrafficLog struct {
	ID            uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID        uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	NodeID        uuid.UUID `gorm:"type:uuid;index" json:"node_id"`
	UploadBytes   int64     `gorm:"default:0" json:"upload_bytes"`
	DownloadBytes int64     `gorm:"default:0" json:"download_bytes"`
	RecordedAt    time.Time `gorm:"index" json:"recorded_at"`
}

// EnrollmentToken 节点注册令牌
type EnrollmentToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	NodeID    uuid.UUID `gorm:"type:uuid" json:"node_id"`
	TokenHash string    `gorm:"size:256;not null" json:"-"`
	Used      bool      `gorm:"default:false" json:"used"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate 自动生成 UUID
func (e *EnrollmentToken) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
