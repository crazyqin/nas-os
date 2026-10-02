# syntax=docker/dockerfile:1.4
# NAS-OS Dockerfile
# 多阶段构建，Core 二进制 + 必需的存储运行工具
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
# Go 版本: 1.26（与 go.mod 保持一致）
#
# 镜像特性:
# - 基于 Debian bookworm slim，包含启动扫描所需的 sudo/btrfs
# - UPX 压缩进一步减小体积
# - 内置健康检查工具
# - 默认 Core；额外系统工具见 Dockerfile.full（Alpine）

# ========== 构建阶段 ==========
FROM golang:1.26-alpine AS builder

# 构建参数
ARG VERSION=dev
ARG BUILD_TIME
ARG REVISION
# Default Core-only (matches make build / docs). Full product surface:
#   docker build --build-arg BUILD_TAGS=nasd_full ...
# nasd_full = product managers (docker/vm/photos/…); empty = Core-only slim binary
ARG BUILD_TAGS=
# BuildKit 自动注入的跨平台构建参数
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

WORKDIR /build

# 安装构建依赖（最小化）
RUN apk add --no-cache git ca-certificates tzdata upx

# 复制 go mod 文件（利用 Docker 缓存）
COPY go.mod go.sum ./
COPY VERSION ./

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
    V="${VERSION#v}"; \
    if ! echo "$V" | grep -Eq '^[0-9]+\.[0-9]+'; then V="$(sed 's/^v//' VERSION)"; fi; \
    if [ -n "${BUILD_TAGS}" ]; then TAGS="-tags ${BUILD_TAGS}"; else TAGS=""; fi; \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build ${TAGS} -ldflags="-w -s -X nas-os/internal/version.Version=$V -X nas-os/internal/version.BuildTime=${BUILD_TIME} -X nas-os/internal/version.Commit=${REVISION}" \
    -o nasd ./cmd/nasd && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s -X nas-os/internal/version.Version=$V -X nas-os/internal/version.BuildTime=${BUILD_TIME} -X nas-os/internal/version.Commit=${REVISION}" \
    -o nasctl ./cmd/nasctl

# UPX 压缩（进一步减小 30-50%）
# armv7 跳过 UPX：QEMU 下极慢且兼容性有限
RUN if [ "${TARGETARCH}" != "arm" ] || [ "${TARGETVARIANT}" != "v7" ]; then \
      echo "Running UPX compression for ${TARGETARCH}${TARGETVARIANT}..."; \
      upx --best --lzma nasd nasctl 2>/dev/null || echo "UPX compression skipped"; \
    else \
      echo "Skipping UPX for armv7 (slow in QEMU, limited support)"; \
    fi

# ========== 健康检查工具构建阶段 ==========
FROM golang:1.26-alpine AS healthcheck-builder

# 构建一个极简的健康检查工具（使用 Dockerfile 1.4 heredoc 语法）
COPY <<EOF /tmp/health.go
package main

import (
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:8080/api/v1/system/health")
	if err != nil || resp.StatusCode != 200 {
		os.Exit(1)
	}
	resp.Body.Close()
}
EOF
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /healthcheck /tmp/health.go

# ========== 运行阶段（Core + 必需的存储工具） ==========
# Core 启动必须扫描 btrfs；仅有静态 Go 二进制的 distroless 无法启动。
# 保持 Debian 12 用户空间，补齐实际命令，不跳过初始化或伪造健康。
FROM debian:bookworm-slim

RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      btrfs-progs sudo ca-certificates tzdata && \
    rm -rf /var/lib/apt/lists/*

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

# 更多系统工具（samba、nfs-utils 等）见 Dockerfile.full。

# 复制编译产物
COPY --from=builder --chmod=755 /build/nasd /usr/local/bin/nasd
COPY --from=builder --chmod=755 /build/nasctl /usr/local/bin/nasctl
COPY --chmod=644 configs/default.yaml /etc/nas-os/config.yaml
COPY --from=healthcheck-builder --chmod=755 /healthcheck /usr/local/bin/healthcheck

# 复制 Web UI 静态文件（运行时需要）
# 注意：使用 /usr/share/nas-os/webui 而非 /var/lib/nas-os/webui
# 原因：docker-compose.yml 中 /var/lib/nas-os 会被卷挂载覆盖
COPY --from=builder --chmod=644 /build/webui/ /usr/share/nas-os/webui/

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

# 健康检查（v2.123.0 优化）
# 使用内置健康检查工具，无外部依赖
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/healthcheck"]

# 启动命令
ENTRYPOINT ["nasd"]
CMD ["--config", "/etc/nas-os/config.yaml"]
