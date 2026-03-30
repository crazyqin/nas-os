# Cloudflare Tunnel 远程访问方案设计

## 概述

Cloudflare Tunnel（原名Argo Tunnel）提供了一种无需开放公网端口即可安全远程访问NAS的方案。本设计参考飞牛fnOS 1.1的Cloudflare Tunnel集成方案。

---

## 核心优势

| 特性 | 传统方案 | Cloudflare Tunnel |
|------|----------|------------------|
| **端口暴露** | 需开放公网端口 | 无需开放端口 |
| **DDoS防护** | 需额外配置 | 内置防护 |
| **SSL证书** | 需手动配置 | 自动证书管理 |
| **IP隐藏** | 公网IP可见 | IP完全隐藏 |
| **配置复杂度** | 较高 | 较低 |

---

## 技术架构

### 架构图（概念）

```
用户设备 --> Cloudflare Edge --> Cloudflare Tunnel --> nas-os
             (全球节点)          (cloudflared守护进程)
```

### 核心组件

1. **cloudflared守护进程**
   - 运行在NAS上
   - 建立到Cloudflare Edge的出站连接
   - 无需入站端口

2. **Cloudflare Edge**
   - 全球分布式节点
   - DDoS防护层
   - SSL终止

3. **nas-os集成模块**
   - Tunnel配置管理
   - 认证集成
   - 访问策略配置

---

## 实现方案

### Phase 1: 基础集成（P1）

```go
// internal/tunnel/cloudflare/tunnel.go

type CloudflareTunnel struct {
    TunnelID     string
    AccountID    string
    Token        string // API Token
    ConnectorID  string
    Routes       []TunnelRoute
    Status       TunnelStatus
}

type TunnelRoute struct {
    Hostname    string // e.g., nas.example.com
    Service     string // e.g., http://localhost:80
    PathPrefix  string // Optional path prefix
}
```

### 配置流程

1. **用户准备**
   - 注册Cloudflare账号
   - 添加域名到Cloudflare
   - 创建API Token（Tunnel权限）

2. **nas-os配置**
   - 输入API Token
   - 自动创建Tunnel
   - 配置路由规则

3. **连接建立**
   - 启动cloudflared
   - 建立出站连接
   - 验证连接状态

---

### Phase 2: Zero Trust访问控制（P2）

集成Cloudflare Access提供细粒度访问控制：

```yaml
access_policy:
  - name: "NAS Admin Access"
    include:
      - email: "admin@example.com"
    require:
      - email_verification: true
    applications:
      - "nas.example.com/admin/*"
```

### 访问策略类型

| 策略类型 | 说明 |
|---------|------|
| Email | 指定邮箱用户 |
| Email Verification | 邮箱验证码 |
| IP Range | IP范围限制 |
| Country | 国家限制 |
| MFA | 多因素认证 |
| Device Posture | 设备安全检查 |

---

## 与现有方案对比

### nas-os现有远程访问方案

| 方案 | 状态 | 优势 | 劣势 |
|------|------|------|------|
| **FN Connect风格** | 🚧 开发中 | 免费、本土化 | 依赖第三方服务 |
| **自建VPN** | ✅ 支持 | 完全自主 | 需技术能力 |
| **Cloudflare Tunnel** | 📋 本设计 | 无端口、安全 | 需Cloudflare账号 |

### 建议

1. **初级用户**: Cloudflare Tunnel（配置简单）
2. **高级用户**: 自建VPN（完全自主）
3. **国内用户**: FN Connect风格方案（网络优化）

---

## 配置示例

### docker-compose集成

```yaml
services:
  cloudflared:
    image: cloudflare/cloudflared:latest
    command: tunnel --no-autoupdate run --token ${CLOUDFLARE_TOKEN}
    environment:
      - CLOUDFLARE_TOKEN=${CLOUDFLARE_TOKEN}
    restart: unless-stopped
    networks:
      - nas-internal
```

### API接口设计

```yaml
# /api/v1/tunnel/cloudflare
endpoints:
  - path: /config
    method: GET
    desc: Get Cloudflare Tunnel configuration
    
  - path: /config
    method: POST
    desc: Create/Update Tunnel configuration
    
  - path: /status
    method: GET
    desc: Get Tunnel connection status
    
  - path: /routes
    method: GET
    desc: List Tunnel routes
    
  - path: /routes
    method: POST
    desc: Add Tunnel route
    
  - path: /routes/{id}
    method: DELETE
    desc: Remove Tunnel route
```

---

## 安全考量

### 优势

1. **无公网端口** - 减少攻击面
2. **内置DDoS防护** - Cloudflare全球防护网络
3. **自动SSL** - 无需证书管理
4. **Zero Trust** - 可选细粒度访问控制

### 注意事项

1. **依赖Cloudflare** - 服务可用性依赖第三方
2. **数据路径** - 数据经过Cloudflare Edge（可选择绕过）
3. **账号要求** - 需Cloudflare账号和域名

---

## 开发计划

| Phase | 功能 | 时间 | 状态 |
|-------|------|------|------|
| P1 | 基础Tunnel集成 | 2026-04 | 📋 |
| P2 | Zero Trust访问控制 | 2026-05 | 📋 |
| P3 | 自动配置向导 | 2026-06 | 📋 |

---

## 参考

- 飞牛fnOS 1.1 Cloudflare Tunnel集成
- Cloudflare Tunnel官方文档
- nas-os FN Connect设计文档

---

**工部**
2026-03-30