# NAS-OS 设计系统

> 版本：1.0 | 创建：2026-03-10

完整的设计系统规范，指导 UI 开发、品牌视觉和交互体验。

---

## 🎨 设计原则

### 1. 简洁 (Simple)
- 减少视觉噪音，突出核心功能
- 一页一任务，避免信息过载
- 清晰的层级和留白

### 2. 可靠 (Reliable)
- 一致的交互模式
- 明确的状态反馈
- 可预测的操作结果

### 3. 高效 (Efficient)
- 常用操作触手可及
- 批量操作支持
- 键盘快捷键

### 4. 亲和 (Friendly)
- 友好的文案语调
- 清晰的错误提示
- 渐进式引导

---

## 🖌️ 色彩系统

### 主色板

| 名称 | 色值 | RGB | 使用场景 |
|------|------|-----|----------|
| Primary | `#2563EB` | rgb(37, 99, 235) | 主按钮、链接、焦点 |
| Primary Hover | `#1D4ED8` | rgb(29, 78, 216) | 悬停状态 |
| Primary Light | `#DBEAFE` | rgb(219, 174, 254) | 背景、高亮 |

### 功能色

| 名称 | 色值 | 使用场景 |
|------|------|----------|
| Success | `#10B981` | 成功、正常、运行中 |
| Warning | `#F59E0B` | 警告、注意、待处理 |
| Error | `#EF4444` | 错误、危险、删除 |
| Info | `#3B82F6` | 信息、提示 |

### 中性色

| 名称 | 色值 | 使用场景 |
|------|------|----------|
| Gray 50 | `#F9FAFB` | 最浅背景 |
| Gray 100 | `#F3F4F6` | 卡片背景、分隔 |
| Gray 200 | `#E5E7EB` | 边框、分割线 |
| Gray 300 | `#D1D5DB` | 禁用边框 |
| Gray 400 | `#9CA3AF` | 占位符、次要图标 |
| Gray 500 | `#6B7280` | 次要文本 |
| Gray 600 | `#4B5563` | 常规文本 |
| Gray 700 | `#374151` | 主要文本 |
| Gray 800 | `#1F2937` | 标题 |
| Gray 900 | `#111827` | 强调文本 |

### 深色模式 (预留)

| 名称 | 色值 | 使用场景 |
|------|------|----------|
| Dark BG | `#111827` | 深色背景 |
| Dark Surface | `#1F2937` | 深色卡片 |
| Dark Border | `#374151` | 深色边框 |

---

## 📐 字体系统

### 字体栈

```css
/* 中文优先 */
font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", 
             "Microsoft YaHei", "SimHei", sans-serif;

/* 英文字体 */
font-family: Inter, -apple-system, BlinkMacSystemFont, 
             "Segoe UI", Roboto, sans-serif;

/* 代码字体 */
font-family: "JetBrains Mono", "Fira Code", monospace;
```

### 字号规范

| 层级 | CSS 变量 | 字号 | 字重 | 行高 | 场景 |
|------|----------|------|------|------|------|
| Display | `--text-display` | 40px | 700 | 1.1 | 大标题 |
| H1 | `--text-h1` | 32px | 700 | 1.2 | 页面标题 |
| H2 | `--text-h2` | 24px | 600 | 1.3 | 章节标题 |
| H3 | `--text-h3` | 20px | 600 | 1.4 | 小节标题 |
| H4 | `--text-h4` | 18px | 600 | 1.4 | 卡片标题 |
| Body | `--text-body` | 16px | 400 | 1.6 | 正文 |
| Small | `--text-small` | 14px | 400 | 1.5 | 辅助说明 |
| Caption | `--text-caption` | 12px | 400 | 1.4 | 注释、标签 |

### 字重

| 名称 | 值 | 使用场景 |
|------|-----|----------|
| Regular | 400 | 正文、说明 |
| Medium | 500 | 强调文本 |
| Semibold | 600 | 小标题、按钮 |
| Bold | 700 | 主标题、重要信息 |

---

## 📏 间距系统

### 基础单位：4px

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-5: 20px;
--space-6: 24px;
--space-8: 32px;
--space-10: 40px;
--space-12: 48px;
--space-16: 64px;
--space-20: 80px;
--space-24: 96px;
```

### 使用原则
- 组件内部：4-8px
- 组件之间：16-24px
- 区块之间：32-48px
- 页面边距：24-64px

---

## 🎭 组件规范

### 按钮 (Button)

```css
.btn {
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  transition: all 0.2s ease;
  border: none;
  cursor: pointer;
}

.btn-primary {
  background: #2563EB;
  color: white;
}
.btn-primary:hover {
  background: #1D4ED8;
  box-shadow: 0 2px 4px rgba(37, 99, 235, 0.3);
}

.btn-secondary {
  background: white;
  color: #374151;
  border: 1px solid #D1D5DB;
}
.btn-secondary:hover {
  background: #F9FAFB;
  border-color: #9CA3AF;
}

.btn-danger {
  background: #EF4444;
  color: white;
}
.btn-danger:hover {
  background: #DC2626;
}

.btn-sm { padding: 4px 12px; font-size: 13px; }
.btn-lg { padding: 12px 24px; font-size: 16px; }
```

### 输入框 (Input)

```css
.input {
  padding: 8px 12px;
  border: 1px solid #D1D5DB;
  border-radius: 6px;
  font-size: 14px;
  transition: all 0.2s;
}
.input:focus {
  outline: none;
  border-color: #2563EB;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}
.input:disabled {
  background: #F3F4F6;
  color: #9CA3AF;
  cursor: not-allowed;
}
```

### 卡片 (Card)

```css
.card {
  background: white;
  border-radius: 12px;
  padding: 24px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
  transition: box-shadow 0.2s;
}
.card:hover {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}
```

### 表格 (Table)

```css
.table {
  width: 100%;
  border-collapse: collapse;
}
.table th {
  background: #F9FAFB;
  padding: 12px 16px;
  text-align: left;
  font-weight: 600;
  color: #374151;
  border-bottom: 2px solid #E5E7EB;
}
.table td {
  padding: 12px 16px;
  border-bottom: 1px solid #E5E7EB;
  color: #4B5563;
}
.table tr:hover {
  background: #F9FAFB;
}
```

### 状态标签 (Badge)

```css
.badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}
.badge-success { background: #D1FAE5; color: #065F46; }
.badge-warning { background: #FEF3C7; color: #92400E; }
.badge-error { background: #FEE2E2; color: #991B1B; }
.badge-info { background: #DBEAFE; color: #1E40AF; }
```

### 进度条 (Progress)

```css
.progress {
  background: #E5E7EB;
  border-radius: 999px;
  height: 8px;
  overflow: hidden;
}
.progress-bar {
  background: #2563EB;
  height: 100%;
  transition: width 0.3s ease;
  border-radius: 999px;
}
```

---

## 📱 响应式断点

```css
/* 手机竖屏 */
@media (max-width: 639px) { /* xs */ }

/* 手机横屏 */
@media (min-width: 640px) { /* sm */ }

/* 平板 */
@media (min-width: 768px) { /* md */ }

/* 桌面 */
@media (min-width: 1024px) { /* lg */ }

/* 大桌面 */
@media (min-width: 1280px) { /* xl */ }
```

### 布局适配

| 断点 | 列数 | 边距 | 槽宽 |
|------|------|------|------|
| xs | 4 | 16px | 16px |
| sm | 6 | 24px | 24px |
| md | 8 | 24px | 24px |
| lg | 12 | 32px | 32px |
| xl | 12 | 40px | 32px |

---

## 🎬 动效规范

### 过渡时间

| 类型 | 时长 | 缓动 | 场景 |
|------|------|------|------|
| 快速 | 100ms | ease | 微交互、颜色变化 |
| 标准 | 200ms | ease | 按钮、卡片悬停 |
| 慢速 | 300ms | ease-out | 展开、滑动 |
| 入场 | 400ms | ease-out | 页面加载、模态框 |

### 缓动函数

```css
/* 标准缓动 */
transition-timing-function: ease;

/* 入场缓动 */
transition-timing-function: ease-out;

/* 出场缓动 */
transition-timing-function: ease-in;

/* 自定义贝塞尔 */
transition-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
```

---

## 🧩 图标系统

### 尺寸

| 尺寸 | 值 | 场景 |
|------|-----|------|
| xs | 12px | 紧凑列表、标签 |
| sm | 16px | 按钮内、表单 |
| md | 20px | 导航、卡片 |
| lg | 24px | 功能入口 |
| xl | 32px | 空状态、插图 |

### 风格
- 线性图标为主 (1.5px 描边)
- 圆角端点
- 简洁几何

---

## 📊 数据可视化

### 图表配色

```
顺序使用:
1. #2563EB (主蓝)
2. #10B981 (绿)
3. #F59E0B (黄)
4. #EF4444 (红)
5. #8B5CF6 (紫)
6. #EC4899 (粉)
```

### 图表规范
- 简洁清晰，避免 3D 效果
- 颜色有意义 (绿=好，红=坏)
- 添加数据标签
- 留白充足

---

## 🎯 交互模式

### 操作反馈

| 操作 | 反馈类型 | 延迟 |
|------|----------|------|
| 点击按钮 | 即时视觉反馈 | <50ms |
| 表单提交 | Loading 状态 | 即时 |
| 删除确认 | 模态框确认 | 即时 |
| 数据加载 | 骨架屏/Spinner | <200ms |

### 错误处理
1. 明确告知问题原因
2. 提供解决建议
3. 保留用户输入
4. 允许重试

---

## 📝 文案规范

### 语调原则
- 简洁直接
- 友好专业
- 积极正向

### 示例

| 场景 | ❌ 避免 | ✅ 推荐 |
|------|--------|--------|
| 成功 | "操作已成功完成" | "已完成" |
| 错误 | "您可能遇到了一个错误" | "操作失败，请重试" |
| 加载 | "数据正在加载中" | "加载中..." |
| 空状态 | "暂无数据" | "还没有内容，点击创建" |

---

## ♿ 无障碍规范

### 对比度要求
- 正常文本：4.5:1 最低
- 大文本 (18px+)：3:1 最低
- UI 组件：3:1 最低

### 键盘导航
- 所有交互元素可 Tab 到达
- 焦点状态清晰可见
- 支持 Enter/Space 激活

### 屏幕阅读器
- 语义化 HTML 标签
- 适当的 aria 属性
- 图标添加 aria-label

---

## 📁 文件结构

```
nas-os/
├── design-system.md      # 本文件
├── brand/
│   ├── logo/             # Logo 源文件
│   │   ├── logo.svg      # 矢量源文件
│   │   ├── logo-512.png  # 大尺寸
│   │   ├── logo-256.png  # 中尺寸
│   │   ├── logo-128.png  # 小尺寸
│   │   ├── logo-64.png   # 图标尺寸
│   │   └── logo-32.png   # favicon 尺寸
│   ├── icons/            # 图标集
│   └── templates/        # 模板文件
└── webui/
    ├── index.html        # 主界面
    ├── css/
    │   └── design-system.css  # 设计系统 CSS
    └── js/
        └── components.js      # 组件库
```

---

## 🔧 CSS 变量

```css
:root {
  /* 颜色 */
  --color-primary: #2563EB;
  --color-primary-hover: #1D4ED8;
  --color-primary-light: #DBEAFE;
  --color-success: #10B981;
  --color-warning: #F59E0B;
  --color-error: #EF4444;
  
  /* 字体 */
  --font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", sans-serif;
  --font-size-base: 16px;
  
  /* 间距 */
  --space-1: 4px;
  --space-2: 8px;
  --space-4: 16px;
  --space-6: 24px;
  --space-8: 32px;
  
  /* 圆角 */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-xl: 16px;
  
  /* 阴影 */
  --shadow-sm: 0 1px 2px rgba(0,0,0,0.05);
  --shadow-md: 0 4px 6px rgba(0,0,0,0.07);
  --shadow-lg: 0 10px 15px rgba(0,0,0,0.1);
}
```

---

*设计系统是活的文档，随产品迭代持续更新。*
