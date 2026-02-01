# 更新日志

## Phase 3: 实盘交易与风险管理系统

### 新增功能

#### 1. Python EasyTrader 微服务
- **文件**: `trading/broker_service/easytrader_service.py`
- **功能**:
  - 基于 FastAPI 的 REST API 服务（端口 8888）
  - 支持多券商登录/登出
  - 买入/卖出/撤单操作
  - 查询持仓、余额、委托、成交
  - 自动登录支持
  - 健康检查接口

#### 2. Go 交易核心模块
- **broker_interface.go**: 券商接口抽象定义
- **easytrader_broker.go**: EasyTrader HTTP 客户端实现
- **broker_connector.go**: 券商连接管理器（工厂、重试、心跳）
- **risk_manager.go**: 风险管理器（仓位控制、止损）
- **position_manager.go**: 持仓管理器（成本计算、盈亏追踪）
- **order_executor.go**: 订单执行引擎
- **signal_handler.go**: AI+ML 信号融合决策
- **trade_history.go**: 交易历史记录与绩效计算

#### 3. HTTP API 扩展
- **文件**: `http/trading_handlers.go`
- **新增接口**:
  - GET `/api/trading/portfolio` - 投资组合
  - GET `/api/trading/balance` - 账户余额
  - POST `/api/trading/buy` - 下买单
  - POST `/api/trading/sell` - 下卖单
  - POST `/api/trading/cancel` - 撤单
  - GET `/api/trading/orders` - 订单历史
  - GET `/api/trading/trades` - 成交记录
  - GET `/api/trading/performance` - 绩效统计
  - GET `/api/trading/daily_pnl` - 日度盈亏
  - GET `/api/trading/risk` - 风险指标
  - POST `/api/trading/auto_trade/start` - 启动自动交易
  - POST `/api/trading/auto_trade/stop` - 停止自动交易
  - GET `/api/trading/auto_trade/status` - 自动交易状态

#### 4. 数据库扩展
- **文件**: `db/sqlite.go`
- **新增表**:
  - `trades` - 成交记录
  - `orders` - 委托记录
  - `positions` - 持仓记录
  - `daily_performance` - 日度绩效

#### 5. 配置文件更新
- **文件**: `config.yaml`
- **新增配置节**:
  - `trading.broker` - 券商配置
  - `trading.risk` - 风险管理配置
  - `trading.auto_trade` - 自动交易配置

#### 6. Docker 支持
- **文件**: `docker-compose.yml`
- **服务**:
  - cloudquant (Go API) - 端口 8080
  - easytrader (Python) - 端口 8888

#### 7. 辅助脚本
- `start.sh` - 一键启动脚本
- `test_api.sh` - API 测试脚本
- `.env.example` - 环境变量模板

#### 8. 文档
- `README.md` - 更新完整文档
- `PHASE3.md` - Phase 3 详细说明文档
- `CHANGELOG.md` - 更新日志

### 风险管理规则

1. **初始资金**: 100元
2. **单只股票仓位**: 最多30%
3. **最大持仓数**: 3只股票
4. **单日止损**: 亏损10%时全部平仓
5. **单股止损**: 亏损5%时触发止损
6. **最小下单金额**: 100元

### 支持的券商

- 华泰证券 (ht)
- 银河证券 (yh) - 推荐
- 佣金宝 (yjb)
- 雪球 (xq)

### 绩效指标

- 总收益率
- 日均收益率
- 最大回撤
- 胜率
- 盈亏比
- 夏普比率

### 文件清单

```
trading/
├── broker_interface.go       (新增)
├── easytrader_broker.go      (新增)
├── broker_connector.go        (新增)
├── risk_manager.go           (新增)
├── position_manager.go       (新增)
├── order_executor.go         (新增)
├── signal_handler.go         (新增)
└── trade_history.go          (新增)

trading/broker_service/
├── easytrader_service.py     (新增)
├── requirements.txt          (新增)
└── Dockerfile               (新增)

http/
└── trading_handlers.go       (新增)

db/
└── sqlite.go                (修改 - 添加交易相关表)

config.yaml                  (修改 - 添加trading配置)
docker-compose.yml           (修改 - 添加easytrader服务)
main.go                     (修改 - 初始化交易系统)
README.md                   (修改 - 更新文档)
.gitignore                  (修改 - 添加.env等)
```

### 使用方式

#### Docker 方式
```bash
# 1. 配置环境变量
cp .env.example .env
vim .env

# 2. 启动服务
docker-compose up --build
```

#### 本地方式
```bash
# 1. 启动 easytrader 服务
cd trading/broker_service
pip install -r requirements.txt
python easytrader_service.py

# 2. 启动 Go 服务
go run main.go
```

### API 测试

```bash
# 测试所有API
./test_api.sh
```

### 免责声明

- 本系统仅供学习研究使用
- 实盘交易涉及真实资金，请谨慎使用
- 开发者不对任何损失负责
- 使用前请充分测试

---

## 之前版本

### Phase 2: DeepSeek AI 和 ML 集成
- DeepSeek LLM 分析
- 决策树模型
- 特征工程

### Phase 1: 基础行情系统
- 实时行情获取
- 技术指标计算
- SQLite 存储
- HTTP API
