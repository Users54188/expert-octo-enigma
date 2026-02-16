# CloudQuantBot API文档

本文档提供CloudQuantBot的完整API接口文档。

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **版本**: v1
- **协议**: HTTP/1.1, WebSocket
- **数据格式**: JSON

## 认证

当前版本未实现认证机制。计划支持：

- API Key认证
- JWT Token认证
- OAuth 2.0

## 通用响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    // 业务数据
  },
  "timestamp": 1234567890
}
```

### 错误响应

```json
{
  "code": 1001,
  "message": "Parameter error",
  "error": "Invalid symbol format",
  "timestamp": 1234567890
}
```

## 错误码

| 错误码 | 说明 |
|-------|------|
| 0 | 成功 |
| 1000 | 未知错误 |
| 1001 | 参数错误 |
| 1002 | 未找到资源 |
| 1003 | 权限不足 |
| 2000 | 数据库错误 |
| 3000 | 外部服务错误 |

## API端点

### 系统相关

#### 1. 健康检查

```http
GET /api/v1/health
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "healthy",
    "uptime": 3600,
    "version": "1.0.0"
  }
}
```

#### 2. 版本信息

```http
GET /api/v1/version
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "version": "1.0.0",
    "build_time": "2024-01-01_12:00:00",
    "git_commit": "abc123"
  }
}
```

### 市场数据

#### 1. 获取实时行情

```http
GET /api/v1/market/{symbol}
```

**参数**:
- `symbol`: 股票代码（必需）

**示例**:
```bash
curl http://localhost:8080/api/v1/market/sh600000
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "symbol": "sh600000",
    "name": "浦发银行",
    "price": 10.50,
    "change": 0.10,
    "change_percent": 0.96,
    "open": 10.45,
    "high": 10.55,
    "low": 10.40,
    "volume": 1000000,
    "amount": 10500000,
    "timestamp": 1234567890
  }
}
```

#### 2. 批量获取行情

```http
GET /api/v1/market/batch?symbols=sh600000,sh601398,sz000001
```

#### 3. 获取K线数据

```http
GET /api/v1/market/kline/{symbol}?period=1d&start=2024-01-01&end=2024-01-31
```

**参数**:
- `symbol`: 股票代码（必需）
- `period`: 周期（1m, 5m, 15m, 30m, 1h, 1d, 1w, 1M）
- `start`: 开始日期（可选）
- `end`: 结束日期（可选）

### 交易相关

#### 1. 提交订单

```http
POST /api/v1/trading/orders
Content-Type: application/json
```

**请求体**:
```json
{
  "symbol": "sh600000",
  "side": "buy",
  "type": "limit",
  "quantity": 1000,
  "price": 10.50
}
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "order_id": "ord_1234567890",
    "status": "pending"
  }
}
```

#### 2. 查询订单

```http
GET /api/v1/trading/orders/{order_id}
```

#### 3. 查询持仓

```http
GET /api/v1/trading/positions
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "symbol": "sh600000",
      "quantity": 1000,
      "avg_price": 10.50,
      "current_price": 10.60,
      "market_value": 10600,
      "pnl": 100,
      "pnl_percent": 0.95
    }
  ]
}
```

#### 4. 取消订单

```http
DELETE /api/v1/trading/orders/{order_id}
```

### 策略相关

#### 1. 获取策略列表

```http
GET /api/v1/strategies
```

#### 2. 启用策略

```http
POST /api/v1/strategies/{strategy_name}/enable
```

#### 3. 禁用策略

```http
POST /api/v1/strategies/{strategy_name}/disable
```

#### 4. 获取策略信号

```http
GET /api/v1/strategies/{strategy_name}/signals?symbol=sh600000
```

### 风控相关

#### 1. 获取风险限额

```http
GET /api/v1/risk/limits
```

#### 2. 更新风险限额

```http
PUT /api/v1/risk/limits/{limit_name}
Content-Type: application/json
```

**请求体**:
```json
{
  "warning_threshold": 0.25,
  "critical_threshold": 0.30
}
```

#### 3. 获取风险事件

```http
GET /api/v1/risk/events?limit=100
```

### 回测相关

#### 1. 创建回测任务

```http
POST /api/v1/backtest/tasks
Content-Type: application/json
```

**请求体**:
```json
{
  "strategies": ["ma_strategy", "rsi_strategy"],
  "symbols": ["sh600000", "sh601398"],
  "start_date": "2023-01-01",
  "end_date": "2024-01-01",
  "initial_capital": 100000,
  "commission": 0.001,
  "slippage": 0.0005
}
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "task_id": "bt_1234567890",
    "status": "running"
  }
}
```

#### 2. 查询回测结果

```http
GET /api/v1/backtest/tasks/{task_id}
```

#### 3. 获取回测报告

```http
GET /api/v1/backtest/tasks/{task_id}/report
```

### 监控相关

#### 1. 获取指标

```http
GET /api/v1/monitoring/metrics
```

#### 2. 获取系统状态

```http
GET /api/v1/monitoring/status
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "uptime": 3600,
    "cpu_usage": 45.5,
    "memory_usage": 2048,
    "goroutines": 150,
    "connections": 10
  }
}
```

#### 3. 获取业务统计

```http
GET /api/v1/monitoring/business/stats
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "trade_count": 100,
    "trade_volume": 1000000,
    "order_count": 120,
    "position_count": 5,
    "strategy_stats": [
      {
        "name": "ma_strategy",
        "signal_count": 50,
        "trade_count": 30,
        "win_rate": 0.6,
        "total_pnl": 500
      }
    ]
  }
}
```

## WebSocket接口

### 连接

```
ws://localhost:8080/api/v1/ws
```

### 订阅频道

#### 市场数据

```json
{
  "action": "subscribe",
  "channel": "market",
  "symbols": ["sh600000", "sh601398"]
}
```

#### 持仓更新

```json
{
  "action": "subscribe",
  "channel": "position"
}
```

#### 信号更新

```json
{
  "action": "subscribe",
  "channel": "signal",
  "strategies": ["ma_strategy"]
}
```

#### 告警推送

```json
{
  "action": "subscribe",
  "channel": "alert"
}
```

### 消息格式

#### 市场数据推送

```json
{
  "channel": "market",
  "data": {
    "symbol": "sh600000",
    "price": 10.50,
    "volume": 1000000,
    "timestamp": 1234567890
  }
}
```

#### 告警推送

```json
{
  "channel": "alert",
  "data": {
    "id": "risk_1234567890",
    "type": "position_risk",
    "level": "high",
    "message": "Single position exceeds limit",
    "timestamp": 1234567890
  }
}
```

## 限流

- 普通请求: 1000 requests/minute
- WebSocket连接: 100 connections
- 订单提交: 100 requests/minute

## SDK

### Go SDK

```go
import "github.com/yourorg/cloudquant-go"

client := cloudquant.NewClient("http://localhost:8080")

// 获取行情
market, err := client.GetMarket("sh600000")

// 提交订单
order, err := client.SubmitOrder(&cloudquant.Order{
    Symbol:   "sh600000",
    Side:     "buy",
    Type:     "limit",
    Quantity: 1000,
    Price:    10.50,
})
```

### Python SDK

```python
from cloudquant import Client

client = Client("http://localhost:8080")

# 获取行情
market = client.get_market("sh600000")

# 提交订单
order = client.submit_order(
    symbol="sh600000",
    side="buy",
    type="limit",
    quantity=1000,
    price=10.50
)
```

## 示例代码

### 获取实时行情

```bash
# curl
curl http://localhost:8080/api/v1/market/sh600000

# wget
wget -qO- http://localhost:8080/api/v1/market/sh600000
```

### 提交订单

```bash
curl -X POST http://localhost:8080/api/v1/trading/orders \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "sh600000",
    "side": "buy",
    "type": "limit",
    "quantity": 1000,
    "price": 10.50
  }'
```

### WebSocket示例

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/ws');

ws.onopen = function() {
  // 订阅市场数据
  ws.send(JSON.stringify({
    action: 'subscribe',
    channel: 'market',
    symbols: ['sh600000', 'sh601398']
  }));
};

ws.onmessage = function(event) {
  const data = JSON.parse(event.data);
  console.log('Received:', data);
};
```

## 相关文档

- [测试指南](TESTING.md)
- [生产部署](PRODUCTION.md)
- [架构设计](ARCHITECTURE.md)
