# NAT Pierce Server - 中继服务器
# 用于 NAT 穿透的中继服务，支持 TCP/UDP 打洞

# ==================== 构建阶段 ====================
FROM golang:1.26-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# 复制 go.mod 和 go.sum 以利用缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建中继服务器
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=$(cat VERSION 2>/dev/null || echo 'dev')" -o natpierce-server ./cmd/natpierce-server

# ==================== 运行阶段 ====================
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非 root 用户
RUN adduser -D -g '' -s /sbin/nologin natpierce

WORKDIR /app

# 从构建阶段复制二进制
COPY --from=builder /build/natpierce-server /app/

# 创建数据目录
RUN mkdir -p /app/data /app/logs && chown -R natpierce:natpierce /app

# 切换到非 root 用户
USER natpierce

# 暴露端口
# TCP 控制端口
EXPOSE 7000
# UDP 打洞端口
EXPOSE 7001/udp
# 数据端口范围
EXPOSE 10000-20000/udp

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD nc -z localhost 7000 || exit 1

# 启动服务
ENTRYPOINT ["/app/natpierce-server"]
CMD ["--config", "/app/config.yaml"]