/**
 * NAS-OS Storage Dashboard Module
 * v1.0.0 - 兵部实现
 * 功能：存储池状态可视化、容量趋势图表、告警通知组件
 */

const StorageDashboard = {
    // 配置
    config: {
        apiBase: '/api/v1',
        wsChannel: 'storage',
        maxDataPoints: 60,
        alertCheckInterval: 30000
    },

    // 状态
    state: {
        ws: null,
        wsConnected: false,
        charts: {},
        pools: [],
        alerts: [],
        metrics: {
            capacity: [],
            cost: []
        },
        alertTimer: null
    },

    // 初始化
    init() {
        console.log('[StorageDashboard] 初始化...');
        this.initCharts();
        this.loadInitialData();
        this.bindEvents();
        this.startAlertCheck();
        this.subscribeToUpdates();
    },

    // 初始化图表
    initCharts() {
        this.initCapacityTrendChart();
        this.initCostChart();
    },

    getChartColors() {
        const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
        return {
            text: isDark ? '#e4e4e7' : '#374151',
            gridLine: isDark ? '#3f3f46' : '#e5e7eb',
            primary: '#2563EB',
            success: '#10B981',
            warning: '#F59E0B',
            danger: '#EF4444',
            purple: '#8B5CF6'
        };
    },

    // 容量趋势图表
    initCapacityTrendChart() {
        const canvas = document.getElementById('capacity-trend-chart');
        if (!canvas) return;

        const colors = this.getChartColors();
        const ctx = canvas.getContext('2d');

        const gradient = ctx.createLinearGradient(0, 0, 0, 200);
        gradient.addColorStop(0, colors.primary + '40');
        gradient.addColorStop(1, colors.primary + '00');

        this.state.charts.capacity = new Chart(ctx, {
            type: 'line',
            data: {
                labels: this.generateTimeLabels(24),
                datasets: [{
                    label: '已用容量 (TB)',
                    data: [],
                    borderColor: colors.primary,
                    backgroundColor: gradient,
                    borderWidth: 2,
                    tension: 0.4,
                    fill: true,
                    pointRadius: 0,
                    pointHoverRadius: 4
                }, {
                    label: '可用容量 (TB)',
                    data: [],
                    borderColor: colors.success,
                    backgroundColor: colors.success + '20',
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

    // 成本分析图表
    initCostChart() {
        const canvas = document.getElementById('cost-chart');
        if (!canvas) return;

        const colors = this.getChartColors();

        this.state.charts.cost = new Chart(canvas, {
            type: 'bar',
            data: {
                labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
                datasets: [{
                    label: '存储成本',
                    data: [],
                    backgroundColor: colors.primary,
                    borderRadius: 4
                }, {
                    label: '流量成本',
                    data: [],
                    backgroundColor: colors.purple,
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
                        grid: { display: false },
                        ticks: { color: colors.text, font: { size: 10 } }
                    },
                    y: {
                        beginAtZero: true,
                        grid: { color: colors.gridLine },
                        ticks: {
                            color: colors.text,
                            font: { size: 10 },
                            callback: (value) => '¥' + value
                        }
                    }
                }
            }
        });
    },

    // 加载初始数据
    async loadInitialData() {
        await Promise.all([
            this.loadStorageHealth(),
            this.loadStoragePools(),
            this.loadAlerts(),
            this.loadCapacityTrend(),
            this.loadCostAnalysis()
        ]);
    },

    // 加载存储健康概览
    async loadStorageHealth() {
        try {
            const res = await fetch(`${this.config.apiBase}/system/disks/health`);
            const result = await res.json();
            
            if (result.code === 0 && result.data) {
                this.renderStorageHealth(result.data);
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            console.error('[StorageDashboard] 加载存储健康失败:', e);
            this.renderStorageHealth(this.getMockHealthData());
        }
    },

    // 渲染存储健康概览
    renderStorageHealth(data) {
        const container = document.getElementById('storage-health-grid');
        const statusBadge = document.getElementById('storage-health-status');
        if (!container) return;

        const disks = data.disks || data;
        let allHealthy = true;
        let hasWarning = false;

        const html = disks.map(disk => {
            const status = disk.health || 'healthy';
            if (status === 'warning') {
                allHealthy = false;
                hasWarning = true;
            } else if (status === 'critical') {
                allHealthy = false;
            }

            const icon = this.getHealthIcon(status);
            const statusClass = status === 'healthy' ? '' : status;

            return `
                <div class="health-card ${statusClass}">
                    <span class="health-icon">${icon}</span>
                    <div class="health-info">
                        <div class="health-label">${disk.device || disk.name}</div>
                        <div class="health-value">${disk.temperature || '--'}°C · ${disk.healthPercent || 100}%</div>
                        <div class="health-detail">${disk.model || 'Unknown'} · ${this.formatBytes(disk.size)}</div>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = html;

        // 更新状态徽章
        if (statusBadge) {
            if (allHealthy) {
                statusBadge.className = 'status-badge healthy';
                statusBadge.textContent = '健康';
            } else if (hasWarning) {
                statusBadge.className = 'status-badge warning';
                statusBadge.textContent = '警告';
            } else {
                statusBadge.className = 'status-badge critical';
                statusBadge.textContent = '异常';
            }
        }
    },

    getHealthIcon(status) {
        const icons = {
            'healthy': '✅',
            'warning': '⚠️',
            'critical': '❌',
            'unknown': '❓'
        };
        return icons[status] || icons.unknown;
    },

    // 加载存储池状态
    async loadStoragePools() {
        try {
            const res = await fetch(`${this.config.apiBase}/storage/pools`);
            const result = await res.json();

            if (result.code === 0 && result.data) {
                this.state.pools = result.data;
                this.renderStoragePools(result.data);
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            console.error('[StorageDashboard] 加载存储池失败:', e);
            this.renderStoragePools(this.getMockPoolsData());
        }
    },

    // 渲染存储池状态
    renderStoragePools(pools) {
        const container = document.getElementById('storage-pools-list');
        if (!container) return;

        if (!pools || pools.length === 0) {
            container.innerHTML = '<div class="empty-state">暂无存储池</div>';
            return;
        }

        const html = pools.map(pool => {
            const statusClass = pool.status === 'online' ? 'online' : 
                               pool.status === 'degraded' ? 'degraded' : 'offline';
            const usedGB = (pool.used / 1024 / 1024 / 1024).toFixed(1);
            const totalGB = (pool.total / 1024 / 1024 / 1024).toFixed(1);
            const percent = pool.total > 0 ? (pool.used / pool.total * 100).toFixed(1) : 0;

            return `
                <div class="pool-item">
                    <div class="pool-header">
                        <span class="pool-name">
                            <span>🗄️</span>
                            ${pool.name}
                        </span>
                        <span class="pool-status ${statusClass}">${this.getStatusText(pool.status)}</span>
                    </div>
                    <div class="progress" style="height: 8px; margin-bottom: 12px;">
                        <div class="progress-bar ${percent > 90 ? 'error' : percent > 70 ? 'warning' : 'success'}" 
                             style="width: ${percent}%"></div>
                    </div>
                    <div class="pool-stats">
                        <div class="pool-stat">
                            <span class="pool-stat-label">容量</span>
                            <span class="pool-stat-value">${usedGB} / ${totalGB} GB</span>
                        </div>
                        <div class="pool-stat">
                            <span class="pool-stat-label">使用率</span>
                            <span class="pool-stat-value">${percent}%</span>
                        </div>
                        <div class="pool-stat">
                            <span class="pool-stat-label">磁盘数</span>
                            <span class="pool-stat-value">${pool.diskCount || '--'}</span>
                        </div>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = html;
    },

    getStatusText(status) {
        const texts = {
            'online': '在线',
            'degraded': '降级',
            'offline': '离线',
            'rebuilding': '重建中'
        };
        return texts[status] || status;
    },

    // 加载告警
    async loadAlerts() {
        try {
            const res = await fetch(`${this.config.apiBase}/system/alerts?level=warning,critical&limit=10`);
            const result = await res.json();

            if (result.code === 0 && result.data) {
                this.state.alerts = result.data;
                this.renderAlerts(result.data);
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            console.error('[StorageDashboard] 加载告警失败:', e);
            this.renderAlerts(this.getMockAlerts());
        }
    },

    // 渲染告警
    renderAlerts(alerts) {
        const container = document.getElementById('alerts-list');
        const countBadge = document.getElementById('alert-count');
        
        if (!container) return;

        if (countBadge) {
            const criticalCount = alerts.filter(a => a.level === 'critical').length;
            countBadge.textContent = `${alerts.length} 条告警`;
            countBadge.style.color = criticalCount > 0 ? 'var(--color-error)' : 'var(--text-muted)';
        }

        if (!alerts || alerts.length === 0) {
            container.innerHTML = '<div class="empty-state">✅ 暂无告警</div>';
            return;
        }

        const html = alerts.map(alert => {
            const levelClass = alert.level === 'critical' ? 'critical' : 
                              alert.level === 'warning' ? '' : 'info';
            const icon = alert.level === 'critical' ? '🚨' : 
                        alert.level === 'warning' ? '⚠️' : 'ℹ️';
            const time = new Date(alert.timestamp).toLocaleString('zh-CN');

            return `
                <div class="alert-item ${levelClass}" data-id="${alert.id}">
                    <span class="alert-icon">${icon}</span>
                    <div class="alert-content">
                        <div class="alert-message">${alert.message}</div>
                        <div class="alert-time">${time}</div>
                    </div>
                    <button class="alert-dismiss" onclick="StorageDashboard.dismissAlert('${alert.id}')">✕</button>
                </div>
            `;
        }).join('');

        container.innerHTML = html;
    },

    // 消除告警
    async dismissAlert(alertId) {
        try {
            await fetch(`${this.config.apiBase}/system/alerts/${alertId}`, {
                method: 'DELETE'
            });

            this.state.alerts = this.state.alerts.filter(a => a.id !== alertId);
            this.renderAlerts(this.state.alerts);
        } catch (e) {
            console.error('[StorageDashboard] 消除告警失败:', e);
        }
    },

    // 加载容量趋势
    async loadCapacityTrend() {
        try {
            const res = await fetch(`${this.config.apiBase}/storage/capacity/trend?period=day`);
            const result = await res.json();

            if (result.code === 0 && result.data) {
                this.updateCapacityChart(result.data);
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            console.error('[StorageDashboard] 加载容量趋势失败:', e);
            this.updateCapacityChart(this.getMockCapacityTrend());
        }
    },

    // 更新容量图表
    updateCapacityChart(data) {
        const chart = this.state.charts.capacity;
        if (!chart) return;

        chart.data.labels = data.labels || this.generateTimeLabels(24);
        chart.data.datasets[0].data = data.used || [];
        chart.data.datasets[1].data = data.free || [];
        chart.update('none');
    },

    // 加载成本分析
    async loadCostAnalysis() {
        try {
            const period = document.getElementById('cost-period-select')?.value || 'week';
            const res = await fetch(`${this.config.apiBase}/storage/cost/analysis?period=${period}`);
            const result = await res.json();

            if (result.code === 0 && result.data) {
                this.updateCostChart(result.data);
                this.updateCostSummary(result.data);
            } else {
                throw new Error(result.message);
            }
        } catch (e) {
            console.error('[StorageDashboard] 加载成本分析失败:', e);
            const mockData = this.getMockCostData();
            this.updateCostChart(mockData);
            this.updateCostSummary(mockData);
        }
    },

    // 更新成本图表
    updateCostChart(data) {
        const chart = this.state.charts.cost;
        if (!chart) return;

        chart.data.labels = data.labels || ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
        chart.data.datasets[0].data = data.storageCosts || [];
        chart.data.datasets[1].data = data.trafficCosts || [];
        chart.update('none');
    },

    // 更新成本摘要
    updateCostSummary(data) {
        const storageCost = document.getElementById('storage-cost');
        const trafficCost = document.getElementById('traffic-cost');
        const totalCost = document.getElementById('total-cost');

        const sc = data.totalStorageCost || 0;
        const tc = data.totalTrafficCost || 0;
        const total = sc + tc;

        if (storageCost) storageCost.textContent = `¥${sc.toFixed(2)}`;
        if (trafficCost) trafficCost.textContent = `¥${tc.toFixed(2)}`;
        if (totalCost) totalCost.textContent = `¥${total.toFixed(2)}`;
    },

    // WebSocket 订阅实时更新
    subscribeToUpdates() {
        // 复用主 Dashboard 的 WebSocket 连接
        if (window.Dashboard && window.Dashboard.state.ws) {
            this.state.ws = window.Dashboard.state.ws;
            this.state.wsConnected = true;
        }

        // 监听 WebSocket 消息
        const originalHandler = window.Dashboard?.handleWsMessage;
        if (window.Dashboard) {
            window.Dashboard.handleWsMessage = (data) => {
                if (originalHandler) originalHandler.call(window.Dashboard, data);
                this.handleWsMessage(data);
            };
        }
    },

    handleWsMessage(data) {
        switch (data.type) {
            case 'storage':
            case 'pool':
                if (data.pools) this.renderStoragePools(data.pools);
                break;
            case 'health':
                if (data.disks) this.renderStorageHealth(data.disks);
                break;
            case 'alert':
                this.state.alerts.unshift(data);
                this.renderAlerts(this.state.alerts);
                break;
            case 'metrics':
                if (data.storage) {
                    this.updateRealtimeMetrics(data.storage);
                }
                break;
        }
    },

    // 更新实时监控指标
    updateRealtimeMetrics(metrics) {
        // CPU
        if (metrics.cpu !== undefined) {
            const cpuEl = document.getElementById('realtime-cpu');
            const cpuBar = document.getElementById('realtime-cpu-bar');
            if (cpuEl) cpuEl.textContent = metrics.cpu.toFixed(1) + '%';
            if (cpuBar) {
                cpuBar.style.width = metrics.cpu + '%';
                cpuBar.className = 'monitor-bar-fill' + 
                    (metrics.cpu > 90 ? ' critical' : metrics.cpu > 70 ? ' warning' : '');
            }
        }

        // Memory
        if (metrics.memory !== undefined) {
            const memEl = document.getElementById('realtime-memory');
            const memBar = document.getElementById('realtime-memory-bar');
            if (memEl) memEl.textContent = metrics.memory.toFixed(1) + '%';
            if (memBar) {
                memBar.style.width = metrics.memory + '%';
                memBar.className = 'monitor-bar-fill' + 
                    (metrics.memory > 90 ? ' critical' : metrics.memory > 70 ? ' warning' : '');
            }
        }

        // Temperature
        if (metrics.temperature !== undefined) {
            const tempEl = document.getElementById('realtime-temp');
            const tempBar = document.getElementById('realtime-temp-bar');
            if (tempEl) tempEl.textContent = metrics.temperature + '°C';
            if (tempBar) {
                // 温度条：0-100°C 映射
                const percent = Math.min(metrics.temperature, 100);
                tempBar.style.width = percent + '%';
            }
        }

        // Network
        if (metrics.network) {
            const netEl = document.getElementById('realtime-network');
            const rxEl = document.getElementById('realtime-rx');
            const txEl = document.getElementById('realtime-tx');
            
            const total = (metrics.network.rx || 0) + (metrics.network.tx || 0);
            if (netEl) netEl.textContent = (total * 8 / 1000).toFixed(1) + ' Mbps';
            if (rxEl) rxEl.textContent = (metrics.network.rx || 0).toFixed(1);
            if (txEl) txEl.textContent = (metrics.network.tx || 0).toFixed(1);
        }
    },

    // 开始告警检查
    startAlertCheck() {
        this.state.alertTimer = setInterval(() => {
            this.checkStorageAlerts();
        }, this.config.alertCheckInterval);
    },

    // 检查存储告警条件
    checkStorageAlerts() {
        this.state.pools.forEach(pool => {
            const percent = pool.total > 0 ? (pool.used / pool.total * 100) : 0;
            
            if (percent > 90) {
                this.addAlert({
                    id: `pool-full-${pool.name}`,
                    level: 'critical',
                    message: `存储池 ${pool.name} 使用率超过 90%，请及时扩容`,
                    timestamp: new Date().toISOString()
                });
            } else if (percent > 80) {
                this.addAlert({
                    id: `pool-warning-${pool.name}`,
                    level: 'warning',
                    message: `存储池 ${pool.name} 使用率超过 80%`,
                    timestamp: new Date().toISOString()
                });
            }
        });
    },

    addAlert(alert) {
        // 避免重复告警
        if (!this.state.alerts.find(a => a.id === alert.id)) {
            this.state.alerts.unshift(alert);
            this.renderAlerts(this.state.alerts);
        }
    },

    // 事件绑定
    bindEvents() {
        // 刷新存储池
        const refreshPoolsBtn = document.getElementById('refresh-pools-btn');
        if (refreshPoolsBtn) {
            refreshPoolsBtn.addEventListener('click', () => {
                this.loadStoragePools();
                this.loadStorageHealth();
            });
        }

        // 成本周期选择
        const costPeriodSelect = document.getElementById('cost-period-select');
        if (costPeriodSelect) {
            costPeriodSelect.addEventListener('change', () => {
                this.loadCostAnalysis();
            });
        }
    },

    // 工具函数
    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    },

    generateTimeLabels(count) {
        const labels = [];
        for (let i = count - 1; i >= 0; i--) {
            const time = new Date(Date.now() - i * 3600000);
            labels.push(time.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }));
        }
        return labels;
    },

    // 模拟数据
    getMockHealthData() {
        return {
            disks: [
                { device: '/dev/sda', temperature: 42, healthPercent: 100, model: 'WDC WD40EFRX', size: 4e12, health: 'healthy' },
                { device: '/dev/sdb', temperature: 45, healthPercent: 98, model: 'WDC WD40EFRX', size: 4e12, health: 'healthy' },
                { device: '/dev/sdc', temperature: 51, healthPercent: 92, model: 'ST4000VN008', size: 4e12, health: 'warning' },
                { device: '/dev/sdd', temperature: 38, healthPercent: 100, model: 'Samsung 870 EVO', size: 1e12, health: 'healthy' }
            ]
        };
    },

    getMockPoolsData() {
        return [
            { name: 'main-pool', status: 'online', used: 8e12, total: 12e12, diskCount: 4 },
            { name: 'backup-pool', status: 'online', used: 2e12, total: 4e12, diskCount: 2 }
        ];
    },

    getMockAlerts() {
        return [
            { id: '1', level: 'warning', message: '存储池 main-pool 使用率超过 80%', timestamp: new Date(Date.now() - 3600000).toISOString() },
            { id: '2', level: 'info', message: '快照备份已完成', timestamp: new Date(Date.now() - 7200000).toISOString() }
        ];
    },

    getMockCapacityTrend() {
        const labels = this.generateTimeLabels(24);
        const used = Array.from({ length: 24 }, (_, i) => 8 + Math.random() * 0.5);
        const free = Array.from({ length: 24 }, (_, i) => 4 - Math.random() * 0.5);
        return { labels, used, free };
    },

    getMockCostData() {
        return {
            labels: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
            storageCosts: [12.5, 13.2, 12.8, 14.1, 13.5, 11.8, 12.0],
            trafficCosts: [3.2, 4.1, 2.8, 5.2, 3.9, 2.1, 1.8],
            totalStorageCost: 89.9,
            totalTrafficCost: 23.1
        };
    },

    // 清理
    destroy() {
        if (this.state.alertTimer) {
            clearInterval(this.state.alertTimer);
        }
        Object.values(this.state.charts).forEach(chart => {
            if (chart) chart.destroy();
        });
    }
};

// 导出全局对象
window.StorageDashboard = StorageDashboard;

// 页面加载后初始化
document.addEventListener('DOMContentLoaded', () => {
    // 等待主 Dashboard 初始化完成后再初始化
    setTimeout(() => {
        StorageDashboard.init();
    }, 100);
});

// 清理
window.addEventListener('beforeunload', () => {
    StorageDashboard.destroy();
});