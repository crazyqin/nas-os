# NAS-OS Dockerfile
# 多阶段构建，优化后的生产镜像约 20-25MB
# 支持 amd64, arm64, arm/v7 架构
#
# 镜像地址: ghcr.io/nas-os/nas-os
#
# 构建命令:
#   docker build -t ghcr.io/nas-os/nas-os:latest .
#   docker build --build-arg VERSION=v1.0.0 -t ghcr.io/nas-os/nas-os:v1.0.0 .
#
# 多架构构建:
#   docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t ghcr.io/nas-os/nas-os:latest .
#
# v2.91.0 更新（工部优化）：
# - 统一健康检查端点说明
# - 更新版本注释格式
#
# v2.90.0 更新：
# - 迁移到 GHCR (GitHub Container Registry)
# - 更新镜像地址注释
#
# v2.88.0 更新：
# - 更新版本注释
#
# v2.86.0 更新：
# - 优化健康检查（简化检测逻辑）
# - 更新基础镜像版本
# - 改进多架构构建支持
#
# v2.35.0 更新：
# - 优化构建缓存策略
# - 增强健康检查
# - 添加 curl 替代 wget（更可靠）

# ========== 构建阶段 ==========
FROM golang:1.24-alpine AS builder

# 构建参数
ARG VERSION=dev
ARG BUILD_TIME
ARG REVISION
# BuildKit 自动注入的跨平台构建参数
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /build

# 安装构建依赖（最小化）
RUN apk add --no-cache git ca-certificates tzdata upx

# 复制 go mod 文件（利用 Docker 缓存）
COPY go.mod go.sum ./

# 下载依赖（使用缓存挂载加速）
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 复制源码（分开复制，利用缓存层）
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY pkg/ ./pkg/
COPY webui/ ./webui/
COPY docs/swagger ./docs/swagger

# 编译参数
ENV CGO_ENABLED=0

# 编译（静态链接，优化大小）
# 使用缓存挂载加速编译
# 支持 BuildKit 自动注入的跨平台参数
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Revision=${REVISION}" \
    -o nasd ./cmd/nasd && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Revision=${REVISION}" \
    -o nasctl ./cmd/nasctl

# UPX 压缩（进一步减小 30-50%）
# 某些架构可能不支持，失败时静默跳过
RUN upx --best --lzma nasd nasctl 2>/dev/null || echo "UPX compression skipped (not supported on this platform)"

# ========== 运行阶段 ==========
FROM alpine:3.21

# 重新声明构建参数（运行阶段需要）
ARG VERSION=dev
ARG BUILD_TIME
ARG REVISION

# OCI 标签
LABEL maintainer="NAS-OS Team"
LABEL org.opencontainers.image.title="NAS-OS"
LABEL org.opencontainers.image.description="Home NAS Management System - Lightweight and Secure"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILD_TIME}"
LABEL org.opencontainers.image.revision="${REVISION}"
LABEL org.opencontainers.image.source="https://github.com/nas-os/nas-os"
LABEL org.opencontainers.image.url="https://nas-os.io"
LABEL org.opencontainers.image.documentation="https://docs.nas-os.io"
LABEL org.opencontainers.image.vendor="NAS-OS Team"
LABEL org.opencontainers.image.licenses="MIT"

# 安装运行时依赖（最小化，按需安装）
RUN apk add --no-cache \
    btrfs-progs \
    samba \
    nfs-utils \
    ca-certificates \
    tzdata \
    curl \
    procps \
    && rm -rf /var/cache/apk/* /tmp/* /var/tmp/*

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

# 健康检查（优化版 v2.86.0）
# 简化检测逻辑：API 健康端点 + 指标端点
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -sf http://localhost:8080/api/v1/health && \
         curl -sf http://localhost:8080/api/v1/metrics > /dev/null 2>&1 || exit 1

# 启动命令
ENTRYPOINT ["nasd"]
CMD ["--config", "/etc/nas-os/config.yaml"]