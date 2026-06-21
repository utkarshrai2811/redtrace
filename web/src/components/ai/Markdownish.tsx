// A deliberately tiny, SAFE renderer for assistant text. It does NOT interpret
// HTML — React escapes every string node by default, so there is no injection
// surface and no dangerouslySetInnerHTML. The only structure it recognises is
// fenced code blocks (```), which it lays out as monospace <pre> panes; all
// other text is rendered verbatim with whitespace preserved.

interface MarkdownishProps {
  text: string;
}

export function Markdownish({ text }: MarkdownishProps) {
  // Splitting on ``` yields alternating prose (even index) and code (odd index)
  // segments. An unterminated fence leaves a trailing code segment, which is
  // fine — it still renders as code.
  const segments = text.split('```');

  return (
    <div className="space-y-2">
      {segments.map((segment, i) =>
        i % 2 === 0 ? (
          segment ? (
            <div key={i} className="whitespace-pre-wrap break-words text-zinc-200">
              {segment}
            </div>
          ) : null
        ) : (
          <pre
            key={i}
            className="raw-http overflow-x-auto rounded border border-zinc-800 bg-canvas px-3 py-2 text-zinc-300"
          >
            {/* Strip an optional leading language token (e.g. ```http). */}
            {segment.replace(/^[a-zA-Z0-9]*\n/, '')}
          </pre>
        ),
      )}
    </div>
  );
}
