// Small presentation helpers used across the traffic table and panels.

/** Format an ISO timestamp as HH:MM:SS (local time). */
export function formatTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '--:--:--';
  return d.toLocaleTimeString('en-GB', { hour12: false });
}

/** Format a byte count compactly (e.g. 0, 512, 1.2k, 3.4M). */
export function formatBytes(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0';
  if (n < 1024) return String(n);
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}k`;
  return `${(n / (1024 * 1024)).toFixed(1)}M`;
}

/** Format a duration in ms (e.g. 12, 340, 1.2s). */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '–';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

/** Compact integer with thousands separators. */
export function formatCount(n: number): string {
  return n.toLocaleString('en-US');
}
