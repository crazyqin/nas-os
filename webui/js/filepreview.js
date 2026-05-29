/**
 * NAS-OS File Preview Manager
 * Support: Images, Videos, PDFs, Audio, Text
 */

class FilePreviewManager {
    constructor(options = {}) {
        this.apiBase = options.apiBase || '/api/v1/files';
        this.previewableExtensions = {
            image: ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'],
            video: ['mp4', 'webm', 'ogg', 'mov', 'avi', 'mkv'],
            audio: ['mp3', 'wav', 'ogg', 'flac', 'aac', 'm4a'],
            pdf: ['pdf'],
            text: ['txt', 'md', 'json', 'xml', 'yml', 'yaml', 'csv', 'log', 'js', 'ts', 'py', 'java', 'cpp', 'c', 'h', 'html', 'css', 'sh', 'bash']
        };
        
        this.currentPreview = null;
        this.previewList = [];
        this.currentIndex = -1;
        
        this.init();
    }

    init() {
        this.createPreviewModal();
        this.bindKeyboardEvents();
    }

    createPreviewModal() {
        this.modal = document.createElement('div');
        this.modal.className = 'fm-preview-modal';
        this.modal.innerHTML = `
            <div class="fm-preview-container">
                <button class="fm-preview-close" onclick="filePreview.close()">✕</button>
                <button class="fm-preview-nav prev" onclick="filePreview.prev()">◀</button>
                <button class="fm-preview-nav next" onclick="filePreview.next()">▶</button>
                <div id="preview-content"></div>
                <div class="fm-preview-info" id="preview-info"></div>
            </div>
        `;
        
        // Click outside to close
        this.modal.addEventListener('click', (e) => {
            if (e.target === this.modal) {
                this.close();
            }
        });
        
        document.body.appendChild(this.modal);
    }

    bindKeyboardEvents() {
        document.addEventListener('keydown', (e) => {
            if (!this.modal.classList.contains('active')) return;
            
            switch(e.key) {
                case 'Escape':
                    this.close();
                    break;
                case 'ArrowLeft':
                    this.prev();
                    break;
                case 'ArrowRight':
                    this.next();
                    break;
            }
        });
    }

    canPreview(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return Object.values(this.previewableExtensions).some(exts => exts.includes(ext));
    }

    getFileType(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        for (const [type, extensions] of Object.entries(this.previewableExtensions)) {
            if (extensions.includes(ext)) return type;
        }
        return 'unknown';
    }

    async open(file, fileList = []) {
        this.previewList = fileList.length > 0 ? fileList : [file];
        this.currentIndex = this.previewList.findIndex(f => f.path === file.path || f.name === file.name);
        if (this.currentIndex === -1) this.currentIndex = 0;
        
        await this.showPreview(this.previewList[this.currentIndex]);
        this.modal.classList.add('active');
        document.body.style.overflow = 'hidden';
    }

    async showPreview(file) {
        const content = document.getElementById('preview-content');
        const info = document.getElementById('preview-info');
        const fileType = this.getFileType(file.name);
        const fileUrl = file.url || `${this.apiBase}/preview?path=${encodeURIComponent(file.path)}`;
        
        // Update navigation visibility
        const prevBtn = this.modal.querySelector('.prev');
        const nextBtn = this.modal.querySelector('.next');
        if (prevBtn) prevBtn.style.display = this.previewList.length > 1 ? 'flex' : 'none';
        if (nextBtn) nextBtn.style.display = this.previewList.length > 1 ? 'flex' : 'none';

        // Clear previous content
        content.innerHTML = '';
        
        // Show loading
        content.innerHTML = '<div class="fm-loading"><div class="fm-spinner"></div></div>';

        try {
            switch(fileType) {
                case 'image':
                    await this.renderImage(content, fileUrl, file);
                    break;
                case 'video':
                    this.renderVideo(content, fileUrl, file);
                    break;
                case 'audio':
                    this.renderAudio(content, fileUrl, file);
                    break;
                case 'pdf':
                    this.renderPDF(content, fileUrl, file);
                    break;
                case 'text':
                    await this.renderText(content, fileUrl, file);
                    break;
                default:
                    this.renderUnsupported(content, file);
            }
        } catch (error) {
            content.innerHTML = `
                <div style="text-align: center; color: white; padding: 2rem;">
                    <div style="font-size: 48px; margin-bottom: 1rem;">⚠️</div>
                    <div>预览加载失败</div>
                    <div style="font-size: 14px; opacity: 0.7; margin-top: 0.5rem;">${error.message}</div>
                </div>
            `;
        }

        // Update info
        if (info) {
            info.textContent = `${file.name} (${this.formatSize(file.size)})`;
        }

        this.currentPreview = file;
    }

    renderImage(container, url, file) {
        return new Promise((resolve, reject) => {
            const img = new Image();
            img.className = 'fm-preview-image';
            img.alt = file.name;
            
            img.onload = () => {
                container.innerHTML = '';
                container.appendChild(img);
                resolve();
            };
            
            img.onerror = () => {
                reject(new Error('图片加载失败'));
            };
            
            img.src = url;
        });
    }

    renderVideo(container, url, file) {
        container.innerHTML = `
            <video class="fm-preview-video" controls autoplay>
                <source src="${url}" type="video/${file.name.split('.').pop()}">
                您的浏览器不支持视频播放
            </video>
        `;
    }

    renderAudio(container, url, file) {
        container.innerHTML = `
            <div class="fm-preview-audio">
                <div style="font-size: 64px; margin-bottom: 1rem;">🎵</div>
                <div style="font-size: 18px; font-weight: 600; margin-bottom: 1.5rem; color: var(--fm-text);">${this.escapeHtml(file.name)}</div>
                <audio controls autoplay style="width: 100%;">
                    <source src="${url}" type="audio/${file.name.split('.').pop()}">
                    您的浏览器不支持音频播放
                </audio>
            </div>
        `;
    }

    renderPDF(container, url, file) {
        container.innerHTML = `
            <iframe class="fm-preview-pdf" src="${url}" title="${this.escapeHtml(file.name)}"></iframe>
        `;
    }

    async renderText(container, url, file) {
        try {
            const response = await fetch(url);
            const text = await response.text();
            
            container.innerHTML = `
                <div style="
                    background: #1e1e1e;
                    color: #d4d4d4;
                    padding: 1.5rem;
                    border-radius: var(--fm-radius);
                    max-width: 80vw;
                    max-height: 80vh;
                    overflow: auto;
                    font-family: 'Consolas', 'Monaco', monospace;
                    font-size: 14px;
                    line-height: 1.6;
                    white-space: pre-wrap;
                    word-wrap: break-word;
                ">${this.escapeHtml(text)}</div>
            `;
        } catch (error) {
            throw new Error('文本加载失败');
        }
    }

    renderUnsupported(container, file) {
        container.innerHTML = `
            <div style="text-align: center; color: white; padding: 2rem;">
                <div style="font-size: 64px; margin-bottom: 1rem;">📄</div>
                <div style="font-size: 18px; font-weight: 600; margin-bottom: 0.5rem;">${this.escapeHtml(file.name)}</div>
                <div style="font-size: 14px; opacity: 0.7;">此文件类型不支持预览</div>
                <button class="btn-fm btn-fm-primary" style="margin-top: 1rem;" onclick="filePreview.download('${this.escapeHtml(file.path)}')">
                    ⬇️ 下载文件
                </button>
            </div>
        `;
    }

    close() {
        this.modal.classList.remove('active');
        document.body.style.overflow = '';
        
        // Stop video/audio
        const video = this.modal.querySelector('video');
        const audio = this.modal.querySelector('audio');
        if (video) video.pause();
        if (audio) audio.pause();
        
        this.currentPreview = null;
    }

    async prev() {
        if (this.previewList.length <= 1) return;
        this.currentIndex = (this.currentIndex - 1 + this.previewList.length) % this.previewList.length;
        await this.showPreview(this.previewList[this.currentIndex]);
    }

    async next() {
        if (this.previewList.length <= 1) return;
        this.currentIndex = (this.currentIndex + 1) % this.previewList.length;
        await this.showPreview(this.previewList[this.currentIndex]);
    }

    download(filePath) {
        const a = document.createElement('a');
        a.href = `${this.apiBase}/download?path=${encodeURIComponent(filePath)}`;
        a.download = filePath.split('/').pop();
        a.click();
    }

    formatSize(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text || '';
        return div.innerHTML;
    }

    generateThumbnail(file) {
        return new Promise((resolve) => {
            if (!this.previewableExtensions.image.includes(file.name.split('.').pop().toLowerCase())) {
                resolve(null);
                return;
            }

            const reader = new FileReader();
            reader.onload = (e) => {
                const img = new Image();
                img.onload = () => {
                    const canvas = document.createElement('canvas');
                    const ctx = canvas.getContext('2d');
                    const size = 64;
                    
                    canvas.width = size;
                    canvas.height = size;
                    
                    const scale = Math.max(size / img.width, size / img.height);
                    const x = (size - img.width * scale) / 2;
                    const y = (size - img.height * scale) / 2;
                    
                    ctx.drawImage(img, x, y, img.width * scale, img.height * scale);
                    resolve(canvas.toDataURL('image/jpeg', 0.7));
                };
                img.src = e.target.result;
            };
            reader.readAsDataURL(file);
        });
    }
}

// Export
window.FilePreviewManager = FilePreviewManager;
