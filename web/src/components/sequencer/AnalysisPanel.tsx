import { useState } from 'react';
import { QualityBadge } from './badges';
import type { SequencerReport } from '../../lib/types';

/** A single summary statistic, label above a mono value. */
function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded border border-zinc-800 bg-canvas px-2.5 py-1.5">
      <div className="text-2xs uppercase tracking-wider text-zinc-500">{label}</div>
      <div className="font-mono text-sm tabular-nums text-zinc-200">{value}</div>
    </div>
  );
}

/**
 * Per-position entropy as a row of vertical bars (no chart library — CSP blocks
 * external resources). Each bar's height is proportional to the position's
 * entropy relative to the theoretical maximum log2(alphabetSize).
 */
function EntropyBars({ entropy, alphabetSize }: { entropy: number[]; alphabetSize: number }) {
  const maxBits = alphabetSize > 1 ? Math.log2(alphabetSize) : 0;
  if (entropy.length === 0) {
    return <p className="text-xs text-zinc-400">No per-position entropy available.</p>;
  }
  return (
    <div className="flex h-24 items-end gap-px overflow-x-auto rounded border border-zinc-800 bg-canvas p-2">
      {entropy.map((bits, i) => {
        const ratio = maxBits > 0 ? Math.min(1, Math.max(0, bits / maxBits)) : 0;
        const pct = `${(ratio * 100).toFixed(1)}%`;
        return (
          <div
            key={i}
            className="flex h-full min-w-[3px] flex-1 items-end"
            title={`Position ${i}: ${bits.toFixed(2)} bits`}
          >
            <div
              className="w-full rounded-sm bg-accent/70"
              style={{ height: pct }}
              aria-hidden="true"
            />
          </div>
        );
      })}
    </div>
  );
}

export function AnalysisPanel({
  report,
  tokens,
}: {
  report: SequencerReport;
  tokens: string[];
}) {
  const [showTokens, setShowTokens] = useState(false);
  const sample = tokens.slice(0, 20);

  return (
    <div className="space-y-3 p-3">
      <div className="flex items-center gap-2">
        <span className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Analysis
        </span>
        <QualityBadge quality={report.quality} />
      </div>

      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        <Stat label="Samples" value={report.sampleCount.toLocaleString('en-US')} />
        <Stat label="Unique" value={report.uniqueCount.toLocaleString('en-US')} />
        <Stat label="Effective bits" value={`${report.effectiveBits.toFixed(1)} bits`} />
        <Stat label="Bits / char" value={report.bitsPerChar.toFixed(2)} />
        <Stat label="Alphabet size" value={report.alphabetSize.toLocaleString('en-US')} />
        <Stat label="Min length" value={String(report.minLength)} />
        <Stat label="Max length" value={String(report.maxLength)} />
      </div>

      {report.notes.length > 0 && (
        <ul className="list-disc space-y-1 pl-5 text-xs text-zinc-400">
          {report.notes.map((note, i) => (
            <li key={i}>{note}</li>
          ))}
        </ul>
      )}

      <div className="space-y-1.5">
        <div className="text-2xs font-semibold uppercase tracking-wider text-zinc-500">
          Per-position entropy
        </div>
        <EntropyBars entropy={report.positionEntropy} alphabetSize={report.alphabetSize} />
      </div>

      {tokens.length > 0 && (
        <div className="space-y-1.5">
          <button
            type="button"
            aria-expanded={showTokens}
            onClick={() => setShowTokens((v) => !v)}
            className="text-2xs font-semibold uppercase tracking-wider text-zinc-500 transition-colors hover:text-zinc-300 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent/60"
          >
            {showTokens ? '▾' : '▸'} Collected tokens (first {sample.length})
          </button>
          {showTokens && (
            <pre className="max-h-48 overflow-auto rounded border border-zinc-800 bg-canvas px-3 py-2 font-mono text-2xs text-zinc-300">
              {sample.join('\n')}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}
