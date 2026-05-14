package api

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"gorm.io/gorm"
)

// InboundHandler 入站配置处理器
type InboundHandler struct {
	db *gorm.DB
}

// NewInboundHandler 创建入站配置处理器
func NewInboundHandler(db *gorm.DB) *InboundHandler {
	return &InboundHandler{db: db}
}

type createInboundRequest struct {
	NodeID         string          `json:"node_id" binding:"required"`
	Protocol       string          `json:"protocol" binding:"required"`
	Port           int             `json:"port" binding:"required"`
	Settings       json.RawMessage `json:"settings" binding:"required"`
	StreamSettings json.RawMessage `json:"stream_settings"`
	Tag            string          `json:"tag" binding:"required"`
}

type updateInboundRequest struct {
	Protocol       *string         `json:"protocol"`
	Port           *int            `json:"port"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"stream_settings"`
	Tag            *string         `json:"tag"`
	Enabled        *bool           `json:"enabled"`
}

// List 返回入站配置列表，支持 ?node_id=xxx 过滤
// GET /api/inbounds
func (h *InboundHandler) List(c *gin.Context) {
	query := h.db.Model(&model.Inbound{})

	if nodeID := c.Query("node_id"); nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}

	var inbounds []model.Inbound
	if err := query.Preload("Node").Find(&inbounds).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list inbounds")
		return
	}

	Success(c, inbounds)
}

// Create 创建入站配置
// POST /api/inbounds
func (h *InboundHandler) Create(c *gin.Context) {
	var req createInboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	nodeID, err := uuid.Parse(req.NodeID)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid node_id")
		return
	}

	inbound := model.Inbound{
		NodeID:         nodeID,
		Protocol:       req.Protocol,
		Port:           req.Port,
		Settings:       model.JSON(req.Settings),
		StreamSettings: model.JSON(req.StreamSettings),
		Tag:            req.Tag,
		Enabled:        true,
	}

	if err := h.db.Create(&inbound).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create inbound")
		return
	}

	Success(c, inbound)
}

// Update 更新入站配置
// PUT /api/inbounds/:id
func (h *InboundHandler) Update(c *gin.Context) {
	id := c.Param("id")
	inboundID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid inbound ID")
		return
	}

	var inbound model.Inbound
	if err := h.db.First(&inbound, inboundID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "inbound not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get inbound")
		return
	}

	var req updateInboundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.Protocol != nil {
		updates["protocol"] = *req.Protocol
	}
	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.Settings != nil {
		updates["settings"] = model.JSON(req.Settings)
	}
	if req.StreamSettings != nil {
		updates["stream_settings"] = model.JSON(req.StreamSettings)
	}
	if req.Tag != nil {
		updates["tag"] = *req.Tag
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	if len(updates) > 0 {
		if err := h.db.Model(&inbound).Updates(updates).Error; err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to update inbound")
			return
		}
	}

	h.db.Preload("Node").First(&inbound, inboundID)
	Success(c, inbound)
}

// Delete 删除入站配置
// DELETE /api/inbounds/:id
func (h *InboundHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	inboundID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid inbound ID")
		return
	}

	if err := h.db.Delete(&model.Inbound{}, inboundID).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete inbound")
		return
	}

	Success(c, nil)
}
