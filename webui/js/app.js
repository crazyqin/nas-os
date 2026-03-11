/**
 * NAS-OS Web UI Application
 * Version: 2.0
 * Features: Dark Mode, ECharts Visualization, PWA, WebSocket Notifications
 */

// ============================================
// 全局配置
// ============================================
const CONFIG = {
    API_BASE: '/api/v1',
    WS_URL: `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/ws`,
    THEME_KEY: 'nas-os-theme',
    CHART_REFRESH_INTERVAL: 5000,
    RECONNECT_INTERVAL: 5000
};

// ============================================
// 深色模式管理
// ============================================
const ThemeManager = {
    init() {
        // 从 localStorage 或系统偏好加载主题
        const savedTheme = localStorage.getItem(CONFIG.THEME_KEY);
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        
        if (savedTheme) {
            this.setTheme(savedTheme);
        } else if (prefersDark) {
            this.setTheme('dark');
        } else {
            this.setTheme('light');
        }

        // 监听系统主题变化
        window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
            if (!localStorage.getItem(CONFIG.THEME_KEY)) {
                this.setTheme(e.matches ? 'dark' : 'light');
            }
        });
    },

    setTheme(theme) {
        document.documentElement.setAttribute('data-theme', theme);
        localStorage.setItem(CONFIG.THEME_KEY, theme);
        this.updateMetaTheme(theme);
        this.updateToggleButton();
        
        // 更新图表主题
        if (window.ChartManager) {
            ChartManager.updateTheme(theme);
        }
    },

    toggle() {
        const current = document.documentElement.getAttribute('data-theme') || 'light';
        this.setTheme(current === 'dark' ? 'light' : 'dark');
    },

    updateMetaTheme(theme) {
        const meta = document.querySelector('meta[name="theme-color"]');
        if (meta) {
            meta.setAttribute('content', theme === 'dark' ? '#1f2937' : '#2563EB');
        }
    },

    updateToggleButton() {
        const btn = document.getElementById('theme-toggle');
        if (btn) {
            const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
            btn.innerHTML = isDark 
                ? '<svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z"/></svg>'
                : '<svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z"/></svg>';
            btn.title = isDark ? '切换到亮色模式' : '切换到深色模式';
        }
    }
};

// ============================================
// 图表管理 (ECharts)
// ============================================
const ChartManager = {
    charts: {},
    refreshInterval: null,

    async init() {
        // 动态加载 ECharts
        if (typeof echarts === 'undefined') {
            await this.loadECharts();
        }
        
        this.initCharts();
        this.startAutoRefresh();
    },

    loadECharts() {
        return new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = 'https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/echarts.min.js';
            script.onload = resolve;
            script.onerror = reject;
            document.head.appendChild(script);
        });
    },

    initCharts() {
        this.initCpuChart();
        this.initMemoryChart();
        this.initDiskChart();
        this.initNetworkChart();
    },

    getChartTheme() {
        return document.documentElement.getAttribute('data-theme') === 'dark' ? 'dark' : 'light';
    },

    getChartColors() {
        const isDark = this.getChartTheme() === 'dark';
        return {
            background: isDark ? 'transparent' : '#fff',
            text: isDark ? '#e4e4e7' : '#374151',
            axisLine: isDark ? '#3f3f46' : '#e5e7eb',
            series: ['#2563EB', '#10B981', '#F59E0B', '#EF4444', '#8B5CF6']
        };
    },

    createChart(containerId, option) {
        const container = document.getElementById(containerId);
        if (!container) return null;

        const chart = echarts.init(container);
        chart.setOption(option);
        this.charts[containerId] = chart;

        // 响应式调整
        window.addEventListener('resize', () => chart.resize());
        return chart;
    },

    initCpuChart() {
        const colors = this.getChartColors();
        this.createChart('cpu-chart', {
            tooltip: { trigger: 'axis' },
            grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: this.generateTimeLabels(12),
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text }
            },
            yAxis: {
                type: 'value',
                max: 100,
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text, formatter: '{value}%' },
                splitLine: { lineStyle: { color: colors.axisLine, type: 'dashed' } }
            },
            series: [{
                name: 'CPU使用率',
                type: 'line',
                smooth: true,
                areaStyle: { opacity: 0.3 },
                lineStyle: { width: 2 },
                itemStyle: { color: colors.series[0] },
                data: this.generateRandomData(12, 20, 80)
            }]
        });
    },

    initMemoryChart() {
        const colors = this.getChartColors();
        this.createChart('memory-chart', {
            tooltip: { trigger: 'item' },
            series: [{
                name: '内存使用',
                type: 'pie',
                radius: ['50%', '70%'],
                avoidLabelOverlap: false,
                itemStyle: { borderRadius: 10, borderColor: colors.background, borderWidth: 2 },
                label: { show: false },
                emphasis: { label: { show: true, fontSize: 14, fontWeight: 'bold' } },
                labelLine: { show: false },
                data: [
                    { value: 8192, name: '已用', itemStyle: { color: colors.series[0] } },
                    { value: 4096, name: '缓存', itemStyle: { color: colors.series[1] } },
                    { value: 4096, name: '可用', itemStyle: { color: colors.series[4] } }
                ]
            }]
        });
    },

    initDiskChart() {
        const colors = this.getChartColors();
        this.createChart('disk-chart', {
            tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
            grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
            xAxis: {
                type: 'category',
                data: ['系统盘', '数据盘1', '数据盘2', '备份盘'],
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text }
            },
            yAxis: {
                type: 'value',
                max: 100,
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text, formatter: '{value}%' },
                splitLine: { lineStyle: { color: colors.axisLine, type: 'dashed' } }
            },
            series: [{
                name: '使用率',
                type: 'bar',
                barWidth: '60%',
                itemStyle: {
                    color: (params) => {
                        const colors = this.getChartColors();
                        const value = params.value;
                        if (value > 90) return colors.series[3];
                        if (value > 70) return colors.series[2];
                        return colors.series[1];
                    },
                    borderRadius: [4, 4, 0, 0]
                },
                data: [45, 72, 85, 32]
            }]
        });
    },

    initNetworkChart() {
        const colors = this.getChartColors();
        this.createChart('network-chart', {
            tooltip: { trigger: 'axis' },
            legend: { data: ['下载', '上传'], textStyle: { color: colors.text } },
            grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
            xAxis: {
                type: 'category',
                boundaryGap: false,
                data: this.generateTimeLabels(12),
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text }
            },
            yAxis: {
                type: 'value',
                axisLine: { lineStyle: { color: colors.axisLine } },
                axisLabel: { color: colors.text, formatter: '{value} MB/s' },
                splitLine: { lineStyle: { color: colors.axisLine, type: 'dashed' } }
            },
            series: [
                {
                    name: '下载',
                    type: 'line',
                    smooth: true,
                    areaStyle: { opacity: 0.3 },
                    lineStyle: { width: 2 },
                    itemStyle: { color: colors.series[0] },
                    data: this.generateRandomData(12, 5, 50)
                },
                {
                    name: '上传',
                    type: 'line',
                    smooth: true,
                    areaStyle: { opacity: 0.3 },
                    lineStyle: { width: 2 },
                    itemStyle: { color: colors.series[1] },
                    data: this.generateRandomData(12, 1, 20)
                }
            ]
        });
    },

    updateTheme(theme) {
        const colors = this.getChartColors();
        Object.values(this.charts).forEach(chart => {
            if (chart) {
                chart.dispose();
            }
        });
        this.charts = {};
        this.initCharts();
    },

    updateChartsWithData(systemData) {
        // 更新 CPU 图表
        if (this.charts['cpu-chart'] && systemData.cpu) {
            const option = this.charts['cpu-chart'].getOption();
            option.series[0].data.shift();
            option.series[0].data.push(systemData.cpu.usage);
            option.xAxis[0].data.shift();
            option.xAxis[0].data.push(new Date().toLocaleTimeString());
            this.charts['cpu-chart'].setOption(option);
        }

        // 更新内存图表
        if (this.charts['memory-chart'] && systemData.memory) {
            this.charts['memory-chart'].setOption({
                series: [{
                    data: [
                        { value: systemData.memory.used, name: '已用' },
                        { value: systemData.memory.cached, name: '缓存' },
                        { value: systemData.memory.free, name: '可用' }
                    ]
                }]
            });
        }

        // 更新网络图表
        if (this.charts['network-chart'] && systemData.network) {
            const option = this.charts['network-chart'].getOption();
            option.series[0].data.shift();
            option.series[0].data.push(systemData.network.download);
            option.series[1].data.shift();
            option.series[1].data.push(systemData.network.upload);
            option.xAxis[0].data.shift();
            option.xAxis[0].data.push(new Date().toLocaleTimeString());
            this.charts['network-chart'].setOption(option);
        }
    },

    generateTimeLabels(count) {
        const labels = [];
        for (let i = count - 1; i >= 0; i--) {
            const time = new Date(Date.now() - i * 60000);
            labels.push(time.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
        }
        return labels;
    },

    generateRandomData(count, min, max) {
        return Array.from({ length: count }, () => 
            Math.floor(Math.random() * (max - min + 1)) + min
        );
    },

    startAutoRefresh() {
        this.refreshInterval = setInterval(async () => {
            try {
                const res = await fetch(`${CONFIG.API_BASE}/system/stats`);
                if (res.ok) {
                    const data = await res.json();
                    this.updateChartsWithData(data.data || data);
                }
            } catch (e) {
                // 使用模拟数据更新
                this.updateChartsWithData(this.generateMockData());
            }
        }, CONFIG.CHART_REFRESH_INTERVAL);
    },

    generateMockData() {
        return {
            cpu: { usage: Math.random() * 40 + 20 },
            memory: {
                used: Math.random() * 4000 + 4000,
                cached: Math.random() * 2000 + 2000,
                free: Math.random() * 2000 + 2000
            },
            network: {
                download: Math.random() * 30 + 5,
                upload: Math.random() * 10 + 1
            }
        };
    },

    destroy() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        Object.values(this.charts).forEach(chart => {
            if (chart) chart.dispose();
        });
    }
};

// ============================================
// WebSocket 实时通知
// ============================================
const NotificationManager = {
    ws: null,
    reconnectAttempts: 0,
    maxReconnectAttempts: 10,
    reconnectTimeout: null,
    notifications: [],
    unreadCount: 0,

    init() {
        this.connect();
        this.setupUI();
    },

    connect() {
        try {
            this.ws = new WebSocket(CONFIG.WS_URL);

            this.ws.onopen = () => {
                console.log('[WS] 已连接');
                this.reconnectAttempts = 0;
                this.updateConnectionStatus(true);
                
                // 发送认证消息
                const token = localStorage.getItem('auth_token');
                if (token) {
                    this.ws.send(JSON.stringify({ type: 'auth', token }));
                }
            };

            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleMessage(data);
                } catch (e) {
                    console.error('[WS] 解析消息失败:', e);
                }
            };

            this.ws.onclose = () => {
                console.log('[WS] 连接关闭');
                this.updateConnectionStatus(false);
                this.scheduleReconnect();
            };

            this.ws.onerror = (error) => {
                console.error('[WS] 连接错误:', error);
                this.updateConnectionStatus(false);
            };
        } catch (e) {
            console.error('[WS] 创建连接失败:', e);
            this.scheduleReconnect();
        }
    },

    scheduleReconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.log('[WS] 达到最大重连次数');
            return;
        }

        this.reconnectAttempts++;
        const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 30000);
        
        console.log(`[WS] ${delay/1000}秒后重连 (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
        
        this.reconnectTimeout = setTimeout(() => {
            this.connect();
        }, delay);
    },

    handleMessage(data) {
        switch (data.type) {
            case 'notification':
                this.showNotification(data);
                break;
            case 'system':
                this.handleSystemMessage(data);
                break;
            case 'alert':
                this.handleAlert(data);
                break;
            case 'metrics':
                if (window.ChartManager) {
                    ChartManager.updateChartsWithData(data.payload);
                }
                break;
            default:
                console.log('[WS] 未知消息类型:', data.type);
        }
    },

    showNotification(data) {
        const notification = {
            id: data.id || Date.now(),
            title: data.title || '系统通知',
            message: data.message || data.body || '',
            type: data.level || 'info',
            timestamp: data.timestamp || new Date().toISOString(),
            read: false
        };

        this.notifications.unshift(notification);
        this.unreadCount++;
        this.updateNotificationBadge();

        // 浏览器通知
        if ('Notification' in window && Notification.permission === 'granted') {
            new Notification(notification.title, {
                body: notification.message,
                icon: '/brand/logo/logo-192.png',
                tag: notification.id
            });
        }

        // 页面内提示
        this.showToast(notification);
    },

    handleSystemMessage(data) {
        console.log('[WS] 系统消息:', data);
        this.showNotification({
            title: '系统消息',
            message: data.message,
            level: 'info'
        });
    },

    handleAlert(data) {
        console.log('[WS] 警报:', data);
        this.showNotification({
            title: '⚠️ 系统警报',
            message: data.message,
            level: data.level || 'warning'
        });
    },

    showToast(notification) {
        const container = document.getElementById('toast-container') || this.createToastContainer();
        const toast = document.createElement('div');
        toast.className = `toast toast-${notification.type}`;
        toast.innerHTML = `
            <div class="toast-header">
                <strong>${notification.title}</strong>
                <button class="toast-close" onclick="this.parentElement.parentElement.remove()">×</button>
            </div>
            <div class="toast-body">${notification.message}</div>
        `;
        container.appendChild(toast);

        // 5秒后自动消失
        setTimeout(() => {
            toast.classList.add('toast-fade-out');
            setTimeout(() => toast.remove(), 300);
        }, 5000);
    },

    createToastContainer() {
        const container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
        return container;
    },

    setupUI() {
        // 请求通知权限
        if ('Notification' in window && Notification.permission === 'default') {
            Notification.requestPermission();
        }
    },

    updateConnectionStatus(connected) {
        const indicator = document.getElementById('ws-status');
        if (indicator) {
            indicator.className = connected ? 'ws-connected' : 'ws-disconnected';
            indicator.title = connected ? '实时连接正常' : '连接断开，正在重连...';
        }
    },

    updateNotificationBadge() {
        const badge = document.getElementById('notification-badge');
        if (badge) {
            badge.textContent = this.unreadCount > 99 ? '99+' : this.unreadCount;
            badge.style.display = this.unreadCount > 0 ? 'flex' : 'none';
        }
    },

    markAllRead() {
        this.notifications.forEach(n => n.read = true);
        this.unreadCount = 0;
        this.updateNotificationBadge();
    },

    getNotifications() {
        return this.notifications;
    },

    destroy() {
        if (this.ws) {
            this.ws.close();
        }
        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
        }
    }
};

// ============================================
// PWA 管理
// ============================================
const PWAManager = {
    deferredPrompt: null,

    init() {
        this.registerServiceWorker();
        this.setupInstallPrompt();
        this.setupOfflineHandler();
    },

    async registerServiceWorker() {
        if ('serviceWorker' in navigator) {
            try {
                const registration = await navigator.serviceWorker.register('/sw.js');
                console.log('[PWA] Service Worker 注册成功:', registration.scope);

                // 检查更新
                registration.addEventListener('updatefound', () => {
                    const newWorker = registration.installing;
                    newWorker.addEventListener('statechange', () => {
                        if (newWorker.state === 'installed' && navigator.serviceWorker.controller) {
                            this.showUpdateNotification();
                        }
                    });
                });
            } catch (error) {
                console.error('[PWA] Service Worker 注册失败:', error);
            }
        }
    },

    setupInstallPrompt() {
        window.addEventListener('beforeinstallprompt', (e) => {
            e.preventDefault();
            this.deferredPrompt = e;
            this.showInstallBanner();
        });

        window.addEventListener('appinstalled', () => {
            console.log('[PWA] 应用已安装');
            this.deferredPrompt = null;
            this.hideInstallBanner();
        });
    },

    showInstallBanner() {
        const banner = document.getElementById('install-banner');
        if (banner) {
            banner.classList.add('show');
        }
    },

    hideInstallBanner() {
        const banner = document.getElementById('install-banner');
        if (banner) {
            banner.classList.remove('show');
        }
    },

    async promptInstall() {
        if (!this.deferredPrompt) return false;

        this.deferredPrompt.prompt();
        const { outcome } = await this.deferredPrompt.userChoice;
        console.log('[PWA] 用户选择:', outcome);
        
        this.deferredPrompt = null;
        return outcome === 'accepted';
    },

    showUpdateNotification() {
        const toast = document.createElement('div');
        toast.className = 'update-toast';
        toast.innerHTML = `
            <div class="update-content">
                <span>🔄 有新版本可用</span>
                <button onclick="PWAManager.updateApp()" class="btn btn-sm btn-primary">更新</button>
                <button onclick="this.parentElement.parentElement.remove()" class="btn btn-sm btn-secondary">稍后</button>
            </div>
        `;
        document.body.appendChild(toast);
    },

    async updateApp() {
        const registration = await navigator.serviceWorker.getRegistration();
        if (registration && registration.waiting) {
            registration.waiting.postMessage({ type: 'SKIP_WAITING' });
        }
        window.location.reload();
    },

    setupOfflineHandler() {
        window.addEventListener('online', () => {
            console.log('[PWA] 已连接网络');
            NotificationManager.showNotification({
                title: '网络已恢复',
                message: '您现在可以正常使用所有功能',
                level: 'success'
            });
        });

        window.addEventListener('offline', () => {
            console.log('[PWA] 网络已断开');
            NotificationManager.showNotification({
                title: '网络已断开',
                message: '您正在离线模式下运行',
                level: 'warning'
            });
        });
    }
};

// ============================================
// 移动端菜单
// ============================================
const MobileMenu = {
    init() {
        this.setupMenu();
        this.setupSwipeGesture();
    },

    setupMenu() {
        const menuBtn = document.getElementById('mobile-menu-btn');
        const overlay = document.getElementById('mobile-menu-overlay');
        const menu = document.getElementById('mobile-menu');
        const closeBtn = document.getElementById('mobile-menu-close');

        if (menuBtn && menu) {
            menuBtn.addEventListener('click', () => this.open());
        }

        if (overlay) {
            overlay.addEventListener('click', () => this.close());
        }

        if (closeBtn) {
            closeBtn.addEventListener('click', () => this.close());
        }
    },

    setupSwipeGesture() {
        let touchStartX = 0;
        let touchEndX = 0;

        document.addEventListener('touchstart', (e) => {
            touchStartX = e.changedTouches[0].screenX;
        }, { passive: true });

        document.addEventListener('touchend', (e) => {
            touchEndX = e.changedTouches[0].screenX;
            this.handleSwipe();
        }, { passive: true });

        this.handleSwipe = () => {
            const diff = touchEndX - touchStartX;
            if (diff > 100 && touchStartX < 50) {
                // 从左边缘向右滑动，打开菜单
                this.open();
            } else if (diff < -100) {
                // 向左滑动，关闭菜单
                this.close();
            }
        };
    },

    open() {
        const overlay = document.getElementById('mobile-menu-overlay');
        const menu = document.getElementById('mobile-menu');
        if (overlay) overlay.classList.add('active');
        if (menu) menu.classList.add('active');
        document.body.style.overflow = 'hidden';
    },

    close() {
        const overlay = document.getElementById('mobile-menu-overlay');
        const menu = document.getElementById('mobile-menu');
        if (overlay) overlay.classList.remove('active');
        if (menu) menu.classList.remove('active');
        document.body.style.overflow = '';
    }
};

// ============================================
// 初始化
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    // 初始化主题
    ThemeManager.init();

    // 初始化图表
    if (document.getElementById('cpu-chart')) {
        ChartManager.init();
    }

    // 初始化通知系统
    NotificationManager.init();

    // 初始化 PWA
    PWAManager.init();

    // 初始化移动端菜单
    MobileMenu.init();

    // 主题切换按钮
    const themeToggle = document.getElementById('theme-toggle');
    if (themeToggle) {
        themeToggle.addEventListener('click', () => ThemeManager.toggle());
    }

    console.log('[App] 初始化完成');
});

// 页面卸载时清理
window.addEventListener('beforeunload', () => {
    ChartManager.destroy();
    NotificationManager.destroy();
});

// 导出全局对象供 HTML 调用
window.ThemeManager = ThemeManager;
window.ChartManager = ChartManager;
window.NotificationManager = NotificationManager;
window.PWAManager = PWAManager;
window.MobileMenu = MobileMenu;