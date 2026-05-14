package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
	PlanID   string `json:"plan_id,omitempty"`
}

// UserService 用户业务逻辑
type UserService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewUserService 创建用户服务
func NewUserService(db *gorm.DB, logger *zap.Logger) *UserService {
	return &UserService{
		db:     db,
		logger: logger,
	}
}

// CreateUser 创建用户并分配套餐
// 1. 创建 User 记录
// 2. 如果指定了 plan_id，设置 quota 和 expires_at
// 3. 为用户生成 UUID（用作 VLESS 的 user id）
func (s *UserService) CreateUser(req *CreateUserRequest) (*model.User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &model.User{
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		VlessUUID:    uuid.New().String(),
		State:        "active",
		TrafficRate:  1.0,
	}

	// If plan_id specified, assign plan
	if req.PlanID != "" {
		planID, err := uuid.Parse(req.PlanID)
		if err != nil {
			return nil, fmt.Errorf("invalid plan_id: %w", err)
		}

		var plan model.Plan
		if err := s.db.First(&plan, "id = ?", planID).Error; err != nil {
			return nil, fmt.Errorf("plan not found: %w", err)
		}

		user.PlanID = &planID
		user.QuotaBytes = plan.QuotaBytes
		user.TrafficRate = plan.TrafficRate
		user.MaxConnections = plan.MaxConnections
		expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)
		user.ExpiresAt = &expiresAt
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.logger.Info("user created",
		zap.String("user_id", user.ID.String()),
		zap.String("username", user.Username),
	)

	return user, nil
}

// SuspendUser 封停用户（超额/过期）
func (s *UserService) SuspendUser(userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	result := s.db.Model(&model.User{}).
		Where("id = ? AND state != ?", id, "suspended").
		Update("state", "suspended")
	if result.Error != nil {
		return fmt.Errorf("failed to suspend user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user %s not found or already suspended", userID)
	}

	s.logger.Info("user suspended", zap.String("user_id", userID))
	return nil
}

// ReactivateUser 恢复用户
func (s *UserService) ReactivateUser(userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	result := s.db.Model(&model.User{}).
		Where("id = ? AND state = ?", id, "suspended").
		Update("state", "active")
	if result.Error != nil {
		return fmt.Errorf("failed to reactivate user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("user %s not found or not suspended", userID)
	}

	s.logger.Info("user reactivated", zap.String("user_id", userID))
	return nil
}

// UpdateTraffic 更新用户流量使用量
// 由 NodeHub 收到 usage_push 后调用
func (s *UserService) UpdateTraffic(userID, nodeID string, upload, download int64) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	nid, err := uuid.Parse(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node_id: %w", err)
	}

	// Get user to apply traffic rate
	var user model.User
	if err := s.db.First(&user, "id = ?", uid).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Apply traffic rate: actual deduction = actual usage * traffic_rate
	ratedUpload := int64(float64(upload) * user.TrafficRate)
	ratedDownload := int64(float64(download) * user.TrafficRate)
	ratedTotal := ratedUpload + ratedDownload

	// Update user's used bytes
	if err := s.db.Model(&user).
		Update("used_bytes", gorm.Expr("used_bytes + ?", ratedTotal)).Error; err != nil {
		return fmt.Errorf("failed to update user traffic: %w", err)
	}

	// Create traffic log (raw bytes, no rate applied)
	trafficLog := model.TrafficLog{
		UserID:        uid,
		NodeID:        nid,
		UploadBytes:   upload,
		DownloadBytes: download,
		RecordedAt:    time.Now(),
	}

	if err := s.db.Create(&trafficLog).Error; err != nil {
		return fmt.Errorf("failed to create traffic log: %w", err)
	}

	s.logger.Debug("traffic updated",
		zap.String("user_id", userID),
		zap.String("node_id", nodeID),
		zap.Int64("upload", upload),
		zap.Int64("download", download),
		zap.Int64("rated_total", ratedTotal),
	)

	return nil
}

// CheckQuota 检查用户是否超额
func (s *UserService) CheckQuota(userID string) (bool, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user_id: %w", err)
	}

	var user model.User
	if err := s.db.First(&user, "id = ?", id).Error; err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	// If quota is 0, it means unlimited
	if user.QuotaBytes == 0 {
		return false, nil
	}

	return user.UsedBytes >= user.QuotaBytes, nil
}
