// Type definitions mirroring the RedTrace Go backend API contract.

export interface HealthResponse {
  status: string;
  version: string;
}

export interface ProxySettings {
  proxyAddr: string;
  upstreamProxy: string;
}

export interface RequestSummary {
  id: string;
  timestamp: string;
  source: string;
  method: string;
  scheme: string;
  host: string;
  port: number;
  path: string;
  url: string;
  inScope: boolean;
  statusCode: number;
  responseLength: number;
  mimeType: string;
  durationMs: number;
  hasResponse: boolean;
}

export interface RequestMeta {
  id: string;
  projectId?: string;
  timestamp: string;
  source: string;
  method: string;
  scheme: string;
  host: string;
  port: number;
  path: string;
  query?: string;
  url: string;
  httpVersion: string;
  contentType?: string;
  bodySize: number;
  inScope: boolean;
}

export interface ResponseMeta {
  id: string;
  requestId: string;
  timestamp: string;
  statusCode: number;
  reason?: string;
  httpVersion: string;
  mimeType?: string;
  bodySize: number;
  durationMs: number;
}

export interface RequestDetail {
  request: RequestMeta;
  response?: ResponseMeta;
  requestRaw: string;
  responseRaw?: string;
}

export interface RequestListResponse {
  data: RequestSummary[];
  page: number;
  perPage: number;
  total: number;
}

export type ScopeKind = 'include' | 'exclude';
export type ScopeMatcher = 'host' | 'host_regex' | 'path_prefix' | 'cidr';

export interface ScopeRule {
  id: string;
  enabled: boolean;
  kind: ScopeKind;
  matcher: ScopeMatcher;
  value: string;
}

export type MatchReplacePart =
  | 'request_method'
  | 'request_url'
  | 'request_header'
  | 'request_body'
  | 'response_header'
  | 'response_body';

export type MatchType = 'literal' | 'regex';

export interface MatchReplaceRule {
  id: string;
  enabled: boolean;
  name: string;
  part: MatchReplacePart;
  matchType: MatchType;
  match: string;
  replace: string;
  priority: number;
}

export type InterceptDirection = 'request' | 'response';

export interface HeldItem {
  id: string;
  direction: InterceptDirection;
  method?: string;
  url?: string;
  host?: string;
  statusCode?: number;
  raw: string;
}

export interface InterceptState {
  enabled: boolean;
  interceptResponses: boolean;
  queue: HeldItem[];
  count: number;
}

// WebSocket frames
export interface TrafficFrame {
  type: 'traffic';
  data: RequestSummary;
}

export interface InterceptFrame {
  type: 'intercept';
  data: InterceptState;
}

export type WsFrame = TrafficFrame | InterceptFrame;

// Error envelope returned by the backend on failure.
export interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
  };
}

export interface RequestFilters {
  method: string;
  host: string;
  status: string;
  mime: string;
  q: string;
  scope: boolean;
}
