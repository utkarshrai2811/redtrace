import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import type {
  SequencerReport,
  SeqSource,
  SeqTaskInput,
  SeqTaskView,
  SeqUpdate,
} from '../lib/types';

/** The editable capture config kept in memory while the user works on it. */
export interface SeqDraft {
  source: SeqSource;
  selector: string;
  target: number;
}

function draftFromTask(task: SeqTaskView): SeqDraft {
  return { source: task.source, selector: task.selector, target: task.target };
}

/**
 * Build the update payload from a task and its edited draft. The request
 * template is read-only, so it is passed through unchanged.
 */
function inputFromDraft(task: SeqTaskView, draft: SeqDraft): SeqTaskInput {
  return {
    name: task.name,
    scheme: task.scheme,
    host: task.host,
    template: task.template,
    httpVersion: task.httpVersion,
    source: draft.source,
    selector: draft.selector,
    target: draft.target,
  };
}

const TERMINAL: ReadonlySet<SeqTaskView['status']> = new Set([
  'completed',
  'stopped',
  'error',
]);

interface SequencerState {
  tasks: SeqTaskView[];
  loadingTasks: boolean;
  tasksError: string | null;

  selectedTaskId: string | null;
  selectedTask: SeqTaskView | null;
  report: SequencerReport | null;
  tokens: string[];
  draft: SeqDraft | null;

  loadingDetail: boolean;
  detailError: string | null;
  actionError: string | null;

  fetchTasks: () => Promise<void>;
  selectTask: (id: string) => Promise<void>;
  clearSelectedTask: () => void;
  updateDraft: (patch: Partial<SeqDraft>) => void;
  saveDraft: () => Promise<SeqTaskView | null>;
  startTask: (id: string) => Promise<void>;
  stopTask: (id: string) => Promise<void>;
  deleteTask: (id: string) => Promise<void>;
  applyUpdate: (u: SeqUpdate) => void;
}

// Guards against out-of-order detail loads clobbering a newer selection.
let detailToken = 0;

export const useSequencerStore = create<SequencerState>((set, get) => ({
  tasks: [],
  loadingTasks: false,
  tasksError: null,

  selectedTaskId: null,
  selectedTask: null,
  report: null,
  tokens: [],
  draft: null,

  loadingDetail: false,
  detailError: null,
  actionError: null,

  fetchTasks: async () => {
    set({ loadingTasks: true, tasksError: null });
    try {
      const tasks = await api.listSequencerTasks();
      set({ loadingTasks: false, tasks });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load tasks';
      set({ loadingTasks: false, tasksError: message });
    }
  },

  selectTask: async (id) => {
    const token = ++detailToken;
    set({
      selectedTaskId: id,
      loadingDetail: true,
      detailError: null,
      actionError: null,
      selectedTask: null,
      report: null,
      tokens: [],
      draft: null,
    });
    try {
      const { task, report, tokens } = await api.getSequencerTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        loadingDetail: false,
        selectedTask: task,
        report,
        tokens,
        draft: draftFromTask(task),
        // Keep the list row in sync with the freshly loaded detail.
        tasks: s.tasks.map((t) => (t.id === task.id ? task : t)),
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load task';
      set({ loadingDetail: false, detailError: message });
    }
  },

  clearSelectedTask: () => {
    detailToken++;
    set({
      selectedTaskId: null,
      selectedTask: null,
      report: null,
      tokens: [],
      draft: null,
      detailError: null,
    });
  },

  updateDraft: (patch) => {
    set((s) => {
      if (!s.draft) return {};
      return { draft: { ...s.draft, ...patch } };
    });
  },

  saveDraft: async () => {
    const { selectedTaskId, selectedTask, draft } = get();
    if (!selectedTaskId || !selectedTask || !draft) return null;
    const token = detailToken;
    set({ actionError: null });
    try {
      const updated = await api.updateSequencerTask(
        selectedTaskId,
        inputFromDraft(selectedTask, draft),
      );
      if (token !== detailToken) return updated;
      set((s) => ({
        selectedTask: s.selectedTaskId === updated.id ? updated : s.selectedTask,
        tasks: s.tasks.map((t) => (t.id === updated.id ? updated : t)),
      }));
      return updated;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to save task';
      set({ actionError: message });
      return null;
    }
  },

  startTask: async (id) => {
    const token = detailToken;
    set({ actionError: null });
    try {
      // Persist the editor contents first; the backend validates the config on
      // start.
      const saved = await get().saveDraft();
      if (token !== detailToken) return;
      if (!saved) return;
      const started = await api.startSequencerTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        tasks: s.tasks.map((t) => (t.id === started.id ? started : t)),
        selectedTask: s.selectedTaskId === started.id ? started : s.selectedTask,
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to start task';
      set({ actionError: message });
    }
  },

  stopTask: async (id) => {
    set({ actionError: null });
    try {
      await api.stopSequencerTask(id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to stop task';
      set({ actionError: message });
    }
  },

  deleteTask: async (id) => {
    try {
      await api.deleteSequencerTask(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) }));
    if (get().selectedTaskId === id) {
      get().clearSelectedTask();
    }
  },

  applyUpdate: (u) => {
    if (!u.taskId) return;
    set((s) => {
      const patch = (t: SeqTaskView): SeqTaskView => ({
        ...t,
        status: u.status ?? t.status,
        collected: u.collected,
      });
      const tasks = s.tasks.map((t) => (t.id === u.taskId ? patch(t) : t));
      const selectedTask =
        s.selectedTask && s.selectedTask.id === u.taskId ? patch(s.selectedTask) : s.selectedTask;
      return { tasks, selectedTask };
    });

    // When a capture finishes for the selected task, refetch the report+tokens.
    // selectTask carries the detailToken guard, so a stale completion is safe.
    if (
      u.kind === 'status' &&
      u.status &&
      TERMINAL.has(u.status) &&
      u.taskId === get().selectedTaskId
    ) {
      void get().selectTask(u.taskId);
    }
  },
}));
