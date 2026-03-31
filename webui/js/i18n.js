/**
 * NAS-OS Internationalization Module
 * Supports: zh-CN, en-US, ja-JP, ko-KR
 */

const I18n = {
  locale: 'zh-CN',
  translations: {},
  
  async init() {
    // 从 localStorage 或浏览器获取语言偏好
    const savedLocale = localStorage.getItem('nas-os-locale');
    const browserLocale = navigator.language || navigator.userLanguage;
    
    if (savedLocale && this.isSupported(savedLocale)) {
      this.locale = savedLocale;
    } else {
      // 自动检测浏览器语言
      this.locale = this.detectLocale(browserLocale);
    }
    
    await this.loadTranslations();
    this.applyTranslations();
    this.setupLanguageSelector();
  },
  
  isSupported(locale) {
    return ['zh-CN', 'en-US', 'ja-JP', 'ko-KR'].includes(locale);
  },
  
  detectLocale(browserLocale) {
    // 映射浏览器语言到支持的语言
    const localeMap = {
      'zh': 'zh-CN',
      'zh-CN': 'zh-CN',
      'zh-Hans': 'zh-CN',
      'zh-TW': 'zh-CN',
      'zh-Hant': 'zh-CN',
      'en': 'en-US',
      'en-US': 'en-US',
      'en-GB': 'en-US',
      'ja': 'ja-JP',
      'ja-JP': 'ja-JP',
      'ko': 'ko-KR',
      'ko-KR': 'ko-KR'
    };
    return localeMap[browserLocale] || 'zh-CN';
  },
  
  async loadTranslations() {
    try {
      const response = await fetch(`i18n/${this.locale}.json`);
      this.translations = await response.json();
    } catch (error) {
      console.error('[i18n] 加载翻译失败:', error);
      // 尝试加载默认语言
      if (this.locale !== 'zh-CN') {
        try {
          const fallback = await fetch('i18n/zh-CN.json');
          this.translations = await fallback.json();
        } catch (e) {
          console.error('[i18n] 加载默认翻译失败:', e);
        }
      }
    }
  },
  
  t(key, params = {}) {
    const keys = key.split('.');
    let value = this.translations;
    
    for (const k of keys) {
      if (value && typeof value === 'object' && k in value) {
        value = value[k];
      } else {
        console.warn(`[i18n] 翻译缺失: ${key}`);
        return key;
      }
    }
    
    if (typeof value !== 'string') {
      return key;
    }
    
    // 替换参数
    return value.replace(/\{\{(\w+)\}\}/g, (_, param) => params[param] || '');
  },
  
  applyTranslations() {
    // 翻译所有带有 data-i18n 属性的元素
    document.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.getAttribute('data-i18n');
      el.textContent = this.t(key);
    });
    
    // 翻译 placeholder
    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
      const key = el.getAttribute('data-i18n-placeholder');
      el.placeholder = this.t(key);
    });
    
    // 翻译 title
    document.querySelectorAll('[data-i18n-title]').forEach(el => {
      const key = el.getAttribute('data-i18n-title');
      el.title = this.t(key);
    });
    
    // 更新 HTML lang 属性
    document.documentElement.lang = this.locale;
  },
  
  setLocale(locale) {
    if (this.isSupported(locale)) {
      this.locale = locale;
      localStorage.setItem('nas-os-locale', locale);
      this.loadTranslations().then(() => this.applyTranslations());
    }
  },
  
  setupLanguageSelector() {
    const selector = document.getElementById('language-selector');
    if (selector) {
      selector.value = this.locale;
      selector.addEventListener('change', (e) => {
        this.setLocale(e.target.value);
      });
    }
  },
  
  // 获取所有支持的语言列表
  getSupportedLocales() {
    return [
      { code: 'zh-CN', name: '简体中文', native: '简体中文' },
      { code: 'en-US', name: 'English', native: 'English' },
      { code: 'ja-JP', name: 'Japanese', native: '日本語' },
      { code: 'ko-KR', name: 'Korean', native: '한국어' }
    ];
  },
  
  // 格式化日期
  formatDate(date, options = {}) {
    const defaultOptions = { year: 'numeric', month: 'short', day: 'numeric' };
    try {
      return new Intl.DateTimeFormat(this.locale, { ...defaultOptions, ...options }).format(date);
    } catch {
      return date.toLocaleDateString();
    }
  },
  
  // 格式化数字
  formatNumber(number, options = {}) {
    try {
      return new Intl.NumberFormat(this.locale, options).format(number);
    } catch {
      return number.toString();
    }
  },
  
  // 格式化相对时间
  formatRelativeTime(date) {
    const now = new Date();
    const diff = now - date;
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);
    
    if (days > 0) return this.t('common.daysAgo', { count: days });
    if (hours > 0) return this.t('common.hoursAgo', { count: hours });
    if (minutes > 0) return this.t('common.minutesAgo', { count: minutes });
    return this.t('common.justNow');
  }
};

// 导出全局对象
window.I18n = I18n;

// 初始化
document.addEventListener('DOMContentLoaded', () => {
  I18n.init();
});