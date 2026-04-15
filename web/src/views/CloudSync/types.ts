/**
 * Cloud Drive Sync - 类型定义
 */

export type SyncDirection = 'bidirectional' | 'upload' | 'download';
export type SyncStatus = 'running' | 'idle' | 'paused' | 'error';
export type ConflictStrategy = 'newer' | 'bigger' | 'keep_both' | 'local' | 'remote';
export type CloudProviderType = 'google' | 'dropbox' | 'onedrive' | 's3' | 'aliyun' | 'tencent' | 'webdav';

export interface CloudProvider {
  id: string;
  type: CloudProviderType;
  name: string;
  status: 'connected' | 'disconnected' | 'error';
  used: number;        // bytes
  quota: number;       // bytes
  sync_count: number;
  last_sync?: string;
  created_at: string;
}

export interface SyncTask {
  id: string;
  name: string;
  provider_id: string;
  provider_name: string;
  local_path: string;
  remote_path: string;
  direction: SyncDirection;
  conflict_strategy: ConflictStrategy;
  schedule: 'realtime' | 'hourly' | 'daily' | 'weekly' | 'manual';
  status: SyncStatus;
  progress: number;     // 0-100
  files_total: number;
  files_synced: number;
  bytes_total: number;
  bytes_synced: number;
  speed_bps: number;
  eta_seconds: number;
  error_message?: string;
  last_run?: string;
  next_run?: string;
  created_at: string;
  updated_at: string;
}

export interface SyncActivity {
  id: string;
  task_id: string;
  task_name: string;
  type: 'upload' | 'download' | 'delete' | 'conflict' | 'error';
  filename: string;
  filepath: string;
  size: number;         // bytes
  direction: 'to_cloud' | 'to_local';
  status: 'success' | 'failed' | 'skipped';
  timestamp: string;
}

export interface SyncHistory {
  id: string;
  task_id: string;
  started_at: string;
  finished_at?: string;
  files_total: number;
  files_synced: number;
  bytes_total: number;
  bytes_synced: number;
  status: 'success' | 'failed' | 'cancelled';
  error_message?: string;
}

export interface FileVersion {
  id: string;
  task_id: string;
  filepath: string;
  filename: string;
  version_id: string;
  size: number;
  checksum: string;
  modified_at: string;
  synced_at: string;
  source: 'local' | 'remote';
}

export interface SyncStats {
  total_tasks: number;
  running_tasks: number;
  idle_tasks: number;
  error_tasks: number;
  upload_today: number;
  download_today: number;
  upload_week: number;
  download_week: number;
}

export interface ConflictFile {
  task_id: string;
  filepath: string;
  local_version: FileVersion;
  remote_version: FileVersion;
  strategy: ConflictStrategy;
}

export interface TaskFormData {
  name: string;
  provider_id: string;
  local_path: string;
  remote_path: string;
  direction: SyncDirection;
  conflict_strategy: ConflictStrategy;
  schedule: SyncTask['schedule'];
}
