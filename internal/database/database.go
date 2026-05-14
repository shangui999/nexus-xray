package database

import (
	"fmt"
	"time"

	"github.com/shangui999/nexus-xray/internal/common/config"
	"github.com/shangui999/nexus-xray/internal/server/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDB 根据配置连接 PostgreSQL 并执行 AutoMigrate
func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	// 1. 构建 DSN
	dsn := cfg.DSN()

	// 2. gorm.Open with postgres driver
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 3. 设置连接池参数
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 4. AutoMigrate 所有模型
	if err := db.AutoMigrate(
		&model.Admin{},
		&model.Node{},
		&model.Plan{},
		&model.User{},
		&model.Inbound{},
		&model.TrafficLog{},
		&model.EnrollmentToken{},
	); err != nil {
		return nil, fmt.Errorf("failed to auto migrate: %w", err)
	}

	// 5. 创建默认管理员（如果不存在）
	var count int64
	db.Model(&model.Admin{}).Count(&count)
	if count == 0 {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash default admin password: %w", err)
		}
		admin := model.Admin{
			Username:     "admin",
			PasswordHash: string(hashedPassword),
			Role:         "admin",
		}
		if err := db.Create(&admin).Error; err != nil {
			return nil, fmt.Errorf("failed to create default admin: %w", err)
		}
	}

	return db, nil
}
