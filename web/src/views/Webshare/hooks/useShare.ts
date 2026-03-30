import { useState, useCallback, useEffect } from 'react';
import {
  Share,
  CreateShareRequest,
  ShareResponse,
  ShareListResponse,
} from '../types';

interface UseShareResult {
  shares: Share[];
  loading: boolean;
  error: Error | null;
  currentShare: Share | null;
  shareUrl: string | null;
  
  // Actions
  createShare: (request: CreateShareRequest) => Promise<ShareResponse | null>;
  getShare: (id: string) => Promise<Share | null>;
  getShareByToken: (token: string) => Promise<Share | null>;
  updateShare: (id: string, updates: Partial<Share>) => Promise<Share | null>;
  deleteShare: (id: string) => Promise<boolean>;
  revokeShare: (id: string) => Promise<boolean>;
  listShares: (page?: number, pageSize?: number) => Promise<void>;
  verifyPassword: (token: string, password: string) => Promise<boolean>;
  clearCurrentShare: () => void;
}

export function useShare(): UseShareResult {
  const [shares, setShares] = useState<Share[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [currentShare, setCurrentShare] = useState<Share | null>(null);
  const [shareUrl, setShareUrl] = useState<string | null>(null);

  // 创建分享
  const createShare = useCallback(async (request: CreateShareRequest): Promise<ShareResponse | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch('/api/v1/webshare/shares', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.message || `HTTP error: ${response.status}`);
      }

      const data: ShareResponse = await response.json();
      setCurrentShare(data.share);
      setShareUrl(data.shareUrl);
      
      // 更新列表
      setShares(prev => [data.share, ...prev]);
      
      return data;
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to create share'));
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  // 获取分享详情
  const getShare = useCallback(async (id: string): Promise<Share | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/v1/webshare/shares/${id}`);
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const share: Share = await response.json();
      setCurrentShare(share);
      return share;
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to get share'));
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  // 通过 token 获取分享
  const getShareByToken = useCallback(async (token: string): Promise<Share | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/v1/webshare/public/${token}`);
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const share: Share = await response.json();
      setCurrentShare(share);
      return share;
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to get share'));
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  // 更新分享
  const updateShare = useCallback(async (id: string, updates: Partial<Share>): Promise<Share | null> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/v1/webshare/shares/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(updates),
      });

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const share: Share = await response.json();
      setCurrentShare(share);
      
      // 更新列表中的项
      setShares(prev => prev.map(s => s.id === id ? share : s));
      
      return share;
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to update share'));
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

  // 删除分享
  const deleteShare = useCallback(async (id: string): Promise<boolean> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/v1/webshare/shares/${id}`, {
        method: 'DELETE',
      });

      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      setShares(prev => prev.filter(s => s.id !== id));
      
      if (currentShare?.id === id) {
        setCurrentShare(null);
        setShareUrl(null);
      }
      
      return true;
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to delete share'));
      return false;
    } finally {
      setLoading(false);
    }
  }, [currentShare]);

  // 撤销分享
  const revokeShare = useCallback(async (id: string): Promise<boolean> => {
    return updateShare(id, { isActive: false }).then(share => share !== null);
  }, [updateShare]);

  // 获取分享列表
  const listShares = useCallback(async (page: number = 1, pageSize: number = 20): Promise<void> => {
    setLoading(true);
    setError(null);

    try {
      const url = new URL('/api/v1/webshare/shares', window.location.origin);
      url.searchParams.set('page', String(page));
      url.searchParams.set('pageSize', String(pageSize));

      const response = await fetch(url.toString());
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }

      const data: ShareListResponse = await response.json();
      setShares(data.shares);
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to list shares'));
    } finally {
      setLoading(false);
    }
  }, []);

  // 验证分享密码
  const verifyPassword = useCallback(async (token: string, password: string): Promise<boolean> => {
    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/v1/webshare/public/${token}/verify`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      });

      if (!response.ok) {
        return false;
      }

      const result = await response.json();
      return result.valid === true;
    } catch {
      return false;
    } finally {
      setLoading(false);
    }
  }, []);

  // 清除当前分享
  const clearCurrentShare = useCallback(() => {
    setCurrentShare(null);
    setShareUrl(null);
  }, []);

  return {
    shares,
    loading,
    error,
    currentShare,
    shareUrl,
    createShare,
    getShare,
    getShareByToken,
    updateShare,
    deleteShare,
    revokeShare,
    listShares,
    verifyPassword,
    clearCurrentShare,
  };
}