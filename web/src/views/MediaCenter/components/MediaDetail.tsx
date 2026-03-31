import React, { useState } from 'react';
import { MediaMetadata, TVShowMetadata } from '../types';

interface MediaDetailProps {
  item: MediaMetadata;
  onClose: () => void;
  onPlay: (filePath: string) => void;
}

// 获取海报URL
function getPosterUrl(path: string | undefined, size: string = 'w500'): string {
  if (!path) return '';
  if (path.startsWith('/')) {
    return `https://image.tmdb.org/t/p/${size}${path}`;
  }
  return path;
}

// 格式化时长
function formatRuntime(minutes: number | undefined): string {
  if (!minutes) return '';
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}小时${m}分钟` : `${m}分钟`;
}

// 格式化日期
function formatDate(date: string | undefined): string {
  if (!date) return '';
  return date.replace(/-/g, '/');
}

export const MediaDetail: React.FC<MediaDetailProps> = ({ item, onClose, onPlay }) => {
  const [selectedSeason, setSelectedSeason] = useState(0);
  const isTVShow = item.type === 'tv';
  const tvItem = item as TVShowMetadata;

  return (
    <div className="media-detail fixed inset-0 z-50 bg-black/80 flex items-center justify-center p-4 md:p-8">
      <div className="detail-content bg-gray-900 rounded-xl max-w-4xl w-full max-h-[90vh] overflow-hidden flex flex-col md:flex-row shadow-2xl">
        {/* 左侧海报 */}
        <div className="poster-section md:w-1/3 relative">
          <img
            src={getPosterUrl(item.posterPath, 'w780')}
            alt={item.title}
            className="w-full h-64 md:h-full object-cover"
          />
          {/* 评分徽章 */}
          {item.rating && (
            <div className="absolute top-4 left-4 bg-yellow-500 text-black text-lg font-bold px-3 py-2 rounded-lg shadow-lg">
              <span className="text-2xl">{item.rating.toFixed(1)}</span>
              <span className="text-xs ml-1">/10</span>
            </div>
          )}
        </div>

        {/* 右侧详情 */}
        <div className="info-section md:w-2/3 p-6 overflow-y-auto">
          {/* 关闭按钮 */}
          <button
            onClick={onClose}
            className="close-btn absolute top-4 right-4 md:top-6 md:right-6 p-2 bg-gray-800/50 hover:bg-gray-700 rounded-full transition-colors"
          >
            <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>

          {/* 标题区 */}
          <div className="title-section mb-6">
            <h1 className="text-white text-2xl md:text-3xl font-bold">{item.title}</h1>
            {item.originalTitle && item.originalTitle !== item.title && (
              <p className="text-gray-400 text-sm mt-1">{item.originalTitle}</p>
            )}
            <div className="meta-info flex items-center gap-4 mt-3 text-gray-400">
              <span className="release-date">{formatDate(item.releaseDate)}</span>
              {item.runtime && <span>{formatRuntime(item.runtime)}</span>}
              {item.type === 'tv' && tvItem.numberOfSeasons && (
                <span>{tvItem.numberOfSeasons}季</span>
              )}
              {item.type === 'tv' && tvItem.numberOfEpisodes && (
                <span>{tvItem.numberOfEpisodes}集</span>
              )}
              {item.status && <span className="status-badge bg-blue-600/30 text-blue-400 px-2 py-1 rounded text-xs">{item.status}</span>}
            </div>
          </div>

          {/* 类型标签 */}
          <div className="genres flex flex-wrap gap-2 mb-6">
            {item.genres?.map(genre => (
              <span key={genre} className="genre-tag bg-gray-800 text-gray-300 px-3 py-1 rounded-full text-sm">
                {genre}
              </span>
            ))}
          </div>

          {/* 简介 */}
          {item.overview && (
            <div className="overview mb-6">
              <h3 className="text-white font-medium mb-2">剧情简介</h3>
              <p className="text-gray-400 leading-relaxed">{item.overview}</p>
            </div>
          )}

          {/* 标语 */}
          {item.tagline && (
            <div className="tagline italic text-gray-500 mb-6 border-l-2 border-gray-600 pl-4">
              "{item.tagline}"
            </div>
          )}

          {/* 导演 */}
          {item.directors && item.directors.length > 0 && (
            <div className="directors mb-6">
              <h3 className="text-white font-medium mb-2">导演</h3>
              <p className="text-gray-400">{item.directors.join(', ')}</p>
            </div>
          )}

          {/* 演员 */}
          {item.cast && item.cast.length > 0 && (
            <div className="cast-section mb-6">
              <h3 className="text-white font-medium mb-3">演员</h3>
              <div className="cast-grid grid grid-cols-2 md:grid-cols-3 gap-3">
                {item.cast.slice(0, 6).map(actor => (
                  <div key={actor.name} className="cast-card bg-gray-800/50 rounded-lg p-2 flex items-center gap-2">
                    <img
                      src={getPosterUrl(actor.profilePath, 'w185') || 'data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' width=\'40\' height=\'40\'%3E%3Crect fill=\'%23333\' width=\'40\' height=\'40\'/%3E%3C/svg%3E'}
                      alt={actor.name}
                      className="w-10 h-10 rounded-full object-cover"
                    />
                    <div className="flex-1 min-w-0">
                      <p className="text-white text-sm truncate">{actor.name}</p>
                      {actor.character && (
                        <p className="text-gray-500 text-xs truncate">{actor.character}</p>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* 电视剧季列表 */}
          {isTVShow && tvItem.seasons && tvItem.seasons.length > 0 && (
            <div className="seasons-section mb-6">
              <h3 className="text-white font-medium mb-3">季与集</h3>
              <div className="season-tabs flex gap-2 mb-4">
                {tvItem.seasons.map(season => (
                  <button
                    key={season.seasonNumber}
                    onClick={() => setSelectedSeason(season.seasonNumber)}
                    className={`season-tab px-4 py-2 rounded-lg transition-colors ${selectedSeason === season.seasonNumber ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:text-white'}`}
                  >
                    第{season.seasonNumber}季
                  </button>
                ))}
              </div>

              {/* 集列表 */}
              {tvItem.seasons[selectedSeason]?.episodes && (
                <div className="episode-list grid grid-cols-2 md:grid-cols-3 gap-2">
                  {tvItem.seasons[selectedSeason].episodes.map(ep => (
                    <button
                      key={ep.episodeNumber}
                      onClick={() => onPlay(`${item.id}/S${selectedSeason}E${ep.episodeNumber}`)}
                      className="episode-btn bg-gray-800 hover:bg-gray-700 rounded-lg p-3 text-left transition-colors"
                    >
                      <div className="text-white text-sm">第{ep.episodeNumber}集</div>
                      {ep.name && <div className="text-gray-400 text-xs truncate">{ep.name}</div>}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* 播放按钮 */}
          <div className="action-buttons flex gap-3 mt-6">
            <button
              onClick={() => onPlay(item.id)}
              className="play-btn flex-1 bg-blue-600 hover:bg-blue-500 text-white py-3 rounded-lg font-medium transition-colors flex items-center justify-center gap-2"
            >
              <svg className="w-6 h-6" fill="currentColor" viewBox="0 0 24 24">
                <path d="M8 5v14l11-7z" />
              </svg>
              播放
            </button>
            <button
              className="add-btn bg-gray-800 hover:bg-gray-700 text-white py-3 px-6 rounded-lg transition-colors flex items-center justify-center gap-2"
            >
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              添加列表
            </button>
          </div>

          {/* 来源标识 */}
          {item.source && (
            <div className="source-badge text-gray-500 text-xs mt-4">
              数据来源: {item.source === 'tmdb' ? 'TMDB' : item.source === 'douban' ? '豆瓣' : item.source}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default MediaDetail;