# syntax=docker/dockerfile:1

# 构建阶段 - ARM64版本
FROM --platform=linux/arm64 library/golang:1.24.5-alpine AS builder

# 安装必要的构建工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 复制源代码
COPY . .

# 初始化go模块并添加依赖
RUN go mod init go-uploader
RUN go get github.com/gin-gonic/gin
RUN go mod tidy

# 构建应用程序，启用CGO以支持某些功能，优化二进制文件
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-w -s" \
    -o uploader main.go

# 运行阶段 - ARM64版本
FROM --platform=linux/arm64 library/golang:1.24.5-alpine

# 安装必要的运行时依赖和工具
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# 创建非root用户
RUN adduser -D -s /bin/sh -u 1001 uploader

WORKDIR /app

# 复制构建的二进制文件
COPY --from=builder /app/uploader .

# 复制静态文件
COPY --from=builder /app/static/ ./static/

# 复制配置文件（如果存在）
COPY --from=builder /app/config.json ./config.json

# 创建必要的目录并设置权限
RUN mkdir -p /app/upload /app/merged /app/logs \
    && chown -R uploader:uploader /app \
    && chmod +x /app/uploader

# 切换到非root用户
USER uploader

# 设置环境变量
ENV GIN_MODE=release
ENV TZ=Asia/Shanghai

# 暴露端口
EXPOSE 9876

# 启动命令
CMD ["./uploader"]
