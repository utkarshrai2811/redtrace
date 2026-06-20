import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import type {
  ScanIssueSummary,
  ScanIssueView,
  ScannerUpdate,
  ScanTaskView,
  Severity,
} from '../lib/types';

/** Rank for sorting issues high → medium → low → info. */
const SEVERITY_RANK: Record<Severity, number> = {
  high: 3,
  medium: 2,
  low: 1,
  info: 0,
};

/**
 * Sort issues by severity (high first) then newest first. Live issues arriving
 * over the WebSocket have no createdAt, so they sort as newest within a tier
 * (they are prepended on arrival and treated as the most recent).
 */
function sortIssues(issues: ScanIssueView[]): ScanIssueView[] {
  return issues
    .slice()
    .sort((a, b) => {
      const rank = SEVERITY_RANK[b.severity] - SEVERITY_RANK[a.severity];
      if (rank !== 0) return rank;
      // Newest first; missing/empty createdAt sorts ahead (it is a live row).
      return (b.createdAt ?? '').localeCompare(a.createdAt ?? '');
    });
}

/**
 * Promote a live WS issue summary to a full row. The summary lacks the detail
 * fields, so they default to empty — that is fine for the list (which only
 * needs the columns the summary carries); the issue detail view refetches the
 * complete record by id.
 */
function rowFromSummary(s: ScanIssueSummary): ScanIssueView {
  return {
    ...s,
    detail: '',
    evidence: '',
    remediation: '',
    createdAt: '',
  };
}

interface ScannerState {
  issues: ScanIssueView[];
  loadingIssues: boolean;
  issuesError: string | null;

  tasks: ScanTaskView[];
  loadingTasks: boolean;
  tasksError: string | null;

  selectedTaskId: string | null;
  selectedTask: ScanTaskView | null;
  taskIssues: ScanIssueView[];
  loadingDetail: boolean;
  detailError: string | null;
  actionError: string | null;

  // Filters for the issues dashboard.
  severityFilter: Set<Severity>;
  hostFilter: string;

  fetchIssues: () => Promise<void>;
  fetchTasks: () => Promise<void>;
  selectTask: (id: string) => Promise<void>;
  clearSelectedTask: () => void;
  startTask: (id: string) => Promise<void>;
  stopTask: (id: string) => Promise<void>;
  deleteTask: (id: string) => Promise<void>;
  clearIssues: () => Promise<void>;
  toggleSeverity: (s: Severity) => void;
  setHostFilter: (host: string) => void;
  applyUpdate: (u: ScannerUpdate) => void;
}

// Guards against out-of-order detail loads clobbering a newer selection.
let detailToken = 0;

export const useScannerStore = create<ScannerState>((set, get) => ({
  issues: [],
  loadingIssues: false,
  issuesError: null,

  tasks: [],
  loadingTasks: false,
  tasksError: null,

  selectedTaskId: null,
  selectedTask: null,
  taskIssues: [],
  loadingDetail: false,
  detailError: null,
  actionError: null,

  severityFilter: new Set<Severity>(),
  hostFilter: '',

  fetchIssues: async () => {
    set({ loadingIssues: true, issuesError: null });
    try {
      const issues = await api.listScanIssues();
      set({ loadingIssues: false, issues: sortIssues(issues) });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load issues';
      set({ loadingIssues: false, issuesError: message });
    }
  },

  fetchTasks: async () => {
    set({ loadingTasks: true, tasksError: null });
    try {
      const tasks = await api.listScanTasks();
      set({ loadingTasks: false, tasks });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load scans';
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
      taskIssues: [],
    });
    try {
      const { task, issues } = await api.getScanTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        loadingDetail: false,
        selectedTask: task,
        taskIssues: sortIssues(issues),
        // Keep the list row in sync with the freshly loaded detail.
        tasks: s.tasks.map((t) => (t.id === task.id ? task : t)),
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load scan';
      set({ loadingDetail: false, detailError: message });
    }
  },

  clearSelectedTask: () => {
    detailToken++;
    set({ selectedTaskId: null, selectedTask: null, taskIssues: [], detailError: null });
  },

  startTask: async (id) => {
    const token = detailToken;
    set({ actionError: null });
    try {
      const started = await api.startScanTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        tasks: s.tasks.map((t) => (t.id === started.id ? started : t)),
        selectedTask: s.selectedTaskId === started.id ? started : s.selectedTask,
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to start scan';
      set({ actionError: message });
    }
  },

  stopTask: async (id) => {
    set({ actionError: null });
    try {
      await api.stopScanTask(id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to stop scan';
      set({ actionError: message });
    }
  },

  deleteTask: async (id) => {
    try {
      await api.deleteScanTask(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) }));
    if (get().selectedTaskId === id) {
      get().clearSelectedTask();
    }
  },

  clearIssues: async () => {
    try {
      await api.clearScanIssues();
    } catch {
      // Clear the view regardless to stay responsive.
    }
    set({ issues: [] });
  },

  toggleSeverity: (sev) => {
    set((s) => {
      const next = new Set(s.severityFilter);
      if (next.has(sev)) next.delete(sev);
      else next.add(sev);
      return { severityFilter: next };
    });
  },

  setHostFilter: (host) => set({ hostFilter: host }),

  applyUpdate: (u) => {
    set((s) => {
      if (u.kind === 'issue' && u.issue) {
        const summary = u.issue;
        // Dedup by id: a reload + live updates must never duplicate a finding.
        if (s.issues.some((i) => i.id === summary.id)) return {};
        const row = rowFromSummary(summary);
        // Prepend then re-sort so the new finding lands in its severity tier as
        // the newest entry.
        return { issues: sortIssues([row, ...s.issues]) };
      }

      // progress / status: update the matching task's counters in the list, and
      // the selected task if it is the one progressing.
      if (!u.taskId) return {};
      const patch = (t: ScanTaskView): ScanTaskView => ({
        ...t,
        status: u.status ?? t.status,
        completed: u.completed,
        total: u.total,
        issues: u.issues,
      });
      const tasks = s.tasks.map((t) => (t.id === u.taskId ? patch(t) : t));
      const selectedTask =
        s.selectedTask && s.selectedTask.id === u.taskId ? patch(s.selectedTask) : s.selectedTask;
      return { tasks, selectedTask };
    });
  },
}));
