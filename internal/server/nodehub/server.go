package nodehub

import (
	"crypto/tls"

	pb "github.com/shangui999/nexus-xray/internal/common/proto/nodehub/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewGRPCServer 创建 gRPC server 实例（不监听，由 main 统一管理）
// 如果 tlsConfig 为 nil，则不启用 TLS（适用于 h2c 明文模式）
func NewGRPCServer(hub *Hub, tlsConfig *tls.Config) *grpc.Server {
	var opts []grpc.ServerOption
	if tlsConfig != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsConfig)))
	}
	server := grpc.NewServer(opts...)

	// 注册 NodeAgentService
	pb.RegisterNodeAgentServiceServer(server, hub)

	return server
}
