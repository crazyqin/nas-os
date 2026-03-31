// 媒体中心类型定义

// 媒体类型
export type MediaType = 'movie' | 'tv' | 'episode' | 'unknown';

// 视频文件
export interface VideoFile {
  id: string;
  path: string;
  filename: string;
  size: number;
  duration?: number;
  width?: number;
  height?: number;
  codec?: string;
  bitrate?: number;
  createdAt: string;
  modifiedAt: string;
}

// 演员信息
export interface Cast {
  name: string;
  character?: string;
  profilePath?: string;
  order?: number;
}

// 媒体元数据
export interface MediaMetadata {
  id: string;
  tmdbId: number;
  imdbId?: string;
  type: MediaType;
  title: string;
  originalTitle?: string;
  overview?: string;
  tagline?: string;
  posterPath?: string;
  backdropPath?: string;
  rating?: number;
  voteCount?: number;
  releaseDate?: string;
  runtime?: number;
  genres?: string[];
  cast?: Cast[];
  directors?: string[];
  studios?: string[];
  countries?: string[];
  languages?: string[];
  scrapedAt: string;
  localPosterPath?: string;
  source?: string; // tmdb, douban, imdb
}

// 季信息
export interface Season {
  seasonNumber: number;
  name?: string;
  overview?: string;
  posterPath?: string;
  airDate?: string;
  episodes?: Episode[];
}

// 集信息
export interface Episode {
  episodeNumber: number;
  seasonNumber: number;
  name?: string;
  overview?: string;
  stillPath?: string;
  airDate?: string;
  runtime?: number;
}

// 电视剧元数据
export interface TVShowMetadata extends MediaMetadata {
  seasons?: Season[];
  numberOfSeasons?: number;
  numberOfEpisodes?: number;
  status?: string;
  networks?: string[];
}

// 媒体库
export interface MediaLibrary {
  id: string;
  name: string;
  path: string;
  type: MediaType;
  mediaCount: number;
  totalSize: number;
  lastScanned: string;
  posterUrl?: string;
}

// 扫描结果
export interface ScanResult {
  libraryId: string;
  totalFiles: number;
  newFiles: number;
  updatedFiles: number;
  removedFiles: number;
  errors?: ScanError[];
  duration: number;
}

// 扫描错误
export interface ScanError {
  path: string;
  message: string;
}

// 转码任务
export interface TranscodeJob {
  id: string;
  inputPath: string;
  outputPath: string;
  config: TranscodeConfig;
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled';
  progress: number;
  currentFrame: number;
  totalFrames: number;
  speed: string;
  eta: string;
  startTime?: string;
  endTime?: string;
  error?: string;
}

// 转码配置
export interface TranscodeConfig {
  videoCodec?: string;
  audioCodec?: string;
  videoBitrate?: string;
  audioBitrate?: string;
  resolution?: string;
  framerate?: number;
  outputFormat?: string;
  preset?: string;
  crf?: number;
  hwAccel?: string;
  copyVideo?: boolean;
  copyAudio?: boolean;
}

// 海报墙视图模式
export type PosterViewMode = 'grid' | 'list' | 'banner';

// 海报尺寸
export type PosterSize = 'small' | 'medium' | 'large';

// 媒体中心状态
export interface MediaCenterState {
  libraries: MediaLibrary[];
  selectedLibrary: string | null;
  mediaItems: MediaMetadata[];
  selectedMedia: MediaMetadata | null;
  viewMode: PosterViewMode;
  posterSize: PosterSize;
  searchTerm: string;
  filterGenre: string | null;
  filterYear: number | null;
  sortBy: 'title' | 'rating' | 'year' | 'added';
  sortOrder: 'asc' | 'desc';
  isLoading: boolean;
  error: string | null;
}

// 播放信息
export interface PlaybackInfo {
  mediaId: string;
  filePath: string;
  streamUrl: string;
  hlsUrl?: string;
  dashUrl?: string;
  currentTime: number;
  duration: number;
  volume: number;
  muted: boolean;
  paused: boolean;
  quality: string;
}

// HDR格式
export type HDRFormat = 'sdr' | 'hdr10' | 'hdr10plus' | 'dolbyvision' | 'hlg';

// 视频编码
export type VideoCodec = 'h264' | 'h265' | 'hevc' | 'av1' | 'vp9';

// 音频编码
export type AudioCodec = 'aac' | 'ac3' | 'eac3' | 'dts' | 'truehd' | 'atmos';

// 媒体分析结果
export interface MediaAnalysis {
  filePath: string;
  duration: number;
  width: number;
  height: number;
  videoCodec: VideoCodec;
  audioCodec: AudioCodec;
  bitrate: number;
  hdrFormat: HDRFormat;
  bitDepth: number;
  frameRate: number;
  hasAtmos: boolean;
  hasTrueHD: boolean;
}