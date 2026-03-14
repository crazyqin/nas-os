# NAS-OS Dockerfile
# 多阶段构建，优化后的生产镜像约 25MB

# ========== 构建阶段 ==========
FROM golang:1.26-alpine AS builder

WORKDIR /build

# 安装构建依赖
RUN apk add --no-cache git ca-certificates upx

# 复制 go mod 文件（利用 Docker 缓存）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY webui/ ./webui/
COPY docs/swagger ./docs/swagger

# 编译参数
ARG VERSION=dev
ARG BUILD_TIME

# 编译（静态链接，优化大小）
ENV CGO_ENABLED=0
RUN go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o nasd ./cmd/nasd && \
    go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}" -o nasctl ./cmd/nasctl

# UPX 压缩（可选，进一步减小 30-50%）
RUN upx --best --lzma nasd nasctl 2>/dev/null || true

# ========== 运行阶段 ==========
FROM alpine:3.21

LABEL maintainer="NAS-OS Team"
LABEL org.opencontainers.image.title="NAS-OS"
LABEL org.opencontainers.image.description="Home NAS Management System"
LABEL org.opencontainers.image.version="2.7.0"
LABEL org.opencontainers.image.source="https://github.com/nas-os/nas-os"

# 安装运行时依赖（最小化）
RUN apk add --no-cache \
    btrfs-progs \
    samba \
    nfs-utils \
    ca-certificates \
    tzdata \
    && rm -rf /var/cache/apk/* /tmp/*

# 创建非 root 用户（可选，某些操作需要 root）
# RUN addgroup -g 1000 nasos && \
#     adduser -u 1000 -G nasos -s /bin/sh -D nasos

# 创建必要的目录
RUN mkdir -p /mnt /etc/nas-os /var/log/nas-os /var/lib/nas-os && \
    chmod 755 /etc/nas-os /var/log/nas-os /var/lib/nas-os

# 复制编译产物
COPY --from=builder --chmod=755 /build/nasd /usr/local/bin/nasd
COPY --from=builder --chmod=755 /build/nasctl /usr/local/bin/nasctl
COPY --chmod=644 configs/default.yaml /etc/nas-os/config.yaml

# 暴露端口
# Web UI
EXPOSE 8080/tcp
# SMB
EXPOSE 445/tcp
EXPOSE 139/tcp
# NFS
EXPOSE 2049/tcp
EXPOSE 111/tcp
EXPOSE 111/udp
# NFS auxiliary
EXPOSE 20048/tcp

# 健康检查（增强版）
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/api/v1/health && \
    wget -q --spider http://localhost:8080/api/v1/system/status || exit 1

# 启动命令
ENTRYPOINT ["nasd"]
CMD ["--config", "/etc/nas-os/config.yaml"]