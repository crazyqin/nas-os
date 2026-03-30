// Webshare 类型定义

// 文件类型
export interface FileItem {
  id: string;
  name: string;
  path: string;
  size: number;
  mimeType: string;
  extension: string;
  isDirectory: boolean;
  createdAt: string;
  modifiedAt: string;
  accessedAt?: string;
  permissions?: FilePermissions;
  thumbnail?: string;
  preview?: string;
}

export interface FilePermissions {
  read: boolean;
  write: boolean;
  execute: boolean;
  owner: string;
  group: string;
}

// 文件列表响应
export interface FileListResponse {
  files: FileItem[];
  total: number;
  page: number;
  pageSize: number;
  hasMore: boolean;
  currentPath: string;
}

// 分享类型
export interface Share {
  id: string;
  name: string;
  path: string;
  type: 'file' | 'directory';
  token: string;
  url: string;
  password?: string;
  hasPassword: boolean;
  maxDownloads?: number;
  downloadCount: number;
  expiresAt?: string;
  createdAt: string;
  createdBy: string;
  isActive: boolean;
}

// 创建分享请求
export interface CreateShareRequest {
  path: string;
  name?: string;
  password?: string;
  maxDownloads?: number;
  expiresAt?: string;
  expiresIn?: number; // 秒
}

// 分享响应
export interface ShareResponse {
  share: Share;
  shareUrl: string;
}

// 分享列表响应
export interface ShareListResponse {
  shares: Share[];
  total: number;
  page: number;
  pageSize: number;
}

// 上传状态
export interface UploadFile {
  id: string;
  file: File;
  name: string;
  size: number;
  progress: number;
  status: 'pending' | 'uploading' | 'completed' | 'error';
  error?: string;
  speed?: number;
  uploadedBytes: number;
}

// 视图模式
export type ViewMode = 'grid' | 'list';

// 排序选项
export type SortField = 'name' | 'size' | 'modifiedAt' | 'createdAt';
export type SortOrder = 'asc' | 'desc';

export interface SortConfig {
  field: SortField;
  order: SortOrder;
}

// 搜索参数
export interface SearchParams {
  query: string;
  type?: 'file' | 'directory' | 'all';
  mimeType?: string;
  minSize?: number;
  maxSize?: number;
  modifiedAfter?: string;
  modifiedBefore?: string;
}

// 搜索结果
export interface SearchResult {
  files: FileItem[];
  total: number;
  query: string;
  highlights?: Record<string, string[]>;
}

// 面包屑项
export interface BreadcrumbItem {
  name: string;
  path: string;
  isRoot?: boolean;
}

// 文件预览类型
export type PreviewType = 
  | 'image' 
  | 'video' 
  | 'audio' 
  | 'pdf' 
  | 'document' 
  | 'code' 
  | 'text' 
  | 'archive'
  | 'unknown';

// 预览信息
export interface PreviewInfo {
  type: PreviewType;
  url: string;
  canPreview: boolean;
  canEdit?: boolean;
  size: number;
  mimeType: string;
}

// API 响应包装
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

// 分享有效期选项
export interface ExpiryOption {
  label: string;
  value: number | null; // null 表示永不过期
}

export const EXPIRY_OPTIONS: ExpiryOption[] = [
  { label: '1小时', value: 3600 },
  { label: '24小时', value: 86400 },
  { label: '7天', value: 604800 },
  { label: '30天', value: 2592000 },
  { label: '永不过期', value: null },
];