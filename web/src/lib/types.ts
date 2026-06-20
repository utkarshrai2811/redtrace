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

export interface IntruderFrame {
  type: 'intruder';
  data: IntruderUpdate;
}

export interface ScannerFrame {
  type: 'scanner';
  data: ScannerUpdate;
}

export interface CrawlFrame {
  type: 'crawl';
  data: CrawlUpdate;
}

export interface SequencerFrame {
  type: 'sequencer';
  data: SeqUpdate;
}

export type WsFrame =
  | TrafficFrame
  | InterceptFrame
  | IntruderFrame
  | ScannerFrame
  | CrawlFrame
  | SequencerFrame;

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

// --- Phase 2 ---

// Decoder
export type DecoderOp =
  | 'url_encode'
  | 'url_decode'
  | 'base64_encode'
  | 'base64_decode'
  | 'base64url_encode'
  | 'base64url_decode'
  | 'hex_encode'
  | 'hex_decode'
  | 'html_encode'
  | 'html_decode'
  | 'gzip_compress'
  | 'gzip_decompress'
  | 'jwt_decode';

export interface DecoderRunResponse {
  output: string;
}

export interface DecoderDetectResponse {
  op: string;
  ok: boolean;
}

// Comparer
export type ComparerMode = 'lines' | 'words';
export type DiffOp = 'equal' | 'insert' | 'delete';

export interface DiffSegment {
  op: DiffOp;
  text: string;
}

export interface ComparerResponse {
  segments: DiffSegment[];
}

// Repeater (raw fields are base64)
export interface TabView {
  id: string;
  name: string;
  scheme: string;
  host: string;
  followRedirects: boolean;
  httpVersion: string;
  createdAt: string;
  updatedAt: string;
  raw: string;
}

export interface HistoryView {
  id: string;
  statusCode: number;
  durationMs: number;
  createdAt: string;
  requestRaw: string;
  responseRaw: string;
}

export interface TabDetail {
  tab: TabView;
  history: HistoryView[];
}

/** Fields accepted when creating/updating a Repeater tab (raw is base64). */
export interface TabInput {
  name: string;
  scheme: string;
  host: string;
  followRedirects: boolean;
  httpVersion: string;
  raw: string;
}

export interface SendResponse {
  response: {
    raw: string;
    statusCode: number;
    durationMs: number;
  };
  history: HistoryView;
}

// Site Map
export interface SitemapPath {
  path: string;
  methods: string[];
  count: number;
  inScope: boolean;
  note?: string;
  tags?: string[];
}

export interface SitemapHost {
  host: string;
  count: number;
  paths: SitemapPath[];
}

export interface SitemapNoteInput {
  host: string;
  path: string;
  note: string;
  tags: string[];
}

// --- Intruder (template/raw fields are base64) ---

export type AttackType = 'sniper' | 'battering_ram' | 'pitchfork' | 'cluster_bomb';

export type ProcessorKind =
  | 'prefix'
  | 'suffix'
  | 'base64'
  | 'base64url'
  | 'url'
  | 'upper'
  | 'lower'
  | 'sha256'
  | 'md5';

export interface Processor {
  kind: ProcessorKind;
  value?: string;
}

export interface PayloadSet {
  payloads: string[];
  processors?: Processor[];
}

export interface AttackView {
  id: string;
  name: string;
  scheme: string;
  host: string;
  /** base64 of the raw request, with § markers */
  template: string;
  type: AttackType;
  payloadSets: PayloadSet[];
  followRedirects: boolean;
  httpVersion: string;
  concurrency: number;
  status: 'pending' | 'running' | 'completed' | 'stopped' | 'error';
  total: number;
  completed: number;
  createdAt: string;
  updatedAt: string;
}

export interface IntruderInput {
  name: string;
  scheme: string;
  host: string;
  template: string;
  type: AttackType;
  payloadSets: PayloadSet[];
  followRedirects: boolean;
  httpVersion: string;
  concurrency: number;
}

export interface IntruderResultView {
  id: string;
  index: number;
  payloads: string[];
  statusCode: number;
  length: number;
  durationMs: number;
  error?: string;
}

export interface IntruderAttackDetail {
  attack: AttackView;
  results: IntruderResultView[];
}

export interface IntruderResultDetail {
  result: IntruderResultView;
  requestRaw: string;
  responseRaw: string;
}

/** Progress frame pushed over the WebSocket. */
export interface IntruderUpdate {
  attackId: string;
  status: AttackView['status'];
  completed: number;
  total: number;
  /** present on per-request updates, absent on status transitions */
  result?: IntruderResultView;
}

// --- Scanner (request/response raw fields are base64) ---

export type Severity = 'info' | 'low' | 'medium' | 'high';
export type Confidence = 'tentative' | 'firm' | 'certain';
export type ScanOrigin = 'passive' | 'active';
export type ScanStatus = 'pending' | 'running' | 'completed' | 'stopped' | 'error';

export interface ScanIssueView {
  id: string;
  taskId?: string;
  type: string;
  name: string;
  severity: Severity;
  confidence: Confidence;
  scheme: string;
  host: string;
  port: number;
  path: string;
  method: string;
  param?: string;
  payload?: string;
  detail: string;
  evidence: string;
  remediation: string;
  origin: ScanOrigin;
  createdAt: string;
}

export interface ScanTaskView {
  id: string;
  name: string;
  scheme: string;
  host: string;
  /** base64 of the raw request */
  template: string;
  httpVersion: string;
  followRedirects: boolean;
  status: ScanStatus;
  total: number;
  completed: number;
  issues: number;
  createdAt: string;
  updatedAt: string;
}

export interface ScanTaskInput {
  name: string;
  scheme: string;
  host: string;
  template: string;
  httpVersion: string;
  followRedirects: boolean;
}

export interface ScanIssueDetail {
  issue: ScanIssueView;
  requestRaw: string;
  responseRaw: string;
}

export interface ScanTaskDetail {
  task: ScanTaskView;
  issues: ScanIssueView[];
}

/** A compact issue summary pushed live over the WebSocket (no createdAt). */
export interface ScanIssueSummary {
  id: string;
  type: string;
  name: string;
  severity: Severity;
  confidence: Confidence;
  scheme: string;
  host: string;
  port: number;
  path: string;
  method: string;
  param?: string;
  origin: ScanOrigin;
}

/** Progress / issue frame pushed over the WebSocket. */
export interface ScannerUpdate {
  kind: 'issue' | 'progress' | 'status';
  issue?: ScanIssueSummary;
  taskId?: string;
  status?: ScanStatus;
  completed: number;
  total: number;
  issues: number;
}

// --- Crawler (raw byte fields are base64 strings; CrawlTask/CrawlURL have no raw fields) ---

export type CrawlStatus = 'pending' | 'running' | 'completed' | 'stopped' | 'error';

export interface CrawlTaskView {
  id: string;
  name: string;
  seed: string;
  scheme: string;
  host: string;
  maxDepth: number;
  maxPages: number;
  status: CrawlStatus;
  pages: number;
  found: number;
  createdAt: string;
  updatedAt: string;
}

export interface CrawlURLView {
  id: string;
  taskId: string;
  url: string;
  method: string;
  statusCode: number;
  length: number;
  contentType: string;
  depth: number;
  createdAt: string;
}

export interface CrawlTaskInput {
  name: string;
  seed: string;
  maxDepth: number;
  maxPages: number;
}

export interface CrawlTaskDetail {
  task: CrawlTaskView;
  urls: CrawlURLView[];
}

export interface CrawlPageSummary {
  url: string;
  statusCode: number;
  contentType: string;
  depth: number;
}

export interface CrawlUpdate {
  kind: 'page' | 'progress' | 'status';
  taskId?: string;
  status?: CrawlStatus;
  pages: number;
  found: number;
  page?: CrawlPageSummary;
}

// --- Sequencer (template is base64 of the raw request) ---

export type SeqStatus = 'pending' | 'running' | 'completed' | 'stopped' | 'error';
export type SeqSource = 'cookie' | 'regex';
export type SeqQuality = 'poor' | 'reasonable' | 'good' | 'excellent';

export interface SeqTaskView {
  id: string;
  name: string;
  scheme: string;
  host: string;
  /** base64 of the raw request */
  template: string;
  httpVersion: string;
  source: SeqSource;
  selector: string;
  target: number;
  status: SeqStatus;
  collected: number;
  createdAt: string;
  updatedAt: string;
}

export interface SeqTaskInput {
  name: string;
  scheme: string;
  host: string;
  template: string;
  httpVersion: string;
  source: SeqSource;
  selector: string;
  target: number;
}

export interface SequencerReport {
  sampleCount: number;
  uniqueCount: number;
  minLength: number;
  maxLength: number;
  alphabetSize: number;
  positionEntropy: number[];
  effectiveBits: number;
  bitsPerChar: number;
  quality: SeqQuality;
  notes: string[];
}

export interface SeqTaskDetail {
  task: SeqTaskView;
  report: SequencerReport | null;
  tokens: string[];
}

export interface SeqUpdate {
  kind: 'progress' | 'status';
  taskId?: string;
  status?: SeqStatus;
  collected: number;
  target: number;
}
