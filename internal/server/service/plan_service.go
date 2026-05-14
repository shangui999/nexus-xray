package service

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// PlanService 套餐业务逻辑
type PlanService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewPlanService 创建套餐服务
func NewPlanService(db *gorm.DB, logger *zap.Logger) *PlanService {
	return &PlanService{
		db:     db,
		logger: logger,
	}
}

// AssignPlan 为用户分配套餐
// 1. 获取 Plan 信息
// 2. 设置 user.QuotaBytes = plan.QuotaBytes
// 3. 设置 user.ExpiresAt = now + plan.DurationDays
// 4. 设置 user.MaxConnections = plan.MaxConnections
// 5. 设置 user.TrafficRate = plan.TrafficRate
// 6. 重置 user.UsedBytes = 0
// 7. 设置 user.State = "active"
func (s *PlanService) AssignPlan(userID, planID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	pid, err := uuid.Parse(planID)
	if err != nil {
		return fmt.Errorf("invalid plan_id: %w", err)
	}

	// Get plan
	var plan model.Plan
	if err := s.db.First(&plan, "id = ?", pid).Error; err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// Get user
	var user model.User
	if err := s.db.First(&user, "id = ?", uid).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Update user with plan settings
	expiresAt := time.Now().AddDate(0, 0, plan.DurationDays)
	user.PlanID = &pid
	user.QuotaBytes = plan.QuotaBytes
	user.ExpiresAt = &expiresAt
	user.MaxConnections = plan.MaxConnections
	user.TrafficRate = plan.TrafficRate
	user.UsedBytes = 0
	user.State = "active"

	if err := s.db.Save(&user).Error; err != nil {
		return fmt.Errorf("failed to assign plan: %w", err)
	}

	s.logger.Info("plan assigned to user",
		zap.String("user_id", userID),
		zap.String("plan_id", planID),
	)

	return nil
}

// RenewPlan 续费（延长有效期）
func (s *PlanService) RenewPlan(userID, planID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user_id: %w", err)
	}

	pid, err := uuid.Parse(planID)
	if err != nil {
		return fmt.Errorf("invalid plan_id: %w", err)
	}

	// Get plan
	var plan model.Plan
	if err := s.db.First(&plan, "id = ?", pid).Error; err != nil {
		return fmt.Errorf("plan not found: %w", err)
	}

	// Get user
	var user model.User
	if err := s.db.First(&user, "id = ?", uid).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Extend expiry: if current expiry is in the future, extend from it; otherwise from now
	baseTime := time.Now()
	if user.ExpiresAt != nil && user.ExpiresAt.After(baseTime) {
		baseTime = *user.ExpiresAt
	}

	newExpiry := baseTime.AddDate(0, 0, plan.DurationDays)
	user.ExpiresAt = &newExpiry
	user.PlanID = &pid

	// Update plan settings
	user.QuotaBytes = plan.QuotaBytes
	user.MaxConnections = plan.MaxConnections
	user.TrafficRate = plan.TrafficRate
	user.UsedBytes = 0 // 续费重置流量
	user.State = "active"

	if err := s.db.Save(&user).Error; err != nil {
		return fmt.Errorf("failed to renew plan: %w", err)
	}

	s.logger.Info("plan renewed for user",
		zap.String("user_id", userID),
		zap.String("plan_id", planID),
		zap.Time("new_expiry", newExpiry),
	)

	return nil
}
