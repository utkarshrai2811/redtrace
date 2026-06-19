import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, CardTitle } from '../ui/Card';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { Input } from '../ui/Input';
import { Toggle } from '../ui/Toggle';
import { Spinner } from '../ui/Spinner';
import { api, ApiError } from '../../lib/api';
import type { ScopeKind, ScopeMatcher, ScopeRule } from '../../lib/types';

const MATCHERS: { value: ScopeMatcher; label: string }[] = [
  { value: 'host', label: 'Host' },
  { value: 'host_regex', label: 'Host regex' },
  { value: 'path_prefix', label: 'Path prefix' },
  { value: 'cidr', label: 'CIDR' },
];

function newRule(): ScopeRule {
  return {
    id: crypto.randomUUID(),
    enabled: true,
    kind: 'include',
    matcher: 'host',
    value: '',
  };
}

export function ScopeEditor() {
  const [rules, setRules] = useState<ScopeRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        setRules(await api.getScope());
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'Failed to load scope');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const update = (id: string, patch: Partial<ScopeRule>) => {
    setRules((rs) => rs.map((r) => (r.id === id ? { ...r, ...patch } : r)));
    setSaved(false);
  };

  const remove = (id: string) => {
    setRules((rs) => rs.filter((r) => r.id !== id));
    setSaved(false);
  };

  const add = () => {
    setRules((rs) => [...rs, newRule()]);
    setSaved(false);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const updated = await api.putScope(rules);
      setRules(updated);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save scope');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <div>
          <CardTitle>Scope</CardTitle>
          <p className="mt-0.5 text-2xs text-zinc-500">
            Include/exclude rules decide which traffic counts as in-scope.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={add}>
          + Add rule
        </Button>
      </CardHeader>
      <CardBody className="space-y-2">
        {loading ? (
          <div className="flex items-center gap-2 py-4 text-xs text-zinc-500">
            <Spinner /> Loading…
          </div>
        ) : rules.length === 0 ? (
          <p className="py-3 text-xs text-zinc-600">No scope rules. Everything is in scope.</p>
        ) : (
          rules.map((rule) => (
            <div
              key={rule.id}
              className="flex flex-wrap items-center gap-2 rounded-md border border-zinc-800 bg-zinc-900/40 px-2.5 py-2"
            >
              <Toggle
                tone="green"
                checked={rule.enabled}
                onChange={(v) => update(rule.id, { enabled: v })}
              />
              <Select
                value={rule.kind}
                onChange={(e) => update(rule.id, { kind: e.target.value as ScopeKind })}
              >
                <option value="include">Include</option>
                <option value="exclude">Exclude</option>
              </Select>
              <Select
                value={rule.matcher}
                onChange={(e) => update(rule.id, { matcher: e.target.value as ScopeMatcher })}
              >
                {MATCHERS.map((m) => (
                  <option key={m.value} value={m.value}>
                    {m.label}
                  </option>
                ))}
              </Select>
              <Input
                value={rule.value}
                placeholder="value (e.g. example.com)"
                onChange={(e) => update(rule.id, { value: e.target.value })}
                className="min-w-[160px] flex-1 font-mono"
              />
              <Button
                size="icon"
                variant="ghost"
                title="Remove rule"
                onClick={() => remove(rule.id)}
                className="text-zinc-500 hover:text-red-400"
              >
                ✕
              </Button>
            </div>
          ))
        )}

        {error && <p className="text-xs text-red-400">{error}</p>}

        <div className="flex items-center gap-3 pt-1">
          <Button variant="primary" size="sm" disabled={saving || loading} onClick={() => void save()}>
            {saving ? 'Saving…' : 'Save scope'}
          </Button>
          {saved && <span className="text-2xs text-emerald-400">Saved</span>}
        </div>
      </CardBody>
    </Card>
  );
}
