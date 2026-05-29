/**
 * NAS-OS File Upload Manager
 * Drag & Drop + File Upload Component
 */

class FileUploadManager {
    constructor(options = {}) {
        this.apiBase = options.apiBase || '/api/v1/files';
        this.currentPath = options.currentPath || '/';
        this.maxFileSize = options.maxFileSize || 10 * 1024 * 1024 * 1024; // 10GB
        this.allowedTypes = options.allowedTypes || null; // null = all
        this.onUploadComplete = options.onUploadComplete || (() => {});
        this.onUploadError = options.onUploadError || (() => {});
        
        this.uploads = [];
        this.isUploading = false;
        
        this.init();
    }

    init() {
        this.createDropOverlay();
        this.createUploadPanel();
        this.bindEvents();
    }

    createDropOverlay() {
        this.dropOverlay = document.createElement('div');
        this.dropOverlay.className = 'fm-drop-overlay';
        this.dropOverlay.innerHTML = `
            <div class="fm-drop-content">
                <div class="fm-drop-icon">📤</div>
                <div class="fm-drop-title">拖放文件到这里</div>
                <div class="fm-drop-subtitle">松开鼠标开始上传</div>
            </div>
        `;
        document.body.appendChild(this.dropOverlay);
    }

    createUploadPanel() {
        this.uploadPanel = document.createElement('div');
        this.uploadPanel.className = 'fm-upload-panel';
        this.uploadPanel.innerHTML = `
            <div class="fm-upload-header">
                <h3>📤 上传队列</h3>
                <div style="display: flex; gap: 0.5rem;">
                    <button class="btn-fm btn-fm-secondary" onclick="fileUpload.clearCompleted()" style="padding: 0.25rem 0.5rem; font-size: 11px;">清除已完成</button>
                    <button class="btn-fm btn-fm-secondary" onclick="fileUpload.togglePanel()" style="padding: 0.25rem 0.5rem; font-size: 11px;">✕</button>
                </div>
            </div>
            <div class="fm-upload-body" id="upload-list"></div>
        `;
        document.body.appendChild(this.uploadPanel);
    }

    bindEvents() {
        // Prevent default drag behaviors
        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            document.body.addEventListener(eventName, (e) => {
                e.preventDefault();
                e.stopPropagation();
            });
        });

        // Show/hide drop overlay
        let dragCounter = 0;

        document.body.addEventListener('dragenter', (e) => {
            dragCounter++;
            if (e.dataTransfer.types.includes('Files')) {
                this.dropOverlay.classList.add('active');
            }
        });

        document.body.addEventListener('dragleave', (e) => {
            dragCounter--;
            if (dragCounter === 0) {
                this.dropOverlay.classList.remove('active');
            }
        });

        document.body.addEventListener('drop', (e) => {
            dragCounter = 0;
            this.dropOverlay.classList.remove('active');
            
            const files = e.dataTransfer.files;
            if (files.length > 0) {
                this.handleFiles(files);
            }
        });

        // File input change
        document.addEventListener('change', (e) => {
            if (e.target.type === 'file' && e.target.multiple) {
                this.handleFiles(e.target.files);
                e.target.value = '';
            }
        });
    }

    handleFiles(fileList) {
        const files = Array.from(fileList);
        
        // Validate files
        const validFiles = files.filter(file => {
            if (file.size > this.maxFileSize) {
                this.showToast(`${file.name} 超过最大文件大小限制`, 'error');
                return false;
            }
            if (this.allowedTypes && !this.isAllowedType(file.type)) {
                this.showToast(`${file.name} 文件类型不支持`, 'error');
                return false;
            }
            return true;
        });

        if (validFiles.length === 0) return;

        // Add to queue
        validFiles.forEach(file => {
            this.addToQueue(file);
        });

        // Start uploading
        this.uploadPanel.classList.add('active');
        this.processQueue();
    }

    addToQueue(file) {
        const upload = {
            id: Date.now() + Math.random(),
            file: file,
            name: file.name,
            size: file.size,
            progress: 0,
            status: 'pending', // pending, uploading, success, error
            error: null
        };
        this.uploads.push(upload);
        this.renderUploadItem(upload);
    }

    renderUploadItem(upload) {
        const list = document.getElementById('upload-list');
        if (!list) return;

        const item = document.createElement('div');
        item.className = 'fm-upload-item';
        item.id = `upload-${upload.id}`;
        item.innerHTML = `
            <div class="fm-upload-item-icon">${this.getFileIcon(upload.name)}</div>
            <div class="fm-upload-item-info">
                <div class="fm-upload-item-name">${this.escapeHtml(upload.name)}</div>
                <div class="fm-upload-progress">
                    <div class="fm-upload-progress-bar" style="width: 0%"></div>
                </div>
                <div class="fm-upload-item-status">等待中...</div>
            </div>
        `;
        list.appendChild(item);
    }

    async processQueue() {
        if (this.isUploading) return;
        
        this.isUploading = true;
        const pending = this.uploads.filter(u => u.status === 'pending');
        
        for (const upload of pending) {
            await this.uploadFile(upload);
        }
        
        this.isUploading = false;
    }

    async uploadFile(upload) {
        upload.status = 'uploading';
        this.updateUploadUI(upload, 0, '上传中...');

        const formData = new FormData();
        formData.append('file', upload.file);
        formData.append('path', this.currentPath);

        try {
            const xhr = new XMLHttpRequest();
            
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    const percent = Math.round((e.loaded / e.total) * 100);
                    this.updateUploadUI(upload, percent, `${percent}%`);
                }
            });

            xhr.addEventListener('load', () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    upload.status = 'success';
                    this.updateUploadUI(upload, 100, '✓ 完成');
                    this.onUploadComplete(upload);
                    this.showToast(`${upload.name} 上传成功`, 'success');
                } else {
                    throw new Error(`HTTP ${xhr.status}`);
                }
            });

            xhr.addEventListener('error', () => {
                throw new Error('网络错误');
            });

            xhr.open('POST', `${this.apiBase}/upload`);
            xhr.send(formData);

        } catch (error) {
            upload.status = 'error';
            upload.error = error.message;
            this.updateUploadUI(upload, 0, `✗ 失败: ${error.message}`);
            this.onUploadError(upload, error);
            this.showToast(`${upload.name} 上传失败`, 'error');
        }
    }

    updateUploadUI(upload, progress, statusText) {
        const item = document.getElementById(`upload-${upload.id}`);
        if (!item) return;

        const bar = item.querySelector('.fm-upload-progress-bar');
        const status = item.querySelector('.fm-upload-item-status');
        
        if (bar) bar.style.width = `${progress}%`;
        if (status) {
            status.textContent = statusText;
            status.className = 'fm-upload-item-status';
            if (upload.status === 'success') status.classList.add('success');
            if (upload.status === 'error') status.classList.add('error');
        }
    }

    clearCompleted() {
        this.uploads = this.uploads.filter(u => u.status !== 'success');
        const list = document.getElementById('upload-list');
        if (list) {
            list.querySelectorAll('.fm-upload-item').forEach(item => {
                const id = item.id.replace('upload-', '');
                const upload = this.uploads.find(u => u.id.toString() === id);
                if (!upload) item.remove();
            });
        }
    }

    togglePanel() {
        this.uploadPanel.classList.toggle('active');
    }

    setCurrentPath(path) {
        this.currentPath = path;
    }

    isAllowedType(mimeType) {
        if (!this.allowedTypes) return true;
        return this.allowedTypes.some(type => {
            if (type.endsWith('/*')) {
                return mimeType.startsWith(type.replace('/*', '/'));
            }
            return mimeType === type;
        });
    }

    getFileIcon(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        const iconMap = {
            // Images
            'jpg': '🖼️', 'jpeg': '🖼️', 'png': '🖼️', 'gif': '🖼️', 'webp': '🖼️', 'svg': '🖼️',
            // Videos
            'mp4': '🎬', 'avi': '🎬', 'mkv': '🎬', 'mov': '🎬', 'wmv': '🎬',
            // Audio
            'mp3': '🎵', 'wav': '🎵', 'flac': '🎵', 'aac': '🎵', 'ogg': '🎵',
            // Documents
            'pdf': '📄', 'doc': '📝', 'docx': '📝', 'xls': '📊', 'xlsx': '📊', 'ppt': '📊', 'pptx': '📊',
            // Archives
            'zip': '📦', 'rar': '📦', '7z': '📦', 'tar': '📦', 'gz': '📦',
            // Code
            'js': '💻', 'ts': '💻', 'py': '💻', 'java': '💻', 'cpp': '💻', 'html': '💻', 'css': '💻',
            'json': '💻', 'xml': '💻', 'yml': '💻', 'yaml': '💻', 'md': '📝'
        };
        return iconMap[ext] || '📄';
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
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
window.FileUploadManager = FileUploadManager;
