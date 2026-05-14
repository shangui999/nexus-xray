package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"gorm.io/gorm"
)

// StatsHandler 统计处理器
type StatsHandler struct {
	db *gorm.DB
}

// NewStatsHandler 创建统计处理器
func NewStatsHandler(db *gorm.DB) *StatsHandler {
	return &StatsHandler{db: db}
}

// Overview 返回系统概览统计
// GET /api/stats/overview
func (h *StatsHandler) Overview(c *gin.Context) {
	var totalUsers int64
	var activeUsers int64
	var totalNodes int64
	var onlineNodes int64

	h.db.Model(&model.User{}).Count(&totalUsers)
	h.db.Model(&model.User{}).Where("state = ?", "active").Count(&activeUsers)
	h.db.Model(&model.Node{}).Count(&totalNodes)
	h.db.Model(&model.Node{}).Where("status = ?", "online").Count(&onlineNodes)

	// 今日流量统计
	var todayUpload int64
	var todayDownload int64
	today := time.Now().Truncate(24 * time.Hour)
	h.db.Model(&model.TrafficLog{}).
		Where("recorded_at >= ?", today).
		Select("COALESCE(SUM(upload_bytes), 0)").
		Scan(&todayUpload)
	h.db.Model(&model.TrafficLog{}).
		Where("recorded_at >= ?", today).
		Select("COALESCE(SUM(download_bytes), 0)").
		Scan(&todayDownload)

	Success(c, gin.H{
		"total_users":         totalUsers,
		"active_users":       activeUsers,
		"total_nodes":        totalNodes,
		"online_nodes":       onlineNodes,
		"total_traffic_today": todayUpload + todayDownload,
	})
}

// Traffic 返回流量统计
// GET /api/stats/traffic?period=24h/7d/30d
func (h *StatsHandler) Traffic(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")

	var startTime time.Time
	switch period {
	case "7d":
		startTime = time.Now().Add(-7 * 24 * time.Hour)
	case "30d":
		startTime = time.Now().Add(-30 * 24 * time.Hour)
	default: // 24h
		startTime = time.Now().Add(-24 * time.Hour)
	}

	var logs []model.TrafficLog
	if err := h.db.Where("recorded_at > ?", startTime).
		Order("recorded_at ASC").
		Find(&logs).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to get traffic data")
		return
	}

	type trafficEntry struct {
		Time     time.Time `json:"time"`
		Upload   int64     `json:"upload"`
		Download int64     `json:"download"`
	}

	entries := make([]trafficEntry, len(logs))
	for i, log := range logs {
		entries[i] = trafficEntry{
			Time:     log.RecordedAt,
			Upload:   log.UploadBytes,
			Download: log.DownloadBytes,
		}
	}

	Success(c, gin.H{
		"data": entries,
	})
}
