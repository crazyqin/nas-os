# 更新日志

所有重要的更改都将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.291.0] - 2026-03-28

### 新增
- 🧑 **人脸识别服务** (兵部) - 开发启动
  - 人脸检测服务框架 (internal/ai/face/service.go)
  - 人脸聚类算法（基于相似度阈值）
  - 人脸相册API设计
  - 人脸标签管理功能
- 🔒 **人脸隐私合规** (刑部)
  - 知情同意机制 (internal/ai/face/privacy.go)
  - 人脸数据加密存储设计
  - 人脸数据删除/导出API
  - 隐私政策说明页面
- 📦 **应用模板扩展** (工部)
  - jellyfin.yml - 媒体服务器
  - homeassistant.yml - 智能家居
  - vaultwarden.yml - 密码管理
  - nextcloud.yml - 私有云盘
  - transmission.yml - 下载器
- 📝 **六部任务分配** (吏部)
  - docs/TASK_ASSIGNMENT_v2.291.0.md

### 竞品学习
- **群晖 DSM 7.3**: Photos人脸识别、套件中心设计
- **飞牛 fnOS 1.1**: 本地AI人脸识别（Intel核显加速）
- **TrueNAS 24.10**: Docker Compose原生支持

### 文档
- 📝 ROADMAP.md 更新 - 人脸识别里程碑规划
- 📝 MILESTONES.md 更新 - v2.291.0记录

---

## [v2.290.0] - 2026-03-28

### 新增
- 📦 **应用中心框架** (兵部) - 开发启动
  - 应用服务管理器设计
  - 应用目录结构规划
  - 应用安装器框架
- 🐳 **应用模板设计** (工部)
  - 默认应用模板（nginx/postgres/redis/qdrant）
- 🎨 **应用中心 WebUI完善** (礼部) - 功能实现
  - 应用目录展示（卡片布局、图标+描述+分类）
  - 分类筛选（全部/媒体/生产力/智能家居/网络/下载/开发/安全）
  - 已安装应用管理面板（状态监控、启动/停止/重启/卸载）
  - 安装配置弹窗（端口自定义、存储卷映射、环境变量配置）
  - 应用状态实时监控（运行中/已停止/错误状态指示）
  - 手动安装支持（Docker Compose/Docker镜像双模式）
  - 应用详情查看（端口映射、存储卷、访问地址）
  - 一键启动/停止/重启/更新/卸载操作
  - 定时状态刷新（30秒轮询）
- 🔒 **应用安全设计** (刑部)
  - 应用权限隔离方案
  - 应用沙箱设计
- 📊 **应用资源统计** (户部)
  - 应用 CPU/内存/存储统计接口

### 文档
- 📝 竞品分析报告更新 (docs/COMPETITOR_ANALYSIS.md)
  - 飞牛fnOS 1.1正式版动态更新
  - 功能对比矩阵更新（应用中心改版、Cloudflare Tunnel）
- 📝 六部任务分配 (docs/TASK_ASSIGNMENT_v2.290.0.md)

---

## [v2.289.0] - 2026-03-28

### 六部协同开发 - 第67轮

#### 吏部（项目管理）
- 版本号更新：v2.288.2 → v2.289.0
- MILESTONES.md 里程碑记录更新

#### 兵部（软件工程）
- go vet 0 错误
- go test 全部通过

#### 礼部（文档品牌）
- README.md 版本号同步至 v2.289.0
- Docker badge 更新

#### 刑部（安全审计）
- govulncheck 安全检查完成

#### 工部（DevOps）
- Dockerfile 语法检查
- docker-compose 配置验证

#### 户部（资源统计）
- 项目统计报告生成

---

## [v2.288.2] - 2026-03-28

### 新增
- 💾 **NVMe-oF 存储模块** (兵部)
  - NVMe over Fabrics 支持
  - 高性能存储网络
- 🐳 **Docker 构建优化** (工部)
  - 多架构镜像构建改进
  - AI 服务镜像分离


[1027 more lines in file.4270 more lines total. Use offset=81 to continue.]