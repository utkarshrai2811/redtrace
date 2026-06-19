import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { PageHeader } from '../components/layout/PageHeader';
import { Card, CardBody, CardHeader, CardTitle } from '../components/ui/Card';
import { MethodBadge, statusTextClass } from '../components/proxy/badges';
import { api } from '../lib/api';
import { useSettingsStore } from '../store/settingsStore';
import { useProxyStore } from '../store/proxyStore';
import { formatCount, formatTime } from '../lib/format';
import type { RequestSummary } from '../lib/types';

interface Stats {
  total: number;
  inScope: number;
  hosts: number;
}

const EMPTY_STATS: Stats = { total: 0, inScope: 0, hosts: 0 };

function StatCard({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent?: 'accent' | 'green' | 'amber';
}) {
  const color =
    accent === 'accent'
      ? 'text-accent'
      : accent === 'green'
        ? 'text-emerald-400'
        : accent === 'amber'
          ? 'text-amber-400'
          : 'text-zinc-100';
  return (
    <Card>
      <CardBody>
        <div className="text-2xs font-medium uppercase tracking-wider text-zinc-500">{label}</div>
        <div className={`mt-1 font-mono text-2xl font-semibold tabular-nums ${color}`}>{value}</div>
      </CardBody>
    </Card>
  );
}

export function DashboardPage() {
  const health = useSettingsStore((s) => s.health);
  const fetchAll = useSettingsStore((s) => s.fetchAll);
  // Live total from the proxy store keeps the headline count fresh via WS.
  const liveTotal = useProxyStore((s) => s.total);

  const [stats, setStats] = useState<Stats>(EMPTY_STATS);
  const [recent, setRecent] = useState<RequestSummary[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    void fetchAll();
    let cancelled = false;
    void (async () => {
      const baseFilters = {
        method: 'ANY',
        host: 'ANY',
        status: 'ANY',
        mime: '',
        q: '',
        scope: false,
      };
      const [allResult, scopeResult, hostsResult] = await Promise.allSettled([
        api.listRequests(baseFilters, 1, 10),
        api.listRequests({ ...baseFilters, scope: true }, 1, 1),
        api.hosts(),
      ]);
      if (cancelled) return;

      const total = allResult.status === 'fulfilled' ? allResult.value.total : 0;
      const inScope = scopeResult.status === 'fulfilled' ? scopeResult.value.total : 0;
      const hosts = hostsResult.status === 'fulfilled' ? hostsResult.value.length : 0;
      setStats({ total, inScope, hosts });
      if (allResult.status === 'fulfilled') setRecent(allResult.value.data);
      setLoaded(true);
    })();
    return () => {
      cancelled = true;
    };
  }, [fetchAll]);

  const total = Math.max(stats.total, liveTotal);
  const outOfScope = Math.max(0, total - stats.inScope);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader
        title="Dashboard"
        subtitle="Overview of captured traffic and scope"
        actions={
          health && (
            <span className="font-mono text-2xs text-zinc-500">backend v{health.version}</span>
          )
        }
      />

      <div className="min-h-0 flex-1 overflow-auto p-4">
        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          <StatCard label="Requests captured" value={formatCount(total)} accent="accent" />
          <StatCard label="Distinct hosts" value={formatCount(stats.hosts)} />
          <StatCard label="In scope" value={formatCount(stats.inScope)} accent="green" />
          <StatCard label="Out of scope" value={formatCount(outOfScope)} accent="amber" />
        </div>

        <div className="mt-4">
          <Card>
            <CardHeader className="flex items-center justify-between">
              <CardTitle>Recent activity</CardTitle>
              <Link to="/proxy" className="text-2xs text-accent-fg hover:text-accent">
                View all →
              </Link>
            </CardHeader>
            <CardBody className="p-0">
              {recent.length === 0 ? (
                <div className="px-4 py-8 text-center text-xs text-zinc-600">
                  {loaded ? 'No traffic captured yet.' : 'Loading…'}
                </div>
              ) : (
                <table className="w-full border-collapse font-mono text-xs">
                  <tbody>
                    {recent.map((row) => (
                      <tr key={row.id} className="border-b border-zinc-800/50 last:border-0">
                        <td className="px-3 py-1.5">
                          <MethodBadge method={row.method} />
                        </td>
                        <td className="max-w-0 truncate px-2 py-1.5 text-zinc-300" title={row.url}>
                          <span className="text-zinc-500">{row.host}</span>
                          {row.path || '/'}
                        </td>
                        <td
                          className={`px-2 py-1.5 text-right tabular-nums ${statusTextClass(row.statusCode)}`}
                        >
                          {row.hasResponse && row.statusCode > 0 ? row.statusCode : '–'}
                        </td>
                        <td className="px-3 py-1.5 text-right text-zinc-500 tabular-nums">
                          {formatTime(row.timestamp)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </CardBody>
          </Card>
        </div>
      </div>
    </div>
  );
}
