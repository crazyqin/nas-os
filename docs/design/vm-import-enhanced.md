# VM多格式导入增强设计

> **兵部第162轮任务** - 对标TrueNAS 25.10 VM Import/Export
> **支持格式**: QCOW2/QED/RAW/VDI/VHDX/VMDK

---

## 1.概述

对标TrueNAS 25.10虚拟机多格式导入支持，扩展nas-os VM管理能力。

### 1.1支持格式

| 格式 | 说明 | 来源 |
|------|------|------|
| QCOW2 | QEMU Copy-On-Write | QEMU/KVM |
| QED | QEMU Enhanced Disk | QEMU (legacy) |
| RAW | Raw disk image | 通用 |
| VDI | VirtualBox Disk Image | VirtualBox |
| VHDX | Hyper-V Virtual Hard Disk | Microsoft |
| VMDK | VMware Virtual Disk | VMware |

---

## 2. API设计

### 2.1 REST API

```yaml
POST /api/v1/vm/import
  Body:
    - name: VM名称
    - format: qcow2/qed/raw/vdi/vhdx/vmdk
    - source: 文件路径或URL
    - secure_boot: 是否启用安全启动
    - uefi: 是否使用UEFI

GET /api/v1/vm/export/:name
  Parameters:
    - format: 导出格式
    - compress: 是否压缩
```

---

## 3. 实现要点

- 使用qemu-img进行格式转换
- 支持异步导入（大文件）
- 导入进度监控
- Secure Boot配置支持

---

**预计完成**: Phase 1 - M108