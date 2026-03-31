import React, { useMemo } from 'react';
import { MediaMetadata, PosterViewMode, PosterSize } from '../types';

interface PosterWallProps {
  items: MediaMetadata[];
  viewMode: PosterViewMode;
  posterSize: PosterSize;
  sortBy: 'title' | 'rating' | 'year' | 'added';
  sortOrder: 'asc' | 'desc';
  filterGenre?: string | null;
  filterYear?: number | null;
  searchTerm?: string;
  onItemClick: (item: MediaMetadata) => void;
  loading?: boolean;
}

// 海报尺寸映射
const POSTER_DIMENSIONS: Record<PosterSize, { width: number; height: number }> = {
  small: { width: 120, height: 180 },
  medium: { width: 180, height: 270 },
  large: { width: 240, height: 360 },
};

// 获取海报URL
function getPosterUrl(item: MediaMetadata, size: PosterSize): string {
  if (item.localPosterPath) {
    return `/api/media/posters/${encodeURIComponent(item.localPosterPath)}`;
  }
  if (item.posterPath) {
    const tmdbSize = size === 'small' ? 'w185' : size === 'medium' ? 'w342' : 'w500';
    return `https://image.tmdb.org/t/p/${tmdbSize}${item.posterPath}`;
  }
  // 默认占位图
  const dim = POSTER_DIMENSIONS[size];
  return `data:image/svg+xml,${encodeURIComponent(`
    <svg xmlns="http://www.w3.org/2000/svg" width="${dim.width}" height="${dim.height}" viewBox="0 0 ${dim.width} ${dim.height}">
      <rect fill="#1a1a2e" width="${dim.width}" height="${dim.height}"/>
      <text fill="#666" font-family="Arial" font-size="14" x="50%" y="50%" text-anchor="middle">${item.title.slice(0, 20)}</text>
    </svg>
  `)}`;
}

// 过滤和排序媒体项
function useFilteredItems(
  items: MediaMetadata[],
  sortBy: string,
  sortOrder: string,
  filterGenre?: string | null,
  filterYear?: number | null,
  searchTerm?: string
): MediaMetadata[] {
  return useMemo(() => {
    let result = [...items];

    // 搜索过滤
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      result = result.filter(item =>
        item.title.toLowerCase().includes(term) ||
        (item.originalTitle && item.originalTitle.toLowerCase().includes(term)) ||
        (item.overview && item.overview.toLowerCase().includes(term)) ||
        (item.cast && item.cast.some(c => c.name.toLowerCase().includes(term)))
      );
    }

    // 类型过滤
    if (filterGenre) {
      result = result.filter(item =>
        item.genres && item.genres.includes(filterGenre)
      );
    }

    // 年份过滤
    if (filterYear) {
      result = result.filter(item => {
        if (!item.releaseDate) return false;
        const year = parseInt(item.releaseDate.slice(0, 4), 10);
        return year === filterYear;
      });
    }

    // 排序
    result.sort((a, b) => {
      let cmp = 0;
      switch (sortBy) {
        case 'title':
          cmp = a.title.localeCompare(b.title);
          break;
        case 'rating':
          cmp = (b.rating || 0) - (a.rating || 0);
          break;
        case 'year':
          const yearA = a.releaseDate ? parseInt(a.releaseDate.slice(0, 4), 10) : 0;
          const yearB = b.releaseDate ? parseInt(b.releaseDate.slice(0, 4), 10) : 0;
          cmp = yearB - yearA;
          break;
        case 'added':
          cmp = new Date(b.scrapedAt).getTime() - new Date(a.scrapedAt).getTime();
          break;
      }
      return sortOrder === 'desc' ? -cmp : cmp;
    });

    return result;
  }, [items, sortBy, sortOrder, filterGenre, filterYear, searchTerm]);
}

// 海报卡片组件
const PosterCard: React.FC<{
  item: MediaMetadata;
  size: PosterSize;
  onClick: () => void;
}> = ({ item, size, onClick }) => {
  const dim = POSTER_DIMENSIONS[size];
  const posterUrl = getPosterUrl(item, size);

  return (
    <div
      className="poster-card cursor-pointer group relative overflow-hidden rounded-lg shadow-lg transition-all duration-300 hover:shadow-xl hover:scale-105"
      style={{ width: dim.width }}
      onClick={onClick}
    >
      {/* 海报图片 */}
      <img
        src={posterUrl}
        alt={item.title}
        className="w-full h-auto object-cover aspect-[2/3]"
        loading="lazy"
      />

      {/* hover overlay */}
      <div className="absolute inset-0 bg-gradient-to-t from-black/80 via-black/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex flex-col items-end justify-end p-2">
        {/* 评分徽章 */}
        {item.rating && (
          <div className="absolute top-2 right-2 bg-yellow-500/90 text-black text-xs font-bold px-2 py-1 rounded">
            {item.rating.toFixed(1)}
          </div>
        )}

        {/* 标题信息 */}
        <div className="text-white text-sm font-medium truncate w-full">
          {item.title}
        </div>
        {item.releaseDate && (
          <div className="text-gray-300 text-xs">
            {item.releaseDate.slice(0, 4)}
          </div>
        )}

        {/* 类型标签 */}
        <div className="flex flex-wrap gap-1 mt-1">
          {item.genres?.slice(0, 2).map(g => (
            <span key={g} className="bg-blue-600/70 text-white text-xs px-1 rounded">
              {g}
            </span>
          ))}
        </div>
      </div>

      {/* 底部标题栏（非hover状态） */}
      <div className="poster-title-bar bg-gradient-to-t from-black/60 to-transparent p-2 absolute bottom-0 left-0 right-0 group-hover:hidden">
        <div className="text-white text-sm font-medium truncate">
          {item.title}
        </div>
      </div>

      {/* 评分角标 */}
      {item.rating && item.rating >= 8 && (
        <div className="absolute top-0 left-0 bg-yellow-500 text-black text-xs font-bold px-2 py-1 rounded-br-lg">
          ★
        </div>
      )}
    </div>
  );
};

// 列表行组件
const ListRow: React.FC<{
  item: MediaMetadata;
  onClick: () => void;
}> = ({ item, onClick }) => {
  const posterUrl = getPosterUrl(item, 'small');

  return (
    <div
      className="list-row flex items-center gap-4 p-3 bg-gray-800/50 rounded-lg cursor-pointer hover:bg-gray-700/50 transition-colors"
      onClick={onClick}
    >
      {/* 小海报 */}
      <img
        src={posterUrl}
        alt={item.title}
        className="w-16 h-24 object-cover rounded"
        loading="lazy"
      />

      {/* 基本信息 */}
      <div className="flex-1 min-w-0">
        <h3 className="text-white font-medium truncate">{item.title}</h3>
        <div className="text-gray-400 text-sm mt-1">
          {item.releaseDate?.slice(0, 4)} · {item.runtime ? `${item.runtime}分钟` : ''}
        </div>
        <div className="text-gray-500 text-xs mt-1 truncate">
          {item.genres?.join(' / ')}
        </div>
      </div>

      {/* 评分 */}
      {item.rating && (
        <div className="flex items-center gap-1 bg-yellow-500/20 px-2 py-1 rounded">
          <span className="text-yellow-500">★</span>
          <span className="text-white font-medium">{item.rating.toFixed(1)}</span>
        </div>
      )}

      {/* 播放按钮 */}
      <button className="play-btn bg-blue-600 hover:bg-blue-500 text-white px-4 py-2 rounded-lg transition-colors">
        播放
      </button>
    </div>
  );
};

// Banner海报组件（大横幅）
const BannerItem: React.FC<{
  item: MediaMetadata;
  onClick: () => void;
}> = ({ item, onClick }) => {
  const backdropUrl = item.backdropPath
    ? `https://image.tmdb.org/t/p/w1280${item.backdropPath}`
    : getPosterUrl(item, 'large');

  return (
    <div
      className="banner-item relative h-48 md:h-64 rounded-xl overflow-hidden cursor-pointer group"
      onClick={onClick}
    >
      {/* 背景图 */}
      <img
        src={backdropUrl}
        alt={item.title}
        className="w-full h-full object-cover transition-transform duration-500 group-hover:scale-110"
        loading="lazy"
      />

      {/* 内容overlay */}
      <div className="absolute inset-0 bg-gradient-to-r from-black/80 via-black/40 to-transparent flex items-center">
        {/* 海报 */}
        <div className="poster-wrapper ml-4 md:ml-8">
          <img
            src={getPosterUrl(item, 'medium')}
            alt={item.title}
            className="w-24 md:w-32 h-auto rounded-lg shadow-2xl"
          />
        </div>

        {/* 信息 */}
        <div className="ml-4 md:ml-6 flex-1 min-w-0">
          <h2 className="text-white text-xl md:text-2xl font-bold truncate">
            {item.title}
          </h2>
          <div className="text-gray-300 text-sm mt-2 flex items-center gap-3">
            <span>{item.releaseDate?.slice(0, 4)}</span>
            {item.rating && (
              <span className="flex items-center gap-1">
                <span className="text-yellow-500">★</span>
                <span>{item.rating.toFixed(1)}</span>
              </span>
            )}
            {item.runtime && <span>{item.runtime}分钟</span>}
          </div>
          <p className="text-gray-400 text-sm mt-2 line-clamp-2 max-w-md">
            {item.overview}
          </p>
        </div>
      </div>
    </div>
  );
};

// 主海报墙组件
export const PosterWall: React.FC<PosterWallProps> = ({
  items,
  viewMode,
  posterSize,
  sortBy,
  sortOrder,
  filterGenre,
  filterYear,
  searchTerm,
  onItemClick,
  loading,
}) => {
  const filteredItems = useFilteredItems(items, sortBy, sortOrder, filterGenre, filterYear, searchTerm);

  if (loading) {
    return (
      <div className="poster-wall-loading flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-blue-500"></div>
        <span className="text-gray-400 ml-4">加载中...</span>
      </div>
    );
  }

  if (filteredItems.length === 0) {
    return (
      <div className="poster-wall-empty flex flex-col items-center justify-center h-64 text-gray-500">
        <svg className="w-16 h-16 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 4v16M17 4v16M3 8h4m10 0h4M3 12h4m10 0h4M3 16h4m10 0h4M4 20h16a1 1 0 001-1V5a1 1 0 00-1-1H4a1 1 0 00-1 1v14a1 1 0 001 1z" />
        </svg>
        <p>暂无媒体内容</p>
        {searchTerm && <p className="text-sm mt-2">未找到匹配 "{searchTerm}" 的结果</p>}
      </div>
    );
  }

  // 网格视图
  if (viewMode === 'grid') {
    const gridCols = posterSize === 'small' ? 'grid-cols-4 md:grid-cols-6 lg:grid-cols-8' :
                     posterSize === 'medium' ? 'grid-cols-3 md:grid-cols-5 lg:grid-cols-6' :
                     'grid-cols-2 md:grid-cols-3 lg:grid-cols-4';

    return (
      <div className={`poster-wall-grid grid ${gridCols} gap-4 p-4`}>
        {filteredItems.map(item => (
          <PosterCard
            key={item.id}
            item={item}
            size={posterSize}
            onClick={() => onItemClick(item)}
          />
        ))}
      </div>
    );
  }

  // 列表视图
  if (viewMode === 'list') {
    return (
      <div className="poster-wall-list flex flex-col gap-3 p-4">
        {filteredItems.map(item => (
          <ListRow
            key={item.id}
            item={item}
            onClick={() => onItemClick(item)}
          />
        ))}
      </div>
    );
  }

  // Banner视图
  if (viewMode === 'banner') {
    return (
      <div className="poster-wall-banner flex flex-col gap-4 p-4">
        {filteredItems.slice(0, 10).map(item => (
          <BannerItem
            key={item.id}
            item={item}
            onClick={() => onItemClick(item)}
          />
        ))}
        {/* 剩余内容转为网格 */}
        {filteredItems.length > 10 && (
          <div className="grid grid-cols-4 md:grid-cols-6 gap-4">
            {filteredItems.slice(10).map(item => (
              <PosterCard
                key={item.id}
                item={item}
                size="small"
                onClick={() => onItemClick(item)}
              />
            ))}
          </div>
        )}
      </div>
    );
  }

  return null;
};

export default PosterWall;