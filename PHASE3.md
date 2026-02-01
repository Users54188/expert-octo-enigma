# CloudQuantBot Phase 3：实盘交易与风险管理系统

## 概述

Phase 3 为 CloudQuantBot 添加了完整的实盘交易和风险管理功能，支持通过 easytrader 连接真实券商账户进行自动化交易，并内置严格的风险控制机制。

## 核心功能

### 1. 券商接口集成
- **Python微服务**：基于 FastAPI 的 easytrader 微服务（端口8888）
- **多券商支持**：华泰(ht)、银河(yh)、佣金宝(yjb)、雪球(xq)
- **推荐券商**：银河证券（最稳定）

### 2. 风险管理
严格的风险控制机制：
- **初始资金**：100元（可配置）
- **单只股票仓位**：最多总资金的30%
- **最大持仓数**：3只股票
- **单日止损**：亏损10%时全部平仓
- **单股止损**：亏损5%时触发止损
- **最小下单金额**：100元

### 3. 订单执行
- 买入/卖出订单提交
- 订单撤销
- 订单状态查询
- 自动同步成交记录

### 4. 信号处理（AI+ML融合）
- AI信号分析（DeepSeek）
- ML预测信号（决策树）
- 信号融合决策
- 置信度阈值过滤

### 5. 交易历史与绩效
- 完整的交易记录
- 日度盈亏统计
- 绩效指标计算：
  - 总收益率
  - 日均收益率
  - 最大回撤
  - 胜率
  - 盈亏比
  - 夏普比率

## 架构设计

```
┌─────────────────────────────────────────────────────┐
│                    用户界面/API                       │
│              (http://localhost:8080)                │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                HTTP API 层                          │
│  - handlers.go (基础API)                             │
│  - trading_handlers.go (交易API)                    │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                业务逻辑层                           │
│  ├── RiskManager (风险管理)                        │
│  ├── PositionManager (持仓管理)                     │
│  ├── OrderExecutor (订单执行)                       │
│  ├── SignalHandler (信号处理)                       │
│  └── TradeHistory (交易历史)                        │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│             券商接口层                              │
│  ├── BrokerConnector (连接管理)                     │
│  └── EasyTraderBroker (HTTP客户端)                 │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│        EasyTrader 微服务 (Python)                   │
│              (localhost:8888)                       │
│  - login/logout                                     │
│  - buy/sell/cancel                                 │
│  - balance/portfolio/orders                         │
└─────────────────────────────────────────────────────┘
```

## 使用指南

### 1. 环境准备

#### 使用 Docker（推荐）

1. 复制环境变量模板：
   ```bash
   cp .env.example .env
   ```

2. 编辑 `.env` 文件，填写配置：
   ```bash
   DEEPSEEK_API_KEY=your_api_key
   BROKER_USERNAME=your_username
   BROKER_PASSWORD=your_password
   ```

3. 启动服务：
   ```bash
   docker-compose up --build
   ```

#### 本地运行

1. 启动 easytrader 服务：
   ```bash
   cd trading/broker_service
   pip install -r requirements.txt
   python easytrader_service.py
   ```

2. 启动 Go API：
   ```bash
   go run main.go
   ```

### 2. 测试API

使用提供的测试脚本：
```bash
./test_api.sh
```

### 3. 手动交易示例

#### 买入股票
```bash
curl -X POST http://localhost:8080/api/trading/bay \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "sh600000",
    "price": 10.0,
    "amount": 1000.0
  }'
```

#### 卖出股票
```bash
curl -X POST http://localhost:8080/api/trading/sell \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "sh600000",
    "price": 10.0,
    "quantity": 100
  }'
```

#### 撤销订单
```bash
curl -X POST http://localhost:8080/api/trading/cancel \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "order_id_here"
  }'
```

### 4. 自动交易

#### 启动自动交易
```bash
curl -X POST http://localhost:8080/api/trading/auto_trade/start
```

#### 停止自动交易
```bash
curl -X POST http://localhost:8080/api/trading/auto_trade/stop
```

#### 查看状态
```bash
curl http://localhost:8080/api/trading/auto_trade/status
```

## 配置说明

在 `config.yaml` 中配置交易系统参数：

```yaml
trading:
  broker:
    type: "easytrader"
    service_url: "http://localhost:8888"
    broker_type: "yh"  # 券商类型
    username: "${BROKER_USERNAME}"
    password: "${BROKER_PASSWORD}"

  risk:
    initial_capital: 100.0      # 初始资金
    max_single_position: 0.3    # 单只股票最大仓位
    max_positions: 3           # 最大持仓数
    max_daily_loss: 0.1        # 单日最大亏损
    min_order_amount: 100.0    # 最小下单金额
    stop_loss_percent: 0.05    # 止损比例

  auto_trade:
    enabled: false             # 是否启用
    check_interval: "1m"       # 检查间隔
    ai_threshold: 0.7          # AI阈值
    ml_confidence: 0.6         # ML置信度
```

## 数据库表结构

### trades - 成交记录
| 字段 | 类型 | 说明 |
|------|------|------|
| trade_id | TEXT | 成交编号 |
| order_id | TEXT | 委托编号 |
| symbol | TEXT | 股票代码 |
| type | TEXT | 买卖类型 |
| price | REAL | 成交价格 |
| amount | INTEGER | 成交数量 |
| commission | REAL | 手续费 |
| trade_time | DATETIME | 成交时间 |

### orders - 委托记录
| 字段 | 类型 | 说明 |
|------|------|------|
| order_id | TEXT | 委托编号 |
| symbol | TEXT | 股票代码 |
| type | TEXT | 买卖类型 |
| price | REAL | 委托价格 |
| amount | INTEGER | 委托数量 |
| filled_amount | INTEGER | 成交数量 |
| status | TEXT | 委托状态 |
| order_time | DATETIME | 委托时间 |

### positions - 持仓记录
| 字段 | 类型 | 说明 |
|------|------|------|
| symbol | TEXT | 股票代码 |
| amount | INTEGER | 持仓数量 |
| cost_price | REAL | 成本价 |
| total_cost | REAL | 总成本 |
| current_price | REAL | 当前价 |
| market_value | REAL | 市值 |
| unrealized_pnl | REAL | 浮动盈亏 |

### daily_performance - 日度绩效
| 字段 | 类型 | 说明 |
|------|------|------|
| date | TEXT | 日期 |
| open_equity | REAL | 开盘权益 |
| close_equity | REAL | 收盘权益 |
| daily_pnl | REAL | 日盈亏 |
| daily_pnl_percent | REAL | 日盈亏比例 |
| trade_count | INTEGER | 交易次数 |

## 安全注意事项

### ⚠️ 重要警告

1. **实盘交易涉及真实资金**：请在充分测试后才使用实盘交易功能。

2. **风险机制仅供参考**：风险管理机制不能保证100%避免亏损。

3. **测试优先**：
   - 先使用小资金测试
   - 理解所有风险参数
   - 模拟交易确认无误后再实盘

4. **配置安全**：
   - 不要将 `.env` 文件提交到版本控制
   - 定期更改券商密码
   - 使用强密码

5. **监控交易**：
   - 定期检查交易记录
   - 关注账户余额变化
   - 设置报警通知

6. **合规性**：
   - 遵守相关法律法规
   - 了解券商交易规则
   - 注意交易费用和税收

## 故障排查

### 问题：无法连接券商

**检查清单**：
- [ ] 券商客户端是否已安装并运行
- [ ] easytrader 服务是否启动（端口8888）
- [ ] 用户名密码是否正确
- [ ] config.yaml 中的 service_url 是否正确

**解决方案**：
```bash
# 检查 easytrader 服务
curl http://localhost:8888/health

# 查看 easytrader 服务日志
docker-compose logs easytrader
```

### 问题：订单提交失败

**可能原因**：
1. 资金不足
2. 超过风险限制
3. 券商未连接
4. 价格不在有效范围

**调试步骤**：
```bash
# 检查账户余额
curl http://localhost:8080/api/trading/balance

# 检查风险状态
curl http://localhost:8080/api/trading/risk

# 查看日志
docker-compose logs cloudquant
```

### 问题：止损未触发

**检查**：
- 自动交易是否已启动
- 持仓数据是否正确更新
- 风险参数配置是否正确

## 性能优化

1. **数据同步**：合理设置 check_interval，避免频繁请求
2. **缓存策略**：broker_connector 已实现本地缓存
3. **并发控制**：订单执行采用串行处理，保证一致性

## 下一步计划

- [ ] 添加更多技术指标
- [ ] 支持更多券商
- [ ] 添加图表可视化
- [ ] 实现策略回测功能
- [ ] 添加通知推送（邮件/微信）
- [ ] 支持多账户管理

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

请参阅 [LICENSE](LICENSE) 文件。
