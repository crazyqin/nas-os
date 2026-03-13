/**
 * NAS-OS 配额使用图表组件
 * Version: 2.1.0
 * 提供配额使用可视化功能
 */

class QuotaChart {
    constructor(containerId, options = {}) {
        this.container = document.getElementById(containerId);
        if (!this.container) {
            console.error(`容器 #${containerId} 不存在`);
            return;
        }
        this.options = {
            apiBase: options.apiBase || '/api/v1',
            refreshInterval: options.refreshInterval || 30000,
            showLegend: options.showLegend !== false,
            showTitle: options.showTitle !== false,
            height: options.height || 300,
            ...options
        };
        this.data = null;
        this.init();
    }

    init() {
        this.container.innerHTML = `
            <div class="quota-chart-wrapper" style="min-height: ${this.options.height}px;">
                <div class="quota-chart-header">
                    <h3 class="quota-chart-title" style="display: ${this.options.showTitle ? 'flex' : 'none'};">
                        <svg width="20" height="20" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"/>
                        </svg>
                        存储配额概览
                    </h3>
                    <button class="quota-chart-refresh" onclick="this.closest('.quota-chart-wrapper').__chart__.refresh()">
                        <svg width="16" height="16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                        </svg>
                    </button>
                </div>
                <div class="quota-chart-content">
                    <div class="quota-chart-loading">
                        <div class="spinner"></div>
                        <span>加载中...</span>
                    </div>
                </div>
                <div class="quota-chart-legend" style="display: ${this.options.showLegend ? 'flex' : 'none'};"></div>
            </div>
        `;

        // 注入样式
        this.injectStyles();

        // 保存引用
        this.container.querySelector('.quota-chart-wrapper').__chart__ = this;

        // 加载数据
        this.load();
    }

    injectStyles() {
        if (document.getElementById('quota-chart-styles')) return;

        const styles = document.createElement('style');
        styles.id = 'quota-chart-styles';
        styles.textContent = `
            .quota-chart-wrapper {
                background: var(--surface-color, #fff);
                border-radius: var(--radius-lg, 12px);
                padding: 1.5rem;
                box-shadow: var(--shadow-md, 0 4px 6px rgba(0,0,0,0.07));
                border: 1px solid var(--surface-border, #E5E7EB);
            }
            .quota-chart-header {
                display: flex;
                justify-content: space-between;
                align-items: center;
                margin-bottom: 1rem;
            }
            .quota-chart-title {
                font-size: 16px;
                font-weight: 600;
                margin: 0;
                display: flex;
                align-items: center;
                gap: 0.5rem;
                color: var(--text-primary, #111827);
            }
            .quota-chart-refresh {
                background: var(--bg-tertiary, #F3F4F6);
                border: 1px solid var(--surface-border, #E5E7EB);
                border-radius: 6px;
                padding: 0.5rem;
                cursor: pointer;
                transition: all 0.2s;
                color: var(--text-secondary, #4B5563);
            }
            .quota-chart-refresh:hover {
                background: var(--color-primary-light, #DBEAFE);
                color: var(--color-primary, #2563EB);
            }
            .quota-chart-content {
                display: flex;
                gap: 1.5rem;
                flex-wrap: wrap;
            }
            .quota-chart-loading {
                display: flex;
                align-items: center;
                justify-content: center;
                gap: 0.75rem;
                color: var(--text-muted, #6B7280);
                padding: 2rem;
                width: 100%;
            }
            .quota-chart-pie {
                flex: 0 0 180px;
                display: flex;
                flex-direction: column;
                align-items: center;
            }
            .quota-pie-chart {
                width: 150px;
                height: 150px;
                border-radius: 50%;
                position: relative;
                background: var(--bg-tertiary, #F3F4F6);
            }
            .quota-pie-center {
                position: absolute;
                top: 50%;
                left: 50%;
                transform: translate(-50%, -50%);
                width: 80px;
                height: 80px;
                background: var(--surface-color, #fff);
                border-radius: 50%;
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
            }
            .quota-pie-value {
                font-size: 20px;
                font-weight: 700;
                color: var(--text-primary, #111827);
            }
            .quota-pie-label {
                font-size: 11px;
                color: var(--text-muted, #6B7280);
            }
            .quota-chart-stats {
                flex: 1;
                min-width: 200px;
            }
            .quota-stat-row {
                display: flex;
                justify-content: space-between;
                padding: 0.75rem 0;
                border-bottom: 1px solid var(--surface-border, #E5E7EB);
            }
            .quota-stat-row:last-child {
                border-bottom: none;
            }
            .quota-stat-label {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                color: var(--text-secondary, #4B5563);
            }
            .quota-stat-dot {
                width: 10px;
                height: 10px;
                border-radius: 2px;
            }
            .quota-stat-value {
                font-weight: 600;
                color: var(--text-primary, #111827);
            }
            .quota-chart-legend {
                display: flex;
                flex-wrap: wrap;
                gap: 1rem;
                margin-top: 1rem;
                padding-top: 1rem;
                border-top: 1px solid var(--surface-border, #E5E7EB);
            }
            .quota-legend-item {
                display: flex;
                align-items: center;
                gap: 0.5rem;
                font-size: 13px;
                color: var(--text-secondary, #4B5563);
            }
            .quota-legend-dot {
                width: 12px;
                height: 12px;
                border-radius: 3px;
            }
            .quota-chart-error {
                text-align: center;
                padding: 2rem;
                color: var(--color-error, #EF4444);
            }
            .quota-bar-chart {
                width: 100%;
            }
            .quota-bar-item {
                margin-bottom: 1rem;
            }
            .quota-bar-header {
                display: flex;
                justify-content: space-between;
                margin-bottom: 0.25rem;
                font-size: 13px;
            }
            .quota-bar-name {
                color: var(--text-primary, #111827);
                font-weight: 500;
            }
            .quota-bar-value {
                color: var(--text-muted, #6B7280);
            }
            .quota-bar-track {
                height: 8px;
                background: var(--bg-tertiary, #F3F4F6);
                border-radius: 4px;
                overflow: hidden;
            }
            .quota-bar-fill {
                height: 100%;
                border-radius: 4px;
                transition: width 0.3s ease;
            }
        `;
        document.head.appendChild(styles);
    }

    async load() {
        try {
            const response = await fetch(`${this.options.apiBase}/quota-usage/summary`);
            const result = await response.json();

            if (result.code === 0 && result.data) {
                this.data = result.data;
                this.render();
            } else {
                this.showError(result.message || '加载失败');
            }
        } catch (error) {
            // 尝试使用聚合数据
            try {
                await this.loadFromEndpoints();
            } catch (e) {
                this.showError('无法加载配额数据');
            }
        }
    }

    async loadFromEndpoints() {
        const endpoints = [
            { key: 'directories', url: `${this.options.apiBase}/quota-usage/directories` },
            { key: 'users', url: `${this.options.apiBase}/quota-usage/users` },
            { key: 'groups', url: `${this.options.apiBase}/quota-usage/groups` }
        ];

        const results = await Promise.all(
            endpoints.map(async ({ key, url }) => {
                try {
                    const res = await fetch(url);
                    const data = await res.json();
                    return { key, items: data.data || [] };
                } catch {
                    return { key, items: [] };
                }
            })
        );

        this.data = {
            directories: results.find(r => r.key === 'directories')?.items || [],
            users: results.find(r => r.key === 'users')?.items || [],
            groups: results.find(r => r.key === 'groups')?.items || []
        };

        this.render();
    }

    render() {
        const content = this.container.querySelector('.quota-chart-content');
        if (!this.data) return;

        const allItems = [
            ...(this.data.directories || []),
            ...(this.data.users || []),
            ...(this.data.groups || [])
        ];

        if (allItems.length === 0) {
            content.innerHTML = `
                <div class="quota-chart-loading">
                    <span>暂无配额数据</span>
                </div>
            `;
            return;
        }

        // 计算统计数据
        const totalLimit = allItems.reduce((sum, q) => sum + (q.hard_limit || 0), 0);
        const totalUsed = allItems.reduce((sum, q) => sum + (q.used_bytes || 0), 0);
        const avgUsage = allItems.reduce((sum, q) => sum + (q.usage_percent || 0), 0) / allItems.length;

        const dirCount = (this.data.directories || []).length;
        const userCount = (this.data.users || []).length;
        const groupCount = (this.data.groups || []).length;
        const totalCount = dirCount + userCount + groupCount;

        // 渲染饼图和统计
        content.innerHTML = `
            <div class="quota-chart-pie">
                <div class="quota-pie-chart" style="background: conic-gradient(
                    #3B82F6 0deg ${(dirCount / totalCount) * 360}deg,
                    #10B981 ${(dirCount / totalCount) * 360}deg ${((dirCount + userCount) / totalCount) * 360}deg,
                    #8B5CF6 ${((dirCount + userCount) / totalCount) * 360}deg 360deg
                );">
                    <div class="quota-pie-center">
                        <div class="quota-pie-value">${totalCount}</div>
                        <div class="quota-pie-label">配额数</div>
                    </div>
                </div>
            </div>
            <div class="quota-chart-stats">
                <div class="quota-stat-row">
                    <div class="quota-stat-label">
                        <div class="quota-stat-dot" style="background: #3B82F6;"></div>
                        目录配额
                    </div>
                    <div class="quota-stat-value">${dirCount}</div>
                </div>
                <div class="quota-stat-row">
                    <div class="quota-stat-label">
                        <div class="quota-stat-dot" style="background: #10B981;"></div>
                        用户配额
                    </div>
                    <div class="quota-stat-value">${userCount}</div>
                </div>
                <div class="quota-stat-row">
                    <div class="quota-stat-label">
                        <div class="quota-stat-dot" style="background: #8B5CF6;"></div>
                        组配额
                    </div>
                    <div class="quota-stat-value">${groupCount}</div>
                </div>
                <div class="quota-stat-row">
                    <div class="quota-stat-label">总限制容量</div>
                    <div class="quota-stat-value">${this.formatBytes(totalLimit)}</div>
                </div>
                <div class="quota-stat-row">
                    <div class="quota-stat-label">已使用容量</div>
                    <div class="quota-stat-value">${this.formatBytes(totalUsed)}</div>
                </div>
                <div class="quota-stat-row">
                    <div class="quota-stat-label">平均使用率</div>
                    <div class="quota-stat-value">${avgUsage.toFixed(1)}%</div>
                </div>
            </div>
        `;

        // 渲染图例
        const legend = this.container.querySelector('.quota-chart-legend');
        if (legend) {
            legend.innerHTML = `
                <div class="quota-legend-item">
                    <div class="quota-legend-dot" style="background: #3B82F6;"></div>
                    目录配额
                </div>
                <div class="quota-legend-item">
                    <div class="quota-legend-dot" style="background: #10B981;"></div>
                    用户配额
                </div>
                <div class="quota-legend-item">
                    <div class="quota-legend-dot" style="background: #8B5CF6;"></div>
                    组配额
                </div>
            `;
        }
    }

    showError(message) {
        const content = this.container.querySelector('.quota-chart-content');
        content.innerHTML = `
            <div class="quota-chart-error">
                <svg width="32" height="32" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/>
                </svg>
                <p>${message}</p>
            </div>
        `;
    }

    formatBytes(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    refresh() {
        this.container.querySelector('.quota-chart-content').innerHTML = `
            <div class="quota-chart-loading">
                <div class="spinner"></div>
                <span>刷新中...</span>
            </div>
        `;
        this.load();
    }

    destroy() {
        if (this.refreshTimer) {
            clearInterval(this.refreshTimer);
        }
        this.container.innerHTML = '';
    }
}

// 配额条形图组件
class QuotaBarChart {
    constructor(containerId, options = {}) {
        this.container = document.getElementById(containerId);
        if (!this.container) return;

        this.options = {
            apiBase: options.apiBase || '/api/v1',
            limit: options.limit || 10,
            type: options.type || 'all', // 'all', 'directories', 'users', 'groups'
            ...options
        };

        this.init();
    }

    init() {
        this.container.innerHTML = '<div class="quota-bar-chart"><div class="quota-chart-loading">加载中...</div></div>';
        this.load();
    }

    async load() {
        try {
            let items = [];

            if (this.options.type === 'all' || this.options.type === 'directories') {
                const res = await fetch(`${this.options.apiBase}/quota-usage/directories`);
                const data = await res.json();
                items = items.concat((data.data || []).map(q => ({ ...q, _type: 'directory' })));
            }

            if (this.options.type === 'all' || this.options.type === 'users') {
                const res = await fetch(`${this.options.apiBase}/quota-usage/users`);
                const data = await res.json();
                items = items.concat((data.data || []).map(q => ({ ...q, _type: 'user' })));
            }

            if (this.options.type === 'all' || this.options.type === 'groups') {
                const res = await fetch(`${this.options.apiBase}/quota-usage/groups`);
                const data = await res.json();
                items = items.concat((data.data || []).map(q => ({ ...q, _type: 'group' })));
            }

            // 按使用率排序
            items.sort((a, b) => (b.usage_percent || 0) - (a.usage_percent || 0));
            items = items.slice(0, this.options.limit);

            this.render(items);
        } catch (error) {
            this.container.innerHTML = '<div class="quota-chart-error">加载失败</div>';
        }
    }

    render(items) {
        if (items.length === 0) {
            this.container.innerHTML = '<div class="quota-chart-loading">暂无数据</div>';
            return;
        }

        const html = `
            <div class="quota-bar-chart">
                ${items.map(item => {
                    const name = item._type === 'directory' ?
                        (item.path ? item.path.split('/').pop() : '未知') :
                        (item.name || item.target_id || '未知');
                    const usage = item.usage_percent || 0;
                    const colorClass = usage >= 95 ? '#DC2626' : usage >= 80 ? '#F59E0B' : '#10B981';

                    return `
                        <div class="quota-bar-item">
                            <div class="quota-bar-header">
                                <span class="quota-bar-name">${name}</span>
                                <span class="quota-bar-value">${this.formatBytes(item.used_bytes)} / ${this.formatBytes(item.hard_limit)}</span>
                            </div>
                            <div class="quota-bar-track">
                                <div class="quota-bar-fill" style="width: ${Math.min(usage, 100)}%; background: ${colorClass};"></div>
                            </div>
                        </div>
                    `;
                }).join('')}
            </div>
        `;

        this.container.innerHTML = html;
    }

    formatBytes(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }
}

// 导出到全局
window.QuotaChart = QuotaChart;
window.QuotaBarChart = QuotaBarChart;

// 如果支持 ES 模块，也导出
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { QuotaChart, QuotaBarChart };
}