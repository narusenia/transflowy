import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Button } from "@/components/ui/button";
import { clock } from "@/lib/media";
import { useMessages } from "@/i18n";
import type { Transcript as T } from "@/mygo";

/** Index of the line playing at time t: the last one that started, -1 before the first. */
function activeIndex(segments: T["segments"], t: number) {
  let lo = 0, hi = segments.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (segments[mid]!.start <= t) lo = mid + 1;
    else hi = mid;
  }
  return lo - 1;
}

/** The lines of a transcript. Only the visible ones are drawn (TanStack Virtual). */
export function Transcript({ transcript, time, onSeek, follow, setFollow }: {
  transcript: T;
  time: number;
  onSeek(seconds: number): void;
  follow: boolean;
  setFollow(on: boolean): void;
}) {
  const t = useMessages();
  const scroller = useRef<HTMLDivElement>(null);
  const segs = transcript.segments;
  const active = useMemo(() => activeIndex(segs, time), [segs, time]);
  const v = useVirtualizer({ count: segs.length, getScrollElement: () => scroller.current, estimateSize: () => 52, overscan: 8 });
  const programmatic = useRef(false); // our own scroll must not turn follow off

  useEffect(() => {
    if (follow && active >= 0) {
      programmatic.current = true;
      v.scrollToIndex(active, { align: "center" });
      setTimeout(() => (programmatic.current = false), 100);
    }
  }, [active, follow]); // eslint-disable-line react-hooks/exhaustive-deps

  const [shown, setShown] = useState(false); // the button, once the user scrolled away
  useEffect(() => setShown(!follow), [follow]);

  if (!segs.length) return <p className="p-4 text-sm text-muted-foreground">{t.transcriptEmpty()}</p>;
  const speaker = (n: number) => (n < 0 ? "" : transcript.speakers?.[String(n)] || `${n + 1}`);
  return (
    <div className="relative h-full">
      <div
        ref={scroller}
        className="h-full overflow-auto"
        onWheel={() => !programmatic.current && setFollow(false)}
        onTouchMove={() => setFollow(false)}
      >
        <div style={{ height: v.getTotalSize() }} className="relative">
          {v.getVirtualItems().map((row) => {
            const s = segs[row.index]!;
            return (
              <button
                key={row.key}
                data-index={row.index}
                ref={v.measureElement}
                onClick={() => onSeek(s.start)}
                className={`absolute left-0 top-0 flex w-full cursor-pointer gap-3 px-4 py-2 text-left text-sm hover:bg-muted/60 ${row.index === active ? "bg-muted" : ""}`}
                style={{ transform: `translateY(${row.start}px)` }}
              >
                <span className="w-12 shrink-0 pt-0.5 text-xs tabular-nums text-muted-foreground">{clock(s.start)}</span>
                <span className="min-w-0">
                  {speaker(s.speaker) && <span className="mr-2 font-medium">{speaker(s.speaker)}</span>}
                  {s.text}
                </span>
              </button>
            );
          })}
        </div>
      </div>
      {shown && (
        <Button size="sm" className="absolute bottom-3 right-4 shadow" onClick={() => setFollow(true)}>
          {t.followPlayback()}
        </Button>
      )}
    </div>
  );
}
