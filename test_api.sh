#!/bin/bash

# CloudQuantBot API测试脚本

BASE_URL="${API_BASE_URL:-http://localhost:8080}"

echo "======================================"
echo "  CloudQuantBot API 测试"
echo "  API地址: $BASE_URL"
echo "======================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 测试函数
test_api() {
    local name=$1
    local url=$2
    local method=${3:-GET}
    local data=$4

    echo -n "测试: $name ... "

    if [ -n "$data" ]; then
        response=$(curl -s -X "$method" "$BASE_URL$url" \
            -H "Content-Type: application/json" \
            -d "$data")
    else
        response=$(curl -s -X "$method" "$BASE_URL$url")
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC}"
        echo "  响应: $response"
    else
        echo -e "${RED}✗${NC}"
        echo "  错误: 请求失败"
    fi
    echo ""
}

# 1. 健康检查
test_api "健康检查" "/api/health"

# 2. 实时行情
test_api "实时行情(sh600000)" "/api/tick/sh600000"

# 3. 技术指标
test_api "技术指标(sh600000)" "/api/indicators/sh600000?days=30"

# 4. K线数据
test_api "K线数据(sh600000)" "/api/klines/sh600000?limit=10"

# 5. 投资组合
test_api "投资组合" "/api/trading/portfolio"

# 6. 账户余额
test_api "账户余额" "/api/trading/balance"

# 7. 订单历史
test_api "订单历史" "/api/trading/orders?limit=10"

# 8. 成交记录
test_api "成交记录" "/api/trading/trades?limit=10"

# 9. 绩效统计
test_api "绩效统计" "/api/trading/performance"

# 10. 日度盈亏
test_api "日度盈亏" "/api/trading/daily_pnl?days=7"

# 11. 风险指标
test_api "风险指标" "/api/trading/risk"

# 12. 自动交易状态
test_api "自动交易状态" "/api/trading/auto_trade/status"

echo "======================================"
echo "  测试完成"
echo "======================================"
