package stats

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/shangui999/nexus-xray/internal/agent/xray"
	"go.uber.org/zap"
)

// Collector 周期性采集 Xray 流量并上报
type Collector struct {
	xrayAPI  *xray.APIClient
	interval time.Duration
	sendFunc func(traffics []xray.UserTraffic) error
	logger   *zap.Logger
	stopCh   chan struct{}
	mu       sync.Mutex
	running  bool
	seq      int64
}

// NewCollector 创建采集器
func NewCollector(
	xrayAPI *xray.APIClient,
	interval time.Duration,
	sendFunc func([]xray.UserTraffic) error,
	logger *zap.Logger,
) *Collector {
	return &Collector{
		xrayAPI:  xrayAPI,
		interval: interval,
		sendFunc: sendFunc,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动周期采集
// 每 interval 调用 xrayAPI.QueryAllUserTraffic(reset=true)
// 将结果通过 sendFunc 发送给 Server
func (c *Collector) Start(ctx context.Context) {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go c.collectLoop(ctx)
}

// collectLoop 采集循环
func (c *Collector) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info("stats collector started", zap.Duration("interval", c.interval))

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("stats collector stopped by context")
			c.setRunning(false)
			return
		case <-c.stopCh:
			c.logger.Info("stats collector stopped")
			c.setRunning(false)
			return
		case <-ticker.C:
			c.collect()
		}
	}
}

// collect 执行一次采集
func (c *Collector) collect() {
	traffics, err := c.xrayAPI.QueryAllUserTraffic(true)
	if err != nil {
		c.logger.Error("failed to query user traffic", zap.Error(err))
		return
	}

	// 没有流量数据则跳过上报
	if len(traffics) == 0 {
		c.logger.Debug("no traffic data to report")
		return
	}

	c.logger.Debug("collected traffic data",
		zap.Int("user_count", len(traffics)),
	)

	if err := c.sendFunc(traffics); err != nil {
		c.logger.Error("failed to send traffic data", zap.Error(err))
		return
	}

	c.seq++
}

// Stop 停止采集
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	close(c.stopCh)
	c.running = false
}

func (c *Collector) setRunning(running bool) {
	c.mu.Lock()
	c.running = running
	c.mu.Unlock()
}

// EncodeTrafficReport 将流量数据编码为 JSON 用于上报
func EncodeTrafficReport(traffics []xray.UserTraffic) ([]byte, error) {
	return json.Marshal(traffics)
}
