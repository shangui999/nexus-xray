package service

import (
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CronService 管理所有定时任务
type CronService struct {
	cron   *cron.Cron
	db     *gorm.DB
	logger *zap.Logger
}

// NewCronService 创建定时任务服务
func NewCronService(db *gorm.DB, logger *zap.Logger) *CronService {
	return &CronService{
		db:     db,
		logger: logger,
	}
}

// Start 启动定时任务
func (s *CronService) Start() {
	s.cron = cron.New()

	// 每分钟：检查用户配额和有效期
	s.cron.AddFunc("@every 1m", s.checkUsersQuotaAndExpiry)

	// 每小时：汇总流量统计
	s.cron.AddFunc("@every 1h", s.aggregateTraffic)

	// 每天凌晨3点：清理过期数据
	s.cron.AddFunc("0 3 * * *", s.cleanupExpiredData)

	s.cron.Start()
	s.logger.Info("cron service started")
}

// Stop 停止定时任务
func (s *CronService) Stop() {
	if s.cron != nil {
		s.cron.Stop()
		s.logger.Info("cron service stopped")
	}
}

// checkUsersQuotaAndExpiry 检查所有活跃用户
// - 流量超额 -> 设置 state=suspended
// - 已过期 -> 设置 state=expired
func (s *CronService) checkUsersQuotaAndExpiry() {
	s.logger.Debug("checking users quota and expiry")

	now := time.Now()

	// Check quota: active users whose used_bytes >= quota_bytes (and quota > 0)
	quotaResult := s.db.Model(&model.User{}).
		Where("state = ? AND quota_bytes > 0 AND used_bytes >= quota_bytes", "active").
		Update("state", "suspended")
	if quotaResult.Error != nil {
		s.logger.Error("failed to check quota", zap.Error(quotaResult.Error))
	} else if quotaResult.RowsAffected > 0 {
		s.logger.Info("suspended users for quota exceeded",
			zap.Int64("count", quotaResult.RowsAffected))
	}

	// Check expiry: active users whose expires_at < now
	expiryResult := s.db.Model(&model.User{}).
		Where("state = ? AND expires_at IS NOT NULL AND expires_at < ?", "active", now).
		Update("state", "expired")
	if expiryResult.Error != nil {
		s.logger.Error("failed to check expiry", zap.Error(expiryResult.Error))
	} else if expiryResult.RowsAffected > 0 {
		s.logger.Info("expired users", zap.Int64("count", expiryResult.RowsAffected))
	}

	// Also check suspended users that have expired
	suspendedExpiryResult := s.db.Model(&model.User{}).
		Where("state = ? AND expires_at IS NOT NULL AND expires_at < ?", "suspended", now).
		Update("state", "expired")
	if suspendedExpiryResult.Error != nil {
		s.logger.Error("failed to check suspended users expiry", zap.Error(suspendedExpiryResult.Error))
	} else if suspendedExpiryResult.RowsAffected > 0 {
		s.logger.Info("suspended users expired",
			zap.Int64("count", suspendedExpiryResult.RowsAffected))
	}
}

// aggregateTraffic 汇总流量日志到用户的 used_bytes 字段
// 定期对所有用户做一次对账，确保 used_bytes 与流量日志一致
func (s *CronService) aggregateTraffic() {
	s.logger.Debug("aggregating traffic")

	// 查询每个用户的流量日志汇总
	type TrafficSum struct {
		UserID        uuid.UUID
		TotalUpload   int64
		TotalDownload int64
	}

	var sums []TrafficSum
	err := s.db.Model(&model.TrafficLog{}).
		Select("user_id, SUM(upload_bytes) as total_upload, SUM(download_bytes) as total_download").
		Group("user_id").
		Find(&sums).Error

	if err != nil {
		s.logger.Error("failed to query traffic sums", zap.Error(err))
		return
	}

	for _, sum := range sums {
		var user model.User
		if err := s.db.First(&user, "id = ?", sum.UserID).Error; err != nil {
			s.logger.Error("failed to find user for traffic aggregation",
				zap.String("user_id", sum.UserID.String()),
				zap.Error(err),
			)
			continue
		}

		// Apply traffic rate
		ratedTotal := int64(float64(sum.TotalUpload+sum.TotalDownload) * user.TrafficRate)

		// 对账：将 used_bytes 设置为日志汇总值 * 倍率
		if err := s.db.Model(&user).Update("used_bytes", ratedTotal).Error; err != nil {
			s.logger.Error("failed to update user used_bytes",
				zap.String("user_id", user.ID.String()),
				zap.Error(err),
			)
			continue
		}

		s.logger.Debug("traffic aggregated for user",
			zap.String("user_id", sum.UserID.String()),
			zap.Int64("raw_total", sum.TotalUpload+sum.TotalDownload),
			zap.Int64("rated_total", ratedTotal),
		)
	}

	s.logger.Info("traffic aggregation completed", zap.Int("users_processed", len(sums)))
}

// cleanupExpiredData 清理 30 天前的流量日志
func (s *CronService) cleanupExpiredData() {
	s.logger.Debug("cleaning up expired traffic logs")

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	result := s.db.Where("recorded_at < ?", thirtyDaysAgo).Delete(&model.TrafficLog{})
	if result.Error != nil {
		s.logger.Error("failed to cleanup expired traffic logs", zap.Error(result.Error))
		return
	}

	s.logger.Info("expired traffic logs cleaned up",
		zap.Int64("deleted_count", result.RowsAffected))
}
