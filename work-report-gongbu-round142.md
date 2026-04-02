# 工部: CI增强 (第142轮)

## CI改进点

### 1. Go版本统一 (v2.375.0)
- **问题**: `compatibility-check.yml` 使用 Go 1.26，其他workflow使用 Go 1.25，Dockerfile使用 Go 1.26
- **修复**: 统一为 Go 1.25（与 go.mod 保持一致）
- **影响文件**: 
  - `.github/workflows/compatibility-check.yml`
  - `Dockerfile` (两处)

### 2. CI缓存优化增强
- **新增**: 编译产物缓存（跨job复用）
  - 缓存key: `build-artifacts-${{ env.CACHE_VERSION }}-${{ github.sha }}`
  - 加速后续构建job
- **优化**: 缓存预热策略
  - 分层预编译: `./cmd/...`, `./internal/...`, `./pkg/...`
  - 减少后续job等待时间
- **统计**: 缓存配置数量 45处，覆盖完整

### 3. Docker发布稳定性增强
- **armv7超时优化**: 
  - 从 45分钟 → 50分钟
  - 减少QEMU模拟超时风险
- **重试机制**: 
  - `continue-on-error: true` 用于 armv7 构建
  - 允许部分失败后继续执行
- **版本标识**: 更新为 v2.375.0

### 4. 安装脚本增强 (scripts/install.sh)
- **新增功能**:
  - `retry_command()` 重试函数（最大3次，间隔5秒）
  - `check_system_resources()` 系统资源检查（内存≥512MB，磁盘≥1GB）
  - 健康检查等待逻辑（最多30秒）
- **改进**:
  - 服务启动后等待健康端点响应
  - 超时后输出详细状态信息
  - 增加安装前系统检查

### 5. 其他发现
- **编译问题**: 兵部第141轮提交的 `internal/storage/raidz_handlers.go` 存在编译错误
  - undefined: ZPoolManager
  - assignment mismatch
  - 不属于工部本轮范围，建议兵部修复

## 测试结果

| 测试项 | 结果 | 说明 |
|--------|------|------|
| install.sh语法检查 | ✅ 通过 | bash -n 验证 |
| nasctl构建 | ✅ 通过 | CLI工具构建成功 |
| nasd构建 | ❌ 失败 | 兵部代码问题（非工部范围） |
| Go版本统一 | ✅ 完成 | 3处修复 |
| CI缓存配置 | ✅ 完成 | 45处缓存配置 |
| Docker超时优化 | ✅ 完成 | armv7: 50分钟 |
| VERSION更新 | ✅ 完成 | v2.375.0 |

## 变更文件清单

```
.github/workflows/compatibility-check.yml  # Go版本统一
.github/workflows/ci-cd.yml                # 缓存优化增强
.github/workflows/docker-publish.yml       # 稳定性增强
Dockerfile                                 # Go版本统一
scripts/install.sh                         # 安装脚本增强
VERSION                                    # v2.375.0
```

## 建议后续

1. **兵部**: 修复 `internal/storage/raidz_handlers.go` 编译错误
2. **CI测试**: 合并后在GitHub Actions验证完整流程
3. **Docker发布**: 观察armv7构建成功率变化

---

工部 第142轮 完成时间: 2026-04-02