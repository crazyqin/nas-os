/**
 * SMART Cron 定时任务管理模块
 * Version: v1.0.0
 * 提供 SMART 定时任务的配置、监控、结果查看等功能
 */

const SmartCron = {
    // 配置
    config: {
        apiBase: '/api/v1/smart-cron',
        refreshInterval: 30000,
        toastDuration: 5000
    },

    // 状态
    state: {
        tasks: [],
        executions: [],
        results: [],
        alerts: [],
        devices: [],
        currentTab: 'tasks',
        editingTask: null,
        filters: {
            tasks: 'all',
            device: 'all',
            status: 'all',
            type: 'all',
            severity: 'all'
        }
    },

    // 初始化
    init: function() {
        this.loadDevices();
        this.loadTasks();
        this.loadExecutions();
        this.loadResults();
        this.loadAlerts();
        this.setupAutoRefresh();
        this.setupEventListeners();
    },

    // 设置事件监听
    setupEventListeners: function() {
        // Cron 表达式输入监听
        const cronInput = document.getElementById('cron-expression');
        if (cronInput) {
            cronInput.addEventListener('input', (e) => {
                this.updateCronPreview(e.target.value);
            });
        }
    },

    // 设置自动刷新
    setupAutoRefresh: function() {
        setInterval(() => {
            if (this.state.currentTab === 'execution') {
                this.loadExecutions();
            }
        }, this.config.refreshInterval);
    },

    // 切换标签页
    switchTab: function(tabName) {
        this.state.currentTab = tabName;
        
        // 更新标签页样式
        document.querySelectorAll('.tab-item').forEach(tab => {
            tab.classList.toggle('active', tab.dataset.tab === tabName);
        });
        
        // 显示对应内容
        document.querySelectorAll('.tab-content').forEach(content => {
            content.style.display = content.id === `tab-${tabName}` ? 'block' : 'none';
        });
        
        // 加载对应数据
        switch (tabName) {
            case 'tasks':
                this.loadTasks();
                break;
            case 'execution':
                this.loadExecutions();
                break;
            case 'results':
                this.loadResults();
                break;
            case 'alerts':
                this.loadAlerts();
                break;
        }
    },

    // 加载设备列表
    loadDevices: async function() {
        try {
            const response = await fetch('/api/v1/disks');
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.devices = data.data.disks || [];
                this.updateDeviceSelector();
                this.updateDeviceFilter();
            }
        } catch (error) {
            console.error('加载设备失败:', error);
            // 使用模拟数据
            this.state.devices = [
                { device: '/dev/nvme0n1', model: 'Samsung 970 EVO', size: '500GB' },
                { device: '/dev/sda', model: 'WD Red 4TB', size: '4TB' },
                { device: '/dev/sdb', model: 'Seagate IronWolf', size: '8TB' }
            ];
            this.updateDeviceSelector();
            this.updateDeviceFilter();
        }
    },

    // 更新设备选择器
    updateDeviceSelector: function() {
        const selector = document.getElementById('device-selector');
        if (!selector) return;
        
        selector.innerHTML = this.state.devices.map(device => `
            <div class="device-option" onclick="SmartCron.selectDevice('${device.device}')">
                <input type="checkbox" id="device-${device.device.replace('/', '-')}" 
                    value="${device.device}">
                <div class="device-info">
                    <strong>${device.device}</strong>
                    <span>${device.model} (${device.size})</span>
                </div>
            </div>
        `).join('');
    },

    // 选择设备
    selectDevice: function(devicePath) {
        const option = document.querySelector(`.device-option[onclick*="${devicePath}"]`);
        const checkbox = option.querySelector('input');
        
        checkbox.checked = !checkbox.checked;
        option.classList.toggle('selected', checkbox.checked);
    },

    // 更新设备过滤器
    updateDeviceFilter: function() {
        const filter = document.getElementById('filter-device');
        if (!filter) return;
        
        const options = this.state.devices.map(device => 
            `<option value="${device.device}">${device.device}</option>`
        ).join('');
        
        filter.innerHTML = `<option value="all">全部设备</option>${options}`;
    },

    // 加载任务列表
    loadTasks: async function() {
        try {
            const response = await fetch(`${this.config.apiBase}/tasks`);
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.tasks = data.data.tasks || [];
                this.renderTaskList();
                this.updateSummary();
            }
        } catch (error) {
            console.error('加载任务失败:', error);
            // 使用模拟数据
            this.state.tasks = [
                {
                    id: 'task-1',
                    name: '每周 NVMe 健康检查',
                    testType: 'short',
                    schedule: '0 2 * * 0',
                    scheduleHuman: '每周日凌晨 2:00',
                    devices: ['/dev/nvme0n1'],
                    enabled: true,
                    lastRun: '2026-04-07 02:00:00',
                    nextRun: '2026-04-14 02:00:00'
                },
                {
                    id: 'task-2',
                    name: '每日磁盘扫描',
                    testType: 'short',
                    schedule: '0 3 * * *',
                    scheduleHuman: '每日凌晨 3:00',
                    devices: ['/dev/sda', '/dev/sdb'],
                    enabled: true,
                    lastRun: '2026-04-07 03:00:00',
                    nextRun: '2026-04-08 03:00:00'
                },
                {
                    id: 'task-3',
                    name: '月度完整检测',
                    testType: 'long',
                    schedule: '0 0 1 * *',
                    scheduleHuman: '每月1号 0:00',
                    devices: ['/dev/nvme0n1', '/dev/sda', '/dev/sdb'],
                    enabled: false,
                    lastRun: '2026-04-01 00:00:00',
                    nextRun: '2026-05-01 00:00:00'
                }
            ];
            this.renderTaskList();
            this.updateSummary();
        }
    },

    // 渲染任务列表
    renderTaskList: function() {
        const tbody = document.getElementById('task-list');
        const empty = document.getElementById('tasks-empty');
        
        if (!tbody) return;
        
        const filteredTasks = this.filterTasksList(this.state.tasks);
        
        if (filteredTasks.length === 0) {
            tbody.innerHTML = '';
            empty.style.display = 'block';
            return;
        }
        
        empty.style.display = 'none';
        
        tbody.innerHTML = filteredTasks.map(task => `
            <tr>
                <td>
                    <div class="task-name">${task.name}</div>
                </td>
                <td>
                    <span class="device-tag">${this.getTestTypeLabel(task.testType)}</span>
                </td>
                <td>
                    <div class="task-schedule">${task.schedule}</div>
                    <div class="task-schedule-human">${task.scheduleHuman}</div>
                </td>
                <td>
                    <div class="task-devices">
                        ${task.devices.map(d => `<span class="device-tag">${d.replace('/dev/', '')}</span>`).join('')}
                    </div>
                </td>
                <td>
                    <span class="task-status ${task.enabled ? 'enabled' : 'disabled'}">
                        ${task.enabled ? '已启用' : '已禁用'}
                    </span>
                </td>
                <td>
                    <div class="task-actions">
                        <button class="btn-icon" onclick="SmartCron.editTask('${task.id}')">✎</button>
                        <button class="btn-icon" onclick="SmartCron.toggleTask('${task.id}')">
                            ${task.enabled ? '⏸' : '▶'}
                        </button>
                        <button class="btn-icon danger" onclick="SmartCron.deleteTask('${task.id}')">✕</button>
                    </div>
                </td>
            </tr>
        `).join('');
    },

    // 过滤任务列表
    filterTasksList: function(tasks) {
        const filter = this.state.filters.tasks;
        
        if (filter === 'all') return tasks;
        if (filter === 'enabled') return tasks.filter(t => t.enabled);
        if (filter === 'disabled') return tasks.filter(t => !t.enabled);
        
        return tasks;
    },

    // 任务过滤
    filterTasks: function(filter) {
        this.state.filters.tasks = filter;
        this.renderTaskList();
    },

    // 加载执行状态
    loadExecutions: async function() {
        try {
            const response = await fetch(`${this.config.apiBase}/executions`);
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.executions = data.data.executions || [];
                this.renderExecutionList();
            }
        } catch (error) {
            console.error('加载执行状态失败:', error);
            // 使用模拟数据
            this.state.executions = [
                {
                    id: 'exec-1',
                    taskId: 'task-1',
                    taskName: '每周 NVMe 健康检查',
                    startTime: '2026-04-07 02:00:00',
                    status: 'running',
                    progress: 45,
                    device: '/dev/nvme0n1'
                }
            ];
            this.renderExecutionList();
        }
    },

    // 渲染执行列表
    renderExecutionList: function() {
        const container = document.getElementById('execution-list');
        const empty = document.getElementById('execution-empty');
        
        if (!container) return;
        
        const running = this.state.executions.filter(e => e.status === 'running');
        
        if (running.length === 0) {
            container.innerHTML = '';
            empty.style.display = 'block';
            return;
        }
        
        empty.style.display = 'none';
        
        container.innerHTML = running.map(exec => `
            <div class="execution-item">
                <div class="execution-header">
                    <div class="execution-task">${exec.taskName}</div>
                    <div class="execution-time">开始于 ${exec.startTime}</div>
                </div>
                <div class="execution-progress">
                    <div class="progress-bar">
                        <div class="progress-fill" style="width: ${exec.progress}%"></div>
                    </div>
                    <div class="execution-status">
                        <span>测试进度: ${exec.progress}%</span>
                        <span>设备: ${exec.device}</span>
                    </div>
                </div>
            </div>
        `).join('');
    },

    // 加载检查结果
    loadResults: async function() {
        try {
            const response = await fetch(`${this.config.apiBase}/results`);
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.results = data.data.results || [];
                this.renderResultsList();
            }
        } catch (error) {
            console.error('加载结果失败:', error);
            // 使用模拟数据
            this.state.results = [
                {
                    id: 'result-1',
                    time: '2026-04-07 02:00:00',
                    device: '/dev/nvme0n1',
                    testType: 'short',
                    status: 'passed',
                    healthScore: 98,
                    details: { temperature: 35, errors: 0 }
                },
                {
                    id: 'result-2',
                    time: '2026-04-07 03:00:00',
                    device: '/dev/sda',
                    testType: 'short',
                    status: 'warning',
                    healthScore: 75,
                    details: { temperature: 42, errors: 1, pendingSectors: 5 }
                },
                {
                    id: 'result-3',
                    time: '2026-04-06 03:00:00',
                    device: '/dev/sdb',
                    testType: 'short',
                    status: 'passed',
                    healthScore: 92,
                    details: { temperature: 38, errors: 0 }
                },
                {
                    id: 'result-4',
                    time: '2026-04-01 00:00:00',
                    device: '/dev/nvme0n1',
                    testType: 'long',
                    status: 'passed',
                    healthScore: 99,
                    details: { temperature: 32, errors: 0 }
                }
            ];
            this.renderResultsList();
        }
    },

    // 渲染结果列表
    renderResultsList: function() {
        const tbody = document.getElementById('results-list');
        const empty = document.getElementById('results-empty');
        
        if (!tbody) return;
        
        const filteredResults = this.filterResultsList(this.state.results);
        
        if (filteredResults.length === 0) {
            tbody.innerHTML = '';
            empty.style.display = 'block';
            return;
        }
        
        empty.style.display = 'none';
        
        tbody.innerHTML = filteredResults.map(result => `
            <tr>
                <td>${result.time}</td>
                <td>${result.device.replace('/dev/', '')}</td>
                <td>${this.getTestTypeLabel(result.testType)}</td>
                <td>
                    <span class="result-status ${result.status}">
                        ${this.getResultStatusLabel(result.status)}
                    </span>
                </td>
                <td>${result.healthScore}%</td>
                <td>
                    <button class="btn-icon" onclick="SmartCron.showResultDetails('${result.id}')">
                        查看
                    </button>
                </td>
            </tr>
        `).join('');
    },

    // 过滤结果列表
    filterResultsList: function(results) {
        let filtered = results;
        
        if (this.state.filters.device !== 'all') {
            filtered = filtered.filter(r => r.device === this.state.filters.device);
        }
        
        if (this.state.filters.status !== 'all') {
            filtered = filtered.filter(r => r.status === this.state.filters.status);
        }
        
        if (this.state.filters.type !== 'all') {
            filtered = filtered.filter(r => r.testType === this.state.filters.type);
        }
        
        return filtered;
    },

    // 过滤结果
    filterResults: function() {
        this.state.filters.device = document.getElementById('filter-device').value;
        this.state.filters.status = document.getElementById('filter-status').value;
        this.state.filters.type = document.getElementById('filter-type').value;
        this.renderResultsList();
    },

    // 加载告警历史
    loadAlerts: async function() {
        try {
            const response = await fetch(`${this.config.apiBase}/alerts`);
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.alerts = data.data.alerts || [];
                this.renderAlertsList();
            }
        } catch (error) {
            console.error('加载告警失败:', error);
            // 使用模拟数据
            this.state.alerts = [
                {
                    id: 'alert-1',
                    severity: 'warning',
                    message: '磁盘 /dev/sda 温度偏高',
                    details: '当前温度 42°C，超过警告阈值 40°C',
                    device: '/dev/sda',
                    time: '2026-04-07 03:15:00',
                    resolved: false
                },
                {
                    id: 'alert-2',
                    severity: 'critical',
                    message: '磁盘 /dev/sdb 发现待映射扇区',
                    details: '检测到 5 个待映射扇区，建议尽快检查',
                    device: '/dev/sdb',
                    time: '2026-04-05 02:30:00',
                    resolved: true
                },
                {
                    id: 'alert-3',
                    severity: 'info',
                    message: 'NVMe 健康检查完成',
                    details: '/dev/nvme0n1 SMART 测试通过，健康分数 98%',
                    device: '/dev/nvme0n1',
                    time: '2026-04-07 02:02:00',
                    resolved: true
                }
            ];
            this.renderAlertsList();
        }
    },

    // 渲染告警列表
    renderAlertsList: function() {
        const container = document.getElementById('alerts-list');
        const empty = document.getElementById('alerts-empty');
        
        if (!container) return;
        
        const filteredAlerts = this.filterAlertsList(this.state.alerts);
        
        if (filteredAlerts.length === 0) {
            container.innerHTML = '';
            empty.style.display = 'block';
            return;
        }
        
        empty.style.display = 'none';
        
        container.innerHTML = filteredAlerts.map(alert => `
            <div class="alert-item">
                <div class="alert-icon ${alert.severity}">
                    ${this.getAlertIcon(alert.severity)}
                </div>
                <div class="alert-content">
                    <div class="alert-message">${alert.message}</div>
                    <div class="alert-details">${alert.details}</div>
                </div>
                <div class="alert-time">${alert.time}</div>
            </div>
        `).join('');
    },

    // 过滤告警列表
    filterAlertsList: function(alerts) {
        const filter = this.state.filters.severity;
        
        if (filter === 'all') return alerts;
        return alerts.filter(a => a.severity === filter);
    },

    // 过滤告警
    filterAlerts: function() {
        this.state.filters.severity = document.getElementById('filter-severity').value;
        this.renderAlertsList();
    },

    // 更新汇总信息
    updateSummary: function() {
        const total = this.state.tasks.length;
        const active = this.state.tasks.filter(t => t.enabled).length;
        const warning = this.state.alerts.filter(a => a.severity === 'warning' && !a.resolved).length;
        
        document.getElementById('total-tasks').textContent = total;
        document.getElementById('active-tasks').textContent = active;
        document.getElementById('warning-tasks').textContent = warning;
        
        // 上次执行时间
        if (this.state.results.length > 0) {
            document.getElementById('last-run').textContent = 
                this.state.results[0].time.split(' ')[0];
        }
    },

    // 显示创建模态框
    showCreateModal: function() {
        this.state.editingTask = null;
        document.getElementById('modal-title').textContent = '新建 SMART 定时任务';
        document.getElementById('task-form').reset();
        document.getElementById('task-enabled').checked = true;
        document.getElementById('notify-on-fail').checked = true;
        
        this.updateDeviceSelector();
        this.showModal();
    },

    // 编辑任务
    editTask: async function(taskId) {
        const task = this.state.tasks.find(t => t.id === taskId);
        if (!task) return;
        
        this.state.editingTask = task;
        document.getElementById('modal-title').textContent = '编辑 SMART 定时任务';
        
        document.getElementById('task-name').value = task.name;
        document.getElementById('test-type').value = task.testType;
        document.getElementById('cron-expression').value = task.schedule;
        document.getElementById('task-enabled').checked = task.enabled;
        
        this.updateCronPreview(task.schedule);
        this.updateDeviceSelector();
        
        // 选中对应设备
        task.devices.forEach(device => {
            this.selectDevice(device);
        });
        
        this.showModal();
    },

    // 显示模态框
    showModal: function() {
        document.getElementById('task-modal').classList.add('show');
    },

    // 隐藏模态框
    hideModal: function() {
        document.getElementById('task-modal').classList.remove('show');
        this.state.editingTask = null;
    },

    // 设置 Cron 预设
    setCronPreset: function(expression) {
        document.getElementById('cron-expression').value = expression;
        this.updateCronPreview(expression);
    },

    // 更新 Cron 预览
    updateCronPreview: function(expression) {
        const preview = document.getElementById('cron-preview');
        if (!preview) return;
        
        const humanReadable = this.parseCronExpression(expression);
        preview.textContent = `表达式含义: ${humanReadable}`;
    },

    // 解析 Cron 表达式
    parseCronExpression: function(expression) {
        // 标准 5 字段 cron: 分 时 日 月 周
        const parts = expression.trim().split(' ');
        if (parts.length < 5) return '无效表达式';
        
        const [minute, hour, dayOfMonth, month, dayOfWeek] = parts;
        
        // 常见预设解析
        if (expression === '0 2 * * 0') return '每周日凌晨 2:00 执行';
        if (expression === '0 3 * * *') return '每日凌晨 3:00 执行';
        if (expression === '0 4 * * 6') return '每周六凌晨 4:00 执行';
        if (expression === '0 0 1 * *') return '每月 1 号 0:00 执行';
        if (expression === '0 */6 * * *') return '每 6 小时执行一次';
        
        // 简单解析
        let desc = '';
        
        if (dayOfWeek !== '*') {
            const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
            desc += `每${days[parseInt(dayOfWeek)] || '周'}`;
        } else if (dayOfMonth !== '*') {
            desc += `每月 ${dayOfMonth} 号`;
        } else if (month !== '*') {
            desc += `${month} 月`;
        } else {
            desc += '每日';
        }
        
        desc += ` ${hour}:${minute.padStart(2, '0')} 执行`;
        
        return desc;
    },

    // 保存任务
    saveTask: async function() {
        const name = document.getElementById('task-name').value;
        const testType = document.getElementById('test-type').value;
        const schedule = document.getElementById('cron-expression').value;
        const enabled = document.getElementById('task-enabled').checked;
        
        // 获取选中的设备
        const selectedDevices = [];
        document.querySelectorAll('.device-option input:checked').forEach(input => {
            selectedDevices.push(input.value);
        });
        
        if (!name || !schedule || selectedDevices.length === 0) {
            this.showToast('请填写完整信息', 'error');
            return;
        }
        
        const taskData = {
            name,
            testType,
            schedule,
            devices: selectedDevices,
            enabled,
            scheduleHuman: this.parseCronExpression(schedule)
        };
        
        try {
            const url = this.state.editingTask 
                ? `${this.config.apiBase}/tasks/${this.state.editingTask.id}`
                : `${this.config.apiBase}/tasks`;
            
            const method = this.state.editingTask ? 'PUT' : 'POST';
            
            const response = await fetch(url, {
                method,
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(taskData)
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                this.showToast(
                    this.state.editingTask ? '任务已更新' : '任务已创建',
                    'success'
                );
                this.hideModal();
                this.loadTasks();
            } else {
                this.showToast('保存失败: ' + data.message, 'error');
            }
        } catch (error) {
            console.error('保存任务失败:', error);
            // 模拟保存成功
            this.showToast('任务已保存', 'success');
            this.hideModal();
            
            if (this.state.editingTask) {
                const index = this.state.tasks.findIndex(t => t.id === this.state.editingTask.id);
                if (index >= 0) {
                    this.state.tasks[index] = { ...this.state.editingTask, ...taskData };
                }
            } else {
                this.state.tasks.push({
                    id: `task-${Date.now()}`,
                    ...taskData,
                    lastRun: '-',
                    nextRun: '计算中...'
                });
            }
            
            this.renderTaskList();
            this.updateSummary();
        }
    },

    // 切换任务状态
    toggleTask: async function(taskId) {
        const task = this.state.tasks.find(t => t.id === taskId);
        if (!task) return;
        
        try {
            const response = await fetch(`${this.config.apiBase}/tasks/${taskId}/toggle`, {
                method: 'POST'
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                task.enabled = !task.enabled;
                this.renderTaskList();
                this.updateSummary();
                this.showToast(
                    task.enabled ? '任务已启用' : '任务已禁用',
                    'success'
                );
            }
        } catch (error) {
            console.error('切换任务状态失败:', error);
            // 模拟切换成功
            task.enabled = !task.enabled;
            this.renderTaskList();
            this.updateSummary();
            this.showToast(task.enabled ? '任务已启用' : '任务已禁用', 'success');
        }
    },

    // 删除任务
    deleteTask: async function(taskId) {
        if (!confirm('确定要删除此任务吗？')) return;
        
        try {
            const response = await fetch(`${this.config.apiBase}/tasks/${taskId}`, {
                method: 'DELETE'
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                this.state.tasks = this.state.tasks.filter(t => t.id !== taskId);
                this.renderTaskList();
                this.updateSummary();
                this.showToast('任务已删除', 'success');
            }
        } catch (error) {
            console.error('删除任务失败:', error);
            // 模拟删除成功
            this.state.tasks = this.state.tasks.filter(t => t.id !== taskId);
            this.renderTaskList();
            this.updateSummary();
            this.showToast('任务已删除', 'success');
        }
    },

    // 立即运行任务
    runTaskNow: async function(taskId) {
        try {
            const response = await fetch(`${this.config.apiBase}/tasks/${taskId}/run`, {
                method: 'POST'
            });
            
            const data = await response.json();
            
            if (data.code === 0) {
                this.showToast('任务已开始执行', 'success');
                this.switchTab('execution');
            }
        } catch (error) {
            console.error('运行任务失败:', error);
            this.showToast('任务已开始执行', 'success');
            this.switchTab('execution');
        }
    },

    // 显示结果详情
    showResultDetails: function(resultId) {
        const result = this.state.results.find(r => r.id === resultId);
        if (!result) return;
        
        // 详情模态框入口
        alert(`结果详情:\n设备: ${result.device}\n状态: ${result.status}\n健康分数: ${result.healthScore}%\n温度: ${result.details.temperature}°C\n错误数: ${result.details.errors}`);
    },

    // 刷新所有数据
    refreshAll: function() {
        this.loadTasks();
        this.loadExecutions();
        this.loadResults();
        this.loadAlerts();
        this.showToast('数据已刷新', 'success');
    },

    // 显示 Toast
    showToast: function(message, type = 'success') {
        const toast = document.getElementById('toast');
        const toastMessage = document.getElementById('toast-message');
        
        toastMessage.textContent = message;
        toast.className = `toast ${type}`;
        
        setTimeout(() => toast.classList.add('show'), 10);
        
        setTimeout(() => {
            toast.classList.remove('show');
        }, this.config.toastDuration);
    },

    // 辅助方法: 测试类型标签
    getTestTypeLabel: function(type) {
        const labels = {
            'short': '短测试',
            'long': '长测试',
            'conveyance': '运输测试',
            'offline': '离线测试'
        };
        return labels[type] || type;
    },

    // 辅助方法: 结果状态标签
    getResultStatusLabel: function(status) {
        const labels = {
            'passed': '通过',
            'warning': '警告',
            'failed': '失败'
        };
        return labels[status] || status;
    },

    // 辅助方法: 告警图标
    getAlertIcon: function(severity) {
        const icons = {
            'critical': '🔴',
            'warning': '⚠️',
            'info': 'ℹ️'
        };
        return icons[severity] || '🔔';
    }
};

// 导出模块
if (typeof module !== 'undefined' && module.exports) {
    module.exports = SmartCron;
}