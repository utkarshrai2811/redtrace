import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import { decodeBase64, encodeBase64 } from '../lib/encoding';
import type { HistoryView, TabInput, TabView } from '../lib/types';

const DEFAULT_RAW = 'GET / HTTP/1.1\r\nHost: \r\n\r\n';

/** A tab's edited request fields, kept in memory while the user works on it. */
export interface TabDraft {
  name: string;
  scheme: string;
  host: string;
  httpVersion: string;
  followRedirects: boolean;
  /** Decoded, editable raw request text. */
  raw: string;
}

function draftFromTab(tab: TabView): TabDraft {
  return {
    name: tab.name,
    scheme: tab.scheme,
    host: tab.host,
    httpVersion: tab.httpVersion,
    followRedirects: tab.followRedirects,
    raw: decodeBase64(tab.raw),
  };
}

function inputFromDraft(draft: TabDraft): TabInput {
  return {
    name: draft.name,
    scheme: draft.scheme,
    host: draft.host,
    httpVersion: draft.httpVersion,
    followRedirects: draft.followRedirects,
    raw: encodeBase64(draft.raw),
  };
}

interface RepeaterState {
  tabs: TabView[];
  loadingTabs: boolean;
  tabsError: string | null;

  selectedId: string | null;
  draft: TabDraft | null;
  history: HistoryView[];
  /** Index into `history` of the entry whose request/response is displayed. */
  selectedHistoryIndex: number;

  loadingDetail: boolean;
  detailError: string | null;
  sending: boolean;
  sendError: string | null;

  fetchTabs: () => Promise<void>;
  createTab: () => Promise<void>;
  selectTab: (id: string) => Promise<void>;
  updateDraft: (patch: Partial<TabDraft>) => void;
  selectHistory: (index: number) => void;
  saveAndSend: () => Promise<void>;
  deleteTab: (id: string) => Promise<void>;
}

// Guards against out-of-order detail loads clobbering a newer selection.
let detailToken = 0;

export const useRepeaterStore = create<RepeaterState>((set, get) => ({
  tabs: [],
  loadingTabs: false,
  tabsError: null,

  selectedId: null,
  draft: null,
  history: [],
  selectedHistoryIndex: -1,

  loadingDetail: false,
  detailError: null,
  sending: false,
  sendError: null,

  fetchTabs: async () => {
    set({ loadingTabs: true, tabsError: null });
    try {
      const tabs = await api.repeaterTabs();
      set({ loadingTabs: false, tabs });
      // Auto-select the first tab when nothing is selected (or the selection
      // no longer exists), so the page is never blank with tabs available.
      const { selectedId } = get();
      const stillExists = selectedId && tabs.some((t) => t.id === selectedId);
      if (!stillExists && tabs.length > 0) {
        void get().selectTab(tabs[0].id);
      } else if (tabs.length === 0) {
        set({ selectedId: null, draft: null, history: [], selectedHistoryIndex: -1 });
      }
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load tabs';
      set({ loadingTabs: false, tabsError: message });
    }
  },

  createTab: async () => {
    set({ tabsError: null });
    try {
      const tab = await api.createRepeaterTab({
        name: `Tab ${get().tabs.length + 1}`,
        scheme: 'https',
        host: '',
        followRedirects: false,
        httpVersion: 'HTTP/1.1',
        raw: encodeBase64(DEFAULT_RAW),
      });
      set((s) => ({ tabs: [...s.tabs, tab] }));
      void get().selectTab(tab.id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to create tab';
      set({ tabsError: message });
    }
  },

  selectTab: async (id) => {
    const token = ++detailToken;
    set({
      selectedId: id,
      loadingDetail: true,
      detailError: null,
      sendError: null,
      draft: null,
      history: [],
      selectedHistoryIndex: -1,
    });
    try {
      const { tab, history } = await api.getRepeaterTab(id);
      if (token !== detailToken) return;
      set({
        loadingDetail: false,
        draft: draftFromTab(tab),
        history,
        // Default to the most recent send, if any.
        selectedHistoryIndex: history.length > 0 ? history.length - 1 : -1,
      });
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load tab';
      set({ loadingDetail: false, detailError: message });
    }
  },

  updateDraft: (patch) => {
    set((s) => (s.draft ? { draft: { ...s.draft, ...patch } } : {}));
  },

  selectHistory: (index) => {
    set((s) => {
      if (index < 0 || index >= s.history.length) return {};
      return { selectedHistoryIndex: index };
    });
  },

  saveAndSend: async () => {
    const { selectedId, draft } = get();
    if (!selectedId || !draft) return;
    // Capture the selection token: if the user switches tabs (even away and
    // back to this same tab, which re-fetches its history) before the send
    // resolves, this stale result must not be appended onto the refreshed list.
    const token = detailToken;
    set({ sending: true, sendError: null });
    try {
      // The backend is the source of truth: persist the editor contents first,
      // then send. A network failure surfaces inline (502) without crashing.
      const updated = await api.updateRepeaterTab(selectedId, inputFromDraft(draft));
      if (token !== detailToken) {
        set({ sending: false });
        return;
      }
      set((s) => ({ tabs: s.tabs.map((t) => (t.id === updated.id ? updated : t)) }));

      const result = await api.sendRepeaterTab(selectedId);
      set((s) => {
        if (token !== detailToken) return { sending: false };
        const history = [...s.history, result.history];
        return {
          sending: false,
          history,
          selectedHistoryIndex: history.length - 1,
        };
      });
    } catch (err) {
      if (token !== detailToken) {
        set({ sending: false });
        return;
      }
      const message = err instanceof ApiError ? err.message : 'Send failed';
      set({ sending: false, sendError: message });
    }
  },

  deleteTab: async (id) => {
    try {
      await api.deleteRepeaterTab(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    set((s) => ({ tabs: s.tabs.filter((t) => t.id !== id) }));
    if (get().selectedId === id) {
      const remaining = get().tabs;
      if (remaining.length > 0) {
        void get().selectTab(remaining[0].id);
      } else {
        set({ selectedId: null, draft: null, history: [], selectedHistoryIndex: -1 });
      }
    }
  },
}));
