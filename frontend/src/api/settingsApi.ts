import { api } from './authApi';

export interface PlatformRateConfig {
  enabled: boolean;
  concurrency: number;
  request_delay_ms: number;
  max_requests_per_second: number;
}

export interface MemoryConfig {
  history_ttl_minutes: number;
  cleanup_interval_minutes: number;
}

export const settingsApi = {
  getRateConfigSettings: async () => {
    const response = await api.get<{ data: Record<string, PlatformRateConfig> }>('/settings/rate-config');
    return response.data.data;
  },

  updateRateConfigSettings: async (settings: Record<string, PlatformRateConfig>) => {
    const response = await api.put<{
      message: string;
      data: Record<string, PlatformRateConfig>;
    }>('/settings/rate-config', { settings });
    return response.data;
  },

  getMemoryConfig: async () => {
    const response = await api.get<{ data: MemoryConfig }>('/settings/memory-config');
    return response.data.data;
  },

  updateMemoryConfig: async (config: MemoryConfig) => {
    const response = await api.put<{
      message: string;
      data: MemoryConfig;
    }>('/settings/memory-config', { config });
    return response.data;
  },
};
