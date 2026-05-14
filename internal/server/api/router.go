package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/shangui999/nexus-xray/internal/common/config"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// SetupRouter 初始化 Gin Engine 并注册所有路由
func SetupRouter(db *gorm.DB, cfg *config.ServerConfig, logger *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())
	r.Use(LoggerMiddleware(logger))

	// 初始化处理器
	authHandler := NewAuthHandler(db, cfg.Server.JWTSecret)
	// 确定对外 gRPC 地址：优先使用 external_addr 配置，回退到 localhost:grpc_port
	grpcAddr := cfg.Server.ExternalAddr
	if grpcAddr == "" {
		grpcAddr = fmt.Sprintf("localhost:%d", cfg.Server.GRPCPort)
	}
	nodeHandler := NewNodeHandler(db, grpcAddr)
	userHandler := NewUserHandler(db)
	planHandler := NewPlanHandler(db)
	inboundHandler := NewInboundHandler(db)
	statsHandler := NewStatsHandler(db)
	subHandler := NewSubscriptionHandler(db, cfg.Server.JWTSecret)
	upgradeHandler := NewUpgradeHandler(cfg, logger)

	// 公开路由
	public := r.Group("/api")
	{
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/refresh", authHandler.RefreshToken)
		public.GET("/subscription/:token", subHandler.GetSubscription)
	}

	// Agent 升级路由（需要 node token 或 JWT 认证）
	agent := r.Group("/api/agent")
	agent.Use(NodeTokenAuthMiddleware(cfg.AgentRelease.CurrentVersion))
	{
		agent.GET("/version", upgradeHandler.GetVersion)
		agent.GET("/download", upgradeHandler.DownloadBinary)
	}

	// 需认证路由
	protected := r.Group("/api")
	protected.Use(JWTAuthMiddleware(cfg.Server.JWTSecret))
	{
		// 节点
		protected.GET("/nodes", nodeHandler.List)
		protected.POST("/nodes", nodeHandler.Create)
		protected.PUT("/nodes/:id", nodeHandler.Update)
		protected.DELETE("/nodes/:id", nodeHandler.Delete)
		protected.GET("/nodes/:id/status", nodeHandler.GetStatus)

		// 用户
		protected.GET("/users", userHandler.List)
		protected.POST("/users", userHandler.Create)
		protected.PUT("/users/:id", userHandler.Update)
		protected.DELETE("/users/:id", userHandler.Delete)
		protected.GET("/users/:id/traffic", userHandler.GetTraffic)

		// 套餐
		protected.GET("/plans", planHandler.List)
		protected.POST("/plans", planHandler.Create)
		protected.PUT("/plans/:id", planHandler.Update)
		protected.DELETE("/plans/:id", planHandler.Delete)

		// 入站
		protected.GET("/inbounds", inboundHandler.List)
		protected.POST("/inbounds", inboundHandler.Create)
		protected.PUT("/inbounds/:id", inboundHandler.Update)
		protected.DELETE("/inbounds/:id", inboundHandler.Delete)

		// 统计
		protected.GET("/stats/overview", statsHandler.Overview)
		protected.GET("/stats/traffic", statsHandler.Traffic)

		// Agent 升级管理（JWT 认证，管理端查看）
		protected.GET("/agent/version", upgradeHandler.GetVersion)
	}

	return r
}
