package nodehub

import (
	"crypto/tls"
	"fmt"
	"net"

	pb "github.com/shangui999/nexus-xray/internal/common/proto/nodehub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"go.uber.org/zap"
)

// StartGRPCServer 启动带 mTLS 的 gRPC 服务
func StartGRPCServer(port int, hub *Hub, tlsConfig *tls.Config, logger *zap.Logger) error {
	// 1. 创建 TLS credentials (require client cert)
	creds := credentials.NewTLS(tlsConfig)

	// 2. grpc.NewServer with credentials
	server := grpc.NewServer(grpc.Creds(creds))

	// 3. 注册 NodeAgentService
	pb.RegisterNodeAgentServiceServer(server, hub)

	// 4. 监听端口并 Serve
	addr := fmt.Sprintf(":%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	logger.Info("gRPC server started", zap.Int("port", port))

	if err := server.Serve(listener); err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	return nil
}
