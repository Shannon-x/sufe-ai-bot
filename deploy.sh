#!/bin/bash

# ==============================================================================
#  cf-ai-tgbot-go Docker Deployment Script
# ==============================================================================

set -e

# --- Configuration ---
IMAGE_NAME="cf-ai-tgbot-go"
CONTAINER_NAME="cf-ai-tgbot-go"

# --- Colors ---
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# --- Functions ---
print_step() {
    echo -e "${YELLOW}$1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

# --- Script Start ---
echo -e "${GREEN}======================================================${NC}"
echo -e "${GREEN}  欢迎使用 cf-ai-tgbot-go 部署向导  ${NC}"
echo -e "${GREEN}======================================================${NC}"
echo

# 1. Check Docker and Docker Compose
print_step "步骤 1: 检查 Docker 环境..."
if ! command -v docker &> /dev/null; then
    print_error "未找到 Docker。请先安装 Docker。"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    print_error "未找到 Docker Compose。请先安装 Docker Compose。"
    exit 1
fi

if ! docker info &> /dev/null; then
    print_error "Docker 守护进程未运行。请启动 Docker 服务。"
    exit 1
fi
print_success "Docker 环境正常。"
echo

# 2. Check for .env file
print_step "步骤 2: 检查配置文件..."
if [ ! -f .env ]; then
    print_error "未找到 .env 文件。现在创建一个..."
    
    # Create .env file interactively
    read -s -p "请输入 BOT_TOKEN: " BOT_TOKEN
    echo
    read -s -p "请输入 CLOUDFLARE_API_TOKEN: " CLOUDFLARE_API_TOKEN
    echo
    read -s -p "请输入 CLOUDFLARE_ACCOUNT_ID: " CLOUDFLARE_ACCOUNT_ID
    echo
    read -s -p "请输入 CLOUDFLARE_GATEWAY_NAME: " CLOUDFLARE_GATEWAY_NAME
    echo
    
    # Ask about Redis configuration
    echo
    print_info "Redis 配置"
    echo "1) 使用内置 Redis（默认）"
    echo "2) 使用外部 Redis"
    read -p "选择 Redis 选项 (1 或 2) [1]: " REDIS_OPTION
    REDIS_OPTION=${REDIS_OPTION:-1}
    
    if [ "$REDIS_OPTION" = "2" ]; then
        read -p "请输入 REDIS_HOST [localhost]: " REDIS_HOST
        REDIS_HOST=${REDIS_HOST:-localhost}
        read -p "请输入 REDIS_PORT [6379]: " REDIS_PORT
        REDIS_PORT=${REDIS_PORT:-6379}
        read -s -p "请输入 REDIS_PASSWORD（如果没有请留空）: " REDIS_PASSWORD
        echo
        read -p "请输入 REDIS_DB [0]: " REDIS_DB
        REDIS_DB=${REDIS_DB:-0}
        
        # Create .env with external Redis
        cat > .env << EOF
BOT_TOKEN=${BOT_TOKEN}
CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}
CLOUDFLARE_ACCOUNT_ID=${CLOUDFLARE_ACCOUNT_ID}
CLOUDFLARE_GATEWAY_NAME=${CLOUDFLARE_GATEWAY_NAME}

# External Redis Configuration
REDIS_HOST=${REDIS_HOST}
REDIS_PORT=${REDIS_PORT}
REDIS_PASSWORD=${REDIS_PASSWORD}
REDIS_DB=${REDIS_DB}
EOF
        USE_EXTERNAL_REDIS=true
    else
        # Create .env for built-in Redis
        cat > .env << EOF
BOT_TOKEN=${BOT_TOKEN}
CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}
CLOUDFLARE_ACCOUNT_ID=${CLOUDFLARE_ACCOUNT_ID}
CLOUDFLARE_GATEWAY_NAME=${CLOUDFLARE_GATEWAY_NAME}
EOF
        USE_EXTERNAL_REDIS=false
    fi
    
    print_success ".env 文件创建成功。"
else
    print_success "找到 .env 文件。"
    # Check if external Redis is configured
    if grep -q "REDIS_HOST" .env 2>/dev/null; then
        USE_EXTERNAL_REDIS=true
        print_info "检测到外部 Redis 配置。"
    else
        USE_EXTERNAL_REDIS=false
    fi
fi
echo

# 3. Select appropriate docker-compose file
print_step "步骤 3: 选择部署配置..."
if [ "$USE_EXTERNAL_REDIS" = true ]; then
    COMPOSE_FILE="docker-compose.external-redis.yml"
    print_info "使用外部 Redis 配置。"
else
    COMPOSE_FILE="docker-compose.yml"
    print_info "使用内置 Redis 配置。"
fi
echo

# 4. Build or pull image
print_step "步骤 4: 构建 Docker 镜像..."
if docker-compose -f $COMPOSE_FILE build; then
    print_success "Docker 镜像构建成功。"
else
    print_error "Docker 镜像构建失败。"
    exit 1
fi
echo

# 5. Stop existing containers
print_step "步骤 5: 检查现有容器..."
if docker-compose -f $COMPOSE_FILE ps | grep -q ${CONTAINER_NAME}; then
    echo "发现现有容器。正在停止..."
    docker-compose -f $COMPOSE_FILE down
    print_success "现有容器已停止。"
else
    echo "未发现现有容器。"
fi
echo

# 6. Start services
print_step "步骤 6: 启动服务..."
if docker-compose -f $COMPOSE_FILE up -d; then
    print_success "服务启动成功。"
else
    print_error "服务启动失败。"
    exit 1
fi
echo

# 7. Check deployment status
print_step "步骤 7: 检查部署状态..."
sleep 5

if docker-compose -f $COMPOSE_FILE ps | grep -q "Up"; then
    echo -e "${GREEN}======================================================${NC}"
    echo -e "${GREEN}🎉 部署成功！ 🎉${NC}"
    echo
    echo -e "您的 Telegram AI 机器人现在正在运行。"
    echo -e "配置文件: ${BLUE}$COMPOSE_FILE${NC}"
    echo
    echo -e "您可以使用以下命令管理机器人:"
    echo -e "  - 查看日志: ${YELLOW}docker-compose -f $COMPOSE_FILE logs -f bot${NC}"
    echo -e "  - 停止服务: ${YELLOW}docker-compose -f $COMPOSE_FILE down${NC}"
    echo -e "  - 重启服务: ${YELLOW}docker-compose -f $COMPOSE_FILE restart${NC}"
    echo -e "  - 查看指标: ${YELLOW}http://localhost:9090/metrics${NC}"
    echo -e "  - Grafana 仪表板: ${YELLOW}http://localhost:3000${NC} (admin/admin)"
    echo -e "${GREEN}======================================================${NC}"
else
    print_error "部署失败。使用以下命令查看日志: docker-compose -f $COMPOSE_FILE logs"
    exit 1
fi

exit 0