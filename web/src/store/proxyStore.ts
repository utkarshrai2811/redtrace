import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import type {
  InterceptState,
  RequestDetail,
  RequestFilters,
  RequestSummary,
} from '../lib/types';

const PER_PAGE = 100;

// Cap the live in-memory list so high-volume traffic never grows the DOM/table
// unbounded. The full history is still queryable from the database.
const MAX_ROWS = 500;

const DEFAULT_FILTERS: RequestFilters = {
  method: 'ANY',
  host: 'ANY',
  status: 'ANY',
  mime: '',
  q: '',
  scope: false,
};

const EMPTY_INTERCEPT: InterceptState = {
  enabled: false,
  interceptResponses: false,
  queue: [],
  count: 0,
};

interface ProxyState {
  rows: RequestSummary[];
  total: number;
  loadingList: boolean;
  listError: string | null;

  selectedId: string | null;
  detail: RequestDetail | null;
  loadingDetail: boolean;
  detailError: string | null;

  filters: RequestFilters;
  hosts: string[];

  intercept: InterceptState;

  fetchList: () => Promise<void>;
  fetchHosts: () => Promise<void>;
  setFilter: <K extends keyof RequestFilters>(key: K, value: RequestFilters[K]) => void;
  resetFilters: () => void;

  selectRow: (id: string | null) => Promise<void>;
  deleteRow: (id: string) => Promise<void>;
  clearRows: () => Promise<void>;

  fetchIntercept: () => Promise<void>;
  setInterceptEnabled: (enabled: boolean) => Promise<void>;
  setInterceptResponses: (interceptResponses: boolean) => Promise<void>;
  forwardItem: (id: string, raw?: string) => Promise<void>;
  dropItem: (id: string) => Promise<void>;
  forwardAll: () => Promise<void>;
  dropAll: () => Promise<void>;

  applyTrafficFrame: (summary: RequestSummary) => void;
  applyInterceptFrame: (state: InterceptState) => void;
}

function matchesFilters(row: RequestSummary, f: RequestFilters): boolean {
  if (f.method !== 'ANY' && row.method !== f.method) return false;
  if (f.host !== 'ANY' && row.host !== f.host) return false;
  if (f.scope && !row.inScope) return false;
  return true;
}

export const useProxyStore = create<ProxyState>((set, get) => ({
  rows: [],
  total: 0,
  loadingList: false,
  listError: null,

  selectedId: null,
  detail: null,
  loadingDetail: false,
  detailError: null,

  filters: { ...DEFAULT_FILTERS },
  hosts: [],

  intercept: { ...EMPTY_INTERCEPT },

  fetchList: async () => {
    set({ loadingList: true, listError: null });
    try {
      const res = await api.listRequests(get().filters, 1, PER_PAGE);
      set({ rows: res.data, total: res.total, loadingList: false });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load requests';
      set({ loadingList: false, listError: message });
    }
  },

  fetchHosts: async () => {
    try {
      const hosts = await api.hosts();
      set({ hosts });
    } catch {
      // Non-fatal: host dropdown just stays empty.
    }
  },

  setFilter: (key, value) => {
    set((s) => ({ filters: { ...s.filters, [key]: value } }));
    void get().fetchList();
  },

  resetFilters: () => {
    set({ filters: { ...DEFAULT_FILTERS } });
    void get().fetchList();
  },

  selectRow: async (id) => {
    if (id === null) {
      set({ selectedId: null, detail: null, detailError: null });
      return;
    }
    set({ selectedId: id, loadingDetail: true, detail: null, detailError: null });
    try {
      const detail = await api.getRequest(id);
      // Guard against a race where the user selected another row meanwhile.
      if (get().selectedId === id) {
        set({ detail, loadingDetail: false });
      }
    } catch (err) {
      if (get().selectedId === id) {
        const message = err instanceof ApiError ? err.message : 'Failed to load request';
        set({ loadingDetail: false, detailError: message });
      }
    }
  },

  deleteRow: async (id) => {
    try {
      await api.deleteRequest(id);
    } catch {
      // If the delete fails we still drop it from the view to stay responsive;
      // a refresh would bring it back if the server kept it.
    }
    set((s) => {
      const rows = s.rows.filter((r) => r.id !== id);
      const clearing = s.selectedId === id;
      return {
        rows,
        total: Math.max(0, s.total - 1),
        selectedId: clearing ? null : s.selectedId,
        detail: clearing ? null : s.detail,
      };
    });
  },

  clearRows: async () => {
    try {
      await api.clearRequests();
    } catch {
      // Clear the view regardless so the UI stays responsive.
    }
    set({ rows: [], total: 0, selectedId: null, detail: null, detailError: null });
  },

  fetchIntercept: async () => {
    try {
      const intercept = await api.getIntercept();
      set({ intercept });
    } catch {
      // Non-fatal.
    }
  },

  setInterceptEnabled: async (enabled) => {
    try {
      const intercept = await api.updateIntercept({ enabled });
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  setInterceptResponses: async (interceptResponses) => {
    try {
      const intercept = await api.updateIntercept({ interceptResponses });
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  forwardItem: async (id, raw) => {
    try {
      const intercept = await api.forwardItem(id, raw);
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  dropItem: async (id) => {
    try {
      const intercept = await api.dropItem(id);
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  forwardAll: async () => {
    try {
      const intercept = await api.forwardAll();
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  dropAll: async () => {
    try {
      const intercept = await api.dropAll();
      set({ intercept });
    } catch {
      void get().fetchIntercept();
    }
  },

  applyTrafficFrame: (summary) => {
    set((s) => {
      if (!matchesFilters(summary, s.filters)) {
        // Still bump the total so the dashboard/count stays roughly accurate,
        // but keep it out of the filtered view.
        return { total: s.total + 1 };
      }
      // Replace if we already have this id (e.g. response arrived after request).
      const existingIndex = s.rows.findIndex((r) => r.id === summary.id);
      if (existingIndex >= 0) {
        const rows = s.rows.slice();
        rows[existingIndex] = summary;
        return { rows };
      }
      const rows = [summary, ...s.rows];
      if (rows.length > MAX_ROWS) rows.length = MAX_ROWS;
      return { rows, total: s.total + 1 };
    });
  },

  applyInterceptFrame: (state) => {
    set({ intercept: state });
  },
}));
