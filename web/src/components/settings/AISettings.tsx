import { useEffect, useState } from 'react';
import { Card, CardBody, CardHeader, CardTitle } from '../ui/Card';
import { Button } from '../ui/Button';
import { Input } from '../ui/Input';
import { Select } from '../ui/Select';
import { useAIStore } from '../../store/aiStore';
import type { AIProvider } from '../../lib/types';

const MODEL_PLACEHOLDER: Record<AIProvider, string> = {
  anthropic: 'claude-sonnet-4-6',
  openai: 'gpt-4o-mini',
};

function Label({ children }: { children: string }) {
  return (
    <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">{children}</span>
  );
}

export function AISettings() {
  const config = useAIStore((s) => s.config);
  const fetchConfig = useAIStore((s) => s.fetchConfig);
  const saveConfig = useAIStore((s) => s.saveConfig);
  const savingConfig = useAIStore((s) => s.savingConfig);
  const configError = useAIStore((s) => s.configError);

  const [provider, setProvider] = useState<AIProvider>('anthropic');
  const [model, setModel] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [apiKey, setApiKey] = useState('');

  useEffect(() => {
    void fetchConfig();
  }, [fetchConfig]);

  // Seed the form from the loaded config once it arrives.
  useEffect(() => {
    if (config) {
      setProvider(config.provider);
      setModel(config.model);
      setBaseUrl(config.baseUrl);
    }
  }, [config]);

  const save = () => {
    void saveConfig({
      provider,
      model,
      baseUrl,
      // Only send apiKey when the user typed something; an empty field keeps
      // the current key (the placeholder makes this explicit).
      ...(apiKey ? { apiKey } : {}),
    });
    setApiKey('');
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>AI assistant</CardTitle>
      </CardHeader>
      <CardBody className="space-y-3">
        <p className="text-xs leading-relaxed text-zinc-400">
          Configure a provider and API key to enable AI-assisted analysis, triage, and payload
          suggestions. Keys are stored by the backend and never returned to the browser.
        </p>

        <div className="grid gap-3 sm:grid-cols-2">
          <label className="flex flex-col gap-1">
            <Label>Provider</Label>
            <Select
              value={provider}
              onChange={(e) => setProvider(e.target.value as AIProvider)}
              aria-label="AI provider"
            >
              <option value="anthropic">Anthropic</option>
              <option value="openai">OpenAI-compatible</option>
            </Select>
          </label>

          <label className="flex flex-col gap-1">
            <Label>Model</Label>
            <Input
              value={model}
              placeholder={MODEL_PLACEHOLDER[provider]}
              aria-label="AI model"
              onChange={(e) => setModel(e.target.value)}
              className="font-mono"
            />
          </label>
        </div>

        <label className="flex flex-col gap-1">
          <Label>Base URL</Label>
          <Input
            value={baseUrl}
            placeholder="https://api.anthropic.com"
            aria-label="AI base URL"
            onChange={(e) => setBaseUrl(e.target.value)}
            className="font-mono"
          />
          <span className="text-2xs text-zinc-600">
            OpenAI-compatible only — e.g. http://127.0.0.1:11434/v1 for a local model. Optional for
            Anthropic.
          </span>
        </label>

        <label className="flex flex-col gap-1">
          <Label>API key</Label>
          <Input
            type="password"
            value={apiKey}
            placeholder={config?.keySet ? '•••••••• (set — leave blank to keep)' : 'sk-…'}
            aria-label="AI API key"
            onChange={(e) => setApiKey(e.target.value)}
            className="font-mono"
          />
        </label>

        <div className="flex items-center gap-3">
          <Button variant="primary" size="sm" disabled={savingConfig} onClick={save}>
            {savingConfig ? 'Saving…' : 'Save'}
          </Button>
          {config &&
            (config.enabled ? (
              <span className="text-2xs text-emerald-400">
                Enabled ✓ — {config.provider} · {config.model || 'no model'}
              </span>
            ) : (
              <span className="text-2xs text-zinc-500">Not configured</span>
            ))}
        </div>

        {configError && <p className="text-2xs text-red-400">{configError}</p>}
      </CardBody>
    </Card>
  );
}
