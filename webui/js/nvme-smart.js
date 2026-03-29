/**
 * NVMe SMART 监控模块
 * Version: v1.0.0
 * 提供 NVMe 设备健康监控、SMART 测试、数据可视化等功能
 */

const NVMeSMART = {
    // 配置
    config: {
        apiBase: '/api/v1/nvme',
        refreshInterval: 30000, // 30秒刷新
        testPollInterval: 2000, // 测试进度轮询间隔
        maxHistoryPoints: 100,
        toastDuration: 5000
    },

    // 状态
    state: {
        devices: [],
        currentDevice: null,
        testInProgress: false,
        batchTestInProgress: false,
        refreshTimer: null,
        testPollTimer: null
    },

    // 初始化
    init: function() {
        this.loadDevices();
        this.setupAutoRefresh();
        this.setupEventListeners();
    },

    // 加载设备列表
    loadDevices: async function() {
        try {
            const response = await fetch(this.config.apiBase);
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.devices = data.data.devices || [];
                this.updateSummary(data.data.summary);
                this.renderDeviceCards();
                this.hideLoading();
            } else {
                this.showError('加载设备失败: ' + data.message);
            }
        } catch (error) {
            this.showError('网络错误: ' + error.message);
        }
    },

    // 更新汇总信息
    updateSummary: function(summary) {
        document.getElementById('total-devices').textContent = summary.total || 0;
        document.getElementById('healthy-devices').textContent = summary.healthy || 0;
        document.getElementById('warning-devices').textContent = summary.warning || 0;
        document.getElementById('critical-devices').textContent = summary.critical || 0;
    },

    // 渲染设备卡片
    renderDeviceCards: function() {
        const dashboard = document.getElementById('nvme-dashboard');
        dashboard.innerHTML = '';

        this.state.devices.forEach(device => {
            const card = this.createDeviceCard(device);
            dashboard.appendChild(card);
        });
    },

    // 创建设备卡片
    createDeviceCard: function(device) {
        const card = document.createElement('div');
        card.className = 'nvme-card';
        card.dataset.device = device.device;

        // 健康状态样式
        const healthClass = this.getHealthClass(device.status);
        const progressClass = this.getProgressClass(device.healthPercentage);

        // 计算健康环进度
        const circumference = 326.73; // 2 * π * 52
        const offset = circumference - (device.healthPercentage / 100) * circumference;

        card.innerHTML = `
            <div class="nvme-card-header">
                <div class="nvme-device-name">${device.device}</div>
                <div class="nvme-health-badge ${healthClass}">${this.getStatusLabel(device.status)}</div>
            </div>
            
            <div class="nvme-health-ring">
                <svg width="120" height="120">
                    <circle class="nvme-health-ring-bg" cx="60" cy="60" r="52"/>
                    <circle class="nvme-health-ring-progress ${progressClass}" cx="60" cy="60" r="52"
                        stroke-dasharray="${circumference}" 
                        stroke-dashoffset="${offset}"/>
                </svg>
                <div class="nvme-health-percentage">${device.healthPercentage}%</div>
            </div>

            <div class="nvme-metrics-grid">
                <div class="nvme-metric-item">
                    <span class="nvme-metric-label">温度</span>
                    <span class="nvme-metric-value">${device.temperature?.current || '-'}°C</span>
                </div>
                <div class="nvme-metric-item">
                    <span class="nvme-metric-label">写入量</span>
                    <span class="nvme-metric-value">${this.formatBytes(device.usage?.totalWrites || 0)}</span>
                </div>
                <div class="nvme-metric-item">
                    <span class="nvme-metric-label">使用率</span>
                    <span class="nvme-metric-value">${device.usage?.percentageUsed || 0}%</span>
                </div>
                <div class="nvme-metric-item">
                    <span class="nvme-metric-label">运行时间</span>
                    <span class="nvme-metric-value">${this.formatHours(device.powerOnHours || 0)}</span>
                </div>
            </div>

            <div class="nvme-temp-gauge">
                <div class="nvme-temp-bar">
                    <div class="nvme-temp-indicator" style="left: ${this.calcTempPosition(device.temperature?.current || 25)}%;"></div>
                </div>
                <div class="nvme-temp-labels">
                    <span>25°C</span>
                    <span>50°C</span>
                    <span>70°C</span>
                    <span>85°C</span>
                </div>
            </div>

            <div class="nvme-test-panel">
                <button class="nvme-test-btn" onclick="NVMeSMART.quickTest('${device.device}')">快速测试</button>
                <div class="nvme-test-progress" style="display: none;">
                    <div class="nvme-test-progress-bar">
                        <div class="nvme-test-progress-fill"></div>
                    </div>
                </div>
            </div>
        `;

        // 点击卡片显示详情
        card.addEventListener('click', (e) => {
            if (!e.target.classList.contains('nvme-test-btn')) {
                this.showDeviceDetail(device.device);
            }
        });

        return card;
    },

    // 获取健康状态样式类
    getHealthClass: function(status) {
        switch (status) {
            case 'healthy': return 'healthy';
            case 'warning': return 'warning';
            case 'critical': return 'critical';
            default: return 'unknown';
        }
    },

    // 获取进度条样式类
    getProgressClass: function(percentage) {
        if (percentage >= 80) return '';
        if (percentage >= 50) return 'warning';
        return 'critical';
    },

    // 获取状态标签
    getStatusLabel: function(status) {
        const labels = {
            'healthy': '健康',
            'warning': '警告',
            'critical': '严重',
            'unknown': '未知',
            'offline': '离线'
        };
        return labels[status] || '未知';
    },

    // 计算温度指示器位置
    calcTempPosition: function(temp) {
        // 温度范围: 25-85°C
        const minTemp = 25;
        const maxTemp = 85;
        const position = ((temp - minTemp) / (maxTemp - minTemp)) * 100;
        return Math.min(100, Math.max(0, position));
    },

    // 格式化字节
    formatBytes: function(bytes) {
        if (bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(2) + ' ' + units[i];
    },

    // 格式化小时
    formatHours: function(hours) {
        if (hours < 24) return hours + 'h';
        const days = Math.floor(hours / 24);
        const remainingHours = hours % 24;
        return days + 'd ' + remainingHours + 'h';
    },

    // 显示设备详情
    showDeviceDetail: async function(deviceName) {
        this.state.currentDevice = deviceName;
        
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName);
            const data = await response.json();
            
            if (data.code === 0) {
                this.renderDeviceDetail(data.data);
                document.getElementById('nvme-detail-panel').style.display = 'block';
            }
        } catch (error) {
            this.showError('加载设备详情失败');
        }
    },

    // 渲染设备详情
    renderDeviceDetail: function(device) {
        // 基本信息
        document.getElementById('detail-device-name').textContent = device.device;
        document.getElementById('info-model').textContent = device.model || '-';
        document.getElementById('info-serial').textContent = device.serial || '-';
        document.getElementById('info-firmware').textContent = device.firmware || '-';
        document.getElementById('info-size').textContent = this.formatBytes(device.size || 0);
        document.getElementById('info-controller').textContent = device.controller || '-';

        // 健康环
        const circumference = 326.73;
        const offset = circumference - (device.healthPercentage / 100) * circumference;
        const progressCircle = document.querySelector('#detail-health-ring .nvme-health-ring-progress');
        progressCircle.setAttribute('stroke-dashoffset', offset);
        progressCircle.className = 'nvme-health-ring-progress ' + this.getProgressClass(device.healthPercentage);
        document.getElementById('detail-health-percent').textContent = device.healthPercentage + '%';

        // 最近测试结果
        if (device.lastTestResult) {
            document.getElementById('last-test-type').textContent = device.lastTestResult.testType;
            document.getElementById('last-test-result-status').textContent = device.lastTestResult.result;
            document.getElementById('last-test-time').textContent = new Date(device.lastTestResult.startTime).toLocaleString();
        }
    },

    // 切换详情标签页
    switchDetailTab: async function(tabName) {
        // 更新标签样式
        document.querySelectorAll('.nvme-detail-tab').forEach(tab => {
            tab.classList.remove('active');
            if (tab.dataset.tab === tabName) {
                tab.classList.add('active');
            }
        });

        // 显示对应内容
        document.querySelectorAll('.nvme-detail-content').forEach(content => {
            content.style.display = 'none';
        });
        document.getElementById('tab-' + tabName).style.display = 'block';

        // 加载标签页数据
        switch (tabName) {
            case 'smart':
                await this.loadSMARTAttributes(this.state.currentDevice);
                break;
            case 'temperature':
                await this.loadTemperatureHistory(this.state.currentDevice);
                break;
            case 'usage':
                await this.loadUsageStats(this.state.currentDevice);
                break;
            case 'tests':
                await this.loadTestHistory(this.state.currentDevice);
                break;
        }
    },

    // 加载 SMART 属性
    loadSMARTAttributes: async function(deviceName) {
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName + '/smart');
            const data = await response.json();
            
            if (data.code === 0) {
                this.renderSMARTTable(data.data.attributes);
            }
        } catch (error) {
            console.error('加载 SMART 属性失败:', error);
        }
    },

    // 渲染 SMART 表格
    renderSMARTTable: function(attributes) {
        const tbody = document.getElementById('smart-attributes-table');
        tbody.innerHTML = '';

        attributes.forEach(attr => {
            const row = document.createElement('tr');
            const valueClass = attr.status === 'warning' ? 'warning' : 
                              attr.status === 'critical' ? 'critical' : '';

            row.innerHTML = `
                <td>${attr.name}</td>
                <td class="nvme-smart-value ${valueClass}">${attr.value}</td>
                <td>${attr.threshold || '-'}</td>
                <td>${attr.worst || '-'}</td>
                <td>${this.getStatusLabel(attr.status)}</td>
            `;
            tbody.appendChild(row);
        });
    },

    // 加载温度历史
    loadTemperatureHistory: async function(deviceName) {
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName + '/temperature');
            const data = await response.json();
            
            if (data.code === 0) {
                this.renderTemperatureChart(data.data);
            }
        } catch (error) {
            console.error('加载温度历史失败:', error);
        }
    },

    // 渲染温度图表（简化版，实际可用 Chart.js）
    renderTemperatureChart: function(data) {
        const chart = document.getElementById('temperature-chart');
        chart.innerHTML = `
            <div style="background: #f5f5f5; padding: 20px; text-align: center;">
                温度趋势图表<br>
                当前: ${data.current || '-'}°C | 最高: ${data.max || '-'}°C | 最低: ${data.min || '-'}°C
            </div>
        `;
        
        document.getElementById('temp-current').textContent = (data.current || '-') + '°C';
        document.getElementById('temp-max').textContent = (data.max || '-') + '°C';
        document.getElementById('temp-min').textContent = (data.min || '-') + '°C';
        document.getElementById('temp-events').textContent = data.overTempEvents || 0;
    },

    // 加载使用统计
    loadUsageStats: async function(deviceName) {
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName + '/usage');
            const data = await response.json();
            
            if (data.code === 0) {
                this.renderUsageStats(data.data);
            }
        } catch (error) {
            console.error('加载使用统计失败:', error);
        }
    },

    // 渲染使用统计
    renderUsageStats: function(data) {
        document.getElementById('total-writes').textContent = this.formatBytes(data.totalWrites || 0);
        document.getElementById('tbw-value').textContent = (data.tbw || 0) + ' TB';
        document.getElementById('usage-percent').textContent = (data.percentageUsed || 0) + '%';
        document.getElementById('life-estimate').textContent = data.estimatedLife || '计算中...';

        // 磨损度显示
        const wearFill = document.querySelector('.nvme-wear-fill');
        if (wearFill) {
            wearFill.style.width = (data.percentageUsed || 0) + '%';
        }
    },

    // 加载测试历史
    loadTestHistory: async function(deviceName) {
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName + '/test');
            const data = await response.json();
            
            if (data.code === 0) {
                this.renderTestHistory(data.data.history || []);
            }
        } catch (error) {
            console.error('加载测试历史失败:', error);
        }
    },

    // 渲染测试历史
    renderTestHistory: function(history) {
        const tbody = document.getElementById('test-history-table');
        tbody.innerHTML = '';

        history.forEach(test => {
            const row = document.createElement('tr');
            row.innerHTML = `
                <td>${new Date(test.startTime).toLocaleString()}</td>
                <td>${test.testType}</td>
                <td>${test.result}</td>
                <td>${this.formatDuration(test.duration)}</td>
                <td>${test.numErrors || 0}</td>
            `;
            tbody.appendChild(row);
        });
    },

    // 格式化时长
    formatDuration: function(ms) {
        if (!ms) return '-';
        const seconds = Math.floor(ms / 1000);
        if (seconds < 60) return seconds + 's';
        const minutes = Math.floor(seconds / 60);
        const remainingSeconds = seconds % 60;
        return minutes + 'm ' + remainingSeconds + 's';
    },

    // 运行设备测试
    runDeviceTest: async function(testType) {
        if (this.state.testInProgress) {
            this.showToast('已有测试进行中', 'warning');
            return;
        }

        const device = this.state.currentDevice;
        if (!device) {
            this.showToast('请先选择设备', 'error');
            return;
        }

        this.state.testInProgress = true;

        try {
            const response = await fetch(this.config.apiBase + '/' + device + '/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ testType: testType })
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                this.showToast('测试已启动', 'success');
                this.pollTestProgress(device);
            } else {
                this.showError('启动测试失败: ' + data.message);
                this.state.testInProgress = false;
            }
        } catch (error) {
            this.showError('网络错误: ' + error.message);
            this.state.testInProgress = false;
        }
    },

    // 快速测试（从卡片按钮触发）
    quickTest: async function(deviceName) {
        this.state.currentDevice = deviceName;
        await this.runDeviceTest('short');
    },

    // 轮询测试进度
    pollTestProgress: function(device) {
        const progressBar = document.querySelector('#device-test-progress');
        const progressFill = document.querySelector('#device-test-progress .nvme-test-progress-fill');
        const progressPanel = document.getElementById('device-test-progress');

        if (progressPanel) {
            progressPanel.style.display = 'block';
        }

        this.state.testPollTimer = setInterval(async () => {
            try {
                const response = await fetch(this.config.apiBase + '/' + device + '/test');
                const data = await response.json();
                
                if (data.code === 0) {
                    const test = data.data.current || data.data.last;
                    
                    if (test && test.status === 'running') {
                        if (progressFill) {
                            progressFill.style.width = test.progress + '%';
                        }
                    } else {
                        // 测试完成
                        clearInterval(this.state.testPollTimer);
                        this.state.testInProgress = false;
                        
                        if (progressPanel) {
                            progressPanel.style.display = 'none';
                        }

                        if (test && test.result === 'pass') {
                            this.showToast('测试通过', 'success');
                        } else if (test) {
                            this.showToast('测试完成: ' + test.result, test.result === 'fail' ? 'error' : 'warning');
                        }

                        // 刷新设备状态
                        this.refreshDevice(device);
                    }
                }
            } catch (error) {
                console.error('轮询测试进度失败:', error);
            }
        }, this.config.testPollInterval);
    },

    // 刷新单个设备
    refreshDevice: async function(deviceName) {
        try {
            const response = await fetch(this.config.apiBase + '/' + deviceName + '/refresh', {
                method: 'POST'
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                // 更新设备卡片
                const card = document.querySelector(`.nvme-card[data-device="${deviceName}"]`);
                if (card) {
                    const newCard = this.createDeviceCard(data.data);
                    card.replaceWith(newCard);
                }

                // 如果正在查看详情，更新详情
                if (this.state.currentDevice === deviceName) {
                    this.renderDeviceDetail(data.data);
                }
            }
        } catch (error) {
            console.error('刷新设备失败:', error);
        }
    },

    // 批量测试
    runAllTests: async function() {
        if (this.state.devices.length === 0) {
            this.showToast('没有可用设备', 'warning');
            return;
        }

        if (this.state.batchTestInProgress) {
            this.showToast('批量测试已进行中', 'warning');
            return;
        }

        this.state.batchTestInProgress = true;
        document.getElementById('batch-test-panel').style.display = 'block';

        try {
            const response = await fetch(this.config.apiBase + '/test-all', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ testType: 'short' })
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                this.showToast('批量测试已启动', 'success');
                this.pollBatchTestProgress();
            } else {
                this.showError('启动批量测试失败: ' + data.message);
                this.state.batchTestInProgress = false;
                document.getElementById('batch-test-panel').style.display = 'none';
            }
        } catch (error) {
            this.showError('网络错误: ' + error.message);
            this.state.batchTestInProgress = false;
            document.getElementById('batch-test-panel').style.display = 'none';
        }
    },

    // 轮询批量测试进度
    pollBatchTestProgress: function() {
        this.state.testPollTimer = setInterval(async () => {
            try {
                const response = await fetch(this.config.apiBase + '/summary');
                const data = await response.json();
                
                if (data.code === 0 && data.data.batchTest) {
                    const batch = data.data.batchTest;
                    
                    document.querySelector('.nvme-batch-test-panel .nvme-test-progress-fill').style.width = batch.progress + '%';
                    document.getElementById('batch-test-current').textContent = batch.currentDevice || '-';
                    document.getElementById('batch-test-percent').textContent = batch.progress + '%';

                    if (batch.status !== 'running') {
                        clearInterval(this.state.testPollTimer);
                        this.state.batchTestInProgress = false;
                        document.getElementById('batch-test-panel').style.display = 'none';
                        
                        if (batch.status === 'completed') {
                            this.showToast('批量测试完成', 'success');
                        } else {
                            this.showToast('批量测试已停止', 'warning');
                        }

                        this.loadDevices();
                    }
                }
            } catch (error) {
                console.error('轮询批量测试进度失败:', error);
            }
        }, this.config.testPollInterval);
    },

    // 暂停批量测试
    pauseBatchTest: async function() {
        try {
            await fetch(this.config.apiBase + '/test-all', { method: 'DELETE' });
            this.showToast('批量测试已暂停', 'warning');
        } catch (error) {
            this.showError('暂停失败');
        }
    },

    // 取消批量测试
    cancelBatchTest: async function() {
        try {
            await fetch(this.config.apiBase + '/test-all', { method: 'DELETE' });
            clearInterval(this.state.testPollTimer);
            this.state.batchTestInProgress = false;
            document.getElementById('batch-test-panel').style.display = 'none';
            this.showToast('批量测试已取消', 'warning');
        } catch (error) {
            this.showError('取消失败');
        }
    },

    // 刷新所有设备
    refreshAllDevices: function() {
        this.loadDevices();
        this.showToast('数据已刷新', 'success');
    },

    // 显示测试历史弹窗
    showTestHistory: function() {
        this.showToast('测试历史功能开发中...', 'info');
    },

    // 关闭详情面板
    closeDetailPanel: function() {
        document.getElementById('nvme-detail-panel').style.display = 'none';
        this.state.currentDevice = null;
    },

    // 隐藏加载状态
    hideLoading: function() {
        const loading = document.querySelector('.nvme-loading');
        if (loading) {
            loading.style.display = 'none';
        }
    },

    // 设置自动刷新
    setupAutoRefresh: function() {
        this.state.refreshTimer = setInterval(() => {
            this.loadDevices();
        }, this.config.refreshInterval);
    },

    // 设置事件监听
    setupEventListeners: function() {
        // 键盘快捷键
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                this.closeDetailPanel();
            }
            if (e.key === 'r' && e.ctrlKey) {
                e.preventDefault();
                this.refreshAllDevices();
            }
        });
    },

    // 显示 Toast 通知
    showToast: function(message, type = 'info') {
        const toast = document.getElementById('nvme-toast');
        const toastMessage = toast.querySelector('.nvme-toast-message');
        
        toastMessage.textContent = message;
        toast.className = 'nvme-toast show ' + type;
        
        setTimeout(() => {
            toast.className = 'nvme-toast';
        }, this.config.toastDuration);
    },

    // 显示错误
    showError: function(message) {
        this.showToast(message, 'error');
        console.error('[NVMeSMART]', message);
    },

    // 清理资源
    destroy: function() {
        if (this.state.refreshTimer) {
            clearInterval(this.state.refreshTimer);
        }
        if (this.state.testPollTimer) {
            clearInterval(this.state.testPollTimer);
        }
    }
};

// 全局函数绑定（供 HTML onclick 使用）
window.refreshAllDevices = NVMeSMART.refreshAllDevices.bind(NVMeSMART);
window.runAllTests = NVMeSMART.runAllTests.bind(NVMeSMART);
window.showTestHistory = NVMeSMART.showTestHistory.bind(NVMeSMART);
window.runDeviceTest = NVMeSMART.runDeviceTest.bind(NVMeSMART);
window.switchDetailTab = NVMeSMART.switchDetailTab.bind(NVMeSMART);
window.closeDetailPanel = NVMeSMART.closeDetailPanel.bind(NVMeSMART);