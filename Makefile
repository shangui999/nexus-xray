.PHONY: build-server build-agent proto frontend docker-build clean dev

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# 编译 Server 二进制
build-server:
	go build -o bin/server ./cmd/server

# 编译 Agent 二进制（带版本注入）
build-agent:
	go build -ldflags="-w -s -X github.com/shangui999/nexus-xray/internal/agent/updater.Version=$(VERSION)" -o bin/agent ./cmd/agent

# 生成 protobuf Go 代码
proto:
	protoc --go_out=. --go_opt=module=github.com/shangui999/nexus-xray \
		--go-grpc_out=. --go-grpc_opt=module=github.com/shangui999/nexus-xray \
		proto/nodehub/v1/hub.proto

# 构建前端
frontend:
	cd web && npm install && npm run build

# 构建 Docker 镜像
docker-build:
	docker build -f Dockerfile.server -t xray-manager-server .
	docker build -f Dockerfile.agent -t xray-manager-agent .

# 清理构建产物
clean:
	rm -rf bin/

# 启动生产环境
dev:
	docker-compose up -d

# 启动开发环境（仅数据库）
dev-db:
	docker-compose -f docker-compose.dev.yml up -d
