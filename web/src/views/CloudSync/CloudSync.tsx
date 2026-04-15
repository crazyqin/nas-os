/**
 * Cloud Drive Sync - 主界面
 * 包含：任务列表、新建/编辑、详情、版本浏览 四个子视图
 */
import React, { useState, useEffect, useCallback } from 'react';
import { useStats, useSyncTasks, useProviders, useActivities, useSyncHistory, useFileVersions, useConflicts } from './hooks';
import type { SyncTask, TaskFormData, FileVersion, ConflictFile, SyncHistory } from './types';
import './CloudSync.css';

/* ───────── 工具函数 ───────── */
function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatSpeed(bps: number): string {
  return formatSize(bps) + '/s';
}

function formatEta(seconds: number): string {
  if (seconds <= 0) return '--';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
}

function formatTime(iso: string): string {
  if (!iso) return '--';
  const d = new Date(iso);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  if (diff < 60000) return '刚刚';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
  return d.toLocaleDateString('zh-CN');
}

const STATUS_TEXT: Record<string, string> = {
  running: '同步中',
  idle: '空闲',
  paused: '已暂停',
  error: '错误',
};

const STATUS_CLASS: Record<string, string> = {
  running: 'syncing',
  idle: 'synced',
  paused: 'paused',
  error: 'error',
};

const DIR_TEXT: Record<string, string> = {
  bidirectional: '双向同步',
  upload: '仅上传',
  download: '仅下载',
};

const SCHEDULE_TEXT: Record<string, string> = {
  realtime: '实时',
  hourly: '每小时',
  daily: '每天',
  weekly: '每周',
  manual: '手动',
};

const CONFLICT_TEXT: Record<string, string> = {
  newer: '保留较新',
  bigger: '保留较大',
  keep_both: '保留两份',
  local: '保留本地',
  remote: '保留云端',
};

/* ───────── 统计卡 ───────── */
function StatsBar({ stats }: { stats: ReturnType<typeof useStats>['stats'] }) {
  if (!stats) return null;
  return (
    <div className="cs-stats-bar">
      <div className="cs-stat-card">
        <div className="cs-stat-label">同步任务</div>
        <div className="cs-stat-value">{stats.total_tasks}</div>
        <div className="cs-stat-sub">
          <span className="cs-dot running" />{stats.running_tasks} 运行中 &nbsp;
          <span className="cs-dot error" />{stats.error_tasks} 错误
        </div>
      </div>
      <div className="cs-stat-card">
        <div className="cs-stat-label">今日上传</div>
        <div className="cs-stat-value success">{formatSize(stats.upload_today)}</div>
        <div className="cs-stat-sub">本周 {formatSize(stats.upload_week)}</div>
      </div>
      <div className="cs-stat-card">
        <div className="cs-stat-label">今日下载</div>
        <div className="cs-stat-value">{formatSize(stats.download_today)}</div>
        <div className="cs-stat-sub">本周 {formatSize(stats.download_week)}</div>
      </div>
    </div>
  );
}

/* ───────── 进度条 ───────── */
function ProgressBar({ value, size = 'normal' }: { value: number; size?: 'normal' | 'large' }) {
  return (
    <div className={`cs-progress ${size}`}>
      <div className="cs-progress-fill" style={{ width: `${Math.min(100, Math.max(0, value))}%` }} />
      <span className="cs-progress-text">{value}%</span>
    </div>
  );
}

/* ───────── Tab 切换 ───────── */
type SubView = 'list' | 'detail' | 'versions' | 'conflicts';

function TabNav({ active, onChange }: { active: SubView; onChange: (v: SubView) => void }) {
  const tabs: { key: SubView; label: string }[] = [
    { key: 'list', label: '📋 同步任务' },
    { key: 'conflicts', label: '⚠️ 冲突处理' },
  ];
  return (
    <div className="cs-tabs">
      {tabs.map(t => (
        <button
          key={t.key}
          className={`cs-tab ${active === t.key ? 'active' : ''}`}
          onClick={() => onChange(t.key)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

/* ───────── 任务表单弹窗 ───────── */
function TaskFormModal({
  task,
  providers,
  onSave,
  onClose,
}: {
  task?: SyncTask;
  providers: { id: string; name: string }[];
  onSave: (data: TaskFormData) => void;
  onClose: () => void;
}) {
  const [form, setForm] = useState<TaskFormData>({
    name: task?.name ?? '',
    provider_id: task?.provider_id ?? '',
    local_path: task?.local_path ?? '',
    remote_path: task?.remote_path ?? '',
    direction: task?.direction ?? 'bidirectional',
    conflict_strategy: task?.conflict_strategy ?? 'newer',
    schedule: task?.schedule ?? 'realtime',
  });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');

  const set = (key: keyof TaskFormData, val: string) =>
    setForm(prev => ({ ...prev, [key]: val }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name || !form.provider_id || !form.local_path) {
      setError('请填写必填项');
      return;
    }
    setSaving(true);
    setError('');
    try {
      await onSave(form);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '保存失败');
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="cs-modal-overlay" onClick={e => e.target === e.currentTarget && onClose()}>
      <div className="cs-modal">
        <div className="cs-modal-header">
          <h3>{task ? '编辑同步任务' : '新建同步任务'}</h3>
          <button className="cs-modal-close" onClick={onClose}>&times;</button>
        </div>
        <form onSubmit={handleSubmit}>
          <div className="cs-modal-body">
            {error && <div className="cs-form-error">{error}</div>}

            <div className="cs-form-group">
              <label className="cs-label">任务名称 *</label>
              <input className="cs-input" value={form.name} onChange={e => set('name', e.target.value)} placeholder="例如：照片备份" />
            </div>

            <div className="cs-form-group">
              <label className="cs-label">云存储 *</label>
              <select className="cs-select" value={form.provider_id} onChange={e => set('provider_id', e.target.value)}>
                <option value="">选择云存储...</option>
                {providers.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}
              </select>
            </div>

            <div className="cs-form-row">
              <div className="cs-form-group">
                <label className="cs-label">本地路径 *</label>
                <input className="cs-input" value={form.local_path} onChange={e => set('local_path', e.target.value)} placeholder="/data/photos" />
              </div>
              <div className="cs-form-group">
                <label className="cs-label">云端路径</label>
                <input className="cs-input" value={form.remote_path} onChange={e => set('remote_path', e.target.value)} placeholder="/backup/photos" />
              </div>
            </div>

            <div className="cs-form-group">
              <label className="cs-label">同步方向</label>
              <div className="cs-radio-group">
                {(['bidirectional', 'upload', 'download'] as const).map(d => (
                  <label key={d} className="cs-radio-label">
                    <input type="radio" name="direction" value={d} checked={form.direction === d} onChange={() => set('direction', d)} />
                    <span>{DIR_TEXT[d]}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="cs-form-group">
              <label className="cs-label">冲突处理策略</label>
              <select className="cs-select" value={form.conflict_strategy} onChange={e => set('conflict_strategy', e.target.value as typeof form.conflict_strategy)}>
                <option value="newer">保留较新版本</option>
                <option value="bigger">保留较大文件</option>
                <option value="keep_both">保留两份</option>
                <option value="local">保留本地版本</option>
                <option value="remote">保留云端版本</option>
              </select>
            </div>

            <div className="cs-form-group">
              <label className="cs-label">同步计划</label>
              <select className="cs-select" value={form.schedule} onChange={e => set('schedule', e.target.value as typeof form.schedule)}>
                <option value="realtime">实时同步</option>
                <option value="hourly">每小时</option>
                <option value="daily">每天</option>
                <option value="weekly">每周</option>
                <option value="manual">手动触发</option>
              </select>
            </div>
          </div>
          <div className="cs-modal-footer">
            <button type="button" className="cs-btn cs-btn-secondary" onClick={onClose}>取消</button>
            <button type="submit" className="cs-btn cs-btn-primary" disabled={saving}>
              {saving ? '保存中...' : task ? '保存更改' : '创建任务'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

/* ───────── 任务详情视图 ───────── */
function TaskDetailView({
  task,
  onBack,
  onVersions,
}: {
  task: SyncTask;
  onBack: () => void;
  onVersions: () => void;
}) {
  const { activities } = useActivities(task.id);
  const { history } = useSyncHistory(task.id);

  return (
    <div className="cs-detail">
      <div className="cs-detail-header">
        <button className="cs-btn cs-btn-secondary" onClick={onBack}>← 返回</button>
        <div className="cs-detail-title">
          <h2>{task.name}</h2>
          <span className={`cs-badge ${STATUS_CLASS[task.status]}`}>{STATUS_TEXT[task.status]}</span>
        </div>
        <button className="cs-btn cs-btn-primary" onClick={onVersions}>📜 版本历史</button>
      </div>

      {/* 进度卡 */}
      <div className="cs-detail-grid">
        <div className="cs-info-card">
          <div className="cs-info-label">同步进度</div>
          <ProgressBar value={task.progress} size="large" />
          <div className="cs-info-row">
            <span>已同步 {task.files_synced} / {task.files_total} 文件</span>
            <span>{formatSize(task.bytes_synced)} / {formatSize(task.bytes_total)}</span>
          </div>
        </div>
        <div className="cs-info-card">
          <div className="cs-info-label">实时速度</div>
          <div className="cs-speed">{task.status === 'running' ? formatSpeed(task.speed_bps) : '--'}</div>
          <div className="cs-info-row">
            <span>预计剩余: {formatEta(task.eta_seconds)}</span>
          </div>
        </div>
        <div className="cs-info-card">
          <div className="cs-info-label">配置信息</div>
          <div className="cs-info-row"><span>方向</span><span>{DIR_TEXT[task.direction]}</span></div>
          <div className="cs-info-row"><span>冲突策略</span><span>{CONFLICT_TEXT[task.conflict_strategy]}</span></div>
          <div className="cs-info-row"><span>计划</span><span>{SCHEDULE_TEXT[task.schedule]}</span></div>
        </div>
        <div className="cs-info-card">
          <div className="cs-info-label">路径</div>
          <div className="cs-info-row"><span>本地</span><span className="cs-path">{task.local_path}</span></div>
          <div className="cs-info-row"><span>云端</span><span className="cs-path">{task.remote_path}</span></div>
          {task.error_message && (
            <div className="cs-error-msg">⚠️ {task.error_message}</div>
          )}
        </div>
      </div>

      {/* 历史运行记录 */}
      <div className="cs-section">
        <h3>📊 运行历史</h3>
        <div className="cs-history-list">
          {history.length === 0 ? (
            <div className="cs-empty">暂无历史记录</div>
          ) : (
            history.map(h => (
              <div key={h.id} className={`cs-history-item ${h.status}`}>
                <div className="cs-history-icon">
                  {h.status === 'success' ? '✅' : h.status === 'failed' ? '❌' : '⏹️'}
                </div>
                <div className="cs-history-info">
                  <div className="cs-history-title">
                    {new Date(h.started_at).toLocaleString('zh-CN')}
                  </div>
                  <div className="cs-history-meta">
                    {h.files_synced}/{h.files_total} 文件 · {formatSize(h.bytes_synced)}
                    {h.error_message && <span className="cs-error-inline"> · {h.error_message}</span>}
                  </div>
                </div>
                <div className="cs-history-duration">
                  {h.finished_at
                    ? formatEta((new Date(h.finished_at).getTime() - new Date(h.started_at).getTime()) / 1000)
                    : '--'}
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* 实时活动 */}
      <div className="cs-section">
        <h3>📡 实时活动</h3>
        <div className="cs-activity-list">
          {activities.length === 0 ? (
            <div className="cs-empty">暂无活动记录</div>
          ) : (
            activities.slice(0, 50).map(a => (
              <div key={a.id} className={`cs-activity-item ${a.type}`}>
                <div className="cs-activity-icon">
                  {a.type === 'upload' ? '⬆️' : a.type === 'download' ? '⬇️' : a.type === 'delete' ? '🗑️' : a.type === 'error' ? '❌' : '⚡'}
                </div>
                <div className="cs-activity-info">
                  <div className="cs-activity-name">{a.filename}</div>
                  <div className="cs-activity-meta">{a.filepath} · {formatSize(a.size)} · {formatTime(a.timestamp)}</div>
                </div>
                <div className={`cs-activity-status ${a.status}`}>
                  {a.status === 'success' ? '成功' : a.status === 'failed' ? '失败' : '跳过'}
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );
}

/* ───────── 文件版本视图 ───────── */
function VersionsView({
  taskId,
  onBack,
}: {
  taskId: string;
  onBack: () => void;
}) {
  const { versions, loading, restore, remove } = useFileVersions(taskId);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [restoring, setRestoring] = useState<string | null>(null);

  const grouped = versions.reduce<Record<string, FileVersion[]>>((acc, v) => {
    const key = v.filepath;
    if (!acc[key]) acc[key] = [];
    acc[key].push(v);
    return acc;
  }, {});

  const handleRestore = async (v: FileVersion) => {
    setRestoring(v.id);
    try {
      await restore(v.id);
    } finally {
      setRestoring(null);
    }
  };

  return (
    <div className="cs-versions">
      <div className="cs-detail-header">
        <button className="cs-btn cs-btn-secondary" onClick={onBack}>← 返回</button>
        <h2>📜 文件版本历史</h2>
      </div>

      {loading ? (
        <div className="cs-loading">加载中...</div>
      ) : Object.keys(grouped).length === 0 ? (
        <div className="cs-empty">暂无版本记录</div>
      ) : (
        Object.entries(grouped).map(([filepath, vers]) => (
          <div key={filepath} className="cs-version-file">
            <div className="cs-version-file-header" onClick={() => setSelectedFile(selectedFile === filepath ? null : filepath)}>
              <span className="cs-version-icon">📄</span>
              <span className="cs-version-path">{filepath}</span>
              <span className="cs-version-count">{vers.length} 个版本</span>
              <span className="cs-version-toggle">{selectedFile === filepath ? '▲' : '▼'}</span>
            </div>
            {selectedFile === filepath && (
              <div className="cs-version-list">
                {vers
                  .sort((a, b) => new Date(b.modified_at).getTime() - new Date(a.modified_at).getTime())
                  .map((v, idx) => (
                    <div key={v.id} className={`cs-version-item ${idx === 0 ? 'current' : ''}`}>
                      <div className="cs-version-info">
                        <div className="cs-version-name">
                          {idx === 0 && <span className="cs-current-badge">当前</span>}
                          {v.filename}
                        </div>
                        <div className="cs-version-meta">
                          {formatSize(v.size)} · {formatTime(v.synced_at)} · {v.source === 'local' ? '本地' : '云端'}
                        </div>
                        <div className="cs-version-checksum">MD5: {v.checksum}</div>
                      </div>
                      <div className="cs-version-actions">
                        <button
                          className="cs-btn cs-btn-primary cs-btn-sm"
                          onClick={() => handleRestore(v)}
                          disabled={restoring === v.id || idx === 0}
                        >
                          {restoring === v.id ? '恢复中...' : idx === 0 ? '已是当前' : '恢复此版本'}
                        </button>
                        <button
                          className="cs-btn cs-btn-danger cs-btn-sm"
                          onClick={() => remove(v.id)}
                        >
                          删除
                        </button>
                      </div>
                    </div>
                  ))}
              </div>
            )}
          </div>
        ))
      )}
    </div>
  );
}

/* ───────── 冲突处理视图 ───────── */
function ConflictsView({ conflicts }: { conflicts: ConflictFile[] }) {
  const { resolve } = useConflicts();

  if (conflicts.length === 0) {
    return (
      <div className="cs-empty-state">
        <div className="cs-empty-icon">✅</div>
        <div className="cs-empty-title">暂无冲突文件</div>
        <p>所有文件已成功同步</p>
      </div>
    );
  }

  return (
    <div className="cs-conflicts">
      <div className="cs-conflicts-header">
        <h3>⚠️ {conflicts.length} 个冲突文件待处理</h3>
        <p className="cs-conflicts-desc">选择保留哪个版本来解决冲突</p>
      </div>
      <div className="cs-conflicts-grid">
        {conflicts.map(cf => (
          <div key={`${cf.task_id}-${cf.filepath}`} className="cs-conflict-card">
            <div className="cs-conflict-filename">{cf.filepath}</div>
            <div className="cs-conflict-versions">
              <div className="cs-conflict-version local">
                <div className="cs-conflict-version-header">📱 本地版本</div>
                <div className="cs-conflict-version-info">
                  {formatSize(cf.local_version.size)} · {formatTime(cf.local_version.modified_at)}
                </div>
                <button
                  className="cs-btn cs-btn-primary"
                  onClick={() => resolve(cf.task_id, cf.filepath, 'local')}
                >
                  保留本地
                </button>
              </div>
              <div className="cs-conflict-divider">VS</div>
              <div className="cs-conflict-version remote">
                <div className="cs-conflict-version-header">☁️ 云端版本</div>
                <div className="cs-conflict-version-info">
                  {formatSize(cf.remote_version.size)} · {formatTime(cf.remote_version.modified_at)}
                </div>
                <button
                  className="cs-btn cs-btn-primary"
                  onClick={() => resolve(cf.task_id, cf.filepath, 'remote')}
                >
                  保留云端
                </button>
              </div>
            </div>
            <button
              className="cs-btn cs-btn-secondary"
              style={{ marginTop: '0.75rem', width: '100%' }}
              onClick={() => resolve(cf.task_id, cf.filepath, 'keep_both')}
            >
              保留两份
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ───────── 任务列表视图 ───────── */
function TaskListView({
  tasks,
  providers,
  onNew,
  onEdit,
  onDetail,
  onDelete,
  onTrigger,
  onPause,
  onResume,
  onSyncAll,
}: {
  tasks: SyncTask[];
  providers: { id: string; name: string }[];
  onNew: () => void;
  onEdit: (t: SyncTask) => void;
  onDetail: (t: SyncTask) => void;
  onDelete: (t: SyncTask) => void;
  onTrigger: (t: SyncTask) => void;
  onPause: (t: SyncTask) => void;
  onResume: (t: SyncTask) => void;
  onSyncAll: () => void;
}) {
  const getProviderName = (id: string) => providers.find(p => p.id === id)?.name ?? id;

  return (
    <div className="cs-task-list-view">
      <div className="cs-list-toolbar">
        <h3>同步任务列表 ({tasks.length})</h3>
        <div className="cs-toolbar-actions">
          <button className="cs-btn cs-btn-secondary" onClick={onSyncAll}>🔄 全部同步</button>
          <button className="cs-btn cs-btn-primary" onClick={onNew}>+ 新建任务</button>
        </div>
      </div>

      {tasks.length === 0 ? (
        <div className="cs-empty-state">
          <div className="cs-empty-icon">☁️</div>
          <div className="cs-empty-title">暂无同步任务</div>
          <p>创建同步任务以自动同步文件到云端</p>
          <button className="cs-btn cs-btn-primary" onClick={onNew}>+ 新建任务</button>
        </div>
      ) : (
        <div className="cs-task-table">
          <div className="cs-task-thead">
            <div>任务名称</div>
            <div>云存储</div>
            <div>方向</div>
            <div>状态</div>
            <div>进度</div>
            <div>操作</div>
          </div>
          {tasks.map(t => (
            <div key={t.id} className={`cs-task-row ${t.status}`}>
              <div className="cs-task-name-cell">
                <span className="cs-task-icon">☁️</span>
                <div>
                  <div className="cs-task-name">{t.name}</div>
                  <div className="cs-task-path">{t.local_path}</div>
                </div>
              </div>
              <div>{getProviderName(t.provider_id)}</div>
              <div>{DIR_TEXT[t.direction]}</div>
              <div>
                <span className={`cs-badge ${STATUS_CLASS[t.status]}`}>{STATUS_TEXT[t.status]}</span>
                {t.last_run && <div className="cs-task-lastrun">上次 {formatTime(t.last_run)}</div>}
              </div>
              <div>
                <ProgressBar value={t.progress} />
                <div className="cs-task-files">{t.files_synced}/{t.files_total} 文件</div>
              </div>
              <div className="cs-task-actions">
                <button className="cs-btn cs-btn-sm cs-btn-secondary" title="详情" onClick={() => onDetail(t)}>📋</button>
                {t.status === 'idle' && <button className="cs-btn cs-btn-sm cs-btn-secondary" title="立即同步" onClick={() => onTrigger(t)}>▶️</button>}
                {t.status === 'running' && <button className="cs-btn cs-btn-sm cs-btn-secondary" title="暂停" onClick={() => onPause(t)}>⏸️</button>}
                {t.status === 'paused' && <button className="cs-btn cs-btn-sm cs-btn-secondary" title="继续" onClick={() => onResume(t)}>▶️</button>}
                <button className="cs-btn cs-btn-sm cs-btn-secondary" title="编辑" onClick={() => onEdit(t)}>⚙️</button>
                <button className="cs-btn cs-btn-sm cs-btn-danger" title="删除" onClick={() => onDelete(t)}>🗑️</button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/* ───────── 主组件 ───────── */
const CloudSync: React.FC = () => {
  const { stats } = useStats();
  const { providers } = useProviders();
  const { tasks, create, update, remove, trigger, pause, resume, syncAll } = useSyncTasks();
  const { conflicts } = useConflicts();

  const [subView, setSubView] = useState<SubView>('list');
  const [detailTask, setDetailTask] = useState<SyncTask | null>(null);
  const [editTask, setEditTask] = useState<SyncTask | null>(null);
  const [showNew, setShowNew] = useState(false);

  // 详情页 → 版本页
  const handleShowVersions = useCallback(() => {
    if (detailTask) {
      setSubView('versions');
    }
  }, [detailTask]);

  const handleDetail = useCallback((t: SyncTask) => {
    setDetailTask(t);
    setSubView('detail');
  }, []);

  const handleEdit = useCallback((t: SyncTask) => {
    setEditTask(t);
    setShowNew(true);
  }, []);

  const handleNew = useCallback(() => {
    setEditTask(null);
    setShowNew(true);
  }, []);

  const handleSave = useCallback(async (form: TaskFormData) => {
    if (editTask) {
      await update(editTask.id, form);
    } else {
      await create(form);
    }
    setShowNew(false);
    setEditTask(null);
  }, [editTask, create, update]);

  const handleDelete = useCallback(async (t: SyncTask) => {
    if (!window.confirm(`确定要删除同步任务"${t.name}"吗？`)) return;
    await remove(t.id);
  }, [remove]);

  const handleTrigger = useCallback(async (t: SyncTask) => {
    await trigger(t.id);
  }, [trigger]);

  const handlePause = useCallback(async (t: SyncTask) => {
    await pause(t.id);
  }, [pause]);

  const handleResume = useCallback(async (t: SyncTask) => {
    await resume(t.id);
  }, [resume]);

  const handleSyncAll = useCallback(async () => {
    await syncAll();
  }, [syncAll]);

  return (
    <div className="cs-root">
      {/* Header */}
      <div className="cs-page-header">
        <div>
          <h1 className="cs-page-title">☁️ Cloud Drive Sync</h1>
          <p className="cs-page-desc">管理云存储连接与文件同步任务</p>
        </div>
      </div>

      {/* 统计条 */}
      <StatsBar stats={stats} />

      {/* Tab 导航 */}
      <TabNav active={subView} onChange={v => setSubView(v === 'list' ? 'list' : v)} />

      {/* 列表视图 */}
      {subView === 'list' && (
        <>
          <TaskListView
            tasks={tasks}
            providers={providers}
            onNew={handleNew}
            onEdit={handleEdit}
            onDetail={handleDetail}
            onDelete={handleDelete}
            onTrigger={handleTrigger}
            onPause={handlePause}
            onResume={handleResume}
            onSyncAll={handleSyncAll}
          />
          {/* 冲突处理入口 */}
          {conflicts.length > 0 && (
            <div className="cs-conflict-banner" onClick={() => setSubView('conflicts')}>
              ⚠️ 有 {conflicts.length} 个文件存在冲突，请尽快处理
            </div>
          )}
        </>
      )}

      {/* 冲突视图 */}
      {subView === 'conflicts' && <ConflictsView conflicts={conflicts} />}

      {/* 详情视图 */}
      {subView === 'detail' && detailTask && (
        <TaskDetailView
          task={detailTask}
          onBack={() => setSubView('list')}
          onVersions={handleShowVersions}
        />
      )}

      {/* 版本视图 */}
      {subView === 'versions' && detailTask && (
        <VersionsView
          taskId={detailTask.id}
          onBack={() => setSubView('detail')}
        />
      )}

      {/* 任务表单弹窗 */}
      {showNew && (
        <TaskFormModal
          task={editTask ?? undefined}
          providers={providers}
          onSave={handleSave}
          onClose={() => { setShowNew(false); setEditTask(null); }}
        />
      )}
    </div>
  );
};

export default CloudSync;
