import { Badge } from '../ui/Badge';
import type { AIKind } from '../../lib/types';

const KIND_META: Record<AIKind, { label: string; tone: 'zinc' | 'blue' | 'amber' | 'green' }> = {
  chat: { label: 'Chat', tone: 'zinc' },
  explain: { label: 'Explain', tone: 'blue' },
  triage: { label: 'Triage', tone: 'amber' },
  payloads: { label: 'Payloads', tone: 'green' },
};

export function KindBadge({ kind }: { kind: AIKind }) {
  const meta = KIND_META[kind] ?? KIND_META.chat;
  return <Badge tone={meta.tone}>{meta.label}</Badge>;
}
