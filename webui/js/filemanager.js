/**
 * NAS-OS File Manager Core
 * Main file browser with grid/list view, context menu, batch operations
 */

class FileManager {
    constructor(options = {}) {
        this.apiBase = options.apiBase || '/api/v1/files';
        this.containerId = options.containerId || 'file-manager';
        this.currentPath = options.currentPath || '/';
        this.viewMode = options.viewMode || 'grid'; // grid or list
        this.sortBy = options.sortBy || 'name'; // name, size, date, type
        this.sortOrder = options.sortOrder || 'asc';
        
        this.files = [];
        this.selectedFiles = new Set();
        this.isBatchMode = false;
        this.clipboard = null; // for copy/cut
        
        this.init();
    }

    init() {
        this.container = document.getElementById(this.containerId);
        if (!this.container) {
            console.error('File manager container not found:', this.containerId);
            return;
        }

        // Initialize sub-managers
        this.uploadManager = new FileUploadManager({
            apiBase: this.apiBase,
            currentPath: this.currentPath,
            onUploadComplete: () => this.refresh()
        });

        this.previewManager = new FilePreviewManager({
            apiBase: this.apiBase
        });

        this.shareManager = new FileShareManager({
            apiBase: this.apiBase
        });

        this.render();
        this.bindEvents();
        this.loadFiles(this.currentPath);
    }

    render() {
        this.container.innerHTML = `
            <div class="file-manager batch-mode-${this.isBatchMode ? 'active' : ''}">
                <!-- Header -->
                <div class="fm-header">
                    <h1 class="fm-title">📁 文件管理器</h1>
                    <div class="fm-actions">
                        <button class="btn-fm btn-fm-primary" onclick="fileManager.showUploadDialog()">
                            📤 上传
                        </button>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.createFolder()">
                            📁 新建文件夹
                        </button>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.toggleBatchMode()">
                            ☑️ 批量选择
                        </button>
                    </div>
                </div>

                <!-- Breadcrumb -->
                <div class="fm-breadcrumb" id="fm-breadcrumb"></div>

                <!-- Toolbar -->
                <div class="fm-toolbar" id="fm-toolbar">
                    <div class="fm-toolbar-left">
                        <button class="btn-fm-icon ${this.viewMode === 'grid' ? 'active' : ''}" onclick="fileManager.setViewMode('grid')" title="网格视图">
                            ▦
                        </button>
                        <button class="btn-fm-icon ${this.viewMode === 'list' ? 'active' : ''}" onclick="fileManager.setViewMode('list')" title="列表视图">
                            ☰
                        </button>
                        <span style="color: var(--fm-text-muted); font-size: 13px;" id="file-count"></span>
                    </div>
                    <div class="fm-toolbar-right">
                        <input type="text" class="fm-share-input" placeholder="🔍 搜索文件..." style="width: 200px;" oninput="fileManager.searchFiles(this.value)">
                        <select class="fm-share-select" style="width: 120px;" onchange="fileManager.setSortBy(this.value)">
                            <option value="name">按名称</option>
                            <option value="size">按大小</option>
                            <option value="date">按日期</option>
                            <option value="type">按类型</option>
                        </select>
                    </div>
                </div>

                <!-- Batch Mode Toolbar -->
                <div class="fm-toolbar" id="batch-toolbar" style="display: none;">
                    <div class="fm-toolbar-left">
                        <span class="batch-count" id="selected-count">0 项已选择</span>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.selectAll()">全选</button>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.deselectAll()">取消全选</button>
                    </div>
                    <div class="fm-toolbar-right">
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.batchDownload()">⬇️ 下载</button>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.batchMove()">📦 移动</button>
                        <button class="btn-fm btn-fm-danger" onclick="fileManager.batchDelete()">🗑️ 删除</button>
                        <button class="btn-fm btn-fm-secondary" onclick="fileManager.toggleBatchMode()">✕ 退出批量</button>
                    </div>
                </div>

                <!-- File Container -->
                <div id="fm-files-container"></div>

                <!-- Empty State -->
                <div class="fm-empty" id="fm-empty" style="display: none;">
                    <div class="fm-empty-icon">📂</div>
                    <div class="fm-empty-text">此文件夹为空</div>
                    <div class="fm-empty-hint">拖放文件到这里开始上传</div>
                </div>

                <!-- Loading -->
                <div class="fm-loading" id="fm-loading" style="display: none;">
                    <div class="fm-spinner"></div>
                </div>
            </div>

            <!-- Context Menu -->
            <div class="fm-context-menu" id="fm-context-menu">
                <div class="fm-context-item" onclick="fileManager.contextAction('open')">📂 打开</div>
                <div class="fm-context-item" onclick="fileManager.contextAction('preview')">👁️ 预览</div>
                <div class="fm-context-item" onclick="fileManager.contextAction('download')">⬇️ 下载</div>
                <div class="fm-context-divider"></div>
                <div class="fm-context-item" onclick="fileManager.contextAction('share')">🔗 分享</div>
                <div class="fm-context-item" onclick="fileManager.contextAction('copy')">📋 复制</div>
                <div class="fm-context-item" onclick="fileManager.contextAction('cut')">✂️ 剪切</div>
                <div class="fm-context-divider"></div>
                <div class="fm-context-item" onclick="fileManager.contextAction('rename')">✏️ 重命名</div>
                <div class="fm-context-item" onclick="fileManager.contextAction('info')">ℹ️ 属性</div>
                <div class="fm-context-divider"></div>
                <div class="fm-context-item danger" onclick="fileManager.contextAction('delete')">🗑️ 删除</div>
            </div>
        `;

        this.updateBreadcrumb();
    }

    bindEvents() {
        // Click outside to close context menu
        document.addEventListener('click', () => {
            this.hideContextMenu();
        });

        // Keyboard shortcuts
        document.addEventListener('keydown', (e) => {
            // Ctrl+A - Select all
            if (e.ctrlKey && e.key === 'a' && this.isBatchMode) {
                e.preventDefault();
                this.selectAll();
            }
            // Delete - Delete selected
            if (e.key === 'Delete' && this.selectedFiles.size > 0) {
                this.batchDelete();
            }
            // Escape - Deselect all
            if (e.key === 'Escape') {
                this.deselectAll();
                this.hideContextMenu();
            }
            // Ctrl+C - Copy
            if (e.ctrlKey && e.key === 'c' && this.selectedFiles.size > 0) {
                this.copySelected();
            }
            // Ctrl+X - Cut
            if (e.ctrlKey && e.key === 'x' && this.selectedFiles.size > 0) {
                this.cutSelected();
            }
            // Ctrl+V - Paste
            if (e.ctrlKey && e.key === 'v' && this.clipboard) {
                this.paste();
            }
        });
    }

    async loadFiles(path) {
        this.currentPath = path;
        this.uploadManager.setCurrentPath(path);
        
        // Show loading
        document.getElementById('fm-loading').style.display = 'flex';
        document.getElementById('fm-files-container').innerHTML = '';
        document.getElementById('fm-empty').style.display = 'none';

        try {
            // In real app, fetch from API
            // const response = await fetch(`${this.apiBase}/list?path=${encodeURIComponent(path)}`);
            // const data = await response.json();
            // this.files = data.files || [];

            // Mock data for demo
            this.files = this.getMockFiles(path);
            
            this.renderFiles();
            this.updateBreadcrumb();
            this.updateFileCount();
        } catch (error) {
            console.error('Failed to load files:', error);
            this.showToast('加载文件列表失败', 'error');
        } finally {
            document.getElementById('fm-loading').style.display = 'none';
        }
    }

    getMockFiles(path) {
        // Mock file data for demo
        const mockFiles = [
            { name: '文档', path: path + '文档', isDir: true, size: 0, modified: '2024-03-15T10:30:00' },
            { name: '图片', path: path + '图片', isDir: true, size: 0, modified: '2024-03-14T15:20:00' },
            { name: '视频', path: path + '视频', isDir: true, size: 0, modified: '2024-03-13T09:15:00' },
            { name: '音乐', path: path + '音乐', isDir: true, size: 0, modified: '2024-03-12T14:45:00' },
            { name: '项目计划.pdf', path: path + '项目计划.pdf', isDir: false, size: 2516582, modified: '2024-03-15T16:30:00' },
            { name: '财务报表.xlsx', path: path + '财务报表.xlsx', isDir: false, size: 1887436, modified: '2024-03-14T11:20:00' },
            { name: '封面图.png', path: path + '封面图.png', isDir: false, size: 3355443, modified: '2024-03-13T08:15:00' },
            { name: '演示视频.mp4', path: path + '演示视频.mp4', isDir: false, size: 134217728, modified: '2024-03-12T20:30:00' },
            { name: '会议录音.mp3', path: path + '会议录音.mp3', isDir: false, size: 8388608, modified: '2024-03-11T17:45:00' },
            { name: '项目代码.zip', path: path + '项目代码.zip', isDir: false, size: 52428800, modified: '2024-03-10T12:00:00' },
            { name: 'readme.md', path: path + 'readme.md', isDir: false, size: 4096, modified: '2024-03-09T10:00:00' },
            { name: 'config.json', path: path + 'config.json', isDir: false, size: 2048, modified: '2024-03-08T09:30:00' }
        ];

        // Sort
        return this.sortFiles(mockFiles);
    }

    sortFiles(files) {
        return files.sort((a, b) => {
            // Folders first
            if (a.isDir && !b.isDir) return -1;
            if (!a.isDir && b.isDir) return 1;

            let comparison = 0;
            switch(this.sortBy) {
                case 'name':
                    comparison = a.name.localeCompare(b.name);
                    break;
                case 'size':
                    comparison = a.size - b.size;
                    break;
                case 'date':
                    comparison = new Date(a.modified) - new Date(b.modified);
                    break;
                case 'type':
                    const extA = a.name.split('.').pop().toLowerCase();
                    const extB = b.name.split('.').pop().toLowerCase();
                    comparison = extA.localeCompare(extB);
                    break;
            }
            return this.sortOrder === 'asc' ? comparison : -comparison;
        });
    }

    renderFiles() {
        const container = document.getElementById('fm-files-container');
        const emptyState = document.getElementById('fm-empty');

        if (this.files.length === 0) {
            container.innerHTML = '';
            emptyState.style.display = 'block';
            return;
        }

        emptyState.style.display = 'none';

        if (this.viewMode === 'grid') {
            this.renderGridView(container);
        } else {
            this.renderListView(container);
        }
    }

    renderGridView(container) {
        container.innerHTML = `<div class="fm-file-grid ${this.isBatchMode ? 'batch-mode' : ''}" id="fm-grid">
            ${this.files.map(file => this.renderGridItem(file)).join('')}
        </div>`;
    }

    renderGridItem(file) {
        const isSelected = this.selectedFiles.has(file.path);
        const icon = file.isDir ? '📁' : this.getFileIcon(file.name);
        const thumbnail = !file.isDir && this.isImage(file.name) ? 
            `<img class="fm-thumbnail" src="${this.apiBase}/thumbnail?path=${encodeURIComponent(file.path)}" alt="" onerror="this.style.display='none'">` : '';

        return `
            <div class="fm-file-item ${isSelected ? 'selected' : ''}" 
                 data-path="${this.escapeHtml(file.path)}"
                 ondblclick="fileManager.openItem('${this.escapeHtml(file.path)}', ${file.isDir})"
                 onclick="fileManager.handleItemClick(event, '${this.escapeHtml(file.path)}')"
                 oncontextmenu="fileManager.showContextMenu(event, '${this.escapeHtml(file.path)}')">
                <div class="select-checkbox" onclick="event.stopPropagation(); fileManager.toggleSelect('${this.escapeHtml(file.path)}')">
                    ${isSelected ? '✓' : ''}
                </div>
                ${thumbnail || `<div class="fm-file-icon ${this.getFileType(file.name)}">${icon}</div>`}
                <div class="fm-file-name" title="${this.escapeHtml(file.name)}">${this.escapeHtml(file.name)}</div>
                <div class="fm-file-meta">${file.isDir ? '文件夹' : this.formatSize(file.size)}</div>
            </div>
        `;
    }

    renderListView(container) {
        container.innerHTML = `
            <div class="fm-file-list ${this.isBatchMode ? 'batch-mode' : ''}">
                <div class="fm-file-list-header">
                    <div></div>
                    <div onclick="fileManager.setSortBy('name')" style="cursor: pointer;">名称 ↕</div>
                    <div onclick="fileManager.setSortBy('size')" style="cursor: pointer;">大小 ↕</div>
                    <div onclick="fileManager.setSortBy('date')" style="cursor: pointer;">修改日期 ↕</div>
                    <div>类型</div>
                </div>
                ${this.files.map(file => this.renderListItem(file)).join('')}
            </div>
        `;
    }

    renderListItem(file) {
        const isSelected = this.selectedFiles.has(file.path);
        const icon = file.isDir ? '📁' : this.getFileIcon(file.name);

        return `
            <div class="fm-file-list-item ${isSelected ? 'selected' : ''}"
                 data-path="${this.escapeHtml(file.path)}"
                 ondblclick="fileManager.openItem('${this.escapeHtml(file.path)}', ${file.isDir})"
                 onclick="fileManager.handleItemClick(event, '${this.escapeHtml(file.path)}')"
                 oncontextmenu="fileManager.showContextMenu(event, '${this.escapeHtml(file.path)}')">
                <div style="cursor: pointer;" onclick="event.stopPropagation(); fileManager.toggleSelect('${this.escapeHtml(file.path)}')">
                    ${isSelected ? '☑️' : '☐'}
                </div>
                <div style="display: flex; align-items: center; gap: 0.5rem;">
                    <span>${icon}</span>
                    <span class="fm-file-name">${this.escapeHtml(file.name)}</span>
                </div>
                <div class="fm-file-meta">${file.isDir ? '-' : this.formatSize(file.size)}</div>
                <div class="fm-file-meta">${this.formatDate(file.modified)}</div>
                <div class="fm-file-meta">${file.isDir ? '文件夹' : this.getFileType(file.name)}</div>
            </div>
        `;
    }

    updateBreadcrumb() {
        const breadcrumb = document.getElementById('fm-breadcrumb');
        if (!breadcrumb) return;

        const parts = this.currentPath.split('/').filter(p => p);
        let html = `<a href="#" onclick="fileManager.loadFiles('/')">🏠 根目录</a>`;
        let path = '';

        parts.forEach((part, index) => {
            path += '/' + part;
            html += `<span class="fm-breadcrumb-sep">/</span>`;
            if (index === parts.length - 1) {
                html += `<span>${this.escapeHtml(part)}</span>`;
            } else {
                html += `<a href="#" onclick="fileManager.loadFiles('${this.escapeHtml(path)}')">${this.escapeHtml(part)}</a>`;
            }
        });

        breadcrumb.innerHTML = html;
    }

    updateFileCount() {
        const countEl = document.getElementById('file-count');
        if (countEl) {
            countEl.textContent = `${this.files.length} 个项目`;
        }
    }

    openItem(path, isDir) {
        if (isDir) {
            this.loadFiles(path + '/');
        } else {
            const file = this.files.find(f => f.path === path);
            if (file) {
                this.previewManager.open(file, this.files.filter(f => !f.isDir));
            }
        }
    }

    handleItemClick(event, path) {
        if (this.isBatchMode) {
            event.preventDefault();
            this.toggleSelect(path);
        }
    }

    toggleSelect(path) {
        if (this.selectedFiles.has(path)) {
            this.selectedFiles.delete(path);
        } else {
            this.selectedFiles.add(path);
        }
        this.updateSelectionUI();
    }

    selectAll() {
        this.files.forEach(f => this.selectedFiles.add(f.path));
        this.updateSelectionUI();
    }

    deselectAll() {
        this.selectedFiles.clear();
        this.updateSelectionUI();
    }

    updateSelectionUI() {
        // Update selected count
        const countEl = document.getElementById('selected-count');
        if (countEl) {
            countEl.textContent = `${this.selectedFiles.size} 项已选择`;
        }

        // Update visual selection
        document.querySelectorAll('.fm-file-item, .fm-file-list-item').forEach(el => {
            const path = el.dataset.path;
            if (this.selectedFiles.has(path)) {
                el.classList.add('selected');
            } else {
                el.classList.remove('selected');
            }
        });
    }

    toggleBatchMode() {
        this.isBatchMode = !this.isBatchMode;
        const toolbar = document.getElementById('fm-toolbar');
        const batchToolbar = document.getElementById('batch-toolbar');

        if (this.isBatchMode) {
            toolbar.style.display = 'none';
            batchToolbar.style.display = 'flex';
        } else {
            toolbar.style.display = 'flex';
            batchToolbar.style.display = 'none';
            this.deselectAll();
        }

        this.renderFiles();
    }

    setViewMode(mode) {
        this.viewMode = mode;
        this.render();
        this.renderFiles();
    }

    setSortBy(sortBy) {
        if (this.sortBy === sortBy) {
            this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
        } else {
            this.sortBy = sortBy;
            this.sortOrder = 'asc';
        }
        this.files = this.sortFiles([...this.files]);
        this.renderFiles();
    }

    searchFiles(query) {
        if (!query) {
            this.loadFiles(this.currentPath);
            return;
        }

        const filtered = this.files.filter(f => 
            f.name.toLowerCase().includes(query.toLowerCase())
        );
        
        const container = document.getElementById('fm-files-container');
        const emptyState = document.getElementById('fm-empty');

        if (filtered.length === 0) {
            container.innerHTML = '';
            emptyState.style.display = 'block';
            document.querySelector('.fm-empty-text').textContent = '没有找到匹配的文件';
        } else {
            emptyState.style.display = 'none';
            this.files = filtered;
            this.renderFiles();
        }
    }

    showContextMenu(event, path) {
        event.preventDefault();
        event.stopPropagation();

        this.contextMenuTarget = path;
        const menu = document.getElementById('fm-context-menu');
        
        // Position menu
        menu.style.left = `${event.clientX}px`;
        menu.style.top = `${event.clientY}px`;
        menu.classList.add('active');

        // Adjust if near edge
        setTimeout(() => {
            const rect = menu.getBoundingClientRect();
            if (rect.right > window.innerWidth) {
                menu.style.left = `${event.clientX - rect.width}px`;
            }
            if (rect.bottom > window.innerHeight) {
                menu.style.top = `${event.clientY - rect.height}px`;
            }
        }, 0);
    }

    hideContextMenu() {
        const menu = document.getElementById('fm-context-menu');
        if (menu) menu.classList.remove('active');
    }

    contextAction(action) {
        const path = this.contextMenuTarget;
        const file = this.files.find(f => f.path === path);
        
        if (!file) return;

        switch(action) {
            case 'open':
                this.openItem(path, file.isDir);
                break;
            case 'preview':
                if (!file.isDir) {
                    this.previewManager.open(file, this.files.filter(f => !f.isDir));
                }
                break;
            case 'download':
                this.downloadFile(path);
                break;
            case 'share':
                this.shareManager.open(file);
                break;
            case 'copy':
                this.copySelected();
                break;
            case 'cut':
                this.cutSelected();
                break;
            case 'rename':
                this.renameFile(path);
                break;
            case 'info':
                this.showFileInfo(file);
                break;
            case 'delete':
                this.deleteFile(path);
                break;
        }

        this.hideContextMenu();
    }

    showUploadDialog() {
        const input = document.createElement('input');
        input.type = 'file';
        input.multiple = true;
        input.onchange = () => {
            this.uploadManager.handleFiles(input.files);
        };
        input.click();
    }

    async createFolder() {
        const name = prompt('请输入文件夹名称:');
        if (!name) return;

        try {
            // In real app, call API
            // await fetch(`${this.apiBase}/mkdir`, {
            //     method: 'POST',
            //     headers: { 'Content-Type': 'application/json' },
            //     body: JSON.stringify({ path: this.currentPath + name })
            // });
            
            this.showToast(`文件夹 "${name}" 已创建`, 'success');
            this.refresh();
        } catch (error) {
            this.showToast('创建文件夹失败', 'error');
        }
    }

    async renameFile(path) {
        const file = this.files.find(f => f.path === path);
        if (!file) return;

        const newName = prompt('请输入新名称:', file.name);
        if (!newName || newName === file.name) return;

        try {
            // In real app, call API
            // await fetch(`${this.apiBase}/rename`, {
            //     method: 'POST',
            //     headers: { 'Content-Type': 'application/json' },
            //     body: JSON.stringify({ path: path, newName: newName })
            // });
            
            this.showToast('重命名成功', 'success');
            this.refresh();
        } catch (error) {
            this.showToast('重命名失败', 'error');
        }
    }

    async deleteFile(path) {
        const file = this.files.find(f => f.path === path);
        if (!file) return;

        if (!confirm(`确定要删除 "${file.name}" 吗？`)) return;

        try {
            // In real app, call API
            // await fetch(`${this.apiBase}/delete`, {
            //     method: 'DELETE',
            //     headers: { 'Content-Type': 'application/json' },
            //     body: JSON.stringify({ path: path })
            // });
            
            this.showToast(`"${file.name}" 已删除`, 'success');
            this.refresh();
        } catch (error) {
            this.showToast('删除失败', 'error');
        }
    }

    async batchDelete() {
        if (this.selectedFiles.size === 0) return;
        
        if (!confirm(`确定要删除选中的 ${this.selectedFiles.size} 个项目吗？`)) return;

        try {
            // In real app, call API for each file
            // for (const path of this.selectedFiles) {
            //     await fetch(`${this.apiBase}/delete`, { method: 'DELETE', body: JSON.stringify({ path }) });
            // }
            
            this.showToast(`已删除 ${this.selectedFiles.size} 个项目`, 'success');
            this.selectedFiles.clear();
            this.refresh();
        } catch (error) {
            this.showToast('批量删除失败', 'error');
        }
    }

    batchDownload() {
        if (this.selectedFiles.size === 0) return;
        
        // In real app, create zip and download
        this.showToast('正在准备下载...', 'info');
        
        // Mock download
        setTimeout(() => {
            this.showToast('下载已开始', 'success');
        }, 1000);
    }

    batchMove() {
        if (this.selectedFiles.size === 0) return;
        
        const dest = prompt('请输入目标路径:');
        if (!dest) return;

        // In real app, call API
        this.showToast(`正在移动 ${this.selectedFiles.size} 个项目到 ${dest}`, 'info');
    }

    copySelected() {
        if (this.selectedFiles.size === 0) return;
        this.clipboard = {
            action: 'copy',
            files: Array.from(this.selectedFiles)
        };
        this.showToast(`已复制 ${this.selectedFiles.size} 个项目`, 'info');
    }

    cutSelected() {
        if (this.selectedFiles.size === 0) return;
        this.clipboard = {
            action: 'cut',
            files: Array.from(this.selectedFiles)
        };
        this.showToast(`已剪切 ${this.selectedFiles.size} 个项目`, 'info');
    }

    async paste() {
        if (!this.clipboard) return;

        try {
            // In real app, call API
            // await fetch(`${this.apiBase}/paste`, {
            //     method: 'POST',
            //     body: JSON.stringify({
            //         action: this.clipboard.action,
            //         files: this.clipboard.files,
            //         destination: this.currentPath
            //     })
            // });

            this.showToast(`已粘贴 ${this.clipboard.files.length} 个项目`, 'success');
            
            if (this.clipboard.action === 'cut') {
                this.clipboard = null;
            }
            
            this.refresh();
        } catch (error) {
            this.showToast('粘贴失败', 'error');
        }
    }

    downloadFile(path) {
        const a = document.createElement('a');
        a.href = `${this.apiBase}/download?path=${encodeURIComponent(path)}`;
        a.download = path.split('/').pop();
        a.click();
    }

    showFileInfo(file) {
        alert(`
文件名: ${file.name}
路径: ${file.path}
类型: ${file.isDir ? '文件夹' : this.getFileType(file.name)}
大小: ${file.isDir ? '-' : this.formatSize(file.size)}
修改时间: ${this.formatDate(file.modified)}
        `.trim());
    }

    refresh() {
        this.loadFiles(this.currentPath);
    }

    // Helper methods
    getFileIcon(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        const iconMap = {
            'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️', 'gif': '🖼️', 'webp': '🖼️', 'svg': '🖼️',
            'mp4': '🎬', 'avi': '🎬', 'mkv': '🎬', 'mov': '🎬',
            'mp3': '🎵', 'wav': '🎵', 'flac': '🎵', 'aac': '🎵',
            'pdf': '📄', 'doc': '📝', 'docx': '📝', 'xls': '📊', 'xlsx': '📊',
            'zip': '📦', 'rar': '📦', '7z': '📦', 'tar': '📦', 'gz': '📦',
            'js': '💻', 'ts': '💻', 'py': '💻', 'java': '💻', 'html': '💻', 'css': '💻',
            'json': '📋', 'xml': '📋', 'yml': '📋', 'yaml': '📋', 'md': '📝', 'txt': '📝'
        };
        return iconMap[ext] || '📄';
    }

    getFileType(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        const typeMap = {
            'jpg': 'image', 'jpeg': 'image', 'png': 'image', 'gif': 'image', 'webp': 'image', 'svg': 'image',
            'mp4': 'video', 'avi': 'video', 'mkv': 'video', 'mov': 'video', 'wmv': 'video',
            'mp3': 'audio', 'wav': 'audio', 'flac': 'audio', 'aac': 'audio', 'ogg': 'audio',
            'pdf': 'document', 'doc': 'document', 'docx': 'document', 'xls': 'document', 'xlsx': 'document',
            'zip': 'archive', 'rar': 'archive', '7z': 'archive', 'tar': 'archive', 'gz': 'archive',
            'js': 'code', 'ts': 'code', 'py': 'code', 'java': 'code', 'cpp': 'code', 'html': 'code', 'css': 'code'
        };
        return typeMap[ext] || 'file';
    }

    isImage(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp'].includes(ext);
    }

    formatSize(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    formatDate(dateStr) {
        if (!dateStr) return '-';
        const date = new Date(dateStr);
        return date.toLocaleDateString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit'
        });
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text || '';
        return div.innerHTML;
    }

    showToast(message, type = 'info') {
        const toast = document.createElement('div');
        toast.className = `fm-toast ${type}`;
        toast.textContent = message;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 3000);
    }
}

// Export
window.FileManager = FileManager;
