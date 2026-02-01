#!/bin/bash

# CloudQuantBot 启动脚本

set -e

echo "======================================"
echo "  CloudQuantBot 启动脚本"
echo "======================================"

# 检查.env文件
if [ ! -f .env ]; then
    echo "⚠️  未找到.env文件，从.env.example复制..."
    cp .env.example .env
    echo "✅ 已创建.env文件，请编辑并填入正确的配置！"
    echo ""
    echo "需要配置以下内容："
    echo "  - DEEPSEEK_API_KEY: DeepSeek API密钥"
    echo "  - BROKER_USERNAME: 券商用户名（如使用实盘交易）"
    echo "  - BROKER_PASSWORD: 券商密码（如使用实盘交易）"
    echo ""
    read -p "按回车键继续，或Ctrl+C退出编辑配置..."
fi

# 创建必要的目录
mkdir -p data models

# 检查是否使用Docker
if command -v docker &> /dev/null && command -v docker-compose &> /dev/null; then
    echo "🐳 检测到Docker环境，使用Docker启动..."
    echo ""

    # 检查docker-compose文件
    if [ -f docker-compose.yml ]; then
        docker-compose up --build
    else
        echo "❌ 未找到docker-compose.yml文件"
        exit 1
    fi
else
    echo "🚀 使用本地Go环境启动..."
    echo ""

    # 检查Go
    if ! command -v go &> /dev/null; then
        echo "❌ 未安装Go，请先安装Go 1.22+"
        exit 1
    fi

    # 安装依赖
    echo "📦 安装Go依赖..."
    go mod tidy

    # 运行
    echo "▶️  启动服务..."
    go run main.go
fi
