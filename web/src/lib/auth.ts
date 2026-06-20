// Shared-token auth for the bundled UI. When RedTrace is started with --token
// (an intentional remote bind), every API call and the traffic WebSocket must
// carry the token. The operator supplies it once via `?token=<t>` in the URL;
// it is captured into sessionStorage and stripped from the address bar so it is
// not left in history or copied into shared links.

const STORAGE_KEY = 'redtrace.token';

function captureFromUrl(): string | null {
  const params = new URLSearchParams(window.location.search);
  const t = params.get('token');
  if (!t) return null;
  sessionStorage.setItem(STORAGE_KEY, t);
  params.delete('token');
  const qs = params.toString();
  const url = window.location.pathname + (qs ? `?${qs}` : '') + window.location.hash;
  window.history.replaceState(null, '', url);
  return t;
}

let token: string | null = captureFromUrl() ?? sessionStorage.getItem(STORAGE_KEY);

/** The active auth token, or null when none has been provided. */
export function getToken(): string | null {
  return token;
}

/** Persist a token (e.g. entered by the operator) for subsequent requests. */
export function setToken(value: string): void {
  token = value || null;
  if (value) sessionStorage.setItem(STORAGE_KEY, value);
  else sessionStorage.removeItem(STORAGE_KEY);
}

/** Auth headers to merge into every REST request (empty when no token is set). */
export function authHeaders(): Record<string, string> {
  return token ? { 'X-RedTrace-Token': token } : {};
}
