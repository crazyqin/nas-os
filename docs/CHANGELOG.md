# NAS-OS 变更日志

所有重要的项目变更将记录在此文件中。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [未发布]

### 新增
- 项目初始化
- btrfs 存储管理框架
- Web 服务框架
- 项目文档体系

### 改进
- 无

### 修复
- 无

### 安全
- 无

---

## [0.1.0] - 2026-03-10

### 新增
- 项目骨架创建
- Go 模块配置 (go.mod/go.sum)
- 目录结构:
  - `cmd/nasd/` - 主程序入口
  - `internal/storage/` - 存储管理模块
  - `internal/web/` - Web 服务模块
  - `pkg/` - 公共库
  - `webui/` - 前端界面
  - `configs/` - 配置文件
- README.md 项目说明
- MILESTONES.md 项目里程碑
- docs/TASKS.md 六部任务分配
- docs/README.md 文档索引
- docs/PROGRESS.md 进度跟踪

### 技术栈
- **后端**: Go 1.21+
- **Web 框架**: Gin (待定)
- **存储**: btrfs
- **前端**: Vue 3 + Ant Design Vue (计划)

---

## 版本说明

### 版本号规则
- **主版本号**: 重大功能更新或不兼容变更
- **次版本号**: 新功能添加 (向下兼容)
- **修订号**: Bug 修复和小改进

### 发布周期
- **Alpha**: 每两周 (功能开发阶段)
- **Beta**: 每月 (功能完善阶段)
- **Stable**: 每季度 (稳定版本)

### 发布流程
1. 代码冻结
2. 测试验证
3. 文档更新
4. 版本发布
5. 变更日志更新

---

*维护者：吏部*  
*最后更新：2026-03-10*
