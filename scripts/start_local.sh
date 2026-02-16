#!/bin/bash
# CloudQuantBot 本地启动脚本
# 适用于6c8g开发环境

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查环境
check_environment() {
    log_info "检查运行环境..."

    # 检查Go版本
    if ! command -v go &> /dev/null; then
        log_error "Go未安装，请先安装Go 1.22+"
        exit 1
    fi

    GO_VERSION=$(go version | grep -o 'go[0-9.]*' | head -1)
    log_info "Go版本: $GO_VERSION"

    # 检查SQLite3
    if ! command -v sqlite3 &> /dev/null; then
        log_warn "SQLite3未安装，数据库功能可能受限"
    else
        log_info "SQLite3已安装"
    fi
}

# 创建必要目录
create_directories() {
    log_info "创建必要目录..."

    mkdir -p "$PROJECT_DIR/data"
    mkdir -p "$PROJECT_DIR/logs"
    mkdir -p "$PROJECT_DIR/models"
    mkdir -p "$PROJECT_DIR/tmp"

    log_info "目录创建完成"
}

# 复制配置文件
copy_config() {
    log_info "检查配置文件..."

    if [ ! -f "$PROJECT_DIR/.env" ]; then
        if [ -f "$PROJECT_DIR/.env.example" ]; then
            cp "$PROJECT_DIR/.env.example" "$PROJECT_DIR/.env"
            log_warn ".env文件已创建，请根据需要修改配置"
        else
            log_warn ".env.example不存在，跳过配置文件创建"
        fi
    fi
}

# 下载依赖
download_dependencies() {
    log_info "下载依赖..."

    cd "$PROJECT_DIR"
    go mod download

    if [ $? -eq 0 ]; then
        log_info "依赖下载完成"
    else
        log_error "依赖下载失败"
        exit 1
    fi
}

# 编译项目
build_project() {
    log_info "编译项目..."

    cd "$PROJECT_DIR"
    go build -o cloudquant -ldflags="-s -w" .

    if [ $? -eq 0 ]; then
        log_info "编译成功: ./cloudquant"
    else
        log_error "编译失败"
        exit 1
    fi
}

# 运行健康检查
health_check() {
    log_info "等待服务启动..."
    sleep 3

    local max_attempts=10
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if curl -s http://localhost:8080/api/health > /dev/null; then
            log_info "服务健康检查通过"
            return 0
        fi
        log_warn "健康检查第 $attempt/$max_attempts 次重试..."
        sleep 2
        attempt=$((attempt + 1))
    done

    log_error "健康检查失败"
    return 1
}

# 打开浏览器
open_browser() {
    log_info "尝试打开浏览器..."

    if command -v xdg-open &> /dev/null; then
        xdg-open "http://localhost:8080" &
    elif command -v open &> /dev/null; then
        open "http://localhost:8080" &
    else
        log_warn "无法自动打开浏览器，请手动访问 http://localhost:8080"
    fi
}

# 显示使用信息
show_usage() {
    cat << EOF

CloudQuantBot 本地启动完成！

访问地址:
  - 首页:      http://localhost:8080
  - API文档:   http://localhost:8080/api/docs
  - 健康检查:  http://localhost:8080/api/health
  - WebSocket: ws://localhost:8080/api/ws/dashboard

常用API端点:
  - GET  /api/tick/{symbol}           获取实时行情
  - GET  /api/industry/exposure       行业暴露分析
  - GET  /api/risk/metrics            风险指标
  - GET  /api/visualization/equity    权益曲线

按 Ctrl+C 停止服务

EOF
}

# 清理函数
cleanup() {
    log_info "正在停止服务..."
    
    # 停止WebSocket管理器
    if [ -n "$WSPID" ]; then
        kill $WSPID 2>/dev/null || true
    fi
    
    exit 0
}

# 设置信号处理
trap cleanup SIGINT SIGTERM

# 主函数
main() {
    log_info "CloudQuantBot 本地启动脚本"
    log_info "项目目录: $PROJECT_DIR"

    # 执行步骤
    check_environment
    create_directories
    copy_config
    download_dependencies
    build_project

    # 启动服务
    log_info "启动服务..."
    cd "$PROJECT_DIR"
    
    # 后台运行并捕获PID
    ./cloudquant &
    APP_PID=$!
    
    # 等待服务启动
    if health_check; then
        show_usage
        open_browser
        
        # 等待应用退出
        wait $APP_PID
    else
        log_error "服务启动失败，请检查日志"
        kill $APP_PID 2>/dev/null || true
        exit 1
    fi
}

# 运行主函数
main "$@"
