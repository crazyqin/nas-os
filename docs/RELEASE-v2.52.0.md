# NAS-OS v2.52.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新功能

### 系统监控仪表板 (Dashboard)

全新的监控仪表板系统，提供实时系统状态可视化：

- **小组件系统**: 支持 CPU、内存、磁盘、网络等多种监控小组件
- **自定义布局**: 可拖拽配置仪表板布局，支持多种尺寸小组件
- **实时更新**: 可配置刷新率，默认 5 秒更新一次
- **数据持久化**: 仪表板配置自动保存，重启后恢复
- **事件订阅**: 支持订阅仪表板数据变化事件

**代码位置**: `internal/dashboard/`
- `manager.go` - 仪表板管理器核心
- `types.go` - 数据类型定义
- `widgets.go` - 小组件数据提供者

### 健康检查器集成

增强的系统健康检查模块：

- **健康评分系统**: 综合评估系统健康状态
- **多维度检查**: CPU、内存、磁盘、网络、服务状态等
- **阈值告警**: 可配置告警阈值，异常自动通知
- **API 端点**: 提供完整的健康检查 API

**代码位置**: `internal/health/`
- `health.go` - 健康检查核心逻辑
- `handlers.go` - HTTP API 端点
- `health_test.go` - 完整测试覆盖

## 修复

### CI/CD 优化

- 修复 govet 静态检查错误
- 代码格式规范化
- 构建流程优化

## API 变更

### 新增端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/dashboard | 获取仪表板列表 |
| POST | /api/v1/dashboard | 创建仪表板 |
| GET | /api/v1/dashboard/:id | 获取仪表板详情 |
| PUT | /api/v1/dashboard/:id | 更新仪表板 |
| DELETE | /api/v1/dashboard/:id | 删除仪表板 |
| GET | /api/v1/dashboard/:id/widgets | 获取小组件数据 |
| GET | /api/v1/health | 系统健康检查 |

## 数据结构

### 仪表板配置

```json
{
  "id": "default",
  "name": "系统概览",
  "widgets": [
    {
      "id": "cpu-1",
      "type": "cpu",
      "title": "CPU 使用率",
      "size": "medium",
      "position": {"x": 0, "y": 0},
      "config": {
        "showPerCore": true,
        "warningThreshold": 80,
        "criticalThreshold": 95
      }
    }
  ],
  "layout": {
    "columns": 4,
    "rows": 3,
    "gap": 16
  }
}
```

### 健康检查响应

```json
{
  "status": "healthy",
  "score": 95,
  "checks": [
    {
      "name": "cpu",
      "status": "ok",
      "value": 45.2,
      "message": "CPU 使用率正常"
    },
    {
      "name": "memory",
      "status": "warning",
      "value": 85.5,
      "message": "内存使用率较高"
    }
  ]
}
```

## 升级说明

### 从 v2.50.0 升级

1. 停止现有服务
2. 替换二进制文件或更新 Docker 镜像
3. 启动服务，仪表板数据将自动迁移

### 兼容性

- 无破坏性 API 变更
- 现有配置文件兼容
- 数据库 schema 无变更

## 下载

### 二进制文件

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.52.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.52.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.52.0/nasd-linux-armv7) |

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.52.0
```

## 贡献者

- 礼部 - 系统监控仪表板
- 工部 - 健康检查器集成、CI/CD 修复

---

**完整变更日志**: [CHANGELOG.md](./CHANGELOG.md)