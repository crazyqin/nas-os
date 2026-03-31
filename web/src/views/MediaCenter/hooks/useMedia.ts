import { useState, useCallback, useEffect } from 'react';
import { MediaMetadata, MediaType, PosterViewMode, PosterSize } from '../types';

// 搜索媒体元数据
export function useMediaSearch() {
  const [results, setResults] = useState<MediaMetadata[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const search = useCallback(async (query: string, type?: MediaType, year?: number) => {
    setLoading(true);
    setError(null);
    try {
      const params = new URLSearchParams({ q: query });
      if (type) params.set('type', type);
      if (year) params.set('year', String(year));
      
      const res = await fetch(`/api/media/metadata/search?${params}`);
      if (!res.ok) throw new Error('搜索失败');
      const data = await res.json();
      setResults(Array.isArray(data) ? data : [data]);
    } catch (e) {
      setError(e instanceof Error ? e.message : '未知错误');
    } finally {
      setLoading(false);
    }
  }, []);

  return { results, loading, error, search };
}

// 获取媒体库列表
export function useMediaLibraries() {
  const [libraries, setLibraries] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch('/api/media/libraries')
      .then(res => res.json())
      .then(data => {
        setLibraries(data);
        setLoading(false);
      })
      .catch(e => {
        setError(e.message);
        setLoading(false);
      });
  }, []);

  const createLibrary = useCallback(async (name: string, path: string, type: MediaType) => {
    const res = await fetch('/api/media/libraries', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, path, type }),
    });
    if (!res.ok) throw new Error('创建失败');
    const lib = await res.json();
    setLibraries(prev => [...prev, lib]);
    return lib;
  }, []);

  const scanLibrary = useCallback(async (id: string) => {
    const res = await fetch(`/api/media/libraries/${id}/scan`, { method: 'POST' });
    if (!res.ok) throw new Error('扫描失败');
    return res.json();
  }, []);

  const deleteLibrary = useCallback(async (id: string) => {
    const res = await fetch(`/api/media/libraries/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('删除失败');
    setLibraries(prev => prev.filter(l => l.id !== id));
  }, []);

  return { libraries, loading, error, createLibrary, scanLibrary, deleteLibrary };
}

// 海报墙状态
export function usePosterWall() {
  const [viewMode, setViewMode] = useState<PosterViewMode>('grid');
  const [posterSize, setPosterSize] = useState<PosterSize>('medium');
  const [sortBy, setSortBy] = useState<'title' | 'rating' | 'year' | 'added'>('title');
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('asc');
  const [filterGenre, setFilterGenre] = useState<string | null>(null);
  const [filterYear, setFilterYear] = useState<number | null>(null);

  const toggleSortOrder = useCallback(() => {
    setSortOrder(prev => prev === 'asc' ? 'desc' : 'asc');
  }, []);

  const resetFilters = useCallback(() => {
    setFilterGenre(null);
    setFilterYear(null);
    setSearchTerm('');
  }, []);

  const [searchTerm, setSearchTerm] = useState('');

  return {
    viewMode, setViewMode,
    posterSize, setPosterSize,
    sortBy, setSortBy,
    sortOrder, setSortOrder, toggleSortOrder,
    filterGenre, setFilterGenre,
    filterYear, setFilterYear,
    searchTerm, setSearchTerm,
    resetFilters,
  };
}

// 媒体播放
export function useMediaPlayback(mediaId: string) {
  const [playback, setPlayback] = useState({
    currentTime: 0,
    volume: 1,
    muted: false,
    paused: true,
    quality: 'auto',
  });
  const [streamUrl, setStreamUrl] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const startPlayback = useCallback(async (filePath: string) => {
    setLoading(true);
    try {
      const res = await fetch('/api/media/stream/hls', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sourcePath: filePath }),
      });
      const data = await res.json();
      setStreamUrl(data.hlsUrl);
      setPlayback(prev => ({ ...prev, paused: false }));
    } catch (e) {
      console.error('启动播放失败:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  const stopPlayback = useCallback(async () => {
    if (streamUrl) {
      // 通知服务器停止流
      try {
        await fetch(`/api/media/stream/${mediaId}`, { method: 'DELETE' });
      } catch (e) {
        console.error('停止播放失败:', e);
      }
    }
    setStreamUrl(null);
    setPlayback(prev => ({ ...prev, paused: true }));
  }, [mediaId, streamUrl]);

  return { playback, setPlayback, streamUrl, loading, startPlayback, stopPlayback };
}

// 智能刮削
export function useSmartScrape() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const scrape = useCallback(async (filename: string, hints?: { type?: MediaType; title?: string; year?: number }) => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/media/scrape', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ filename, ...hints }),
      });
      if (!res.ok) throw new Error('刮削失败');
      return await res.json();
    } catch (e) {
      setError(e instanceof Error ? e.message : '未知错误');
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  const batchScrape = useCallback(async (files: string[], workers?: number) => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/media/batch-scrape', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ items: files.map(f => ({ filename: f })), workers }),
      });
      if (!res.ok) throw new Error('批量刮削失败');
      return await res.json();
    } catch (e) {
      setError(e instanceof Error ? e.message : '未知错误');
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  return { loading, error, scrape, batchScrape };
}