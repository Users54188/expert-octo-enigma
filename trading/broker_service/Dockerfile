FROM python:3.10-slim

WORKDIR /app

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    xvfb \
    wine \
    wine32 \
    && rm -rf /var/lib/apt/lists/*

# 复制依赖文件
COPY requirements.txt .

# 安装Python依赖
RUN pip install --no-cache-dir -r requirements.txt

# 复制服务代码
COPY easytrader_service.py .

# 暴露端口
EXPOSE 8888

# 设置环境变量
ENV PYTHONUNBUFFERED=1

# 启动服务
CMD ["python", "easytrader_service.py"]
