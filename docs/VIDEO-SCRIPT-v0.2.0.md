# NAS-OS v0.2.0 产品演示视频脚本

**视频时长**: 3-4 分钟  
**目标受众**: 技术爱好者、NAS 用户、开发者  
**发布平台**: YouTube, Bilibili, Twitter, Product Hunt

---

## 🎬 开场 (0:00 - 0:15)

**画面**: 黑色背景，白色文字逐行出现
```
你还在为群晖的昂贵价格犹豫吗？
你想拥有完全可控的 NAS 系统吗？
```

**画面切换**: NAS-OS Logo + v0.2.0 版本号

**旁白**:
> "今天，我们带来 NAS-OS v0.2.0 Alpha —— 一个完全开源、免费的家用 NAS 系统。"

---

## 📦 第一部分：什么是 NAS-OS (0:15 - 0:45)

**画面**: 项目 GitHub 页面截图 + 特性列表动画

**旁白**:
> "NAS-OS 是一个基于 Go 语言开发的轻量级 NAS 操作系统。
> 它支持 btrfs 存储管理、SMB/NFS 文件共享，以及简洁的 Web 管理界面。
> 完全开源，完全免费，完全由你掌控。"

**画面**: 架构简图（后端 Go + 前端 Web UI + btrfs）

**字幕**:
- ✅ 开源免费 (MIT License)
- ✅ 轻量级 (二进制 <30MB)
- ✅ 支持 amd64/arm64
- ✅ Docker 一键部署

---

## 🔥 第二部分：v0.2.0 新功能演示 (0:45 - 2:30)

### 2.1 文件共享 - SMB (0:45 - 1:15)

**画面**: Web UI 操作录屏
1. 打开 http://localhost:8080
2. 点击"文件共享"标签
3. 点击"新建 SMB 共享"
4. 填写表单：共享名=public, 路径=/data/public, 允许访客访问=✓
5. 点击"创建"

**旁白**:
> "v0.2.0 的核心功能是文件共享。
> 只需几步，就能创建一个 SMB 共享。
> 支持访客访问、用户权限控制。"

**画面切换**: Windows 资源管理器
- 地址栏输入：\\192.168.1.100\public
- 成功访问共享文件夹
- 拖拽文件进去

**字幕**: ✅ 从 Windows/macOS/Linux 无缝访问

---

### 2.2 文件共享 - NFS (1:15 - 1:45)

**画面**: Web UI 操作录屏
1. 点击"新建 NFS 共享"
2. 填写表单：路径=/data/nfs, 允许客户端=192.168.1.0/24
3. 点击"创建"

**旁白**:
> "对于 Linux 用户，NFS 共享更加高效。
> 配置网段访问控制，安全又便捷。"

**画面切换**: Linux 终端
```bash
sudo mount -t nfs 192.168.1.100:/data/nfs /mnt/nas
df -h  # 显示挂载成功
```

**字幕**: ✅ Linux 原生支持，高性能传输

---

### 2.3 配置持久化 (1:45 - 2:10)

**画面**: 终端操作录屏
```bash
# 查看配置文件
cat /etc/nas-os/config.yaml

# 修改配置
sudo nano /etc/nas-os/config.yaml

# 重启服务
sudo systemctl restart nas-os

# 验证配置保留
curl http://localhost:8080/api/v1/shares
```

**旁白**:
> "v0.2.0 实现了配置持久化。
> 所有设置保存在 YAML 配置文件中，重启不丢失。
> 支持热重载，修改配置无需重启服务。"

**画面**: 配置文件示例（高亮显示 shares 部分）

---

### 2.4 btrfs 存储管理 (2:10 - 2:30)

**画面**: Web UI 存储管理页面
- 显示卷列表
- 创建子卷
- 创建快照
- 查看空间使用情况

**旁白**:
> "基于 btrfs 文件系统，NAS-OS 提供高级存储管理功能。
> 子卷、快照、数据平衡、数据校验，一应俱全。"

**字幕**:
- 📁 子卷管理
- 📸 快照备份
- ⚖️ 数据平衡
- ✅ 数据校验

---

## 🚀 第三部分：快速开始 (2:30 - 3:00)

**画面**: 三种安装方式并列展示

### 方式一：Docker (推荐)
```bash
docker run -d \
  --name nasd \
  -p 8080:8080 \
  -v /data:/data \
  nas-os/nasd:v0.2.0
```

### 方式二：二进制
```bash
wget https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo ./nasd-linux-amd64
```

### 方式三：源码编译
```bash
git clone https://github.com/nas-os/nasd.git
cd nasd && make build
sudo make install
```

**旁白**:
> "三种安装方式，总有一款适合你。
> 推荐 Docker 部署，一键启动。"

---

## 🎯 结尾：路线图 & 呼吁行动 (3:00 - 3:30)

**画面**: 路线图时间轴动画

**旁白**:
> "v0.2.0 只是开始。
> v0.3.0 将带来完整的 Web UI 和用户认证系统。
> v1.0.0 将在 6 月 30 日发布，生产就绪。"

**画面**: GitHub 星标动画 + Discord 二维码

**旁白**:
> "如果你觉得这个项目有趣，请：
> - 在 GitHub 上给我们一个 ⭐
> - 加入 Discord 社区参与讨论
> - 在 Product Hunt 上为我们投票
> 
> 你的支持，是我们前进的动力！"

**画面**: NAS-OS Logo + "开源 · 免费 · 可控" + GitHub 链接

**字幕**: 
- GitHub: github.com/nas-os/nasd
- Discord: [二维码]
- 许可证：MIT

---

## 📋 拍摄清单

### 录屏内容
- [ ] Web UI 完整操作流程
- [ ] Windows 访问 SMB 共享
- [ ] Linux 挂载 NFS 共享
- [ ] 配置文件编辑演示
- [ ] btrfs 管理界面

### 素材准备
- [ ] NAS-OS Logo (SVG/PNG)
- [ ] 项目架构图
- [ ] 路线图时间轴
- [ ] GitHub/Discord 二维码

### 后期制作
- [ ] 添加字幕
- [ ] 背景音乐 (轻快科技感)
- [ ] 转场动画
- [ ] 片头片尾

---

## 🎵 推荐背景音乐
- YouTube Audio Library: "Tech Talk" 或 "Innovation"
- 风格：轻快、科技感、不抢旁白风头

---

*脚本版本：1.0*  
*创建日期：2026-03-10*  
*礼部 制作*
