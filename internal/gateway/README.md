# 统一网关访问模块 (Unified Gateway Module)

## 概述

对标飞牛 fnOS 的统一网关访问功能，为 nas-os 开发的统一网关模块。

## 功能特性

### 1. 反向代理 (Reverse Proxy)
- 统一入口代理所有服务
- 支持多个后端服务器
- 自动健康检查
- 请求转发和负载均衡

### 2. 路由引擎 (Router)
- 基于域名的路由
- 基于路径的路由
- 支持正则表达式路径匹配
- 通配符域名支持 (如 `*.example.com`)
- 路由优先级排序
- 动态添加/删除/更新路由

### 3. SSL/TLS 终止
- 集中管理 SSL 证书
- 支持自动证书管理
- 证书过期检查
- 多域名证书支持

### 4. 访问控制
- 基本认证 (Basic Auth)
- API Key 认证
- 基于角色的访问控制 (RBAC)
- 公开路径白名单
- 细粒度权限控制

### 5. 速率限制
- 基于 IP 的速率限制
- 基于用户的速率限制
- 可配置的速率和突发限制
- 令牌桶算法

### 6. WebSocket 支持
- WebSocket 连接升级
- 可配置的缓冲区大小
- 握手超时设置
- 源检查配置

### 7. 负载均衡
- 轮询算法 (Round-Robin)
- 权重分配
- 健康检查集成
- 自动故障转移

## 文件结构

```
internal/gateway/
├── proxy.go          # 反向代理核心
├── router.go         # 路由引擎
├── middleware.go      # 中间件
├── api.go            # REST API 接口
└── gateway_test.go   # 单元测试
```

## API 接口

### 路由管理

#### 获取所有路由
```http
GET /api/v1/routes
```

#### 创建路由
```http
POST /api/v1/routes
Content-Type: application/json

{
  "id": "route1",
  "name": "API Route",
  "domain": "api.example.com",
  "path": "/v1",
  "backendUrl": "http://localhost:8081",
  "priority": 10,
  "type": "domain"
}
```

#### 获取单个路由
```http
GET /api/v1/routes/{id}
```

#### 更新路由
```http
PUT /api/v1/routes/{id}
Content-Type: application/json

{
  "name": "Updated Route",
  "domain": "api.example.com",
  "path": "/v2",
  "backendUrl": "http://localhost:8082"
}
```

#### 删除路由
```http
DELETE /api/v1/routes/{id}
```

### 后端管理

#### 获取所有后端
```http
GET /api/v1/backends
```

#### 添加后端
```http
POST /api/v1/backends
Content-Type: application/json

{
  "id": "backend1",
  "url": "http://localhost:8081",
  "weight": 1
}
```

#### 删除后端
```http
DELETE /api/v1/backends/{id}
```

### 系统状态

#### 获取状态
```http
GET /api/v1/status
```

#### 健康检查
```http
GET /api/v1/health
```

#### 获取指标
```http
GET /api/v1/metrics
```

## 配置示例

```go
config := &GatewayConfig{
    ListenAddr:     ":8080",
    TLSCertFile:    "/path/to/cert.pem",
    TLSKeyFile:     "/path/to/key.pem",
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   30 * time.Second,
    IdleTimeout:    120 * time.Second,
    MaxHeaderBytes: 1 << 20,
    CORS: &CORSConfig{
        AllowOrigins:     []string{"https://example.com"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
        AllowHeaders:     []string{"Content-Type", "Authorization"},
        AllowCredentials: true,
        MaxAge:           86400,
    },
    RateLimit: &RateLimitConfig{
        Rate:  100, // 100 requests per second
        Burst: 200,
        ByIP:  true,
    },
    Auth: &AuthConfig{
        Enabled: true,
        Users: []User{
            {Username: "admin", Password: "secure-password", Role: "admin"},
        },
        APIKeys:     []string{"api-key-1", "api-key-2"},
        PublicPaths: []string{"/health", "/public"},
    },
    RBAC: &RBACConfig{
        Roles: map[string][]string{
            "admin": {"*"},
            "user":  {"/api/v1/users", "/api/v1/profile"},
            "guest": {"/api/v1/public"},
        },
        DefaultRole: "guest",
    },
}
```

## 使用示例

### 启动网关

```go
package main

import (
    "log"
    "nas-os/internal/gateway"
)

func main() {
    config := gateway.DefaultGatewayConfig()
    gw := gateway.NewGateway(config)
    
    // 添加路由
    route := &gateway.Route{
        ID:         "api-route",
        Name:       "API Service",
        Domain:     "api.example.com",
        Path:       "/",
        BackendURL: "http://localhost:8081",
        Priority:   10,
    }
    gw.AddRoute(route)
    
    // 添加后端
    backend := &gateway.Backend{
        ID:  "backend1",
        URL: "http://localhost:8081",
    }
    gw.AddBackend(backend)
    
    // 启动网关
    log.Fatal(gw.Start())
}
```

### 使用 GatewayManager 管理多个网关

```go
manager := gateway.NewGatewayManager()

// 添加多个网关
manager.AddGateway("http-gw", httpGateway)
manager.AddGateway("https-gw", httpsGateway)

// 启动所有网关
manager.StartAll()

// 获取所有网关状态
status := manager.GetStatusAll()
```

## 中间件链

中间件按以下顺序执行：

1. RecoveryMiddleware - 恢复 panic
2. RequestIDMiddleware - 添加请求 ID
3. LoggingMiddleware - 记录请求日志
4. MetricsMiddleware - 收集指标
5. SecurityHeadersMiddleware - 添加安全头
6. HealthCheckMiddleware - 健康检查端点
7. CORSMiddleware - CORS 处理
8. RateLimitMiddleware - 速率限制
9. BasicAuthMiddleware - 基本认证
10. RBACMiddleware - RBAC 权限控制

## 测试

运行所有测试：

```bash
go test ./internal/gateway/... -v
```

运行特定测试：

```bash
go test ./internal/gateway/... -run TestReverseProxy
go test ./internal/gateway/... -run TestRouter
go test ./internal/gateway/... -run TestMiddleware
go test ./internal/gateway/... -run TestGatewayAPI
```

## 依赖

- `golang.org/x/time` - 速率限制
- `net/http/httputil` - 反向代理
- `regexp` - 正则表达式匹配
- `encoding/json` - JSON 处理

## 安全考虑

1. **速率限制** - 防止 DDoS 攻击
2. **认证** - 支持多种认证方式
3. **RBAC** - 细粒度权限控制
4. **安全头** - 防止 XSS、点击劫持等
5. **CORS** - 跨域资源共享控制
6. **SSL/TLS** - 加密通信

## 性能优化

1. **连接池** - 复用后端连接
2. **健康检查** - 自动剔除不健康后端
3. **负载均衡** - 分散请求压力
4. **中间件优化** - 最小化处理开销

## 后续扩展

1. **自动证书管理** - Let's Encrypt 集成
2. **缓存** - 响应缓存
3. **压缩** - gzip/brotli 压缩
4. **监控** - Prometheus 指标导出
5. **日志** - 结构化日志输出
6. **配置热更新** - 动态配置加载
