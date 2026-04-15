/**
 * Cloud Drive Sync - API hooks
 */
import { useState, useEffect, useCallback, useRef } from 'react';
import type {
  SyncTask,
  SyncStats,
  SyncActivity,
  SyncHistory,
  FileVersion,
  CloudProvider,
  TaskFormData,
  ConflictFile,
} from './types';

const API_BASE = '/api/v1/cloud-sync';

/* ───────── helpers ───────── */

async function api<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  });
  const json = await res.json();
  if (json.code !== 0 && json.code !== undefined) {
    throw new Error(json.message || '请求失败');
  }
  return (json.data ?? json) as T;
}

/* ───────── Stats ───────── */

export function useStats(pollMs = 30000) {
  const [stats, setStats] = useState<SyncStats | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api<SyncStats>(`${API_BASE}/stats`);
      setStats(data);
    } catch (e) {
      console.error('加载统计失败:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const id = setInterval(refresh, pollMs);
    return () => clearInterval(id);
  }, [refresh, pollMs]);

  return { stats, loading, refresh };
}

/* ───────── Providers ───────── */

export function useProviders() {
  const [providers, setProviders] = useState<CloudProvider[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api<CloudProvider[]>(`${API_BASE}/providers`);
      setProviders(data);
    } catch (e) {
      console.error('加载云存储列表失败:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const remove = useCallback(async (id: string) => {
    await api(`${API_BASE}/providers/${id}`, { method: 'DELETE' });
    await refresh();
  }, [refresh]);

  const syncProvider = useCallback(async (id: string) => {
    await api(`${API_BASE}/providers/${id}/sync`, { method: 'POST' });
  }, []);

  return { providers, loading, refresh, remove, syncProvider };
}

/* ───────── Sync Tasks ───────── */

export function useSyncTasks() {
  const [tasks, setTasks] = useState<SyncTask[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api<SyncTask[]>(`${API_BASE}/tasks`);
      setTasks(data);
    } catch (e) {
      console.error('加载任务列表失败:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const create = useCallback(async (form: TaskFormData) => {
    await api(`${API_BASE}/tasks`, {
      method: 'POST',
      body: JSON.stringify(form),
    });
    await refresh();
  }, [refresh]);

  const update = useCallback(async (id: string, form: Partial<TaskFormData>) => {
    await api(`${API_BASE}/tasks/${id}`, {
      method: 'PUT',
      body: JSON.stringify(form),
    });
    await refresh();
  }, [refresh]);

  const remove = useCallback(async (id: string) => {
    await api(`${API_BASE}/tasks/${id}`, { method: 'DELETE' });
    await refresh();
  }, [refresh]);

  const trigger = useCallback(async (id: string) => {
    await api(`${API_BASE}/tasks/${id}/sync`, { method: 'POST' });
    await refresh();
  }, [refresh]);

  const pause = useCallback(async (id: string) => {
    await api(`${API_BASE}/tasks/${id}/pause`, { method: 'POST' });
    await refresh();
  }, [refresh]);

  const resume = useCallback(async (id: string) => {
    await api(`${API_BASE}/tasks/${id}/resume`, { method: 'POST' });
    await refresh();
  }, [refresh]);

  const syncAll = useCallback(async () => {
    await api(`${API_BASE}/sync-all`, { method: 'POST' });
    await refresh();
  }, [refresh]);

  return { tasks, loading, refresh, create, update, remove, trigger, pause, resume, syncAll };
}

/* ───────── Activities (WebSocket + REST) ───────── */

export function useActivities(taskId?: string) {
  const [activities, setActivities] = useState<SyncActivity[]>([]);
  const [loading, setLoading] = useState(true);
  const wsRef = useRef<WebSocket | null>(null);

  // REST fallback
  const refresh = useCallback(async () => {
    try {
      const url = taskId ? `${API_BASE}/tasks/${taskId}/activities` : `${API_BASE}/activities`;
      const data = await api<SyncActivity[]>(url);
      setActivities(data);
    } catch (e) {
      console.error('加载活动记录失败:', e);
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  // WebSocket 实时推送
  useEffect(() => {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${proto}//${location.host}${API_BASE}/ws`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === 'activity') {
          setActivities(prev => [msg.data as SyncActivity, ...prev].slice(0, 200));
        } else if (msg.type === 'task_update') {
          // handled by parent via refresh
        }
      } catch { /* ignore */ }
    };

    ws.onerror = () => ws.close();

    return () => { ws.close(); wsRef.current = null; };
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  return { activities, loading, refresh };
}

/* ───────── Sync History ───────── */

export function useSyncHistory(taskId: string) {
  const [history, setHistory] = useState<SyncHistory[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api<SyncHistory[]>(`${API_BASE}/tasks/${taskId}/history`);
      setHistory(data);
    } catch (e) {
      console.error('加载历史记录失败:', e);
    } finally {
      setLoading(false);
    }
  }, [taskId]);

  useEffect(() => { refresh(); }, [refresh]);

  return { history, loading, refresh };
}

/* ───────── File Versions ───────── */

export function useFileVersions(taskId: string, filepath?: string) {
  const [versions, setVersions] = useState<FileVersion[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const params = new URLSearchParams({ task_id: taskId });
      if (filepath) params.set('filepath', filepath);
      const data = await api<FileVersion[]>(`${API_BASE}/versions?${params}`);
      setVersions(data);
    } catch (e) {
      console.error('加载文件版本失败:', e);
    } finally {
      setLoading(false);
    }
  }, [taskId, filepath]);

  useEffect(() => { refresh(); }, [refresh]);

  const restore = useCallback(async (versionId: string) => {
    await api(`${API_BASE}/versions/${versionId}/restore`, { method: 'POST' });
    await refresh();
  }, [refresh]);

  const remove = useCallback(async (versionId: string) => {
    await api(`${API_BASE}/versions/${versionId}`, { method: 'DELETE' });
    await refresh();
  }, [refresh]);

  return { versions, loading, refresh, restore, remove };
}

/* ───────── Conflicts ───────── */

export function useConflicts() {
  const [conflicts, setConflicts] = useState<ConflictFile[]>([]);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const data = await api<ConflictFile[]>(`${API_BASE}/conflicts`);
      setConflicts(data);
    } catch (e) {
      console.error('加载冲突列表失败:', e);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { refresh(); }, [refresh]);

  const resolve = useCallback(async (taskId: string, filepath: string, choice: 'local' | 'remote' | 'keep_both') => {
    await api(`${API_BASE}/conflicts/resolve`, {
      method: 'POST',
      body: JSON.stringify({ task_id: taskId, filepath, choice }),
    });
    await refresh();
  }, [refresh]);

  return { conflicts, loading, refresh, resolve };
}
