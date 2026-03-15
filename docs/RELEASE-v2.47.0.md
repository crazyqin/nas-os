# NAS-OS v2.47.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 新增功能

### 📋 项目管理 API (吏部)

全新的项目管理系统，支持任务追踪、里程碑管理和项目统计仪表板。

#### 核心功能

| 功能 | 说明 |
|------|------|
| 📊 项目管理 | 项目 CRUD 操作，成员管理，项目统计 |
| ✅ 任务追踪 | 任务创建/更新/删除/查询，状态流转 |
| 🎯 里程碑 | 里程碑创建/更新/删除/查询，进度追踪 |
| 📈 统计仪表板 | 项目进度、任务分布、完成率统计 |

#### 任务状态流转

```
待办 (todo) → 进行中 (in_progress) → 审核 (review) → 完成 (done)
                                              ↘ 取消 (cancelled)
```

#### API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/projects | 获取项目列表 |
| POST | /api/v1/projects | 创建项目 |
| GET | /api/v1/projects/:id | 获取项目详情 |
| PUT | /api/v1/projects/:id | 更新项目 |
| DELETE | /api/v1/projects/:id | 删除项目 |
| GET | /api/v1/projects/:id/stats | 获取项目统计 |
| GET | /api/v1/projects/:id/milestones | 获取里程碑列表 |
| POST | /api/v1/projects/:id/milestones | 创建里程碑 |
| GET | /api/v1/projects/:id/tasks | 获取任务列表 |
| POST | /api/v1/projects/:id/tasks | 创建任务 |

### 🔧 版本同步

- README.md 版本号更新至 v2.47.0
- Docker 镜像标签更新
- 下载链接更新至最新版本

## 升级说明

### Docker 用户

```bash
# 拉取新镜像
docker pull ghcr.io/crazyqin/nas-os:v2.47.0

# 停止旧容器
docker stop nasd

# 启动新容器
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.47.0
```

### 二进制用户

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.47.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

## 下载

| 平台 | 文件 |
|------|------|
| Linux AMD64 | nasd-linux-amd64 |
| Linux ARM64 | nasd-linux-arm64 |
| Linux ARMv7 | nasd-linux-armv7 |
| Docker | ghcr.io/crazyqin/nas-os:v2.47.0 |

## 完整变更日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

**吏部** - 项目管理与创业孵化  
**礼部** - 品牌营销与内容创作