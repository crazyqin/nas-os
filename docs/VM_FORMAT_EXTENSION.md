# VM 多格式导入导出扩展设计

**版本**: v1.0
**日期**: 2026-04-24
**对标**: TrueNAS 25.10 VM多格式支持
**负责**: 工部

---

## 1. TrueNAS 25.10 VM增强

TrueNAS 25.10 引入了完整的VM磁盘格式支持：
- QCOW2 (QEMU Copy On Write)
- QED (QEMU Enhanced Disk)
- RAW (原始格式)
- VDI (VirtualBox Disk Image)
- VHDX (Hyper-V Virtual Hard Disk)
- VMDK (VMware Virtual Disk)

---

## 2. nas-os VM现状

### 2.1 当前支持
- RAW格式磁盘
- ISO镜像挂载
- 基础VM管理

### 2.2 对标差距

| 格式 | TrueNAS 25.10 | nas-os | 需要 |
|------|---------------|--------|------|
| RAW | ✅ | ✅ | - |
| QCOW2 | ✅ | ❌ | P0 |
| VMDK | ✅ | ❌ | P0 |
| VHDX | ✅ | ❌ | P1 |
| VDI | ✅ | ❌ | P1 |
| QED | ✅ | ❌ | P2 |

---

## 3. 多格式支持设计

### 3.1 架构

```
┌─────────────────────────────────────────────────────────┐
│                   VM格式转换架构                         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────┐                                       │
│  │ Web界面/API │                                       │
│  └─────────────┘                                       │
│         │                                              │
│         ▼                                              │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Format Manager                      │   │
│  ├─────────────────────────────────────────────────┤   │
│  │ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐│   │
│  │ │QCOW2    │ │VMDK     │ │VHDX     │ │VDI      ││   │
│  │ │Handler  │ │Handler  │ │Handler  │ │Handler  ││   │
│  │ └─────────┘ └─────────┘ └─────────┘ └─────────┘│   │
│  │ ┌─────────┐ ┌─────────┐                        │   │
│  │ │RAW      │ │QED      │                        │   │
│  │ │Handler  │ │Handler  │                        │   │
│  │ └─────────┘ └─────────┘                        │   │
│  └─────────────────────────────────────────────────┘   │
│         │                                              │
│         ▼                                              │
│  ┌─────────────┐                                       │
│  │ qemu-img   │                                       │
│  │ 工具调用   │                                       │
│  └─────────────┘                                       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 3.2 模块划分

```
internal/vm/format/
├── manager.go       # 格式管理器
├── handler.go       # 格式处理器接口
├── qcow2.go         # QCOW2处理
├── vmdk.go          # VMDK处理
├── vhdx.go          # VHDX处理
├── vdi.go           # VDI处理
├── raw.go           # RAW处理（已有）
├── converter.go     # 格式转换
└── inspector.go     # 格式信息解析
```

---

## 4. 功能规格

### 4.1 导入功能

| 功能 | 说明 | 优先级 |
|------|------|--------|
| QCOW2导入 | 支持QEMU镜像导入 | P0 |
| VMDK导入 | 支持VMware镜像导入 | P0 |
| VHDX导入 | 支持Hyper-V镜像导入 | P1 |
| VDI导入 | 支持VirtualBox镜像导入 | P1 |
| 自动转换 | 导入时自动转RAW | P0 |

### 4.2 导出功能

| 功能 | 说明 | 优先级 |
|------|------|--------|
| QCOW2导出 | 导出为QEMU格式 | P0 |
| VMDK导出 | 导出为VMware格式 | P0 |
| VHDX导出 | 导出为Hyper-V格式 | P1 |
| VDI导出 | 导出为VirtualBox格式 | P1 |
| RAW导出 | 导出原始格式 | ✅已有 |

### 4.3 格式信息

| 功能 | 说明 | 优先级 |
|------|------|--------|
| 格式识别 | 自动识别磁盘格式 | P0 |
| 信息解析 | 解析格式元数据 | P0 |
| 快照信息 | QCOW2快照列表 | P1 |

---

## 5. API设计

### 5.1 导入导出API

| API | 说明 | Method |
|-----|------|--------|
| /api/v1/vm/disks/import | 导入磁盘 | POST |
| /api/v1/vm/disks/:id/export | 导出磁盘 | POST |
| /api/v1/vm/disks/:id/convert | 格式转换 | POST |
| /api/v1/vm/disks/:id/info | 磁盘信息 | GET |
| /api/v1/vm/formats | 支持格式列表 | GET |

### 5.2 导入请求示例

```json
{
  "source": "/path/to/disk.qcow2",
  "format": "qcow2",
  "targetFormat": "raw",
  "vmId": "vm-001",
  "options": {
    "sparse": true,
    "overwrite": false
  }
}
```

---

## 6. 实现路径

### 6.1 Phase 1: QCOW2/VMDK支持（v2.470.0）
- qemu-img工具集成
- QCOW2/VMDK导入导出
- 格式识别

### 6.2 Phase 2: VHDX/VDI支持（v2.480.0）
- VHDX处理实现
- VDI处理实现
- 格式转换优化

### 6.3 Phase 3: 完善功能（v2.490.0）
- 快照信息解析
- 批量转换
- Web界面完善

---

## 7. 技术依赖

### 7.1 qemu-img工具
- 系统需安装qemu-utils
- 命令调用封装

### 7.2 Go实现
- 格式头解析
- 元数据提取
- 进度追踪

---

## 8. 测试计划

### 8.1 功能测试
- 各格式导入
- 各格式导出
- 格式转换
- 格式识别

### 8.2 兼容性测试
- QEMU创建的QCOW2
- VMware创建的VMDK
- Hyper-V创建的VHDX
- VirtualBox创建的VDI

### 8.3 性能测试
- 大磁盘转换时间
- 资源占用测试

---

## 9. 文档规划

- `docs/vm-format-guide.md`: VM格式指南
- `docs/vm-import-export.md`: 导入导出教程
- `docs/vm-migration.md`: 跨平台迁移指南