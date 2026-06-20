import { Badge } from '../ui/Badge';
import type { CrawlStatus } from '../../lib/types';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

const STATUS_TONE: Record<CrawlStatus, Tone> = {
  pending: 'zinc',
  running: 'blue',
  completed: 'green',
  stopped: 'amber',
  error: 'red',
};

export function StatusBadge({ status }: { status: CrawlStatus }) {
  return (
    <Badge tone={STATUS_TONE[status]} className="capitalize">
      {status}
    </Badge>
  );
}
