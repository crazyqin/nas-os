/**
 * 告警管理中心
 * 智能告警聚合、降噪、规则管理
 * 对标群晖Active Insight + TrueNAS Alert
 */
import React, { useState, useEffect, useCallback } from 'react';
import './AlertCenter.css';

interface Alert {
  id: string;
  type: 'critical' | 'warning' | 'info';
  category: 'storage' | 'network' | 'system' | 'security' | 'application';
  title: string;
  message: string;
  source: string;
  status: 'active' | 'acknowledged' | 'resolved';
  created_at: string;
  acknowledged_at?: string;
  resolved_at?: string;
  acknowledged_by?: string;
  count: number;
  first_seen: string;
  last_seen: string;
}

interface AlertRule {
  id: string;
  name: string;
  condition: string;
  severity: 'critical' | 'warning' | 'info';
  threshold: number;
  duration: number;
  actions: string[];
  enabled: boolean;
}

interface AlertGroup {
  id: string;
  name: string;
  alert_ids: string[];
  count: number;
  created_at: string;
}

const AlertCenter: React.FC = () => {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [groups, setGroups] = useState<AlertGroup[]>([]);
  const [activeTab, setActiveTab] = useState<'alerts' | 'rules' | 'groups'>('alerts');
  const [filterType, setFilterType] = useState<string>('all');
  const [filterStatus, setFilterStatus] = useState<string>('active');
  const [selectedAlert, setSelectedAlert] = useState<Alert | null>(null);
  const [loading, setLoading] = useState(true);
  const [wsConnected, setWsConnected] = useState(false);
  const [showCreateRule, setShowCreateRule] = useState(false);
  const [stats, setStats] = useState({
    total: 0,
    critical: 0,
    warning: 0,
    info: 0,
    acknowledged: 0,
    resolved: 0,
  });

  // WebSocket连接实时推送告警
  useEffect(() => {
    const ws = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/v1/alerts/ws`);
    
    ws.onopen = () => setWsConnected(true);
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      handleAlertEvent(data);
    };
    ws.onclose = () => setWsConnected(false);
    
    return () => ws.close();
  }, []);

  const handleAlertEvent = useCallback((event: { type: string; data: Alert }) => {
    switch (event.type) {
      case 'alert_new':
        setAlerts(prev => [event.data, ...prev]);
        updateStats([event.data, ...alerts]);
        break;
      case 'alert_acknowledged':
        setAlerts(prev => prev.map(a => 
          a.id === event.data.id ? { ...a, status: 'acknowledged', ...event.data } : a
        ));
        break;
      case 'alert_resolved':
        setAlerts(prev => prev.map(a => 
          a.id === event.data.id ? { ...a, status: 'resolved', ...event.data } : a
        ));
        break;
      case 'alert_count_update':
        setAlerts(prev => prev.map(a => 
          a.id === event.data.id ? { ...a, count: event.data.count } : a
        ));
        break;
    }
  }, [alerts]);

  // 加载初始数据
  useEffect(() => {
    fetchAlerts();
    fetchRules();
    fetchGroups();
  }, []);

  const fetchAlerts = async () => {
    try {
      const res = await fetch('/api/v1/alerts');
      const data = await res.json();
      setAlerts(data.alerts || []);
      updateStats(data.alerts || []);
    } catch (err) {
      console.error('Failed to fetch alerts:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchRules = async () => {
    try {
      const res = await fetch('/api/v1/alerts/rules');
      const data = await res.json();
      setRules(data.rules || []);
    } catch (err) {
      console.error('Failed to fetch rules:', err);
    }
  };

  const fetchGroups = async () => {
    try {
      const res = await fetch('/api/v1/alerts/groups');
      const data = await res.json();
      setGroups(data.groups || []);
    } catch (err) {
      console.error('Failed to fetch groups:', err);
    }
  };

  const updateStats = (alertsList: Alert[]) => {
    setStats({
      total: alertsList.length,
      critical: alertsList.filter(a => a.type === 'critical' && a.status === 'active').length,
      warning: alertsList.filter(a => a.type === 'warning' && a.status === 'active').length,
      info: alertsList.filter(a => a.type === 'info' && a.status === 'active').length,
      acknowledged: alertsList.filter(a => a.status === 'acknowledged').length,
      resolved: alertsList.filter(a => a.status === 'resolved').length,
    });
  };

  // 告警操作
  const acknowledgeAlert = async (id: string) => {
    try {
      await fetch(`/api/v1/alerts/${id}/acknowledge`, { method: 'POST' });
      setAlerts(prev => prev.map(a => 
        a.id === id ? { ...a, status: 'acknowledged', acknowledged_at: new Date().toISOString() } : a
      ));
    } catch (err) {
      console.error('Failed to acknowledge:', err);
    }
  };

  const resolveAlert = async (id: string) => {
    try {
      await fetch(`/api/v1/alerts/${id}/resolve`, { method: 'POST' });
      setAlerts(prev => prev.map(a => 
        a.id === id ? { ...a, status: 'resolved', resolved_at: new Date().toISOString() } : a
      ));
    } catch (err) {
      console.error('Failed to resolve:', err);
    }
  };

  const bulkAcknowledge = async (ids: string[]) => {
    try {
      await fetch('/api/v1/alerts/bulk-acknowledge', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      });
      setAlerts(prev => prev.map(a => 
        ids.includes(a.id) ? { ...a, status: 'acknowledged' } : a
      ));
    } catch (err) {
      console.error('Bulk acknowledge failed:', err);
    }
  };

  const bulkResolve = async (ids: string[]) => {
    try {
      await fetch('/api/v1/alerts/bulk-resolve', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ids }),
      });
      setAlerts(prev => prev.map(a => 
        ids.includes(a.id) ? { ...a, status: 'resolved' } : a
      ));
    } catch (err) {
      console.error('Bulk resolve failed:', err);
    }
  };

  // 过滤告警
  const filteredAlerts = alerts.filter(a => {
    if (filterType !== 'all' && a.type !== filterType) return false;
    if (filterStatus !== 'all' && a.status !== filterStatus) return false;
    return true;
  });

  // 类型图标
  const typeIcon = (type: string): string => {
    switch (type) {
      case 'critical': return '🔴';
      case 'warning': return '🟡';
      case 'info': return '🔵';
      default: return '⚪';
    }
  };

  // 分类图标
  const categoryIcon = (category: string): string => {
    switch (category) {
      case 'storage': return '💾';
      case 'network': return '🌐';
      case 'system': return '⚙️';
      case 'security': return '🔒';
      case 'application': return '📦';
      default: return '📋';
    }
  };

  // 时间格式化
  const formatTime = (time: string): string => {
    const date = new Date(time);
    const now = new Date();
    const diff = now.getTime() - date.getTime();
    
    if (diff < 60000) return '刚刚';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}小时前`;
    if (diff < 604800000) return `${Math.floor(diff / 86400000)}天前`;
    return date.toLocaleDateString();
  };

  // 告警统计条
  const StatBar = () => (
    <div className="alert-stats">
      <div className="stat-item critical">
        <span className="stat-icon">🔴</span>
        <span className="stat-value">{stats.critical}</span>
        <span className="stat-label">严重</span>
      </div>
      <div className="stat-item warning">
        <span className="stat-icon">🟡</span>
        <span className="stat-value">{stats.warning}</span>
        <span className="stat-label">警告</span>
      </div>
      <div className="stat-item info">
        <span className="stat-icon">🔵</span>
        <span className="stat-value">{stats.info}</span>
        <span className="stat-label">信息</span>
      </div>
      <div className="stat-item acknowledged">
        <span className="stat-icon">✅</span>
        <span className="stat-value">{stats.acknowledged}</span>
        <span className="stat-label">已确认</span>
      </div>
      <div className="stat-item resolved">
        <span className="stat-icon">✔️</span>
        <span className="stat-value">{stats.resolved}</span>
        <span className="stat-label">已解决</span>
      </div>
    </div>
  );

  // 告警列表项
  const AlertItem = ({ alert }: { alert: Alert }) => (
    <div 
      className={`alert-item ${alert.type} ${alert.status} ${selectedAlert?.id === alert.id ? 'selected' : ''}`}
      onClick={() => setSelectedAlert(alert)}
    >
      <div className="alert-header">
        <span className="alert-icon">{typeIcon(alert.type)}</span>
        <span className="alert-category">{categoryIcon(alert.category)}</span>
        <span className="alert-title">{alert.title}</span>
        {alert.count > 1 && (
          <span className="alert-count">×{alert.count}</span>
        )}
        <span className={`alert-status-badge ${alert.status}`}>
          {alert.status === 'active' ? '活跃' : 
           alert.status === 'acknowledged' ? '已确认' : '已解决'}
        </span>
      </div>
      <div className="alert-meta">
        <span className="alert-source">{alert.source}</span>
        <span className="alert-time">{formatTime(alert.last_seen)}</span>
      </div>
      {alert.status === 'active' && (
        <div className="alert-actions">
          <button className="btn-ack" onClick={(e) => { e.stopPropagation(); acknowledgeAlert(alert.id); }}>
            确认
          </button>
          <button className="btn-resolve" onClick={(e) => { e.stopPropagation(); resolveAlert(alert.id); }}>
            解决
          </button>
        </div>
      )}
    </div>
  );

  // 告警详情面板
  const AlertDetail = ({ alert }: { alert: Alert }) => (
    <div className="alert-detail-panel">
      <div className="detail-header">
        <span className="detail-icon">{typeIcon(alert.type)}</span>
        <h3>{alert.title}</h3>
        <span className={`detail-status ${alert.status}`}>
          {alert.status === 'active' ? '活跃' : 
           alert.status === 'acknowledged' ? '已确认' : '已解决'}
        </span>
      </div>
      
      <div className="detail-section">
        <h4>详细信息</h4>
        <p className="detail-message">{alert.message}</p>
        <div className="detail-info">
          <div className="info-row">
            <label>来源:</label>
            <span>{alert.source}</span>
          </div>
          <div className="info-row">
            <label>分类:</label>
            <span>{alert.category}</span>
          </div>
          <div className="info-row">
            <label>首次出现:</label>
            <span>{new Date(alert.first_seen).toLocaleString()}</span>
          </div>
          <div className="info-row">
            <label>最后出现:</label>
            <span>{new Date(alert.last_seen).toLocaleString()}</span>
          </div>
          <div className="info-row">
            <label>重复次数:</label>
            <span>{alert.count}</span>
          </div>
          {alert.acknowledged_by && (
            <div className="info-row">
              <label>确认人:</label>
              <span>{alert.acknowledged_by}</span>
            </div>
          )}
          {alert.acknowledged_at && (
            <div className="info-row">
              <label>确认时间:</label>
              <span>{new Date(alert.acknowledged_at).toLocaleString()}</span>
            </div>
          )}
          {alert.resolved_at && (
            <div className="info-row">
              <label>解决时间:</label>
              <span>{new Date(alert.resolved_at).toLocaleString()}</span>
            </div>
          )}
        </div>
      </div>

      <div className="detail-section">
        <h4>时间线</h4>
        <div className="timeline">
          <div className="timeline-item">
            <div className="timeline-dot" style={{ background: '#ff4d4f' }} />
            <div className="timeline-content">
              <span className="timeline-label">创建</span>
              <span className="timeline-time">{new Date(alert.created_at).toLocaleString()}</span>
            </div>
          </div>
          {alert.acknowledged_at && (
            <div className="timeline-item">
              <div className="timeline-dot" style={{ background: '#faad14' }} />
              <div className="timeline-content">
                <span className="timeline-label">确认</span>
                <span className="timeline-time">{new Date(alert.acknowledged_at).toLocaleString()}</span>
              </div>
            </div>
          )}
          {alert.resolved_at && (
            <div className="timeline-item">
              <div className="timeline-dot" style={{ background: '#52c41a' }} />
              <div className="timeline-content">
                <span className="timeline-label">解决</span>
                <span className="timeline-time">{new Date(alert.resolved_at).toLocaleString()}</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {alert.status === 'active' && (
        <div className="detail-actions">
          <button className="btn-primary" onClick={() => acknowledgeAlert(alert.id)}>
            确认告警
          </button>
          <button className="btn-success" onClick={() => resolveAlert(alert.id)}>
            标记解决
          </button>
        </div>
      )}
    </div>
  );

  // 规则列表
  const RuleCard = ({ rule }: { rule: AlertRule }) => (
    <div className={`rule-card ${rule.enabled ? 'enabled' : 'disabled'}`}>
      <div className="rule-header">
        <h4>{rule.name}</h4>
        <span className={`rule-severity ${rule.severity}`}>
          {rule.severity === 'critical' ? '严重' : 
           rule.severity === 'warning' ? '警告' : '信息'}
        </span>
      </div>
      <div className="rule-condition">
        <span className="condition-text">{rule.condition}</span>
        <span className="threshold">阈值: {rule.threshold}</span>
        <span className="duration">持续时间: {rule.duration}s</span>
      </div>
      <div className="rule-actions-list">
        {rule.actions.map((action, i) => (
          <span key={i} className="action-tag">{action}</span>
        ))}
      </div>
      <div className="rule-controls">
        <button className="btn-toggle" onClick={() => toggleRule(rule.id, !rule.enabled)}>
          {rule.enabled ? '禁用' : '启用'}
        </button>
        <button className="btn-edit" onClick={() => editRule(rule.id)}>
          编辑
        </button>
      </div>
    </div>
  );

  const toggleRule = async (id: string, enabled: boolean) => {
    try {
      await fetch(`/api/v1/alerts/rules/${id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled }),
      });
      setRules(prev => prev.map(r => r.id === id ? { ...r, enabled } : r));
    } catch (err) {
      console.error('Toggle rule failed:', err);
    }
  };

  const editRule = (id: string) => {
    // 打开编辑弹窗
    console.log('Edit rule:', id);
  };

  if (loading) {
    return <div className="alert-loading">加载告警数据...</div>;
  }

  return (
    <div className="alert-center">
      <header className="alert-header-bar">
        <div className="header-info">
          <h1>
            <span className="header-icon">🔔</span>
            告警管理中心
            <span className={`ws-indicator ${wsConnected ? 'connected' : 'disconnected'}`}>
              {wsConnected ? '实时' : '离线'}
            </span>
          </h1>
          <span className="total-count">共 {stats.total} 条告警</span>
        </div>
        <div className="header-actions">
          <button className="btn-primary" onClick={() => setShowCreateRule(true)}>
            + 创建规则
          </button>
          {stats.critical + stats.warning > 0 && (
            <button className="btn-danger" onClick={() => bulkAcknowledge(
              alerts.filter(a => a.status === 'active').map(a => a.id)
            )}>
              批量确认
            </button>
          )}
        </div>
      </header>

      {/* 统计条 */}
      <StatBar />

      {/* Tab导航 */}
      <nav className="alert-tabs">
        <button className={activeTab === 'alerts' ? 'active' : ''} onClick={() => setActiveTab('alerts')}>
          告警 ({alerts.filter(a => a.status === 'active').length})
        </button>
        <button className={activeTab === 'rules' ? 'active' : ''} onClick={() => setActiveTab('rules')}>
          规则 ({rules.length})
        </button>
        <button className={activeTab === 'groups' ? 'active' : ''} onClick={() => setActiveTab('groups')}>
          告警组 ({groups.length})
        </button>
      </nav>

      {/* 告警列表 */}
      {activeTab === 'alerts' && (
        <div className="alerts-container">
          <div className="alerts-filter">
            <select value={filterType} onChange={e => setFilterType(e.target.value)}>
              <option value="all">全部类型</option>
              <option value="critical">严重</option>
              <option value="warning">警告</option>
              <option value="info">信息</option>
            </select>
            <select value={filterStatus} onChange={e => setFilterStatus(e.target.value)}>
              <option value="active">活跃</option>
              <option value="acknowledged">已确认</option>
              <option value="resolved">已解决</option>
              <option value="all">全部状态</option>
            </select>
          </div>
          
          <div className="alerts-list-panel">
            <div className="alerts-list">
              {filteredAlerts.length === 0 ? (
                <div className="empty-alerts">
                  <span className="empty-icon">✨</span>
                  <p>暂无告警</p>
                </div>
              ) : (
                filteredAlerts.map(alert => <AlertItem key={alert.id} alert={alert} />)
              )}
            </div>
            
            {selectedAlert && (
              <AlertDetail alert={selectedAlert} />
            )}
          </div>
        </div>
      )}

      {/* 规则列表 */}
      {activeTab === 'rules' && (
        <div className="rules-grid">
          {rules.length === 0 ? (
            <div className="empty-rules">
              <p>暂无告警规则</p>
              <button className="btn-primary" onClick={() => setShowCreateRule(true)}>
                创建第一条规则
              </button>
            </div>
          ) : (
            rules.map(rule => <RuleCard key={rule.id} rule={rule} />)
          )}
        </div>
      )}

      {/* 告警组列表 */}
      {activeTab === 'groups' && (
        <div className="groups-list">
          {groups.length === 0 ? (
            <div className="empty-groups">暂无告警组</div>
          ) : (
            <table className="groups-table">
              <thead>
                <tr>
                  <th>组名</th>
                  <th>告警数</th>
                  <th>创建时间</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {groups.map(group => (
                  <tr key={group.id}>
                    <td>{group.name}</td>
                    <td>{group.count}</td>
                    <td>{new Date(group.created_at).toLocaleString()}</td>
                    <td>
                      <button className="btn-small">查看</button>
                      <button className="btn-small btn-danger">解散</button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  );
};

export default AlertCenter;