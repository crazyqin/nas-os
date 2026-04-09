/**
 * FRP WebUI 前端 - 隧道管理界面
 * 对标飞牛FN Connect WebUI
 * 
 * 功能:
 * - 隧道列表/状态展示
 * - 创建/编辑/删除隧道
 * - 一键连接/断开
 * - 节点选择（自动测速选优）
 * - 实时状态推送（WebSocket）
 */
import React, { useState, useEffect, useCallback } from 'react';
import './FRPManager.css';

// 类型定义
interface Tunnel {
  id: string;
  name: string;
  type: 'tcp' | 'udp' | 'http' | 'https' | 'stcp' | 'xtcp';
  client_id: string;
  status: 'running' | 'stopped' | 'error' | 'connecting';
  local_addr: string;
  remote_addr?: string;
  public_url?: string;
  bytes_sent: number;
  bytes_recv: number;
  connections: number;
  last_active: string;
  enabled: boolean;
}

interface Node {
  id: string;
  name: string;
  region: string;
  host: string;
  port: number;
  load: number;
  latency: number;
  status: 'online' | 'offline' | 'maintenance';
}

interface ClientStatus {
  id: string;
  name: string;
  status: string;
  tunnels: number;
  uptime: number;
  bytes_sent: number;
  bytes_recv: number;
}

const FRPManager: React.FC = () => {
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [clients, setClients] = useState<ClientStatus[]>([]);
  const [selectedTunnel, setSelectedTunnel] = useState<Tunnel | null>(null);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [loading, setLoading] = useState(true);
  const [wsConnected, setWsConnected] = useState(false);
  const [activeTab, setActiveTab] = useState<'tunnels' | 'nodes' | 'clients'>('tunnels');

  // WebSocket 连接
  useEffect(() => {
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/frp/ws`);
    
    ws.onopen = () => {
      setWsConnected(true);
      console.log('FRP WebSocket connected');
    };
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleWSEvent(data);
    };
    
    ws.onclose = () => {
      setWsConnected(false);
      console.log('FRP WebSocket disconnected');
    };
    
    return () => ws.close();
  }, []);

  const handleWSEvent = useCallback((event: { type: string; data: any }) => {
    switch (event.type) {
      case 'tunnel_status':
        setTunnels(prev => 
          prev.map(t => t.id === event.data.id ? { ...t, ...event.data } : t)
        );
        break;
      case 'tunnel_created':
        setTunnels(prev => [...prev, event.data]);
        break;
      case 'tunnel_deleted':
        setTunnels(prev => prev.filter(t => t.id !== event.data.id));
        break;
      case 'client_status':
        setClients(prev => {
          const idx = prev.findIndex(c => c.id === event.data.id);
          if (idx >= 0) {
            const updated = [...prev];
            updated[idx] = { ...updated[idx], ...event.data };
            return updated;
          }
          return [...prev, event.data];
        });
        break;
    }
  }, []);

  // 加载初始数据
  useEffect(() => {
    fetchTunnels();
    fetchNodes();
    fetchClients();
  }, []);

  const fetchTunnels = async () => {
    try {
      const res = await fetch('/api/v1/frp/tunnels');
      const data = await res.json();
      setTunnels(data.tunnels || []);
    } catch (err) {
      console.error('Failed to fetch tunnels:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchNodes = async () => {
    try {
      const res = await fetch('/api/v1/frp/nodes');
      const data = await res.json();
      setNodes(data.nodes || []);
    } catch (err) {
      console.error('Failed to fetch nodes:', err);
    }
  };

  const fetchClients = async () => {
    try {
      const res = await fetch('/api/v1/frp/clients/status');
      const data = await res.json();
      setClients(data.clients || []);
    } catch (err) {
      console.error('Failed to fetch clients:', err);
    }
  };

  // 隧道操作
  const startTunnel = async (id: string) => {
    try {
      await fetch(`/api/v1/frp/tunnels/${id}/start`, { method: 'POST' });
    } catch (err) {
      console.error('Failed to start tunnel:', err);
    }
  };

  const stopTunnel = async (id: string) => {
    try {
      await fetch(`/api/v1/frp/tunnels/${id}/stop`, { method: 'POST' });
    } catch (err) {
      console.error('Failed to stop tunnel:', err);
    }
  };

  const deleteTunnel = async (id: string) => {
    if (!confirm('确定要删除这个隧道吗？')) return;
    try {
      await fetch(`/api/v1/frp/tunnels/${id}`, { method: 'DELETE' });
      setTunnels(prev => prev.filter(t => t.id !== id));
    } catch (err) {
      console.error('Failed to delete tunnel:', err);
    }
  };

  // 一键连接最佳节点
  const quickConnect = async () => {
    try {
      const res = await fetch('/api/v1/frp/nodes/best');
      const bestNode = await res.json();
      await fetch('/api/v1/frp/quick-connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ node_id: bestNode.id }),
      });
    } catch (err) {
      console.error('Quick connect failed:', err);
    }
  };

  // 格式化流量
  const formatBytes = (bytes: number): string => {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
    return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
  };

  // 状态颜色
  const statusColor = (status: string): string => {
    switch (status) {
      case 'running': return '#52c41a';
      case 'stopped': return '#8c8c8c';
      case 'error': return '#ff4d4f';
      case 'connecting': return '#faad14';
      default: return '#8c8c8c';
    }
  };

  // 隧道类型标签
  const tunnelTypeLabel = (type: string): string => {
    const labels: Record<string, string> = {
      tcp: 'TCP',
      udp: 'UDP',
      http: 'HTTP',
      https: 'HTTPS',
      stcp: 'STCP',
      xtcp: 'XTCP',
    };
    return labels[type] || type.toUpperCase();
  };

  if (loading) {
    return <div className="frp-loading">加载中...</div>;
  }

  return (
    <div className="frp-manager">
      <header className="frp-header">
        <h1>
          <span className="frp-icon">🔗</span>
          FRP 内网穿透
          <span className={`ws-status ${wsConnected ? 'connected' : 'disconnected'}`}>
            {wsConnected ? '●' : '○'}
          </span>
        </h1>
        <div className="frp-actions">
          <button className="btn-primary" onClick={() => setShowCreateModal(true)}>
            + 创建隧道
          </button>
          <button className="btn-secondary" onClick={quickConnect}>
            ⚡ 一键连接
          </button>
        </div>
      </header>

      {/* Tab导航 */}
      <nav className="frp-tabs">
        <button 
          className={activeTab === 'tunnels' ? 'active' : ''} 
          onClick={() => setActiveTab('tunnels')}
        >
          隧道 ({tunnels.length})
        </button>
        <button 
          className={activeTab === 'nodes' ? 'active' : ''} 
          onClick={() => setActiveTab('nodes')}
        >
          节点 ({nodes.length})
        </button>
        <button 
          className={activeTab === 'clients' ? 'active' : ''} 
          onClick={() => setActiveTab('clients')}
        >
          客户端 ({clients.length})
        </button>
      </nav>

      {/* 隧道列表 */}
      {activeTab === 'tunnels' && (
        <div className="tunnel-list">
          {tunnels.length === 0 ? (
            <div className="empty-state">
              <p>暂无隧道配置</p>
              <button className="btn-primary" onClick={() => setShowCreateModal(true)}>
                创建第一个隧道
              </button>
            </div>
          ) : (
            <table className="tunnel-table">
              <thead>
                <tr>
                  <th>名称</th>
                  <th>类型</th>
                  <th>本地地址</th>
                  <th>公网地址</th>
                  <th>状态</th>
                  <th>流量 ↑/↓</th>
                  <th>连接数</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {tunnels.map(tunnel => (
                  <tr key={tunnel.id} className={tunnel.status === 'error' ? 'error-row' : ''}>
                    <td>{tunnel.name}</td>
                    <td>
                      <span className={`type-badge type-${tunnel.type}`}>
                        {tunnelTypeLabel(tunnel.type)}
                      </span>
                    </td>
                    <td>{tunnel.local_addr}</td>
                    <td>
                      {tunnel.public_url ? (
                        <a href={tunnel.public_url} target="_blank" rel="noopener noreferrer">
                          {tunnel.public_url}
                        </a>
                      ) : tunnel.remote_addr || '-'}
                    </td>
                    <td>
                      <span className="status-dot" style={{ backgroundColor: statusColor(tunnel.status) }} />
                      {tunnel.status}
                    </td>
                    <td>
                      {formatBytes(tunnel.bytes_sent)} / {formatBytes(tunnel.bytes_recv)}
                    </td>
                    <td>{tunnel.connections}</td>
                    <td className="actions">
                      {tunnel.status === 'running' ? (
                        <button className="btn-small btn-warning" onClick={() => stopTunnel(tunnel.id)}>
                          停止
                        </button>
                      ) : (
                        <button className="btn-small btn-success" onClick={() => startTunnel(tunnel.id)}>
                          启动
                        </button>
                      )}
                      <button className="btn-small btn-danger" onClick={() => deleteTunnel(tunnel.id)}>
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* 节点列表 */}
      {activeTab === 'nodes' && (
        <div className="node-list">
          <div className="node-grid">
            {nodes.map(node => (
              <div key={node.id} className={`node-card ${node.status}`}>
                <div className="node-header">
                  <h3>{node.name}</h3>
                  <span className={`node-status ${node.status}`}>
                    {node.status === 'online' ? '在线' : node.status === 'offline' ? '离线' : '维护中'}
                  </span>
                </div>
                <div className="node-info">
                  <div className="node-region">📍 {node.region}</div>
                  <div className="node-host">{node.host}:{node.port}</div>
                  <div className="node-metrics">
                    <span>负载: {node.load}%</span>
                    <span>延迟: {node.latency}ms</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 客户端列表 */}
      {activeTab === 'clients' && (
        <div className="client-list">
          <table className="client-table">
            <thead>
              <tr>
                <th>客户端ID</th>
                <th>名称</th>
                <th>状态</th>
                <th>隧道数</th>
                <th>运行时间</th>
                <th>流量 ↑/↓</th>
              </tr>
            </thead>
            <tbody>
              {clients.map(client => (
                <tr key={client.id}>
                  <td>{client.id}</td>
                  <td>{client.name}</td>
                  <td>
                    <span className="status-dot" style={{ backgroundColor: statusColor(client.status) }} />
                    {client.status}
                  </td>
                  <td>{client.tunnels}</td>
                  <td>{Math.floor(client.uptime / 3600)}h</td>
                  <td>{formatBytes(client.bytes_sent)} / {formatBytes(client.bytes_recv)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* 创建隧道弹窗 */}
      {showCreateModal && (
        <CreateTunnelModal
          nodes={nodes}
          onClose={() => setShowCreateModal(false)}
          onCreated={(tunnel) => {
            setTunnels(prev => [...prev, tunnel]);
            setShowCreateModal(false);
          }}
        />
      )}
    </div>
  );
};

// 创建隧道弹窗组件
interface CreateTunnelModalProps {
  nodes: Node[];
  onClose: () => void;
  onCreated: (tunnel: Tunnel) => void;
}

const CreateTunnelModal: React.FC<CreateTunnelModalProps> = ({ nodes, onClose, onCreated }) => {
  const [form, setForm] = useState({
    name: '',
    type: 'tcp' as Tunnel['type'],
    local_ip: '127.0.0.1',
    local_port: '',
    remote_port: '',
    custom_domain: '',
    node_id: '',
  });
  const [submitting, setSubmitting] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await fetch('/api/v1/frp/tunnels', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      });
      const tunnel = await res.json();
      onCreated(tunnel);
    } catch (err) {
      console.error('Failed to create tunnel:', err);
      alert('创建隧道失败');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal-content" onClick={e => e.stopPropagation()}>
        <h2>创建隧道</h2>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>隧道名称</label>
            <input
              type="text"
              value={form.name}
              onChange={e => setForm({ ...form, name: e.target.value })}
              placeholder="例如: web-server"
              required
            />
          </div>

          <div className="form-group">
            <label>隧道类型</label>
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value as Tunnel['type'] })}>
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
              <option value="http">HTTP</option>
              <option value="https">HTTPS</option>
              <option value="stcp">STCP (密钥TCP)</option>
              <option value="xtcp">XTCP (P2P TCP)</option>
            </select>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label>本地IP</label>
              <input
                type="text"
                value={form.local_ip}
                onChange={e => setForm({ ...form, local_ip: e.target.value })}
              />
            </div>
            <div className="form-group">
              <label>本地端口</label>
              <input
                type="number"
                value={form.local_port}
                onChange={e => setForm({ ...form, local_port: e.target.value })}
                placeholder="8080"
                required
              />
            </div>
          </div>

          {(form.type === 'tcp' || form.type === 'udp') && (
            <div className="form-group">
              <label>远程端口</label>
              <input
                type="number"
                value={form.remote_port}
                onChange={e => setForm({ ...form, remote_port: e.target.value })}
                placeholder="留空自动分配"
              />
            </div>
          )}

          {(form.type === 'http' || form.type === 'https') && (
            <div className="form-group">
              <label>自定义域名</label>
              <input
                type="text"
                value={form.custom_domain}
                onChange={e => setForm({ ...form, custom_domain: e.target.value })}
                placeholder="your-domain.com"
              />
            </div>
          )}

          <div className="form-group">
            <label>节点选择</label>
            <select value={form.node_id} onChange={e => setForm({ ...form, node_id: e.target.value })}>
              <option value="">自动选择最佳节点</option>
              {nodes.filter(n => n.status === 'online').map(node => (
                <option key={node.id} value={node.id}>
                  {node.name} ({node.region}) - {node.latency}ms
                </option>
              ))}
            </select>
          </div>

          <div className="form-actions">
            <button type="button" className="btn-secondary" onClick={onClose}>
              取消
            </button>
            <button type="submit" className="btn-primary" disabled={submitting}>
              {submitting ? '创建中...' : '创建'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default FRPManager;