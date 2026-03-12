# 系统监控仪表盘

NAS-OS 系统监控仪表盘提供实时系统状态监控、历史数据分析和告警管理功能。

## 核心功能

### 1. 系统概览
- **CPU 使用率**: 实时 CPU 占用百分比，显示核心数和温度
- **内存使用**: 已用/总内存，使用百分比
- **磁盘使用**: 所有磁盘的平均使用率
- **网络流量**: 实时上传/下载速度

### 2. 资源图表
- **实时趋势图**: CPU 和内存使用率实时曲线（1 秒刷新）
- **历史数据**: 支持 24 小时/7 天/30 天视图
- **Chart.js 可视化**: 平滑曲线，响应式设计

### 3. 磁盘健康
- **SMART 数据**: 磁盘型号、序列号、健康状态
- **温度监控**: 实时磁盘温度
- **运行时间**: 磁盘通电时间（小时）
- **错误统计**: 重映射扇区、待映射扇区

### 4. 网络流量
- **接口统计**: 每个网络接口的独立统计
- **实时速度**: 下载/上传速度（KB/s 或 MB/s）
- **数据包统计**: RX/TX 数据包数量和错误

### 5. 进程监控
- **Top 10 进程**: 按 CPU 使用率排序
- **详细信息**: PID、进程名、CPU%、内存%、内存 (MB)、用户

### 6. 告警中心
- **实时告警**: CPU、内存、磁盘阈值告警
- **级别分类**: Critical（严重）、Warning（警告）、Info（信息）
- **告警管理**: 确认、删除告警

## 技术架构

### 后端 (Go)

#### 文件结构
```
internal/system/
├── monitor.go      # 核心监控逻辑
└── handlers.go     # HTTP/WebSocket 处理器
```

#### 数据采集
- **CPU**: `/proc/stat` 解析
- **内存**: `/proc/meminfo` 解析
- **磁盘**: `df` 命令 + `/proc/diskstats`
- **网络**: `/proc/net/dev` 解析
- **SMART**: `smartctl` 命令
- **进程**: `ps aux` 命令

#### WebSocket 实时推送
- 1 秒刷新间隔
- 自动重连机制（最多 5 次）
- 广播模式支持多客户端

#### SQLite 历史存储
- 表：`system_history`
- 字段：时间戳、CPU、内存、网络速度
- 自动清理：保留 90 天数据

### 前端 (HTML/JS)

#### 页面结构
```
webui/pages/dashboard.html
```

#### 技术栈
- **Chart.js**: 数据可视化
- **原生 WebSocket**: 实时通信
- **响应式设计**: 桌面/移动适配
- **CSS Grid/Flexbox**: 现代化布局

## API 接口

### WebSocket
```
GET /api/v1/system/ws
```
实时数据推送，连接后自动接收：
```json
{
  "type": "realtime",
  "system": {
    "cpuUsage": 25.5,
    "cpuCores": 4,
    "cpuTemp": 45,
    "memoryUsage": 62.3,
    "memoryTotal": 17179869184,
    "memoryUsed": 10703044608,
    "uptime": "5 天 12 小时 30 分钟",
    "loadAvg": [1.2, 1.5, 1.3],
    "processes": 156
  },
  "disks": [...],
  "network": [...],
  "timestamp": "2026-03-12T10:30:00Z"
}
```

### REST API

#### 系统统计
```
GET /api/v1/system/stats
```

#### 磁盘信息
```
GET /api/v1/system/disks
GET /api/v1/system/disks/smart/:device
POST /api/v1/system/disks/check
```

#### 网络统计
```
GET /api/v1/system/network
```

#### 进程列表
```
GET /api/v1/system/processes?limit=10
```

#### 历史数据
```
GET /api/v1/system/history?duration=24h&interval=1m
```
参数：
- `duration`: 24h | 7d | 30d
- `interval`: 数据间隔

#### 告警管理
```
GET /api/v1/system/alerts
POST /api/v1/system/alerts/:id/acknowledge
```

## 使用指南

### 访问仪表盘
1. 启动 NAS-OS: `./nasd`
2. 打开浏览器访问：`http://localhost:8080/pages/dashboard.html`

### 依赖安装
确保系统已安装：
```bash
# SMART 工具
sudo apt install smartmontools

# Go 依赖（自动安装）
go get github.com/gorilla/websocket
go get github.com/mattn/go-sqlite3
```

### 配置告警阈值
告警规则在 `handlers.go` 中定义，可自定义：
```go
alertRules: []*AlertRule{
    {Name: "cpu-warning", Type: "cpu", Threshold: 80, Level: "warning"},
    {Name: "cpu-critical", Type: "cpu", Threshold: 95, Level: "critical"},
    {Name: "memory-warning", Type: "memory", Threshold: 85, Level: "warning"},
    {Name: "disk-warning", Type: "disk", Threshold: 85, Level: "warning"},
}
```

## 性能优化

### 数据采集
- 使用 goroutine 并发采集
- 1 秒间隔平衡实时性和资源消耗
- 增量计算网络速度（避免重复查询）

### 数据存储
- SQLite 轻量级嵌入式数据库
- 自动清理 90 天前数据
- 索引优化查询性能

### WebSocket
- 连接池管理
- 自动断开无效连接
- 二进制消息压缩（可选）

## 扩展开发

### 添加新指标
1. 在 `monitor.go` 添加数据采集方法
2. 在 `RealTimeData` 结构体添加字段
3. 在 `dashboard.html` 添加显示逻辑

### 添加新告警类型
1. 在 `checkAlerts()` 添加检查逻辑
2. 在 `Alert` 结构体添加类型
3. 在前端添加告警样式

### 自定义图表
修改 `dashboard.html` 中的 Chart.js 配置：
```javascript
resourceChart = new Chart(ctx, {
    type: 'line', // 可改为 'bar', 'pie' 等
    data: {...},
    options: {...}
});
```

## 故障排查

### WebSocket 连接失败
- 检查防火墙是否允许 WebSocket
- 确认 API 路径正确：`/api/v1/system/ws`
- 查看浏览器控制台错误日志

### 数据不更新
- 检查 WebSocket 连接状态
- 确认后端数据采集 goroutine 运行
- 查看 SQLite 数据库是否正常写入

### SMART 数据无法读取
- 确认 `smartctl` 已安装
- 检查磁盘设备路径是否正确
- 确保有足够权限访问 `/dev/sd*`

## 安全考虑

- WebSocket 连接需要认证（生产环境）
- 告警数据限制最近 100 条
- SQLite 数据库文件权限保护
- API 速率限制防止滥用

## 未来规划

- [ ] 支持自定义监控指标
- [ ] 添加邮件/短信告警通知
- [ ] 支持监控数据导出（CSV/PDF）
- [ ] 添加容器/VM 监控
- [ ] 支持多节点集群监控
- [ ] 移动端 App 推送
