import React from 'react';
import { PosterViewMode, PosterSize } from '../types';

interface ToolbarProps {
  viewMode: PosterViewMode;
  onViewModeChange: (mode: PosterViewMode) => void;
  posterSize: PosterSize;
  onPosterSizeChange: (size: PosterSize) => void;
  sortBy: 'title' | 'rating' | 'year' | 'added';
  onSortByChange: (sortBy: 'title' | 'rating' | 'year' | 'added') => void;
  sortOrder: 'asc' | 'desc';
  onToggleSortOrder: () => void;
  searchTerm: string;
  onSearchChange: (term: string) => void;
  genres?: string[];
  filterGenre?: string | null;
  onFilterGenreChange: (genre: string | null) => void;
  years?: number[];
  filterYear?: number | null;
  onFilterYearChange: (year: number | null) => void;
  onResetFilters: () => void;
  onScan?: () => void;
  onAddLibrary?: () => void;
  totalItems: number;
  filteredItems: number;
}

export const MediaToolbar: React.FC<ToolbarProps> = ({
  viewMode,
  onViewModeChange,
  posterSize,
  onPosterSizeChange,
  sortBy,
  onSortByChange,
  sortOrder,
  onToggleSortOrder,
  searchTerm,
  onSearchChange,
  genres,
  filterGenre,
  onFilterGenreChange,
  years,
  filterYear,
  onFilterYearChange,
  onResetFilters,
  onScan,
  onAddLibrary,
  totalItems,
  filteredItems,
}) => {
  return (
    <div className="media-toolbar flex flex-col md:flex-row items-start md:items-center gap-4 p-4 bg-gray-900/80 rounded-lg">
      {/* 搜索框 */}
      <div className="search-box flex-1 min-w-0 max-w-md">
        <div className="relative">
          <input
            type="text"
            placeholder="搜索电影、演员..."
            value={searchTerm}
            onChange={e => onSearchChange(e.target.value)}
            className="w-full bg-gray-800 text-white placeholder-gray-500 px-4 py-2 pl-10 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <svg className="absolute left-3 top-2.5 w-5 h-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
        </div>
      </div>

      {/* 视图切换 */}
      <div className="view-modes flex items-center gap-2">
        <button
          onClick={() => onViewModeChange('grid')}
          className={`p-2 rounded-lg transition-colors ${viewMode === 'grid' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}
          title="网格视图"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
          </svg>
        </button>
        <button
          onClick={() => onViewModeChange('list')}
          className={`p-2 rounded-lg transition-colors ${viewMode === 'list' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}
          title="列表视图"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 10h16M4 14h16M4 18h16" />
          </svg>
        </button>
        <button
          onClick={() => onViewModeChange('banner')}
          className={`p-2 rounded-lg transition-colors ${viewMode === 'banner' ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}
          title="横幅视图"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
        </button>
      </div>

      {/* 海报尺寸（仅网格模式） */}
      {viewMode === 'grid' && (
        <div className="poster-size flex items-center gap-1">
          {(['small', 'medium', 'large'] as PosterSize[]).map(size => (
            <button
              key={size}
              onClick={() => onPosterSizeChange(size)}
              className={`px-3 py-1 rounded text-sm transition-colors ${posterSize === size ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}
            >
              {size === 'small' ? '小' : size === 'medium' ? '中' : '大'}
            </button>
          ))}
        </div>
      )}

      {/* 排序 */}
      <div className="sort-controls flex items-center gap-2">
        <select
          value={sortBy}
          onChange={e => onSortByChange(e.target.value as 'title' | 'rating' | 'year' | 'added')}
          className="bg-gray-800 text-white px-3 py-2 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <option value="title">标题</option>
          <option value="rating">评分</option>
          <option value="year">年份</option>
          <option value="added">添加时间</option>
        </select>
        <button
          onClick={onToggleSortOrder}
          className={`p-2 rounded-lg bg-gray-800 text-gray-400 hover:text-white transition-colors ${sortOrder === 'desc' ? 'rotate-180' : ''}`}
          title={sortOrder === 'asc' ? '升序' : '降序'}
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 11l5-5m0 0l5 5m-5-5v12" />
          </svg>
        </button>
      </div>

      {/* 筛选器 */}
      <div className="filters flex items-center gap-2">
        {/* 类型筛选 */}
        {genres && genres.length > 0 && (
          <select
            value={filterGenre || ''}
            onChange={e => onFilterGenreChange(e.target.value || null)}
            className="bg-gray-800 text-white px-3 py-2 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">全部类型</option>
            {genres.map(g => (
              <option key={g} value={g}>{g}</option>
            ))}
          </select>
        )}

        {/* 年份筛选 */}
        {years && years.length > 0 && (
          <select
            value={filterYear || ''}
            onChange={e => onFilterYearChange(e.target.value ? parseInt(e.target.value, 10) : null)}
            className="bg-gray-800 text-white px-3 py-2 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="">全部年份</option>
            {years.map(y => (
              <option key={y} value={y}>{y}</option>
            ))}
          </select>
        )}

        {/* 重置筛选 */}
        {(filterGenre || filterYear || searchTerm) && (
          <button
            onClick={onResetFilters}
            className="px-3 py-2 bg-gray-800 text-gray-400 hover:text-white rounded-lg transition-colors"
          >
            重置
          </button>
        )}
      </div>

      {/* 操作按钮 */}
      <div className="actions flex items-center gap-2">
        {onScan && (
          <button
            onClick={onScan}
            className="px-4 py-2 bg-green-600 hover:bg-green-500 text-white rounded-lg transition-colors flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            扫描
          </button>
        )}
        {onAddLibrary && (
          <button
            onClick={onAddLibrary}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg transition-colors flex items-center gap-2"
          >
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
            </svg>
            添加库
          </button>
        )}
      </div>

      {/* 统计 */}
      <div className="stats text-gray-400 text-sm">
        {filteredItems} / {totalItems} 项
      </div>
    </div>
  );
};

export default MediaToolbar;