import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import { decodeBase64, encodeBase64 } from '../lib/encoding';
import type {
  AttackType,
  AttackView,
  IntruderInput,
  IntruderResultView,
  IntruderUpdate,
  PayloadSet,
} from '../lib/types';

const DEFAULT_TEMPLATE = 'GET / HTTP/1.1\r\nHost: \r\n\r\n';

/** The position marker the editor wraps a selection in (U+00A7). */
export const MARKER = '§';

/** Worker cap enforced by the backend engine (intruder/engine.go maxConcurrency). */
export const MAX_CONCURRENCY = 64;

/** Count payload positions in a template: each pair of § markers is one position. */
export function countPositions(template: string): number {
  let count = 0;
  for (const ch of template) {
    if (ch === MARKER) count += 1;
  }
  return Math.floor(count / 2);
}

/** Attack types that take a single shared payload set across all positions. */
function sharesOnePayloadSet(type: AttackType): boolean {
  return type === 'sniper' || type === 'battering_ram';
}

/**
 * The number of payload-set editors an attack needs: one for sniper/battering
 * ram, one per position for pitchfork/cluster bomb (at least one so the UI is
 * never empty).
 */
export function payloadSetCount(type: AttackType, positions: number): number {
  if (sharesOnePayloadSet(type)) return 1;
  return Math.max(1, positions);
}

/**
 * Projected number of requests an attack will generate, mirroring the backend
 * Count() so the UI can gate Start (and warn about large attacks) before the
 * server validates. Blank payload lines are dropped, matching inputFromDraft.
 */
export function projectedJobCount(
  type: AttackType,
  positions: number,
  payloadSets: PayloadSet[],
): number {
  if (positions <= 0) return 0;
  const lens = payloadSets.map((s) => s.payloads.filter((p) => p.length > 0).length);
  switch (type) {
    case 'sniper':
      return positions * (lens[0] ?? 0);
    case 'battering_ram':
      return lens[0] ?? 0;
    case 'pitchfork': {
      const contributing = lens.slice(0, positions);
      return contributing.length > 0 ? Math.min(...contributing) : 0;
    }
    case 'cluster_bomb': {
      const contributing = lens.slice(0, positions);
      return contributing.length > 0 && contributing.every((l) => l > 0)
        ? contributing.reduce((a, b) => a * b, 1)
        : 0;
    }
    default:
      return 0;
  }
}

function emptyPayloadSet(): PayloadSet {
  return { payloads: [], processors: [] };
}

/** Pad/trim a payload-set list to the count the attack type requires. */
function syncPayloadSets(sets: PayloadSet[], count: number): PayloadSet[] {
  const next = sets.slice(0, count);
  while (next.length < count) next.push(emptyPayloadSet());
  return next;
}

/** An attack's edited config, kept in memory while the user works on it. */
export interface AttackDraft {
  name: string;
  scheme: string;
  host: string;
  httpVersion: string;
  followRedirects: boolean;
  concurrency: number;
  type: AttackType;
  /** Decoded, editable template text (with § markers). */
  templateText: string;
  payloadSets: PayloadSet[];
}

function draftFromAttack(attack: AttackView): AttackDraft {
  const templateText = decodeBase64(attack.template);
  const positions = countPositions(templateText);
  return {
    name: attack.name,
    scheme: attack.scheme,
    host: attack.host,
    httpVersion: attack.httpVersion,
    followRedirects: attack.followRedirects,
    concurrency: attack.concurrency,
    type: attack.type,
    templateText,
    payloadSets: syncPayloadSets(attack.payloadSets ?? [], payloadSetCount(attack.type, positions)),
  };
}

function inputFromDraft(draft: AttackDraft): IntruderInput {
  const positions = countPositions(draft.templateText);
  const sets = syncPayloadSets(draft.payloadSets, payloadSetCount(draft.type, positions));
  return {
    name: draft.name,
    scheme: draft.scheme,
    host: draft.host,
    httpVersion: draft.httpVersion,
    followRedirects: draft.followRedirects,
    concurrency: draft.concurrency,
    type: draft.type,
    template: encodeBase64(draft.templateText),
    // Drop blank payload lines kept for textarea round-tripping; keep processors.
    payloadSets: sets.map((s) => ({
      payloads: s.payloads.filter((p) => p.length > 0),
      processors: s.processors ?? [],
    })),
  };
}

interface IntruderState {
  attacks: AttackView[];
  loadingAttacks: boolean;
  attacksError: string | null;

  selectedId: string | null;
  draft: AttackDraft | null;
  results: IntruderResultView[];

  loadingDetail: boolean;
  detailError: string | null;
  starting: boolean;
  actionError: string | null;

  fetchAttacks: () => Promise<void>;
  createAttack: () => Promise<void>;
  selectAttack: (id: string) => Promise<void>;
  updateDraft: (patch: Partial<AttackDraft>) => void;
  saveDraft: () => Promise<AttackView | null>;
  start: () => Promise<void>;
  stop: () => Promise<void>;
  deleteAttack: (id: string) => Promise<void>;
  applyUpdate: (u: IntruderUpdate) => void;
}

// Guards against out-of-order detail loads clobbering a newer selection.
let detailToken = 0;

export const useIntruderStore = create<IntruderState>((set, get) => ({
  attacks: [],
  loadingAttacks: false,
  attacksError: null,

  selectedId: null,
  draft: null,
  results: [],

  loadingDetail: false,
  detailError: null,
  starting: false,
  actionError: null,

  fetchAttacks: async () => {
    set({ loadingAttacks: true, attacksError: null });
    try {
      const attacks = await api.listIntruderAttacks();
      set({ loadingAttacks: false, attacks });
      // Auto-select the first attack when nothing is selected (or the selection
      // no longer exists), so the page is never blank with attacks available.
      const { selectedId } = get();
      const stillExists = selectedId && attacks.some((a) => a.id === selectedId);
      if (!stillExists && attacks.length > 0) {
        void get().selectAttack(attacks[0].id);
      } else if (attacks.length === 0) {
        set({ selectedId: null, draft: null, results: [] });
      }
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load attacks';
      set({ loadingAttacks: false, attacksError: message });
    }
  },

  createAttack: async () => {
    set({ attacksError: null });
    try {
      const attack = await api.createIntruderAttack({
        name: `Attack ${get().attacks.length + 1}`,
        scheme: 'https',
        host: '',
        template: encodeBase64(DEFAULT_TEMPLATE),
        type: 'sniper',
        payloadSets: [emptyPayloadSet()],
        followRedirects: false,
        httpVersion: 'HTTP/1.1',
        concurrency: 5,
      });
      set((s) => ({ attacks: [...s.attacks, attack] }));
      void get().selectAttack(attack.id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to create attack';
      set({ attacksError: message });
    }
  },

  selectAttack: async (id) => {
    const token = ++detailToken;
    set({
      selectedId: id,
      loadingDetail: true,
      detailError: null,
      actionError: null,
      draft: null,
      results: [],
    });
    try {
      const { attack, results } = await api.getIntruderAttack(id);
      if (token !== detailToken) return;
      set((s) => ({
        loadingDetail: false,
        draft: draftFromAttack(attack),
        results,
        // Keep the list row in sync with the freshly loaded detail.
        attacks: s.attacks.map((a) => (a.id === attack.id ? attack : a)),
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load attack';
      set({ loadingDetail: false, detailError: message });
    }
  },

  updateDraft: (patch) => {
    set((s) => {
      if (!s.draft) return {};
      const draft = { ...s.draft, ...patch };
      // Keep payloadSets in sync whenever the type or template (position count)
      // changes, so the editors always match what the attack needs.
      const positions = countPositions(draft.templateText);
      draft.payloadSets = syncPayloadSets(draft.payloadSets, payloadSetCount(draft.type, positions));
      return { draft };
    });
  },

  saveDraft: async () => {
    const { selectedId, draft } = get();
    if (!selectedId || !draft) return null;
    const token = detailToken;
    try {
      const updated = await api.updateIntruderAttack(selectedId, inputFromDraft(draft));
      if (token !== detailToken) return updated;
      set((s) => ({ attacks: s.attacks.map((a) => (a.id === updated.id ? updated : a)) }));
      return updated;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to save attack';
      set({ actionError: message });
      return null;
    }
  },

  start: async () => {
    const { selectedId } = get();
    if (!selectedId) return;
    const token = detailToken;
    set({ starting: true, actionError: null });
    try {
      // Persist the editor contents first; the backend is the source of truth
      // and validates the config on start.
      const saved = await get().saveDraft();
      if (token !== detailToken) {
        set({ starting: false });
        return;
      }
      if (!saved) {
        set({ starting: false });
        return;
      }
      const started = await api.startIntruderAttack(selectedId);
      if (token !== detailToken) {
        set({ starting: false });
        return;
      }
      set((s) => ({
        starting: false,
        results: [],
        attacks: s.attacks.map((a) => (a.id === started.id ? started : a)),
      }));
    } catch (err) {
      if (token !== detailToken) {
        set({ starting: false });
        return;
      }
      const message = err instanceof ApiError ? err.message : 'Failed to start attack';
      set({ starting: false, actionError: message });
    }
  },

  stop: async () => {
    const { selectedId } = get();
    if (!selectedId) return;
    set({ actionError: null });
    try {
      await api.stopIntruderAttack(selectedId);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to stop attack';
      set({ actionError: message });
    }
  },

  deleteAttack: async (id) => {
    try {
      await api.deleteIntruderAttack(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    set((s) => ({ attacks: s.attacks.filter((a) => a.id !== id) }));
    if (get().selectedId === id) {
      const remaining = get().attacks;
      if (remaining.length > 0) {
        void get().selectAttack(remaining[0].id);
      } else {
        set({ selectedId: null, draft: null, results: [] });
      }
    }
  },

  applyUpdate: (u) => {
    set((s) => {
      // Always reflect attack-level progress in the list row.
      const attacks = s.attacks.map((a) =>
        a.id === u.attackId
          ? { ...a, status: u.status, completed: u.completed, total: u.total }
          : a,
      );

      if (u.attackId !== s.selectedId) {
        return { attacks };
      }

      // Update the selected attack's status/progress in the draft-adjacent list
      // entry above; merge per-result rows into the results list, deduped by
      // index so a reload + live updates never duplicate, and kept in index
      // order on insert so the table never has to re-sort the whole array.
      let results = s.results;
      if (u.result) {
        const incoming = u.result;
        const idx = results.findIndex((r) => r.index === incoming.index);
        if (idx >= 0) {
          results = results.slice();
          results[idx] = incoming;
        } else {
          const next = results.slice();
          let i = next.length;
          while (i > 0 && next[i - 1].index > incoming.index) i--;
          next.splice(i, 0, incoming);
          results = next;
        }
      }

      return { attacks, results };
    });
  },
}));
