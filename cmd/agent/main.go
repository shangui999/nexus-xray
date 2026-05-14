package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shangui999/nexus-xray/internal/agent/connector"
	"github.com/shangui999/nexus-xray/internal/agent/stats"
	"github.com/shangui999/nexus-xray/internal/agent/updater"
	"github.com/shangui999/nexus-xray/internal/agent/xray"
	"github.com/shangui999/nexus-xray/internal/common/config"
	"go.uber.org/zap"
)

func main() {
	// 1. 加载配置
	cfg, err := config.LoadAgentConfig("configs/agent.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化 Logger
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

	logger.Info("xray-manager agent starting",
		zap.String("server_addr", cfg.Agent.ServerAddr),
		zap.String("node_id", cfg.Agent.NodeID),
	)

	// 3. 创建 XrayManager
	xrayManager := xray.NewManager("xray", "/tmp/xray-config.json", "127.0.0.1:10085", logger)

	// 4. 创建 Connector（注册 handler）
	conn := connector.NewConnector(
		cfg.Agent.ServerAddr,
		cfg.Agent.NodeID,
		cfg.Agent.CertDir,
		logger,
	)

	// 5. 注册 "xray.UpdateConfig" handler → 调用 manager.Reload
	conn.RegisterHandler("xray.UpdateConfig", func(method string, body []byte) ([]byte, error) {
		var newConfig xray.XrayConfig
		if err := json.Unmarshal(body, &newConfig); err != nil {
			return nil, fmt.Errorf("unmarshal xray config: %w", err)
		}
		if err := xrayManager.Reload(&newConfig); err != nil {
			return nil, fmt.Errorf("reload xray config: %w", err)
		}
		logger.Info("xray config updated via server command")
		return []byte(`{"ok":true}`), nil
	})

	// 6. 注册 "xray.AddUser" handler → 向配置添加用户
	conn.RegisterHandler("xray.AddUser", func(method string, body []byte) ([]byte, error) {
		var req struct {
			InboundTag string         `json:"inbound_tag"`
			User       xray.ProxyUser `json:"user"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, fmt.Errorf("unmarshal add user request: %w", err)
		}
		if err := xrayManager.AddUserToInbound(req.InboundTag, req.User); err != nil {
			return nil, fmt.Errorf("add user: %w", err)
		}
		logger.Info("user added",
			zap.String("inbound_tag", req.InboundTag),
			zap.String("email", req.User.Email),
		)
		return []byte(`{"ok":true}`), nil
	})

	// 7. 注册 "xray.RemoveUser" handler → 从配置移除用户
	conn.RegisterHandler("xray.RemoveUser", func(method string, body []byte) ([]byte, error) {
		var req struct {
			InboundTag string `json:"inbound_tag"`
			Email      string `json:"email"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			return nil, fmt.Errorf("unmarshal remove user request: %w", err)
		}
		if err := xrayManager.RemoveUserFromInbound(req.InboundTag, req.Email); err != nil {
			return nil, fmt.Errorf("remove user: %w", err)
		}
		logger.Info("user removed",
			zap.String("inbound_tag", req.InboundTag),
			zap.String("email", req.Email),
		)
		return []byte(`{"ok":true}`), nil
	})

	// 8. 创建 Stats Collector（sendFunc 使用 connector.SendEvent）
	var seq int64
	xrayAPIClient := xray.NewAPIClient("127.0.0.1:10085", logger)

	collector := stats.NewCollector(
		xrayAPIClient,
		cfg.Agent.StatsInterval,
		func(traffics []xray.UserTraffic) error {
			body, err := stats.EncodeTrafficReport(traffics)
			if err != nil {
				return fmt.Errorf("encode traffic report: %w", err)
			}
			seq++
			if err := conn.SendEvent("usage_push", seq, body); err != nil {
				return fmt.Errorf("send usage_push event: %w", err)
			}
			logger.Debug("traffic report sent", zap.Int64("seq", seq))
			return nil
		},
		logger,
	)

	// 9. 启动 Xray（使用默认配置）
	defaultConfig := xray.GenerateDefaultConfig()
	if err := xrayManager.Start(defaultConfig); err != nil {
		logger.Warn("failed to start xray (may not be installed)", zap.Error(err))
	}

	// 10. 启动 Updater（自动升级）
	updateInterval := cfg.Agent.UpdateInterval
	if updateInterval == 0 {
		updateInterval = 1 * time.Hour
	}
	upd := updater.NewUpdater(
		cfg.Agent.HTTPAddr,
		cfg.Agent.NodeToken,
		updater.Version,
		updateInterval,
		logger,
	)
	updaterCtx, updaterCancel := context.WithCancel(context.Background())
	go upd.Start(updaterCtx)
	defer updaterCancel()
	defer upd.Stop()

	logger.Info("updater started",
		zap.String("current_version", updater.Version),
		zap.Duration("check_interval", updateInterval),
	)

	// 11. 启动 Connector（自动重连）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := conn.Connect(ctx); err != nil {
			logger.Error("failed to connect to server", zap.Error(err))
			go conn.ReconnectLoop(ctx)
		}
	}()

	// 12. 启动 Collector
	collector.Start(ctx)

	logger.Info("agent fully started",
		zap.String("version", updater.Version),
	)

	// 13. 等待信号 graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("received shutdown signal", zap.String("signal", sig.String()))

	// 按顺序优雅关闭
	collector.Stop()
	if err := xrayManager.Stop(); err != nil {
		logger.Error("failed to stop xray", zap.Error(err))
	}
	if err := xrayAPIClient.Close(); err != nil {
		logger.Error("failed to close xray api client", zap.Error(err))
	}
	conn.Stop()
	cancel()

	logger.Info("agent shutdown complete")
}
