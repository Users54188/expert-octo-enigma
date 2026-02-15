#!/bin/bash

# CloudQuantBot 开发模式启动脚本
# 带有热重载功能，用于开发调试

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "======================================"
echo "CloudQuantBot 开发模式启动脚本"
echo "======================================"
echo ""

# 检查 air 工具（热重载）
if ! command -v air &> /dev/null; then
    echo "未找到 air 工具，正在安装..."
    go install github.com/cosmtrek/air@latest
    echo "air 工具已安装"
fi

# 检查必需文件
if [ ! -f "config.local.yaml" ]; then
    echo "创建本地配置文件..."
    ./scripts/start_local.sh &
    sleep 2
    kill %1 2>/dev/null || true
fi

# 创建 .air.toml 配置（如果不存在）
if [ ! -f ".air.toml" ]; then
    cat > .air.toml << 'EOF'
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ."
  bin = "tmp/main"
  include_ext = ["go", "yaml", "yml"]
  exclude_dir = ["tmp", "vendor", "testdata"]
  include_dir = []
  exclude_file = []
  delay = 1000
  stop_on_error = true
  send_interrupt = false
  kill_delay = 0

[log]
  time = true
  main_only = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"
EOF
fi

echo "开发模式配置:"
echo "  - 热重载: 启用"
echo "  - 日志级别: debug"
echo "  - 数据源: mock"
echo ""

# 设置环境变量
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
fi

# 使用 air 启动
echo "启动开发服务器（带热重载）..."
CONFIG_FILE="config.local.yaml" air -c .air.toml
