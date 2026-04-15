# 兵部任务 - 第229轮

## 工作目录
`/home/mrafter/nas-os`

## 任务1: Synology Drive文件同步客户端 Phase1

参考 Synology Drive 功能，设计并实现文件同步核心模块。

### 功能需求
- 文件双向同步（本地 ↔ 远程/云存储）
- 冲突检测与处理（文件同时修改策略：newer_wins / keep_both / ask）
- 增量同步（只传变更部分）
- 带宽控制（限速上传/下载）
- 同步状态跟踪（syncing / synced / error / conflict）
- 同步历史记录（保留30天）

### 实现位置
- `internal/drive/sync/sync.go` - 同步引擎核心
- `internal/drive/sync/conflict.go` - 冲突处理策略
- `internal/drive/sync/watcher.go` - 文件变更监控（inotify）
- `internal/drive/sync/api.go` - HTTP API handlers
- `internal/drive/sync/config.go` - 同步配置管理

### 参考代码
- `internal/backup/enhanced_backup.go` - 已有分片上传逻辑
- `internal/drive/` - 如果已有drive目录，先读一下

### API设计
```go
type SyncJob struct {
    ID          string    `json:"id"`
    SourcePath  string    `json:"source_path"`
    TargetPath  string    `json:"target_path"`
    Direction   string    `json:"direction"` // "bidirectional" / "upload" / "download"
    ConflictMode string   `json:"conflict_mode"` // "newer_wins" / "keep_both" / "ask"
    BandwidthLimit int64  `json:"bandwidth_limit"` // bytes/s, 0=无限制
    Status      string    `json:"status"`
    LastSync    time.Time `json:"last_sync"`
    CreatedAt   time.Time `json:"created_at"`
}

POST   /api/v1/drive/sync/jobs        - 创建同步任务
GET    /api/v1/drive/sync/jobs        - 列出同步任务
GET    /api/v1/drive/sync/jobs/:id   - 获取任务详情
POST   /api/v1/drive/sync/jobs/:id/start  - 启动同步
POST   /api/v1/drive/sync/jobs/:id/stop   - 停止同步
DELETE /api/v1/drive/sync/jobs/:id   - 删除同步任务
GET    /api/v1/drive/sync/jobs/:id/history - 同步历史
```

### 冲突处理策略
```
ConflictMode: newer_wins
  → 比较mtime，保留最新

ConflictMode: keep_both  
  → 源文件保留，目标文件重命名加时间戳

ConflictMode: ask
  → 记录冲突到 /var/lib/nas-os/drive/conflicts/
  → 等待用户通过API或UI解决
```

---

## 任务2: RAIDZ Expansion UI完成

API已实现在 `internal/storage/raidz.go`，需要完成 Web UI 引导扩容流程。

### 参考设计
参考 TrueNAS RAIDZ Expansion 交互：
1. 显示当前池状态和可用插槽
2. 选择扩容方式（添加单盘 / 添加镜像 / 添加RAIDZ）
3. 显示扩容预估时间和风险提示
4. 确认并执行扩容
5. 实时进度显示

### 实现位置
- `webui/src/pages/storage/RaidzExpansion.tsx` - 扩容向导页面
- `webui/src/pages/storage/RaidzExpansion.module.css` - 样式
- `webui/src/api/storage.ts` - API调用（如无则创建）

### 组件设计
```
RaidzExpansionWizard
├── StepPoolInfo         - 当前池信息（容量、使用率、盘位）
├── StepExpansionOptions - 扩容选项（新增磁盘选择）
├── StepConfirmation     - 确认页面（预估时间/风险）
└── StepProgress         - 扩容进度（实时）

API: GET /api/v1/storage/pools/:id/expansion-candidates
     POST /api/v1/storage/pools/:id/expand
     GET /api/v1/storage/pools/:id/expand/status
```

---

## 工作汇报格式

完成后，在 `/home/mrafter/nas-os/memory/bingbu-report-229.md` 写入：

```
# 兵部报告 - 第229轮

## 完成内容
- [x] 任务1: Drive Sync Phase1
  - 创建了 xxx.go
  - 实现了 xxx 功能
- [x] 任务2: RAIDZ Expansion UI
  - 创建了 RaidzExpansion.tsx
  - 实现了 xx 步骤

## 新增文件
- internal/drive/sync/sync.go
- internal/drive/sync/conflict.go
- ...

## 遇到的问题
- 问题1及解决方案

## 建议
- 后续优化建议
```

## 注意
- Go代码请遵循项目已有的代码风格（参考 internal/ 目录）
- WebUI请使用 React + TypeScript，参考 webui/src/ 目录现有组件
- 不要修改其他部门的代码
- 完成后写好报告，文件名为 `memory/bingbu-report-229.md`
