# NAS-OS v2.617.0 开发任务

## 任务背景
基于竞品分析（飞牛/群晖/TrueNAS），开发差异化新功能，巩固竞争优势。

## 紧急任务（工部）

### 任务1：修复 Actions 失败
**问题**：`internal/lxccontainer/lxccontainer_test.go` 编译失败
- 缺少 `NewManager()` 函数（测试用，应为 `NewContainerManager()` 别名）
- 缺少 `validateResources()` 函数

**修复方案**：
1. 在 `container.go` 添加 `NewManager()` 别名函数
2. 在 `container.go` 添加 `validateResources()` 函数
3. 运行测试确认修复

```go
// NewManager 是 NewContainerManager 的别名，用于测试兼容.
func NewManager() *ContainerManager {
    return NewContainerManager()
}

// validateResources 验证资源配置合法性.
func validateResources(res ResourceLimit) error {
    if res.MemoryMB == 0 {
        return fmt.Errorf("内存不能为0")
    }
    if res.CPUCores < 0 {
        return fmt.Errorf("CPU核心数不能为负")
    }
    if res.CPUPercent < 0 || res.CPUPercent > 100 {
        return fmt.Errorf("CPU百分比必须在0-100之间")
    }
    if res.ProcessMax < 0 {
        return fmt.Errorf("最大进程数不能为负")
    }
    return nil
}
```

---

## 新功能开发（兵部）

### 任务2：勒索软件蜜罐检测（对标 TrueNAS Ransomware Defense）
**目录**：`internal/ransomware_honeypot/`

**功能要求**：
1. 创建诱饵文件（Office/PDF/图片格式）
2. 监控文件熵值变化（加密特征）
3. 检测异常批量重命名
4. 自动告警并隔离受感染共享
5. 恢复点管理

**文件结构**：
- `types.go` - 类型定义
- `honeypot.go` - 蜜罐管理器
- `detector.go` - 检测引擎
- `handlers.go` - API 处理器
- `honeypot_test.go` - 测试

**API 路由**：
```
POST   /api/v1/ransomware/honeypot/create    # 创建蜜罐
GET    /api/v1/ransomware/honeypot/list       # 列出蜜罐
POST   /api/v1/ransomware/scan                # 手动扫描
GET    /api/v1/ransomware/alerts               # 告警列表
POST   /api/v1/ransomware/alerts/:id/respond   # 响应告警
```

---

### 任务3：FIDO2/WebAuthn 硬件密钥认证（对标群晖 FIDO2）
**目录**：`internal/fido2/`

**功能要求**：
1. WebAuthn 注册流程
2. 硬件密钥验证（YubiKey/TouchID/Windows Hello）
3. 多密钥管理
4. 会话断言验证
5. 备份密钥支持

**文件结构**：
- `types.go` - 类型定义
- `authenticator.go` - 认证器
- `credential.go` - 凭据管理
- `handlers.go` - API 处理器
- `fido2_test.go` - 测试

**API 路由**：
```
POST   /api/v1/auth/fido2/register/begin     # 开始注册
POST   /api/v1/auth/fido2/register/finish    # 完成注册
POST   /api/v1/auth/fido2/login/begin        # 开始登录
POST   /api/v1/auth/fido2/login/finish       # 完成登录
GET    /api/v1/auth/fido2/credentials        # 凭据列表
DELETE /api/v1/auth/fido2/credentials/:id    # 删除凭据
```

---

### 任务4：存储效率仪表板（对标群晖 Storage Efficiency）
**目录**：`internal/storage_efficiency/`

**功能要求**：
1. 压缩率统计（文件级/块级）
2. 去重效果分析
3. 空间节省计算
4. 优化建议引擎
5. 趋势图表数据

**文件结构**：
- `types.go` - 类型定义
- `analyzer.go` - 分析引擎
- `optimizer.go` - 优化建议
- `handlers.go` - API 处理器
- `efficiency_test.go` - 测试

**API 路由**：
```
GET    /api/v1/storage/efficiency/summary     # 效率概览
GET    /api/v1/storage/efficiency/compression  # 压缩统计
GET    /api/v1/storage/efficiency/dedup        # 去重统计
GET    /api/v1/storage/efficiency/suggestions  # 优化建议
POST   /api/v1/storage/efficiency/analyze      # 触发分析
```

---

## 代码规范
1. 每个模块必须有完整测试
2. 使用中文注释
3. 遵循项目现有风格
4. Handler 注册到 main router

## 版本
当前：v2.616.0
目标：v2.617.0
