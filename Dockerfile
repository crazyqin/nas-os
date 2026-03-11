# NAS-OS Dockerfile
# 多阶段构建，生产镜像约 30MB

# ========== 构建阶段 ==========
FROM golang:1.26.1-alpine AS builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git ca-certificates

# 复制 go mod 文件（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download -x

# 复制源码
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY webui/ ./webui/
COPY docs/swagger ./docs/swagger

# 编译（静态链接，无 CGO）
ENV CGO_ENABLED=0
RUN go build -ldflags="-w -s" -o nasd ./cmd/nasd
# nasctl CLI 待创建
# RUN go build -ldflags="-w -s" -o nasctl ./cmd/nasctl

# ========== 运行阶段 ==========
FROM alpine:3.21

LABEL maintainer="NAS-OS Team"
LABEL description="Home NAS Management System"

# 安装运行时依赖
RUN apk add --no-cache \
    btrfs-progs \
    samba \
    nfs-utils \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# 创建必要的目录
RUN mkdir -p /mnt /etc/nas-os /var/log/nas-os

# 复制编译产物
COPY --from=builder /build/nasd /usr/local/bin/nasd
# nasctl CLI 待创建
# COPY --from=builder /build/nasctl /usr/local/bin/nasctl
COPY configs/default.yaml /etc/nas-os/config.yaml

# 暴露端口
EXPOSE 8080
EXPOSE 445
EXPOSE 2049
EXPOSE 111

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/v1/health || exit 1

# 启动命令
ENTRYPOINT ["nasd"]
CMD ["--config", "/etc/nas-os/config.yaml"]
