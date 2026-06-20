import { Badge } from '../ui/Badge';
import type { Confidence, ScanOrigin, ScanStatus, Severity } from '../../lib/types';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

// Severity → color: high = red, medium = amber, low = yellow (amber on this
// palette), info = blue. There is no dedicated yellow tone, so low reuses amber
// but is distinguished by its label.
const SEVERITY_TONE: Record<Severity, Tone> = {
  high: 'red',
  medium: 'amber',
  low: 'amber',
  info: 'blue',
};

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <Badge tone={SEVERITY_TONE[severity]} className="w-[60px] justify-center uppercase">
      {severity}
    </Badge>
  );
}

const ORIGIN_TONE: Record<ScanOrigin, Tone> = {
  passive: 'zinc',
  active: 'blue',
};

export function OriginBadge({ origin }: { origin: ScanOrigin }) {
  return (
    <Badge tone={ORIGIN_TONE[origin]} className="capitalize">
      {origin}
    </Badge>
  );
}

export function ConfidenceBadge({ confidence }: { confidence: Confidence }) {
  return (
    <Badge tone="zinc" className="capitalize">
      {confidence}
    </Badge>
  );
}

const STATUS_TONE: Record<ScanStatus, Tone> = {
  pending: 'zinc',
  running: 'blue',
  completed: 'green',
  stopped: 'amber',
  error: 'red',
};

export function StatusBadge({ status }: { status: ScanStatus }) {
  return (
    <Badge tone={STATUS_TONE[status]} className="capitalize">
      {status}
    </Badge>
  );
}
