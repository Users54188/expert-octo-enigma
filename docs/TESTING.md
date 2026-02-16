# CloudQuantBot 测试指南

本文档提供CloudQuantBot的完整测试指南，包括单元测试、集成测试、性能测试和覆盖率要求。

## 目录

- [测试概述](#测试概述)
- [环境准备](#环境准备)
- [单元测试](#单元测试)
- [集成测试](#集成测试)
- [性能测试](#性能测试)
- [测试覆盖率](#测试覆盖率)
- [Mock系统](#mock系统)
- [CI/CD测试](#cicd测试)

## 测试概述

CloudQuantBot采用多层测试策略：

1. **单元测试**: 测试单个函数和方法
2. **集成测试**: 测试模块间的交互
3. **性能测试**: 测试系统性能和资源使用
4. **端到端测试**: 测试完整的工作流

### 测试覆盖率要求

- 总体覆盖率: ≥ 70%
- 核心模块覆盖率: ≥ 80%
- 业务逻辑覆盖率: ≥ 75%

## 环境准备

### 安装测试工具

```bash
# 安装Go
go version  # 需要 Go 1.22+

# 安装测试工具
go install github.com/golang/mock/mockgen@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 配置测试环境

```bash
# 复制测试配置
cp config.yaml config.test.yaml

# 修改测试配置
# - 使用测试数据库
# - 使用Mock数据源
# - 禁用真实交易
```

## 单元测试

### 运行所有单元测试

```bash
# 运行所有测试
go test ./...

# 运行带详细输出的测试
go test -v ./...

# 运行特定包的测试
go test -v ./trading/risk/...

# 运行特定测试函数
go test -v ./trading/risk/... -run TestRiskManager
```

### 并发测试（Race Detector）

```bash
# 运行race检测
go test -race ./...

# 生成race报告
go test -race ./... 2> race_report.txt
```

### 基准测试

```bash
# 运行所有基准测试
go test -bench=. ./...

# 运行特定基准测试
go test -bench=BenchmarkOrderManager ./trading/order/...

# 基准测试带内存分析
go test -bench=. -benchmem ./...
```

### 编写单元测试示例

```go
// trading/risk/realtime/realtime_risk_test.go
package realtime

import (
    "testing"
    "time"
)

func TestRealtimeRiskMonitor_Start(t *testing.T) {
    // 准备测试数据
    config := MonitorConfig{
        CheckInterval: 1 * time.Second,
        MaxEventHistory: 100,
    }

    monitor := NewRealtimeRiskMonitor(nil, nil, config)

    // 测试启动
    err := monitor.Start()
    if err != nil {
        t.Fatalf("Failed to start monitor: %v", err)
    }

    // 清理
    monitor.Stop()
}

func TestRealtimeRiskMonitor_CheckPositionRisk(t *testing.T) {
    // 准备测试环境
    // ...

    // 执行测试
    err := monitor.checkPositionRisk(ctx)

    // 验证结果
    if err != nil {
        t.Errorf("checkPositionRisk failed: %v", err)
    }
}
```

## 集成测试

### 运行集成测试

```bash
# 运行所有集成测试
go test ./tests/integration/... -v

# 运行特定集成测试
go test ./tests/integration/api/... -v

# 带测试环境的集成测试
go test ./tests/integration/... -tags=integration
```

### API集成测试

```bash
# 启动测试服务器
go run main.go --config config.test.yaml &

# 等待服务启动
sleep 5

# 运行API测试
go test ./tests/integration/api/... -v

# 停止服务器
pkill -f "main.go --config"
```

### 数据库集成测试

```bash
# 运行数据库测试
go test ./tests/integration/database/... -v

# 使用SQLite内存数据库
TEST_DB_PATH=":memory:" go test ./tests/integration/database/... -v
```

## 性能测试

### 基准测试

```bash
# 运行所有基准测试
go test -bench=. -benchmem ./...

# 运行特定模块的基准测试
go test -bench=BenchmarkOrderManager -benchmem ./trading/order/...

# 持续运行基准测试（用于稳定性）
go test -bench=. -benchtime=10x ./...
```

### 压力测试

```bash
# 使用Apache Bench进行HTTP压力测试
ab -n 10000 -c 100 http://localhost:8080/api/v1/health

# 使用wrk进行并发测试
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/market/sh600000
```

### 性能分析

```bash
# CPU性能分析
go test -cpuprofile=cpu.prof -bench=. ./trading/order/...
go tool pprof cpu.prof

# 内存性能分析
go test -memprofile=mem.prof -bench=. ./trading/order/...
go tool pprof mem.prof

# 生成可视化报告
go tool pprof -http=:8081 cpu.prof
```

## 测试覆盖率

### 生成覆盖率报告

```bash
# 生成覆盖率文件
go test -coverprofile=coverage.out -covermode=atomic ./...

# 查看覆盖率摘要
go tool cover -func=coverage.out | grep total

# 生成HTML报告
go tool cover -html=coverage.out -o coverage.html

# 在浏览器中查看
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

### 覆盖率目标

| 模块 | 目标覆盖率 |
|------|-----------|
| 核心交易模块 | 80%+ |
| 风险管理 | 75%+ |
| 监控系统 | 70%+ |
| 数据管道 | 70%+ |
| 订单管理 | 80%+ |
| 工具函数 | 85%+ |

### 增量覆盖率检查

```bash
# 检查增量覆盖率
git diff HEAD~1 | diff-cover --coverage-file=coverage.out -
```

## Mock系统

### 使用Mock数据

```go
// 创建Mock数据提供者
mockProvider := NewMockDataProvider()

// 设置Mock数据
mockProvider.SetMarketData("sh600000", &MarketData{
    Symbol:    "sh600000",
    Price:     10.50,
    Volume:    1000000,
    Timestamp: time.Now().Unix(),
})

// 使用Mock提供者
service := NewMarketService(mockProvider)
```

### 生成Mock

```bash
# 生成接口的Mock
mockgen -source=trading/order/manager.go -destination=tests/mocks/order_manager_mock.go
```

### 测试数据生成器

```go
// 生成测试数据
generator := NewTestDataGenerator()

// 生成市场数据
marketData := generator.GenerateMarketData("sh600000", 100)

// 生成订单
order := generator.GenerateOrder("buy", "sh600000", 100, 10.50)
```

## CI/CD测试

### 本地预提交检查

```bash
# 运行CI测试脚本
./scripts/ci/ci_test.sh

# 只运行快速检查
./scripts/ci/ci_test.sh --fast
```

### GitHub Actions测试

测试在GitHub Actions中自动运行：
- Push到main/develop分支
- 创建Pull Request
- 创建Release

测试步骤：
1. 代码格式检查
2. 静态分析（go vet, golangci-lint）
3. 单元测试（带race detector）
4. 覆盖率检查（≥ 70%）
5. 安全检查（govulncheck）
6. 多平台构建验证

## 测试最佳实践

### 1. 表驱动测试

```go
func TestCalculatePosition(t *testing.T) {
    tests := []struct {
        name     string
        input    float64
        expected float64
    }{
        {"positive", 100.0, 100.0},
        {"zero", 0.0, 0.0},
        {"negative", -100.0, 0.0},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := CalculatePosition(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### 2. 使用testify

```go
import (
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestWithTestify(t *testing.T) {
    assert.NotNil(t, manager)
    assert.Equal(t, 100.0, manager.Capital)

    require.NoError(t, err)  // 立即失败
}
```

### 3. 子测试

```go
func TestOrderManager(t *testing.T) {
    t.Run("SubmitOrder", func(t *testing.T) {
        // 测试提交订单
    })

    t.Run("CancelOrder", func(t *testing.T) {
        // 测试取消订单
    })
}
```

### 4. 测试清理

```go
func TestWithCleanup(t *testing.T) {
    // 设置
    db := setupTestDB()
    t.Cleanup(func() {
        db.Close()
    })

    // 测试...
}
```

## 常见问题

### Q: 测试失败如何调试？

```bash
# 使用详细输出
go test -v ./...

# 使用调试器
dlv test ./trading/risk/... -test.run TestRiskManager
```

### Q: 如何处理外部依赖？

使用Mock或测试桩（test doubles）来隔离外部依赖。

### Q: 性能测试不稳定？

增加运行次数或使用内存缓存预热。

## 相关文档

- [开发指南](DEVELOPMENT.md)
- [生产部署](PRODUCTION.md)
- [API文档](API.md)
- [架构设计](ARCHITECTURE.md)
