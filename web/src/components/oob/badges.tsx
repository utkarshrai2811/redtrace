import { Badge } from '../ui/Badge';
import type { OOBProtocol } from '../../lib/types';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

// Protocol → color: dns = blue, http = green, https = green (a slightly richer
// "secure" green). There is no dedicated teal tone on this palette, so https
// reuses green and is distinguished by its label.
const PROTOCOL_TONE: Record<OOBProtocol, Tone> = {
  dns: 'blue',
  http: 'green',
  https: 'green',
};

export function ProtocolBadge({ protocol }: { protocol: OOBProtocol }) {
  return (
    <Badge tone={PROTOCOL_TONE[protocol]} className="w-[52px] justify-center uppercase">
      {protocol}
    </Badge>
  );
}
