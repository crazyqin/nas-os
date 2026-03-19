# syntax=docker/dockerfile:1.4
# NAS-OS Dockerfile
# 多阶段构建，优化后的生产镜像约 15-18MB
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
# Go 版本: 1.25（与 go.mod 保持一致）
#
# 镜像特性:
# - 基于 distroless/static，约 15-18MB
# - UPX 压缩进一步减小体积
# - 内置健康检查工具
# - 支持 minimal（distroless）和 full（alpine）两种版本

# ========== 构建阶段 ==========
FROM golang:1.25-alpine AS builder

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

# ========== 健康检查工具构建阶段 ==========
FROM golang:1.25-alpine AS healthcheck-builder

# 构建一个极简的健康检查工具（使用 Dockerfile 1.4 heredoc 语法）
COPY <<EOF /tmp/health.go
package main

import (
	"net/http"
	"os"
)

func main() {
	resp, err := http.Get("http://localhost:8080/api/v1/health")
	if err != nil || resp.StatusCode != 200 {
		os.Exit(1)
	}
	resp.Body.Close()
}
EOF
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /healthcheck /tmp/health.go

# ========== 运行阶段（轻量级） ==========
# 使用 distroless/static 作为基础镜像（约 2MB，无 shell）
# 对于需要系统工具的场景，可以使用 distroless/base 或 alpine
FROM gcr.io/distroless/static-debian12:latest

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

# 注意：distroless 镜像没有 shell，无法使用 RUN 命令
# 如需系统工具（btrfs-progs, samba, nfs-utils），请使用 alpine 版本

# 复制编译产物
COPY --from=builder --chmod=755 /build/nasd /usr/local/bin/nasd
COPY --from=builder --chmod=755 /build/nasctl /usr/local/bin/nasctl
COPY --chmod=644 configs/default.yaml /etc/nas-os/config.yaml
COPY --from=healthcheck-builder --chmod=755 /healthcheck /usr/local/bin/healthcheck

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