# CloudQuantBot 生产环境部署指南

本文档提供CloudQuantBot在生产环境中的部署、配置和运维指南。

## 目录

- [系统要求](#系统要求)
- [安装步骤](#安装步骤)
- [配置说明](#配置说明)
- [服务管理](#服务管理)
- [监控和维护](#监控和维护)
- [故障排查](#故障排查)
- [备份和恢复](#备份和恢复)
- [性能优化](#性能优化)

## 系统要求

### 硬件要求

| 资源 | 最小配置 | 推荐配置 |
|------|---------|---------|
| CPU | 4核 | 6核+ |
| 内存 | 4GB | 8GB |
| 存储 | 20GB SSD | 50GB SSD |
| 网络 | 100Mbps | 1Gbps |

### 软件要求

- 操作系统: Ubuntu 20.04 LTS / 22.04 LTS
- Go: 1.22+
- SQLite: 3.35+
- Docker (可选): 20.10+

## 安装步骤

### 1. 系统准备

```bash
# 更新系统
sudo apt-get update
sudo apt-get upgrade -y

# 安装必要工具
sudo apt-get install -y git curl wget vim htop net-tools

# 配置系统参数
sudo sysctl -w net.core.somaxconn=4096
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=4096
sudo sysctl -w vm.max_map_count=262144

# 持久化系统参数
echo "net.core.somaxconn=4096" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_max_syn_backlog=4096" | sudo tee -a /etc/sysctl.conf
echo "vm.max_map_count=262144" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### 2. 创建用户和目录

```bash
# 创建服务用户
sudo useradd -r -s /bin/false cloudquant
sudo mkdir -p /opt/cloudquant
sudo mkdir -p /var/log/cloudquant
sudo mkdir -p /etc/cloudquant

# 设置权限
sudo chown -R cloudquant:cloudquant /opt/cloudquant
sudo chown -R cloudquant:cloudquant /var/log/cloudquant
sudo chmod 755 /opt/cloudquant
sudo chmod 755 /var/log/cloudquant
```

### 3. 安装CloudQuantBot

```bash
# 下载最新版本
cd /opt/cloudquant
wget https://github.com/yourorg/cloudquant/releases/latest/download/cloudquant-linux-amd64.tar.gz

# 解压
tar -xzf cloudquant-linux-amd64.tar.gz
chmod +x cloudquant

# 或者从源码构建
git clone https://github.com/yourorg/cloudquant.git
cd cloudquant
go build -o cloudquant .
```

### 4. 配置文件

```bash
# 复制配置文件
sudo cp config/config.production.yaml /etc/cloudquant/config.yaml

# 创建环境变量文件
sudo touch /etc/cloudquant/cloudquant.env
sudo chmod 600 /etc/cloudquant/cloudquant.env

# 编辑环境变量
sudo vim /etc/cloudquant/cloudquant.env

# 添加以下内容（根据实际情况修改）
DEEPSEEK_API_KEY=your_api_key_here
BROKER_USERNAME=your_broker_username
BROKER_PASSWORD=your_broker_password
FEISHU_WEBHOOK=your_feishu_webhook_url
DINGDING_WEBHOOK=your_dingding_webhook_url
```

### 5. 数据库初始化

```bash
# 创建数据目录
sudo mkdir -p /opt/cloudquant/data
sudo chown cloudquant:cloudquant /opt/cloudquant/data

# 初始化数据库（自动）
sudo -u cloudquant /opt/cloudquant/cloudquant --init-db

# 验证
ls -la /opt/cloudquant/data/
```

### 6. 安装systemd服务

```bash
# 复制服务文件
sudo cp systemd/cloudquant.service /etc/systemd/system/

# 重载systemd
sudo systemctl daemon-reload

# 启用服务
sudo systemctl enable cloudquant

# 启动服务
sudo systemctl start cloudquant

# 查看状态
sudo systemctl status cloudquant
```

## 配置说明

### 生产配置详解

#### 数据库优化

```yaml
database:
  driver: "sqlite3"
  # WAL模式提高并发性能
  dsn: "./data/quant.db?_journal_mode=WAL&_busy_timeout=5000&_cache_size=10000&_synchronous=NORMAL"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 1h
```

#### 日志配置

```yaml
log:
  level: "warn"        # 生产环境使用warn级别
  format: "json"       # JSON格式便于日志分析
  output: "file"
  file:
    path: "/var/log/cloudquant/cloudquant.log"
    max_size: 100      # 单个日志文件最大100MB
    max_age: 30        # 保留30天
    max_backups: 10    # 最多保留10个备份
    compress: true     # 压缩旧日志
```

#### 性能配置

```yaml
performance:
  gc_target: 100       # GC目标百分比
  max_memory_mb: 4096  # 最大内存使用4GB
  max_cpu_percent: 80  # 最大CPU使用率80%
```

### 环境变量

| 变量 | 说明 | 必需 |
|------|------|------|
| DEEPSEEK_API_KEY | DeepSeek API密钥 | 否 |
| BROKER_USERNAME | 券商账号 | 否 |
| BROKER_PASSWORD | 券商密码 | 否 |
| FEISHU_WEBHOOK | 飞书webhook | 否 |
| DINGDING_WEBHOOK | 钉钉webhook | 否 |

## 服务管理

### 启动和停止

```bash
# 启动服务
sudo systemctl start cloudquant

# 停止服务
sudo systemctl stop cloudquant

# 重启服务
sudo systemctl restart cloudquant

# 重载配置（如果支持）
sudo systemctl reload cloudquant
```

### 查看状态

```bash
# 查看服务状态
sudo systemctl status cloudquant

# 查看实时日志
sudo journalctl -u cloudquant -f

# 查看最近100条日志
sudo journalctl -u cloudquant -n 100

# 查看应用日志
sudo tail -f /var/log/cloudquant/cloudquant.log
```

### 服务自检

```bash
# 检查健康状态
curl http://localhost:8080/api/v1/health

# 查看指标
curl http://localhost:8080/api/v1/metrics

# 查看版本
curl http://localhost:8080/api/v1/version
```

## 监控和维护

### 日志管理

#### 日志轮转

使用logrotate自动轮转日志：

```bash
# 创建logrotate配置
sudo cat > /etc/logrotate.d/cloudquant << 'EOF'
/var/log/cloudquant/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    create 0644 cloudquant cloudquant
    sharedscripts
    postrotate
        systemctl reload cloudquant > /dev/null 2>&1 || true
    endscript
}
EOF

# 测试配置
sudo logrotate -d /etc/logrotate.d/cloudquant
```

#### 日志分析

```bash
# 查看错误日志
grep -i error /var/log/cloudquant/cloudquant.log

# 查看告警
grep -i alert /var/log/cloudquant/cloudquant.log

# 统计错误数量
grep -c "ERROR" /var/log/cloudquant/cloudquant.log
```

### 性能监控

#### 系统监控

```bash
# 查看CPU和内存
top -u cloudquant

# 查看进程详情
ps aux | grep cloudquant

# 查看文件描述符
lsof -p $(pgrep cloudquant) | wc -l

# 查看网络连接
netstat -an | grep 8080
ss -tnlp | grep 8080
```

#### 应用监控

```bash
# 查看Prometheus指标
curl http://localhost:8080/metrics

# 查看业务指标
curl http://localhost:8080/api/v1/business/stats
```

### 定期维护任务

```bash
# 每日备份数据库
0 2 * * * cloudquant cp /opt/cloudquant/data/quant.db /opt/cloudquant/backups/quant_$(date +\%Y\%m\%d).db

# 每周清理旧日志
0 3 * * 0 find /var/log/cloudquant -name "*.log.*" -mtime +30 -delete

# 每月优化数据库
0 4 1 * * cloudquant /opt/cloudquant/cloudquant --vacuum-db
```

## 故障排查

### 常见问题

#### 1. 服务无法启动

```bash
# 查看详细错误
sudo journalctl -u cloudquant -n 50 --no-pager

# 检查配置文件
sudo -u cloudquant /opt/cloudquant/cloudquant --check-config

# 检查端口占用
sudo netstat -tulpn | grep 8080
```

#### 2. 数据库锁定

```bash
# 检查SQLite锁
lsof /opt/cloudquant/data/quant.db

# 删除WAL文件（谨慎）
rm /opt/cloudquant/data/quant.db-wal /opt/cloudquant/data/quant.db-shm
```

#### 3. 内存不足

```bash
# 查看内存使用
free -h

# 调整配置
sudo vim /etc/cloudquant/config.yaml
# 修改 memory_limit 参数

# 重启服务
sudo systemctl restart cloudquant
```

#### 4. 性能问题

```bash
# 性能分析
sudo -u cloudquant /opt/cloudquant/cloudquant --profile

# 生成火焰图
go tool pprof -http=:8080 cpu.prof
```

### 日志错误码

| 错误 | 说明 | 解决方案 |
|------|------|---------|
| E001 | 数据库连接失败 | 检查数据库文件权限 |
| E002 | 配置文件错误 | 验证YAML语法 |
| E003 | 网络连接失败 | 检查网络和防火墙 |
| E004 | 内存不足 | 增加系统内存或调整配置 |
| E005 | 文件描述符耗尽 | 增加ulimit限制 |

## 备份和恢复

### 数据备份

#### 自动备份脚本

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/opt/cloudquant/backups"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# 备份数据库
cp /opt/cloudquant/data/quant.db $BACKUP_DIR/quant_${DATE}.db

# 备份配置
cp /etc/cloudquant/config.yaml $BACKUP_DIR/config_${DATE}.yaml

# 压缩备份
tar -czf $BACKUP_DIR/cloudquant_${DATE}.tar.gz -C /opt/cloudquant data/ config/

# 清理旧备份（保留最近7天）
find $BACKUP_DIR -name "*.tar.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR/cloudquant_${DATE}.tar.gz"
```

#### 定时备份

```bash
# 添加到crontab
crontab -e

# 每天凌晨2点备份
0 2 * * * /opt/cloudquant/scripts/backup.sh
```

### 数据恢复

```bash
# 停止服务
sudo systemctl stop cloudquant

# 恢复数据库
cp /opt/cloudquant/backups/quant_20240101_020000.db /opt/cloudquant/data/quant.db

# 修复数据库（如果需要）
sqlite3 /opt/cloudquant/data/quant.db "PRAGMA integrity_check;"

# 启动服务
sudo systemctl start cloudquant
```

## 性能优化

### 系统优化

#### 1. 文件系统优化

```bash
# 使用XFS或ext4（已包含noatime选项）
sudo tune2fs -o noatime /dev/sdb1
```

#### 2. 网络优化

```bash
# 增加TCP缓冲区
echo "net.core.rmem_max = 16777216" | sudo tee -a /etc/sysctl.conf
echo "net.core.wmem_max = 16777216" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_rmem = 4096 87380 16777216" | sudo tee -a /etc/sysctl.conf
echo "net.ipv4.tcp_wmem = 4096 65536 16777216" | sudo tee -a /etc/sysctl.conf

sudo sysctl -p
```

#### 3. 内核优化

```bash
# 禁用swap（如果内存足够）
sudo swapoff -a
sudo sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab
```

### 应用优化

#### 1. 数据库优化

```yaml
# 使用WAL模式
database:
  dsn: "./data/quant.db?_journal_mode=WAL&_synchronous=NORMAL"

# 定期VACUUM
sudo -u cloudquant /opt/cloudquant/cloudquant --vacuum-db
```

#### 2. 缓存优化

```yaml
# 增加LRU缓存大小
cache:
  type: "lru"
  size: 10000  # 根据内存调整
```

#### 3. 并发优化

```yaml
# 调整工作池大小
server:
  http:
    max_connections: 100
```

### Docker优化

#### 使用多阶段构建

```dockerfile
# 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o cloudquant .

# 运行阶段
FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/cloudquant /cloudquant
CMD ["/cloudquant"]
```

#### 资源限制

```yaml
# docker-compose.yml
services:
  cloudquant:
    image: cloudquant:latest
    deploy:
      resources:
        limits:
          cpus: '6'
          memory: 6G
        reservations:
          cpus: '2'
          memory: 2G
```

## 安全建议

### 1. 网络安全

```bash
# 配置防火墙
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow from 127.0.0.1 to any port 8080
sudo ufw enable
```

### 2. 文件权限

```bash
# 保护配置文件
sudo chmod 600 /etc/cloudquant/cloudquant.env
sudo chmod 644 /etc/cloudquant/config.yaml

# 保护数据目录
sudo chmod 750 /opt/cloudquant/data
```

### 3. 定期更新

```bash
# 更新系统
sudo apt-get update && sudo apt-get upgrade -y

# 更新CloudQuantBot
sudo systemctl stop cloudquant
cd /opt/cloudquant
git pull origin main
go build -o cloudquant .
sudo systemctl start cloudquant
```

## 相关文档

- [测试指南](TESTING.md)
- [API文档](API.md)
- [架构设计](ARCHITECTURE.md)
- [变更日志](../CHANGELOG.md)
