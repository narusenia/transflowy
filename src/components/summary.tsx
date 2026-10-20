import { Fragment, type ReactNode } from "react";
import { stamp, stampSeconds } from "@/lib/media";

/** Inline Markdown: `code`, **bold** and [12:34] stamps that seek the player. */
function inline(text: string, onSeek: (s: number) => void): ReactNode[] {
  return text.split(/(`[^`]+`|\*\*[^*]+\*\*)/).map((part, i) => {
    if (part.startsWith("`") && part.length > 2) return <code key={i} className="rounded bg-muted px-1">{part.slice(1, -1)}</code>;
    const bold = part.startsWith("**") && part.length > 4;
    const body = bold ? part.slice(2, -2) : part;
    const out: ReactNode[] = [];
    let last = 0;
    for (const m of body.matchAll(stamp)) {
      out.push(body.slice(last, m.index));
      out.push(
        <button key={m.index} className="mx-0.5 cursor-pointer rounded bg-muted px-1 text-xs tabular-nums text-primary hover:underline" onClick={() => onSeek(stampSeconds(m[1]!, m[2]!))}>
          {m[0].slice(1, -1)}
        </button>,
      );
      last = m.index + m[0].length;
    }
    out.push(body.slice(last));
    return bold ? <strong key={i}>{out}</strong> : <Fragment key={i}>{out}</Fragment>;
  });
}

/**
 * A small Markdown renderer for what the summary templates produce: headings,
 * bullet and numbered lists, paragraphs.
 * ponytail: no tables, links or nested lists; swap in a Markdown library if templates need them.
 */
export function Markdown({ text, onSeek }: { text: string; onSeek(seconds: number): void }) {
  const blocks: ReactNode[] = [];
  let list: { ordered: boolean; items: string[] } | null = null;
  const flush = () => {
    if (!list) return;
    const Tag = list.ordered ? "ol" : "ul";
    blocks.push(
      <Tag key={blocks.length} className={`my-2 space-y-1 pl-5 ${list.ordered ? "list-decimal" : "list-disc"}`}>
        {list.items.map((it, i) => <li key={i}>{inline(it, onSeek)}</li>)}
      </Tag>,
    );
    list = null;
  };
  for (const line of text.split("\n")) {
    const h = /^(#{1,4})\s+(.*)/.exec(line);
    const li = /^\s*(?:([-*])|\d+\.)\s+(.*)/.exec(line);
    if (li) {
      const ordered = !li[1];
      if (list && list.ordered !== ordered) flush();
      (list ??= { ordered, items: [] }).items.push(li[2]!);
      continue;
    }
    flush();
    if (h) {
      const size = ["text-lg", "text-base", "text-sm", "text-sm"][h[1]!.length - 1];
      blocks.push(<h3 key={blocks.length} className={`mt-4 mb-1 font-semibold ${size}`}>{inline(h[2]!, onSeek)}</h3>);
    } else if (line.trim()) {
      blocks.push(<p key={blocks.length} className="my-2">{inline(line, onSeek)}</p>);
    }
  }
  flush();
  return <div className="text-sm leading-relaxed">{blocks}</div>;
}
