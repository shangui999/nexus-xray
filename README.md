# XrayManager

Server-Agent 架构的分布式 VPS Xray 代理管理平台，支持多节点集中管控、用户订阅分发与实时流量统计。

![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-MIT-blue.svg)
![Release](https://img.shields.io/badge/Release-v0.1.0-orange)

---

## ✨ 特性

- 🌐 **Server-Agent 分布式架构** — 集中管理多台 VPS 节点，Agent 轻量部署
- 🔒 **gRPC + mTLS 安全通信** — 双向 TLS 认证，确保 Server-Agent 链路安全
- 🔀 **HTTP/gRPC 同端口分流** — 基于 h2c 的单端口设计，HTTP API 与 gRPC 共用 8080 端口
- 📡 **多节点集中管理** — 统一管控所有节点状态、配置与生命周期
- 👥 **完整用户订阅系统** — 自动生成订阅链接，兼容主流客户端
- 🚀 **VLESS + Reality 协议支持** — 前沿协议组合，抗检测能力强
- 🔗 **IPv4/IPv6 双栈订阅** — 同时生成双栈节点链接，适配不同网络环境
- ✏️ **自定义订阅地址** — 每个节点可设置独立订阅域名，灵活应对 CDN/NAT 场景
- 📊 **流量配额与有效期管理** — 套餐制用户管理，支持流量倍率与连接数限制
- 📈 **实时流量统计与图表** — 基于 ECharts 的可视化流量监控面板
- 🔄 **Agent 自动升级** — 基于 GitHub Releases 自动检测并拉取新版本
- ☁️ **Cloudflare Tunnel 支持** — 通过 CF Tunnel 隐藏源站，安全暴露服务
- 🐳 **Docker 一键部署** — 提供完整的 Docker Compose 编排，开箱即用
- 🎨 **Vue 3 现代化管理面板** — Element Plus + ECharts 构建的专业管理界面

---

## 🏗️ 架构

```
                          ┌──────────────────┐
                          │  Cloudflare CDN  │
                          │  (Tunnel / DNS)  │
                          └────────┬─────────┘
                                   │
                          ┌────────▼─────────┐
                          │     Server       │
                          │  ┌────────────┐  │
                          │  │  HTTP API  │  │  ← Vue 3 管理面板
                          │  │  (Gin)     │  │  :8080
                          │  ├────────────┤  │
                          │  │  gRPC Hub  │  │  ← Agent 双向流
                          │  │  (NodeHub) │  │  :8080
                          │  ├────────────┤  │
                          │  │ PostgreSQL │  │
                          │  └────────────┘  │
                          └────────┬─────────┘
                                   │ gRPC (h2c)
                    ┌──────────────┼──────────────┐
                    │              │              │
             ┌──────▼──────┐┌─────▼───────┐┌─────▼───────┐
             │   Agent 1   ││   Agent 2   ││   Agent N   │
             │ ┌─────────┐ ││ ┌─────────┐ ││ ┌─────────┐ │
             │ │  Xray   │ ││ │  Xray   │ ││ │  Xray   │ │
             │ │(in-proc)│ ││ │(in-proc)│ ││ │(in-proc)│ │
             │ └─────────┘ ││ └─────────┘ ││ └─────────┘ │
             │    VPS 1    ││    VPS 2    ││    VPS N    │
             └─────────────┘└─────────────┘└─────────────┘
```

**通信流程：**
- Server 通过 HTTP API 对外提供管理接口与订阅服务
- Server 使用单端口设计（HTTP API + gRPC Agent 通信共用 8080 端口），基于 h2c 实现 HTTP/1.1 与 gRPC (HTTP/2) 同端口分流
- Agent 通过 gRPC 双向流（`NodeAgentService.Session`）与 Server 保持长连接
- Server 下发配置更新、重启 Xray、升级 Agent 等指令
- Agent 上报流量统计、Xray 状态等事件
- 全部通信经 mTLS 加密（生产环境），Agent 需持有效证书方可连接；开发模式支持 h2c 明文连接

---

## 🛠️ 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| **后端语言** | Go 1.22+ | 高性能编译型语言 |
| **Web 框架** | Gin | HTTP REST API |
| **RPC 框架** | gRPC + Protobuf | Server-Agent 双向流通信 |
| **安全认证** | mTLS / JWT / HMAC-SHA256 | 多层安全机制 |
| **数据库** | PostgreSQL 16 (生产) / SQLite (开发) | 通过 GORM ORM 访问 |
| **前端框架** | Vue 3 + TypeScript | Composition API |
| **UI 组件库** | Element Plus | 企业级组件库 |
| **状态管理** | Pinia | Vue 官方推荐 |
| **图表可视化** | ECharts + vue-echarts | 流量统计图表 |
| **构建工具** | Vite 8 | 极速前端构建 |
| **代理核心** | Xray-core (in-process) | 嵌入式集成，非子进程 |
| **容器化** | Docker + Docker Compose | 一键部署 |
| **隧道** | Cloudflare Tunnel | 源站隐藏与公网暴露 |

---

## 🚀 快速开始

### 环境要求

- Go 1.22+
- Node.js 20+
- PostgreSQL 16+
- Docker & Docker Compose（可选，推荐生产部署使用）

### 开发环境

```bash
# 克隆项目
git clone https://github.com/shangui999/nexus-xray.git
cd nexus-xray

# 启动开发数据库
make dev-db

# 运行后端
go run ./cmd/server

# 运行前端（另一终端）
cd web && npm install && npm run dev

# 访问 http://localhost:3000
# 默认管理员: admin / admin123
```

### 生产部署

#### 方式一：一键部署（推荐）

在服务器上执行：

```bash
curl -sSL https://raw.githubusercontent.com/shangui999/nexus-xray/main/scripts/install-server.sh | bash
```

脚本会自动完成：
1. 检测系统并安装 Docker / Docker Compose
2. 创建安装目录 `/opt/nexus-xray`
3. 下载 Docker Compose 配置与 Dockerfile
4. 生成随机密码和密钥（`.env` 文件）
5. 启动 PostgreSQL 和 Server 容器

部署完成后访问 `http://<服务器IP>:8080`，默认账号 `admin / admin123`。

#### 方式二：手动部署

##### 1. 准备环境配置

```bash
git clone https://github.com/shangui999/nexus-xray.git
cd nexus-xray
cp .env.example .env
# 编辑 .env，设置安全密码和密钥
```

`.env` 配置项：

| 变量 | 说明 | 示例 |
|------|------|------|
| `DB_PASSWORD` | PostgreSQL 密码 | `your-secure-password` |
| `JWT_SECRET` | JWT 签名密钥（≥32字符） | `your-jwt-secret-at-least-32-chars` |
| `SUBSCRIPTION_SECRET` | 订阅 HMAC 签名密钥 | `your-subscription-hmac-secret` |
| `CF_TUNNEL_TOKEN` | Cloudflare Tunnel Token | `your-cloudflare-tunnel-token` |

##### 2. 生成 mTLS 证书

```bash
bash scripts/generate-certs.sh ./data/certs
```

将生成的 `ca.crt`、`server.crt`、`server.key` 放置在 `./data/certs/` 目录，`ca.crt` 分发给 Agent。

##### 3. 配置 Cloudflare Tunnel（可选）

在 [Cloudflare Zero Trust](https://one.dash.cloudflare.com/) 控制台创建 Tunnel，获取 Token 填入 `.env`。

Tunnel 路由示例：
- `panel.yourdomain.com` → `http://server:8080`（管理面板 + Agent gRPC，需启用 `http2Origin: true`）

##### 4. 启动服务

```bash
docker compose up -d
```

服务启动后访问 `https://panel.yourdomain.com` 即可进入管理面板。

### 数据目录结构

所有持久化数据存储在 `./data/` 目录下：

```
data/
├── postgres/     # PostgreSQL 数据
├── certs/        # mTLS 证书
└── configs/      # Server 配置文件
```

---

## 🤖 Agent 部署

### 一键安装

在管理面板中添加节点后，系统会自动生成安装命令，复制到目标 VPS 执行即可：

```bash
curl -sSL https://raw.githubusercontent.com/shangui999/nexus-xray/main/scripts/install-agent.sh | bash -s -- \
  --server=your-server.example.com:8080 \
  --node-id=your-node-id \
  --token=your-enrollment-token
```

脚本会自动完成：
1. 检测系统架构（支持 amd64 / arm64 / armv7）
2. 安装 xray-core
3. 下载 Agent 二进制
4. 生成配置文件
5. 执行 Enrollment 获取 mTLS 证书
6. 创建 systemd 服务并启动

支持的安装参数：

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--server` | Server 地址（含端口，HTTP+gRPC 同端口） | — |
| `--node-id` | 节点 ID（必填） | — |
| `--token` | 注册令牌（必填） | — |
| `--install-dir` | 安装目录 | `/opt/xray-manager-agent` |
| `--xray-version` | Xray-core 版本 | `1.8.24` |

### 手动安装

```bash
# 1. 编译 Agent
go build -o agent ./cmd/agent

# 2. 编写配置
cat > agent.yaml <<EOF
agent:
  node_id: "your-node-id"
  server_addr: "your-server:8080"
  cert_dir: "/etc/xray-manager/certs"
  stats_interval: 10s

xray:
  log_level: warning

log:
  level: info
  format: json
EOF

# 3. 启动（首次运行需加 -enroll 完成证书注册）
./agent -config agent.yaml -enroll

# 4. 常规启动
./agent -config agent.yaml
```

### Agent 自动升级

Server 可通过 gRPC 下发 `UpgradeAgent` 指令，Agent 收到后会：

1. 从 GitHub Releases 拉取最新版本二进制
2. 替换自身并重启 systemd 服务
3. 上报升级结果

整个过程无需手动登录 VPS 操作。

---

## ⚙️ 配置说明

### Server 配置 (`configs/server.yaml`)

```yaml
server:
  http_port: 8080        # HTTP API + gRPC 统一端口
  jwt_secret: "change-me-in-production"  # JWT 签名密钥（生产环境务必修改）

database:
  host: localhost
  port: 5432
  user: xray_manager
  password: xray_manager_pass
  dbname: xray_manager
  sslmode: disable       # 生产环境建议启用

log:
  level: info            # 日志级别: debug/info/warn/error
  format: json           # 日志格式: json/text
```

### Agent 配置 (`configs/agent.yaml`)

```yaml
agent:
  node_id: ""                              # 注册时填写，由 Server 分配
  server_addr: "your-server.example.com:8080"  # Server 地址（HTTP + gRPC 同端口）
  cert_dir: "/etc/xray-manager/certs"      # mTLS 证书目录
  stats_interval: 10s                      # 流量上报间隔

xray:
  log_level: warning     # Xray 内部日志级别

log:
  level: info
  format: json
```

---

## 📡 API 文档

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 管理员登录，返回 JWT |
| POST | `/api/auth/refresh` | 刷新 JWT Token |

### 节点管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/nodes` | 节点列表 |
| POST | `/api/nodes` | 创建节点 |
| PUT | `/api/nodes/:id` | 更新节点 |
| DELETE | `/api/nodes/:id` | 删除节点 |
| GET | `/api/nodes/:id/status` | 获取节点实时状态 |

### 用户管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/users` | 用户列表 |
| POST | `/api/users` | 创建用户 |
| PUT | `/api/users/:id` | 更新用户 |
| DELETE | `/api/users/:id` | 删除用户 |
| GET | `/api/users/:id/traffic` | 查询用户流量记录 |

### 套餐管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/plans` | 套餐列表 |
| POST | `/api/plans` | 创建套餐 |
| PUT | `/api/plans/:id` | 更新套餐 |
| DELETE | `/api/plans/:id` | 删除套餐 |

### 入站管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/inbounds` | 入站列表 |
| POST | `/api/inbounds` | 创建入站 |
| PUT | `/api/inbounds/:id` | 更新入站 |
| DELETE | `/api/inbounds/:id` | 删除入站 |

### 统计

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/stats/overview` | 全局统计概览 |
| GET | `/api/stats/traffic` | 流量趋势数据 |

### 订阅

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/subscription/:token` | 获取用户订阅内容（公开接口） |

> 除登录/刷新/订阅外，所有接口均需在 Header 中携带 `Authorization: Bearer <jwt_token>`。

---

## 📋 订阅系统

### 订阅链接生成

1. 系统为每个用户生成唯一的订阅 Token（HMAC-SHA256 签名）
2. Token 格式：`base64url(userID.hmac(userID, secret))`
3. 客户端访问 `/api/subscription/:token` 获取 base64 编码的节点列表
4. 支持 VLESS 和 Trojan 协议链接生成

### 自定义订阅地址

每个节点可配置 `subscription_addr` 字段，优先使用该地址生成订阅链接，适用于：
- 节点使用 CDN 中转（域名与实际 IP 不同）
- NAT VPS（端口映射后的域名）
- 多入口统一域名场景

### IPv6 支持

节点配置 `ipv6_addr` 字段后，订阅中会额外生成一条 IPv6 链接（标记 `[IPv6]`），格式为 `vless://uuid@[ipv6]:port?...#NodeName [IPv6]`，支持双栈客户端自动选择。

---

## 🔐 安全

| 机制 | 应用场景 | 说明 |
|------|----------|------|
| **mTLS** | Server ↔ Agent | 双向 TLS 认证，Agent 必须持 CA 签发证书 |
| **JWT** | Web API | 管理面板接口认证，支持 Token 刷新 |
| **HMAC-SHA256** | 订阅链接 | 防篡改签名，确保订阅 Token 不可伪造 |
| **bcrypt** | 密码存储 | 管理员密码哈希，抗暴力破解 |
| **Enrollment Token** | Agent 注册 | 一次性注册令牌，防止未授权节点接入 |
| **CF Tunnel** | 源站保护 | Server 不直接暴露公网，通过 Cloudflare 隧道访问 |

---

## 🗺️ 路线图

### Phase 1 — 核心功能 ✅

- [x] Server-Agent gRPC 双向流通信
- [x] mTLS 节点认证与 Enrollment
- [x] 节点 CRUD 与状态监控
- [x] 用户与套餐管理
- [x] VLESS / Trojan 入站配置
- [x] 订阅链接生成（IPv4 + IPv6）
- [x] 自定义订阅地址
- [x] 流量统计与配额管理
- [x] Vue 3 管理面板
- [x] Docker Compose 部署
- [x] Cloudflare Tunnel 集成
- [x] Agent 一键安装脚本

### Phase 2 — 增强功能 🔧

- [ ] Agent 远程自动升级
- [ ] Shadowsocks 协议支持
- [ ] 用户在线设备数限制
- [ ] 流量预警与到期提醒
- [ ] 节点负载均衡与故障转移
- [ ] 多管理员与权限角色
- [ ] 操作审计日志

---

## 📁 项目结构

```
xray-manager/
├── cmd/
│   ├── agent/               # Agent 入口
│   └── server/              # Server 入口
├── configs/
│   ├── agent.yaml           # Agent 配置
│   ├── server.yaml          # Server 配置
│   └── cloudflared.yml      # CF Tunnel 配置示例
├── internal/
│   ├── agent/
│   │   ├── connector/       # gRPC 连接管理
│   │   ├── stats/           # 流量采集
│   │   └── xray/            # Xray 进程管理 & API
│   ├── common/
│   │   ├── cert/            # TLS 证书工具
│   │   ├── config/          # 配置加载
│   │   └── proto/           # 生成的 Protobuf 代码
│   ├── database/            # 数据库初始化
│   └── server/
│       ├── api/             # HTTP Handler & 路由
│       ├── model/           # 数据模型
│       ├── nodehub/         # gRPC NodeHub 服务
│       ├── service/         # 业务逻辑 & 定时任务
│       └── subscription/    # 订阅链接生成
├── proto/
│   └── nodehub/v1/          # Protobuf 定义
├── scripts/
│   ├── generate-certs.sh    # 生成 mTLS 证书
│   ├── install-agent.sh     # Agent 一键安装
│   └── install-server.sh   # Server 一键部署
├── web/                     # Vue 3 前端
│   ├── src/
│   │   ├── api/             # API 请求封装
│   │   ├── components/      # 公共组件
│   │   ├── router/          # 路由配置
│   │   ├── store/           # Pinia 状态管理
│   │   ├── types/           # TypeScript 类型
│   │   └── views/           # 页面组件
│   └── ...
├── docker-compose.yml       # 生产部署
├── docker-compose.dev.yml   # 开发数据库
├── Dockerfile.server        # Server 镜像
├── Dockerfile.agent         # Agent 镜像
├── Makefile                 # 构建命令
└── go.mod                   # Go 依赖
```

---

## 🧑‍💻 开发指南

### 常用命令

```bash
# 编译
make build-server          # 编译 Server → bin/server
make build-agent           # 编译 Agent  → bin/agent

# 前端
make frontend              # 安装依赖 & 构建前端

# Docker
make docker-build          # 构建镜像
make dev                   # 启动生产环境 (docker compose up)
make dev-db                # 仅启动开发数据库

# 清理
make clean                 # 清理构建产物
```

### 生成 Protobuf 代码

```bash
# 需要安装 protoc 及 Go 插件
make proto
```

等价于：

```bash
protoc --go_out=. --go_opt=module=github.com/shangui999/nexus-xray \
    --go-grpc_out=. --go-grpc_opt=module=github.com/shangui999/nexus-xray \
    proto/nodehub/v1/hub.proto
```

### 前端开发

```bash
cd web
npm install
npm run dev      # 开发服务器 (HMR)
npm run build    # 生产构建
npm run preview  # 预览构建结果
```

---

## 📄 License

[MIT License](LICENSE)

---

## 🙏 致谢

本项目参考和借鉴了以下优秀项目的思路与设计：

- **[Pulse](https://github.com/iamkhirsariya/pulse)** — Server-Agent 架构设计灵感
- **[3x-ui](https://github.com/MHSanaei/3x-ui)** — Xray 面板功能参考
- **[Xray-core](https://github.com/XTLS/Xray-core)** — 代理核心引擎
