#!/bin/bash

# CloudQuantBot 本地启动脚本
# 此脚本用于本地开发环境启动，不依赖券商配置

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

echo "======================================"
echo "CloudQuantBot 本地启动脚本"
echo "======================================"
echo ""

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo "错误: 未找到 Go 环境，请先安装 Go 1.22+"
    exit 1
fi

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Go 版本: $GO_VERSION"

# 检查必需文件
if [ ! -f "config.yaml" ]; then
    echo "错误: 未找到 config.yaml 文件"
    exit 1
fi

if [ ! -d "data" ]; then
    echo "创建 data 目录..."
    mkdir -p data
fi

# 创建本地配置文件（如果不存在）
if [ ! -f "config.local.yaml" ]; then
    echo "创建本地配置文件 config.local.yaml..."
    cat > config.local.yaml << 'EOF'
# 本地开发配置
mode: "local"

# 数据库
database:
  path: "./data/quant_local.db"

# HTTP服务
http:
  port: 8080

# 日志
log:
  level: "debug"

# 行情数据源配置
data_source:
  primary: "mock"
  fallback: ["sina", "eastmoney", "tencent"]
  mock_on_failure: true
  health_check_interval: "30s"

# Mock 数据配置
mock:
  enabled: true
  update_interval: "1s"

# 监控的股票（示例）
symbols:
  - sh600000
  - sh601398
  - sh600519
  - sh600036
  - sz000858

# LLM 配置（可选）
llm:
  provider: deepseek
  api_key: "${DEEPSEEK_API_KEY:-}"
  model: "deepseek-chat"
  timeout: 10s
  max_tokens: 200

# 机器学习配置
ml:
  model_type: "decision_tree"
  model_path: "./models/dt.model"
  max_tree_depth: 10
  train_interval: "7d"

  features:
    lookback_days: 20
    lookahead_days: 3

  training:
    min_data_points: 100
    test_ratio: 0.2

# 交易系统（本地模式禁用实盘交易）
trading:
  broker:
    type: "mock"
    service_url: ""
    broker_type: ""
    username: ""
    password: ""
    exe_path: ""

  risk:
    initial_capital: 100000.0
    max_single_position: 0.3
    max_positions: 3
    max_daily_loss: 0.1
    min_order_amount: 100.0
    stop_loss_percent: 0.05

  auto_trade:
    enabled: false
    check_interval: "1m"
    ai_threshold: 0.7
    ml_confidence: 0.6

  strategies:
    - name: "ma_strategy"
      type: "ma"
      enabled: true
      weight: 0.3
      priority: 1
      parameters:
        short_period: 5
        long_period: 20
        min_volume: 1000000
        max_change: 0.05

  portfolio_risk:
    max_industry_exposure: 0.6
    max_sector_exposure: 0.8
    max_symbol_exposure: 0.3
    concentration_alert: 0.4

# 监控配置
monitoring:
  websocket:
    enabled: true
    port: 8080
    max_connections: 100

  alerts:
    enabled: true
    channels:
      email:
        enabled: false
      feishu:
        enabled: false
      dingding:
        enabled: false

# 回测配置
backtest:
  enabled: true
  default_config:
    start_date: "2023-01-01"
    end_date: "2024-01-01"
    initial_capital: 100000.0
    commission: 0.001
    slippage: 0.0005
    risk_free_rate: 0.03
    max_drawdown_limit: 0.2
    realtime: false

  parameter_search:
    method: "grid_search"
    metric: "sharpe_ratio"
    max_iterations: 100
    min_samples: 10
    parallel: false
    max_workers: 4
    early_stopping: true
    patience: 10
EOF

    echo "已创建 config.local.yaml"
fi

# 设置环境变量（如果 .env 文件存在）
if [ -f ".env" ]; then
    echo "加载环境变量..."
    export $(grep -v '^#' .env | xargs)
fi

# 检查依赖
echo "检查 Go 依赖..."
if ! go mod tidy 2>&1 | grep -q "error"; then
    echo "依赖检查完成"
else
    echo "警告: 依赖检查出现问题"
fi

echo ""
echo "======================================"
echo "启动服务..."
echo "======================================"
echo ""

# 启动 Go 服务
echo "启动 Go API 服务..."
CONFIG_FILE="${CONFIG_FILE:-config.local.yaml}" go run main.go

echo ""
echo "======================================"
echo "服务已停止"
echo "======================================"
