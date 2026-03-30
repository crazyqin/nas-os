import { useState, useEffect, useCallback } from 'react';
import {
  FileItem,
  FileListResponse,
  SortConfig,
  ViewMode,
} from '../types';

interface UseFileListResult {
  files: FileItem[];
  loading: boolean;
  error: Error | null;
  currentPath: string;
  total: number;
  hasMore: boolean;
  viewMode: ViewMode;
  sortConfig: SortConfig;
  selectedFiles: Set<string>;
  
  // Actions
  navigateTo: (path: string) => Promise<void>;
  goBack: () => void;
  refresh: () => Promise<void>;
  setViewMode: (mode: ViewMode) => void;
  setSortConfig: (config: SortConfig) => void;
  toggleSelect: (fileId: string) => void;
  selectAll: () => void;
  clearSelection: () => void;
  deleteSelected: () => Promise<void>;
  loadMore: () => Promise<void>;
}

const PAGE_SIZE = 50;

export function useFileList(initialPath: string = '/'): UseFileListResult {
  const [files, setFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);
  const [currentPath, setCurrentPath] = useState(initialPath);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>(() => {
    if (typeof window !== 'undefined') {
      return (localStorage.getItem('webshare-view-mode') as ViewMode) || 'grid';
    }
    return 'grid';
  });
  const [sortConfig, setSortConfig] = useState<SortConfig>({
    field: 'name',
    order: 'asc',
  });
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());
  const [pathHistory, setPathHistory] = useState<string[]>([initialPath]);

  // 获取文件列表
  const fetchFiles = useCallback(async (
    path: string,
    pageNum: number = 1,
    append: boolean = false
  ) => {
    setLoading(true);
    setError(null);

    try {
      const url = new URL('/api/v1/webshare/files', window.location.origin);
      url.searchParams.set('path', path);
      url.searchParams.set('page', String(pageNum));
      url.searchParams.set('pageSize', String(PAGE_SIZE));
      url.searchParams.set('sortField', sortConfig.field);
      url.searchParams.set('sortOrder', sortConfig.order);

      const response = await fetch(url.toString());
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const data: FileListResponse = await response.json();
      
      if (append) {
        setFiles(prev => [...prev, ...data.files]);
      } else {
        setFiles(data.files);
      }
      setTotal(data.total);
      setHasMore(data.hasMore);
      setPage(pageNum);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to fetch files'));
    } finally {
      setLoading(false);
    }
  }, [sortConfig]);

  // 导航到指定路径
  const navigateTo = useCallback(async (path: string) => {
    setCurrentPath(path);
    setSelectedFiles(new Set());
    setPathHistory(prev => [...prev, path]);
    await fetchFiles(path, 1, false);
  }, [fetchFiles]);

  // 返回上一级
  const goBack = useCallback(() => {
    if (pathHistory.length > 1) {
      const newHistory = pathHistory.slice(0, -1);
      const prevPath = newHistory[newHistory.length - 1];
      setPathHistory(newHistory);
      setCurrentPath(prevPath);
      setSelectedFiles(new Set());
      fetchFiles(prevPath, 1, false);
    }
  }, [pathHistory, fetchFiles]);

  // 刷新当前目录
  const refresh = useCallback(async () => {
    await fetchFiles(currentPath, 1, false);
  }, [currentPath, fetchFiles]);

  // 加载更多
  const loadMore = useCallback(async () => {
    if (hasMore && !loading) {
      await fetchFiles(currentPath, page + 1, true);
    }
  }, [hasMore, loading, currentPath, page, fetchFiles]);

  // 设置视图模式
  const handleSetViewMode = useCallback((mode: ViewMode) => {
    setViewMode(mode);
    localStorage.setItem('webshare-view-mode', mode);
  }, []);

  // 切换选择
  const toggleSelect = useCallback((fileId: string) => {
    setSelectedFiles(prev => {
      const next = new Set(prev);
      if (next.has(fileId)) {
        next.delete(fileId);
      } else {
        next.add(fileId);
      }
      return next;
    });
  }, []);

  // 全选
  const selectAll = useCallback(() => {
    setSelectedFiles(new Set(files.map(f => f.id)));
  }, [files]);

  // 清除选择
  const clearSelection = useCallback(() => {
    setSelectedFiles(new Set());
  }, []);

  // 删除选中文件
  const deleteSelected = useCallback(async () => {
    if (selectedFiles.size === 0) return;

    try {
      const response = await fetch('/api/v1/webshare/files/batch', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids: Array.from(selectedFiles) }),
      });

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      setSelectedFiles(new Set());
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to delete files'));
    }
  }, [selectedFiles, refresh]);

  // 初始加载
  useEffect(() => {
    fetchFiles(currentPath, 1, false);
  }, [sortConfig]); // 依赖 sortConfig

  return {
    files,
    loading,
    error,
    currentPath,
    total,
    hasMore,
    viewMode,
    sortConfig,
    selectedFiles,
    navigateTo,
    goBack,
    refresh,
    setViewMode: handleSetViewMode,
    setSortConfig,
    toggleSelect,
    selectAll,
    clearSelection,
    deleteSelected,
    loadMore,
  };
}