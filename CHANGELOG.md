# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.253.43] - 2026-03-20

### Security
- 修复 G115 整数溢出转换漏洞 (兵部)
  - internal/backup/smart_manager_v2_unix.go: 使用 SafeMulUint64 安全计算磁盘空间
  - internal/optimizer/optimizer.go: 使用 SafeUint64ToInt64 安全转换 GC 统计
  - internal/quota/optimizer/optimizer.go: 重构差值计算避免溢出
  - internal/monitor/disk_health.go: 使用 SafeUint64ToInt 安全转换温度值

### Maintenance
- 六部协同例行维护检查
- 代码质量: go vet 通过，编译成功
- 安全审计: 无硬编码敏感信息
- CI/CD配置: Go 1.25.0 一致性确认
- 资源统计: 411,836行代码, 264测试文件, 68模块

### Improvements
- 六部协同开发流程优化
- CI/CD 配置检查完善 (工部)
- 文档版本同步检查 (礼部)
- 安全审计报告生成 (刑部)
- 代码量和测试覆盖率统计 (户部)

## [v2.253.42] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.42 (礼部)
  - VERSION、internal/version/version.go
  - README.md、docs/README.md
  - docs/USER_GUIDE.md、docs/api.yaml

## [v2.253.40] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.40 (礼部)
  - README.md、docs/README_EN.md
  - docs/USER_GUIDE.md、docs/QUICKSTART.md
  - docs/FAQ.md、docs/API_GUIDE.md
  - docs/api.yaml、docs/swagger.yaml

## [v2.253.38] - 2026-03-19

### Documentation
- 同步 README.md 版本号至 v2.253.38 (礼部)
- 同步 docs/USER_GUIDE.md 版本号至 v2.253.38 (礼部)

## [v2.253.35] - 2026-03-19

### Documentation
- 同步 README.md 版本号至 v2.253.35 (礼部)
- 同步 docs/README.md 版本号至 v2.253.35 (礼部)
- 同步 docs/README_EN.md 版本号至 v2.253.35 (礼部)
- 同步 docs/USER_GUIDE.md 版本号至 v2.253.35 (礼部)
- 同步 docs/QUICKSTART.md 版本号至 v2.253.35 (礼部)
- 同步 docs/FAQ.md 版本号至 v2.253.35 (礼部)
- 同步 docs/API_GUIDE.md 版本号至 v2.253.35 (礼部)
- 同步 docs/api.yaml 版本号至 2.253.35 (礼部)
- 同步 docs/swagger.yaml 版本号至 2.253.35 (礼部)
- 更新 ROADMAP.md 当前版本至 v2.253.35 (礼部)

## [v2.253.33] - 2026-03-19

### Documentation
- 更新 README.md 版本号至 v2.253.33 (礼部)
- 更新 README.md 下载链接和 Docker badge 至 v2.253.33 (礼部)
- 更新 docs/README.md 版本号至 v2.253.33 (礼部)
- 更新 docs/README_EN.md 版本号和下载链接至 v2.253.33 (礼部)
- 更新 docs/USER_GUIDE.md 版本号至 v2.253.33 (礼部)
- 更新 docs/QUICKSTART.md 版本号和下载链接至 v2.253.33 (礼部)
- 更新 docs/FAQ.md 版本号至 v2.253.33 (礼部)
- 更新 docs/API_GUIDE.md 版本号至 v2.253.33 (礼部)
- 更新 docs/api.yaml 版本号至 2.253.33 (礼部)
- 更新 internal/version/version.go 版本号至 2.253.33 (礼部)