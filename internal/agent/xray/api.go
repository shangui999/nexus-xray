package xray

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.uber.org/zap"
)

const (
	// XrayStatsServiceMethod 是 Xray Stats gRPC 服务方法路径
	xrayStatsQueryStatsMethod = "/xray.app.stats.command.StatsService/QueryStats"
	xrayStatsGetStatsMethod   = "/xray.app.stats.command.StatsService/GetStats"
)

// rawBytesCodec 自定义 gRPC 编解码器，直接传递原始 protobuf 字节
// 使用 Name "proto" 以确保 content-type 为 application/grpc+proto，
// 与 Xray 的 gRPC 服务兼容
type rawBytesCodec struct{}

func (rawBytesCodec) Marshal(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("rawBytesCodec: expected []byte, got %T", v)
	}
	return b, nil
}

func (rawBytesCodec) Unmarshal(data []byte, v any) error {
	pb, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("rawBytesCodec: expected *[]byte, got %T", v)
	}
	*pb = data
	return nil
}

func (rawBytesCodec) Name() string {
	return "proto"
}

// rawBytesCodec 不需要通过 encoding.RegisterCodec 注册，
// 直接通过 grpc.ForceCodec 使用即可。

// APIClient Xray Stats gRPC 客户端
type APIClient struct {
	addr   string
	conn   *grpc.ClientConn
	mu     sync.Mutex
	logger *zap.Logger
}

// NewAPIClient 创建 Xray Stats API 客户端
func NewAPIClient(addr string, logger *zap.Logger) *APIClient {
	return &APIClient{
		addr:   addr,
		logger: logger,
	}
}

// getConn 获取或创建 gRPC 连接
func (c *APIClient) getConn() (*grpc.ClientConn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn, nil
	}

	conn, err := grpc.NewClient(c.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to xray api: %w", err)
	}
	c.conn = conn
	c.logger.Info("connected to xray stats api", zap.String("addr", c.addr))
	return conn, nil
}

// Close 关闭 gRPC 连接
func (c *APIClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// QueryAllUserTraffic 查询所有用户流量（自上次查询后的增量）
// reset=true 表示查询后重置计数器
func (c *APIClient) QueryAllUserTraffic(reset bool) ([]UserTraffic, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}

	// 构造请求 — 空模式匹配所有统计项
	req := &QueryStatsRequest{
		Pattern: "",
		Reset:   reset,
	}
	reqBytes := marshalQueryStatsRequest(req)

	// 调用 Xray Stats gRPC API
	var respBytes []byte
	err = conn.Invoke(context.Background(), xrayStatsQueryStatsMethod,
		reqBytes, &respBytes, grpc.ForceCodec(rawBytesCodec{}))
	if err != nil {
		return nil, fmt.Errorf("query stats rpc: %w", err)
	}

	// 解码响应
	resp, err := unmarshalQueryStatsResponse(respBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal query stats response: %w", err)
	}

	return parseUserTraffic(resp.Stats), nil
}

// QueryUserTraffic 查询单个用户流量
func (c *APIClient) QueryUserTraffic(email string, reset bool) (*UserTraffic, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}

	// 按用户 email 查询
	req := &QueryStatsRequest{
		Pattern: fmt.Sprintf("user>>>%s>>>traffic>>>", email),
		Reset:   reset,
	}
	reqBytes := marshalQueryStatsRequest(req)

	var respBytes []byte
	err = conn.Invoke(context.Background(), xrayStatsQueryStatsMethod,
		reqBytes, &respBytes, grpc.ForceCodec(rawBytesCodec{}))
	if err != nil {
		return nil, fmt.Errorf("query stats rpc: %w", err)
	}

	resp, err := unmarshalQueryStatsResponse(respBytes)
	if err != nil {
		return nil, fmt.Errorf("unmarshal query stats response: %w", err)
	}

	traffics := parseUserTraffic(resp.Stats)
	for i := range traffics {
		if traffics[i].Email == email {
			return &traffics[i], nil
		}
	}

	// 无流量数据
	return &UserTraffic{Email: email}, nil
}

// parseUserTraffic 解析 stat name 格式:
// "user>>>email>>>traffic>>>uplink" 和 "user>>>email>>>traffic>>>downlink"
func parseUserTraffic(stats []*Stat) []UserTraffic {
	trafficMap := make(map[string]*UserTraffic)

	for _, s := range stats {
		parts := strings.Split(s.Name, ">>>")
		if len(parts) != 4 || parts[0] != "user" || parts[2] != "traffic" {
			continue
		}

		email := parts[1]
		direction := parts[3]

		if trafficMap[email] == nil {
			trafficMap[email] = &UserTraffic{Email: email}
		}

		switch direction {
		case "uplink":
			trafficMap[email].Upload = s.Value
		case "downlink":
			trafficMap[email].Download = s.Value
		}
	}

	result := make([]UserTraffic, 0, len(trafficMap))
	for _, t := range trafficMap {
		result = append(result, *t)
	}
	return result
}
