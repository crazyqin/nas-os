import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useSearch } from '../hooks/useSearch';

interface SearchResult {
  id: string;
  type: 'file' | 'user' | 'setting' | 'api' | 'container' | 'share';
  title: string;
  description: string;
  path?: string;
  icon?: string;
  metadata?: Record<string, string | number>;
}

// Type icons and colors
const typeConfig: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
  file: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
      </svg>
    ),
    color: 'text-blue-400 bg-blue-500/20',
    label: '文件'
  },
  user: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
      </svg>
    ),
    color: 'text-green-400 bg-green-500/20',
    label: '用户'
  },
  setting: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
      </svg>
    ),
    color: 'text-purple-400 bg-purple-500/20',
    label: '设置'
  },
  api: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
      </svg>
    ),
    color: 'text-yellow-400 bg-yellow-500/20',
    label: 'API'
  },
  container: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
      </svg>
    ),
    color: 'text-cyan-400 bg-cyan-500/20',
    label: '容器'
  },
  share: {
    icon: (
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 000-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
      </svg>
    ),
    color: 'text-orange-400 bg-orange-500/20',
    label: '共享'
  }
};

// Type filter buttons
const typeFilters = [
  { type: 'all', label: '全部' },
  { type: 'file', label: '文件' },
  { type: 'user', label: '用户' },
  { type: 'setting', label: '设置' },
  { type: 'api', label: 'API' },
  { type: 'container', label: '容器' },
  { type: 'share', label: '共享' }
];

// Recent searches (mock data - could be persisted)
const recentSearches = ['storage', 'docker', 'backup', 'users'];

export const GlobalSearch: React.FC = () => {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [selectedType, setSelectedType] = useState('all');
  const inputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);
  
  const { results, loading, search, clear } = useSearch();

  // Debounced search
  const debouncedSearch = useCallback(
    debounce((query: string, type?: string) => {
      if (query.trim()) {
        search(query, type === 'all' ? undefined : type);
      }
    }, 300),
    [search]
  );

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setInputValue(value);
    setSelectedIndex(0);
    
    if (value.trim()) {
      debouncedSearch(value, selectedType);
    } else {
      clear();
    }
  };

  // Handle keyboard navigation
  const handleKeyDown = (e: React.KeyboardEvent) => {
    const totalItems = results.length || recentSearches.length;
    
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setSelectedIndex(prev => Math.min(prev + 1, totalItems - 1));
        break;
      
      case 'ArrowUp':
        e.preventDefault();
        setSelectedIndex(prev => Math.max(prev - 1, 0));
        break;
      
      case 'Enter':
        e.preventDefault();
        if (results.length > 0 && selectedIndex < results.length) {
          handleSelectResult(results[selectedIndex]);
        } else if (!inputValue.trim() && selectedIndex < recentSearches.length) {
          // Select recent search
          setInputValue(recentSearches[selectedIndex]);
          search(recentSearches[selectedIndex], selectedType === 'all' ? undefined : selectedType);
        }
        break;
      
      case 'Escape':
        e.preventDefault();
        setIsOpen(false);
        break;
    }
  };

  // Handle result selection
  const handleSelectResult = (result: SearchResult) => {
    setIsOpen(false);
    setInputValue('');
    clear();
    
    // Navigate to result or trigger action
    if (result.path) {
      // Could use router or emit event
      console.log('Navigate to:', result.path);
    }
  };

  // Handle type filter change
  const handleTypeChange = (type: string) => {
    setSelectedType(type);
    setSelectedIndex(0);
    
    if (inputValue.trim()) {
      search(inputValue, type === 'all' ? undefined : type);
    }
  };

  // Open search modal with keyboard shortcut
  useEffect(() => {
    const handleGlobalKeyDown = (e: KeyboardEvent) => {
      // Cmd/Ctrl + K
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        setIsOpen(true);
      }
    };

    window.addEventListener('keydown', handleGlobalKeyDown);
    return () => window.removeEventListener('keydown', handleGlobalKeyDown);
  }, []);

  // Focus input when opened
  useEffect(() => {
    if (isOpen && inputRef.current) {
      inputRef.current.focus();
    }
  }, [isOpen]);

  // Scroll selected item into view
  useEffect(() => {
    if (resultsRef.current) {
      const selectedElement = resultsRef.current.children[selectedIndex] as HTMLElement;
      if (selectedElement) {
        selectedElement.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedIndex]);

  return (
    <>
      {/* Trigger button (for reference - usually hidden or integrated) */}
      <button
        onClick={() => setIsOpen(true)}
        className="flex items-center gap-2 px-3 py-2 bg-gray-800 border border-gray-700 rounded-lg hover:border-gray-600 transition-colors group"
      >
        <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <span className="text-sm text-gray-400">搜索...</span>
        <kbd className="hidden md:inline-flex px-1.5 py-0.5 text-xs text-gray-500 bg-gray-900 rounded border border-gray-700">
          {navigator.platform.includes('Mac') ? '⌘K' : 'Ctrl+K'}
        </kbd>
      </button>

      {/* Modal */}
      {isOpen && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* Backdrop */}
          <div 
            className="fixed inset-0 bg-black/60 backdrop-blur-sm"
            onClick={() => setIsOpen(false)}
          />

          {/* Modal content */}
          <div className="relative max-w-2xl mx-auto mt-[15vh] px-4">
            <div className="bg-gray-900 rounded-xl border border-gray-700 shadow-2xl overflow-hidden">
              {/* Search input */}
              <div className="flex items-center gap-3 px-4 py-3 border-b border-gray-700">
                <svg className="w-5 h-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                
                <input
                  ref={inputRef}
                  type="text"
                  value={inputValue}
                  onChange={handleInputChange}
                  onKeyDown={handleKeyDown}
                  placeholder="搜索文件、用户、设置..."
                  className="w-full bg-transparent text-white text-lg placeholder-gray-500 outline-none"
                />

                {loading && (
                  <div className="animate-spin w-5 h-5 border-2 border-gray-400 border-t-transparent rounded-full" />
                )}

                <kbd className="px-1.5 py-0.5 text-xs text-gray-500 bg-gray-800 rounded border border-gray-700">
                  ESC
                </kbd>
              </div>

              {/* Type filters */}
              <div className="flex gap-1 px-4 py-2 border-b border-gray-800">
                {typeFilters.map(filter => (
                  <button
                    key={filter.type}
                    onClick={() => handleTypeChange(filter.type)}
                    className={`px-2.5 py-1 text-xs rounded-full transition-colors ${
                      selectedType === filter.type
                        ? 'bg-blue-500/20 text-blue-400 border border-blue-500/30'
                        : 'text-gray-500 hover:text-gray-400 hover:bg-gray-800'
                    }`}
                  >
                    {filter.label}
                  </button>
                ))}
              </div>

              {/* Results */}
              <div 
                ref={resultsRef}
                className="max-h-[400px] overflow-y-auto py-2"
              >
                {results.length > 0 ? (
                  // Search results
                  <div className="space-y-1 px-2">
                    {results.map((result, index) => {
                      const config = typeConfig[result.type] || typeConfig.file;
                      const isSelected = index === selectedIndex;
                      
                      return (
                        <button
                          key={result.id}
                          onClick={() => handleSelectResult(result)}
                          className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg transition-colors ${
                            isSelected
                              ? 'bg-blue-500/20 text-blue-400'
                              : 'hover:bg-gray-800 text-gray-300'
                          }`}
                        >
                          {/* Type icon */}
                          <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${config.color}`}>
                            {config.icon}
                          </div>
                          
                          {/* Content */}
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2">
                              <span className={`font-medium truncate ${isSelected ? 'text-blue-300' : 'text-gray-200'}`}>
                                {result.title}
                              </span>
                              <span className={`text-xs px-1.5 py-0.5 rounded ${config.color}`}>
                                {config.label}
                              </span>
                            </div>
                            <p className={`text-sm truncate ${isSelected ? 'text-blue-400/70' : 'text-gray-500'}`}>
                              {result.description}
                            </p>
                          </div>
                          
                          {/* Path/shortcut */}
                          {result.path && (
                            <span className="text-xs text-gray-500">
                              {result.path}
                            </span>
                          )}
                          
                          {/* Keyboard indicator */}
                          {isSelected && (
                            <kbd className="px-1.5 py-0.5 text-xs text-gray-400 bg-gray-800 rounded border border-gray-700">
                              Enter
                            </kbd>
                          )}
                        </button>
                      );
                    })}
                  </div>
                ) : inputValue.trim() && !loading ? (
                  // No results
                  <div className="text-center py-8 text-gray-500">
                    <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <p>未找到相关结果</p>
                    <p className="text-sm mt-1">尝试不同的关键词或分类</p>
                  </div>
                ) : !inputValue.trim() ? (
                  // Recent searches
                  <div className="px-4">
                    <h3 className="text-sm font-medium text-gray-400 mb-2">最近搜索</h3>
                    <div className="space-y-1">
                      {recentSearches.map((term, index) => (
                        <button
                          key={term}
                          onClick={() => {
                            setInputValue(term);
                            search(term, selectedType === 'all' ? undefined : selectedType);
                          }}
                          className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
                            index === selectedIndex
                              ? 'bg-gray-800 text-blue-400'
                              : 'hover:bg-gray-800 text-gray-300'
                          }`}
                        >
                          <svg className="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                          </svg>
                          <span>{term}</span>
                        </button>
                      ))}
                    </div>
                    
                    {/* Quick actions */}
                    <div className="mt-4 pt-4 border-t border-gray-800">
                      <h3 className="text-sm font-medium text-gray-400 mb-2">快捷操作</h3>
                      <div className="grid grid-cols-2 gap-2">
                        {[
                          { label: '存储管理', path: '/storage', icon: 'database' },
                          { label: '用户设置', path: '/users', icon: 'users' },
                          { label: '容器列表', path: '/containers', icon: 'container' },
                          { label: '系统状态', path: '/system', icon: 'monitor' }
                        ].map(action => (
                          <button
                            key={action.path}
                            onClick={() => setIsOpen(false)}
                            className="flex items-center gap-2 px-3 py-2 bg-gray-800/50 hover:bg-gray-800 rounded-lg text-sm text-gray-300 transition-colors"
                          >
                            <svg className="w-4 h-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                            </svg>
                            {action.label}
                          </button>
                        ))}
                      </div>
                    </div>
                  </div>
                ) : null}
              </div>

              {/* Footer */}
              <div className="flex items-center justify-between px-4 py-2 border-t border-gray-800 text-xs text-gray-500">
                <div className="flex gap-2">
                  <kbd className="px-1 bg-gray-800 rounded">↑↓</kbd>
                  <span>导航</span>
                  <kbd className="px-1 bg-gray-800 rounded">Enter</kbd>
                  <span>选择</span>
                </div>
                <div className="flex gap-2">
                  <span>{results.length} 结果</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  );
};

// Debounce utility
function debounce<T extends (...args: any[]) => any>(
  func: T,
  wait: number
): (...args: Parameters<T>) => void {
  let timeout: NodeJS.Timeout;
  return (...args: Parameters<T>) => {
    clearTimeout(timeout);
    timeout = setTimeout(() => func(...args), wait);
  };
}

export default GlobalSearch;