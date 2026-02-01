# CloudQuantBot 精简版

CloudQuantBot 是一个轻量级的A股量化交易基础系统，支持行情获取、技术指标计算、AI/ML分析以及实盘交易。

## 核心功能
- **行情获取**：支持从新浪财经获取实时行情和历史 K 线数据。
- **技术指标**：实现 MA、RSI、MACD 等核心技术指标。
- **数据存储**：使用 SQLite 存储 K 线、指标、交易记录等数据。
- **HTTP API**：提供完整的 REST API 接口。
- **AI 分析**：集成 DeepSeek LLM 进行市场分析。
- **ML 预测**：决策树模型进行价格方向预测。
- **实盘交易**：集成 easytrader 支持实盘交易（Phase 3）。
- **风险管理**：严格头寸控制和止损机制（Phase 3）。

## 快速开始

### 依赖
- Go 1.22+
- Python 3.8+ (实盘交易)
- Docker & Docker Compose (可选)

### 本地运行（仅行情和分析功能）
1. 安装依赖：
   ```bash
   go mod tidy
   ```
2. 运行程序：
   ```bash
   go run main.go
   ```
服务将在 http://localhost:8080 启动

### Docker 运行（完整功能，包含实盘交易）
1. 创建 `.env` 文件配置环境变量：
   ```bash
   DEEPSEEK_API_KEY=your_deepseek_api_key
   BROKER_USERNAME=your_broker_username
   BROKER_PASSWORD=your_broker_password
   ```

2. 启动服务：
   ```bash
   docker-compose up --build
   ```
这将启动两个服务：
- Go API 服务：http://localhost:8080
- EasyTrader 服务：http://localhost:8888

### 启用实盘交易

在使用实盘交易功能前，需要：

1. 安装券商客户端（如银河证券）
2. 确保easytrader服务可以访问券商客户端
3. 配置 `config.yaml` 中的券商信息

**注意：** 实盘交易涉及真实资金，请在充分测试后谨慎使用！

## API 说明

### 基础 API

### 1. 健康检查
- **GET** `/api/health`
- **返回**：`{"status": "ok"}`

### 2. 获取实时行情
- **GET** `/api/tick/{symbol}`
- **示例**：`/api/tick/sh600000`

### 3. 获取技术指标
- **GET** `/api/indicators/{symbol}?days=30`
- **示例**：`/api/indicators/sh600000?days=30`

### 4. 获取 K 线数据
- **GET** `/api/klines/{symbol}?limit=100`
- **示例**：`/api/klines/sh600000?limit=100`

### 5. DeepSeek AI 分析
- **GET** `/api/analysis/{symbol}`
- **示例**：`/api/analysis/sh600000`

### 6. DeepSeek AI 批量分析
- **GET** `/api/analysis/batch?symbols=sh600000,sh601398`

### 7. 机器学习预测
- **GET** `/api/predict/{symbol}`

### 8. 触发模型训练
- **POST** `/api/train`

### 实盘交易 API (Phase 3)

### 9. 获取投资组合
- **GET** `/api/trading/portfolio`
- **返回**：持仓信息列表

### 10. 获取账户余额
- **GET** `/api/trading/balance`
- **返回**：账户余额信息

### 11. 买入股票
- **POST** `/api/trading/buy`
- **请求体**：
  ```json
  {
    "symbol": "sh600000",
    "price": 10.0,
    "amount": 1000.0
  }
  ```
- **返回**：订单ID

### 12. 卖出股票
- **POST** `/api/trading/sell`
- **请求体**：
  ```json
  {
    "symbol": "sh600000",
    "price": 10.0,
    "quantity": 100
  }
  ```
- **返回**：订单ID

### 13. 撤销委托
- **POST** `/api/trading/cancel`
- **请求体**：
  ```json
  {
    "order_id": "order_id_here"
  }
  ```

### 14. 获取订单历史
- **GET** `/api/trading/orders?limit=50`
- **返回**：订单列表

### 15. 获取成交记录
- **GET** `/api/trading/trades?limit=50`
- **返回**：成交记录列表

### 16. 获取绩效统计
- **GET** `/api/trading/performance`
- **返回**：
  ```json
  {
    "total_return": 0.05,
    "daily_avg_return": 0.001,
    "max_drawdown": 0.02,
    "win_rate": 0.6,
    "profit_factor": 1.5,
    "sharpe_ratio": 1.2
  }
  ```

### 17. 获取日度盈亏
- **GET** `/api/trading/daily_pnl?days=30`
- **返回**：日度盈亏数据

### 18. 获取风险指标
- **GET** `/api/trading/risk`
- **返回**：当前风险状态

### 19. 启动自动交易
- **POST** `/api/trading/auto_trade/start`

### 20. 停止自动交易
- **POST** `/api/trading/auto_trade/stop`

### 21. 查看自动交易状态
- **GET** `/api/trading/auto_trade/status`

## 项目结构
```
cloudquant/
├── cmd/                      # 命令行工具
├── market/                   # 行情获取与指标计算
├── http/                     # HTTP 服务器与路由处理
│   ├── handlers.go           # 基础API处理器
│   ├── trading_handlers.go   # 交易API处理器
│   └── server.go             # 服务器
├── db/                       # 数据库操作
├── llm/                      # DeepSeek LLM 集成
├── ml/                       # 特征工程与模型实现
├── trading/                  # 实盘交易模块 (Phase 3)
│   ├── broker_interface.go   # 券商接口定义
│   ├── easytrader_broker.go  # EasyTrader客户端
│   ├── broker_connector.go   # 券商连接器
│   ├── risk_manager.go       # 风险管理器
│   ├── position_manager.go   # 持仓管理器
│   ├── order_executor.go     # 订单执行引擎
│   ├── signal_handler.go     # 信号处理器（AI+ML融合）
│   └── trade_history.go      # 交易历史记录
├── trading/broker_service/   # Python EasyTrader微服务
│   ├── easytrader_service.py
│   ├── requirements.txt
│   └── Dockerfile
├── data/                     # SQLite 数据库文件
├── models/                   # ML模型文件
├── config.yaml               # 配置文件
├── main.go                   # 主程序入口
├── docker-compose.yml        # Docker编排
└── Dockerfile               # Go服务镜像
```

## 配置说明

### config.yaml 配置项

```yaml
# 监控的股票
symbols:
  - sh600000
  - sh601398

# 数据库
database:
  path: "./data/quant.db"

# HTTP服务
http:
  port: 8080

# DeepSeek LLM
llm:
  provider: deepseek
  api_key: "${DEEPSEEK_API_KEY}"
  model: "deepseek-chat"
  timeout: 10s
  max_tokens: 200

# 机器学习
ml:
  model_type: "decision_tree"
  model_path: "./models/dt.model"
  max_tree_depth: 10

# 交易系统
trading:
  broker:
    type: "easytrader"
    service_url: "http://localhost:8888"
    broker_type: "yh"  # 银河证券，支持: ht(华泰), yh(银河), yjb(佣金宝)
    username: "${BROKER_USERNAME}"
    password: "${BROKER_PASSWORD}"

  risk:
    initial_capital: 100.0      # 初始资金
    max_single_position: 0.3    # 单只股票最多30%
    max_positions: 3           # 最多持仓3只
    max_daily_loss: 0.1        # 单日亏损10%全部平仓
    min_order_amount: 100.0    # 最小下单100元
    stop_loss_percent: 0.05    # 单只股票亏损5%止损

  auto_trade:
    enabled: false             # 是否启用自动交易
    check_interval: "1m"       # 检查间隔
    ai_threshold: 0.7          # AI信号阈值
    ml_confidence: 0.6         # ML置信度阈值
```

## 风险管理说明

本系统实现了严格的风险控制机制：

1. **单只股票仓位限制**：最多投入总资金的30%
2. **持仓数量限制**：最多同时持仓3只股票
3. **单日止损**：当日亏损达到10%时自动全部平仓
4. **单股止损**：单只股票亏损达到5%时触发止损卖出
5. **最小下单金额**：单笔订单至少100元

## 支持的券商

通过 easytrader 支持：
- 华泰证券 (ht)
- 银河证券 (yh) - 推荐，最稳定
- 佣金宝 (yjb)
- 雪球 (xq)

## DeepSeek 配置
- 在 `config.yaml` 中配置 `llm.api_key` 或通过环境变量 `DEEPSEEK_API_KEY` 注入。
- 模型默认使用 `deepseek-chat`。

## 模型训练
使用训练脚本生成模型：
```bash
go run ./cmd/train_model --symbol=sh600000 --days=500
```

训练完成后模型将保存至 `config.yaml` 中的 `ml.model_path`。

## 免责声明

**重要提示：**

1. 本系统仅供学习和研究使用，不构成任何投资建议。
2. 股市有风险，投资需谨慎。使用本系统进行实盘交易的所有风险由使用者自行承担。
3. 开发者不对使用本系统造成的任何损失负责。
4. 在进行实盘交易前，请务必：
   - 充分测试系统功能
   - 理解风险管理规则
   - 使用小额资金进行模拟测试
   - 遵守相关法律法规

## License

请参阅 [LICENSE](LICENSE) 文件。
