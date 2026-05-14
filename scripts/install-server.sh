#!/bin/bash
# Nexus-Xray Server 一键部署脚本
# 用法: curl -sSL https://raw.githubusercontent.com/shangui999/nexus-xray/main/scripts/install-server.sh | bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

INSTALL_DIR="/opt/nexus-xray"
REPO="shangui999/nexus-xray"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  Nexus-Xray Server 一键部署${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 检测系统
check_system() {
    if [[ ! -f /etc/os-release ]]; then
        echo -e "${RED}不支持的操作系统${NC}"
        exit 1
    fi
    source /etc/os-release
    echo -e "系统: ${ID} ${VERSION_ID}"
}

# 安装 Docker（如果没有）
install_docker() {
    if command -v docker &> /dev/null; then
        echo -e "${GREEN}Docker 已安装${NC}"
        return
    fi
    echo -e "${YELLOW}正在安装 Docker...${NC}"
    curl -fsSL https://get.docker.com | bash
    systemctl enable docker
    systemctl start docker
    echo -e "${GREEN}Docker 安装完成${NC}"
}

# 安装 Docker Compose（如果没有）
install_docker_compose() {
    if docker compose version &> /dev/null; then
        echo -e "${GREEN}Docker Compose 已安装${NC}"
        return
    fi
    echo -e "${YELLOW}正在安装 Docker Compose...${NC}"
    apt-get update && apt-get install -y docker-compose-plugin 2>/dev/null || \
    yum install -y docker-compose-plugin 2>/dev/null || true
    echo -e "${GREEN}Docker Compose 安装完成${NC}"
}

# 创建安装目录
setup_directory() {
    mkdir -p ${INSTALL_DIR}
    cd ${INSTALL_DIR}
    
    # 创建 data 目录
    mkdir -p data/postgres data/certs data/configs
}

# 下载项目文件
download_files() {
    echo -e "${YELLOW}正在下载配置文件...${NC}"
    
    BASE_URL="https://raw.githubusercontent.com/${REPO}/main"
    
    # 下载 docker-compose.yml（使用 GHCR 镜像，无需本地构建）
    curl -sSL "${BASE_URL}/docker-compose.yml" -o docker-compose.yml
    
    # 下载默认配置
    curl -sSL "${BASE_URL}/configs/server.yaml" -o data/configs/server.yaml
    curl -sSL "${BASE_URL}/configs/cloudflared.yml" -o data/configs/cloudflared.yml
    
    # 下载 .env.example
    curl -sSL "${BASE_URL}/.env.example" -o .env.example
}

# 生成配置
generate_config() {
    if [[ ! -f .env ]]; then
        echo -e "${YELLOW}正在生成配置...${NC}"
        
        # 生成随机密码和密钥
        DB_PASS=$(openssl rand -hex 16)
        JWT_SEC=$(openssl rand -hex 32)
        SUB_SEC=$(openssl rand -hex 32)
        
        cat > .env <<EOF
# Database
DB_PASSWORD=${DB_PASS}

# Server
JWT_SECRET=${JWT_SEC}
SUBSCRIPTION_SECRET=${SUB_SEC}

# Cloudflare Tunnel (可选，填写后取消 cloudflared 服务注释)
# CF_TUNNEL_TOKEN=your-cloudflare-tunnel-token
EOF
        echo -e "${GREEN}配置已生成: ${INSTALL_DIR}/.env${NC}"
    else
        echo -e "${GREEN}配置文件已存在，跳过生成${NC}"
    fi
}

# 启动服务
start_services() {
    echo -e "${YELLOW}正在启动服务...${NC}"
    docker compose up -d postgres server
    
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  部署完成！${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo -e "管理面板: http://$(curl -s ifconfig.me):8080"
    echo -e "默认账号: admin / admin123"
    echo -e ""
    echo -e "配置文件: ${INSTALL_DIR}/.env"
    echo -e "数据目录: ${INSTALL_DIR}/data/"
    echo -e ""
    echo -e "${YELLOW}安全提示:${NC}"
    echo -e "1. 请立即修改默认管理员密码"
    echo -e "2. 建议配置 Cloudflare Tunnel 隐藏真实 IP"
    echo -e "3. 查看日志: docker compose logs -f"
    echo -e ""
}

# 主函数
main() {
    check_system
    install_docker
    install_docker_compose
    setup_directory
    download_files
    generate_config
    start_services
}

main
