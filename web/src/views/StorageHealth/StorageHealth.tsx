/**
 * 存储健康仪表盘
 * 整合磁盘、SSD、NVMe的智能健康监控
 * 对标群晖Storage Analyzer + TrueNAS reporting
 */
import React, { useState, useEffect } from 'react';
import './StorageHealth.css';

interface DiskHealth {
  device: string;
  model: string;
  serial: string;
  health: 'healthy' | 'warning' | 'critical' | 'unknown';
  temperature: number;
  power_on_hours: number;
  reallocated_sectors: number;
  temperature_score: number;
  health_score: number;
  capacity: string;
  used: string;
  usage_percent: number;
  smart_status: string;
}

interface SSDHealth {
  device: string;
  health_percent: number;
  total_bytes_written: number;
  total_bytes_read: number;
  power_on_hours: number;
  power_cycle_count: number;
  temperature: number;
  wear_leveling_count: number;
  total_lba_written: number;
  total_lba_read: number;
  temperature_score: number;
  endurance_score: number;
}

interface PoolHealth {
  name: string;
  status: 'online' | 'degraded' | 'offline';
  capacity: string;
  used: string;
  usage_percent: number;
  disks: string[];
  health_score: number;
}

const StorageHealth: React.FC = () => {
  const [disks, setDisks] = useState<DiskHealth[]>([]);
  const [ssds, setSSDs] = useState<SSDHealth[]>([]);
  const [pools, setPools] = useState<PoolHealth[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedDisk, setSelectedDisk] = useState<DiskHealth | null>(null);
  const [refreshInterval, setRefreshInterval] = useState(30);
  const [lastUpdate, setLastUpdate] = useState<Date>(new Date());

  // 自动刷新
  useEffect(() => {
    fetchAllData();
    const interval = setInterval(fetchAllData, refreshInterval * 1000);
    return () => clearInterval(interval);
  }, [refreshInterval]);

  const fetchAllData = async () => {
    setLoading(true);
    try {
      const [diskRes, ssdRes, poolRes] = await Promise.all([
        fetch('/api/v1/storage/disks'),
        fetch('/api/v1/ssd'),
        fetch('/api/v1/storage/pools'),
      ]);
      
      const diskData = await diskRes.json();
      const ssdData = await ssdRes.json();
      const poolData = await poolRes.json();
      
      setDisks(diskData.disks || []);
      setSSDs(ssdData.ssds || []);
      setPools(poolData.pools || []);
      setLastUpdate(new Date());
    } catch (err) {
      console.error('Failed to fetch storage data:', err);
    } finally {
      setLoading(false);
    }
  };

  // 计算总体健康分数
  const calculateOverallHealth = (): number => {
    if (disks.length === 0) return 100;
    const avgDiskHealth = disks.reduce((sum, d) => sum + d.health_score, 0) / disks.length;
    const avgSSDHealth = ssds.length > 0 
      ? ssds.reduce((sum, s) => sum + s.health_percent, 0) / ssds.length 
      : 100;
    return Math.round((avgDiskHealth + avgSSDHealth) / 2);
  };

  // 健康分数颜色
  const healthColor = (score: number): string => {
    if (score >= 80) return '#52c41a';
    if (score >= 60) return '#faad14';
    if (score >= 40) return '#fa8c16';
    return '#ff4d4f';
  };

  // 格式化时间
  const formatHours = (hours: number): string => {
    const days = Math.floor(hours / 24);
    const remainingHours = hours % 24;
    if (days > 365) {
      const years = Math.floor(days / 365);
      const remainingDays = days % 365;
      return `${years}年${remainingDays}天`;
    }
    if (days > 30) {
      const months = Math.floor(days / 30);
      const remainingDays = days % 30;
      return `${months}月${remainingDays}天`;
    }
    return `${days}天${remainingHours}小时`;
  };

  // 格式化字节
  const formatBytes = (bytes: number): string => {
    if (bytes < 1024 * 1024 * 1024) {
      return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
    }
    if (bytes < 1024 * 1024 * 1024 * 1024) {
      return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`;
    }
    return `${(bytes / 1024 / 1024 / 1024 / 1024).toFixed(2)} TB`;
  };

  // 温度指示器
  const TemperatureGauge = ({ temp, max = 70 }: { temp: number; max?: number }) => {
    const percent = Math.min((temp / max) * 100, 100);
    const color = temp < 40 ? '#52c41a' : temp < 50 ? '#faad14' : temp < 60 ? '#fa8c16' : '#ff4d4f';
    
    return (
      <div className="temp-gauge">
        <div className="temp-bar">
          <div className="temp-fill" style={{ width: `${percent}%`, background: color }} />
        </div>
        <span className="temp-value">{temp}°C</span>
      </div>
    );
  };

  // 健康圆环
  const HealthRing = ({ score, size = 120 }: { score: number; size?: number }) => {
    const radius = (size - 12) / 2;
    const circumference = 2 * Math.PI * radius;
    const strokeDashoffset = circumference - (score / 100) * circumference;
    
    return (
      <div className="health-ring" style={{ width: size, height: size }}>
        <svg width={size} height={size}>
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke="#f0f0f0"
            strokeWidth="8"
          />
          <circle
            cx={size / 2}
            cy={size / 2}
            r={radius}
            fill="none"
            stroke={healthColor(score)}
            strokeWidth="8"
            strokeDasharray={circumference}
            strokeDashoffset={strokeDashoffset}
            strokeLinecap="round"
            transform={`rotate(-90 ${size / 2} ${size / 2})`}
          />
        </svg>
        <div className="health-ring-content">
          <span className="health-value">{score}</span>
          <span className="health-label">健康分</span>
        </div>
      </div>
    );
  };

  // 存储池卡片
  const PoolCard = ({ pool }: { pool: PoolHealth }) => (
    <div className={`pool-card ${pool.status}`}>
      <div className="pool-header">
        <h3>{pool.name}</h3>
        <span className={`pool-status ${pool.status}`}>
          {pool.status === 'online' ? '正常' : pool.status === 'degraded' ? '降级' : '离线'}
        </span>
      </div>
      <div className="pool-usage">
        <div className="usage-bar">
          <div 
            className="usage-fill" 
            style={{ 
              width: `${pool.usage_percent}%`,
              background: pool.usage_percent > 90 ? '#ff4d4f' : 
                          pool.usage_percent > 70 ? '#faad14' : '#52c41a'
            }} 
          />
        </div>
        <div className="usage-text">
          {pool.used} / {pool.capacity} ({pool.usage_percent}%)
        </div>
      </div>
      <div className="pool-disks">
        {pool.disks.map((disk, idx) => (
          <span key={idx} className="disk-tag">{disk}</span>
        ))}
      </div>
    </div>
  );

  // 磁盘详情面板
  const DiskDetail = ({ disk }: { disk: DiskHealth }) => (
    <div className="disk-detail">
      <h3>{disk.device}</h3>
      <div className="detail-grid">
        <div className="detail-item">
          <label>型号</label>
          <span>{disk.model}</span>
        </div>
        <div className="detail-item">
          <label>序列号</label>
          <span>{disk.serial}</span>
        </div>
        <div className="detail-item">
          <label>容量</label>
          <span>{disk.capacity}</span>
        </div>
        <div className="detail-item">
          <label>已用</label>
          <span>{disk.used} ({disk.usage_percent}%)</span>
        </div>
        <div className="detail-item">
          <label>运行时间</label>
          <span>{formatHours(disk.power_on_hours)}</span>
        </div>
        <div className="detail-item">
          <label>重分配扇区</label>
          <span className={disk.reallocated_sectors > 100 ? 'warning' : ''}>
            {disk.reallocated_sectors}
          </span>
        </div>
        <div className="detail-item">
          <label>SMART状态</label>
          <span className={`smart-status ${disk.smart_status}`}>{disk.smart_status}</span>
        </div>
      </div>
      <div className="detail-chart">
        <h4>温度趋势</h4>
        <div className="mini-chart">
          {/* 温度趋势图表（简化版） */}
          {[45, 48, 46, 47, 50, 49, 48, 47, 46, 45, 44, 46].map((t, i) => (
            <div 
              key={i} 
              className="chart-bar" 
              style={{ height: `${(t / 70) * 100}%` }}
              title={`${t}°C`}
            />
          ))}
        </div>
      </div>
    </div>
  );

  if (loading && disks.length === 0) {
    return <div className="storage-loading">加载存储数据...</div>;
  }

  return (
    <div className="storage-health">
      <header className="storage-header">
        <div className="header-left">
          <h1>
            <span className="icon">💾</span>
            存储健康中心
          </h1>
          <span className="last-update">
            最后更新: {lastUpdate.toLocaleTimeString()}
          </span>
        </div>
        <div className="header-right">
          <label>刷新间隔:</label>
          <select value={refreshInterval} onChange={e => setRefreshInterval(Number(e.target.value))}>
            <option value={10}>10秒</option>
            <option value={30}>30秒</option>
            <option value={60}>1分钟</option>
            <option value={300}>5分钟</option>
          </select>
          <button className="btn-refresh" onClick={fetchAllData}>
            🔄 刷新
          </button>
        </div>
      </header>

      {/* 总体健康状态 */}
      <div className="overall-status">
        <HealthRing score={calculateOverallHealth()} size={160} />
        <div className="status-summary">
          <div className="summary-item">
            <span className="count">{disks.length}</span>
            <span className="label">物理磁盘</span>
          </div>
          <div className="summary-item">
            <span className="count">{ssds.length}</span>
            <span className="label">SSD</span>
          </div>
          <div className="summary-item">
            <span className="count">{pools.length}</span>
            <span className="label">存储池</span>
          </div>
          <div className="summary-item">
            <span className={`count ${disks.filter(d => d.health !== 'healthy').length > 0 ? 'warning' : ''}`}>
              {disks.filter(d => d.health !== 'healthy').length}
            </span>
            <span className="label">异常磁盘</span>
          </div>
        </div>
      </div>

      {/* 存储池概览 */}
      <section className="pools-section">
        <h2>存储池</h2>
        <div className="pools-grid">
          {pools.length === 0 ? (
            <div className="empty-pools">暂无存储池配置</div>
          ) : (
            pools.map(pool => <PoolCard key={pool.name} pool={pool} />)
          )}
        </div>
      </section>

      {/* 磁盘列表 */}
      <section className="disks-section">
        <h2>物理磁盘</h2>
        <div className="disks-container">
          <div className="disks-list">
            {disks.length === 0 ? (
              <div className="empty-disks">未检测到磁盘</div>
            ) : (
              disks.map(disk => (
                <div 
                  key={disk.device}
                  className={`disk-card ${disk.health} ${selectedDisk?.device === disk.device ? 'selected' : ''}`}
                  onClick={() => setSelectedDisk(disk)}
                >
                  <div className="disk-header">
                    <span className="disk-icon">🔧</span>
                    <span className="disk-name">{disk.device}</span>
                    <span className={`health-badge ${disk.health}`}>
                      {disk.health === 'healthy' ? '正常' : 
                       disk.health === 'warning' ? '警告' : 
                       disk.health === 'critical' ? '危险' : '未知'}
                    </span>
                  </div>
                  <div className="disk-model">{disk.model}</div>
                  <div className="disk-metrics">
                    <div className="metric">
                      <span className="metric-label">温度</span>
                      <TemperatureGauge temp={disk.temperature} />
                    </div>
                    <div className="metric">
                      <span className="metric-label">使用率</span>
                      <div className="usage-mini">
                        <div className="usage-bar-mini">
                          <div 
                            className="usage-fill" 
                            style={{ 
                              width: `${disk.usage_percent}%`,
                              background: healthColor(disk.health_score)
                            }} 
                          />
                        </div>
                        <span>{disk.usage_percent}%</span>
                      </div>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
          
          {/* 磁盘详情 */}
          {selectedDisk && (
            <div className="disk-detail-panel">
              <DiskDetail disk={selectedDisk} />
            </div>
          )}
        </div>
      </section>

      {/* SSD详细信息 */}
      {ssds.length > 0 && (
        <section className="ssds-section">
          <h2>SSD详情</h2>
          <div className="ssds-table">
            <table>
              <thead>
                <tr>
                  <th>设备</th>
                  <th>健康度</th>
                  <th>写入量</th>
                  <th>读取量</th>
                  <th>温度</th>
                  <th>磨损</th>
                  <th>运行时间</th>
                </tr>
              </thead>
              <tbody>
                {ssds.map(ssd => (
                  <tr key={ssd.device}>
                    <td>{ssd.device}</td>
                    <td>
                      <span className="health-bar-container">
                        <div 
                          className="health-bar-fill" 
                          style={{ 
                            width: `${ssd.health_percent}%`,
                            background: healthColor(ssd.health_percent)
                          }}
                        />
                      </span>
                      <span>{ssd.health_percent}%</span>
                    </td>
                    <td>{formatBytes(ssd.total_bytes_written)}</td>
                    <td>{formatBytes(ssd.total_bytes_read)}</td>
                    <td>
                      <TemperatureGauge temp={ssd.temperature} />
                    </td>
                    <td>
                      <span className="wear-level">
                        {100 - ssd.wear_leveling_count}%
                      </span>
                    </td>
                    <td>{formatHours(ssd.power_on_hours)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}
    </div>
  );
};

export default StorageHealth;