import { useState, useEffect, useCallback } from 'react';

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

interface NVMeAlert {
  device: string;
  type: 'temperature' | 'lifespan' | 'spare' | 'media_error';
  severity: 'warning' | 'critical';
  message: string;
  timestamp: string;
}

interface NVMeDashboardData {
  devices: NVMeDevice[];
  criticalCount: number;
  warningCount: number;
  healthyCount: number;
  lastUpdate: string;
}

interface AlertConfig {
  temperatureThreshold: number;
  percentUsedThreshold: number;
  availableSpareThreshold: number;
}

interface UseNVMeResult {
  data: NVMeDashboardData | null;
  alerts: NVMeAlert[];
  loading: boolean;
  error: Error | null;
  refetch: () => Promise<void>;
  updateAlertConfig: (config: AlertConfig) => Promise<void>;
}

export function useNVMe(): UseNVMeResult {
  const [data, setData] = useState<NVMeDashboardData | null>(null);
  const [alerts, setAlerts] = useState<NVMeAlert[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);
    
    try {
      const response = await fetch('/api/v1/hardware/nvme');
      if (!response.ok) {
        throw new Error(`HTTP error: ${response.status}`);
      }
      
      const result: NVMeDashboardData = await response.json();
      setData(result);
      
      // Fetch alerts
      const alertsResponse = await fetch('/api/v1/hardware/nvme/alerts');
      if (alertsResponse.ok) {
        const alertsData: NVMeAlert[] = await alertsResponse.json();
        setAlerts(alertsData);
      }
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Unknown error'));
    } finally {
      setLoading(false);
    }
  }, []);

  const updateAlertConfig = useCallback(async (config: AlertConfig) => {
    try {
      const response = await fetch('/api/v1/hardware/nvme/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config)
      });
      
      if (!response.ok) {
        throw new Error(`Failed to update config: ${response.status}`);
      }
      
      // Refetch after config update
      await fetchData();
    } catch (err) {
      setError(err instanceof Error ? err : new Error('Failed to update config'));
    }
  }, [fetchData]);

  useEffect(() => {
    fetchData();
    
    // Set up polling interval (every 30 seconds)
    const interval = setInterval(fetchData, 30000);
    
    return () => clearInterval(interval);
  }, [fetchData]);

  return {
    data,
    alerts,
    loading,
    error,
    refetch: fetchData,
    updateAlertConfig
  };
}