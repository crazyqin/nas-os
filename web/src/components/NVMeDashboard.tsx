import React from 'react';
import { useNVMe } from '../hooks/useNVMe';

interface NVMeDevice {
  device: string;
  model: string;
  serial: string;
  temperature: number;
  percentUsed: number;
  availableSpare: number;
  criticalWarning: number;
  powerCycles: number;
  powerOnHours: number;
  dataUnitsRead: number;
  dataUnitsWrite: number;
  mediaErrors: number;
  numErrors: number;
  smartStatus: 'healthy' | 'warning' | 'critical' | 'unknown';
  lastChecked: string;
}

// Status badge component
const StatusBadge: React.FC<{ status: string }> = ({ status }) => {
  const colors = {
    healthy: 'bg-green-500/20 text-green-400 border-green-500/30',
    warning: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
    critical: 'bg-red-500/20 text-red-400 border-red-500/30',
    unknown: 'bg-gray-500/20 text-gray-400 border-gray-500/30'
  };

  const labels = {
    healthy: '健康',
    warning: '警告',
    critical: '严重',
    unknown: '未知'
  };

  return (
    <span className={`px-2 py-1 rounded-full text-xs font-medium border ${colors[status as keyof typeof colors] || colors.unknown}`}>
      {labels[status as keyof typeof labels] || status}
    </span>
  );
};

// Temperature gauge component
const TemperatureGauge: React.FC<{ temp: number; threshold: number }> = ({ temp, threshold = 70 }) => {
  const percentage = Math.min((temp / 100) * 100, 100);
  const isWarning = temp >= threshold;
  const isCritical = temp >= 80;

  const barColor = isCritical ? 'bg-red-500' : isWarning ? 'bg-yellow-500' : 'bg-green-500';

  return (
    <div className="flex items-center gap-2">
      <div className="w-24 h-3 bg-gray-700 rounded-full overflow-hidden">
        <div 
          className={`h-full ${barColor} transition-all duration-300`}
          style={{ width: `${percentage}%` }}
        />
      </div>
      <span className={`text-sm font-medium ${isCritical ? 'text-red-400' : isWarning ? 'text-yellow-400' : 'text-gray-300'}`}>
        {temp}°C
      </span>
    </div>
  );
};

// Health progress component
const HealthProgress: React.FC<{ percentUsed: number }> = ({ percentUsed }) => {
  const healthPercent = 100 - percentUsed;
  const isWarning = percentUsed >= 80;
  const isCritical = percentUsed >= 90;

  const barColor = isCritical ? 'bg-red-500' : isWarning ? 'bg-yellow-500' : 'bg-green-500';

  return (
    <div className="flex items-center gap-3">
      <div className="w-32 h-4 bg-gray-700 rounded-full overflow-hidden">
        <div 
          className={`h-full ${barColor} transition-all duration-500`}
          style={{ width: `${healthPercent}%` }}
        />
      </div>
      <span className="text-sm text-gray-300">
        {healthPercent.toFixed(1)}% 剩余
      </span>
    </div>
  );
};

// Format bytes to human readable
const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Format hours to days
const formatHours = (hours: number): string => {
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  if (days > 365) {
    const years = Math.floor(days / 365);
    return `${years}年 ${days % 365}天`;
  }
  return `${days}天 ${remainingHours}小时`;
};

// Single device card
const DeviceCard: React.FC<{ device: NVMeDevice }> = ({ device }) => {
  const [expanded, setExpanded] = React.useState(false);

  return (
    <div className="bg-gray-800/50 rounded-lg border border-gray-700 p-4 hover:border-gray-600 transition-colors">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-blue-500/20 flex items-center justify-center">
            <svg className="w-6 h-6 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
            </svg>
          </div>
          <div>
            <h3 className="text-lg font-semibold text-white">{device.model || device.device}</h3>
            <p className="text-xs text-gray-500">{device.device}</p>
          </div>
        </div>
        <StatusBadge status={device.smartStatus} />
      </div>

      {/* Key metrics */}
      <div className="grid grid-cols-2 gap-4 mb-4">
        <div>
          <label className="text-xs text-gray-500 mb-1 block">温度</label>
          <TemperatureGauge temp={device.temperature} threshold={70} />
        </div>
        <div>
          <label className="text-xs text-gray-500 mb-1 block">健康度</label>
          <HealthProgress percentUsed={device.percentUsed} />
        </div>
      </div>

      {/* Quick stats */}
      <div className="grid grid-cols-3 gap-3 text-sm">
        <div className="bg-gray-900/50 rounded p-2">
          <span className="text-gray-500 text-xs">通电时间</span>
          <p className="text-gray-300">{formatHours(device.powerOnHours)}</p>
        </div>
        <div className="bg-gray-900/50 rounded p-2">
          <span className="text-gray-500 text-xs">写入量</span>
          <p className="text-gray-300">{formatBytes(device.dataUnitsWrite * 512 * 1000)}</p>
        </div>
        <div className="bg-gray-900/50 rounded p-2">
          <span className="text-gray-500 text-xs">备用空间</span>
          <p className="text-gray-300">{device.availableSpare.toFixed(1)}%</p>
        </div>
      </div>

      {/* Expandable SMART details */}
      <button 
        onClick={() => setExpanded(!expanded)}
        className="mt-3 text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1"
      >
        <svg className={`w-4 h-4 transition-transform ${expanded ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
        </svg>
        {expanded ? '收起详情' : '查看SMART详情'}
      </button>

      {expanded && (
        <div className="mt-3 pt-3 border-t border-gray-700">
          <div className="grid grid-cols-2 gap-2 text-sm">
            <div className="flex justify-between">
              <span className="text-gray-500">序列号</span>
              <span className="text-gray-300">{device.serial || 'N/A'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">电源循环</span>
              <span className="text-gray-300">{device.powerCycles} 次</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">读取量</span>
              <span className="text-gray-300">{formatBytes(device.dataUnitsRead * 512 * 1000)}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">媒体错误</span>
              <span className={`text-gray-300 ${device.mediaErrors > 0 ? 'text-red-400' : ''}`}>
                {device.mediaErrors}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">错误计数</span>
              <span className={`text-gray-300 ${device.numErrors > 0 ? 'text-red-400' : ''}`}>
                {device.numErrors}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-500">严重警告</span>
              <span className={`text-gray-300 ${device.criticalWarning > 0 ? 'text-red-400' : ''}`}>
                {device.criticalWarning}
              </span>
            </div>
          </div>
          <p className="mt-2 text-xs text-gray-500">
            最后检查: {new Date(device.lastChecked).toLocaleString('zh-CN')}
          </p>
        </div>
      )}
    </div>
  );
};

// Main dashboard component
export const NVMeDashboard: React.FC = () => {
  const { data, alerts, loading, error, refetch } = useNVMe();

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
        <span className="ml-3 text-gray-400">加载NVMe数据...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bg-red-500/20 border border-red-500/30 rounded-lg p-4">
        <div className="flex items-center gap-2 text-red-400">
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
          </svg>
          <span>加载失败: {error.message}</span>
        </div>
        <button 
          onClick={refetch}
          className="mt-2 px-3 py-1 bg-red-500/30 hover:bg-red-500/40 rounded text-red-300 text-sm transition-colors"
        >
          重试
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Summary cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <div className="bg-gray-800 rounded-lg p-4 border border-gray-700">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-lg bg-blue-500/20 flex items-center justify-center">
              <svg className="w-7 h-7 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
              </svg>
            </div>
            <div>
              <p className="text-2xl font-bold text-white">{data?.devices.length || 0}</p>
              <p className="text-sm text-gray-500">NVMe设备</p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-4 border border-green-500/30">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-lg bg-green-500/20 flex items-center justify-center">
              <svg className="w-7 h-7 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div>
              <p className="text-2xl font-bold text-green-400">{data?.healthyCount || 0}</p>
              <p className="text-sm text-gray-500">健康</p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-4 border border-yellow-500/30">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-lg bg-yellow-500/20 flex items-center justify-center">
              <svg className="w-7 h-7 text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
            </div>
            <div>
              <p className="text-2xl font-bold text-yellow-400">{data?.warningCount || 0}</p>
              <p className="text-sm text-gray-500">警告</p>
            </div>
          </div>
        </div>

        <div className="bg-gray-800 rounded-lg p-4 border border-red-500/30">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-lg bg-red-500/20 flex items-center justify-center">
              <svg className="w-7 h-7 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
            </div>
            <div>
              <p className="text-2xl font-bold text-red-400">{data?.criticalCount || 0}</p>
              <p className="text-sm text-gray-500">严重</p>
            </div>
          </div>
        </div>
      </div>

      {/* Alerts panel */}
      {alerts.length > 0 && (
        <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
          <h3 className="text-lg font-semibold text-white mb-3 flex items-center gap-2">
            <svg className="w-5 h-5 text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
            </svg>
            告警 ({alerts.length})
          </h3>
          <div className="space-y-2">
            {alerts.map((alert, index) => (
              <div 
                key={index}
                className={`p-3 rounded border ${
                  alert.severity === 'critical' 
                    ? 'bg-red-500/10 border-red-500/30' 
                    : 'bg-yellow-500/10 border-yellow-500/30'
                }`}
              >
                <div className="flex items-center justify-between">
                  <span className={`font-medium ${
                    alert.severity === 'critical' ? 'text-red-400' : 'text-yellow-400'
                  }`}>
                    {alert.device}
                  </span>
                  <span className="text-xs text-gray-500">
                    {new Date(alert.timestamp).toLocaleString('zh-CN')}
                  </span>
                </div>
                <p className="text-sm text-gray-300 mt-1">{alert.message}</p>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Devices list */}
      <div className="bg-gray-800 rounded-lg border border-gray-700 p-4">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-semibold text-white">NVMe 设备列表</h2>
          <button 
            onClick={refetch}
            disabled={loading}
            className="px-3 py-1.5 bg-blue-500/20 hover:bg-blue-500/30 disabled:opacity-50 rounded text-blue-400 text-sm transition-colors flex items-center gap-2"
          >
            <svg className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            刷新
          </button>
        </div>

        {data?.devices.length === 0 ? (
          <div className="text-center py-8 text-gray-500">
            <svg className="w-12 h-12 mx-auto mb-3 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
            </svg>
            <p>未检测到NVMe设备</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {data?.devices.map((device) => (
              <DeviceCard key={device.device} device={device} />
            ))}
          </div>
        )}
      </div>

      {/* Last update time */}
      <div className="text-xs text-gray-500 text-right">
        最后更新: {data?.lastUpdate ? new Date(data.lastUpdate).toLocaleString('zh-CN') : 'N/A'}
      </div>
    </div>
  );
};

export default NVMeDashboard;