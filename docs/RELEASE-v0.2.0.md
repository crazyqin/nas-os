# NAS-OS v0.2.0 Alpha 发布说明

**发布日期**: 2026-03-10  
**版本类型**: Alpha (早期预览)  
**状态**: ✅ 已发布

---

## 🎉 概述

NAS-OS v0.2.0 Alpha 是项目的第二个 Alpha 版本，在 v0.1.0 的基础上实现了**基础文件共享能力**，使 NAS-OS 从"技术演示"升级为"可用工具"。

> ⚠️ **警告**: Alpha 版本不适合生产环境使用。数据可能丢失，功能可能变更。

---

## ✨ 新功能列表

### 1. 存储管理核心 (完整实现)
- ✅ btrfs 卷完整管理 (创建/删除/列表/详情)
- ✅ btrfs 子卷管理
- ✅ btrfs 快照管理
- ✅ 数据平衡 (balance)
- ✅ 数据校验 (scrub)
- 单元测试覆盖 >80%

### 2. 文件共享 (新增)
| 功能 | 状态 |
|------|------|
| SMB/CIFS 共享 | ✅ 已实现 |
| NFS 共享 | ✅ 已实现 |
| 共享权限配置 | ✅ 已实现 |
| 访客访问控制 | ✅ 已实现 |

**交付物**:
- `internal/smb/server.go` - Samba 集成
- `internal/nfs/server.go` - NFS 集成
- `internal/share/manager.go` - 共享管理
- 共享配置 API 端点

### 3. 配置持久化 (新增)
- ✅ 配置文件格式 (YAML)
- ✅ 配置加载/保存
- ✅ 配置热重载
- ✅ 配置备份/恢复

**交付物**:
- `internal/config/loader.go` - 配置加载
- `internal/config/saver.go` - 配置保存
- `internal/config/watcher.go` - 配置监听
- 配置文件示例

### 4. API 接口完善
- ✅ 存储 API v1.1 (完整实现)
- ✅ 共享 API v1.0 (新增)
- ✅ 配置 API v1.0 (新增)
- ✅ Swagger 文档

### 5. Web UI 改进
- ✅ 存储管理页面
- ✅ 共享配置页面
- ✅ 系统状态面板
- 基础响应式布局

---

## 📦 技术规格

### 系统要求
| 项目 | 要求 |
|------|------|
| 操作系统 | Linux (amd64/arm64/armv7) |
| 内核版本 | 5.10+ (btrfs 支持) |
| Go 版本 | 1.21+ |
| 内存 | 最小 2GB |
| 存储 | 最小 16GB 可用空间 |

### 依赖项
```go
// go.mod 核心依赖
github.com/gin-gonic/gin      // Web 框架
github.com/spf13/cobra        // CLI 工具
github.com/stretchr/testify   // 测试框架
github.com/go-playground/validator/v10 // 配置验证
```

### 构建产物
| 文件 | 大小 | 说明 |
|------|------|------|
| `nasd-linux-amd64` | ~22MB | 主守护进程 (x86_64) |
| `nasd-linux-arm64` | ~20MB | 主守护进程 (ARM64) |
| `nasd-linux-armv7` | ~21MB | 主守护进程 (ARMv7) |
| `nasctl` | ~15MB | 命令行工具 |
| Docker 镜像 | ~180MB | 容器化部署 |

---

## 🐛 已知问题

### 严重问题
| ID | 问题 | 影响 | 临时方案 | 计划修复 |
|----|------|------|----------|----------|
| #001 | 无用户认证系统 | 安全风险 | 仅限内网使用 | v0.3.0 |

### 中等问题
| ID | 问题 | 影响 | 临时方案 | 计划修复 |
|----|------|------|----------|----------|
| #011 | 无监控告警 | 无法感知故障 | 手动检查日志 | v0.6.0 |
| #012 | Web UI 简陋 | 用户体验差 | 使用 API/nasctl | v0.3.0 |

### 轻微问题
| ID | 问题 | 影响 | 临时方案 | 计划修复 |
|----|------|------|----------|----------|
| #020 | 文档不完整 | 学习成本高 | 查看源码注释 | 持续更新 |

---

## 📥 下载链接

### 二进制文件
| 平台 | 架构 | 下载 | SHA256 |
|------|------|------|--------|
| Linux | amd64 | [nasd-linux-amd64](https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-amd64) | `623ff6fe40bc1182d05808682079277dc64bef60692486ad991d9617000ec4f5` |
| Linux | arm64 | [nasd-linux-arm64](https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-arm64) | `1f6f0e78085de25294657dd8b17c4c1cdf57fc8b159e9389a468abb50eb0d59d` |
| Linux | armv7 | [nasd-linux-armv7](https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-armv7) | `84e9e9688d6f0589fe122a5d4bf10ed8fe061b18404437041a77ab326cd227b0` |

### Docker 镜像
```bash
# 拉取镜像
docker pull nas-os/nasd:v0.2.0
docker pull nas-os/nasd:alpha

# 运行容器
docker run -d \
  --name nasd \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  nas-os/nasd:v0.2.0
```

### 源码
```bash
# 克隆仓库
git clone https://github.com/nas-os/nasd.git
cd nasd
git checkout v0.2.0

# 构建
make build

# 运行测试
make test
```

---

## 🔧 升级指南

### 从 v0.1.0 升级到 v0.2.0

#### 方法一：二进制升级
```bash
# 1. 停止服务
sudo systemctl stop nas-os

# 2. 备份当前版本
sudo cp /usr/local/bin/nasd /usr/local/bin/nasd.bak

# 3. 下载新版本
sudo curl -L https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-amd64 \
    -o /usr/local/bin/nasd
sudo chmod +x /usr/local/bin/nasd

# 4. 备份配置文件 (v0.2.0 配置格式有变更)
sudo cp /etc/nas-os/config.yaml /etc/nas-os/config.yaml.bak

# 5. 复制新配置模板
sudo curl -L https://raw.githubusercontent.com/nas-os/nasd/v0.2.0/configs/default.yaml \
    -o /etc/nas-os/config.yaml.new

# 6. 迁移配置 (合并旧配置到新格式)
# 手动编辑 /etc/nas-os/config.yaml，保留自定义设置

# 7. 启动服务
sudo systemctl start nas-os

# 8. 验证版本
nasd --version
```

#### 方法二：Docker 升级
```bash
# 1. 停止旧容器
docker-compose down

# 2. 拉取新镜像
docker pull nas-os/nasd:v0.2.0

# 3. 更新 docker-compose.yml 中的镜像版本
# 将 image: nas-os/nasd:v0.1.0 改为 image: nas-os/nasd:v0.2.0

# 4. 启动新容器
docker-compose up -d

# 5. 查看日志
docker-compose logs -f
```

#### 方法三：源码编译
```bash
git clone https://github.com/nas-os/nasd.git
cd nasd
git checkout v0.2.0
make build
sudo make install
```

### 配置迁移

v0.2.0 引入了新的配置格式，主要变更：

```yaml
# v0.1.0 配置 (旧)
server:
  port: 8080
  host: 0.0.0.0

storage:
  data_path: /data

# v0.2.0 配置 (新)
version: "0.2.0"

server:
  port: 8080
  host: 0.0.0.0
  tls_enabled: false

storage:
  data_path: /data
  auto_scrub: true
  scrub_schedule: "0 3 * * 0"

shares:
  smb:
    enabled: true
    workgroup: WORKGROUP
  nfs:
    enabled: true
    allowed_networks:
      - 192.168.1.0/24

logging:
  level: info
  format: json
  file: /var/log/nas-os/nasd.log
```

---

## 📋 验证安装

### 健康检查
```bash
# 检查服务状态
curl http://localhost:8080/api/v1/health

# 预期响应
{"status":"ok","version":"v0.2.0"}
```

### 功能测试
```bash
# 1. 创建测试卷
nasctl volume create test_vol --path /data/test

# 2. 创建 SMB 共享
nasctl share create smb public --path /data/public --guest

# 3. 创建 NFS 共享
nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24

# 4. 列出共享
nasctl share list

# 5. 测试 SMB 访问 (从客户端)
# Windows: \\<服务器 IP>\public
# macOS: smb://<服务器 IP>/public
# Linux: smbclient //<服务器 IP>/public

# 6. 测试 NFS 访问 (从 Linux 客户端)
showmount -e <服务器 IP>
sudo mount <服务器 IP>:/backup /mnt/local_backup
```

---

## 📞 获取帮助

### 文档资源
- [安装指南](../docs/INSTALL.md)
- [API 文档](../docs/API.md)
- [常见问题](../docs/FAQ.md)
- [用户手册](./USER_GUIDE.md)

### 社区支持
- GitHub Issues: [提交问题](https://github.com/nas-os/nasd/issues)
- 讨论区: [GitHub Discussions](https://github.com/nas-os/nasd/discussions)

### 紧急联系
- 严重 Bug 报告：使用 GitHub Issues + `bug` + `critical` 标签
- 安全漏洞：发送邮件至 security@nas-os.dev

---

## 🗺️ 后续计划

### v0.3.0 Alpha (2026-04-20)
- Web UI 完整界面
- 用户认证系统
- API 文档 (Swagger)
- 移动端适配

### v0.4.0 Beta (2026-05-10)
- 磁盘监控告警
- 日志审计
- 系统设置界面
- 性能优化

### v1.0.0 Stable (2026-06-30)
- 生产就绪
- 完整功能集
- 安全审计通过
- 长期支持 (LTS)

详细路线图请查看 [RELEASE_PLAN.md](./RELEASE_PLAN.md)

---

## 📝 变更日志

### v0.2.0 (2026-03-10)
**新增**
- SMB/CIFS 文件共享支持
- NFS 文件共享支持
- 配置持久化系统 (YAML)
- 配置热重载功能
- 共享权限管理
- 访客访问控制
- Web UI 存储管理页面
- Web UI 共享配置页面
- 系统状态面板
- Swagger API 文档

**改进**
- btrfs 卷管理完善
- btrfs 子卷/快照功能
- 数据平衡 (balance) 功能
- 数据校验 (scrub) 功能
- 单元测试覆盖 >80%
- API 端点完善

**修复**
- 配置未持久化问题 (v0.1.0 #002)
- 多个 btrfs 操作边界情况

**已知限制**
- 无用户认证 (计划 v0.3.0)
- 无监控告警 (计划 v0.6.0)
- Web UI 简陋 (计划 v0.3.0)

---

## ⚖️ 许可证

MIT License - 详见 [LICENSE](../LICENSE)

---

*发布团队：吏部*  
*发布日期：2026-03-10*  
*文档版本：1.0*
