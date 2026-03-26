# Docker 构建优化方案

> 版本: v1.0.0 | 日期: 2026-03-26 | 工部

## 问题背景

armv7 构建经常超时，已将 timeout 从 25 分钟增加到 35 分钟。但这只是治标不治本，需要从根本上优化构建流程。

## 当前状态分析

### 构建流程现状

```yaml
# docker-publish.yml matrix 策略
matrix:
  - platform: amd64
    runner: ubuntu-latest        # 原生 x86_64
    timeout: 15
  - platform: arm64
    runner: ubuntu-24.04-arm     # 原生 ARM64 ✅
    timeout: 15
  - platform: armv7
    runner: ubuntu-latest        # 需要QEMU模拟 ❌
    timeout: 35
```

### 超时原因分析

1. **QEMU 模拟开销**: armv7 在 x86_64 runner 上通过 QEMU 模拟，性能损失 10-50 倍
2. **Go 编译慢**: Go 1.26 编译本身有开销，加上大量依赖（约 150+ 模块）
3. **依赖下载**: 每次构建都需要下载依赖（虽然有缓存但仍然慢）
4. **UPX 压缩**: 在 QEMU 下执行 UPX 压缩非常慢

### 当前 Dockerfile 分析

```dockerfile
# 当前流程
FROM golang:1.26-alpine AS builder
RUN apk add git ca-certificates tzdata upx  # 每次都要安装
RUN go mod download                          # 依赖下载
RUN go build ...                             # 编译
RUN upx --best --lzma ...                    # UPX 压缩（QEMU 下很慢）
```

---

## 优化方案

### 方案一：预构建 Go 编译镜像 ⭐ 推荐

**原理**: 将 Go 编译环境预构建成镜像，避免每次构建时安装依赖。

```dockerfile
# 新建 Dockerfile.builder
FROM golang:1.26-alpine

# 预安装常用工具
RUN apk add --no-cache git ca-certificates tzdata upx

# 预下载常用 Go 工具链
RUN go install golang.org/x/tools/gopls@latest || true

# 设置 GOPROXY
ENV GOPROXY=https://proxy.golang.org,direct
```

**Workflow 修改**:
```yaml
- name: 构建并推送单架构镜像
  uses: docker/build-push-action@v6
  with:
    context: .
    file: ./Dockerfile
    platforms: ${{ matrix.docker_platform }}
    # 使用预构建镜像作为缓存源
    cache-from: |
      type=registry,ref=ghcr.io/${{ env.IMAGE_NAME }}:builder-cache
      type=gha,scope=${{ env.CACHE_VERSION }}-${{ matrix.platform }}
```

**预期效果**: 减少 2-3 分钟的 apk 安装时间

---

### 方案二：分离 UPX 压缩到独立阶段 ⭐ 推荐

**问题**: UPX 压缩在 QEMU 下执行极慢，且 armv7 支持有限。

**方案**:
```dockerfile
# 修改 Dockerfile
FROM golang:1.26-alpine AS builder
# ... 编译步骤，不做 UPX

# 独立的压缩阶段（可选）
FROM alpine:3.21 AS compressor
ARG TARGETARCH
ARG TARGETVARIANT
COPY --from=builder /build/nasd /build/nasctl /tmp/
RUN apk add --no-cache upx && \
    # 只在特定架构执行压缩
    if [ "$TARGETARCH" = "arm" ] && [ "$TARGETVARIANT" = "v7" ]; then \
      echo "Skipping UPX for armv7 (slow and limited support)"; \
    else \
      upx --best --lzma /tmp/nasd /tmp/nasctl 2>/dev/null || true; \
    fi && \
    cp /tmp/nasd /tmp/nasctl /build/

# 运行阶段
FROM gcr.io/distroless/static-debian12:latest
COPY --from=compressor /build/nasd /usr/local/bin/nasd
```

**或者直接跳过 armv7 的 UPX**:
```dockerfile
# 在 builder 阶段
RUN if [ "$TARGETARCH" != "arm" ] || [ "$TARGETVARIANT" != "v7" ]; then \
      upx --best --lzma nasd nasctl 2>/dev/null || true; \
    fi
```

**预期效果**: armv7 构建减少 3-5 分钟

---

### 方案三：使用 cgo 编译缓存 ⭐⭐ 高效

**原理**: Go 1.21+ 支持 GOCACHEPROG，可以将编译缓存存入镜像层。

**修改 Dockerfile**:
```dockerfile
FROM golang:1.26-alpine AS builder

# 启用编译缓存
ENV GOCACHE=/go/build-cache
ENV GOMODCACHE=/go/pkg/mod

# 先复制 go.mod/go.sum，创建依赖缓存层
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# 复制源码并编译
COPY cmd/ internal/ pkg/ webui/ docs/swagger ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/go/build-cache \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s ..." -o nasd ./cmd/nasd
```

**注意**: 当前配置已经使用了 `--mount=type=cache`，但可以进一步优化：
- 确保 `cache-from` 指向正确的缓存范围
- 使用 `mode=max` 导出缓存

---

### 方案四：使用自托管 ARM Runner（根本解决）

**问题根源**: GitHub 托管的 runner 没有 armv7 原生环境。

**解决方案**:
1. 使用自托管 ARM32 设备（如 Raspberry Pi Zero 2W）
2. 使用支持 ARM32 的云服务（如 Oracle Cloud ARM 实例）

**Workflow 配置**:
```yaml
matrix:
  - platform: armv7
    runner: self-hosted-armv7  # 自托管 runner
    timeout: 15  # 原生运行，timeout 可以恢复正常
```

**成本分析**:
- Raspberry Pi Zero 2W: ~$15/台
- Oracle Cloud ARM: 免费层（4 OCPU + 24GB RAM）
- 一次投入，长期受益

---

### 方案五：跨架构编译 + 原生运行测试

**原理**: 使用 Go 的交叉编译能力，在 x86_64 上编译 armv7 二进制，然后只在 armv7 设备上做简单测试。

```yaml
# 新增 job: 交叉编译
build-cross:
  runs-on: ubuntu-latest
  steps:
    - uses: actions/checkout@v5
    - uses: actions/setup-go@v5
      with:
        go-version: '1.26'
        cache: true
    
    - name: 交叉编译 armv7
      run: |
        CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
          go build -ldflags="-w -s" -o nasd-armv7 ./cmd/nasd
    
    - name: 上传二进制
      uses: actions/upload-artifact@v4
      with:
        name: binary-armv7
        path: nasd-armv7

# Docker 构建只做打包，不做编译
build-docker-armv7:
  needs: build-cross
  runs-on: ubuntu-latest
  steps:
    - uses: actions/download-artifact@v4
      with:
        name: binary-armv7
    
    - name: 构建 Docker 镜像
      uses: docker/build-push-action@v6
      with:
        build-args: |
          BINARY_PATH=.
```

**修改 Dockerfile**:
```dockerfile
# Dockerfile.cross - 交叉编译版本
ARG BINARY_PATH=.

FROM gcr.io/distroless/static-debian12:latest
COPY ${BINARY_PATH}/nasd /usr/local/bin/nasd
# ... 其他配置
```

**预期效果**: 构建时间从 35 分钟降到 5-10 分钟

---

### 方案六：优化依赖下载

**当前问题**: 每次构建都要下载 150+ Go 模块。

**优化**:

1. **使用 Module Proxy**:
```yaml
env:
  GOPROXY: 'https://proxy.golang.org,direct'
  GOSUMDB: 'sum.golang.org'
```

2. **私有 Proxy（可选）**:
```yaml
env:
  GOPROXY: 'https://your-goproxy.example.com,https://proxy.golang.org,direct'
```

3. **使用 Athens 或 Artifactory** 搭建私有 Go 模块代理

---

## Dockerfile.ai 优化

### 问题分析

1. **PyTorch 下载慢**: PyTorch GPU 版本约 2GB+
2. **CUDA 镜像大**: nvidia/cuda 基础镜像约 4GB
3. **重复安装**: 每次构建都重新安装 Python 依赖

### 优化方案

```dockerfile
# 使用预构建的 PyTorch 镜像
FROM pytorch/pytorch:2.5.1-cuda12.4-cudnn9-runtime AS base

# 只安装额外依赖
RUN pip install --no-cache-dir \
    ftfy regex tqdm \
    openai-clip \
    transformers \
    pillow

# 或者使用多阶段构建减少镜像层
FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04 AS builder
# ... 安装 Python 虚拟环境

FROM nvidia/cuda:12.2.0-runtime-ubuntu22.04
COPY --from=builder /opt/venv /opt/venv
```

---

## 实施优先级

| 优先级 | 方案 | 工作量 | 效果 | 风险 |
|-------|------|-------|------|------|
| 1 | 分离 UPX 压缩 | 小 | 高 | 低 |
| 2 | 跳过 armv7 UPX | 小 | 中 | 低 |
| 3 | 优化缓存策略 | 中 | 中 | 低 |
| 4 | 交叉编译模式 | 大 | 高 | 中 |
| 5 | 自托管 Runner | 大 | 极高 | 中 |
| 6 | 预构建镜像 | 中 | 中 | 低 |

---

## 推荐实施步骤

### 第一阶段：快速优化（本周）

1. **跳过 armv7 的 UPX 压缩**
   - 修改 Dockerfile，条件跳过 armv7 的 UPX
   - 预期减少 3-5 分钟

2. **优化 GHA 缓存配置**
   - 确保 `cache-to: type=gha,mode=max`
   - 增加 `restore-keys` 回退

### 第二阶段：中期优化（下周）

3. **实施交叉编译模式**
   - 创建 `Dockerfile.cross`
   - 修改 workflow 支持预编译二进制

4. **Dockerfile.ai 优化**
   - 使用预构建 PyTorch 镜像
   - 分离 GPU/CPU 构建流程

### 第三阶段：长期优化（可选）

5. **自托管 ARM Runner**
   - 评估成本和收益
   - 部署和配置

---

## 附录：修改示例

### A. Dockerfile 修改（跳过 armv7 UPX）

```dockerfile
# 修改 builder 阶段的 RUN upx 命令
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Revision=${REVISION}" \
    -o nasd ./cmd/nasd && \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -ldflags="-w -s -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.Revision=${REVISION}" \
    -o nasctl ./cmd/nasctl

# 条件执行 UPX 压缩（跳过 armv7）
RUN if [ "${TARGETARCH}" != "arm" ] || [ "${TARGETVARIANT}" != "v7" ]; then \
      echo "Running UPX compression for ${TARGETARCH}${TARGETVARIANT}..."; \
      upx --best --lzma nasd nasctl 2>/dev/null || echo "UPX compression failed, continuing..."; \
    else \
      echo "Skipping UPX compression for armv7 (slow and limited support)"; \
    fi
```

### B. Workflow 缓存优化

```yaml
- name: 构建并推送单架构镜像
  uses: docker/build-push-action@v6
  with:
    context: .
    file: ./Dockerfile
    platforms: ${{ matrix.docker_platform }}
    push: ${{ github.event_name != 'pull_request' }}
    tags: |
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.sha.outputs.SHORT_SHA }}-${{ matrix.suffix }}
    # 优化缓存配置
    cache-from: |
      type=gha,scope=${{ env.CACHE_VERSION }}-${{ matrix.platform }}
      type=gha,scope=${{ env.CACHE_VERSION }}-${{ matrix.platform }}-master
      type=registry,ref=${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:cache-${{ matrix.platform }}
    cache-to: type=gha,scope=${{ env.CACHE_VERSION }}-${{ matrix.platform }},mode=max
```

---

## 监控指标

优化后应监控以下指标：

1. **构建时间**: 各平台的平均构建时间
2. **缓存命中率**: GHA 缓存的命中/未命中比例
3. **镜像大小**: 最终镜像的大小变化
4. **失败率**: armv7 构建的超时/失败率

---

## 已实施的优化

### 2026-03-26 实施

1. **跳过 armv7 的 UPX 压缩** ✅
   - 修改了 `Dockerfile` 和 `Dockerfile.full`
   - 预期减少 armv7 构建时间 3-5 分钟
   - 变更:
     ```dockerfile
     # 条件执行 UPX 压缩（跳过 armv7）
     RUN if [ "${TARGETARCH}" != "arm" ] || [ "${TARGETVARIANT}" != "v7" ]; then \
           upx --best --lzma nasd nasctl 2>/dev/null || echo "UPX compression skipped"; \
         else \
           echo "Skipping UPX for armv7 (slow in QEMU, limited support)"; \
         fi
     ```

2. **创建优化文档** ✅
   - 新建 `docs/docker-build-optimization.md`
   - 记录所有优化方案和实施步骤

---

*文档由工部维护 | 版本历史见 Git*