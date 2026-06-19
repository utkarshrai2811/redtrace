import type {
  ComparerMode,
  ComparerResponse,
  DecoderDetectResponse,
  DecoderOp,
  DecoderRunResponse,
  HealthResponse,
  InterceptState,
  MatchReplaceRule,
  ProxySettings,
  RequestDetail,
  RequestFilters,
  RequestListResponse,
  ScopeRule,
  SendResponse,
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

  // --- Send to Repeater (from the proxy) ---

  sendToRepeater(id: string): Promise<TabView> {
    return request<TabView>(`/api/requests/${encodeURIComponent(id)}/send-to-repeater`, {
      method: 'POST',
    });
  },
};
