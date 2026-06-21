import { Badge } from '../ui/Badge';
import type { OOBProtocol } from '../../lib/types';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

// Protocol → color: dns = blue, http = green.
const PROTOCOL_TONE: Record<OOBProtocol, Tone> = {
  dns: 'blue',
  http: 'green',
};

export function ProtocolBadge({ protocol }: { protocol: OOBProtocol }) {
  return (
    <Badge tone={PROTOCOL_TONE[protocol]} className="w-[52px] justify-center uppercase">
      {protocol}
    </Badge>
  );
}
