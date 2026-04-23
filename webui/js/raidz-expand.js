/**
 * RAIDZ 扩容前端组件
 * 兵部 Round 230 - RAIDZ Expansion WebUI
 * 对标 TrueNAS 25.10 RAIDZ Expansion UI
 */

// ============================================
// 全局配置
// ============================================
const RAIDZ_CONFIG = {
    API_BASE: '/api/v1',
    WS_URL: `${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/raidz-expansion/ws`,
    REFRESH_INTERVAL: 3000,
    RECONNECT_INTERVAL: 5000
};

// ============================================
// 状态管理
// ============================================
const RAIDZState = {
    // 当前活跃任务
    activeProgress: null,
    
    // 可用磁盘
    availableDisks: [],
    
    // 可扩展池
    expandablePools: [],
    
    // 历史记录
    history: [],
    
    // 摘要统计
    summary: {
        activeCount: 0,
        completedCount: 0,
        diskCount: 0,
        supported: false
    },
    
    // 向导状态
    wizard: {
        currentStep: 1,
        selectedPool: null,
        selectedDisk: null,
        validationResult: null
    },
    
    // WebSocket 连接
    ws: null,
    wsConnected: false,
    
    // 定时刷新
    refreshTimer: null
};

// ============================================
// API 调用封装
// ============================================
const RAIDZAPI = {
    // 获取Dashboard摘要
    async getDashboard() {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/dashboard`);
        const data = await resp.json();
        return data.data;
    },
    
    // 获取全局状态
    async getGlobalStatus() {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/status`);
        const data = await resp.json();
        return data.data;
    },
    
    // 获取所有进度
    async getAllProgress() {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/progress`);
        const data = await resp.json();
        return data.data;
    },
    
    // 获取指定池进度
    async getPoolProgress(poolName) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/progress/${encodeURIComponent(poolName)}`);
        const data = await resp.json();
        return data.data;
    },
    
    // 获取历史记录
    async getHistory(limit = 20) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/history?limit=${limit}`);
        const data = await resp.json();
        return data.data;
    },
    
    // 获取可用磁盘
    async getAvailableDisks() {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/available-disks`);
        const data = await resp.json();
        return data.data;
    },
    
    // 检查池扩展资格
    async checkEligibility(poolName) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/eligibility/${encodeURIComponent(poolName)}`);
        const data = await resp.json();
        return data.data;
    },
    
    // 验证扩展
    async validateExpansion(poolName, diskPath) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/validate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ poolName, newDisk: diskPath })
        });
        const data = await resp.json();
        return data.data;
    },
    
    // 估算扩展
    async estimateExpansion(poolName, raidzLevel, currentWidth, diskSizeGB, usedBytes) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/estimate`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                poolName,
                raidzLevel,
                currentWidth,
                newDiskSizeGB: diskSizeGB,
                usedBytes
            })
        });
        const data = await resp.json();
        return data.data;
    },
    
    // 启动扩展
    async startExpansion(poolName, diskPath, force = false, dryRun = false) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/start`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                poolName,
                newDisk: diskPath,
                force,
                dryRun
            })
        });
        const data = await resp.json();
        return data;
    },
    
    // 暂停扩展
    async pauseExpansion(poolName) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/pause/${encodeURIComponent(poolName)}`, {
            method: 'POST'
        });
        const data = await resp.json();
        return data;
    },
    
    // 恢复扩展
    async resumeExpansion(poolName) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/resume/${encodeURIComponent(poolName)}`, {
            method: 'POST'
        });
        const data = await resp.json();
        return data;
    },
    
    // 取消扩展
    async cancelExpansion(poolName) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/cancel/${encodeURIComponent(poolName)}`, {
            method: 'POST'
        });
        const data = await resp.json();
        return data;
    },
    
    // 容量计算
    async calculateCapacity(raidzLevel, width, diskSizeGB) {
        const resp = await fetch(`${RAIDZ_CONFIG.API_BASE}/raidz-expansion/capacity/${raidzLevel}/${width}?diskSizeGB=${diskSizeGB}`);
        const data = await resp.json();
        return data.data;
    }
};

// ============================================
// WebSocket 实时进度
// ============================================
const RAIDZWebSocket = {
    // 初始化连接
    connect(poolName) {
        if (RAIDZState.ws) {
            RAIDZWebSocket.disconnect();
        }
        
        const wsUrl = `${RAIDZ_CONFIG.WS_URL}/${encodeURIComponent(poolName)}`;
        console.log('WebSocket连接:', wsUrl);
        
        try {
            RAIDZState.ws = new WebSocket(wsUrl);
            
            RAIDZState.ws.onopen = () => {
                console.log('WebSocket已连接');
                RAIDZState.wsConnected = true;
            };
            
            RAIDZState.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    console.log('WebSocket消息:', data);
                    
                    // 更新进度显示
                    if (data.type === 'progress' || data.type === 'update') {
                        RAIDZUI.updateProgressDisplay(data.data);
                    }
                    
                    // 状态变更
                    if (data.type === 'state_change') {
                        RAIDZUI.updateStateButtons(data.status);
                    }
                } catch (e) {
                    console.error('WebSocket消息解析失败:', e);
                }
            };
            
            RAIDZState.ws.onclose = () => {
                console.log('WebSocket已关闭');
                RAIDZState.wsConnected = false;
                
                // 自动重连
                if (RAIDZState.activeProgress && RAIDZState.activeProgress.status === 'running') {
                    setTimeout(() => {
                        RAIDZWebSocket.connect(poolName);
                    }, RAIDZ_CONFIG.RECONNECT_INTERVAL);
                }
            };
            
            RAIDZState.ws.onerror = (error) => {
                console.error('WebSocket错误:', error);
                RAIDZState.wsConnected = false;
            };
        } catch (e) {
            console.error('WebSocket初始化失败:', e);
        }
    },
    
    // 断开连接
    disconnect() {
        if (RAIDZState.ws) {
            RAIDZState.ws.close();
            RAIDZState.ws = null;
            RAIDZState.wsConnected = false;
        }
    },
    
    // 发送消息
    send(message) {
        if (RAIDZState.ws && RAIDZState.wsConnected) {
            RAIDZState.ws.send(JSON.stringify(message));
        }
    }
};

// ============================================
// UI 渲染
// ============================================
const RAIDZUI = {
    // 初始化页面
    async init() {
        // 加载初始数据
        await RAIDZUI.refreshAll();
        
        // 启动定时刷新
        RAIDZUI.startAutoRefresh();
        
        // 初始化容量计算器
        calculateCapacity();
        
        console.log('RAIDZ扩容页面已初始化');
    },
    
    // 刷新所有数据
    async refreshAll() {
        try {
            // 获取Dashboard摘要
            const dashboard = await RAIDZAPI.getDashboard();
            
            // 更新统计卡片
            document.getElementById('summary-active').textContent = dashboard.activeCount || 0;
            document.getElementById('summary-completed').textContent = dashboard.completedCount || 0;
            document.getElementById('summary-supported').textContent = dashboard.expansionSupported ? '✅' : '❌';
            
            // 更新活跃任务显示
            if (dashboard.latestTask) {
                RAIDZState.activeProgress = dashboard.latestTask;
                RAIDZUI.showActiveTask(dashboard.latestTask);
                
                // 启动WebSocket连接
                RAIDZWebSocket.connect(dashboard.latestTask.poolName);
            } else {
                RAIDZUI.hideActiveTask();
            }
            
            // 更新可扩展池
            RAIDZState.expandablePools = dashboard.expandablePools || [];
            
            // 获取历史记录
            const history = await RAIDZAPI.getHistory(10);
            RAIDZState.history = history;
            
        } catch (e) {
            console.error('刷新数据失败:', e);
        }
    },
    
    // 显示活跃任务
    showActiveTask(progress) {
        document.getElementById('active-task-section').style.display = 'block';
        document.getElementById('empty-state').style.display = 'none';
        
        // 更新状态徽章
        const statusBadge = document.getElementById('task-status-badge');
        statusBadge.className = `progress-status-badge ${progress.status}`;
        statusBadge.textContent = progress.statusText || RAIDZUI.statusText(progress.status);
        
        // 更新进度条
        RAIDZUI.updateProgressDisplay(progress);
        
        // 更新操作按钮
        RAIDZUI.updateStateButtons(progress.status);
        
        // 渲染磁盘槽位
        RAIDZUI.renderDiskSlots(progress);
        
        // 渲染阶段进度
        RAIDZUI.renderPhases(progress.phases);
    },
    
    // 隐藏活跃任务
    hideActiveTask() {
        document.getElementById('active-task-section').style.display = 'none';
        document.getElementById('empty-state').style.display = 'block';
        
        // 断开WebSocket
        RAIDZWebSocket.disconnect();
        
        // 更新统计
        document.getElementById('summary-disks').textContent = RAIDZState.availableDisks.length || 0;
    },
    
    // 更新进度显示
    updateProgressDisplay(progress) {
        if (!progress) return;
        
        // 主进度条
        const percent = progress.percent || 0;
        document.getElementById('progress-fill').style.width = `${percent}%`;
        document.getElementById('progress-percent').textContent = `${percent.toFixed(1)}%`;
        
        // 进度信息
        document.getElementById('bytes-done').textContent = formatBytes(progress.bytesDone);
        document.getElementById('bytes-total').textContent = formatBytes(progress.bytesTotal);
        document.getElementById('speed-mbps').textContent = `${(progress.speedMBps || 0).toFixed(1)} MB/s`;
        document.getElementById('eta-time').textContent = progress.etaFormatted || '-';
        
        // 阶段进度
        if (progress.phases) {
            RAIDZUI.renderPhases(progress.phases);
        }
    },
    
    // 更新操作按钮状态
    updateStateButtons(status) {
        const pauseBtn = document.getElementById('pause-btn');
        const resumeBtn = document.getElementById('resume-btn');
        const cancelBtn = document.getElementById('cancel-btn');
        
        pauseBtn.style.display = status === 'running' ? 'inline-block' : 'none';
        resumeBtn.style.display = status === 'paused' ? 'inline-block' : 'none';
        cancelBtn.style.display = (status === 'running' || status === 'paused') ? 'inline-block' : 'none';
    },
    
    // 渲染磁盘槽位
    renderDiskSlots(progress) {
        const container = document.getElementById('disk-slots-visual');
        container.innerHTML = '';
        
        // 原始磁盘
        const originalDisks = progress.originalDisks || [];
        originalDisks.forEach((disk, i) => {
            const slot = document.createElement('div');
            slot.className = 'disk-slot-box original';
            slot.innerHTML = `
                <span>${disk.path || `盘${i+1}`}</span>
                <span class="disk-slot-label">原始</span>
            `;
            container.appendChild(slot);
        });
        
        // 新磁盘（动画）
        if (progress.newDisk) {
            const newSlot = document.createElement('div');
            newSlot.className = 'disk-slot-box new';
            newSlot.innerHTML = `
                <span>${progress.newDisk.path || '新盘'}</span>
                <span class="disk-slot-label">新增</span>
            `;
            container.appendChild(newSlot);
        }
    },
    
    // 渲染阶段进度
    renderPhases(phases) {
        const container = document.getElementById('phases-container');
        container.innerHTML = '';
        
        const defaultPhases = [
            { name: 'preparing', description: '准备扩展环境' },
            { name: 'data_scan', description: '扫描数据布局' },
            { name: 'data_migration', description: '重分布数据块' },
            { name: 'verification', description: '校验数据完整性' },
            { name: 'finalization', description: '更新元数据' },
            { name: 'completed', description: '扩展完成' }
        ];
        
        const phaseList = phases || defaultPhases;
        
        phaseList.forEach(phase => {
            const item = document.createElement('div');
            item.className = `phase-item ${phase.status || 'pending'}`;
            
            const indicatorClass = phase.status || 'pending';
            const icon = phase.status === 'completed' ? '✓' : 
                        phase.status === 'running' ? '⏳' : '○';
            
            item.innerHTML = `
                <div class="phase-indicator ${indicatorClass}">${icon}</div>
                <div class="phase-content">
                    <div class="phase-name">${phase.description || phase.name}</div>
                    <div class="phase-desc">${phase.name}</div>
                </div>
                <div class="phase-mini-bar">
                    <div class="phase-mini-fill" style="width: ${phase.percent || 0}%"></div>
                </div>
            `;
            
            container.appendChild(item);
        });
    },
    
    // 状态文本映射
    statusText(status) {
        const map = {
            'idle': '空闲',
            'preparing': '准备中',
            'running': '运行中',
            'paused': '已暂停',
            'completed': '已完成',
            'failed': '失败',
            'cancelled': '已取消'
        };
        return map[status] || status;
    },
    
    // 启动自动刷新
    startAutoRefresh() {
        if (RAIDZState.refreshTimer) {
            clearInterval(RAIDZState.refreshTimer);
        }
        
        RAIDZState.refreshTimer = setInterval(() => {
            RAIDZUI.refreshAll();
        }, RAIDZ_CONFIG.REFRESH_INTERVAL);
    },
    
    // 停止自动刷新
    stopAutoRefresh() {
        if (RAIDZState.refreshTimer) {
            clearInterval(RAIDZState.refreshTimer);
            RAIDZState.refreshTimer = null;
        }
    }
};

// ============================================
// 向导控制
// ============================================
const Wizard = {
    // 打开向导
    async start() {
        // 重置状态
        RAIDZState.wizard = {
            currentStep: 1,
            selectedPool: null,
            selectedDisk: null,
            validationResult: null
        };
        
        // 显示模态框
        document.getElementById('wizard-modal').classList.add('show');
        
        // 更新步骤指示器
        Wizard.updateStepIndicators(1);
        
        // 加载步骤内容
        await Wizard.loadStepContent(1);
    },
    
    // 关闭向导
    close() {
        document.getElementById('wizard-modal').classList.remove('show');
    },
    
    // 更新步骤指示器
    updateStepIndicators(step) {
        document.querySelectorAll('.wizard-step').forEach((el, i) => {
            el.classList.remove('active', 'completed');
            if (i + 1 < step) {
                el.classList.add('completed');
            } else if (i + 1 === step) {
                el.classList.add('active');
            }
        });
    },
    
    // 加载步骤内容
    async loadStepContent(step) {
        // 隐藏所有步骤内容
        document.querySelectorAll('.wizard-content').forEach(el => {
            el.style.display = 'none';
        });
        
        // 显示当前步骤
        document.getElementById(`step-${step}-content`).style.display = 'block';
        
        // 加载数据
        switch (step) {
            case 1:
                await Wizard.loadPools();
                break;
            case 2:
                await Wizard.loadDisks();
                break;
            case 3:
                await Wizard.validateAndEstimate();
                break;
            case 4:
                // 执行完成，无需加载
                break;
        }
    },
    
    // 加载可扩展池
    async loadPools() {
        const container = document.getElementById('pool-select-grid');
        container.innerHTML = '<p class="text-muted">正在加载...</p>';
        
        try {
            // 获取可用池（从Dashboard）
            const dashboard = await RAIDZAPI.getDashboard();
            const pools = dashboard.expandablePools || [];
            
            if (pools.length === 0) {
                container.innerHTML = '<p class="text-muted">暂无可扩容的存储池</p>';
                return;
            }
            
            container.innerHTML = '';
            pools.forEach(pool => {
                const card = document.createElement('div');
                card.className = 'disk-select-card';
                if (pool.hasActiveTask) {
                    card.classList.add('disabled');
                }
                
                card.innerHTML = `
                    <div class="disk-select-header">
                        <span class="disk-select-path">${pool.poolName}</span>
                        <span class="disk-select-status">${pool.raidzLevel}</span>
                    </div>
                    <div class="disk-select-details">
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">当前宽度:</span>
                            <span class="disk-detail-value">${pool.currentWidth}盘</span>
                        </div>
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">容量:</span>
                            <span class="disk-detail-value">${pool.currentCapGB.toFixed(0)} GB</span>
                        </div>
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">潜在增益:</span>
                            <span class="disk-detail-value">${pool.potentialGain.toFixed(0)} GB</span>
                        </div>
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">状态:</span>
                            <span class="disk-detail-value">${pool.healthy ? '✅健康' : '❌异常'}</span>
                        </div>
                    </div>
                `;
                
                if (!pool.hasActiveTask) {
                    card.onclick = () => Wizard.selectPool(pool, card);
                }
                
                container.appendChild(card);
            });
        } catch (e) {
            container.innerHTML = `<p class="text-muted" style="color: var(--color-error);">加载失败: ${e.message}</p>`;
        }
    },
    
    // 选择池
    selectPool(pool, card) {
        // 清除其他选择
        document.querySelectorAll('#pool-select-grid .disk-select-card').forEach(el => {
            el.classList.remove('selected');
        });
        
        // 标记选中
        card.classList.add('selected');
        RAIDZState.wizard.selectedPool = pool;
        
        // 启用下一步
        document.getElementById('step1-next').disabled = false;
    },
    
    // 加载可用磁盘
    async loadDisks() {
        const container = document.getElementById('disk-select-grid');
        container.innerHTML = '<p class="text-muted">正在加载...</p>';
        
        try {
            const disks = await RAIDZAPI.getAvailableDisks();
            RAIDZState.availableDisks = disks;
            
            if (disks.length === 0) {
                container.innerHTML = '<p class="text-muted">暂无可用磁盘</p>';
                return;
            }
            
            container.innerHTML = '';
            disks.forEach(disk => {
                const card = document.createElement('div');
                card.className = 'disk-select-card';
                
                card.innerHTML = `
                    <div class="disk-select-header">
                        <span class="disk-select-path">${disk.path}</span>
                        <span class="disk-select-status">${disk.state}</span>
                    </div>
                    <div class="disk-select-details">
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">型号:</span>
                            <span class="disk-detail-value">${disk.model || 'Unknown'}</span>
                        </div>
                        <div class="disk-detail-item">
                            <span class="disk-detail-label">容量:</span>
                            <span class="disk-detail-value">${disk.sizeGB} GB</span>
                        </div>
                    </div>
                `;
                
                card.onclick = () => Wizard.selectDisk(disk, card);
                container.appendChild(card);
            });
        } catch (e) {
            container.innerHTML = `<p class="text-muted" style="color: var(--color-error);">加载失败: ${e.message}</p>`;
        }
    },
    
    // 选择磁盘
    selectDisk(disk, card) {
        document.querySelectorAll('#disk-select-grid .disk-select-card').forEach(el => {
            el.classList.remove('selected');
        });
        
        card.classList.add('selected');
        RAIDZState.wizard.selectedDisk = disk;
        
        document.getElementById('step2-next').disabled = false;
    },
    
    // 验证和估算
    async validateAndEstimate() {
        const pool = RAIDZState.wizard.selectedPool;
        const disk = RAIDZState.wizard.selectedDisk;
        
        // 更新确认信息
        document.getElementById('confirm-pool').textContent = pool.poolName;
        document.getElementById('confirm-raidz').textContent = pool.raidzLevel;
        document.getElementById('confirm-width-before').textContent = `${pool.currentWidth}盘`;
        document.getElementById('confirm-width-after').textContent = `${pool.currentWidth + 1}盘`;
        document.getElementById('confirm-disk').textContent = disk.path;
        document.getElementById('confirm-disk-size').textContent = `${disk.sizeGB} GB`;
        
        // 执行验证
        const checksContainer = document.getElementById('validation-checks');
        checksContainer.innerHTML = '<p class="text-muted">正在验证...</p>';
        
        try {
            const validationResult = await RAIDZAPI.validateExpansion(pool.poolName, disk.path);
            RAIDZState.wizard.validationResult = validationResult;
            
            // 渲染检查结果
            checksContainer.innerHTML = '';
            validationResult.checks.forEach(check => {
                const checkItem = document.createElement('div');
                checkItem.className = 'phase-item';
                checkItem.style.background = check.passed ? 
                    'rgba(var(--color-success-rgb), 0.1)' : 
                    'rgba(var(--color-error-rgb), 0.1)';
                checkItem.innerHTML = `
                    <div class="phase-indicator ${check.passed ? 'completed' : 'failed'}">
                        ${check.passed ? '✓' : '✗'}
                    </div>
                    <div class="phase-content">
                        <div class="phase-name">${check.description}</div>
                    </div>
                `;
                checksContainer.appendChild(checkItem);
            });
            
            // 显示估算
            try {
                const estimate = await RAIDZAPI.estimateExpansion(
                    pool.poolName,
                    pool.raidzLevel,
                    pool.currentWidth,
                    disk.sizeGB,
                    pool.usedBytes || 0
                );
                
                document.getElementById('estimate-time').textContent = estimate.estimatedTime || '-';
                document.getElementById('estimate-capacity').textContent = `${estimate.capacityGainGB.toFixed(0)} GB`;
                document.getElementById('estimate-efficiency').textContent = `${estimate.efficiencyAfter.toFixed(1)}%`;
            } catch (e) {
                document.getElementById('estimate-time').textContent = '需根据数据量计算';
                document.getElementById('estimate-capacity').textContent = `${disk.sizeGB} GB`;
            }
            
            // 如果验证失败，禁用下一步
            if (!validationResult.canExpand) {
                document.getElementById('step3-next').disabled = true;
                document.getElementById('step3-next').textContent = '验证失败，无法扩容';
            }
        } catch (e) {
            checksContainer.innerHTML = `<p style="color: var(--color-error);">验证失败: ${e.message}</p>`;
        }
    },
    
    // 执行扩容
    async execute() {
        const pool = RAIDZState.wizard.selectedPool;
        const disk = RAIDZState.wizard.selectedDisk;
        
        try {
            const result = await RAIDZAPI.startExpansion(pool.poolName, disk.path);
            
            if (result.code === 0 || result.code === 202) {
                // 成功，切换到完成步骤
                Wizard.updateStepIndicators(4);
                await Wizard.loadStepContent(4);
                
                // 刷新主页数据
                setTimeout(() => {
                    RAIDZUI.refreshAll();
                }, 1000);
            } else {
                alert(`扩容启动失败: ${result.message}`);
            }
        } catch (e) {
            alert(`扩容启动失败: ${e.message}`);
        }
    },
    
    // 下一步
    async nextStep() {
        const current = RAIDZState.wizard.currentStep;
        
        if (current === 3) {
            // 执行扩容
            await Wizard.execute();
            return;
        }
        
        RAIDZState.wizard.currentStep = current + 1;
        Wizard.updateStepIndicators(RAIDZState.wizard.currentStep);
        await Wizard.loadStepContent(RAIDZState.wizard.currentStep);
    },
    
    // 上一步
    async prevStep() {
        const current = RAIDZState.wizard.currentStep;
        if (current <= 1) return;
        
        RAIDZState.wizard.currentStep = current - 1;
        Wizard.updateStepIndicators(RAIDZState.wizard.currentStep);
        await Wizard.loadStepContent(RAIDZState.wizard.currentStep);
    }
};

// ============================================
// 操作控制
// ============================================

// 暂停扩容
async function pauseExpansion() {
    if (!RAIDZState.activeProgress) return;
    
    try {
        await RAIDZAPI.pauseExpansion(RAIDZState.activeProgress.poolName);
        RAIDZUI.updateStateButtons('paused');
        
        // 刷新状态
        await RAIDZUI.refreshAll();
    } catch (e) {
        alert(`暂停失败: ${e.message}`);
    }
}

// 恢复扩容
async function resumeExpansion() {
    if (!RAIDZState.activeProgress) return;
    
    try {
        await RAIDZAPI.resumeExpansion(RAIDZState.activeProgress.poolName);
        RAIDZUI.updateStateButtons('running');
        
        // 重新连接WebSocket
        RAIDZWebSocket.connect(RAIDZState.activeProgress.poolName);
        
        // 刷新状态
        await RAIDZUI.refreshAll();
    } catch (e) {
        alert(`恢复失败: ${e.message}`);
    }
}

// 取消扩容
async function cancelExpansion() {
    if (!RAIDZState.activeProgress) return;
    
    if (!confirm('确定要取消当前扩容任务吗？此操作不可撤销。')) {
        return;
    }
    
    try {
        await RAIDZAPI.cancelExpansion(RAIDZState.activeProgress.poolName);
        
        // 断开WebSocket
        RAIDZWebSocket.disconnect();
        
        // 刷新状态
        await RAIDZUI.refreshAll();
    } catch (e) {
        alert(`取消失败: ${e.message}`);
    }
}

// 刷新状态
async function refreshStatus() {
    await RAIDZUI.refreshAll();
}

// 打开向导
function startWizard() {
    Wizard.start();
}

// 关闭向导
function closeWizard() {
    Wizard.close();
}

// 显示历史模态框
async function showHistoryModal() {
    document.getElementById('history-modal').classList.add('show');
    
    const container = document.getElementById('history-list');
    container.innerHTML = '';
    
    RAIDZState.history.forEach(item => {
        const historyItem = document.createElement('div');
        historyItem.className = 'history-item';
        
        const statusClass = item.status === 'completed' ? 'completed' : 
                           item.status === 'failed' ? 'failed' : 'cancelled';
        const statusText = RAIDZUI.statusText(item.status);
        
        historyItem.innerHTML = `
            <div class="history-info">
                <div class="history-pool">${item.poolName}</div>
                <div class="history-time">${formatTime(item.startTime)} - ${formatTime(item.endTime)}</div>
            </div>
            <span class="history-status ${statusClass}">${statusText}</span>
        `;
        
        container.appendChild(historyItem);
    });
}

// 关闭历史模态框
function closeHistoryModal() {
    document.getElementById('history-modal').classList.remove('show');
}

// 容量计算器
function calculateCapacity() {
    const raidzLevel = document.getElementById('calc-raidz-level').value;
    const width = parseInt(document.getElementById('calc-width').value) || 4;
    const diskSize = parseFloat(document.getElementById('calc-disk-size').value) || 1000;
    
    // 本地计算（快速预览）
    const parityCount = {
        'raidz1': 1,
        'raidz2': 2,
        'raidz3': 3
    }[raidzLevel] || 1;
    
    const widthAfter = width + 1;
    const dataDisksBefore = width - parityCount;
    const dataDisksAfter = widthAfter - parityCount;
    
    const capacityBefore = dataDisksBefore * diskSize;
    const capacityAfter = dataDisksAfter * diskSize;
    const capacityGain = diskSize;
    
    const efficiencyBefore = (dataDisksBefore / width) * 100;
    const efficiencyAfter = (dataDisksAfter / widthAfter) * 100;
    
    document.getElementById('capacity-before').textContent = `${capacityBefore.toFixed(0)} GB`;
    document.getElementById('capacity-after').textContent = `${capacityAfter.toFixed(0)} GB`;
    document.getElementById('capacity-gain').textContent = `+${capacityGain.toFixed(0)} GB`;
    document.getElementById('efficiency-before').textContent = `${efficiencyBefore.toFixed(1)}%`;
    document.getElementById('efficiency-after').textContent = `${efficiencyAfter.toFixed(1)}%`;
    document.getElementById('parity-disks').textContent = parityCount;
}

// 重置计算器
function resetCalculator() {
    document.getElementById('calc-raidz-level').value = 'raidz1';
    document.getElementById('calc-width').value = 4;
    document.getElementById('calc-disk-size').value = 1000;
    calculateCapacity();
}

// ============================================
// 辅助函数
// ============================================

// 格式化字节
function formatBytes(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024;
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + units[i];
}

// 格式化时间
function formatTime(dateStr) {
    if (!dateStr) return '-';
    const date = new Date(dateStr);
    return date.toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
    });
}

// ============================================
// 页面初始化
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    RAIDZUI.init();
});