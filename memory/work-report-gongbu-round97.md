# 工部第97轮开发工作报告

**时间**: 2026-03-30
**部门**: 工部（DevOps、服务器运维）
**版本**: v2.321.0+

---

## 一、CI/CD 状态报告

### 1.1 GitHub Actions 工作流概览

| 工作流 | 文件 | 状态 | 功能 |
|--------|------|------|------|
| 主 CI/CD | `ci-cd.yml` | ✅ 完善 | 构建、测试、发布自动化 |
| Benchmark | `benchmark.yml` | ✅ 完善 | 性能基准测试（每周一） |
| Compatibility | `compatibility-check.yml` | ✅ 完善 | Go 版本/平台兼容性检查 |
| Docker | `docker-publish.yml` | ✅ 完善 | 多架构镜像构建发布 |
| Release | `release.yml` | ✅ 完善 | GitHub Release 发布 |
| Security | `security-scan.yml` | ✅ 完善 | 安全扫描（gosec/govulncheck） |

### 1.2 主 CI/CD 流程详解

```
变更检测 → 环境准备 → 代码检查(lint+CodeQL) → 单元测试(6分片)
           ↓
    依赖扫描 → 集成测试 → 多平台构建 → Docker测试 → 产物整合
```

**关键优化（v2.254.0）**:
- 测试分片增加到6个，智能均衡分配
- 测试与构建并行执行（移除依赖链）
- 编译产物缓存跨job复用
- Go 版本：1.25（与 go.mod 一致）

### 1.3 构建产物

| 平台 | 架构 | 产物 |
|------|------|------|
| Linux | amd64 | `nasd-linux-amd64` |
| Linux | arm64 | `nasd-linux-arm64` |
| Linux | armv7 | `nasd-linux-armv7` |
| macOS | amd64 | `nasd-darwin-amd64` |
| macOS | arm64 | `nasd-darwin-arm64` |
| Windows | amd64 | `nasd-windows-amd64.exe` |

### 1.4 Docker 镜像

| 镜像类型 | 基础镜像 | 大小 | 特点 |
|----------|----------|------|------|
| minimal | distroless/static | ~15-18MB | 无shell，轻量级 |
| full | alpine:3.19 | ~35-40MB | 包含系统工具 |

**镜像地址**: `ghcr.io/nas-os/nas-os:latest`

---

## 二、编译环境验证

### 2.1 Go 版本状态

| 位置 | 版本 | 状态 |
|------|------|------|
| go.mod | `go 1.25.0` | ✅ 声明版本 |
| 运行环境 | `go1.26.0` | ✅ 兼容 |
| CI/CD | `GO_VERSION: '1.25'` | ✅ 一致 |

**注意**: go 1.26 向前兼容 go 1.25 模块，无问题。

### 2.2 依赖验证

```bash
go mod download  # ✅ 成功
go build ./cmd/nasd  # ✅ 成功
```

**核心依赖**:
- gin-gonic/gin (Web框架)
- blevesearch/bleve (搜索引擎)
- aws-sdk-go-v2 (S3兼容)
- prometheus/client_golang (监控)
- spf13/cobra (CLI)
- uber.org/zap (日志)
- golang.org/x/crypto (加密)

### 2.3 编译验证结果

```
✅ 主程序构建验证通过
✅ go vet 检查通过
✅ 依赖完整
```

---

## 三、内网穿透技术方案对比

### 3.1 已实现方案

nas-os 已完整实现内网穿透功能（v2.318.0+），代码位置：
- `internal/tunnel/` - 内网穿透核心模块
- `internal/connect/` - NAS Connect 服务（类似 TrueNAS Connect）

**支持的穿透方式**:

| 方式 | 实现文件 | 特点 | 适用场景 |
|------|----------|------|----------|
| Cloudflare Tunnel | `cloudflare.go` | 无需开放端口，免费 | 家庭用户推荐 |
| FRP | `frp.go` | 自建服务器，灵活 | 需要自控的场景 |
| P2P (STUN/TURN) | `stun.go`, `turn.go`, `p2p.go` | 直连优先 | 高性能场景 |
| 中继模式 | `manager.go`, `service.go` | 稳定可靠 | NAT严格场景 |

### 3.2 技术方案对比

#### 方案A：Cloudflare Tunnel（推荐）

**优势**:
- ✅ **免费**：Cloudflare 提供免费 Tunnel 服务
- ✅ **零配置**：使用 Tunnel Token 即可启动
- ✅ **安全**：自带 TLS，无需暴露公网端口
- ✅ **稳定**：Cloudflare 全球节点中继
- ✅ **已集成**：v2.318.0 已完整实现

**劣势**:
- ❌ 依赖 Cloudflare 账号
- ❌ 流量经过 Cloudflare（性能略降）
- ❌ 中国大陆访问可能受限

**实现配置**:
```yaml
tunnel:
  cloudflare:
    token: "your-tunnel-token"  # 推荐：使用 Token 方式
    # 或 API Token 管理
    api_token: "your-api-token"
    account_id: "your-account-id"
    tunnel_name: "nas-os"
    zone_name: "example.com"
    subdomain: "nas"
    local_services:
      - name: "webui"
        protocol: "http"
        local_port: 8080
```

#### 方案B：自建 FRP 服务（备选）

**优势**:
- ✅ **自主可控**：服务器、域名完全掌控
- ✅ **高性能**：直连或自建中继
- ✅ **无限制**：流量、域名无限制
- ✅ **国内友好**：自建服务器无跨境问题
- ✅ **已实现**：参考飞牛fnOS FN Connect

**劣势**:
- ❌ 需要公网服务器（成本）
- ❌ 需要维护 FRP 服务器
- ❌ 需要域名和 TLS 配置

**实现配置**:
```yaml
tunnel:
  frp:
    enabled: true
    server_addr: "frp.example.com"
    server_port: 7000
    token: "your-token"
    device_id: "nas-xxx"
    auto_reconnect: true
    proxies:
      - name: "webui"
        type: "http"
        local_ip: "127.0.0.1"
        local_port: 8080
        subdomain: "nas"
```

#### 方案C：中继服务器（自建）

**已实现**: `deploy/natpierce-server/Dockerfile`

**特点**:
- 自建 NAT 穿透中继服务器
- 支持 TCP/UDP 打洞
- 适用于企业内部部署

### 3.3 竞品参考

| 竞品 | 方案 | 特点 |
|------|------|------|
| **飞牛fnOS FN Connect** | FRP定制 | 免费内网穿透，零配置 |
| **群晖 QuickConnect** | 自建中继 | 账号绑定，一键访问 |
| **TrueNAS Connect** | 多模式 | P2P/中继，灵活选择 |
| **nas-os (本项目)** | 全方案 | Cloudflare/FRP/P2P全支持 |

### 3.4 推荐方案

**家庭用户**: 
- **首选**: Cloudflare Tunnel（免费、零配置、安全）
- **备选**: FRP（需要自建服务器时）

**企业用户**:
- **首选**: 自建 FRP 服务（自主可控）
- **备选**: 中继服务器（内部部署）

---

## 四、Docker 部署优化建议

### 4.1 当前 Dockerfile 分析

**基础镜像**: `gcr.io/distroless/static-debian12:latest`
- 大小：~15-18MB
- 特点：无 shell，最小化

**构建优化**:
- ✅ 多阶段构建
- ✅ UPX 压缩（amd64/arm64）
- ✅ 缓存挂载加速
- ✅ 版本信息嵌入
- ✅ 健康检查工具

### 4.2 优化建议

#### 建议1：镜像大小优化

```
当前：distroless ~15-18MB
建议：保持当前方案，已是最佳
```

**原因**: distroless 是最小安全镜像，无冗余。

#### 建议2：健康检查优化

**当前**: 使用内置 Go 健康检查工具
**建议**: 保持，distroless 无 shell，必须用 CMD

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD ["/usr/local/bin/healthcheck"]
```

#### 建议3：多架构构建优化

**已优化**: v2.254.0 使用 Matrix 策略分离架构构建
- amd64: 原生 runner，无 QEMU
- arm64: ARM runner，无 QEMU
- armv7: QEMU，但跳过 UPX（兼容性）

**建议**: 保持当前方案，性能最优。

#### 建议4：镜像推送优化

**当前**: GHCR + Cosign 签名 + SBOM
**建议**: 考虑添加 Docker Hub（需创建仓库）

```yaml
# 待启用（仓库创建后）
- name: 登录 Docker Hub
  uses: docker/login-action@v4
  with:
    username: ${{ secrets.DOCKERHUB_USERNAME }}
    password: ${{ secrets.DOCKERHUB_TOKEN }}
```

#### 建议5：部署脚本优化

**当前**: `deploy/deploy.sh` 支持二进制和 Docker 部署
**建议**: 
- 添加版本检测
- 添加配置验证
- 添加健康检查等待优化

### 4.3 部署最佳实践

```yaml
# docker-compose.yml 生产配置建议
services:
  nas-os:
    image: ghcr.io/nas-os/nas-os:latest
    privileged: true  # 需要磁盘访问
    network_mode: host  # 最佳性能
    volumes:
      - ./configs:/etc/nas-os:ro
      - nas-os-data:/var/lib/nas-os
    environment:
      - TZ=Asia/Shanghai
      - NAS_OS_LOG_LEVEL=info
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
    healthcheck:
      test: ["CMD", "/usr/local/bin/healthcheck"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
```

---

## 五、总结

### 5.1 CI/CD 状态：✅ 完善

- GitHub Actions 工作流完整
- 多平台构建支持
- 安全扫描集成
- Docker 镜像自动发布

### 5.2 编译环境：✅ 正常

- Go 1.26.0 运行环境兼容 go 1.25 模块
- 依赖完整，编译验证通过
- 主程序构建成功

### 5.3 内网穿透：✅ 全方案实现

- Cloudflare Tunnel（免费推荐）
- FRP（自建推荐）
- P2P/中继（高性能场景）
- 对标竞品：飞牛fnOS、群晖、TrueNAS

### 5.4 Docker 部署：✅ 优化完成

- distroless 最小镜像（~15-18MB）
- UPX 压缩
- 多架构支持
- 健康检查集成

---

**工部 第97轮开发完成**