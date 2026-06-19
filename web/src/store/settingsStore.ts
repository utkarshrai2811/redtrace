import { create } from 'zustand';
import { api } from '../lib/api';
import type { HealthResponse, ProxySettings } from '../lib/types';

interface SettingsState {
  health: HealthResponse | null;
  proxySettings: ProxySettings | null;
  loaded: boolean;
  fetchAll: () => Promise<void>;
}

export const useSettingsStore = create<SettingsState>((set) => ({
  health: null,
  proxySettings: null,
  loaded: false,
  fetchAll: async () => {
    const [healthResult, settingsResult] = await Promise.allSettled([
      api.health(),
      api.proxySettings(),
    ]);
    set({
      health: healthResult.status === 'fulfilled' ? healthResult.value : null,
      proxySettings: settingsResult.status === 'fulfilled' ? settingsResult.value : null,
      loaded: true,
    });
  },
}));
