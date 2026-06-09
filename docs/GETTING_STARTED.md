# NAS-OS 快速入门

> 5 分钟从零到可用的家用 NAS。

## 前提条件

- x86_64 或 ARM64 设备（树莓派 4B+、x86 小主机均可）
- 一块数据盘（建议 ≥1TB）
- 一条网线

## 安装

### 方式一：Docker（推荐）

```bash
# 创建数据目录
mkdir -p ~/nas-data

# 启动 NAS-OS
docker run -d \
  --name nas-os \
  --privileged \
  -p 8080:8080 \
  -p 445:445 \
  -p 2049:2049 \
  -v ~/nas-data:/data \
  -v /dev:/dev \
  ghcr.io/crazyqin/nas-os:latest
```

### 方式二：二进制

```bash
# 下载最新版
curl -LO https://github.com/crazyqin/nas-os/releases/latest/download/nasd-linux-amd64

# 赋权
chmod +x nasd-linux-amd64

# 启动
sudo ./nasd-linux-amd64
```

## 首次访问

1. 浏览器打开 `http://YOUR_IP:8080`
2. 默认账号: `admin` / `admin`
3. **立即修改密码！**

## 初始化向导

NAS-OS 提供智能引导初始化（7 步）：

1. **语言选择** - 中文/English
2. **管理员设置** - 修改默认密码
3. **网络配置** - DHCP 或静态 IP
4. **存储池创建** - 选择磁盘，创建 btrfs 池
5. **共享设置** - 创建第一个 SMB 共享
6. **Docker 配置** - 可选，启用容器管理
7. **完成** - 系统就绪

## 核心操作

### 创建存储池

```bash
# 通过 Web 界面：存储 → 存储池 → 创建
# 或 CLI：
nasctl pool create --name main --disks /dev/sda /dev/sdb --raid raid1
```

### 创建共享

```bash
# SMB 共享
nasctl share create --name documents --path /data/documents --protocol smb

# NFS 共享
nasctl share create --name media --path /data/media --protocol nfs
```

### 启用 AI 功能

```bash
# 安装 Ollama（本地 LLM）
nasctl ai install-ollama

# 下载模型
nasctl ai pull-model --name qwen2.5:7b

# 启用 AI 相册
nasctl ai enable-photo-search
```

## 下一步

- [完整文档](docs/) - 详细功能说明
- [API 文档](docs/api/) - 开发者接口
- [品牌指南](docs/BRAND_GUIDELINES.md) - 品牌资源

---

*版本: v2.582.0 | 最后更新: 2026-06-09*
