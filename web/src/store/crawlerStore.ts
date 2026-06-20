import { create } from 'zustand';
import { api, ApiError } from '../lib/api';
import type {
  CrawlPageSummary,
  CrawlTaskInput,
  CrawlTaskView,
  CrawlUpdate,
  CrawlURLView,
} from '../lib/types';

/**
 * Promote a live WS page summary to a full URL row. The summary lacks the
 * detail fields (id/length/createdAt), so they are synthesized — that is fine
 * for the discovered-URLs table, which only renders url/status/type/depth.
 */
function rowFromPage(taskId: string, p: CrawlPageSummary): CrawlURLView {
  return {
    id: `live:${taskId}:${p.url}`,
    taskId,
    url: p.url,
    method: 'GET',
    statusCode: p.statusCode,
    length: 0,
    contentType: p.contentType,
    depth: p.depth,
    createdAt: '',
  };
}

interface CrawlerState {
  tasks: CrawlTaskView[];
  loadingTasks: boolean;
  tasksError: string | null;

  selectedTaskId: string | null;
  selectedTask: CrawlTaskView | null;
  taskUrls: CrawlURLView[];
  loadingDetail: boolean;
  detailError: string | null;
  actionError: string | null;

  fetchTasks: () => Promise<void>;
  createTask: (input: CrawlTaskInput) => Promise<void>;
  selectTask: (id: string) => Promise<void>;
  clearSelectedTask: () => void;
  startTask: (id: string) => Promise<void>;
  stopTask: (id: string) => Promise<void>;
  deleteTask: (id: string) => Promise<void>;
  applyUpdate: (u: CrawlUpdate) => void;
}

// Guards against out-of-order detail loads clobbering a newer selection.
let detailToken = 0;

export const useCrawlerStore = create<CrawlerState>((set, get) => ({
  tasks: [],
  loadingTasks: false,
  tasksError: null,

  selectedTaskId: null,
  selectedTask: null,
  taskUrls: [],
  loadingDetail: false,
  detailError: null,
  actionError: null,

  fetchTasks: async () => {
    set({ loadingTasks: true, tasksError: null });
    try {
      const tasks = await api.listCrawlTasks();
      set({ loadingTasks: false, tasks });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load crawls';
      set({ loadingTasks: false, tasksError: message });
    }
  },

  createTask: async (input) => {
    set({ tasksError: null });
    try {
      const task = await api.createCrawlTask(input);
      set((s) => ({ tasks: [...s.tasks, task] }));
      void get().selectTask(task.id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to create crawl';
      set({ tasksError: message });
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
      taskUrls: [],
    });
    try {
      const { task, urls } = await api.getCrawlTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        loadingDetail: false,
        selectedTask: task,
        taskUrls: urls,
        // Keep the list row in sync with the freshly loaded detail.
        tasks: s.tasks.map((t) => (t.id === task.id ? task : t)),
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load crawl';
      set({ loadingDetail: false, detailError: message });
    }
  },

  clearSelectedTask: () => {
    detailToken++;
    set({ selectedTaskId: null, selectedTask: null, taskUrls: [], detailError: null });
  },

  startTask: async (id) => {
    const token = detailToken;
    set({ actionError: null });
    try {
      const started = await api.startCrawlTask(id);
      if (token !== detailToken) return;
      set((s) => ({
        tasks: s.tasks.map((t) => (t.id === started.id ? started : t)),
        selectedTask: s.selectedTaskId === started.id ? started : s.selectedTask,
        // Starting a (re)run clears the previous discovered URLs; they stream in
        // live and the full set reloads on the next select.
        taskUrls: s.selectedTaskId === started.id ? [] : s.taskUrls,
      }));
    } catch (err) {
      if (token !== detailToken) return;
      const message = err instanceof ApiError ? err.message : 'Failed to start crawl';
      set({ actionError: message });
    }
  },

  stopTask: async (id) => {
    set({ actionError: null });
    try {
      await api.stopCrawlTask(id);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to stop crawl';
      set({ actionError: message });
    }
  },

  deleteTask: async (id) => {
    try {
      await api.deleteCrawlTask(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) }));
    if (get().selectedTaskId === id) {
      get().clearSelectedTask();
    }
  },

  applyUpdate: (u) => {
    set((s) => {
      if (u.kind === 'page' && u.page && u.taskId && u.taskId === s.selectedTaskId) {
        // Append the freshly discovered URL, deduped by url (a reload + live
        // updates must never duplicate).
        if (s.taskUrls.some((r) => r.url === u.page!.url)) return {};
        return { taskUrls: [...s.taskUrls, rowFromPage(u.taskId, u.page)] };
      }

      // progress / status: patch the matching task's counters in the list, and
      // the selected task if it is the one progressing.
      if (!u.taskId) return {};
      const patch = (t: CrawlTaskView): CrawlTaskView => ({
        ...t,
        status: u.status ?? t.status,
        pages: u.pages,
        found: u.found,
      });
      const tasks = s.tasks.map((t) => (t.id === u.taskId ? patch(t) : t));
      const selectedTask =
        s.selectedTask && s.selectedTask.id === u.taskId ? patch(s.selectedTask) : s.selectedTask;
      return { tasks, selectedTask };
    });
  },
}));
