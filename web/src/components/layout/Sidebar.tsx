import { NavLink } from 'react-router-dom';
import { Wordmark } from './Logo';
import { Badge } from '../ui/Badge';
import { cn } from '../../lib/cn';
import { useProxyStore } from '../../store/proxyStore';
import { useConnectionStore } from '../../store/connectionStore';

interface NavItem {
  label: string;
  to: string;
  icon: JSX.Element;
}

function Icon({ d }: { d: string }) {
  return (
    <svg
      width="15"
      height="15"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="shrink-0"
      aria-hidden="true"
    >
      <path d={d} />
    </svg>
  );
}

const ENABLED: NavItem[] = [
  { label: 'Dashboard', to: '/', icon: <Icon d="M3 13h8V3H3zM13 21h8V11h-8zM13 3v6h8V3zM3 21h8v-6H3z" /> },
  {
    label: 'Proxy',
    to: '/proxy',
    icon: <Icon d="M4 12h16M4 12l4-4M4 12l4 4M20 6v12" />,
  },
  {
    label: 'Site Map',
    to: '/sitemap',
    icon: <Icon d="M3 6l6-3 6 3 6-3v15l-6 3-6-3-6 3zM9 3v15M15 6v15" />,
  },
  {
    label: 'Repeater',
    to: '/repeater',
    icon: <Icon d="M17 1l4 4-4 4M3 11V9a4 4 0 014-4h14M7 23l-4-4 4-4M21 13v2a4 4 0 01-4 4H3" />,
  },
  {
    label: 'Decoder',
    to: '/decoder',
    icon: <Icon d="M7 8l-4 4 4 4M17 8l4 4-4 4M14 4l-4 16" />,
  },
  {
    label: 'Comparer',
    to: '/comparer',
    icon: <Icon d="M9 3H5a2 2 0 00-2 2v14a2 2 0 002 2h4M15 3h4a2 2 0 012 2v14a2 2 0 01-2 2h-4M12 3v18" />,
  },
  {
    label: 'Settings',
    to: '/settings',
    icon: (
      <Icon d="M12 15a3 3 0 100-6 3 3 0 000 6zM19.4 15a1.6 1.6 0 00.3 1.8l.1.1a2 2 0 11-2.8 2.8l-.1-.1a1.6 1.6 0 00-2.7 1.1V21a2 2 0 01-4 0v-.1A1.6 1.6 0 004 19.4l-.1.1a2 2 0 11-2.8-2.8l.1-.1A1.6 1.6 0 002.5 14H2a2 2 0 010-4h.1A1.6 1.6 0 003.6 8L3.5 8a2 2 0 112.8-2.8l.1.1A1.6 1.6 0 009 5.5V5a2 2 0 014 0v.1a1.6 1.6 0 002.7 1.1l.1-.1a2 2 0 112.8 2.8l-.1.1a1.6 1.6 0 00-.3 1.8" />
    ),
  },
];

const DISABLED = ['Intruder', 'Scanner', 'Sequencer', 'Crawler', 'OOB', 'AI'];

function ConnectionDot() {
  const status = useConnectionStore((s) => s.status);
  const tone =
    status === 'open' ? 'bg-emerald-500' : status === 'connecting' ? 'bg-amber-500' : 'bg-red-500';
  const label = status === 'open' ? 'Live' : status === 'connecting' ? 'Connecting' : 'Offline';
  return (
    <div className="flex items-center gap-2 px-4 py-3 text-2xs text-zinc-500">
      <span className={cn('h-1.5 w-1.5 rounded-full', tone, status === 'open' && 'animate-pulse')} />
      <span className="uppercase tracking-wide">{label}</span>
    </div>
  );
}

export function Sidebar() {
  const interceptCount = useProxyStore((s) => s.intercept.count);

  return (
    <aside className="flex w-[200px] shrink-0 flex-col border-r border-zinc-800 bg-panel">
      <Wordmark />
      <div className="mx-3 mb-2 border-t border-zinc-800/80" />

      <nav className="flex-1 space-y-0.5 px-2">
        {ENABLED.map((item) => (
          <NavLink
            key={item.label}
            to={item.to}
            end={item.to === '/'}
            className={({ isActive }) =>
              cn(
                'group flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors',
                isActive
                  ? 'bg-zinc-800/80 text-zinc-100'
                  : 'text-zinc-400 hover:bg-zinc-800/50 hover:text-zinc-200',
              )
            }
          >
            {({ isActive }) => (
              <>
                <span className={cn(isActive ? 'text-accent' : 'text-zinc-500')}>{item.icon}</span>
                <span className="flex-1">{item.label}</span>
                {item.label === 'Proxy' && interceptCount > 0 && (
                  <Badge tone="accent">{interceptCount}</Badge>
                )}
              </>
            )}
          </NavLink>
        ))}

        <div className="px-2.5 pb-1 pt-4 text-2xs font-medium uppercase tracking-wider text-zinc-600">
          Modules
        </div>

        {DISABLED.map((label) => (
          <div
            key={label}
            aria-disabled="true"
            title="Coming soon"
            className="flex cursor-not-allowed items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm text-zinc-600"
          >
            <span className="h-[15px] w-[15px] shrink-0 rounded-sm border border-zinc-700/60" />
            <span className="flex-1">{label}</span>
            <span className="rounded bg-zinc-800 px-1 py-0.5 text-[9px] uppercase tracking-wide text-zinc-500">
              soon
            </span>
          </div>
        ))}
      </nav>

      <div className="border-t border-zinc-800">
        <ConnectionDot />
      </div>
    </aside>
  );
}
