import React, { useState, useCallback, useRef, useEffect } from 'react';
import { FileItem, SearchParams, SearchResult } from '../types';

interface SearchProps {
  onSearch: (params: SearchParams) => Promise<SearchResult>;
  onSelect: (file: FileItem) => void;
  onClose?: () => void;
  placeholder?: string;
}

export const Search: React.FC<SearchProps> = ({
  onSearch,
  onSelect,
  onClose,
  placeholder = '搜索文件...',
}) => {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  
  const [filters, setFilters] = useState<{
    type?: 'file' | 'directory' | 'all';
    mimeType?: string;
  }>({ type: 'all' });
  
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  // 搜索
  const performSearch = useCallback(async (searchQuery: string) => {
    if (!searchQuery.trim()) {
      setResults([]);
      return;
    }

    setLoading(true);
    try {
      const searchResult = await onSearch({
        query: searchQuery,
        type: filters.type,
        mimeType: filters.mimeType,
      });
      setResults(searchResult.files);
      setSelectedIndex(-1);
    } catch {
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, [onSearch, filters]);

  // Debounce search
  useEffect(() => {
    const timer = setTimeout(() => {
      if (query.length >= 2) {
        performSearch(query);
      } else {
        setResults([]);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [query, performSearch]);

  // 键盘导航
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (!isOpen || results.length === 0) return;

    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex(prev => 
          prev < results.length - 1 ? prev + 1 : prev
        );
        break;
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex(prev => prev > 0 ? prev - 1 : -1);
        break;
      case 'Enter':
        e.preventDefault();
        if (selectedIndex >= 0 && selectedIndex < results.length) {
          onSelect(results[selectedIndex]);
          setIsOpen(false);
          setQuery('');
          setResults([]);
        }
        break;
      case 'Escape':
        e.preventDefault();
        setIsOpen(false);
        if (onClose) onClose();
        break;
    }
  }, [isOpen, results, selectedIndex, onSelect, onClose]);

  // 点击外部关闭
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // 获取文件图标
  const getFileIcon = (file: FileItem) => {
    if (file.isDirectory) {
      return (
        <svg className="w-5 h-5 text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
        </svg>
      );
    }

    const ext = file.extension.toLowerCase();
    const iconColors: Record<string, string> = {
      // 图片
      jpg: 'text-pink-400', jpeg: 'text-pink-400', png: 'text-pink-400', gif: 'text-pink-400',
      // 视频
      mp4: 'text-red-400', mov: 'text-red-400', avi: 'text-red-400', mkv: 'text-red-400',
      // 音频
      mp3: 'text-purple-400', wav: 'text-purple-400', flac: 'text-purple-400',
      // 文档
      pdf: 'text-red-500', doc: 'text-blue-400', docx: 'text-blue-400', xls: 'text-green-400',
      // 代码
      js: 'text-yellow-400', ts: 'text-blue-400', py: 'text-green-400', go: 'text-cyan-400',
    };

    const color = iconColors[ext] || 'text-gray-400';

    return (
      <svg className={`w-5 h-5 ${color}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
      </svg>
    );
  };

  return (
    <div ref={containerRef} className="relative w-full max-w-md">
      {/* 搜索输入框 */}
      <div className="relative">
        <input
          ref={inputRef}
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onFocus={() => setIsOpen(true)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="w-full px-4 py-2 pl-10 pr-12 bg-gray-800/80 border border-gray-700 rounded-lg text-white placeholder-gray-500 focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition-colors"
        />
        <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        
        {/* 过滤按钮 */}
        <button
          onClick={() => setShowFilters(!showFilters)}
          className={`absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors ${
            showFilters ? 'text-blue-400 bg-blue-400/20' : 'text-gray-500 hover:text-gray-400'
          }`}
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 6a2 2 0 100-4m0 4a2 2 0 110-4m12 0a2 2 0 100-4m0 4a2 2 0 110-4m-6 6v2m0-6a2 2 0 100-4m0 4a2 2 0 110-4" />
          </svg>
        </button>

        {/* 加载指示器 */}
        {loading && (
          <div className="absolute right-8 top-1/2 -translate-y-1/2">
            <svg className="w-4 h-4 animate-spin text-blue-400" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 017 12H3c0 2.042.847 3.913 2.203 5.291z" />
            </svg>
          </div>
        )}
      </div>

      {/* 过滤面板 */}
      {showFilters && (
        <div className="absolute top-full left-0 right-0 mt-2 p-3 bg-gray-800 border border-gray-700 rounded-lg shadow-xl z-20">
          <div className="flex items-center gap-4">
            <label className="text-sm text-gray-400">类型:</label>
            <select
              value={filters.type || 'all'}
              onChange={(e) => setFilters(prev => ({
                ...prev,
                type: e.target.value as 'file' | 'directory' | 'all'
              }))}
              className="px-2 py-1 bg-gray-700 border border-gray-600 rounded text-sm text-white"
            >
              <option value="all">全部</option>
              <option value="file">文件</option>
              <option value="directory">目录</option>
            </select>
          </div>
        </div>
      )}

      {/* 搜索结果下拉 */}
      {isOpen && results.length > 0 && (
        <div className="absolute top-full left-0 right-0 mt-2 bg-gray-800 border border-gray-700 rounded-lg shadow-xl max-h-80 overflow-y-auto z-10">
          {results.map((file, index) => (
            <button
              key={file.id}
              onClick={() => {
                onSelect(file);
                setIsOpen(false);
                setQuery('');
                setResults([]);
              }}
              className={`w-full flex items-center gap-3 px-4 py-3 text-left transition-colors ${
                index === selectedIndex ? 'bg-blue-500/20' : 'hover:bg-gray-700/50'
              }`}
            >
              {getFileIcon(file)}
              <div className="flex-1 min-w-0">
                <p className="text-white truncate">{file.name}</p>
                <p className="text-xs text-gray-500 truncate">{file.path}</p>
              </div>
              <span className="text-xs text-gray-500">
                {file.isDirectory ? '' : formatFileSize(file.size)}
              </span>
            </button>
          ))}
        </div>
      )}

      {/* 无结果提示 */}
      {isOpen && query.length >= 2 && !loading && results.length === 0 && (
        <div className="absolute top-full left-0 right-0 mt-2 p-4 bg-gray-800 border border-gray-700 rounded-lg shadow-xl text-center text-gray-500 z-10">
          未找到匹配的文件
        </div>
      )}
    </div>
  );
};

// 格式化文件大小
function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
}

export default Search;