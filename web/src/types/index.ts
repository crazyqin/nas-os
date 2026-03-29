// NVMe类型定义
export interface NVMeDevice {
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

export interface NVMeAlert {
  device: string;
  type: 'temperature' | 'lifespan' | 'spare' | 'media_error';
  severity: 'warning' | 'critical';
  message: string;
  timestamp: string;
}

export interface NVMeDashboardData {
  devices: NVMeDevice[];
  criticalCount: number;
  warningCount: number;
  healthyCount: number;
  lastUpdate: string;
}

export interface AlertConfig {
  temperatureThreshold: number;
  percentUsedThreshold: number;
  availableSpareThreshold: number;
}

// 搜索类型定义
export interface SearchResult {
  id: string;
  type: 'file' | 'user' | 'setting' | 'api' | 'container' | 'share';
  title: string;
  description: string;
  path?: string;
  icon?: string;
  metadata?: Record<string, string | number>;
}

export interface SearchResponse {
  results: SearchResult[];
  total: number;
  query: string;
  type?: string;
}