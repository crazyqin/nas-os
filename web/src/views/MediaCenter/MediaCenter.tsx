import React, { useState, useEffect, useMemo } from 'react';
import { PosterWall } from '../components/PosterWall';
import { MediaToolbar } from '../components/MediaToolbar';
import { MediaDetail } from '../components/MediaDetail';
import { usePosterWall, useMediaLibraries, useSmartScrape } from '../hooks/useMedia';
import { MediaMetadata, MediaLibrary } from '../types';

// 媒体中心主页面
export const MediaCenterView: React.FC = () => {
  // 状态
  const [mediaItems, setMediaItems] = useState<MediaMetadata[]>([]);
  const [selectedItem, setSelectedItem] = useState<MediaMetadata | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // hooks
  const posterWall = usePosterWall();
  const { libraries, loading: libsLoading, scanLibrary } = useMediaLibraries();
  const { scrape, batchScrape, loading: scrapeLoading } = useSmartScrape();

  // 加载媒体列表
  useEffect(() => {
    fetchMediaItems();
  }, [libraries]);

  const fetchMediaItems = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch('/api/media/files');
      if (!res.ok) throw new Error('加载失败');
      const files = await res.json();
      
      // 获取每个文件的元数据
      const items: MediaMetadata[] = [];
      for (const file of files) {
        const metaRes = await fetch(`/api/media/files/${file.id}/scrape`);
        if (metaRes.ok) {
          const meta = await metaRes.json();
          items.push(meta.results || meta);
        }
      }
      setMediaItems(items);
    } catch (e) {
      setError(e instanceof Error ? e.message : '未知错误');
    } finally {
      setLoading(false);
    }
  };

  // 提取所有类型和年份用于筛选
  const genres = useMemo(() => {
    const set = new Set<string>();
    mediaItems.forEach(item => item.genres?.forEach(g => set.add(g)));
    return Array.from(set).sort();
  }, [mediaItems]);

  const years = useMemo(() => {
    const set = new Set<number>();
    mediaItems.forEach(item => {
      if (item.releaseDate) {
        const y = parseInt(item.releaseDate.slice(0, 4), 10);
        if (y > 1900) set.add(y);
      }
    });
    return Array.from(set).sort((a, b) => b - a);
  }, [mediaItems]);

  // 处理扫描
  const handleScan = async () => {
    if (libraries.length === 0) {
      setError('请先添加媒体库');
      return;
    }
    setLoading(true);
    try {
      // 扫描所有库
      for (const lib of libraries) {
        await scanLibrary(lib.id);
      }
      // 刷新媒体列表
      await fetchMediaItems();
    } catch (e) {
      setError('扫描失败: ' + (e instanceof Error ? e.message : '未知错误'));
    } finally {
      setLoading(false);
    }
  };

  // 添加媒体库
  const handleAddLibrary = () => {
    // 打开添加库对话框（简化实现）
    const name = prompt('媒体库名称:');
    const path = prompt('媒体库路径:');
    const type = prompt('类型 (movie/tv):', 'movie');
    if (name && path) {
      fetch('/api/media/libraries', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, path, type }),
      })
        .then(() => fetchMediaItems())
        .catch(e => setError('添加失败: ' + e.message));
    }
  };

  // 播放媒体
  const handlePlay = async (filePath: string) => {
    try {
      const res = await fetch('/api/media/stream/hls', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ sourcePath: filePath }),
      });
      const data = await res.json();
      if (data.hlsUrl) {
        // 打开播放器（简化实现）
        window.open(data.hlsUrl, '_blank');
      }
    } catch (e) {
      setError('播放失败: ' + (e instanceof Error ? e.message : '未知错误'));
    }
  };

  // 处理点击媒体项
  const handleItemClick = (item: MediaMetadata) => {
    setSelectedItem(item);
  };

  // 关闭详情页
  const handleCloseDetail = () => {
    setSelectedItem(null);
  };

  return (
    <div className="media-center min-h-screen bg-gray-950 text-white">
      {/* 页头 */}
      <header className="header bg-gray-900/80 backdrop-blur px-6 py-4 sticky top-0 z-10">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold">影视中心</h1>
          <div className="header-actions flex items-center gap-4">
            <button
              onClick={handleScan}
              className="scan-btn bg-green-600 hover:bg-green-500 px-4 py-2 rounded-lg transition-colors flex items-center gap-2"
              disabled={loading}
            >
              <svg className="w-5 h-5 animate-spin" style={{ animationPlayState: loading ? 'running' : 'paused' }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {loading ? '扫描中...' : '扫描库'}
            </button>
            <button
              onClick={handleAddLibrary}
              className="add-btn bg-blue-600 hover:bg-blue-500 px-4 py-2 rounded-lg transition-colors"
            >
              + 添加库
            </button>
          </div>
        </div>
      </header>

      {/* 媒体库选择器 */}
      {libraries.length > 0 && (
        <div className="library-tabs px-6 py-3 bg-gray-900/50 flex items-center gap-2">
          {libraries.map(lib => (
            <button
              key={lib.id}
              className="library-tab bg-gray-800 hover:bg-gray-700 px-4 py-2 rounded-lg transition-colors flex items-center gap-2"
            >
              {lib.posterUrl && (
                <img src={lib.posterUrl} alt={lib.name} className="w-8 h-8 rounded" />
              )}
              <span>{lib.name}</span>
              <span className="text-gray-500 text-sm">{lib.mediaCount}项</span>
            </button>
          ))}
        </div>
      )}

      {/* 工具栏 */}
      <div className="toolbar-wrapper px-4">
        <MediaToolbar
          viewMode={posterWall.viewMode}
          onViewModeChange={posterWall.setViewMode}
          posterSize={posterWall.posterSize}
          onPosterSizeChange={posterWall.setPosterSize}
          sortBy={posterWall.sortBy}
          onSortByChange={posterWall.setSortBy}
          sortOrder={posterWall.sortOrder}
          onToggleSortOrder={posterWall.toggleSortOrder}
          searchTerm={posterWall.searchTerm}
          onSearchChange={posterWall.setSearchTerm}
          genres={genres}
          filterGenre={posterWall.filterGenre}
          onFilterGenreChange={posterWall.setFilterGenre}
          years={years}
          filterYear={posterWall.filterYear}
          onFilterYearChange={posterWall.setFilterYear}
          onResetFilters={posterWall.resetFilters}
          onScan={handleScan}
          onAddLibrary={handleAddLibrary}
          totalItems={mediaItems.length}
          filteredItems={mediaItems.length}
        />
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="error-banner bg-red-900/50 text-red-300 px-6 py-3 rounded-lg mx-4 mt-4 flex items-center justify-between">
          <span>{error}</span>
          <button onClick={() => setError(null)} className="text-red-300 hover:text-white">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      )}

      {/* 海报墙 */}
      <main className="poster-wall-container">
        <PosterWall
          items={mediaItems}
          viewMode={posterWall.viewMode}
          posterSize={posterWall.posterSize}
          sortBy={posterWall.sortBy}
          sortOrder={posterWall.sortOrder}
          filterGenre={posterWall.filterGenre}
          filterYear={posterWall.filterYear}
          searchTerm={posterWall.searchTerm}
          onItemClick={handleItemClick}
          loading={loading}
        />
      </main>

      {/* 详情模态框 */}
      {selectedItem && (
        <MediaDetail
          item={selectedItem}
          onClose={handleCloseDetail}
          onPlay={handlePlay}
        />
      )}

      {/* 空状态 */}
      {!loading && mediaItems.length === 0 && libraries.length === 0 && (
        <div className="empty-state flex flex-col items-center justify-center h-96 text-gray-500">
          <svg className="w-24 h-24 mb-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 4v16M17 4v16M3 8h4m10 0h4M3 12h4m10 0h4M3 16h4m10 0h4M4 20h16a1 1 0 001-1V5a1 1 0 00-1-1H4a1 1 0 00-1 1v14a1 1 0 001 1z" />
          </svg>
          <h2 className="text-xl mb-2">还没有媒体库</h2>
          <p className="mb-6">添加一个媒体库开始管理您的影视收藏</p>
          <button
            onClick={handleAddLibrary}
            className="bg-blue-600 hover:bg-blue-500 px-6 py-3 rounded-lg transition-colors"
          >
            添加第一个媒体库
          </button>
        </div>
      )}
    </div>
  );
};

export default MediaCenterView;