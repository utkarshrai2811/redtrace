import { useEffect } from 'react';
import { PageHeader } from '../components/layout/PageHeader';
import { Card, CardBody, CardHeader, CardTitle } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { ScopeEditor } from '../components/settings/ScopeEditor';
import { MatchReplaceEditor } from '../components/settings/MatchReplaceEditor';
import { AISettings } from '../components/settings/AISettings';
import { useSettingsStore } from '../store/settingsStore';

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-zinc-800/60 py-1.5 last:border-0">
      <span className="text-xs text-zinc-500">{label}</span>
      <span className="font-mono text-xs text-zinc-200">{value}</span>
    </div>
  );
}

export function SettingsPage() {
  const health = useSettingsStore((s) => s.health);
  const proxySettings = useSettingsStore((s) => s.proxySettings);
  const fetchAll = useSettingsStore((s) => s.fetchAll);

  useEffect(() => {
    void fetchAll();
  }, [fetchAll]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader title="Settings" subtitle="Proxy configuration, scope, and traffic rewriting" />

      <div className="min-h-0 flex-1 overflow-auto p-4">
        <div className="mx-auto grid max-w-5xl gap-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>Proxy</CardTitle>
              </CardHeader>
              <CardBody>
                <InfoRow label="Proxy listener" value={proxySettings?.proxyAddr ?? '—'} />
                <InfoRow
                  label="Upstream proxy"
                  value={proxySettings?.upstreamProxy || 'none'}
                />
                <InfoRow label="Backend version" value={health ? `v${health.version}` : '—'} />
                <InfoRow label="Backend status" value={health?.status ?? 'unknown'} />
              </CardBody>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>CA Certificate</CardTitle>
              </CardHeader>
              <CardBody className="space-y-3">
                <p className="text-xs leading-relaxed text-zinc-400">
                  To intercept HTTPS traffic, install RedTrace&apos;s CA certificate and trust it in
                  your OS / browser keychain.
                </p>
                <a href="/api/ca/cert" download>
                  <Button variant="primary" size="sm">
                    <svg
                      width="13"
                      height="13"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    >
                      <path d="M12 3v12M7 10l5 5 5-5M5 21h14" />
                    </svg>
                    Download CA certificate
                  </Button>
                </a>
                <p className="font-mono text-2xs text-zinc-600">
                  macOS: open the .pem in Keychain Access → set &ldquo;Always Trust&rdquo;.
                </p>
              </CardBody>
            </Card>
          </div>

          <AISettings />
          <ScopeEditor />
          <MatchReplaceEditor />
        </div>
      </div>
    </div>
  );
}
