#!/bin/bash
# 构建脚本
# 支持多平台构建

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 版本信息
VERSION=${VERSION:-"1.0.0"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 构建标志
LDFLAGS="-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# 支持的平台
PLATFORMS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

log_info "Building CloudQuantBot v${VERSION}"
log_info "Build time: ${BUILD_TIME}"
log_info "Git commit: ${GIT_COMMIT}"
log_info "=========================="

# 创建输出目录
DIST_DIR="${PROJECT_ROOT}/dist"
mkdir -p "${DIST_DIR}"

# 构建每个平台
for PLATFORM in $PLATFORMS; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT_NAME="cloudquant-${GOOS}-${GOARCH}"

    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi

    log_info "Building for ${GOOS}/${GOARCH}..."

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "${LDFLAGS}" \
        -o "${DIST_DIR}/${OUTPUT_NAME}" \
        .

    if [ $? -ne 0 ]; then
        log_error "Failed to build for ${GOOS}/${GOARCH}"
        exit 1
    fi

    # 计算文件大小
    SIZE=$(du -h "${DIST_DIR}/${OUTPUT_NAME}" | cut -f1)
    log_info "  ✓ ${OUTPUT_NAME} (${SIZE})"

    # 压缩（非Windows）
    if [ "$GOOS" != "windows" ]; then
        gzip -f "${DIST_DIR}/${OUTPUT_NAME}"
        if [ $? -eq 0 ]; then
            SIZE_GZ=$(du -h "${DIST_DIR}/${OUTPUT_NAME}.gz" | cut -f1)
            log_info "  ✓ ${OUTPUT_NAME}.gz (${SIZE_GZ})"
        fi
    fi
done

# 创建checksums
log_info "Generating checksums..."
cd "${DIST_DIR}"
sha256sum cloudquant-* > checksums.txt 2>/dev/null || true
cd "$PROJECT_ROOT"

# 构建Docker镜像
if command -v docker &> /dev/null; then
    log_info "Building Docker image..."

    DOCKER_TAG="cloudquant:${VERSION}"
    if [ -f "${PROJECT_ROOT}/Dockerfile" ]; then
        docker build -t "${DOCKER_TAG}" "${PROJECT_ROOT}"
        if [ $? -eq 0 ]; then
            log_info "  ✓ Docker image built: ${DOCKER_TAG}"

            # 额外的标签
            docker tag "${DOCKER_TAG}" "cloudquant:latest"
            log_info "  ✓ Docker image tagged: cloudquant:latest"
        else
            log_error "Docker build failed"
        fi
    else
        log_warn "Dockerfile not found, skipping Docker build"
    fi
else
    log_warn "Docker not found, skipping Docker build"
fi

log_info "=========================="
log_info "Build completed successfully!"
log_info "Output directory: ${DIST_DIR}"

exit 0
