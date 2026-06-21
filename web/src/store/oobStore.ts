import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import type { OOBConfig, OOBInteractionView, OOBPayloadView } from '../lib/types';

interface OOBState {
  config: OOBConfig | null;
  payloads: OOBPayloadView[];
  interactions: OOBInteractionView[];

  loading: boolean;
  error: string | null;

  generating: boolean;
  actionError: string | null;

  fetchAll: () => Promise<void>;
  generatePayload: () => Promise<OOBPayloadView | null>;
  clearInteractions: () => Promise<void>;
  applyUpdate: (i: OOBInteractionView) => void;
}

export const useOOBStore = create<OOBState>((set) => ({
  config: null,
  payloads: [],
  interactions: [],

  loading: false,
  error: null,

  generating: false,
  actionError: null,

  fetchAll: async () => {
    set({ loading: true, error: null });
    try {
      // Load config first; the listeners may be disabled, in which case the
      // page renders the configuration notice and skips the rest.
      const config = await api.oobConfig();
      if (!config.enabled) {
        set({ loading: false, config, payloads: [], interactions: [] });
        return;
      }
      const [payloads, interactions] = await Promise.all([
        api.oobPayloads(),
        api.oobInteractions(),
      ]);
      set({ loading: false, config, payloads, interactions });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load collaborator state';
      set({ loading: false, error: message });
    }
  },

  generatePayload: async () => {
    set({ generating: true, actionError: null });
    try {
      const payload = await api.generateOOBPayload();
      // Prepend the freshly minted payload so it appears at the top of the list.
      set((s) => ({ generating: false, payloads: [payload, ...s.payloads] }));
      return payload;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to generate payload';
      set({ generating: false, actionError: message });
      return null;
    }
  },

  clearInteractions: async () => {
    try {
      await api.clearOOBInteractions();
    } catch {
      // Clear the view regardless to stay responsive.
    }
    // The backend drops every interaction, so zero each payload's counter here
    // too rather than leaving the badges stale.
    set((s) => ({
      interactions: [],
      payloads: s.payloads.map((p) => ({ ...p, interactions: 0 })),
    }));
  },

  applyUpdate: (i) => {
    set((s) => {
      // Dedup by id (a reload + live updates must never duplicate). Prepend so
      // the newest callback lands at the top of the table.
      if (s.interactions.some((x) => x.id === i.id)) return {};
      // Bump the owning payload's counter when the interaction carries a token.
      const payloads = i.token
        ? s.payloads.map((p) =>
            p.token === i.token ? { ...p, interactions: p.interactions + 1 } : p,
          )
        : s.payloads;
      return { interactions: [i, ...s.interactions], payloads };
    });
  },
}));
