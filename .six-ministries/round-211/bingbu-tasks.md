# 兵部任务 - 第211轮

## 任务清单

### P0: FRP WebUI前端界面开发
**目标**: 完成FRP内网穿透的WebUI前端界面

**已有后端API**:
- `internal/connect/frp/api_handlers.go` - HTTP处理
- `internal/connect/frp/manager.go` - 隧道管理
- `internal/connect/frp/client.go` - FRP客户端

**前端需求**:
1. 隧道列表展示（状态、类型、地址）
2. 创建/编辑/删除隧道
3. 连接状态实时显示
4. 配置参数表单（P2P/Relay/Auto模式）

**参考竞品**: 飞牛FN Connect体验

### P1: RAIDZ Expansion进度监控UI
**已有**: 3,543行核心API实现
**需求**: 前端进度条 + 暂停/恢复按钮

### P2: SMB Stateful Failover架构预研
**对标**: TrueNAS企业级特性
**交付**: 设计文档 outline

---

**交付物**: 前端组件代码或设计文档