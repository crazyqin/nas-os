# NAS-OS v0.2.0 截图指南

**用途**: 产品演示视频、Product Hunt 页面、博客文章、社交媒体  
**分辨率**: 1920x1080 (视频) / 1200x630 (社交媒体)  
**格式**: PNG (无损) / JPEG (压缩，用于网页)

---

## 📸 必截页面清单

### 1. 首页/仪表盘 (Dashboard)
**URL**: `http://localhost:8080`

**截图内容**:
- 系统状态概览 (CPU/内存/磁盘使用率)
- 存储空间可视化图表
- 活跃共享列表
- 快速操作按钮

**拍摄要点**:
- 确保有真实数据（创建几个测试卷和共享）
- 浏览器窗口最大化
- 隐藏浏览器地址栏和书签栏 (F11 全屏)

**文件名**: `01-dashboard.png`

---

### 2. 存储管理 - 卷列表
**URL**: `http://localhost:8080/storage/volumes`

**截图内容**:
- 卷列表表格
- 每个卷的容量、使用率、状态
- "创建卷"按钮

**拍摄要点**:
- 创建 2-3 个测试卷，展示不同状态
- 确保 btrfs 特性标签可见

**文件名**: `02-volumes-list.png`

---

### 3. 存储管理 - 卷详情
**URL**: `http://localhost:8080/storage/volumes/{name}`

**截图内容**:
- 卷详细信息
- 子卷列表
- 快照列表
- 操作按钮 (平衡/校验)

**拍摄要点**:
- 选择一个有子卷和快照的卷
- 展示完整的功能入口

**文件名**: `03-volume-detail.png`

---

### 4. 文件共享 - SMB 配置
**URL**: `http://localhost:8080/shares/smb`

**截图内容**:
- SMB 共享列表
- "新建 SMB 共享"按钮
- 共享状态 (启用/禁用)

**拍摄要点**:
- 创建 2 个测试共享
- 展示访客访问权限标识

**文件名**: `04-smb-shares.png`

---

### 5. 文件共享 - 新建 SMB 表单
**URL**: `http://localhost:8080/shares/smb/new`

**截图内容**:
- 共享名称输入框
- 路径选择器
- 权限选项 (访客访问、用户权限)
- 提交按钮

**拍摄要点**:
- 填写示例数据
- 展示所有配置选项

**文件名**: `05-new-smb-form.png`

---

### 6. 文件共享 - NFS 配置
**URL**: `http://localhost:8080/shares/nfs`

**截图内容**:
- NFS 共享列表
- "新建 NFS 共享"按钮
- 客户端网段配置

**拍摄要点**:
- 创建 1-2 个测试共享
- 展示客户端访问控制列表

**文件名**: `06-nfs-shares.png`

---

### 7. 配置管理
**URL**: `http://localhost:8080/settings/config`

**截图内容**:
- 配置文件预览 (YAML 格式)
- 编辑按钮
- 保存/重载按钮

**拍摄要点**:
- 展示配置持久化特性
- 高亮显示 shares 配置部分

**文件名**: `07-config.png`

---

### 8. 系统设置
**URL**: `http://localhost:8080/settings`

**截图内容**:
- 服务器配置 (端口、主机)
- 日志级别设置
- 系统信息 (版本号 v0.2.0)

**拍摄要点**:
- 确保版本号清晰可见
- 展示完整的设置选项

**文件名**: `08-system-settings.png`

---

### 9. API 文档 (Swagger)
**URL**: `http://localhost:8080/swagger`

**截图内容**:
- Swagger UI 界面
- API 端点列表
- "Try it out" 按钮

**拍摄要点**:
- 展开 2-3 个 API 端点
- 展示请求/响应示例

**文件名**: `09-swagger-api.png`

---

### 10. 命令行工具演示
**场景**: 终端窗口

**截图内容**:
```bash
# 版本信息
nasctl version

# 卷列表
nasctl volume list

# 创建共享
nasctl share smb create public /data/public --guest-ok

# 健康检查
nasctl health
```

**拍摄要点**:
- 使用深色主题终端
- 字体大小适中 (14-16px)
- 展示命令和输出

**文件名**: `10-cli-demo.png`

---

## 🎨 截图规范

### 通用要求
- **分辨率**: 1920x1080 (视频) 或 1200x630 (社交媒体)
- **格式**: PNG (首选) 或 JPEG (质量 90%+)
- **浏览器**: Chrome/Chromium 最新版的
- **主题**: 浅色/浅色模式 (保持统一)
- **语言**: 中文界面

### 浏览器设置
```
1. 打开开发者工具 (F12)
2. 切换到设备工具栏 (Ctrl+Shift+M)
3. 设置分辨率为 1920x1080
4. 禁用缓存
5. 清除所有通知和弹窗
```

### 数据准备
在截图前，确保系统有以下测试数据：
- 2-3 个 btrfs 卷
- 每个卷有 1-2 个子卷
- 每个卷有 1 个快照
- 2 个 SMB 共享
- 1 个 NFS 共享
- 配置文件有完整内容

---

## 🛠️ 截图工具推荐

### Linux
```bash
# 全屏截图
gnome-screenshot -f dashboard.png -d 3

# 区域截图
gnome-screenshot -a -f smb-form.png

# 窗口截图
gnome-screenshot -w -f volume-detail.png
```

### macOS
```bash
# 全屏截图
cmd + shift + 3

# 区域截图
cmd + shift + 4

# 窗口截图
cmd + shift + 4, 然后按空格
```

### 跨平台 (推荐)
- **Flameshot** (Linux/Windows): `flameshot gui`
- **ShareX** (Windows): 功能强大
- **CleanShot X** (macOS): 专业截图工具

---

## 📁 文件组织

```
nas-os/docs/screenshots/v0.2.0/
├── 01-dashboard.png
├── 02-volumes-list.png
├── 03-volume-detail.png
├── 04-smb-shares.png
├── 05-new-smb-form.png
├── 06-nfs-shares.png
├── 07-config.png
├── 08-system-settings.png
├── 09-swagger-api.png
├── 10-cli-demo.png
└── README.md (本文件)
```

---

## 🎬 视频录制额外提示

### 录屏软件推荐
- **OBS Studio**: 免费开源，功能强大
- **ScreenFlow** (macOS): 专业视频编辑
- **Camtasia**: 商业软件，易用

### 录制设置
- **帧率**: 30fps 或 60fps
- **分辨率**: 1920x1080
- **格式**: MP4 (H.264 编码)
- **鼠标**: 启用鼠标高亮效果

### 录制流程
1. 准备测试数据
2. 清空浏览器历史记录
3. 关闭无关通知
4. 测试录音 (如需旁白)
5. 按脚本顺序录制
6. 保留原始素材 (不要边录边剪)

---

## ✅ 截图检查清单

截图完成后，检查以下内容：
- [ ] 所有文字清晰可读
- [ ] 没有个人隐私信息
- [ ] 版本号显示为 v0.2.0
- [ ] 浏览器地址栏隐藏 (视频用)
- [ ] 文件命名规范
- [ ] 图片质量符合要求
- [ ] 已备份原始文件

---

*文档版本：1.0*  
*创建日期：2026-03-10*  
*礼部 制作*
