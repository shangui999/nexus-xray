package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"gorm.io/gorm"
)

// PlanHandler 套餐管理处理器
type PlanHandler struct {
	db *gorm.DB
}

// NewPlanHandler 创建套餐管理处理器
func NewPlanHandler(db *gorm.DB) *PlanHandler {
	return &PlanHandler{db: db}
}

type createPlanRequest struct {
	Name           string  `json:"name" binding:"required"`
	QuotaBytes     int64   `json:"quota_bytes" binding:"required"`
	DurationDays   int     `json:"duration_days" binding:"required"`
	MaxConnections int     `json:"max_connections"`
	TrafficRate    float64 `json:"traffic_rate"`
	PriceCents     int     `json:"price_cents" binding:"required"`
}

type updatePlanRequest struct {
	Name           *string  `json:"name"`
	QuotaBytes     *int64   `json:"quota_bytes"`
	DurationDays   *int     `json:"duration_days"`
	MaxConnections *int     `json:"max_connections"`
	TrafficRate    *float64 `json:"traffic_rate"`
	PriceCents     *int     `json:"price_cents"`
	IsActive       *bool    `json:"is_active"`
}

// List 返回所有套餐
// GET /api/plans
func (h *PlanHandler) List(c *gin.Context) {
	var plans []model.Plan
	if err := h.db.Find(&plans).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list plans")
		return
	}
	Success(c, plans)
}

// Create 创建套餐
// POST /api/plans
func (h *PlanHandler) Create(c *gin.Context) {
	var req createPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	plan := model.Plan{
		Name:           req.Name,
		QuotaBytes:     req.QuotaBytes,
		DurationDays:   req.DurationDays,
		MaxConnections: req.MaxConnections,
		TrafficRate:    req.TrafficRate,
		PriceCents:     req.PriceCents,
		IsActive:       true,
	}

	// 设置默认值
	if plan.MaxConnections == 0 {
		plan.MaxConnections = 3
	}
	if plan.TrafficRate == 0 {
		plan.TrafficRate = 1.0
	}

	if err := h.db.Create(&plan).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create plan")
		return
	}

	Success(c, plan)
}

// Update 更新套餐
// PUT /api/plans/:id
func (h *PlanHandler) Update(c *gin.Context) {
	id := c.Param("id")
	planID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid plan ID")
		return
	}

	var plan model.Plan
	if err := h.db.First(&plan, planID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "plan not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get plan")
		return
	}

	var req updatePlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.QuotaBytes != nil {
		updates["quota_bytes"] = *req.QuotaBytes
	}
	if req.DurationDays != nil {
		updates["duration_days"] = *req.DurationDays
	}
	if req.MaxConnections != nil {
		updates["max_connections"] = *req.MaxConnections
	}
	if req.TrafficRate != nil {
		updates["traffic_rate"] = *req.TrafficRate
	}
	if req.PriceCents != nil {
		updates["price_cents"] = *req.PriceCents
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) > 0 {
		if err := h.db.Model(&plan).Updates(updates).Error; err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to update plan")
			return
		}
	}

	h.db.First(&plan, planID)
	Success(c, plan)
}

// Delete 删除套餐
// DELETE /api/plans/:id
func (h *PlanHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	planID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid plan ID")
		return
	}

	if err := h.db.Delete(&model.Plan{}, planID).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete plan")
		return
	}

	Success(c, nil)
}
