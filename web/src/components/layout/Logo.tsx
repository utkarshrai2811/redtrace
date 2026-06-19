export function Crosshair({ size = 18 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className="shrink-0"
    >
      <circle cx="12" cy="12" r="9" stroke="#ef4444" strokeWidth="2" />
      <circle cx="12" cy="12" r="2.2" fill="#ef4444" />
      <path
        d="M12 1.5v3.5M12 19v3.5M1.5 12h3.5M19 12h3.5"
        stroke="#ef4444"
        strokeWidth="2"
        strokeLinecap="round"
      />
    </svg>
  );
}

export function Wordmark() {
  return (
    <div className="flex items-center gap-2 px-3 py-3.5">
      <Crosshair size={20} />
      <span className="text-[15px] font-semibold tracking-tight text-zinc-100">
        Red<span className="text-accent">Trace</span>
      </span>
    </div>
  );
}
