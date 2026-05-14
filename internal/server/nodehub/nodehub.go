package nodehub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pb "github.com/shangui999/nexus-xray/internal/common/proto/nodehub/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"go.uber.org/zap"
)

// defaultHeartbeatTimeout 默认心跳超时时间
const defaultHeartbeatTimeout = 30 * time.Second

// sendChannelSize 发送 channel 缓冲大小
const sendChannelSize = 256

// Session 表示一个连接的 Agent 会话
type Session struct {
	NodeID string
	Stream grpc.BidiStreamingServer[pb.NodeMessage, pb.ServerMessage]
	SendCh chan *pb.ServerMessage
	cancel context.CancelFunc
}

// Hub 管理所有 Agent 连接
type Hub struct {
	pb.UnimplementedNodeAgentServiceServer

	mu              sync.RWMutex
	sessions        map[string]*Session // nodeID -> Session
	pendingRequests map[string]chan *pb.NodeMessage // requestID -> response channel
	logger          *zap.Logger
	heartbeatTimeout time.Duration
}

// NewHub 创建 NodeHub 实例
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		sessions:        make(map[string]*Session),
		pendingRequests: make(map[string]chan *pb.NodeMessage),
		logger:          logger,
		heartbeatTimeout: defaultHeartbeatTimeout,
	}
}

// SetHeartbeatTimeout 设置心跳超时时间
func (h *Hub) SetHeartbeatTimeout(d time.Duration) {
	h.heartbeatTimeout = d
}

// Session 实现 gRPC 双向流 RPC
func (h *Hub) Session(stream grpc.BidiStreamingServer[pb.NodeMessage, pb.ServerMessage]) error {
	ctx := stream.Context()

	// 1. 从 TLS peer cert 的 CN 提取 nodeID
	nodeID, err := extractNodeID(ctx)
	if err != nil {
		h.logger.Error("failed to extract node ID from TLS cert", zap.Error(err))
		return fmt.Errorf("extract node ID: %w", err)
	}

	h.logger.Info("agent connected", zap.String("node_id", nodeID))

	// 2. 创建 session 并注册到 sessions map
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	sess := &Session{
		NodeID: nodeID,
		Stream: stream,
		SendCh: make(chan *pb.ServerMessage, sendChannelSize),
		cancel: sessionCancel,
	}

	h.registerSession(sess)
	defer h.unregisterSession(nodeID)

	// 3. 启动写 goroutine（从 sendCh 取消息发送）
	go h.sendLoop(sessionCtx, sess)

	// 4. 心跳超时检测
	heartbeatTimer := time.NewTimer(h.heartbeatTimeout)
	defer heartbeatTimer.Stop()

	// 5. 读循环：处理收到的 NodeMessage
	msgCh := make(chan *pb.NodeMessage, sendChannelSize)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				close(msgCh)
				return
			}
			msgCh <- msg
		}
	}()

	for {
		select {
		case <-sessionCtx.Done():
			h.logger.Info("agent disconnected", zap.String("node_id", nodeID))
			return sessionCtx.Err()

		case msg, ok := <-msgCh:
			if !ok {
				h.logger.Info("agent stream closed", zap.String("node_id", nodeID))
				return nil
			}
			// 重置心跳计时器
			if !heartbeatTimer.Stop() {
				select {
				case <-heartbeatTimer.C:
				default:
				}
			}
			heartbeatTimer.Reset(h.heartbeatTimeout)

			h.handleNodeMessage(sess, msg)

		case <-heartbeatTimer.C:
			h.logger.Warn("agent heartbeat timeout", zap.String("node_id", nodeID))
			return fmt.Errorf("heartbeat timeout for node %s", nodeID)
		}
	}
}

// handleNodeMessage 处理收到的 NodeMessage
func (h *Hub) handleNodeMessage(sess *Session, msg *pb.NodeMessage) {
	switch msg.Event {
	case "hello":
		h.logger.Info("received hello from agent",
			zap.String("node_id", sess.NodeID),
			zap.String("id", msg.Id),
		)

	case "usage_push":
		h.logger.Debug("received usage push",
			zap.String("node_id", sess.NodeID),
			zap.Int64("seq", msg.Seq),
		)
		// 回 ack
		ackBody, _ := json.Marshal(map[string]int64{"seq": msg.Seq})
		sess.SendCh <- &pb.ServerMessage{
			Id:     msg.Id,
			Method: "ack",
			Body:   ackBody,
		}

	case "log":
		h.logger.Debug("received log from agent",
			zap.String("node_id", sess.NodeID),
		)

	default:
		h.logger.Debug("received unknown event",
			zap.String("node_id", sess.NodeID),
			zap.String("event", msg.Event),
		)
	}

	// 如果是请求-响应模式，将响应发送到 pending channel
	if msg.Id != "" {
		h.mu.RLock()
		if ch, ok := h.pendingRequests[msg.Id]; ok {
			ch <- msg
		}
		h.mu.RUnlock()
	}
}

// sendLoop 负责从 sendCh 取消息并发送到 stream
func (h *Hub) sendLoop(ctx context.Context, sess *Session) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sess.SendCh:
			if !ok {
				return
			}
			if err := sess.Stream.Send(msg); err != nil {
				h.logger.Error("failed to send message to agent",
					zap.String("node_id", sess.NodeID),
					zap.Error(err),
				)
				return
			}
		}
	}
}

// registerSession 注册 session
func (h *Hub) registerSession(sess *Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 如果旧 session 存在，关闭它
	if old, ok := h.sessions[sess.NodeID]; ok {
		old.cancel()
		close(old.SendCh)
	}

	h.sessions[sess.NodeID] = sess
	h.logger.Info("session registered", zap.String("node_id", sess.NodeID))
}

// unregisterSession 注销 session
func (h *Hub) unregisterSession(nodeID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if sess, ok := h.sessions[nodeID]; ok {
		sess.cancel()
		close(sess.SendCh)
		delete(h.sessions, nodeID)
		h.logger.Info("session unregistered", zap.String("node_id", nodeID))
	}
}

// SendToNode 向指定节点发送消息并等待响应
func (h *Hub) SendToNode(ctx context.Context, nodeID string, method string, body []byte) ([]byte, error) {
	h.mu.RLock()
	sess, ok := h.sessions[nodeID]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("node %s is not online", nodeID)
	}

	requestID := uuid.New().String()
	respCh := make(chan *pb.NodeMessage, 1)

	// 注册 pending request
	h.mu.Lock()
	h.pendingRequests[requestID] = respCh
	h.mu.Unlock()

	// 清理 pending request
	defer func() {
		h.mu.Lock()
		delete(h.pendingRequests, requestID)
		h.mu.Unlock()
	}()

	// 发送请求
	msg := &pb.ServerMessage{
		Id:     requestID,
		Method: method,
		Body:   body,
	}

	select {
	case sess.SendCh <- msg:
	default:
		return nil, fmt.Errorf("send channel full for node %s", nodeID)
	}

	// 等待响应
	select {
	case resp := <-respCh:
		if !resp.Ok {
			return nil, fmt.Errorf("node %s returned error: %s", nodeID, resp.Error)
		}
		return resp.Body, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request to node %s cancelled: %w", nodeID, ctx.Err())
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request to node %s timed out", nodeID)
	}
}

// PushConfig 向节点推送 Xray 配置
func (h *Hub) PushConfig(nodeID string, config []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := h.SendToNode(ctx, nodeID, "UpdateConfig", config)
	return err
}

// GetOnlineNodes 获取在线节点列表
func (h *Hub) GetOnlineNodes() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	nodes := make([]string, 0, len(h.sessions))
	for nodeID := range h.sessions {
		nodes = append(nodes, nodeID)
	}
	return nodes
}

// IsNodeOnline 检查节点是否在线
func (h *Hub) IsNodeOnline(nodeID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.sessions[nodeID]
	return ok
}

// extractNodeID 从 TLS peer certificate 的 CN 提取 nodeID
func extractNodeID(ctx context.Context) (string, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "", fmt.Errorf("failed to get peer from context")
	}

	tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return "", fmt.Errorf("no TLS info in peer auth info")
	}

	if len(tlsInfo.State.PeerCertificates) == 0 {
		return "", fmt.Errorf("no peer certificates found")
	}

	cn := tlsInfo.State.PeerCertificates[0].Subject.CommonName
	if cn == "" {
		return "", fmt.Errorf("empty CommonName in peer certificate")
	}

	return cn, nil
}
