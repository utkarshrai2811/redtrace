import { authHeaders } from './auth';
import type {
  AIConfig,
  AIConfigInput,
  AIConversationDetail,
  AIConversationView,
  AICreateInput,
  AIMessageView,
  ComparerMode,
  ComparerResponse,
  CrawlTaskDetail,
  CrawlTaskInput,
  CrawlTaskView,
  DecoderDetectResponse,
  DecoderOp,
  DecoderRunResponse,
  HealthResponse,
  InterceptState,
  IntruderAttackDetail,
  IntruderInput,
  IntruderResultDetail,
  AttackView,
  MatchReplaceRule,
  OOBConfig,
  OOBInteractionDetail,
  OOBInteractionView,
  OOBPayloadView,
  ProxySettings,
  RequestDetail,
  RequestFilters,
  RequestListResponse,
  ScanIssueDetail,
  ScanIssueView,
  ScanTaskDetail,
  ScanTaskInput,
  ScanTaskView,
  ScopeRule,
  SendResponse,
  SeqTaskDetail,
  SeqTaskInput,
  SeqTaskView,
  SitemapHost,
  SitemapNoteInput,
  TabDetail,
  TabInput,
  TabView,
} from './types';

/**
 * Thrown when the backend returns a non-2xx response. Carries the structured
 * error code/message from the `{ error: { code, message } }` envelope when present.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

function isErrorEnvelope(value: unknown): value is { error: { code?: string; message?: string } } {
  return (
    typeof value === 'object' &&
    value !== null &&
    'error' in value &&
    typeof (value as { error: unknown }).error === 'object'
  );
}

async function request<T>(input: string, init?: RequestInit): Promise<T> {
  const res = await fetch(input, {
    ...init,
    headers: {
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...authHeaders(),
      ...init?.headers,
    },
  });

  if (res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  let parsed: unknown = null;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = null;
    }
  }

  if (!res.ok) {
    if (isErrorEnvelope(parsed)) {
      throw new ApiError(
        res.status,
        parsed.error.code ?? 'unknown',
        parsed.error.message ?? res.statusText,
      );
    }
    throw new ApiError(res.status, 'http_error', text || res.statusText);
  }

  return parsed as T;
}

function buildQuery(params: Record<string, string | number | undefined>): string {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') {
      qs.set(key, String(value));
    }
  }
  const s = qs.toString();
  return s ? `?${s}` : '';
}

export const api = {
  health(): Promise<HealthResponse> {
    return request<HealthResponse>('/api/health');
  },

  proxySettings(): Promise<ProxySettings> {
    return request<ProxySettings>('/api/proxy/settings');
  },

  listRequests(filters: RequestFilters, page: number, perPage: number): Promise<RequestListResponse> {
    const query = buildQuery({
      page,
      per_page: perPage,
      method: filters.method === 'ANY' ? '' : filters.method,
      host: filters.host === 'ANY' ? '' : filters.host,
      status: filters.status === 'ANY' ? '' : filters.status,
      mime: filters.mime,
      q: filters.q,
      scope: filters.scope ? 'in' : '',
    });
    return request<RequestListResponse>(`/api/requests${query}`);
  },

  getRequest(id: string): Promise<RequestDetail> {
    return request<RequestDetail>(`/api/requests/${encodeURIComponent(id)}`);
  },

  deleteRequest(id: string): Promise<void> {
    return request<void>(`/api/requests/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  clearRequests(): Promise<void> {
    return request<void>('/api/requests', { method: 'DELETE' });
  },

  hosts(): Promise<string[]> {
    return request<string[]>('/api/hosts');
  },

  getScope(): Promise<ScopeRule[]> {
    return request<ScopeRule[]>('/api/scope');
  },

  putScope(rules: ScopeRule[]): Promise<ScopeRule[]> {
    return request<ScopeRule[]>('/api/scope', { method: 'PUT', body: JSON.stringify(rules) });
  },

  getRules(): Promise<MatchReplaceRule[]> {
    return request<MatchReplaceRule[]>('/api/intercept/rules');
  },

  putRules(rules: MatchReplaceRule[]): Promise<MatchReplaceRule[]> {
    return request<MatchReplaceRule[]>('/api/intercept/rules', {
      method: 'PUT',
      body: JSON.stringify(rules),
    });
  },

  getIntercept(): Promise<InterceptState> {
    return request<InterceptState>('/api/intercept');
  },

  updateIntercept(body: {
    enabled?: boolean;
    interceptResponses?: boolean;
  }): Promise<InterceptState> {
    return request<InterceptState>('/api/intercept', {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  forwardItem(id: string, raw?: string): Promise<InterceptState> {
    const init: RequestInit = { method: 'POST' };
    if (raw !== undefined) {
      init.body = JSON.stringify({ raw });
    }
    return request<InterceptState>(`/api/intercept/${encodeURIComponent(id)}/forward`, init);
  },

  dropItem(id: string): Promise<InterceptState> {
    return request<InterceptState>(`/api/intercept/${encodeURIComponent(id)}/drop`, {
      method: 'POST',
    });
  },

  forwardAll(): Promise<InterceptState> {
    return request<InterceptState>('/api/intercept/forward-all', { method: 'POST' });
  },

  dropAll(): Promise<InterceptState> {
    return request<InterceptState>('/api/intercept/drop-all', { method: 'POST' });
  },

  // --- Decoder ---

  decoderRun(input: string, operations: DecoderOp[]): Promise<DecoderRunResponse> {
    return request<DecoderRunResponse>('/api/decoder/run', {
      method: 'POST',
      body: JSON.stringify({ input, operations }),
    });
  },

  decoderDetect(input: string): Promise<DecoderDetectResponse> {
    return request<DecoderDetectResponse>('/api/decoder/detect', {
      method: 'POST',
      body: JSON.stringify({ input }),
    });
  },

  // --- Comparer ---

  comparer(a: string, b: string, mode: ComparerMode): Promise<ComparerResponse> {
    return request<ComparerResponse>('/api/comparer', {
      method: 'POST',
      body: JSON.stringify({ a, b, mode }),
    });
  },

  // --- Repeater ---

  repeaterTabs(): Promise<TabView[]> {
    return request<TabView[]>('/api/repeater/tabs');
  },

  createRepeaterTab(body: TabInput): Promise<TabView> {
    return request<TabView>('/api/repeater/tabs', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  getRepeaterTab(id: string): Promise<TabDetail> {
    return request<TabDetail>(`/api/repeater/tabs/${encodeURIComponent(id)}`);
  },

  updateRepeaterTab(id: string, body: TabInput): Promise<TabView> {
    return request<TabView>(`/api/repeater/tabs/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  deleteRepeaterTab(id: string): Promise<void> {
    return request<void>(`/api/repeater/tabs/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  },

  sendRepeaterTab(id: string): Promise<SendResponse> {
    return request<SendResponse>(`/api/repeater/tabs/${encodeURIComponent(id)}/send`, {
      method: 'POST',
    });
  },

  // --- Site Map ---

  sitemap(): Promise<SitemapHost[]> {
    return request<SitemapHost[]>('/api/sitemap');
  },

  putSitemapNote(body: SitemapNoteInput): Promise<void> {
    return request<void>('/api/sitemap/note', {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  // --- Intruder (template/raw fields are base64) ---

  listIntruderAttacks(): Promise<AttackView[]> {
    return request<AttackView[]>('/api/intruder/attacks');
  },

  createIntruderAttack(body: IntruderInput): Promise<AttackView> {
    return request<AttackView>('/api/intruder/attacks', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  getIntruderAttack(id: string): Promise<IntruderAttackDetail> {
    return request<IntruderAttackDetail>(`/api/intruder/attacks/${encodeURIComponent(id)}`);
  },

  updateIntruderAttack(id: string, body: IntruderInput): Promise<AttackView> {
    return request<AttackView>(`/api/intruder/attacks/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  deleteIntruderAttack(id: string): Promise<void> {
    return request<void>(`/api/intruder/attacks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  },

  startIntruderAttack(id: string): Promise<AttackView> {
    return request<AttackView>(`/api/intruder/attacks/${encodeURIComponent(id)}/start`, {
      method: 'POST',
    });
  },

  stopIntruderAttack(id: string): Promise<{ stopped: boolean }> {
    return request<{ stopped: boolean }>(`/api/intruder/attacks/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
    });
  },

  getIntruderResult(id: string): Promise<IntruderResultDetail> {
    return request<IntruderResultDetail>(`/api/intruder/results/${encodeURIComponent(id)}`);
  },

  // --- Scanner (request/response raw fields are base64) ---

  listScanIssues(): Promise<ScanIssueView[]> {
    return request<ScanIssueView[]>('/api/scanner/issues');
  },

  getScanIssue(id: string): Promise<ScanIssueDetail> {
    return request<ScanIssueDetail>(`/api/scanner/issues/${encodeURIComponent(id)}`);
  },

  clearScanIssues(): Promise<void> {
    return request<void>('/api/scanner/issues', { method: 'DELETE' });
  },

  listScanTasks(): Promise<ScanTaskView[]> {
    return request<ScanTaskView[]>('/api/scanner/tasks');
  },

  createScanTask(body: ScanTaskInput): Promise<ScanTaskView> {
    return request<ScanTaskView>('/api/scanner/tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  getScanTask(id: string): Promise<ScanTaskDetail> {
    return request<ScanTaskDetail>(`/api/scanner/tasks/${encodeURIComponent(id)}`);
  },

  updateScanTask(id: string, body: ScanTaskInput): Promise<ScanTaskView> {
    return request<ScanTaskView>(`/api/scanner/tasks/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  deleteScanTask(id: string): Promise<void> {
    return request<void>(`/api/scanner/tasks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  },

  startScanTask(id: string): Promise<ScanTaskView> {
    return request<ScanTaskView>(`/api/scanner/tasks/${encodeURIComponent(id)}/start`, {
      method: 'POST',
    });
  },

  stopScanTask(id: string): Promise<{ stopped: boolean }> {
    return request<{ stopped: boolean }>(`/api/scanner/tasks/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
    });
  },

  // --- Crawler ---

  listCrawlTasks(): Promise<CrawlTaskView[]> {
    return request<CrawlTaskView[]>('/api/crawler/tasks');
  },

  createCrawlTask(body: CrawlTaskInput): Promise<CrawlTaskView> {
    return request<CrawlTaskView>('/api/crawler/tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  getCrawlTask(id: string): Promise<CrawlTaskDetail> {
    return request<CrawlTaskDetail>(`/api/crawler/tasks/${encodeURIComponent(id)}`);
  },

  deleteCrawlTask(id: string): Promise<void> {
    return request<void>(`/api/crawler/tasks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  },

  startCrawlTask(id: string): Promise<CrawlTaskView> {
    return request<CrawlTaskView>(`/api/crawler/tasks/${encodeURIComponent(id)}/start`, {
      method: 'POST',
    });
  },

  stopCrawlTask(id: string): Promise<{ stopped: boolean }> {
    return request<{ stopped: boolean }>(`/api/crawler/tasks/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
    });
  },

  // --- Sequencer (template is base64) ---

  listSequencerTasks(): Promise<SeqTaskView[]> {
    return request<SeqTaskView[]>('/api/sequencer/tasks');
  },

  createSequencerTask(body: SeqTaskInput): Promise<SeqTaskView> {
    return request<SeqTaskView>('/api/sequencer/tasks', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  },

  getSequencerTask(id: string): Promise<SeqTaskDetail> {
    return request<SeqTaskDetail>(`/api/sequencer/tasks/${encodeURIComponent(id)}`);
  },

  updateSequencerTask(id: string, body: SeqTaskInput): Promise<SeqTaskView> {
    return request<SeqTaskView>(`/api/sequencer/tasks/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    });
  },

  deleteSequencerTask(id: string): Promise<void> {
    return request<void>(`/api/sequencer/tasks/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    });
  },

  startSequencerTask(id: string): Promise<SeqTaskView> {
    return request<SeqTaskView>(`/api/sequencer/tasks/${encodeURIComponent(id)}/start`, {
      method: 'POST',
    });
  },

  stopSequencerTask(id: string): Promise<{ stopped: boolean }> {
    return request<{ stopped: boolean }>(`/api/sequencer/tasks/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
    });
  },

  // --- OOB / Collaborator (interaction raw field is base64) ---

  oobConfig(): Promise<OOBConfig> {
    return request<OOBConfig>('/api/oob/config');
  },

  oobPayloads(): Promise<OOBPayloadView[]> {
    return request<OOBPayloadView[]>('/api/oob/payloads');
  },

  generateOOBPayload(): Promise<OOBPayloadView> {
    return request<OOBPayloadView>('/api/oob/payloads', { method: 'POST' });
  },

  oobInteractions(token?: string): Promise<OOBInteractionView[]> {
    const query = token ? `?token=${encodeURIComponent(token)}` : '';
    return request<OOBInteractionView[]>(`/api/oob/interactions${query}`);
  },

  oobInteraction(id: string): Promise<OOBInteractionDetail> {
    return request<OOBInteractionDetail>(`/api/oob/interactions/${encodeURIComponent(id)}`);
  },

  clearOOBInteractions(): Promise<void> {
    return request<void>('/api/oob/interactions', { method: 'DELETE' });
  },

  // --- Send to Repeater / Intruder / Scanner / Crawler / Sequencer (from the proxy) ---

  sendToRepeater(id: string): Promise<TabView> {
    return request<TabView>(`/api/requests/${encodeURIComponent(id)}/send-to-repeater`, {
      method: 'POST',
    });
  },

  sendToIntruder(id: string): Promise<AttackView> {
    return request<AttackView>(`/api/requests/${encodeURIComponent(id)}/send-to-intruder`, {
      method: 'POST',
    });
  },

  sendToScanner(id: string): Promise<ScanTaskView> {
    return request<ScanTaskView>(`/api/requests/${encodeURIComponent(id)}/send-to-scanner`, {
      method: 'POST',
    });
  },

  sendToCrawler(id: string): Promise<CrawlTaskView> {
    return request<CrawlTaskView>(`/api/requests/${encodeURIComponent(id)}/send-to-crawler`, {
      method: 'POST',
    });
  },

  sendToSequencer(id: string): Promise<SeqTaskView> {
    return request<SeqTaskView>(`/api/requests/${encodeURIComponent(id)}/send-to-sequencer`, {
      method: 'POST',
    });
  },

  // --- AI (chat / explain / triage / payloads) ---

  aiConfig(): Promise<AIConfig> {
    return request<AIConfig>('/api/ai/config');
  },

  updateAIConfig(input: AIConfigInput): Promise<AIConfig> {
    // Only include apiKey when defined: omit to keep the current key, send ""
    // to clear, send a string to set it.
    const body: AIConfigInput = {
      provider: input.provider,
      model: input.model,
      baseUrl: input.baseUrl,
    };
    if (input.apiKey !== undefined) {
      body.apiKey = input.apiKey;
    }
    return request<AIConfig>('/api/ai/config', { method: 'PUT', body: JSON.stringify(body) });
  },

  aiConversations(): Promise<AIConversationView[]> {
    return request<AIConversationView[]>('/api/ai/conversations');
  },

  createAIConversation(input: AICreateInput): Promise<AIConversationDetail> {
    return request<AIConversationDetail>('/api/ai/conversations', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  },

  aiConversation(id: string): Promise<AIConversationDetail> {
    return request<AIConversationDetail>(`/api/ai/conversations/${encodeURIComponent(id)}`);
  },

  deleteAIConversation(id: string): Promise<void> {
    return request<void>(`/api/ai/conversations/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  clearAIConversations(): Promise<void> {
    return request<void>('/api/ai/conversations', { method: 'DELETE' });
  },
};

/** Callbacks for an in-progress streamed AI turn. */
export interface StreamCallbacks {
  onDelta(text: string): void;
  onDone(message: AIMessageView): void;
  onError(message: string): void;
}

/**
 * Stream an assistant reply over Server-Sent Events. Uses raw fetch (not the
 * request wrapper) so the response body can be read incrementally. When content
 * is non-empty it appends a user turn first; a null content just generates a
 * reply for the current messages. A non-OK response (e.g. AI disabled) is a JSON
 * error envelope and is rethrown as an ApiError.
 */
export async function streamAIMessage(
  id: string,
  content: string | null,
  cb: StreamCallbacks,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`/api/ai/conversations/${encodeURIComponent(id)}/messages`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify(content ? { content } : {}),
    signal,
  });

  if (!res.ok || !res.body) {
    let code = 'http_error';
    let message = res.statusText;
    try {
      const j: unknown = await res.json();
      if (isErrorEnvelope(j)) {
        code = j.error.code ?? code;
        message = j.error.message ?? message;
      }
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, code, message);
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    let nl: number;
    while ((nl = buf.indexOf('\n\n')) >= 0) {
      const frame = buf.slice(0, nl);
      buf = buf.slice(nl + 2);
      // Parse the "event:" / "data:" lines of one SSE frame.
      let ev = 'message';
      let data = '';
      for (const line of frame.split('\n')) {
        if (line.startsWith('event:')) ev = line.slice(6).trim();
        else if (line.startsWith('data:')) data += line.slice(5).replace(/^ /, '');
      }
      if (!data) continue;
      try {
        const parsed: unknown = JSON.parse(data);
        if (ev === 'delta' && isDelta(parsed)) {
          cb.onDelta(parsed.text);
        } else if (ev === 'done') {
          cb.onDone(parsed as AIMessageView);
        } else if (ev === 'error') {
          cb.onError(isStreamError(parsed) ? parsed.message : 'AI error');
        }
      } catch {
        /* ignore malformed frame */
      }
    }
  }
}

function isDelta(value: unknown): value is { text: string } {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as { text: unknown }).text === 'string'
  );
}

function isStreamError(value: unknown): value is { message: string } {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as { message: unknown }).message === 'string'
  );
}
