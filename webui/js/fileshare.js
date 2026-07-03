/**
 * NAS-OS File Share Manager
 * Generate share links with password and expiration
 */

class FileShareManager {
    constructor(options = {}) {
        this.apiBase = options.apiBase || '/api/v1/shares';
        this.baseUrl = options.baseUrl || window.location.origin;
        this.defaultExpiry = options.defaultExpiry || 24; // hours
        this.onShareCreated = options.onShareCreated || (() => {});
        
        this.init();
    }

    init() {
        this.createShareModal();
    }

    createShareModal() {
        // Create share dialog
        this.shareModal = document.createElement('div');
        this.shareModal.className = 'fm-share-modal';
        this.shareModal.innerHTML = `
            <div class="fm-share-dialog">
                <h3 class="fm-share-title">🔗 生成分享链接</h3>
                
                <div class="fm-share-field">
                    <label class="fm-share-label">分享文件</label>
                    <input type="text" class="fm-share-input" id="share-file-name" readonly>
                </div>

                <div class="fm-share-field">
                    <label class="fm-share-label">链接有效期</label>
                    <select class="fm-share-select" id="share-expiry">
                        <option value="1">1 小时</option>
                        <option value="6">6 小时</option>
                        <option value="24" selected>24 小时</option>
                        <option value="72">3 天</option>
                        <option value="168">7 天</option>
                        <option value="720">30 天</option>
                        <option value="0">永久有效</option>
                    </select>
                </div>

                <div class="fm-share-field">
                    <label class="fm-share-label">
                        <input type="checkbox" id="share-use-password" onchange="fileShare.togglePassword()"> 
                        设置访问密码
                    </label>
                    <div id="share-password-field" style="display: none; margin-top: 0.5rem;">
                        <div style="display: flex; gap: 0.5rem;">
                            <input type="text" class="fm-share-input" id="share-password" placeholder="输入密码或留空自动生成">
                            <button class="btn-fm btn-fm-secondary" onclick="fileShare.generatePassword()">🎲</button>
                        </div>
                    </div>
                </div>

                <div class="fm-share-field">
                    <label class="fm-share-label">
                        <input type="checkbox" id="share-allow-download" checked> 
                        允许下载
                    </label>
                </div>

                <div class="fm-share-field">
                    <label class="fm-share-label">
                        <input type="checkbox" id="share-allow-preview" checked> 
                        允许在线预览
                    </label>
                </div>

                <!-- Share Result (hidden initially) -->
                <div id="share-result" style="display: none;">
                    <div class="fm-share-result">
                        <div style="font-size: 13px; color: var(--fm-text-secondary); margin-bottom: 0.75rem;">分享链接</div>
                        <div class="fm-share-link-row">
                            <input type="text" class="fm-share-link-input" id="share-link-url" readonly>
                            <button class="btn-fm btn-fm-primary" onclick="fileShare.copyLink()">📋 复制</button>
                        </div>
                        <div id="share-password-display" style="display: none;">
                            <div style="font-size: 13px; color: var(--fm-text-secondary); margin-bottom: 0.5rem;">访问密码</div>
                            <div class="fm-share-link-row">
                                <input type="text" class="fm-share-link-input" id="share-link-password" readonly>
                                <button class="btn-fm btn-fm-secondary" onclick="fileShare.copyPassword()">📋</button>
                            </div>
                        </div>
                        <div style="font-size: 12px; color: var(--fm-text-muted); margin-top: 0.75rem;" id="share-expiry-info"></div>
                    </div>
                </div>

                <div class="fm-share-actions">
                    <button class="btn-fm btn-fm-secondary" onclick="fileShare.close()">取消</button>
                    <button class="btn-fm btn-fm-primary" id="share-create-btn" onclick="fileShare.createShare()">生成链接</button>
                    <button class="btn-fm btn-fm-primary" id="share-done-btn" style="display: none;" onclick="fileShare.close()">完成</button>
                </div>
            </div>
        `;

        // Click outside to close
        this.shareModal.addEventListener('click', (e) => {
            if (e.target === this.shareModal) {
                this.close();
            }
        });

        document.body.appendChild(this.shareModal);
    }

    open(file) {
        this.currentFile = file;
        
        // Reset form
        document.getElementById('share-file-name').value = file.name;
        document.getElementById('share-expiry').value = this.defaultExpiry;
        document.getElementById('share-use-password').checked = false;
        document.getElementById('share-password-field').style.display = 'none';
        document.getElementById('share-password').value = '';
        document.getElementById('share-allow-download').checked = true;
        document.getElementById('share-allow-preview').checked = true;
        
        // Hide result
        document.getElementById('share-result').style.display = 'none';
        document.getElementById('share-create-btn').style.display = '';
        document.getElementById('share-done-btn').style.display = 'none';
        
        this.shareModal.classList.add('active');
    }

    close() {
        this.shareModal.classList.remove('active');
        this.currentFile = null;
    }

    togglePassword() {
        const usePassword = document.getElementById('share-use-password').checked;
        document.getElementById('share-password-field').style.display = usePassword ? 'block' : 'none';
        
        if (usePassword && !document.getElementById('share-password').value) {
            this.generatePassword();
        }
    }

    secureRandomString(length, chars) {
        const cryptoObj = window.crypto || window.msCrypto;
        if (!cryptoObj || !cryptoObj.getRandomValues) {
            throw new Error('当前浏览器不支持安全随机数生成');
        }
        const values = new Uint32Array(length);
        cryptoObj.getRandomValues(values);
        return Array.from(values, value => chars[value % chars.length]).join('');
    }

    generatePassword() {
        const chars = 'ABCDEFGHJKMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789';
        document.getElementById('share-password').value = this.secureRandomString(12, chars);
    }

    async createShare() {
        if (!this.currentFile) return;

        const expiryHours = parseInt(document.getElementById('share-expiry').value);
        const usePassword = document.getElementById('share-use-password').checked;
        const password = usePassword ? document.getElementById('share-password').value : null;
        const allowDownload = document.getElementById('share-allow-download').checked;
        const allowPreview = document.getElementById('share-allow-preview').checked;

        // Generate a unique share ID (in real app, this would be from API)
        const shareId = this.generateShareId();
        
        // Calculate expiry
        let expiresAt = null;
        if (expiryHours > 0) {
            expiresAt = new Date(Date.now() + expiryHours * 3600000).toISOString();
        }

        // Build share URL
        const shareUrl = `${this.baseUrl}/share/${shareId}`;

        // Create share object
        const shareData = {
            id: shareId,
            file: this.currentFile,
            url: shareUrl,
            password: password,
            expiresAt: expiresAt,
            allowDownload: allowDownload,
            allowPreview: allowPreview,
            createdAt: new Date().toISOString()
        };

        // In real app, send to API
        try {
            // await this.saveShareToAPI(shareData);
            
            // Show result
            this.showResult(shareData);
            this.onShareCreated(shareData);
            this.showToast('分享链接已生成', 'success');
        } catch (error) {
            this.showToast('生成分享链接失败', 'error');
        }
    }

    showResult(shareData) {
        document.getElementById('share-link-url').value = shareData.url;
        
        // Show/hide password
        const passwordDisplay = document.getElementById('share-password-display');
        if (shareData.password) {
            passwordDisplay.style.display = 'block';
            document.getElementById('share-link-password').value = shareData.password;
        } else {
            passwordDisplay.style.display = 'none';
        }

        // Show expiry info
        const expiryInfo = document.getElementById('share-expiry-info');
        if (shareData.expiresAt) {
            const expiryDate = new Date(shareData.expiresAt);
            expiryInfo.textContent = `有效期至: ${expiryDate.toLocaleString('zh-CN')}`;
        } else {
            expiryInfo.textContent = '永久有效';
        }

        // Toggle buttons
        document.getElementById('share-result').style.display = 'block';
        document.getElementById('share-create-btn').style.display = 'none';
        document.getElementById('share-done-btn').style.display = '';
    }

    copyLink() {
        const input = document.getElementById('share-link-url');
        input.select();
        document.execCommand('copy');
        this.showToast('链接已复制到剪贴板', 'success');
    }

    copyPassword() {
        const input = document.getElementById('share-link-password');
        input.select();
        document.execCommand('copy');
        this.showToast('密码已复制到剪贴板', 'success');
    }

    generateShareId() {
        const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
        return this.secureRandomString(24, chars);
    }

    showToast(message, type = 'info') {
        const toast = document.createElement('div');
        toast.className = `fm-toast ${type}`;
        toast.textContent = message;
        document.body.appendChild(toast);
        setTimeout(() => toast.remove(), 3000);
    }

    // Get list of active shares
    async getShares() {
        try {
            // In real app, fetch from API
            // const response = await fetch(`${this.apiBase}/list`);
            // return await response.json();
            return [];
        } catch (error) {
            console.error('Failed to load shares:', error);
            return [];
        }
    }

    // Delete a share
    async deleteShare(shareId) {
        try {
            // In real app, delete via API
            // await fetch(`${this.apiBase}/${shareId}`, { method: 'DELETE' });
            this.showToast('分享链接已删除', 'success');
            return true;
        } catch (error) {
            this.showToast('删除分享链接失败', 'error');
            return false;
        }
    }
}

// Export
window.FileShareManager = FileShareManager;
