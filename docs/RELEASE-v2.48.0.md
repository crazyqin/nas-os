# NAS-OS v2.48.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable  

## 🚀 新功能

### 项目管理增强 (吏部)

- **项目管理器核心** (`internal/project/manager.go`)
  - 项目生命周期管理（创建/更新/删除/归档/恢复）
  - 项目成员管理与权限控制
  - 项目统计与进度追踪

- **项目数据类型** (`internal/project/types.go`)
  - 完整的项目数据模型定义
  - 项目状态枚举
  - 项目成员角色定义

- **项目管理测试** (`internal/project/manager_test.go`)
  - 项目创建/更新/删除测试
  - 成员管理测试
  - 权限控制测试

## 📋 项目管理 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/projects | 获取项目列表 |
| POST | /api/v1/projects | 创建项目 |
| GET | /api/v1/projects/:id | 获取项目详情 |
| PUT | /api/v1/projects/:id | 更新项目 |
| DELETE | /api/v1/projects/:id | 删除项目 |
| POST | /api/v1/projects/:id/archive | 归档项目 |
| POST | /api/v1/projects/:id/restore | 恢复项目 |
| GET | /api/v1/projects/:id/members | 获取项目成员 |
| POST | /api/v1/projects/:id/members | 添加项目成员 |
| DELETE | /api/v1/projects/:id/members/:userId | 移除项目成员 |
| GET | /api/v1/projects/:id/stats | 获取项目统计 |

## 📥 安装方式

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.48.0
```

### Docker Compose

```yaml
services:
  nas-os:
    image: ghcr.io/crazyqin/nas-os:v2.48.0
    # ...
```

### 二进制文件

```bash
# Linux AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.48.0/nasd-linux-amd64

# Linux ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.48.0/nasd-linux-arm64

# Linux ARMv7
wget https://github.com/crazyqin/nas-os/releases/download/v2.48.0/nasd-linux-armv7
```

## 📝 更新日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

| 平台 | 下载地址 |
|------|----------|
| Docker | ghcr.io/crazyqin/nas-os:v2.48.0 |
| Linux AMD64 | nasd-linux-amd64 |
| Linux ARM64 | nasd-linux-arm64 |
| Linux ARMv7 | nasd-linux-armv7 |