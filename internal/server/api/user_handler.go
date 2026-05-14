package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserHandler 用户管理处理器
type UserHandler struct {
	db *gorm.DB
}

// NewUserHandler 创建用户管理处理器
func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	PlanID   string `json:"plan_id"`
}

type updateUserRequest struct {
	Email          *string  `json:"email"`
	Password       *string  `json:"password"`
	QuotaBytes     *int64   `json:"quota_bytes"`
	UsedBytes      *int64   `json:"used_bytes"`
	State          *string  `json:"state"`
	TrafficRate    *float64 `json:"traffic_rate"`
	MaxConnections *int     `json:"max_connections"`
	PlanID         *string  `json:"plan_id"`
}

// List 分页列表，支持 ?page=1&size=20&state=active
// GET /api/users
func (h *UserHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	state := c.Query("state")

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	query := h.db.Model(&model.User{})

	if state != "" {
		query = query.Where("state = ?", state)
	}

	var total int64
	query.Count(&total)

	var users []model.User
	offset := (page - 1) * size
	if err := query.Preload("Plan").Offset(offset).Limit(size).Find(&users).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to list users")
		return
	}

	Success(c, gin.H{
		"items": users,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// Create 创建用户并分配套餐
// POST /api/users
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body: "+err.Error())
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to hash password")
		return
	}

	user := model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		State:        "active",
	}

	if req.PlanID != "" {
		planID, err := uuid.Parse(req.PlanID)
		if err != nil {
			Error(c, http.StatusBadRequest, 400, "invalid plan_id")
			return
		}

		var plan model.Plan
		if err := h.db.First(&plan, planID).Error; err != nil {
			Error(c, http.StatusBadRequest, 400, "plan not found")
			return
		}

		user.PlanID = &planID
		user.QuotaBytes = plan.QuotaBytes
		user.MaxConnections = plan.MaxConnections
		user.TrafficRate = plan.TrafficRate

		expiresAt := time.Now().Add(time.Duration(plan.DurationDays) * 24 * time.Hour)
		user.ExpiresAt = &expiresAt
	}

	if err := h.db.Create(&user).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to create user")
		return
	}

	Success(c, user)
}

// Update 更新用户信息
// PUT /api/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id := c.Param("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user ID")
		return
	}

	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			Error(c, http.StatusNotFound, 404, "user not found")
			return
		}
		Error(c, http.StatusInternalServerError, 500, "failed to get user")
		return
	}

	var req updateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid request body")
		return
	}

	updates := make(map[string]interface{})

	if req.Email != nil {
		updates["email"] = *req.Email
	}
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to hash password")
			return
		}
		updates["password_hash"] = string(hashedPassword)
	}
	if req.QuotaBytes != nil {
		updates["quota_bytes"] = *req.QuotaBytes
	}
	if req.UsedBytes != nil {
		updates["used_bytes"] = *req.UsedBytes
	}
	if req.State != nil {
		updates["state"] = *req.State
	}
	if req.TrafficRate != nil {
		updates["traffic_rate"] = *req.TrafficRate
	}
	if req.MaxConnections != nil {
		updates["max_connections"] = *req.MaxConnections
	}
	if req.PlanID != nil {
		if *req.PlanID == "" {
			updates["plan_id"] = nil
		} else {
			planID, err := uuid.Parse(*req.PlanID)
			if err != nil {
				Error(c, http.StatusBadRequest, 400, "invalid plan_id")
				return
			}
			updates["plan_id"] = planID
		}
	}

	if len(updates) > 0 {
		if err := h.db.Model(&user).Updates(updates).Error; err != nil {
			Error(c, http.StatusInternalServerError, 500, "failed to update user")
			return
		}
	}

	// 重新加载更新后的用户
	h.db.Preload("Plan").First(&user, userID)

	Success(c, user)
}

// Delete 软删除用户
// DELETE /api/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user ID")
		return
	}

	if err := h.db.Delete(&model.User{}, userID).Error; err != nil {
		Error(c, http.StatusInternalServerError, 500, "failed to delete user")
		return
	}

	Success(c, nil)
}

// GetTraffic 用户流量统计
// GET /api/users/:id/traffic
func (h *UserHandler) GetTraffic(c *gin.Context) {
	id := c.Param("id")
	userID, err := uuid.Parse(id)
	if err != nil {
		Error(c, http.StatusBadRequest, 400, "invalid user ID")
		return
	}

	period := c.DefaultQuery("period", "day")

	var startTime time.Time
	switch period {
	case "week":
		startTime = time.Now().Add(-7 * 24 * time.Hour)
	case "month":
		startTime = time.Now().Add(-30 * 24 * time.Hour)
	default: // day
		startTime = time.Now().Add(-24 * time.Hour)
	}

	var logs []model.TrafficLog
	if err := h.db.Where("user_id = ? AND recorded_at > ?", userID, startTime).
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
		"period": period,
		"data":   entries,
	})
}
