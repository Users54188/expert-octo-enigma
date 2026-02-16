# CloudQuantBot 架构设计文档

## 1. 系统概述

CloudQuantBot是一个生产级量化交易系统，设计用于在6c8g本地计算环境中运行，支持A股市场量化交易。

### 1.1 设计目标

- **高性能**: 支持实时行情处理和交易执行
- **可扩展**: 模块化设计，便于功能扩展
- **可靠性**: 多重数据备份和故障转移
- **易用性**: 完善的API和可视化界面

### 1.2 技术栈

- **语言**: Go 1.22+
- **数据库**: SQLite3 (本地优化)
- **WebSocket**: Gorilla WebSocket
- **缓存**: LRU Cache
- **日志**: Uber Zap

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        HTTP Server Layer                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│  │ REST API    │ │ WebSocket   │ │ Dashboard   │ │ Middleware │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                       Business Logic Layer                       │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│  │ Trading     │ │ Risk        │ │ ML/AI       │ │ Backtest   │ │
│  │ Engine      │ │ Management  │ │ Analysis    │ │ Engine     │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                        Data Access Layer                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│  │ Market      │ │ Industry    │ │ Position    │ │ History    │ │
│  │ Data        │ │ Data        │ │ Manager     │ │ Records    │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────────┐
│                      External Data Layer                         │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌────────────┐ │
│  │ Sina        │ │ EastMoney   │ │ Tencent     │ │ Mock       │ │
│  │ Provider    │ │ Provider    │ │ Provider    │ │ Provider   │ │
│  └─────────────┘ └─────────────┘ └─────────────┘ └────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 模块说明

#### 2.2.1 HTTP Server Layer

- **REST API**: 提供标准的RESTful API接口
- **WebSocket**: 实时数据推送，支持仪表板和回放
- **Dashboard**: 可视化数据展示
- **Middleware**: 日志、恢复、CORS、超时等中间件

#### 2.2.2 Business Logic Layer

- **Trading Engine**: 交易执行引擎，支持多策略
- **Risk Management**: 风险管理和控制
- **ML/AI Analysis**: 机器学习和AI分析
- **Backtest Engine**: 回测和参数优化

#### 2.2.3 Data Access Layer

- **Market Data**: 市场数据缓存和管理
- **Industry Data**: 行业分类和暴露分析
- **Position Manager**: 持仓管理
- **History Records**: 历史记录存储

#### 2.2.4 External Data Layer

- **Sina Provider**: 新浪财经数据源
- **EastMoney Provider**: 东方财富数据源
- **Tencent Provider**: 腾讯财经数据源
- **Mock Provider**: Mock数据（开发/测试使用）

## 3. 数据流

### 3.1 实时行情数据流

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Provider   │────▶│   Manager    │────▶│    Cache     │────▶│   API/WS     │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       │                    │                    │
       ▼                    ▼                    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ Health Check │     │ Failover     │     │ Update       │
└──────────────┘     └──────────────┘     └──────────────┘
```

### 3.2 交易执行数据流

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│   Signal     │────▶│ Risk Check   │────▶│  Position    │────▶│   Order      │
│   Generator  │     │              │     │  Manager     │     │   Executor   │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
       │                    │                    │                    │
       ▼                    ▼                    ▼                    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ ML/AI Model  │     │ VaR Check    │     │ Size Limit   │     │ Broker API   │
└──────────────┘     └──────────────┘     └──────────────┘     └──────────────┘
```

## 4. 核心组件

### 4.1 数据源管理器

数据源管理器实现了多数据源的自动切换和故障转移。

```go
type ProviderManager struct {
    providers    []DataProvider  // 所有数据源
    primary      DataProvider    // 主数据源
    health       map[string]bool // 健康状态
    failoverEnabled bool         // 故障转移开关
}
```

### 4.2 风险管理器

风险管理器包含多个维度的风险控制：

- **Portfolio Risk**: 组合风险，行业/板块暴露
- **Volatility Risk**: 波动率风险
- **Cooldown Risk**: 交易冷却和频率控制
- **AI Risk**: AI智能风险分析

### 4.3 WebSocket管理器

WebSocket管理器支持：

- 最大100个并发连接
- 每连接256个消息缓冲区
- 30秒心跳检测
- 消息序列号防丢包

### 4.4 回放引擎

回放引擎支持：

- 历史数据回放
- 多倍速播放 (1x, 2x, 5x, 10x)
- 信号和事件记录
- 实时状态广播

## 5. 6c8g优化配置

### 5.1 SQLite优化

```yaml
database:
  driver: "sqlite3"
  dsn: "./data/quant.db?_journal_mode=WAL&_busy_timeout=5000&_cache_size=10000"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h
```

### 5.2 WebSocket优化

```yaml
websocket:
  enabled: true
  max_connections: 100
  message_buffer: 256
  heartbeat: 30s
```

### 5.3 内存优化

```yaml
cache:
  type: "lru"
  size: 10000
  ttl: 5m
```

### 5.4 协程控制

- 最大工作协程: 50
- 任务队列: 1000
- 超时控制: context.WithTimeout

### 5.5 日志优化

```yaml
log:
  level: "info"
  format: "json"
  file:
    max_size: 100      # MB
    max_age: 7         # days
    max_backups: 10
    compress: true
```

## 6. API设计

### 6.1 行业数据API

| 端点 | 方法 | 描述 |
|------|------|------|
| /api/industry/exposure | GET | 行业暴露分析 |
| /api/industry/rotation | GET | 板块轮动检测 |
| /api/industry/:symbol/info | GET | 个股行业信息 |
| /api/industry/benchmark | GET | 行业基准权重 |
| /api/industry/correlation | GET | 行业相关性矩阵 |

### 6.2 风险模型API

| 端点 | 方法 | 描述 |
|------|------|------|
| /api/risk/curve | GET | 资金曲线 |
| /api/risk/attribution | GET | 风险归因 |
| /api/risk/metrics | GET | 风险指标 |
| /api/risk/var | GET | VaR/CVaR分析 |
| /api/risk/factors | GET | 因子暴露 |
| /api/risk/report | POST | 生成风险报告 |

### 6.3 可视化API

| 端点 | 方法 | 描述 |
|------|------|------|
| /api/ws/dashboard | WS | 实时仪表板 |
| /api/ws/market | WS | 实时行情 |
| /api/visualization/equity | GET | 权益曲线数据 |
| /api/visualization/heatmap | GET | 收益热力图 |

### 6.4 回放API

| 端点 | 方法 | 描述 |
|------|------|------|
| /api/replay/start | POST | 开始回放 |
| /api/replay/pause | POST | 暂停回放 |
| /api/replay/resume | POST | 恢复回放 |
| /api/replay/stop | POST | 停止回放 |
| /api/replay/:id/status | GET | 回放状态 |

## 7. 部署架构

### 7.1 本地开发环境

```
┌─────────────────────────────────────┐
│         Local Machine (6c8g)        │
│  ┌─────────────────────────────┐    │
│  │   CloudQuantBot Service     │    │
│  │   - Port: 8080              │    │
│  │   - SQLite Database         │    │
│  │   - File Storage            │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
```

### 7.2 生产环境（建议）

```
┌─────────────────────────────────────┐
│         Load Balancer               │
└─────────────────────────────────────┘
              │
    ┌─────────┴─────────┐
    ▼                   ▼
┌──────────┐      ┌──────────┐
│ Instance │      │ Instance │
│    1     │      │    2     │
└──────────┘      └──────────┘
    │                   │
    └─────────┬─────────┘
              ▼
┌─────────────────────────────────────┐
│         Shared Storage              │
│   - PostgreSQL (TimescaleDB)        │
│   - Redis Cache                     │
│   - MinIO (Object Storage)          │
└─────────────────────────────────────┘
```

## 8. 安全设计

### 8.1 数据安全

- 敏感配置使用环境变量
- API密钥不存储在代码中
- 数据库使用WAL模式保证一致性

### 8.2 网络安全

- CORS控制
- 请求大小限制
- 速率限制
- 超时控制

### 8.3 运行安全

- 恢复中间件防止panic
- 资源使用限制
- 日志审计

## 9. 监控与告警

### 9.1 系统监控

- HTTP请求延迟
- WebSocket连接数
- 内存和CPU使用
- 数据库连接池

### 9.2 业务监控

- 交易成功率
- 风险指标
- 数据源健康
- 异常检测

### 9.3 告警渠道

- 飞书Webhook
- 钉钉Webhook
- 邮件通知

## 10. 扩展性设计

### 10.1 新增数据源

实现`DataProvider`接口即可添加新的数据源：

```go
type DataProvider interface {
    Name() string
    FetchTick(ctx context.Context, symbol string) (*Tick, error)
    FetchKLines(ctx context.Context, symbol string, days int) ([]KLine, error)
    HealthCheck() error
    Priority() int
}
```

### 10.2 新增策略

实现`Strategy`接口即可添加新的交易策略。

### 10.3 新增风险模型

继承风险基类并实现`Calculate()`方法。

## 11. 性能指标

### 11.1 目标性能

| 指标 | 目标值 |
|------|--------|
| API响应时间 | < 100ms (P95) |
| WebSocket延迟 | < 50ms |
| 数据库查询 | < 20ms |
| 数据获取延迟 | < 1s |
| 最大并发连接 | 100 |

### 11.2 资源使用

| 资源 | 限制 |
|------|------|
| CPU | 6 cores |
| Memory | 8 GB |
| Disk I/O | 根据数据量 |
| Network | 100 Mbps |

## 12. 开发指南

### 12.1 目录结构

```
cloudquant/
├── cmd/                # 命令行入口
├── http/               # HTTP服务器和处理器
├── market/             # 市场数据和行业数据
│   ├── industry/       # 行业数据模块
│   └── providers/      # 数据提供者
├── trading/            # 交易相关
│   ├── risk/           # 风险管理
│   ├── portfolio/      # 组合管理
│   └── strategies/     # 交易策略
├── monitoring/         # 监控和回放
├── ml/                 # 机器学习
├── llm/                # LLM分析
├── backtest/           # 回测系统
├── db/                 # 数据库
├── data/               # 数据文件
├── logs/               # 日志文件
├── scripts/            # 脚本文件
└── docs/               # 文档
```

### 12.2 代码规范

- 所有导出函数必须有中文注释
- 复杂逻辑必须有文档说明
- 错误处理必须完整
- 接口必须清晰
- 测试覆盖率>70%

### 12.3 测试策略

- 单元测试：覆盖核心功能
- 集成测试：测试模块间交互
- 性能测试：验证6c8g优化效果
- E2E测试：端到端功能验证
