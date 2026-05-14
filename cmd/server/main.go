package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/shangui999/nexus-xray/internal/common/config"
	"github.com/shangui999/nexus-xray/internal/database"
	"github.com/shangui999/nexus-xray/internal/server/api"
	"github.com/shangui999/nexus-xray/internal/server/nodehub"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
		zap.Int("port", cfg.Server.HTTPPort),
	)

	// 初始化数据库连接
	db, err := database.InitDB(&cfg.Database)
	if err != nil {
		logger.Fatal("failed to init database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	logger.Info("database initialized")

	// 创建 gRPC server（不单独监听，由合并 handler 统一管理）
	hub := nodehub.NewHub(logger)
	grpcServer := nodehub.NewGRPCServer(hub, nil) // h2c 模式下不使用 TLS

	// 创建 Gin HTTP router
	router := api.SetupRouter(db, cfg, logger)

	// 合并 handler：gRPC 请求走 grpcServer，其余走 Gin router
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			router.ServeHTTP(w, r)
		}
	})

	// 使用 h2c 支持明文 HTTP/2（gRPC 需要 HTTP/2）
	h2cHandler := h2c.NewHandler(handler, &http2.Server{})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.HTTPPort),
		Handler: h2cHandler,
	}

	logger.Info("server starting (HTTP + gRPC on same port)", zap.Int("port", cfg.Server.HTTPPort))
	if err := server.ListenAndServe(); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}
