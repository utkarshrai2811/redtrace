import { Badge } from '../ui/Badge';

type Tone = 'zinc' | 'green' | 'amber' | 'red' | 'blue';

function methodTone(method: string): Tone {
  switch (method.toUpperCase()) {
    case 'GET':
      return 'green';
    case 'POST':
      return 'amber';
    case 'DELETE':
      return 'red';
    default:
      return 'zinc';
  }
}

export function MethodBadge({ method }: { method: string }) {
  return (
    <Badge tone={methodTone(method)} className="w-[52px] justify-center font-mono">
      {method.toUpperCase()}
    </Badge>
  );
}

// eslint-disable-next-line react-refresh/only-export-components -- small helper colocated with the badges it styles
export function statusTextClass(status: number): string {
  if (status >= 200 && status < 300) return 'text-emerald-400';
  if (status >= 300 && status < 400) return 'text-sky-400';
  if (status >= 400 && status < 500) return 'text-amber-400';
  if (status >= 500) return 'text-red-400';
  return 'text-zinc-500';
}
