# NAS-OS 插件系统

## 概述

NAS-OS 提供了完整的插件系统支持，允许开发者创建和分发扩展功能。插件系统支持：

- **动态加载**：运行时加载 Go 插件（.so 文件）
- **生命周期管理**：安装/启用/禁用/卸载
- **插件市场**：浏览、搜索、评分插件
- **扩展点机制**：插件可以注册扩展点实现

## 架构

```
internal/plugin/
├── plugin.go      # 插件接口定义
├── loader.go      # 插件加载器（动态加载 .so）
├── manager.go     # 插件管理器（生命周期管理）
├── handlers.go    # HTTP API 处理器
└── market.go      # 插件市场客户端
```

## API 接口

### 已安装插件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/plugins | 列出已安装插件 |
| GET | /api/v1/plugins/:id | 获取插件详情 |
| POST | /api/v1/plugins | 安装插件 |
| DELETE | /api/v1/plugins/:id | 卸载插件 |
| POST | /api/v1/plugins/:id/enable | 启用插件 |
| POST | /api/v1/plugins/:id/disable | 禁用插件 |
| POST | /api/v1/plugins/:id/update | 更新插件 |
| PUT | /api/v1/plugins/:id/config | 配置插件 |

### 插件市场

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/plugins/market | 市场插件列表 |
| GET | /api/v1/plugins/market/search | 搜索插件 |
| GET | /api/v1/plugins/market/categories | 获取分类 |
| GET | /api/v1/plugins/market/:id | 插件详情 |
| POST | /api/v1/plugins/market/:id/rate | 提交评分 |
| GET | /api/v1/plugins/market/:id/reviews | 获取评论 |

## 开发插件

### 1. 创建插件项目

```bash
mkdir my-plugin
cd my-plugin
go mod init my-plugin
```

### 2. 实现插件接口

```go
package main

import "nas-os/internal/plugin"

// 插件信息
var PluginInfo = plugin.PluginInfo{
    ID:          "com.example.my-plugin",
    Name:        "我的插件",
    Version:     "1.0.0",
    Author:      "Your Name",
    Description: "插件描述",
    Category:    plugin.CategoryOther,
}

// MyPlugin 插件实现
type MyPlugin struct {
    config map[string]interface{}
}

// New 入口函数
func New() plugin.Plugin {
    return &MyPlugin{}
}

// Info 返回插件信息
func (p *MyPlugin) Info() plugin.PluginInfo {
    return PluginInfo
}

// Init 初始化插件
func (p *MyPlugin) Init(config map[string]interface{}) error {
    p.config = config
    return nil
}

// Start 启动插件
func (p *MyPlugin) Start() error {
    return nil
}

// Stop 停止插件
func (p *MyPlugin) Stop() error {
    return nil
}

// Destroy 销毁插件
func (p *MyPlugin) Destroy() error {
    return nil
}
```

### 3. 创建 manifest.json

```json
{
    "id": "com.example.my-plugin",
    "name": "我的插件",
    "version": "1.0.0",
    "author": "Your Name",
    "description": "插件描述",
    "category": "other",
    "entrypoint": "New",
    "mainFile": "my-plugin.so",
    "license": "MIT",
    "price": "free"
}
```

### 4. 构建插件

```bash
go build -buildmode=plugin -o my-plugin.so
```

### 5. 安装插件

将 `my-plugin.so` 和 `manifest.json` 复制到 `/opt/nas/plugins/my-plugin/` 目录。

或通过 API 安装：

```bash
curl -X POST http://localhost:8080/api/v1/plugins \
  -H "Content-Type: application/json" \
  -d '{"source": "/path/to/my-plugin"}'
```

## 插件分类

| 分类 | ID | 说明 |
|------|-----|------|
| 存储管理 | storage | 磁盘、卷、快照管理 |
| 文件管理 | file-manager | 文件操作增强 |
| 网络工具 | network | 网络监控、配置 |
| 系统工具 | system | 系统管理、监控 |
| 安全工具 | security | 安全扫描、审计 |
| 多媒体 | media | 照片、视频、音乐 |
| 备份同步 | backup | 云同步、备份 |
| 主题外观 | theme | UI 主题、样式 |
| 第三方集成 | integration | 第三方服务集成 |
| 开发工具 | developer | 编程、调试工具 |
| 生产力 | productivity | 办公、效率工具 |
| 其他 | other | 其他类型 |

## 扩展点

插件可以注册扩展点来扩展系统功能：

- `filemanager.toolbar` - 文件管理器工具栏
- `filemanager.contextMenu` - 文件上下文菜单
- `filemanager.preview` - 文件预览
- `theme.style` - 主题样式
- `theme.colors` - 主题颜色

## 钩子系统

插件可以注册钩子来响应系统事件：

| 钩子 | 触发时机 |
|------|----------|
| beforeMount | 挂载卷之前 |
| afterMount | 挂载卷之后 |
| beforeUnmount | 卸载卷之前 |
| afterUnmount | 卸载卷之后 |
| beforeCreate | 创建资源之前 |
| afterCreate | 创建资源之后 |
| beforeDelete | 删除资源之前 |
| afterDelete | 删除资源之后 |

## 权限系统

插件需要声明所需的权限：

```go
Permissions: []plugin.Permission{
    {Name: "file.read", Description: "读取文件"},
    {Name: "file.write", Description: "写入文件"},
    {Name: "network.access", Description: "网络访问"},
},
```

## 示例插件

### 文件管理器增强

位置：`plugins/filemanager-enhance/`

功能：
- 批量复制/移动/删除/重命名
- 文件预览
- 高级搜索

### 暗黑主题

位置：`plugins/dark-theme/`

功能：
- 暗黑主题样式
- 自动切换
- 自定义强调色

## 最佳实践

1. **版本兼容**：声明兼容的 NAS-OS 版本
2. **依赖管理**：明确声明依赖的其他插件
3. **权限最小化**：只请求必要的权限
4. **错误处理**：优雅处理错误，不崩溃系统
5. **资源清理**：在 Destroy 中释放所有资源
6. **日志记录**：使用结构化日志

## 故障排除

### 插件加载失败

- 检查 Go 版本兼容性
- 确认 .so 文件架构（amd64/arm64）
- 查看插件日志

### 插件启动失败

- 检查依赖是否满足
- 验证配置是否正确
- 查看错误信息

### 权限问题

- 确认插件目录权限
- 检查配置目录权限
- 验证数据目录权限