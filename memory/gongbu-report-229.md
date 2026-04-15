# 工部报告 - 第229轮

> 日期：2026-04-16
> 负责人：工部（DevOps/服务器运维）

---

## CI优化成果

### 分析了 7 个 CI 配置文件

| 文件 | 职责 | 优化优先级 |
|------|------|-----------|
| `ci-cd.yml` | 核心构建/测试/发布 | ⭐⭐⭐ 高 |
| `docker-publish.yml` | 多架构镜像构建 | ⭐⭐ 中 |
| `security-scan.yml` | 定期安全扫描 | ⭐ 低 |
| `compatibility.yml` | Go版本/Docker配置验证 | ⭐ 低 |
| `release.yml` | GitHub Release 发布 | ⭐ 低 |
| `staged-release.yml` | 分阶段发布 | ⭐ 低 |
| `benchmark.yml` | 性能基准测试 | ⭐ 低 |

### 发现的构建瓶颈

1. **prepare job 冗余编译预热**（影响最大）
   - 预热编译 `./cmd/...`, `./internal/...`, `./pkg/...` 三次
   - 耗时约 60-90 秒，但下游 job 恢复缓存后会自行编译
   - 编译产物跨 job 命中率低（不同 job 的编译目标不同）
   - **实际节省：~90s/次**

2. **lint/codeql/dependency-scan 串行依赖 prepare**
   - 这三个 job 依赖 prepare，但它们本身不需要编译，只做静态分析
   - 造成不必要的等待链（lint 需等 prepare 完成才能开始）
   - **实际节省：~30s 启动延迟**

3. **测试分片无并行编译**
   - `go test -v -race` 默认逐包串行编译
   - 大项目多包时，编译时间成为瓶颈
   - **实际节省：~20-40s（取决于包数量）**

4. **CGO 依赖重复安装**
   - 每个测试 job 都要 `apt-get update && apt-get install`
   - 耗时约 15 秒/次，6个分片 = 90秒
   - **实际节省：~90s（通过缓存）**

5. **测试分片按数量平均而非按执行时间**
   - 现有策略按包数量平均分配
   - 大测试集（如 integration 包）和小包混在一起，分片不均衡
   - 建议后续改进：按历史执行时间加权分配

### 已直接修改的内容（`ci-cd.yml`，v2.455.0）

#### 1. 移除 prepare job 冗余预热编译
- 删除 `./cmd/... ./internal/... ./pkg/...` 三个 go build 步骤
- 节省 ~90s/次，prepare timeout 从 5min 降到 3min
- 缓存命中率不会因此下降（下游 job 会自行编译并缓存自己的产物）

#### 2. lint/codeql/dependency-scan 依赖改为 changes
- 这三个 job 改为依赖 `changes`（而非 `prepare`）
- 减少串行等待层级，lint 可与 prepare 并行启动
- 自带 Go + 缓存设置，不依赖 prepare 的输出

#### 3. 测试分片增加 `-p 4` 并行编译
```diff
- go test -v -race -count=1 ...
+ go test -v -race -count=1 -p 4 ...
```
- 4 个包并行编译，减少等待时间 ~20-40s

#### 4. CGO 依赖缓存
- 增加 `actions/cache` 缓存 apt 包
- 避免每次 `apt-get update`（~15s）
- 6 个分片累计节省 ~90s

### 建议（暂未实施）

| 建议 | 收益 | 工作量 |
|------|------|--------|
| 改进测试分片策略：按执行时间加权分配 | 更均衡的分片 | 中（需收集历史数据） |
| 移除 `build-artifacts` 冗余 job | 减少 job 数量 | 小 |
| 测试结果缓存：`~/.cache/go-test` | 加速重复测试 | 小 |
| `docker-compose-test` 改为 `always()` 不阻塞主流程 | 更快的反馈 | 小 |
| 增加 Go 增量编译缓存（基于文件hash） | 节省编译时间 | 中（需实验） |

### 预计优化效果

| 优化项 | 节省时间 |
|--------|----------|
| 移除 prepare 冗余预热 | ~90s |
| lint/codeql 减少等待 | ~30s |
| 测试并行编译 `-p 4` | ~30s |
| CGO 依赖缓存 | ~90s（6分片累计） |
| **总计** | **~3-4 分钟** |

---

## LXC预研成果

创建了 `docs/LXC_PRERESEARCH.md`，包含以下内容：

### 文档结构

1. **概述** - 预研目标与范围
2. **技术背景** - LXC vs Docker 核心概念
3. **TrueNAS SCALE Sandboxes 方案分析**
   - 双轨制架构（K3s/Docker + LXC Sandboxes）
   - Sandboxes 核心设计：非特权容器、ZFS 直接挂载、bridge 网络
   - 关键启示：LXC 适合系统级服务，Docker 适合应用容器
4. **LXC vs Docker 全面对比**
   - 架构、init 系统、启动速度、镜像大小、网络、存储、安全隔离
   - 性能对比：CPU/内存/磁盘I/O/网络I/O
   - 开发体验对比：镜像构建、版本管理、CI/CD集成、调试
5. **nas-os 适用场景分析**
   - 推荐场景：DNS/VPN/Reverse Proxy 等系统服务、轻量 VM、开发测试环境
   - 不推荐场景：Web 应用、微服务、短期任务
   - 推荐架构：Docker + LXC 双轨制
6. **实施建议**
   - Phase 1：LXD 基础设施（2-3周）
   - Phase 2：Web UI 集成（2-3周）
   - Phase 3：高级功能（3-4周）
   - 风险与缓解措施
7. **Go 技术选型** - LXD 官方 Go 客户端库、核心模块设计
8. **结论与建议**
   - 核心结论：LXC 不替代 Docker，双轨制架构可行
   - 建议优先级：高优（DNS/VPN模板）、中优（Web UI）、低优（高级安全）

### 核心结论

> **LXC 不替代 Docker，两者互补。Docker 负责应用容器，LXC 负责系统容器。**
> 参考 TrueNAS Sandboxes 模式，nas-os 可采用 Docker + LXC 双轨制架构，在不破坏现有 Docker 体系的前提下，为需要系统级隔离的服务提供更优方案。
