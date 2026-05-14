package connector

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	pb "github.com/shangui999/nexus-xray/internal/common/proto/nodehub/v1"
	"github.com/shangui999/nexus-xray/internal/common/cert"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"go.uber.org/zap"
)

// Handler 处理 Server 下发的命令
type Handler func(method string, body []byte) ([]byte, error)

// Connector Agent 与 Server 的 gRPC 连接管理
type Connector struct {
	serverAddr string
	nodeID     string
	certDir    string
	client     pb.NodeAgentServiceClient
	conn       *grpc.ClientConn
	stream     grpc.BidiStreamingClient[pb.NodeMessage, pb.ServerMessage]
	handlers   map[string]Handler // method -> handler
	logger     *zap.Logger
	stopCh     chan struct{}
	sendCh     chan *pb.NodeMessage
	mu         sync.RWMutex
	connected  bool
}

// NewConnector 创建连接器
func NewConnector(serverAddr, nodeID, certDir string, logger *zap.Logger) *Connector {
	return &Connector{
		serverAddr: serverAddr,
		nodeID:     nodeID,
		certDir:    certDir,
		handlers:   make(map[string]Handler),
		logger:     logger,
		stopCh:     make(chan struct{}),
		sendCh:     make(chan *pb.NodeMessage, 256),
	}
}

// RegisterHandler 注册方法处理器
func (c *Connector) RegisterHandler(method string, handler Handler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[method] = handler
}

// Connect 建立 gRPC 连接（mTLS）并启动 Session
func (c *Connector) Connect(ctx context.Context) error {
	// 1. 加载客户端证书并创建 TLS credentials
	var creds credentials.TransportCredentials

	caCertPath := filepath.Join(c.certDir, "ca.crt")
	clientCertPath := filepath.Join(c.certDir, "client.crt")
	clientKeyPath := filepath.Join(c.certDir, "client.key")

	tlsConfig, err := cert.LoadClientTLSConfig(caCertPath, clientCertPath, clientKeyPath)
	if err != nil {
		c.logger.Warn("failed to load TLS config, falling back to insecure", zap.Error(err))
		creds = insecure.NewCredentials()
	} else {
		creds = credentials.NewTLS(tlsConfig)
	}

	// 2. grpc.Dial with credentials
	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return fmt.Errorf("dial server: %w", err)
	}
	c.conn = conn

	// 3. 创建客户端
	c.client = pb.NewNodeAgentServiceClient(conn)

	// 4. 调用 Session RPC 建立双向流
	stream, err := c.client.Session(ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("open session stream: %w", err)
	}
	c.stream = stream

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	c.logger.Info("connected to server", zap.String("addr", c.serverAddr))

	// 5. 发送 hello 事件
	helloMsg := &pb.NodeMessage{
		Id:    uuid.New().String(),
		Event: "hello",
		Ok:    true,
		Body:  []byte(fmt.Sprintf(`{"node_id":"%s"}`, c.nodeID)),
	}
	if err := stream.Send(helloMsg); err != nil {
		c.logger.Error("failed to send hello", zap.Error(err))
		return fmt.Errorf("send hello: %w", err)
	}

	// 6. 启动读循环（处理 ServerMessage，调用对应 handler，回复 NodeMessage）
	go c.readLoop(ctx)

	// 7. 启动写循环（从 sendCh 取消息发送）
	go c.writeLoop(ctx)

	return nil
}

// readLoop 读取 Server 下发的消息并处理
func (c *Connector) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		msg, err := c.stream.Recv()
		if err != nil {
			c.logger.Error("failed to receive from server", zap.Error(err))
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return
		}

		c.handleServerMessage(ctx, msg)
	}
}

// handleServerMessage 处理 Server 下发的消息
func (c *Connector) handleServerMessage(ctx context.Context, msg *pb.ServerMessage) {
	c.logger.Debug("received server message",
		zap.String("method", msg.Method),
		zap.String("id", msg.Id),
	)

	// ack 消息无需响应
	if msg.Method == "ack" {
		return
	}

	// 查找 handler
	c.mu.RLock()
	handler, ok := c.handlers[msg.Method]
	c.mu.RUnlock()

	respMsg := &pb.NodeMessage{
		Id: msg.Id,
	}

	if !ok {
		c.logger.Warn("no handler for method", zap.String("method", msg.Method))
		respMsg.Ok = false
		respMsg.Error = fmt.Sprintf("no handler for method: %s", msg.Method)
	} else {
		result, err := handler(msg.Method, msg.Body)
		if err != nil {
			respMsg.Ok = false
			respMsg.Error = err.Error()
		} else {
			respMsg.Ok = true
			respMsg.Body = result
		}
	}

	// 回复响应
	c.sendCh <- respMsg
}

// writeLoop 从 sendCh 取消息发送
func (c *Connector) writeLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case msg, ok := <-c.sendCh:
			if !ok {
				return
			}
			if err := c.stream.Send(msg); err != nil {
				c.logger.Error("failed to send message to server", zap.Error(err))
				return
			}
		}
	}
}

// SendEvent 发送事件到 Server（如 usage_push）
func (c *Connector) SendEvent(event string, seq int64, body []byte) error {
	c.mu.RLock()
	connected := c.connected
	c.mu.RUnlock()

	if !connected {
		return fmt.Errorf("not connected to server")
	}

	msg := &pb.NodeMessage{
		Id:    uuid.New().String(),
		Event: event,
		Ok:    true,
		Body:  body,
		Seq:   seq,
	}

	select {
	case c.sendCh <- msg:
		return nil
	default:
		return fmt.Errorf("send channel full")
	}
}

// ReconnectLoop 断线重连（指数退避）
// 初始 1s, 最大 60s, 每次翻倍
func (c *Connector) ReconnectLoop(ctx context.Context) {
	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		// 尝试连接
		err := c.Connect(ctx)
		if err == nil {
			// 连接成功，重置退避时间
			backoff = 1 * time.Second
			c.logger.Info("reconnected to server")

			// 等待断线
			<-ctx.Done()
			return
		}

		c.logger.Error("failed to connect to server, retrying",
			zap.Error(err),
			zap.Duration("backoff", backoff),
		)

		// 等待退避时间
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-time.After(backoff):
			// 翻倍退避
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// Stop 优雅关闭
func (c *Connector) Stop() {
	close(c.stopCh)

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connected = false
	c.mu.Unlock()

	c.logger.Info("connector stopped")
}
