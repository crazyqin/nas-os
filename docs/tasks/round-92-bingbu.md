# 第92轮兵部任务 - 内网穿透服务

## 背景
对标飞牛FN Connect，实现免费内网穿透功能。

## 任务要求

### 1. Cloudflare Tunnel Handler
- 隧道创建/删除/状态监控
- 配置持久化与状态同步
- 支持自定义域名绑定

### 2. 实现位置
- `internal/tunnel/` 目录
- HTTP API: `/api/v1/tunnel`
- WebSocket状态推送

### 3. 核心功能
```go
type TunnelService interface {
    Create(config TunnelConfig) (*Tunnel, error)
    Delete(id string) error
    List() ([]Tunnel, error)
    GetStatus(id string) (*TunnelStatus, error)
    StreamLogs(id string) (<-chan LogEntry, error)
}
```

### 4. 交付物
- `internal/tunnel/service.go` - 核心服务
- `internal/tunnel/cloudflare.go` - Cloudflare实现
- `internal/tunnel/handler.go` - HTTP处理
- `internal/tunnel/config.go` - 配置管理
- 单元测试覆盖 >80%

## 注意事项
- 使用Cloudflare Tunnel SDK
- 支持断线重连
- 错误处理要完善