package main

import (
	"fmt"
	"os"

	"github.com/shangui999/nexus-xray/internal/common/config"
	"github.com/shangui999/nexus-xray/internal/database"
	"github.com/shangui999/nexus-xray/internal/server/api"
	"go.uber.org/zap"
)

func main() {
	// 加载配置
	cfg, err := config.LoadServerConfig("configs/server.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化 Logger
	var logger *zap.Logger
	if cfg.Log.Format == "json" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("xray-manager server starting",
		zap.Int("http_port", cfg.Server.HTTPPort),
		zap.Int("grpc_port", cfg.Server.GRPCPort),
	)

	// 初始化数据库连接
	db, err := database.InitDB(&cfg.Database)
	if err != nil {
		logger.Fatal("failed to init database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	logger.Info("database initialized")

	// 启动 HTTP server (Gin)
	router := api.SetupRouter(db, cfg, logger)
	go func() {
		if err := router.Run(fmt.Sprintf(":%d", cfg.Server.HTTPPort)); err != nil {
			logger.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()
	logger.Info("HTTP server started", zap.Int("port", cfg.Server.HTTPPort))

	// TODO: 启动 gRPC server (NodeHub)
	// TODO: 启动定时任务 (cron)

	select {}
}
