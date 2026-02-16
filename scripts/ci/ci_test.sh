#!/bin/bash
# CI测试脚本
# 功能：代码格式检查、静态分析、单元测试、覆盖率检查、构建验证

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

log_info "Starting CI test pipeline..."

# 1. 代码格式检查
log_info "Step 1: Checking code format..."
if ! command -v gofmt &> /dev/null; then
    log_warn "gofmt not found, skipping format check"
else
    FORMAT_OUTPUT=$(gofmt -l .)
    if [ -n "$FORMAT_OUTPUT" ]; then
        log_error "Code formatting issues found:"
        echo "$FORMAT_OUTPUT"
        log_info "Run 'gofmt -w .' to fix formatting issues"
        exit 1
    fi
    log_info "Code format check passed"
fi

# 2. 静态分析
log_info "Step 2: Running static analysis..."

# go vet
log_info "Running go vet..."
if ! go vet ./...; then
    log_error "go vet failed"
    exit 1
fi

# staticcheck (如果安装)
if command -v staticcheck &> /dev/null; then
    log_info "Running staticcheck..."
    if ! staticcheck ./...; then
        log_error "staticcheck failed"
        exit 1
    fi
else
    log_warn "staticcheck not found, skipping"
fi

# 3. 单元测试
log_info "Step 3: Running unit tests..."

# 检查race detector支持
if [ "$GOOS" = "linux" ] || [ "$GOOS" = "darwin" ]; then
    log_info "Running tests with race detector..."
    TEST_OUTPUT=$(go test -race -v ./... 2>&1 || true)
    if echo "$TEST_OUTPUT" | grep -q "DATA RACE"; then
        log_error "Race condition detected!"
        echo "$TEST_OUTPUT" | grep -A 10 "DATA RACE"
        exit 1
    fi
else
    log_info "Running tests without race detector..."
    if ! go test -v ./...; then
        log_error "Unit tests failed"
        exit 1
    fi
fi

# 4. 覆盖率检查
log_info "Step 4: Checking test coverage..."

COVERAGE_FILE="coverage.out"
go test -coverprofile="$COVERAGE_FILE" -covermode=atomic ./...

# 检查覆盖率
COVERAGE=$(go tool cover -func="$COVERAGE_FILE" | grep total | awk '{print $3}' | sed 's/%//')
log_info "Total coverage: ${COVERAGE}%"

COVERAGE_THRESHOLD=70
if (( $(echo "$COVERAGE < $COVERAGE_THRESHOLD" | bc -l) )); then
    log_error "Coverage ${COVERAGE}% is below threshold ${COVERAGE_THRESHOLD}%"
    exit 1
fi

log_info "Coverage check passed (${COVERAGE}% >= ${COVERAGE_THRESHOLD}%)"

# 5. 构建验证
log_info "Step 5: Validating build..."

# 构建主程序
BUILD_OUTPUT=$(go build -o /tmp/cloudquant_test . 2>&1)
if [ $? -ne 0 ]; then
    log_error "Build failed:"
    echo "$BUILD_OUTPUT"
    exit 1
fi
log_info "Build successful"

# 多平台构建检查
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"
log_info "Checking multi-platform build..."

for PLATFORM in $PLATFORMS; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}

    log_info "Building for $GOOS/$GOARCH..."
    GOOS=$GOOS GOARCH=$GOARCH go build -o /tmp/cloudquant_${GOOS}_${GOARCH} .

    if [ $? -ne 0 ]; then
        log_error "Failed to build for $GOOS/$GOARCH"
        exit 1
    fi
done

log_info "Multi-platform build check passed"

# 6. 安全检查
log_info "Step 6: Running security checks..."

# gosec (如果安装)
if command -v gosec &> /dev/null; then
    log_info "Running gosec..."
    gosec -quiet ./...
    if [ $? -ne 0 ]; then
        log_warn "gosec found security issues, but continuing"
    fi
else
    log_warn "gosec not found, skipping security check"
fi

# 7. 依赖检查
log_info "Step 7: Checking dependencies..."

# 检查过期的依赖
OUTDATED=$(go list -u -m all 2>&1 | grep "\[" || true)
if [ -n "$OUTDATED" ]; then
    log_warn "Some dependencies may be outdated:"
    echo "$OUTDATED"
fi

# 检查漏洞
if command -v govulncheck &> /dev/null; then
    log_info "Running govulncheck..."
    govulncheck ./...
    if [ $? -ne 0 ]; then
        log_error "Vulnerabilities found!"
        exit 1
    fi
else
    log_warn "govulncheck not found, skipping vulnerability check"
fi

# 8. 代码复杂度检查（可选）
log_info "Step 8: Checking code complexity..."

if command -v gocyclo &> /dev/null; then
    COMPLEXITY_OUTPUT=$(gocyclo -over 15 . 2>&1 || true)
    if [ -n "$COMPLEXITY_OUTPUT" ]; then
        log_warn "Functions with high cyclomatic complexity found:"
        echo "$COMPLEXITY_OUTPUT"
    fi
else
    log_warn "gocyclo not found, skipping complexity check"
fi

# 9. 生成覆盖率报告
log_info "Step 9: Generating coverage report..."

if command -v go tool cover &> /dev/null; then
    go tool cover -html="$COVERAGE_FILE" -o coverage.html
    log_info "Coverage report generated: coverage.html"
fi

# 10. 整理
log_info "Step 10: Cleanup..."

rm -f /tmp/cloudquant_test
rm -f /tmp/cloudquant_*

log_info "All CI checks passed successfully!"
log_info "=========================="
log_info "Coverage: ${COVERAGE}%"
log_info "=========================="

exit 0
