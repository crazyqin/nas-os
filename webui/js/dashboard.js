/**
 * NAS-OS Dashboard Module
 * v2.3.0 - 兵部优化
 * 功能：系统概览、服务状态、图表、实时更新、快速操作
 * 优化：WebSocket 实时数据订阅、图表渲染性能优化
 */

const Dashboard = {
    // 配置
    config: {
        apiBase: '/api/v1',
        wsUrl: null, // 动态生成
        refreshInterval: 5000,
        chartUpdateInterval: 1000,
        maxDataPoints: 60,
        // 性能优化配置
        batchUpdateInterval: 500,  // 批量更新间隔
        maxBatchSize: 10,          // 最大批量大小
        chartThrottleMs: 100       // 图表更新节流
    },

    // 状态
    state: {
        ws: null,
        wsConnected: false,
        refreshTimer: null,
        chartTimer: null,
        charts: {},
        metrics: {
            cpu: [],
            memory: [],
            network: { rx: [], tx: [] },
            io: { read: [], write: [] },
            storage: []
        },
        services: [],
        activities: [],
        autoRefresh: true,
        refreshRate: 5000,
        // 性能优化状态
        pendingUpdates: [],         // 待处理更新队列
        batchTimer: null,           // 批量更新定时器
        lastChartUpdate: 0,         // 上次图表更新时间
        isUpdating: false,          // 更新锁
        subscribers: new Map()      // WebSocket 订阅者
    },

    // 初始化
    init() {
        // 动态生成 WebSocket URL
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        this.config.wsUrl = `${protocol}//${location.host}/api/v1/system/ws`;

        this.initTheme();
        this.initCharts();
        this.connectWebSocket();
        this.loadInitialData();
        this.bindEvents();
        this.startAutoRefresh();
        
        console.log('[Dashboard] 初始化完成');
    },

    // 初始化主题
    initTheme() {
        const saved = localStorage.getItem('nas-os-theme');
        if (saved) {
            document.documentElement.setAttribute('data-theme', saved);
        }
        
        // 绑定主题切换
        const toggle = document.getElementById('theme-toggle');
        if (toggle) {
            toggle.addEventListener('click', () => this.toggleTheme());
        }
    },

    toggleTheme() {
        const current = document.documentElement.getAttribute('data-theme') || 'light';
        const next = current === 'dark' ? 'light' : 'dark';
        document.documentElement.setAttribute('data-theme', next);
        localStorage.setItem('nas-os-theme', next);
        this.updateChartsTheme();
    },

    // 初始化图表
    initCharts() {
        this.initCpuChart();
        this.initMemoryChart();
        this.initStorageChart();
        this.initIoChart();
        this.initNetworkChart();
    },

    getChartColors() {
        const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
        return {
            background: isDark ? 'transparent' : '#fff',
            text: isDark ? '#e4e4e7' : '#374151',
            gridLine: isDark ? '#3f3f46' : '#e5e7eb',
            primary: '#2563EB',
            success: '#10B981',
            warning: '#F59E0B',
            danger: '#EF4444',
            purple: '#8B5CF6'
        };
    },

    createLineChart(canvasId, label, color, maxPoints = 60) {
        const canvas = document.getElementById(canvasId);
        if (!canvas) return null;

        const colors = this.getChartColors();
        const ctx = canvas.getContext('2d');
        
        const gradient = ctx.createLinearGradient(0, 0, 0, 200);
        gradient.addColorStop(0, color + '40');
        gradient.addColorStop(1, color + '00');

        return new Chart(ctx, {
            type: 'line',
            data: {
                labels: [],
                datasets: [{
                    label: label,
                    data: [],
                    borderColor: color,
                    backgroundColor: gradient,
                    borderWidth: 2,
                    tension: 0.4,
                    fill: true,
                    pointRadius: 0,
                    pointHoverRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 0 },
                plugins: {
                    legend: { display: false },
                    tooltip: {
                        backgroundColor: colors.background,
                        titleColor: colors.text,
                        bodyColor: colors.text,
                        borderColor: colors.gridLine,
                        borderWidth: 1
                    }
                },
                scales: {
                    x: {
                        display: true,
                        grid: { display: false },
                        ticks: { 
                            color: colors.text,
                            maxTicksLimit: 6,
                            font: { size: 10 }
                        }
                    },
                    y: {
                        display: true,
                        beginAtZero: true,
                        grid: { 
                            color: colors.gridLine,
                            drawBorder: false
                        },
                        ticks: { 
                            color: colors.text,
                            font: { size: 10 }
                        }
                    }
                },
                interaction: {
                    intersect: false,
                    mode: 'index'
                }
            }
        });
    },

    initCpuChart() {
        this.state.charts.cpu = this.createLineChart('cpu-chart', 'CPU %', '#2563EB');
    },

    initMemoryChart() {
        this.state.charts.memory = this.createLineChart('memory-chart', '内存 %', '#10B981');
    },

    initStorageChart() {
        const canvas = document.getElementById('storage-chart');
        if (!canvas) return;

        const colors = this.getChartColors();
        
        this.state.charts.storage = new Chart(canvas, {
            type: 'bar',
            data: {
                labels: [],
                datasets: [{
                    label: '使用量 (GB)',
                    data: [],
                    backgroundColor: colors.primary,
                    borderRadius: 4
                }, {
                    label: '可用量 (GB)',
                    data: [],
                    backgroundColor: colors.success + '40',
                    borderRadius: 4
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                    legend: { 
                        display: true,
                        position: 'top',
                        labels: { color: colors.text, font: { size: 11 } }
                    }
                },
                scales: {
                    x: {
                        stacked: true,
                        grid: { display: false },
                        ticks: { color: colors.text, font: { size: 10 } }
                    },
                    y: {
                        stacked: true,
                        grid: { color: colors.gridLine },
                        ticks: { color: colors.text, font: { size: 10 } }
                    }
                }
            }
        });
    },

    initIoChart() {
        const canvas = document.getElementById('io-chart');
        if (!canvas) return;

        const colors = this.getChartColors();
        
        this.state.charts.io = new Chart(canvas, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: '读取 MB/s',
                        data: [],
                        borderColor: colors.success,
                        backgroundColor: colors.success + '20',
                        borderWidth: 2,
                        tension: 0.4,
                        fill: true,
                        pointRadius: 0
                    },
                    {
                        label: '写入 MB/s',
                        data: [],
                        borderColor: colors.warning,
                        backgroundColor: colors.warning + '20',
                        borderWidth: 2,
                        tension: 0.4,
                        fill: true,
                        pointRadius: 0
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 0 },
                plugins: {
                    legend: { 
                        display: true,
                        position: 'top',
                        labels: { color: colors.text, font: { size: 11 } }
                    }
                },
                scales: {
                    x: {
                        grid: { display: false },
                        ticks: { color: colors.text, maxTicksLimit: 6, font: { size: 10 } }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: colors.gridLine },
                        ticks: { color: colors.text, font: { size: 10 } }
                    }
                }
            }
        });
    },

    initNetworkChart() {
        const canvas = document.getElementById('network-chart');
        if (!canvas) return;

        const colors = this.getChartColors();
        
        this.state.charts.network = new Chart(canvas, {
            type: 'line',
            data: {
                labels: [],
                datasets: [
                    {
                        label: '下载 Mbps',
                        data: [],
                        borderColor: colors.primary,
                        backgroundColor: colors.primary + '20',
                        borderWidth: 2,
                        tension: 0.4,
                        fill: true,
                        pointRadius: 0
                    },
                    {
                        label: '上传 Mbps',
                        data: [],
                        borderColor: colors.purple,
                        backgroundColor: colors.purple + '20',
                        borderWidth: 2,
                        tension: 0.4,
                        fill: true,
                        pointRadius: 0
                    }
                ]
            },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                animation: { duration: 0 },
                plugins: {
                    legend: { 
                        display: true,
                        position: 'top',
                        labels: { color: colors.text, font: { size: 11 } }
                    }
                },
                scales: {
                    x: {
                        grid: { display: false },
                        ticks: { color: colors.text, maxTicksLimit: 6, font: { size: 10 } }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: colors.gridLine },
                        ticks: { color: colors.text, font: { size: 10 } }
                    }
                }
            }
        });
    },

    updateChartsTheme() {
        const colors = this.getChartColors();
        
        Object.values(this.state.charts).forEach(chart => {
            if (!chart) return;
            
            // 更新颜色
            if (chart.options.scales) {
                if (chart.options.scales.x) {
                    chart.options.scales.x.ticks.color = colors.text;
                }
                if (chart.options.scales.y) {
                    chart.options.scales.y.ticks.color = colors.text;
                    chart.options.scales.y.grid.color = colors.gridLine;
                }
            }
            
            if (chart.options.plugins && chart.options.plugins.legend) {
                chart.options.plugins.legend.labels.color = colors.text;
            }
            
            chart.update('none');
        });
    },

    // WebSocket 连接
    connectWebSocket() {
        if (this.state.ws) {
            this.state.ws.close();
        }

        try {
            this.state.ws = new WebSocket(this.config.wsUrl);

            this.state.ws.onopen = () => {
                console.log('[WS] 已连接');
                this.state.wsConnected = true;
                this.updateConnectionStatus(true);
            };

            this.state.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleWsMessage(data);
                } catch (e) {
                    console.error('[WS] 解析消息失败:', e);
                }
            };

            this.state.ws.onclose = () => {
                console.log('[WS] 连接关闭');
                this.state.wsConnected = false;
                this.updateConnectionStatus(false);
                this.scheduleReconnect();
            };

            this.state.ws.onerror = (err) => {
                console.error('[WS] 连接错误:', err);
                this.state.wsConnected = false;
                this.updateConnectionStatus(false);
            };
        } catch (e) {
            console.error('[WS] 创建连接失败:', e);
            this.scheduleReconnect();
        }
    },

    scheduleReconnect() {
        setTimeout(() => {
            if (!this.state.wsConnected) {
                this.connectWebSocket();
            }
        }, 5000);
    },

    // WebSocket 订阅管理
    subscribe(channel, callback) {
        if (!this.state.subscribers.has(channel)) {
            this.state.subscribers.set(channel, new Set());
        }
        this.state.subscribers.get(channel).add(callback);

        // 发送订阅消息
        if (this.state.wsConnected && this.state.ws) {
            this.state.ws.send(JSON.stringify({ type: 'subscribe', channel }));
        }

        // 返回取消订阅函数
        return () => {
            const callbacks = this.state.subscribers.get(channel);
            if (callbacks) {
                callbacks.delete(callback);
                if (callbacks.size === 0) {
                    this.state.subscribers.delete(channel);
                    if (this.state.wsConnected && this.state.ws) {
                        this.state.ws.send(JSON.stringify({ type: 'unsubscribe', channel }));
                    }
                }
            }
        };
    },

    // 发布消息给订阅者
    notifySubscribers(channel, data) {
        const callbacks = this.state.subscribers.get(channel);
        if (callbacks) {
            callbacks.forEach(cb => {
                try {
                    cb(data);
                } catch (e) {
                    console.error(`[Dashboard] 订阅者回调错误 [${channel}]:`, e);
                }
            });
        }
    },

    handleWsMessage(data) {
        // 批量处理消息
        this.queueUpdate(data);
    },

    // 队列更新（性能优化）
    queueUpdate(data) {
        this.state.pendingUpdates.push(data);
        
        if (!this.state.batchTimer) {
            this.state.batchTimer = setTimeout(() => {
                this.processBatchUpdates();
            }, this.config.batchUpdateInterval);
        }
    },

    // 批量处理更新
    processBatchUpdates() {
        this.state.batchTimer = null;
        
        if (this.state.pendingUpdates.length === 0) return;
        
        // 合并同类型更新
        const updates = this.state.pendingUpdates;
        this.state.pendingUpdates = [];

        const mergedData = {
            system: null,
            disks: null,
            network: null,
            alerts: []
        };

        updates.forEach(data => {
            switch (data.type) {
                case 'init':
                case 'system':
                    if (data.system) mergedData.system = { ...mergedData.system, ...data.system };
                    if (data.disks) mergedData.disks = data.disks;
                    if (data.network) mergedData.network = data.network;
                    break;
                case 'metrics':
                    mergedData.system = { ...mergedData.system, ...data.payload };
                    break;
                case 'alert':
                case 'notification':
                    mergedData.alerts.push(data);
                    break;
            }

            // 通知订阅者
            this.notifySubscribers(data.type, data);
            this.notifySubscribers('*', data);
        });

        // 应用合并后的更新
        if (mergedData.system) {
            this.updateSystemStats(mergedData.system);
        }
        if (mergedData.disks) {
            this.updateDiskStats(mergedData.disks);
        }
        if (mergedData.network) {
            this.updateNetworkStats(mergedData.network);
        }
        mergedData.alerts.forEach(alert => {
            this.showNotification(alert.level || 'info', alert.message);
        });
    },

    // 旧版兼容
    handleWsMessageLegacy(data) {
        switch (data.type) {
            case 'init':
            case 'system':
                if (data.system) this.updateSystemStats(data.system);
                if (data.disks) this.updateDiskStats(data.disks);
                if (data.network) this.updateNetworkStats(data.network);
                break;
            case 'metrics':
                this.handleMetrics(data.payload);
                break;
            case 'alert':
                this.showNotification('warning', data.message);
                break;
            case 'notification':
                this.showNotification(data.level || 'info', data.message);
                break;
        }
    },

    updateConnectionStatus(connected) {
        const indicator = document.getElementById('ws-status');
        const text = document.getElementById('connection-status');
        
        if (indicator) {
            indicator.className = connected ? 'status-dot connected' : 'status-dot disconnected';
        }
        if (text) {
            text.textContent = connected ? '实时连接中' : '连接断开';
        }
    },

    // 加载初始数据
    async loadInitialData() {
        await Promise.all([
            this.loadSystemStats(),
            this.loadServices(),
            this.loadActivities(),
            this.loadDisks()
        ]);
    },

    async loadSystemStats() {
        try {
            const res = await fetch(`${this.config.apiBase}/system/stats`);
            const result = await res.json();
            if (result.code === 0 && result.data) {
                this.updateSystemStats(result.data);
            }
        } catch (e) {
            console.error('[Dashboard] 加载系统统计失败:', e);
            this.setMockSystemStats();
        }
    },

    async loadServices() {
        const serviceEndpoints = [
            { name: 'SMB', url: `${this.config.apiBase}/shares/smb/status`, icon: '📁' },
            { name: 'NFS', url: `${this.config.apiBase}/nfs/status`, icon: '📂' },
            { name: 'iSCSI', url: `${this.config.apiBase}/iscsi/status`, icon: '💿' },
            { name: 'WebDAV', url: `${this.config.apiBase}/webdav/status`, icon: '🌐' }
        ];

        const servicesHtml = await Promise.all(serviceEndpoints.map(async (service) => {
            let status = 'unknown';
            let statusText = '未知';
            
            try {
                const res = await fetch(service.url);
                const result = await res.json();
                if (result.code === 0 && result.data) {
                    status = result.data.running ? 'running' : 'stopped';
                    statusText = result.data.running ? '运行中' : '已停止';
                }
            } catch (e) {
                status = 'unknown';
                statusText = '无法连接';
            }

            const statusClass = status === 'running' ? 'success' : status === 'stopped' ? 'error' : 'warning';
            
            return `
                <div class="service-item">
                    <span class="service-icon">${service.icon}</span>
                    <span class="service-name">${service.name}</span>
                    <span class="service-status badge badge-${statusClass}">${statusText}</span>
                </div>
            `;
        }));

        const container = document.getElementById('services-list');
        if (container) {
            container.innerHTML = servicesHtml.join('');
        }
    },

    async loadActivities() {
        try {
            const res = await fetch(`${this.config.apiBase}/system/alerts?limit=10`);
            const result = await res.json();
            if (result.code === 0 && result.data) {
                this.renderActivities(result.data);
            }
        } catch (e) {
            console.error('[Dashboard] 加载活动日志失败:', e);
            this.setMockActivities();
        }
    },

    async loadDisks() {
        try {
            const res = await fetch(`${this.config.apiBase}/system/disks`);
            const result = await res.json();
            if (result.code === 0 && result.data) {
                this.updateDiskStats(result.data);
            }
        } catch (e) {
            console.error('[Dashboard] 加载磁盘信息失败:', e);
        }
    },

    // 更新系统统计
    updateSystemStats(stats) {
        // CPU
        if (stats.cpuUsage !== undefined) {
            const cpuValue = stats.cpuUsage.toFixed(1);
            document.getElementById('cpu-value').textContent = cpuValue + '%';
            document.getElementById('cpu-bar').style.width = cpuValue + '%';
            this.setProgressBarColor('cpu-bar', stats.cpuUsage);
            
            this.addMetricPoint('cpu', stats.cpuUsage);
        }

        // 内存
        if (stats.memoryUsage !== undefined) {
            const memValue = stats.memoryUsage.toFixed(1);
            document.getElementById('memory-value').textContent = memValue + '%';
            document.getElementById('memory-bar').style.width = memValue + '%';
            this.setProgressBarColor('memory-bar', stats.memoryUsage);
            
            if (stats.memoryUsed && stats.memoryTotal) {
                const usedGB = (stats.memoryUsed / 1024 / 1024 / 1024).toFixed(1);
                const totalGB = (stats.memoryTotal / 1024 / 1024 / 1024).toFixed(1);
                document.getElementById('memory-detail').textContent = `${usedGB} / ${totalGB} GB`;
            }
            
            this.addMetricPoint('memory', stats.memoryUsage);
        }

        // 系统信息
        if (stats.cpuCores) {
            document.getElementById('cpu-cores').textContent = stats.cpuCores;
        }
        if (stats.cpuTemp) {
            document.getElementById('cpu-temp').textContent = stats.cpuTemp + '°C';
        }
        if (stats.uptime) {
            document.getElementById('system-uptime').textContent = stats.uptime;
        }
        if (stats.loadAvg) {
            document.getElementById('load-avg').textContent = stats.loadAvg.map(l => l.toFixed(2)).join(' / ');
        }
        if (stats.processes) {
            document.getElementById('process-count').textContent = stats.processes;
        }
        if (stats.hostname) {
            document.getElementById('hostname').textContent = stats.hostname;
        }

        // 更新图表
        this.updateCharts();
    },

    updateNetworkStats(network) {
        if (!network || !Array.isArray(network)) return;

        let totalRx = 0, totalTx = 0;
        network.forEach(n => {
            totalRx += n.rxSpeed || 0;
            totalTx += n.txSpeed || 0;
        });

        // 转换为 MB/s
        const rxBps = totalRx / 1024;
        const txBps = totalTx / 1024;

        document.getElementById('rx-speed').textContent = rxBps.toFixed(1);
        document.getElementById('tx-speed').textContent = txBps.toFixed(1);

        const totalMbps = (rxBps + txBps) * 8 / 1000;
        document.getElementById('network-value').textContent = totalMbps.toFixed(1) + ' Mbps';

        // 更新网络图表
        this.addMetricPoint('network.rx', rxBps);
        this.addMetricPoint('network.tx', txBps);
        this.updateCharts();
    },

    updateDiskStats(disks) {
        if (!disks || !Array.isArray(disks)) return;

        const container = document.getElementById('disk-list');
        if (!container) return;

        document.getElementById('disk-count').textContent = disks.length;

        // 计算总存储
        let totalUsed = 0, totalSize = 0;
        
        const html = disks.map(disk => {
            const usedGB = (disk.used / 1024 / 1024 / 1024).toFixed(1);
            const totalGB = (disk.total / 1024 / 1024 / 1024).toFixed(1);
            const percent = disk.usagePercent.toFixed(1);
            
            totalUsed += disk.used;
            totalSize += disk.total;

            let barClass = '';
            if (disk.usagePercent > 90) barClass = 'error';
            else if (disk.usagePercent > 70) barClass = 'warning';

            return `
                <div class="disk-item">
                    <div class="disk-header">
                        <span class="disk-name">${disk.device}</span>
                        <span class="disk-mount">${disk.mountPoint}</span>
                    </div>
                    <div class="progress" style="height: 6px;">
                        <div class="progress-bar ${barClass}" style="width: ${percent}%"></div>
                    </div>
                    <div class="disk-info">
                        <span>${usedGB} / ${totalGB} GB</span>
                        <span>${percent}%</span>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = html;

        // 更新总存储
        const totalPercent = totalSize > 0 ? (totalUsed / totalSize * 100).toFixed(1) : 0;
        document.getElementById('storage-value').textContent = totalPercent + '%';
        document.getElementById('storage-bar').style.width = totalPercent + '%';
        this.setProgressBarColor('storage-bar', parseFloat(totalPercent));

        // 更新存储图表
        if (this.state.charts.storage) {
            const labels = disks.map(d => d.device);
            const used = disks.map(d => (d.used / 1024 / 1024 / 1024));
            const free = disks.map(d => ((d.total - d.used) / 1024 / 1024 / 1024));
            
            this.state.charts.storage.data.labels = labels;
            this.state.charts.storage.data.datasets[0].data = used;
            this.state.charts.storage.data.datasets[1].data = free;
            this.state.charts.storage.update('none');
        }
    },

    renderActivities(activities) {
        const container = document.getElementById('activities-list');
        if (!container) return;

        if (!activities || activities.length === 0) {
            container.innerHTML = '<div class="empty-state">暂无活动记录</div>';
            return;
        }

        const html = activities.map(activity => {
            const levelClass = activity.level || 'info';
            const time = new Date(activity.timestamp).toLocaleString('zh-CN');
            const icon = this.getActivityIcon(activity.type);
            
            return `
                <div class="activity-item activity-${levelClass}">
                    <span class="activity-icon">${icon}</span>
                    <div class="activity-content">
                        <div class="activity-message">${activity.message}</div>
                        <div class="activity-time">${time}</div>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = html;
    },

    getActivityIcon(type) {
        const icons = {
            'alert': '⚠️',
            'warning': '⚡',
            'error': '❌',
            'success': '✅',
            'info': 'ℹ️',
            'backup': '💾',
            'user': '👤',
            'system': '🔧',
            'network': '🌐'
        };
        return icons[type] || '📋';
    },

    // 指标管理
    addMetricPoint(key, value) {
        const keys = key.split('.');
        let target = this.state.metrics;
        
        for (let i = 0; i < keys.length - 1; i++) {
            if (!target[keys[i]]) target[keys[i]] = {};
            target = target[keys[i]];
        }
        
        const arr = target[keys[keys.length - 1]];
        if (Array.isArray(arr)) {
            arr.push(value);
            if (arr.length > this.config.maxDataPoints) {
                arr.shift();
            }
        }
    },

    updateCharts() {
        // 节流：避免频繁更新图表
        const now = Date.now();
        if (now - this.state.lastChartUpdate < this.config.chartThrottleMs) {
            return;
        }
        this.state.lastChartUpdate = now;

        // 使用 requestAnimationFrame 优化渲染
        requestAnimationFrame(() => {
            this._updateChartsInternal();
        });
    },

    _updateChartsInternal() {
        const timeLabel = new Date().toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
        
        // CPU 图表
        if (this.state.charts.cpu && this.state.metrics.cpu.length > 0) {
            this.updateChartSafely(this.state.charts.cpu, timeLabel, 
                this.state.metrics.cpu[this.state.metrics.cpu.length - 1]);
        }

        // 内存图表
        if (this.state.charts.memory && this.state.metrics.memory.length > 0) {
            this.updateChartSafely(this.state.charts.memory, timeLabel,
                this.state.metrics.memory[this.state.metrics.memory.length - 1]);
        }

        // 网络图表
        if (this.state.charts.network && this.state.metrics.network.rx.length > 0) {
            const chart = this.state.charts.network;
            chart.data.labels.push(timeLabel);
            chart.data.datasets[0].data.push(this.state.metrics.network.rx[this.state.metrics.network.rx.length - 1]);
            chart.data.datasets[1].data.push(this.state.metrics.network.tx[this.state.metrics.network.tx.length - 1]);
            if (chart.data.labels.length > this.config.maxDataPoints) {
                chart.data.labels.shift();
                chart.data.datasets[0].data.shift();
                chart.data.datasets[1].data.shift();
            }
            chart.update('none');
        }
    },

    // 安全更新单个数据集图表
    updateChartSafely(chart, label, value) {
        if (!chart) return;
        
        chart.data.labels.push(label);
        chart.data.datasets[0].data.push(value);
        
        if (chart.data.labels.length > this.config.maxDataPoints) {
            chart.data.labels.shift();
            chart.data.datasets[0].data.shift();
        }
        
        chart.update('none');
    },

    // 批量更新图表数据（性能优化）
    updateChartsBatch(updates) {
        Object.keys(updates).forEach(key => {
            if (this.state.metrics[key]) {
                const arr = Array.isArray(this.state.metrics[key]) ? this.state.metrics[key] : null;
                if (arr) {
                    arr.push(updates[key]);
                    if (arr.length > this.config.maxDataPoints) {
                        arr.shift();
                    }
                }
            }
        });
        
        // 只触发一次图表更新
        this.updateCharts();
    },

    setProgressBarColor(id, value) {
        const bar = document.getElementById(id);
        if (!bar) return;
        
        bar.classList.remove('success', 'warning', 'error');
        if (value > 90) bar.classList.add('error');
        else if (value > 70) bar.classList.add('warning');
        else bar.classList.add('success');
    },

    // 自动刷新
    startAutoRefresh() {
        if (this.state.refreshTimer) {
            clearInterval(this.state.refreshTimer);
        }

        this.state.refreshTimer = setInterval(() => {
            if (this.state.autoRefresh) {
                this.loadSystemStats();
                this.loadServices();
            }
        }, this.state.refreshRate);
    },

    // 事件绑定
    bindEvents() {
        // 刷新按钮
        const refreshBtn = document.getElementById('refresh-btn');
        if (refreshBtn) {
            refreshBtn.addEventListener('click', () => this.manualRefresh());
        }

        // 自动刷新切换
        const autoRefreshToggle = document.getElementById('auto-refresh-toggle');
        if (autoRefreshToggle) {
            autoRefreshToggle.addEventListener('change', (e) => {
                this.state.autoRefresh = e.target.checked;
            });
        }

        // 刷新间隔选择
        const refreshRateSelect = document.getElementById('refresh-rate');
        if (refreshRateSelect) {
            refreshRateSelect.addEventListener('change', (e) => {
                this.state.refreshRate = parseInt(e.target.value);
                this.startAutoRefresh();
            });
        }

        // 快速操作按钮
        this.bindQuickActions();
    },

    bindQuickActions() {
        // 创建快照
        const createSnapshotBtn = document.getElementById('create-snapshot-btn');
        if (createSnapshotBtn) {
            createSnapshotBtn.addEventListener('click', () => this.createSnapshot());
        }

        // 创建共享
        const createShareBtn = document.getElementById('create-share-btn');
        if (createShareBtn) {
            createShareBtn.addEventListener('click', () => this.openCreateShareModal());
        }

        // 系统重启
        const rebootBtn = document.getElementById('reboot-btn');
        if (rebootBtn) {
            rebootBtn.addEventListener('click', () => this.confirmAction('重启系统', () => this.rebootSystem()));
        }

        // 系统关机
        const shutdownBtn = document.getElementById('shutdown-btn');
        if (shutdownBtn) {
            shutdownBtn.addEventListener('click', () => this.confirmAction('关闭系统', () => this.shutdownSystem()));
        }
    },

    // 快速操作
    async createSnapshot() {
        const btn = document.getElementById('create-snapshot-btn');
        const originalText = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = '⏳ 创建中...';

        try {
            const res = await fetch(`${this.config.apiBase}/snapshots`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name: `auto-${Date.now()}`, description: '仪表板快速创建' })
            });
            
            const result = await res.json();
            if (result.code === 0) {
                this.showNotification('success', '快照创建成功');
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            this.showNotification('error', '快照创建失败: ' + e.message);
        } finally {
            btn.disabled = false;
            btn.innerHTML = originalText;
        }
    },

    openCreateShareModal() {
        // 简单实现：跳转到共享页面
        window.location.href = 'shares.html?action=create';
    },

    confirmAction(actionName, callback) {
        const modal = document.createElement('div');
        modal.className = 'modal-overlay';
        modal.innerHTML = `
            <div class="modal">
                <div class="modal-header">
                    <h3>⚠️ 确认${actionName}</h3>
                </div>
                <div class="modal-body">
                    <p>您确定要${actionName}吗？此操作不可撤销。</p>
                    <div class="form-group">
                        <label>请输入 "CONFIRM" 以确认：</label>
                        <input type="text" id="confirm-input" class="form-input" placeholder="CONFIRM">
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-secondary" onclick="this.closest('.modal-overlay').remove()">取消</button>
                    <button class="btn btn-danger" id="confirm-action-btn" disabled>${actionName}</button>
                </div>
            </div>
        `;

        document.body.appendChild(modal);

        const input = modal.querySelector('#confirm-input');
        const confirmBtn = modal.querySelector('#confirm-action-btn');

        input.addEventListener('input', (e) => {
            confirmBtn.disabled = e.target.value !== 'CONFIRM';
        });

        confirmBtn.addEventListener('click', () => {
            modal.remove();
            callback();
        });
    },

    async rebootSystem() {
        try {
            await fetch(`${this.config.apiBase}/system/reboot`, { method: 'POST' });
            this.showNotification('info', '系统正在重启...');
        } catch (e) {
            this.showNotification('error', '重启命令发送失败');
        }
    },

    async shutdownSystem() {
        try {
            await fetch(`${this.config.apiBase}/system/shutdown`, { method: 'POST' });
            this.showNotification('info', '系统正在关闭...');
        } catch (e) {
            this.showNotification('error', '关机命令发送失败');
        }
    },

    // 手动刷新
    manualRefresh() {
        this.loadInitialData();
        this.showNotification('info', '数据已刷新');
    },

    // 通知
    showNotification(type, message) {
        const container = document.getElementById('toast-container') || this.createToastContainer();
        const toast = document.createElement('div');
        toast.className = `toast toast-${type}`;
        toast.innerHTML = `
            <div class="toast-header">
                <strong>${type === 'success' ? '✅ 成功' : type === 'error' ? '❌ 错误' : type === 'warning' ? '⚠️ 警告' : 'ℹ️ 信息'}</strong>
                <button class="toast-close" onclick="this.closest('.toast').remove()">×</button>
            </div>
            <div class="toast-body">${message}</div>
        `;
        container.appendChild(toast);

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

    // 模拟数据（用于演示）
    setMockSystemStats() {
        const mockData = {
            cpuUsage: Math.random() * 40 + 20,
            memoryUsage: Math.random() * 30 + 40,
            cpuCores: 8,
            cpuTemp: Math.floor(Math.random() * 20 + 45),
            uptime: '15天 6小时 32分钟',
            loadAvg: [1.2, 0.9, 0.7],
            processes: 186,
            hostname: 'nas-server',
            memoryUsed: 8 * 1024 * 1024 * 1024,
            memoryTotal: 16 * 1024 * 1024 * 1024
        };
        this.updateSystemStats(mockData);
    },

    setMockActivities() {
        const mockActivities = [
            { type: 'backup', level: 'success', message: '自动备份完成', timestamp: new Date(Date.now() - 3600000).toISOString() },
            { type: 'user', level: 'info', message: '用户 admin 登录', timestamp: new Date(Date.now() - 7200000).toISOString() },
            { type: 'system', level: 'info', message: 'SMB 服务已重启', timestamp: new Date(Date.now() - 14400000).toISOString() },
            { type: 'alert', level: 'warning', message: '磁盘 /dev/sda 使用率超过 80%', timestamp: new Date(Date.now() - 28800000).toISOString() }
        ];
        this.renderActivities(mockActivities);
    },

    // 清理
    destroy() {
        if (this.state.ws) {
            this.state.ws.close();
        }
        if (this.state.refreshTimer) {
            clearInterval(this.state.refreshTimer);
        }
        if (this.state.batchTimer) {
            clearTimeout(this.state.batchTimer);
        }
        // 清理订阅者
        this.state.subscribers.clear();
        // 清理待处理更新
        this.state.pendingUpdates = [];
        // 销毁图表
        Object.values(this.state.charts).forEach(chart => {
            if (chart) chart.destroy();
        });
    }
};

// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    Dashboard.init();
});

// 页面卸载时清理
window.addEventListener('beforeunload', () => {
    Dashboard.destroy();
});

// 导出全局对象
window.Dashboard = Dashboard;