# NAS-OS 性能优化指南

## 1. 压力测试结果分析

### 测试工具
- **hey**: https://github.com/rakyll/hey
- 安装：`go install github.com/rakyll/hey@latest`

### 测试场景

#### 场景 1: 健康检查端点
```bash
hey -c 100 -n 1000 http://localhost:8080/api/v1/system/health
```

**目标指标**:
- 响应时间 P50 < 10ms
- 响应时间 P99 < 50ms
- 吞吐量 > 1000 req/s
- 错误率 < 0.1%

#### 场景 2: 系统信息端点
```bash
hey -c 50 -n 500 http://localhost:8080/api/v1/system/info
```

**目标指标**:
- 响应时间 P50 < 50ms
- 响应时间 P99 < 200ms
- 吞吐量 > 200 req/s

#### 场景 3: 卷列表端点
```bash
hey -c 20 -n 200 http://localhost:8080/api/v1/volumes
```

**目标指标**:
- 响应时间 P50 < 100ms
- 响应时间 P99 < 500ms
- 吞吐量 > 50 req/s

#### 场景 4: 持续压力测试
```bash
hey -c 50 -z 60s http://localhost:8080/api/v1/system/health
```

**目标指标**:
- 60 秒内无错误
- 内存增长 < 10%
- CPU 使用率稳定

---

## 2. 性能瓶颈分析

### 2.1 使用 pprof 分析
```bash
# 启动 pprof
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30

# 查看 CPU 热点
go tool pprof http://localhost:8080/debug/pprof/heap

# Web 界面分析
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap
```

### 2.2 常见瓶颈

#### 中间件链过长
**问题**: 每个请求经过多个中间件，增加延迟

**优化**:
```go
// ❌ 避免：过多中间件
engine.Use(middleware1)
engine.Use(middleware2)
engine.Use(middleware3)
// ...

// ✅ 建议：合并相关中间件，减少链长度
engine.Use(combinedSecurityMiddleware)
```

#### 日志同步写入
**问题**: 同步日志 I/O 阻塞请求

**优化**:
```go
// ❌ 避免：同步写入
log.Printf("request: %s", path)

// ✅ 建议：异步日志或使用缓冲
logger.Info("request", "path", path) // 使用缓冲 logger
```

#### 锁竞争
**问题**: 全局锁导致并发性能下降

**优化**:
```go
// ❌ 避免：全局锁
var mu sync.Mutex
mu.Lock()
// ...
mu.Unlock()

// ✅ 建议：细粒度锁或无锁数据结构
var mu sync.RWMutex
mu.RLock() // 读锁不阻塞
// ...
mu.RUnlock()
```

---

## 3. 代码级优化

### 3.1 Gin 优化

#### 使用参数化路由
```go
// ✅ 好：参数化路由
api.GET("/volumes/:name", s.getVolume)

// ❌ 避免：手动解析
api.GET("/volumes/*name", func(c *gin.Context) {
    name := c.Param("name")
})
```

#### 复用对象 (sync.Pool)
```go
var responsePool = sync.Pool{
    New: func() interface{} {
        return &Response{}
    },
}

func handleRequest(c *gin.Context) {
    resp := responsePool.Get().(*Response)
    defer responsePool.Put(resp)
    
    // 使用 resp...
}
```

#### 避免内存分配
```go
// ❌ 避免：字符串拼接
result := ""
for i := 0; i < 100; i++ {
    result += strconv.Itoa(i)
}

// ✅ 建议：使用 strings.Builder
var builder strings.Builder
builder.Grow(100) // 预分配
for i := 0; i < 100; i++ {
    builder.WriteString(strconv.Itoa(i))
}
result := builder.String()
```

### 3.2 存储操作优化

#### 批量操作
```go
// ❌ 避免：逐个处理
for _, device := range devices {
    s.storageMgr.AddDevice(volumeName, device)
}

// ✅ 建议：批量处理
s.storageMgr.AddDevices(volumeName, devices)
```

#### 缓存热点数据
```go
// 添加内存缓存
type Cache struct {
    data sync.Map
}

func (c *Cache) Get(key string) (interface{}, bool) {
    return c.data.Load(key)
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
    c.data.Store(key, value)
    // 设置过期时间...
}
```

---

## 4. 系统级优化

### 4.1 Go Runtime 调优

```bash
# 设置 GOMAXPROCS (默认等于 CPU 核心数)
export GOMAXPROCS=4

# 调整 GC 目标 (默认 25%, 可调整为 50% 减少 GC 频率)
export GOGC=50

# 限制内存使用
export GOMEMLIMIT=4GiB
```

### 4.2 网络优化

```bash
# 增加文件描述符限制
ulimit -n 65536

# 调整 TCP 参数
sysctl -w net.core.somaxconn=65536
sysctl -w net.ipv4.tcp_max_syn_backlog=65536
sysctl -w net.ipv4.tcp_tw_reuse=1
```

### 4.3 磁盘 I/O 优化

```bash
# 使用 SSD 或 NVMe
# 启用 write-back 缓存
hdparm -W1 /dev/sda

# 调整 I/O 调度器
echo deadline > /sys/block/sda/queue/scheduler
```

---

## 5. 监控和告警

### 5.1 Prometheus 指标

暴露以下关键指标:
```go
// 请求延迟
http_request_duration_seconds{method, path, status}

// 请求数量
http_requests_total{method, path, status}

// 活跃连接
http_connections_active

// Goroutine 数量
go_goroutines

// 内存使用
go_memstats_alloc_bytes
```

### 5.2 告警规则

```yaml
# alert-rules.yaml
groups:
  - name: nas-os
    rules:
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.01
        for: 5m
        
      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m])) > 1
        
      - alert: HighMemoryUsage
        expr: go_memstats_alloc_bytes / go_memstats_sys_bytes > 0.9
```

---

## 6. 优化检查清单

### 启动前
- [ ] 运行压力测试建立基线
- [ ] 检查 pprof 分析结果
- [ ] 验证内存泄漏
- [ ] 测试并发安全性

### 优化中
- [ ] 减少中间件数量
- [ ] 实现缓存层
- [ ] 优化数据库查询
- [ ] 异步处理耗时操作

### 优化后
- [ ] 重新运行压力测试
- [ ] 对比优化前后指标
- [ ] 验证功能正确性
- [ ] 更新文档

---

## 7. 性能目标

| 指标 | 当前 | 目标 | 优先级 |
|------|------|------|--------|
| P50 响应时间 | - | < 50ms | 高 |
| P99 响应时间 | - | < 500ms | 高 |
| 吞吐量 | - | > 500 req/s | 中 |
| 错误率 | - | < 0.1% | 高 |
| 内存使用 | - | < 500MB | 中 |
| CPU 使用 | - | < 50% | 中 |

---

## 下一步
1. 启动服务并运行压力测试脚本
2. 收集性能数据
3. 分析瓶颈并实施优化
4. 验证优化效果
5. 持续监控
