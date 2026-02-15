# CloudQuantBot 本地运行指南

本文档详细介绍如何在本地环境中运行 CloudQuantBot，包括环境配置、依赖安装和常见问题解决。

## 目录

1. [系统要求](#系统要求)
2. [快速开始](#快速开始)
3. [详细配置](#详细配置)
4. [功能说明](#功能说明)
5. [常见问题](#常见问题)

## 系统要求

### 必需环境

- **Go**: 1.22 或更高版本
- **操作系统**: Linux, macOS, 或 Windows (WSL)
- **内存**: 至少 512MB RAM
- **磁盘空间**: 至少 500MB 可用空间

### 可选环境

- **Python**: 3.8+ (用于实盘交易)
- **Docker**: 用于容器化部署

## 快速开始

### 1. 克隆项目

```bash
git clone <repository_url>
cd cloudquant
```

### 2. 使用启动脚本（推荐）

最简单的方式是使用提供的启动脚本：

```bash
chmod +x scripts/start_local.sh
./scripts/start_local.sh
```

这个脚本会自动：
- 检查 Go 环境
- 创建必要的目录
- 生成默认配置文件
- 启动服务

### 3. 手动启动

如果需要更多控制，可以手动启动：

```bash
# 安装依赖
go mod tidy

# 运行服务
go run main.go
```

服务启动后，访问 http://localhost:8080 查看健康状态。

## 详细配置

### 环境变量

创建 `.env` 文件（可选）：

```bash
cp .env.example .env
# 编辑 .env 文件，填写配置
```

主要环境变量：

```bash
# 数据库路径
DB_PATH=./data/quant.db

# DeepSeek API Key（可选，用于AI分析）
DEEPSEEK_API_KEY=your_api_key_here

# 日志级别：debug, info, warn, error
LOG_LEVEL=info

# 本地模式配置
MODE=local
DATA_SOURCE_PRIMARY=mock
MOCK_DATA_ENABLED=true
```

### 配置文件

项目会自动创建 `config.local.yaml`，包含本地运行所需的所有配置：

```yaml
mode: "local"

database:
  path: "./data/quant.db"

http:
  port: 8080

data_source:
  primary: "mock"
  fallback: ["sina", "eastmoney", "tencent"]
  mock_on_failure: true

symbols:
  - sh600000
  - sh601398
  - sh600519
```

## 功能说明

### 1. Mock 数据源

本地模式默认使用 Mock 数据源，提供模拟的实时行情和K线数据：

- 支持 50+ 只热门股票的模拟数据
- 自动生成技术指标
- 支持历史K线数据

切换到真实数据源（需要网络）：

```yaml
data_source:
  primary: "sina"  # 或 "eastmoney", "tencent"
```

### 2. 行业分析

系统提供完整的行业分析功能：

- 股票行业信息查询
- 行业暴露分析
- 板块分类统计

示例：

```bash
# 查询股票行业
curl http://localhost:8080/api/stock/sh600000/industry

# 获取行业暴露
curl http://localhost:8080/api/portfolio/industry_exposure
```

### 3. 风险管理

提供多种风险指标：

- 资金曲线跟踪
- VaR/CVaR 计算
- 风险归因分析
- 回撤监控

示例：

```bash
# 获取资金曲线
curl http://localhost:8080/api/dashboard/equity?days=30

# 获取风险指标
curl http://localhost:8080/api/dashboard/risk
```

### 4. 性能监控

实时绩效分析：

```bash
# 绩效指标
curl http://localhost:8080/api/performance/metrics

# 交易统计
curl http://localhost:8080/api/performance/stats
```

### 5. 数据源管理

查看和管理数据源状态：

```bash
# 数据源状态
curl http://localhost:8080/api/market/providers

# 健康检查
curl http://localhost:8080/api/market/health
```

## 开发模式

使用热重载功能加速开发：

```bash
chmod +x scripts/start_dev.sh
./scripts/start_dev.sh
```

热重载需要安装 `air` 工具：

```bash
go install github.com/cosmtrek/air@latest
```

## API 测试

### 健康检查

```bash
curl http://localhost:8080/api/health
```

### 获取行情

```bash
# 实时行情
curl http://localhost:8080/api/tick/sh600000

# K线数据
curl http://localhost:8080/api/klines/sh600000?limit=30
```

### 技术指标

```bash
curl http://localhost:8080/api/indicators/sh600000?days=30
```

### AI 分析（需要配置 API Key）

```bash
curl http://localhost:8080/api/analysis/sh600000
```

## 常见问题

### 1. 端口被占用

如果 8080 端口被占用，修改配置：

```yaml
http:
  port: 8081  # 改为其他端口
```

### 2. 数据库权限问题

确保 `data` 目录有写权限：

```bash
chmod -R 755 data
```

### 3. Go 版本不兼容

检查 Go 版本：

```bash
go version
```

确保版本 >= 1.22。

### 4. Mock 数据不准确

Mock 数据仅为模拟，如需真实数据：

1. 配置网络连接
2. 修改 `config.local.yaml` 中的 `data_source.primary`
3. 重启服务

### 5. 依赖下载失败

使用 Go 镜像源：

```bash
go env -w GOPROXY=https://goproxy.cn,direct
go mod tidy
```

### 6. 编译错误

清理并重新编译：

```bash
go clean -modcache
go mod download
go build
```

## 日志和调试

### 查看日志

日志默认输出到控制台，包含详细的调试信息。

### 调整日志级别

在 `.env` 中设置：

```bash
LOG_LEVEL=debug
```

### 常见日志级别

- `debug`: 详细调试信息
- `info`: 一般信息（默认）
- `warn`: 警告信息
- `error`: 错误信息

## 性能优化

### 1. 减少监控股票

```yaml
symbols:
  - sh600000  # 只监控核心股票
```

### 2. 调整更新频率

```yaml
mock:
  update_interval: "5s"  # 默认 1s
```

### 3. 禁用不需要的功能

```yaml
monitoring:
  websocket:
    enabled: false  # 禁用 WebSocket
```

## 下一步

- 阅读 [README.md](README.md) 了解完整功能
- 配置实盘交易（需要券商账号）
- 部署到生产环境
- 开发自定义策略

## 技术支持

遇到问题？请查看：

1. 项目 Issues
2. 代码注释
3. API 文档
4. 社区论坛

## 免责声明

本系统仅供学习和研究使用，不构成任何投资建议。使用本系统进行实盘交易的所有风险由使用者自行承担。
