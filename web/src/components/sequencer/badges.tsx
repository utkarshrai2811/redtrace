import { Badge } from '../ui/Badge';
import type { SeqQuality, SeqStatus } from '../../lib/types';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

const STATUS_TONE: Record<SeqStatus, Tone> = {
  pending: 'zinc',
  running: 'blue',
  completed: 'green',
  stopped: 'amber',
  error: 'red',
};

export function StatusBadge({ status }: { status: SeqStatus }) {
  return (
    <Badge tone={STATUS_TONE[status]} className="capitalize">
      {status}
    </Badge>
  );
}

// Quality → color: poor = red, reasonable = amber, good = blue, excellent = green.
const QUALITY_TONE: Record<SeqQuality, Tone> = {
  poor: 'red',
  reasonable: 'amber',
  good: 'blue',
  excellent: 'green',
};

export function QualityBadge({ quality }: { quality: SeqQuality }) {
  return (
    <Badge tone={QUALITY_TONE[quality]} className="capitalize">
      {quality}
    </Badge>
  );
}
