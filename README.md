# CloudQuantBot 精简版

CloudQuantBot 是一个轻量级的A股量化交易基础系统，支持行情获取、技术指标计算和 API 查询。

## 核心功能
- **行情获取**：支持从新浪财经获取实时行情和历史 K 线数据。
- **技术指标**：实现 MA、RSI、MACD 等核心技术指标。
- **数据存储**：使用 SQLite 存储 K 线及指标数据。
- **HTTP API**：提供健康检查、实时价格、技术指标和 K 线数据的 API。

## 快速开始

### 依赖
- Go 1.22+
- Docker & Docker Compose (可选)

### 本地运行
1. 安装依赖：
   ```bash
   go mod tidy
   ```
2. 运行程序：
   ```bash
   go run main.go
   ```

### Docker 运行
```bash
docker-compose up --build
```

## API 说明

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

## 项目结构
- `cmd/`: 主程序入口、训练脚本。
- `market/`: 行情获取与指标计算。
- `http/`: HTTP 服务器与路由处理。
- `db/`: 数据库操作。
- `llm/`: DeepSeek LLM 集成。
- `ml/`: 特征工程与模型实现。
- `data/`: 存储 SQLite 数据库文件。

## DeepSeek 配置
- 在 `config.yaml` 中配置 `llm.api_key` 或通过环境变量 `DEEPSEEK_API_KEY` 注入。
- 模型默认使用 `deepseek-chat`。

## 模型训练
使用训练脚本生成模型：
```bash
go run ./cmd/train_model --symbol=sh600000 --days=500
```

训练完成后模型将保存至 `config.yaml` 中的 `ml.model_path`。
