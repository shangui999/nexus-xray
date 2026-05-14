#!/bin/bash
# xray-manager Agent 一键安装脚本
# 用法: curl -sSL https://your-server/install-agent.sh | bash -s -- --server=host:port --node-id=xxx --token=xxx

set -e

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 默认值
INSTALL_DIR="/opt/xray-manager-agent"
XRAY_VERSION="1.8.24"
SERVICE_NAME="xray-manager-agent"
AGENT_BINARY_NAME="agent"
CONFIG_FILE="agent.yaml"
SERVER_ADDR=""
NODE_ID=""
TOKEN=""
ARCH=""
OS=""

log_info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

# 解析参数
parse_args() {
    for arg in "$@"; do
        case $arg in
            --server=*) SERVER_ADDR="${arg#*=}" ;;
            --node-id=*) NODE_ID="${arg#*=}" ;;
            --token=*) TOKEN="${arg#*=}" ;;
            --install-dir=*) INSTALL_DIR="${arg#*=}" ;;
            --xray-version=*) XRAY_VERSION="${arg#*=}" ;;
            --help|-h)
                echo "Usage: $0 --server=host:port --node-id=id --token=secret [--install-dir=/opt/xray-manager-agent] [--xray-version=1.8.24]"
                exit 0
                ;;
            *) log_error "Unknown argument: $arg"; exit 1 ;;
        esac
    done

    if [ -z "$SERVER_ADDR" ] || [ -z "$NODE_ID" ] || [ -z "$TOKEN" ]; then
        log_error "--server, --node-id, and --token are required"
        echo "Usage: $0 --server=host:port --node-id=id --token=secret"
        exit 1
    fi
}

# 检测操作系统
detect_os() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)      log_error "Unsupported OS: $OS"; exit 1 ;;
    esac
    log_info "Detected OS: $OS"
}

# 检测系统架构
detect_arch() {
    local arch_raw
    arch_raw="$(uname -m)"
    case "$arch_raw" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)   ARCH="arm64" ;;
        armv7l|armhf)    ARCH="armv7" ;;
        *)               log_error "Unsupported architecture: $arch_raw"; exit 1 ;;
    esac
    log_info "Detected architecture: $ARCH"
}

# 检查依赖
check_deps() {
    local missing=()
    for cmd in curl unzip; do
        if ! command -v "$cmd" &>/dev/null; then
            missing+=("$cmd")
        fi
    done
    if [ ${#missing[@]} -gt 0 ]; then
        log_warn "Missing dependencies: ${missing[*]}"
        log_info "Attempting to install..."
        if command -v apt-get &>/dev/null; then
            apt-get update -qq && apt-get install -y -qq "${missing[@]}"
        elif command -v yum &>/dev/null; then
            yum install -y "${missing[@]}"
        elif command -v apk &>/dev/null; then
            apk add --no-cache "${missing[@]}"
        else
            log_error "Cannot install dependencies automatically. Please install: ${missing[*]}"
            exit 1
        fi
    fi
}

# 安装 xray-core
install_xray() {
    log_info "Installing xray-core v${XRAY_VERSION}..."

    local xray_arch
    case "$ARCH" in
        amd64) xray_arch="64" ;;
        arm64)  xray_arch="arm64-v8a" ;;
        armv7)  xray_arch="arm32-v7a" ;;
        *)      log_error "No xray-core binary for arch: $ARCH"; exit 1 ;;
    esac

    local xray_url="https://github.com/XTLS/Xray-core/releases/download/v${XRAY_VERSION}/Xray-linux-${xray_arch}.zip"
    local tmp_dir
    tmp_dir="$(mktemp -d)"

    log_info "Downloading xray-core from $xray_url"
    if ! curl -fsSL -o "${tmp_dir}/xray.zip" "$xray_url"; then
        log_error "Failed to download xray-core"
        rm -rf "$tmp_dir"
        exit 1
    fi

    unzip -o "${tmp_dir}/xray.zip" -d "${tmp_dir}/xray-extracted" >/dev/null
    mkdir -p "${INSTALL_DIR}/bin"
    cp "${tmp_dir}/xray-extracted/xray" "${INSTALL_DIR}/bin/xray"
    chmod +x "${INSTALL_DIR}/bin/xray"

    rm -rf "$tmp_dir"
    log_info "xray-core installed: ${INSTALL_DIR}/bin/xray"
    "${INSTALL_DIR}/bin/xray" version
}

# 下载 agent 二进制
install_agent() {
    log_info "Downloading xray-manager agent..."

    GITHUB_REPO="shangui999/nexus-xray"
        local agent_url="https://github.com/${GITHUB_REPO}/releases/latest/download/nexus-xray-agent-linux-${ARCH}"
    local tmp_dir
    tmp_dir="$(mktemp -d)"

    if ! curl -fsSL -o "${tmp_dir}/agent" "$agent_url"; then
        log_error "Failed to download agent binary from $agent_url"
        log_error "You can build it manually: go build -o agent ./cmd/agent"
        rm -rf "$tmp_dir"
        exit 1
    fi

    mkdir -p "${INSTALL_DIR}/bin"
    cp "${tmp_dir}/agent" "${INSTALL_DIR}/bin/agent"
    chmod +x "${INSTALL_DIR}/bin/agent"

    rm -rf "$tmp_dir"
    log_info "Agent binary installed: ${INSTALL_DIR}/bin/agent"
}

# 生成 agent 配置
generate_config() {
    log_info "Generating agent configuration..."

    mkdir -p "${INSTALL_DIR}/configs"

    cat > "${INSTALL_DIR}/configs/${CONFIG_FILE}" <<EOF
# xray-manager Agent 配置
server:
  address: "${SERVER_ADDR}"
  node_id: ${NODE_ID}
  token: "${TOKEN}"
  # TLS 配置（如果 Server 使用自签证书，需要指定 CA）
  # tls:
  #   ca_cert: "${INSTALL_DIR}/certs/ca.crt"
  #   server_name: "xray-manager-server"

xray:
  binary: "${INSTALL_DIR}/bin/xray"
  config_dir: "${INSTALL_DIR}/xray-configs"
  # api:
  #   address: "127.0.0.1:10085"

log:
  level: info
  # file: "${INSTALL_DIR}/logs/agent.log"
EOF

    mkdir -p "${INSTALL_DIR}/xray-configs"
    log_info "Configuration written to ${INSTALL_DIR}/configs/${CONFIG_FILE}"
}

# 执行 enrollment（获取证书）
enroll() {
    log_info "Performing enrollment with server ${SERVER_ADDR}..."

    if ! "${INSTALL_DIR}/bin/agent" -config "${INSTALL_DIR}/configs/${CONFIG_FILE}" -enroll; then
        log_warn "Enrollment failed. The agent will retry on first start."
        log_warn "Make sure the server is reachable at ${SERVER_ADDR}"
    else
        log_info "Enrollment completed successfully"
    fi
}

# 创建 systemd 服务
create_service() {
    log_info "Creating systemd service..."

    cat > /etc/systemd/system/${SERVICE_NAME}.service <<EOF
[Unit]
Description=Xray Manager Agent
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/bin/agent -config ${INSTALL_DIR}/configs/${CONFIG_FILE}
Restart=always
RestartSec=5
LimitNOFILE=65535

# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=${INSTALL_DIR} /var/log
PrivateTmp=true

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}"

    log_info "Starting ${SERVICE_NAME}..."
    systemctl start "${SERVICE_NAME}"

    sleep 2
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        log_info "Service ${SERVICE_NAME} is running"
    else
        log_error "Service ${SERVICE_NAME} failed to start. Check: journalctl -u ${SERVICE_NAME} -n 50"
        exit 1
    fi
}

# 清理旧安装
cleanup_old() {
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        log_warn "Found existing service, stopping and removing..."
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
    fi
}

# 主函数
main() {
    parse_args "$@"

    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Xray Manager Agent Installer${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""

    # 检查 root 权限
    if [ "$(id -u)" -ne 0 ]; then
        log_error "This script must be run as root"
        exit 1
    fi

    detect_os
    detect_arch
    check_deps
    cleanup_old
    install_xray
    install_agent
    generate_config
    enroll
    create_service

    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Agent installed successfully!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo -e "  Node ID:    ${NODE_ID}"
    echo -e "  Server:     ${SERVER_ADDR}"
    echo -e "  Install Dir: ${INSTALL_DIR}"
    echo -e "  Config:     ${INSTALL_DIR}/configs/${CONFIG_FILE}"
    echo -e "  Service:    systemctl status ${SERVICE_NAME}"
    echo -e "  Logs:       journalctl -u ${SERVICE_NAME} -f"
    echo ""
}

main "$@"
