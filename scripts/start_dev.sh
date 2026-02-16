#!/bin/bash
# CloudQuantBot 开发模式启动脚本
# 支持热重载、详细日志、性能分析

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 脚本目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 配置
LOG_LEVEL=${LOG_LEVEL:-debug}
PPROF_ENABLED=${PPROF_ENABLED:-true}
MOCK_ENABLED=${MOCK_ENABLED:-true}
HOT_RELOAD=${HOT_RELOAD:-true}

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查热重载工具
check_hot_reload() {
    if [ "$HOT_RELOAD" = "true" ]; then
        if command -v air &> /dev/null; then
            log_info "使用 air 进行热重载"
            return 0
        elif command -v reflex &> /dev/null; then
            log_info "使用 reflex 进行热重载"
            return 0
        else
            log_warn "未安装 air 或 reflex，热重载功能不可用"
            log_info "安装 air: go install github.com/cosmtrek/air@latest"
            return 1
        fi
    fi
    return 1
}

# 创建开发配置
create_dev_config() {
    log_info "创建开发配置文件..."

    # 创建 .air.toml
    if [ ! -f "$PROJECT_DIR/.air.toml" ]; then
        cat > "$PROJECT_DIR/.air.toml" << 'EOF'
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ."
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "logs", "data"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
EOF
        log_info ".air.toml 创建完成"
    fi
}

# 设置环境变量
setup_env() {
    log_info "设置开发环境变量..."

    export CLOUDQUANT_MODE=dev
    export CLOUDQUANT_LOG_LEVEL=$LOG_LEVEL
    export CLOUDQUANT_PPROF_ENABLED=$PPROF_ENABLED
    export CLOUDQUANT_MOCK_ENABLED=$MOCK_ENABLED
    export GOMAXPROCS=6  # 限制使用6核
    export GOGC=100      # GC目标100%

    log_debug "LOG_LEVEL=$LOG_LEVEL"
    log_debug "PPROF_ENABLED=$PPROF_ENABLED"
    log_debug "MOCK_ENABLED=$MOCK_ENABLED"
}

# 创建必要目录
create_directories() {
    log_info "创建开发目录..."

    mkdir -p "$PROJECT_DIR/tmp"
    mkdir -p "$PROJECT_DIR/data"
    mkdir -p "$PROJECT_DIR/logs"
    mkdir -p "$PROJECT_DIR/models"
    mkdir -p "$PROJECT_DIR/pprof"
}

# 启动pprof
start_pprof() {
    if [ "$PPROF_ENABLED" = "true" ]; then
        log_info "pprof 性能分析已启用"
        log_info "访问 http://localhost:6060/debug/pprof/ 查看性能数据"
        
        # 在后台启动pprof服务器
        go tool pprof -http=:8081 http://localhost:6060/debug/pprof/heap &
        PPROF_PID=$!
    fi
}

# 启动热重载
start_hot_reload() {
    log_info "启动热重载模式..."

    cd "$PROJECT_DIR"

    if command -v air &> /dev/null; then
        exec air
    elif command -v reflex &> /dev/null; then
        exec reflex -r '\.go$' -s -- go run .
    else
        log_error "没有可用的热重载工具"
        exit 1
    fi
}

# 启动普通开发模式
start_normal() {
    log_info "启动开发模式（无热重载）..."

    cd "$PROJECT_DIR"
    go run .
}

# 清理函数
cleanup() {
    log_info "正在清理..."
    
    if [ -n "$PPROF_PID" ]; then
        kill $PPROF_PID 2>/dev/null || true
    fi

    exit 0
}

# 设置信号处理
trap cleanup SIGINT SIGTERM

# 显示使用信息
show_usage() {
    cat << EOF

CloudQuantBot 开发模式

用法: $0 [选项]

选项:
  --no-hot-reload    禁用热重载
  --no-pprof         禁用pprof性能分析
  --no-mock          禁用Mock数据
  --log-level=LEVEL  设置日志级别 (debug/info/warn/error)
  --help             显示此帮助信息

环境变量:
  LOG_LEVEL          日志级别 (默认: debug)
  PPROF_ENABLED      启用pprof (默认: true)
  MOCK_ENABLED       启用Mock数据 (默认: true)
  HOT_RELOAD         启用热重载 (默认: true)

性能分析端点:
  - pprof: http://localhost:6060/debug/pprof/
  - 内存: http://localhost:6060/debug/pprof/heap
  - CPU:  http://localhost:6060/debug/pprof/profile
  - 阻塞: http://localhost:6060/debug/pprof/block

按 Ctrl+C 停止服务

EOF
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --no-hot-reload)
                HOT_RELOAD=false
                shift
                ;;
            --no-pprof)
                PPROF_ENABLED=false
                shift
                ;;
            --no-mock)
                MOCK_ENABLED=false
                shift
                ;;
            --log-level=*)
                LOG_LEVEL="${1#*=}"
                shift
                ;;
            --help)
                show_usage
                exit 0
                ;;
            *)
                log_warn "未知选项: $1"
                shift
                ;;
        esac
    done
}

# 主函数
main() {
    log_info "CloudQuantBot 开发模式启动"
    log_info "项目目录: $PROJECT_DIR"

    # 解析参数
    parse_args "$@"

    # 设置环境
    setup_env
    create_directories
    create_dev_config

    # 启动pprof
    start_pprof

    # 启动服务
    if check_hot_reload; then
        start_hot_reload
    else
        start_normal
    fi
}

# 运行主函数
main "$@"
