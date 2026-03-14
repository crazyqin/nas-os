# NAS-OS 发布流程

## 版本命名规范

采用语义化版本 (Semantic Versioning): `MAJOR.MINOR.PATCH`

- **MAJOR**: 重大架构变更或不兼容的 API 修改
- **MINOR**: 新功能添加，向后兼容
- **PATCH**: Bug 修复，向后兼容

## 发布流程

### 1. 开发阶段

1. 从 `master` 创建功能分支: `feature/xxx`
2. 开发完成后创建 PR
3. 代码审查通过后合并到 `master`

### 2. 版本准备

```bash
# 更新版本号
vim internal/version/version.go

# 更新 CHANGELOG
vim CHANGELOG.md

# 提交版本更新
git add -A
git commit -m "chore: bump version to vX.Y.Z"
git push
```

### 3. 创建 Release

```bash
# 创建标签
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z

# 或使用 GitHub CLI
gh release create vX.Y.Z --title "vX.Y.Z" --notes-file RELEASE_NOTES.md
```

### 4. CI/CD 自动化

GitHub Actions 自动执行:
- 代码检查 (golangci-lint)
- 单元测试
- 安全扫描 (Trivy)
- Docker 镜像构建
- GitHub Release 发布

### 5. 发布后验证

1. 检查 GitHub Actions 状态
2. 验证 Docker 镜像: `docker pull ghcr.io/crazyqin/nas-os:vX.Y.Z`
3. 测试基本功能

## 版本发布检查清单

- [ ] 版本号已更新
- [ ] CHANGELOG 已更新
- [ ] 所有测试通过
- [ ] 文档已更新
- [ ] Release Notes 已准备
- [ ] GitHub Actions 正常

## 六部协作

发布流程涉及六部协作:

| 部门 | 职责 |
|------|------|
| 司礼监 | 协调发布流程，收集各部门成果 |
| 兵部 | 代码开发，Bug 修复 |
| 工部 | CI/CD 配置，Docker 构建 |
| 礼部 | 文档更新，Release Notes |
| 刑部 | 安全检查，合规审核 |
| 户部 | 版本规划，资源报告 |
| 吏部 | 版本管理，里程碑更新 |

---

*本文档由吏部维护*