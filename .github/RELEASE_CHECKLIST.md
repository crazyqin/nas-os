# 发布检查清单

## 发布前准备

- [ ] 所有测试通过（单元测试 + 集成测试）
- [ ] 代码覆盖率 ≥ 50%
- [ ] 安全扫描无严重问题
- [ ] 更新版本号（遵循语义化版本）
- [ ] 更新 CHANGELOG.md
- [ ] 更新文档（README、DEPLOYMENT 等）
- [ ] 审查所有 PR 和 Issue

## 发布流程

### 1. 创建发布分支（如需要）
```bash
git checkout -b release/v1.0.0
```

### 2. 更新版本号
- [ ] 在代码中更新版本号
- [ ] 在 Dockerfile 中更新版本标签
- [ ] 在文档中更新版本引用

### 3. 运行本地测试
```bash
make test-all
make lint
make docker-build
```

### 4. 提交并推送
```bash
git commit -am "chore: prepare release v1.0.0"
git push origin release/v1.0.0
```

### 5. 创建 Pull Request
- [ ] 创建 PR 到 main 分支
- [ ] 等待 CI 通过
- [ ] 获得至少 1 个审查通过
- [ ] 合并 PR

### 6. 打标签并发布
```bash
git checkout main
git pull origin main
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 7. 验证自动发布
- [ ] GitHub Actions 构建完成
- [ ] Docker 镜像推送成功
- [ ] Release 页面显示正确
- [ ] 所有架构的二进制文件已上传

## 发布后任务

- [ ] 更新官网/文档站点
- [ ] 发布博客/公告（如需要）
- [ ] 通知社区（Discord、邮件列表等）
- [ ] 监控错误报告和反馈
- [ ] 创建下一个里程碑

## 紧急回滚流程

如果发布后发现严重问题：

1. **立即创建回滚标签**
   ```bash
   git tag -a v1.0.1-rollback -m "Rollback v1.0.0"
   git push origin v1.0.1-rollback
   ```

2. **标记问题版本为 Pre-release**
   - 在 GitHub Releases 页面编辑
   - 勾选 "Pre-release"

3. **发布热修复版本**
   - 创建 hotfix 分支
   - 修复问题
   - 快速通道发布 v1.0.1

## 版本命名规范

遵循 [语义化版本 2.0.0](https://semver.org/)：

- **MAJOR.MINOR.PATCH** (例如：1.2.3)
- **MAJOR**: 不兼容的 API 变更
- **MINOR**: 向后兼容的新功能
- **PATCH**: 向后兼容的问题修复

预发布版本：
- `v1.0.0-alpha.1` - 内部测试
- `v1.0.0-beta.1` - 公开测试
- `v1.0.0-rc.1` - 发布候选
