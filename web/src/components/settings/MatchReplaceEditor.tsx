import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, CardTitle } from '../ui/Card';
import { Button } from '../ui/Button';
import { Select } from '../ui/Select';
import { Input } from '../ui/Input';
import { Toggle } from '../ui/Toggle';
import { Spinner } from '../ui/Spinner';
import { api, ApiError } from '../../lib/api';
import type { MatchReplacePart, MatchReplaceRule, MatchType } from '../../lib/types';

const PARTS: { value: MatchReplacePart; label: string }[] = [
  { value: 'request_method', label: 'Request method' },
  { value: 'request_url', label: 'Request URL' },
  { value: 'request_header', label: 'Request header' },
  { value: 'request_body', label: 'Request body' },
  { value: 'response_header', label: 'Response header' },
  { value: 'response_body', label: 'Response body' },
];

function newRule(priority: number): MatchReplaceRule {
  return {
    id: crypto.randomUUID(),
    enabled: true,
    name: '',
    part: 'request_header',
    matchType: 'literal',
    match: '',
    replace: '',
    priority,
  };
}

export function MatchReplaceEditor() {
  const [rules, setRules] = useState<MatchReplaceRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        setRules(await api.getRules());
      } catch (err) {
        setError(err instanceof ApiError ? err.message : 'Failed to load rules');
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const update = (id: string, patch: Partial<MatchReplaceRule>) => {
    setRules((rs) => rs.map((r) => (r.id === id ? { ...r, ...patch } : r)));
    setSaved(false);
  };

  const remove = (id: string) => {
    setRules((rs) => rs.filter((r) => r.id !== id));
    setSaved(false);
  };

  const add = () => {
    setRules((rs) => [...rs, newRule(rs.length + 1)]);
    setSaved(false);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    setSaved(false);
    try {
      const updated = await api.putRules(rules);
      setRules(updated);
      setSaved(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Failed to save rules');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card>
      <CardHeader className="flex items-center justify-between">
        <div>
          <CardTitle>Match &amp; Replace</CardTitle>
          <p className="mt-0.5 text-2xs text-zinc-500">
            Rewrite parts of requests/responses as they pass through the proxy.
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
          <p className="py-3 text-xs text-zinc-600">No match &amp; replace rules defined.</p>
        ) : (
          rules.map((rule) => (
            <div
              key={rule.id}
              className="space-y-2 rounded-md border border-zinc-800 bg-zinc-900/40 px-2.5 py-2"
            >
              <div className="flex flex-wrap items-center gap-2">
                <Toggle
                  tone="green"
                  ariaLabel={rule.enabled ? 'Disable rule' : 'Enable rule'}
                  checked={rule.enabled}
                  onChange={(v) => update(rule.id, { enabled: v })}
                />
                <Input
                  value={rule.name}
                  placeholder="Rule name"
                  onChange={(e) => update(rule.id, { name: e.target.value })}
                  className="w-44"
                />
                <Select
                  value={rule.part}
                  onChange={(e) => update(rule.id, { part: e.target.value as MatchReplacePart })}
                >
                  {PARTS.map((p) => (
                    <option key={p.value} value={p.value}>
                      {p.label}
                    </option>
                  ))}
                </Select>
                <Select
                  value={rule.matchType}
                  onChange={(e) => update(rule.id, { matchType: e.target.value as MatchType })}
                >
                  <option value="literal">Literal</option>
                  <option value="regex">Regex</option>
                </Select>
                <label className="flex items-center gap-1 text-2xs text-zinc-500">
                  prio
                  <Input
                    type="number"
                    value={rule.priority}
                    onChange={(e) =>
                      update(rule.id, { priority: Number(e.target.value) || 0 })
                    }
                    className="w-14"
                  />
                </label>
                <Button
                  size="icon"
                  variant="ghost"
                  title="Remove rule"
                  onClick={() => remove(rule.id)}
                  className="ml-auto text-zinc-500 hover:text-red-400"
                >
                  ✕
                </Button>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <Input
                  value={rule.match}
                  placeholder="match"
                  onChange={(e) => update(rule.id, { match: e.target.value })}
                  className="min-w-[160px] flex-1 font-mono"
                />
                <span className="text-zinc-600">→</span>
                <Input
                  value={rule.replace}
                  placeholder="replace"
                  onChange={(e) => update(rule.id, { replace: e.target.value })}
                  className="min-w-[160px] flex-1 font-mono"
                />
              </div>
            </div>
          ))
        )}

        {error && <p className="text-xs text-red-400">{error}</p>}

        <div className="flex items-center gap-3 pt-1">
          <Button variant="primary" size="sm" disabled={saving || loading} onClick={() => void save()}>
            {saving ? 'Saving…' : 'Save rules'}
          </Button>
          {saved && <span className="text-2xs text-emerald-400">Saved</span>}
        </div>
      </CardBody>
    </Card>
  );
}
