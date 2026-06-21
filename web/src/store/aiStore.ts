import { create } from 'zustand';
import { api, ApiError, streamAIMessage } from '../lib/api';
import type {
  AIConfig,
  AIConfigInput,
  AIConversationView,
  AICreateInput,
  AIMessageView,
} from '../lib/types';

interface AIState {
  config: AIConfig | null;
  conversations: AIConversationView[];
  activeId: string | null;
  messages: AIMessageView[];

  loading: boolean;
  error: string | null;

  savingConfig: boolean;
  configError: string | null;

  streaming: boolean;
  streamError: string | null;
  /** In-progress assistant text being streamed (empty when not streaming). */
  draft: string;

  fetchConfig: () => Promise<void>;
  saveConfig: (input: AIConfigInput) => Promise<void>;
  fetchConversations: () => Promise<void>;
  openConversation: (id: string) => Promise<void>;
  newChat: () => Promise<void>;
  startFromContext: (input: AICreateInput) => Promise<string | null>;
  sendMessage: (content: string) => Promise<void>;
  generate: (content: string | null) => Promise<void>;
  deleteConversation: (id: string) => Promise<void>;
  clearConversations: () => Promise<void>;
}

// A single in-flight stream at a time; navigating to another conversation or
// starting a new chat aborts the prior stream so its deltas stop landing.
let streamController: AbortController | null = null;

function abortActiveStream(): void {
  if (streamController) {
    streamController.abort();
    streamController = null;
  }
}

export const useAIStore = create<AIState>((set, get) => ({
  config: null,
  conversations: [],
  activeId: null,
  messages: [],

  loading: false,
  error: null,

  savingConfig: false,
  configError: null,

  streaming: false,
  streamError: null,
  draft: '',

  fetchConfig: async () => {
    try {
      const config = await api.aiConfig();
      set({ config });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load AI config';
      set({ configError: message });
    }
  },

  saveConfig: async (input) => {
    set({ savingConfig: true, configError: null });
    try {
      const config = await api.updateAIConfig(input);
      set({ savingConfig: false, config });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to save AI config';
      set({ savingConfig: false, configError: message });
    }
  },

  fetchConversations: async () => {
    set({ loading: true, error: null });
    try {
      const conversations = await api.aiConversations();
      set({ loading: false, conversations });
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to load conversations';
      set({ loading: false, error: message });
    }
  },

  openConversation: async (id) => {
    abortActiveStream();
    set({
      activeId: id,
      messages: [],
      streaming: false,
      streamError: null,
      draft: '',
      error: null,
    });
    try {
      const { messages } = await api.aiConversation(id);
      // Ignore a stale load if the selection moved on while it was in flight.
      if (get().activeId !== id) return;
      set({ messages });
    } catch (err) {
      if (get().activeId !== id) return;
      const message = err instanceof ApiError ? err.message : 'Failed to load conversation';
      set({ error: message });
    }
  },

  newChat: async () => {
    abortActiveStream();
    try {
      const { conversation } = await api.createAIConversation({ kind: 'chat' });
      set((s) => ({
        conversations: [conversation, ...s.conversations],
        activeId: conversation.id,
        messages: [],
        streaming: false,
        streamError: null,
        draft: '',
        error: null,
      }));
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to start chat';
      set({ error: message });
    }
  },

  startFromContext: async (input) => {
    abortActiveStream();
    try {
      const { conversation, messages } = await api.createAIConversation(input);
      set((s) => ({
        conversations: [conversation, ...s.conversations],
        activeId: conversation.id,
        messages,
        streaming: false,
        streamError: null,
        draft: '',
        error: null,
      }));
      // The create seeded the first user turn from context; stream the reply now.
      await get().generate(null);
      return conversation.id;
    } catch (err) {
      const message = err instanceof ApiError ? err.message : 'Failed to start conversation';
      set({ error: message });
      return null;
    }
  },

  sendMessage: async (content) => {
    const { activeId } = get();
    if (!activeId) return;
    // Optimistically append the user turn; the backend persists the real row,
    // which reconciles on the next openConversation.
    const optimistic: AIMessageView = {
      id: `local-${Date.now()}`,
      role: 'user',
      content,
      createdAt: new Date().toISOString(),
    };
    set((s) => ({ messages: [...s.messages, optimistic] }));
    await get().generate(content);
  },

  generate: async (content) => {
    const { activeId } = get();
    if (!activeId) return;
    abortActiveStream();
    const controller = new AbortController();
    streamController = controller;
    set({ streaming: true, draft: '', streamError: null });
    try {
      await streamAIMessage(
        activeId,
        content,
        {
          onDelta: (t) => set((s) => ({ draft: s.draft + t })),
          onDone: (message) => {
            // A done frame can be {} when nothing was generated; only push a
            // real assistant message.
            if (!message || !message.id) return;
            set((s) => ({ messages: [...s.messages, message] }));
          },
          onError: (message) => set({ streamError: message }),
        },
        controller.signal,
      );
      // Refresh the conversation list so a newly derived title shows up.
      void get().fetchConversations();
    } catch (err) {
      // An abort is intentional (navigation / new stream); don't surface it.
      if (controller.signal.aborted) return;
      const message = err instanceof ApiError ? err.message : 'Failed to generate reply';
      set({ streamError: message });
    } finally {
      if (streamController === controller) streamController = null;
      // Only clear streaming state if this stream is still the active one.
      if (!controller.signal.aborted) set({ streaming: false, draft: '' });
    }
  },

  deleteConversation: async (id) => {
    try {
      await api.deleteAIConversation(id);
    } catch {
      // Drop it from the view regardless to stay responsive.
    }
    if (get().activeId === id) abortActiveStream();
    set((s) => ({
      conversations: s.conversations.filter((c) => c.id !== id),
      activeId: s.activeId === id ? null : s.activeId,
      messages: s.activeId === id ? [] : s.messages,
      streaming: s.activeId === id ? false : s.streaming,
      streamError: s.activeId === id ? null : s.streamError,
      draft: s.activeId === id ? '' : s.draft,
    }));
  },

  clearConversations: async () => {
    abortActiveStream();
    try {
      await api.clearAIConversations();
    } catch {
      // Clear the view regardless to stay responsive.
    }
    set({
      conversations: [],
      activeId: null,
      messages: [],
      streaming: false,
      streamError: null,
      draft: '',
    });
  },
}));
