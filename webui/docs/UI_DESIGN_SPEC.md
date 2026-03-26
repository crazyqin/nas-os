# NAS-OS 现代化 UI 组件库设计规范

**版本**: v3.0 | **日期**: 2026-03-26 | **部门**: 礼部

---

## 一、设计理念

### 1.1 核心原则

| 原则 | 说明 |
|-----|------|
| **简洁优先** | 参考飞牛 fnOS，界面元素精简，核心功能突出 |
| **深色优先** | 默认深色主题，减少眼睛疲劳，适合 24/7 运维场景 |
| **响应式设计** | 移动端优先，适配 375px ~ 1920px+ |
| **一致性** | 统一的视觉语言、交互模式、间距系统 |

### 1.2 竞品参考

- **飞牛 fnOS**: 现代化界面、深色主题优化、简洁导航
- **群晖 DSM 7.3**: Dashboard 可定制 Widgets、企业级布局
- **TrueNAS Scale**: Dashboard 重构、更多可定制组件

---

## 二、设计令牌 (Design Tokens)

### 2.1 颜色系统

```css
/* ========================================
   Light Theme
   ======================================== */
:root {
  /* Primary - 蓝色系 */
  --color-primary-50: #EFF6FF;
  --color-primary-100: #DBEAFE;
  --color-primary-200: #BFDBFE;
  --color-primary-300: #93C5FD;
  --color-primary-400: #60A5FA;
  --color-primary-500: #3B82F6;  /* 主色 */
  --color-primary-600: #2563EB;
  --color-primary-700: #1D4ED8;
  --color-primary-800: #1E40AF;
  --color-primary-900: #1E3A8A;

  /* Neutral - 灰色系 */
  --color-neutral-0: #FFFFFF;
  --color-neutral-50: #F9FAFB;
  --color-neutral-100: #F3F4F6;
  --color-neutral-200: #E5E7EB;
  --color-neutral-300: #D1D5DB;
  --color-neutral-400: #9CA3AF;
  --color-neutral-500: #6B7280;
  --color-neutral-600: #4B5563;
  --color-neutral-700: #374151;
  --color-neutral-800: #1F2937;
  --color-neutral-900: #111827;
  --color-neutral-950: #030712;

  /* Semantic - 语义色 */
  --color-success: #10B981;
  --color-warning: #F59E0B;
  --color-error: #EF4444;
  --color-info: #3B82F6;
}

/* ========================================
   Dark Theme (Default)
   ======================================== */
[data-theme="dark"] {
  /* Surface - 表面色 */
  --surface-base: #0A0A0A;
  --surface-sunken: #111111;
  --surface-raised: #18181B;
  --surface-overlay: #1F1F23;
  --surface-border: #2A2A2E;
  --surface-border-subtle: #1F1F23;

  /* Text - 文字色 */
  --text-primary: #FAFAFA;
  --text-secondary: #A1A1AA;
  --text-muted: #71717A;
  --text-disabled: #52525B;
  --text-inverse: #0A0A0A;

  /* Accent - 强调色 */
  --accent-primary: #60A5FA;
  --accent-primary-hover: #93C5FD;
  --accent-primary-muted: rgba(96, 165, 250, 0.15);

  /* Semantic Backgrounds */
  --color-success-bg: rgba(16, 185, 129, 0.15);
  --color-warning-bg: rgba(245, 158, 11, 0.15);
  --color-error-bg: rgba(239, 68, 68, 0.15);
  --color-info-bg: rgba(59, 130, 246, 0.15);
}
```

### 2.2 间距系统

```css
:root {
  --space-0: 0;
  --space-1: 4px;    /* 紧凑元素 */
  --space-2: 8px;    /* 小间距 */
  --space-3: 12px;   /* 默认间距 */
  --space-4: 16px;   /* 组件内部 */
  --space-5: 20px;   /* 卡片间距 */
  --space-6: 24px;   /* 区块间距 */
  --space-8: 32px;   /* 大区块 */
  --space-10: 40px;  /* 页面边距 */
  --space-12: 48px;  /* 章节间距 */
  --space-16: 64px;  /* 特大间距 */
}
```

### 2.3 圆角系统

```css
:root {
  --radius-none: 0;
  --radius-sm: 4px;    /* 小元素：badge, tag */
  --radius-md: 8px;    /* 中等：button, input */
  --radius-lg: 12px;   /* 大元素：card, modal */
  --radius-xl: 16px;   /* 特大：dashboard widget */
  --radius-2xl: 24px;  /* 超大：feature card */
  --radius-full: 9999px; /* 圆形 */
}
```

### 2.4 阴影系统

```css
/* Light Theme */
:root {
  --shadow-xs: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.1);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.07);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.1);
  --shadow-xl: 0 20px 25px rgba(0, 0, 0, 0.15);
  --shadow-glow: 0 0 20px rgba(59, 130, 246, 0.3);
}

/* Dark Theme */
[data-theme="dark"] {
  --shadow-xs: 0 1px 2px rgba(0, 0, 0, 0.3);
  --shadow-sm: 0 1px 3px rgba(0, 0, 0, 0.4);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.5);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.6);
  --shadow-xl: 0 20px 25px rgba(0, 0, 0, 0.7);
  --shadow-glow: 0 0 20px rgba(96, 165, 250, 0.25);
}
```

### 2.5 排版系统

```css
:root {
  /* Font Family */
  --font-sans: -apple-system, BlinkMacSystemFont, "PingFang SC", 
               "Microsoft YaHei", "Noto Sans SC", sans-serif;
  --font-mono: "JetBrains Mono", "Fira Code", "SF Mono", monospace;

  /* Font Size */
  --text-xs: 12px;    /* 辅助文字 */
  --text-sm: 13px;    /* 次要文字 */
  --text-base: 14px;  /* 正文 */
  --text-md: 16px;    /* 大正文 */
  --text-lg: 18px;    /* 小标题 */
  --text-xl: 20px;    /* 标题 */
  --text-2xl: 24px;   /* 大标题 */
  --text-3xl: 30px;   /* 页面标题 */
  --text-4xl: 36px;   /* 展示标题 */

  /* Line Height */
  --leading-tight: 1.25;
  --leading-normal: 1.5;
  --leading-relaxed: 1.625;

  /* Font Weight */
  --font-normal: 400;
  --font-medium: 500;
  --font-semibold: 600;
  --font-bold: 700;
}
```

---

## 三、组件设计规范

### 3.1 卡片组件 (Card)

#### 3.1.1 基础卡片

```html
<div class="card">
  <div class="card-header">
    <h3 class="card-title">存储状态</h3>
    <span class="card-badge badge-success">正常</span>
  </div>
  <div class="card-body">
    <!-- 内容 -->
  </div>
  <div class="card-footer">
    <button class="btn btn-secondary">查看详情</button>
  </div>
</div>
```

```css
.card {
  background: var(--surface-raised);
  border: 1px solid var(--surface-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
  transition: all 0.2s ease;
}

.card:hover {
  border-color: var(--surface-border-hover, #3A3A3E);
  box-shadow: var(--shadow-md);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-4) var(--space-5);
  border-bottom: 1px solid var(--surface-border);
}

.card-title {
  font-size: var(--text-md);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.card-body {
  padding: var(--space-5);
}

.card-footer {
  padding: var(--space-4) var(--space-5);
  border-top: 1px solid var(--surface-border);
  background: var(--surface-sunken);
}
```

#### 3.1.2 统计卡片 (Stat Card)

```html
<div class="stat-card stat-card--primary">
  <div class="stat-card__icon">
    <svg><!-- CPU Icon --></svg>
  </div>
  <div class="stat-card__content">
    <span class="stat-card__label">CPU 使用率</span>
    <div class="stat-card__value">45%</div>
    <div class="stat-card__trend trend--up">
      <span>↑ 5%</span>
      <span>较上周</span>
    </div>
  </div>
  <div class="stat-card__chart">
    <!-- Mini Sparkline -->
  </div>
</div>
```

```css
.stat-card {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: var(--space-4);
  align-items: center;
  padding: var(--space-5);
  background: var(--surface-raised);
  border-radius: var(--radius-xl);
  border: 1px solid var(--surface-border);
}

.stat-card--primary {
  background: linear-gradient(135deg, 
    rgba(59, 130, 246, 0.1) 0%, 
    rgba(96, 165, 250, 0.05) 100%);
  border-color: rgba(59, 130, 246, 0.2);
}

.stat-card__icon {
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--accent-primary-muted);
  border-radius: var(--radius-lg);
  color: var(--accent-primary);
}

.stat-card__value {
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  color: var(--text-primary);
}

.stat-card__label {
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

.stat-card__chart {
  width: 80px;
  height: 40px;
}
```

### 3.2 按钮组件 (Button)

#### 3.2.1 按钮类型

```css
/* Base Button */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-4);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  line-height: 1.5;
  border: none;
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.btn:focus-visible {
  outline: 2px solid var(--accent-primary);
  outline-offset: 2px;
}

/* Primary */
.btn-primary {
  background: var(--accent-primary);
  color: var(--text-inverse);
}
.btn-primary:hover {
  background: var(--accent-primary-hover);
  box-shadow: var(--shadow-glow);
}

/* Secondary */
.btn-secondary {
  background: var(--surface-overlay);
  color: var(--text-primary);
  border: 1px solid var(--surface-border);
}
.btn-secondary:hover {
  background: var(--surface-raised);
  border-color: var(--text-muted);
}

/* Ghost */
.btn-ghost {
  background: transparent;
  color: var(--text-secondary);
}
.btn-ghost:hover {
  background: var(--surface-overlay);
  color: var(--text-primary);
}

/* Danger */
.btn-danger {
  background: var(--color-error);
  color: white;
}
.btn-danger:hover {
  background: #DC2626;
}

/* Sizes */
.btn-sm { padding: var(--space-1) var(--space-3); font-size: var(--text-xs); }
.btn-lg { padding: var(--space-3) var(--space-6); font-size: var(--text-md); }

/* Icon Only */
.btn-icon {
  padding: var(--space-2);
  width: 36px;
  height: 36px;
}
```

### 3.3 表单组件 (Form)

#### 3.3.1 输入框

```css
.form-input {
  width: 100%;
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-base);
  font-family: inherit;
  color: var(--text-primary);
  background: var(--surface-sunken);
  border: 1px solid var(--surface-border);
  border-radius: var(--radius-md);
  transition: all 0.15s ease;
}

.form-input::placeholder {
  color: var(--text-muted);
}

.form-input:hover {
  border-color: var(--text-muted);
}

.form-input:focus {
  outline: none;
  border-color: var(--accent-primary);
  box-shadow: 0 0 0 3px var(--accent-primary-muted);
}

.form-input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* With Icon */
.input-group {
  position: relative;
}

.input-group__icon {
  position: absolute;
  left: var(--space-3);
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-muted);
}

.input-group .form-input {
  padding-left: calc(var(--space-3) * 2 + 20px);
}
```

### 3.4 导航组件 (Navigation)

#### 3.4.1 侧边栏

```css
.sidebar {
  position: fixed;
  left: 0;
  top: 0;
  bottom: 0;
  width: 260px;
  background: var(--surface-base);
  border-right: 1px solid var(--surface-border);
  display: flex;
  flex-direction: column;
  z-index: 100;
}

.sidebar__header {
  padding: var(--space-5);
  border-bottom: 1px solid var(--surface-border);
}

.sidebar__brand {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.sidebar__logo {
  width: 36px;
  height: 36px;
}

.sidebar__title {
  font-size: var(--text-lg);
  font-weight: var(--font-semibold);
  color: var(--text-primary);
}

.sidebar__nav {
  flex: 1;
  padding: var(--space-4);
  overflow-y: auto;
}

.nav-group {
  margin-bottom: var(--space-6);
}

.nav-group__title {
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 0 var(--space-3);
  margin-bottom: var(--space-2);
}

.nav-item {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-3);
  color: var(--text-secondary);
  border-radius: var(--radius-md);
  transition: all 0.15s ease;
  cursor: pointer;
  margin-bottom: var(--space-1);
}

.nav-item:hover {
  background: var(--surface-overlay);
  color: var(--text-primary);
}

.nav-item.active {
  background: var(--accent-primary-muted);
  color: var(--accent-primary);
}

.nav-item__icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.nav-item__badge {
  margin-left: auto;
  padding: 2px 6px;
  font-size: var(--text-xs);
  background: var(--color-error);
  color: white;
  border-radius: var(--radius-full);
}
```

### 3.5 表格组件 (Table)

#### 3.5.1 现代表格

```css
.table-container {
  background: var(--surface-raised);
  border: 1px solid var(--surface-border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.table {
  width: 100%;
  border-collapse: collapse;
}

.table th {
  padding: var(--space-3) var(--space-4);
  text-align: left;
  font-size: var(--text-xs);
  font-weight: var(--font-semibold);
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  background: var(--surface-sunken);
  border-bottom: 1px solid var(--surface-border);
}

.table td {
  padding: var(--space-3) var(--space-4);
  font-size: var(--text-sm);
  color: var(--text-secondary);
  border-bottom: 1px solid var(--surface-border-subtle);
}

.table tr:last-child td {
  border-bottom: none;
}

.table tr:hover td {
  background: var(--surface-sunken);
}

/* 可选行 */
.table tr.selectable {
  cursor: pointer;
}

.table tr.selected td {
  background: var(--accent-primary-muted);
}
```

---

## 四、仪表板布局设计

### 4.1 整体布局

```
┌─────────────────────────────────────────────────────────────┐
│  Header (Logo + Search + Theme + Notifications + User)     │
├──────────┬──────────────────────────────────────────────────┤
│          │  Page Header (Title + Breadcrumb + Actions)     │
│  Sidebar ├──────────────────────────────────────────────────┤
│          │                                                  │
│  - Nav   │               Main Content                       │
│  - Items │                                                  │
│  - ...   │   ┌──────────────┬──────────────┬───────────┐   │
│          │   │ Stat Card    │ Stat Card    │ Stat Card │   │
│          │   └──────────────┴──────────────┴───────────┘   │
│          │                                                  │
│          │   ┌────────────────────┬────────────────────┐    │
│          │   │                    │                    │    │
│          │   │   Chart Widget     │   Quick Actions    │    │
│          │   │                    │                    │    │
│          │   └────────────────────┴────────────────────┘    │
│          │                                                  │
├──────────┴──────────────────────────────────────────────────┤
│  Footer (Version + Status + Copyright)                      │
└─────────────────────────────────────────────────────────────┘
```

### 4.2 响应式断点

```css
/* Mobile First */
:root {
  --sidebar-width: 260px;
}

/* Desktop: 1280px+ */
@media (min-width: 1280px) {
  .layout {
    display: grid;
    grid-template-columns: var(--sidebar-width) 1fr;
  }
}

/* Tablet: 768px - 1279px */
@media (max-width: 1279px) {
  .sidebar {
    transform: translateX(-100%);
    transition: transform 0.3s ease;
  }
  
  .sidebar.open {
    transform: translateX(0);
  }
  
  .main-content {
    margin-left: 0;
  }
}

/* Mobile: < 768px */
@media (max-width: 767px) {
  .stats-grid {
    grid-template-columns: 1fr 1fr;
  }
  
  .main-content {
    padding: var(--space-3);
    padding-bottom: calc(var(--space-16) + env(safe-area-inset-bottom));
  }
}

/* Small Mobile: < 480px */
@media (max-width: 479px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
  
  .stat-card {
    padding: var(--space-4);
  }
}
```

### 4.3 Dashboard Widgets

#### 4.3.1 Widget 配置

```javascript
const widgetConfig = {
  // CPU Monitor Widget
  cpu: {
    id: 'cpu',
    title: 'CPU 监控',
    type: 'chart',
    size: 'medium', // small, medium, large
    position: { x: 0, y: 0, w: 6, h: 4 },
    config: {
      chartType: 'line',
      refreshInterval: 1000,
      maxDataPoints: 60
    }
  },
  
  // Storage Overview Widget
  storage: {
    id: 'storage',
    title: '存储概览',
    type: 'stat-grid',
    size: 'large',
    position: { x: 6, y: 0, w: 6, h: 4 },
    config: {
      pools: ['pool1', 'pool2'],
      showHealth: true
    }
  },
  
  // Quick Actions Widget
  quickActions: {
    id: 'quickActions',
    title: '快速操作',
    type: 'action-grid',
    size: 'small',
    position: { x: 0, y: 4, w: 4, h: 3 },
    config: {
      actions: [
        { icon: 'folder', label: '文件管理', href: '/files' },
        { icon: 'docker', label: '容器', href: '/containers' },
        { icon: 'user', label: '用户', href: '/users' },
        { icon: 'settings', label: '设置', href: '/settings' }
      ]
    }
  }
};
```

---

## 五、深色模式优化

### 5.1 颜色对比度

确保所有文本满足 WCAG AA 标准：

| 元素 | 前景色 | 背景色 | 对比度 |
|-----|-------|-------|-------|
| 正文 | #FAFAFA | #0A0A0A | 19.5:1 |
| 次要文本 | #A1A1AA | #0A0A0A | 8.2:1 |
| 静默文本 | #71717A | #0A0A0A | 4.9:1 |
| 主色强调 | #60A5FA | #0A0A0A | 8.6:1 |

### 5.2 图表深色主题

```javascript
const darkChartTheme = {
  background: 'transparent',
  text: '#A1A1AA',
  gridLine: '#2A2A2E',
  colors: {
    primary: '#60A5FA',
    success: '#34D399',
    warning: '#FBBF24',
    danger: '#F87171',
    purple: '#A78BFA'
  }
};
```

---

## 六、移动端优化

### 6.1 触摸优化

```css
/* 最小触摸区域 44x44px */
.btn, .nav-item, .table tr {
  min-height: 44px;
}

/* 移除 hover 延迟 */
@media (hover: none) {
  .btn:active {
    transform: scale(0.98);
  }
}

/* 滑动手势支持 */
.swipeable {
  touch-action: pan-y;
}
```

### 6.2 底部导航

```html
<nav class="bottom-nav">
  <a href="/dashboard" class="bottom-nav__item active">
    <svg class="bottom-nav__icon"><!-- Home --></svg>
    <span class="bottom-nav__label">首页</span>
  </a>
  <a href="/files" class="bottom-nav__item">
    <svg class="bottom-nav__icon"><!-- Files --></svg>
    <span class="bottom-nav__label">文件</span>
  </a>
  <a href="/apps" class="bottom-nav__item">
    <svg class="bottom-nav__icon"><!-- Apps --></svg>
    <span class="bottom-nav__label">应用</span>
  </a>
  <a href="/settings" class="bottom-nav__item">
    <svg class="bottom-nav__icon"><!-- Settings --></svg>
    <span class="bottom-nav__label">设置</span>
  </a>
</nav>
```

```css
.bottom-nav {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  display: none;
  background: var(--surface-base);
  border-top: 1px solid var(--surface-border);
  padding-bottom: env(safe-area-inset-bottom);
  z-index: 100;
}

@media (max-width: 767px) {
  .bottom-nav {
    display: flex;
    justify-content: space-around;
  }
}

.bottom-nav__item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-2);
  color: var(--text-muted);
  text-decoration: none;
  min-width: 64px;
}

.bottom-nav__item.active {
  color: var(--accent-primary);
}

.bottom-nav__icon {
  width: 24px;
  height: 24px;
}

.bottom-nav__label {
  font-size: var(--text-xs);
}
```

---

## 七、动画与过渡

### 7.1 动画规范

```css
:root {
  /* 缓动函数 */
  --ease-in-out: cubic-bezier(0.4, 0, 0.2, 1);
  --ease-out: cubic-bezier(0, 0, 0.2, 1);
  --ease-in: cubic-bezier(0.4, 0, 1, 1);
  --ease-bounce: cubic-bezier(0.68, -0.55, 0.265, 1.55);

  /* 时长 */
  --duration-fast: 100ms;
  --duration-normal: 200ms;
  --duration-slow: 300ms;
}

/* 常用过渡 */
.fade-enter {
  opacity: 0;
}
.fade-enter-active {
  opacity: 1;
  transition: opacity var(--duration-normal) var(--ease-out);
}

.slide-up-enter {
  opacity: 0;
  transform: translateY(10px);
}
.slide-up-enter-active {
  opacity: 1;
  transform: translateY(0);
  transition: all var(--duration-normal) var(--ease-out);
}

/* 减少动画 */
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    animation-duration: 0.01ms !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## 八、组件清单

### 8.1 基础组件

| 组件 | 文件 | 状态 |
|-----|------|------|
| Button | `components/button.css` | ✅ 已有 |
| Card | `components/card.css` | ✅ 已有 |
| Badge | `components/badge.css` | ✅ 已有 |
| Input | `components/input.css` | ✅ 已有 |
| Select | `components/select.css` | ✅ 已有 |
| Table | `components/table.css` | ✅ 已有 |
| Modal | `components/modal.css` | 需优化 |
| Toast | `components/toast.css` | ✅ 已有 |
| Tooltip | `components/tooltip.css` | 需新增 |
| Dropdown | `components/dropdown.css` | 需优化 |

### 8.2 复合组件

| 组件 | 文件 | 状态 |
|-----|------|------|
| Sidebar | `components/sidebar.css` | ✅ 已有 |
| Header | `components/header.css` | ✅ 已有 |
| DataTable | `components/data-table.css` | 需优化 |
| ChartWidget | `components/chart-widget.css` | 需优化 |
| StatCard | `components/stat-card.css` | ✅ 已有 |
| FileExplorer | `components/file-explorer.css` | 需优化 |
| CodeEditor | `components/code-editor.css` | 需新增 |

### 8.3 布局组件

| 组件 | 文件 | 状态 |
|-----|------|------|
| PageLayout | `layouts/page.css` | ✅ 已有 |
| DashboardLayout | `layouts/dashboard.css` | 需优化 |
| SettingsLayout | `layouts/settings.css` | 需优化 |
| WizardLayout | `layouts/wizard.css` | 需新增 |

---

## 九、实施计划

### Phase 1: 设计令牌更新 (Week 1)

1. 更新 CSS 变量系统
2. 实现深色主题优化
3. 创建设计令牌文档

### Phase 2: 基础组件优化 (Week 2-3)

1. 优化按钮、输入框、卡片样式
2. 新增 Tooltip、Dropdown 组件
3. 统一组件 API

### Phase 3: Dashboard 重构 (Week 4-5)

1. 实现可定制 Widget 系统
2. 优化图表深色主题
3. 添加拖拽布局支持

### Phase 4: 移动端优化 (Week 6)

1. 实现底部导航
2. 优化触摸交互
3. 完善响应式断点

---

## 十、参考资源

- [飞牛 fnOS 界面设计](https://www.fnos.com/)
- [群晖 DSM 7.3 设计规范](https://www.synology.com/)
- [TrueNAS Scale Dashboard](https://www.truenas.com/)
- [Radix UI 设计系统](https://www.radix-ui.com/)
- [Shadcn/UI 组件库](https://ui.shadcn.com/)