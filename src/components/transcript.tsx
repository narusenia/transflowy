import { useEffect, useMemo, useRef, useState } from "react";
import { Pencil, Undo2 } from "lucide-react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/icon-button";
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
export function Transcript({ transcript, time, onSeek, follow, setFollow, onEdit, onRevert, onSpeakers }: {
  transcript: T;
  time: number;
  onSeek(seconds: number): void;
  follow: boolean;
  setFollow(on: boolean): void;
  onEdit(index: number, start: number, text: string): void;
  onRevert(index: number, start: number): void;
  onSpeakers(): void; // the speaker label was clicked: name the speakers
}) {
  const t = useMessages();
  const scroller = useRef<HTMLDivElement>(null);
  const segs = transcript.segments;
  const active = useMemo(() => activeIndex(segs, time), [segs, time]);
  const v = useVirtualizer({ count: segs.length, getScrollElement: () => scroller.current, estimateSize: () => 52, overscan: 8 });
  const [editing, setEditing] = useState(-1);
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
  const speaker = (n: number) => (n < 0 ? "" : transcript.speakers?.[String(n)] || t.speakerLabel(n + 1));
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
              <div
                key={row.key}
                data-index={row.index}
                ref={v.measureElement}
                onClick={() => editing < 0 && onSeek(s.start)}
                onDoubleClick={() => setEditing(row.index)}
                className={`group absolute left-0 top-0 flex w-full cursor-pointer gap-3 px-4 py-2 text-left text-sm hover:bg-muted/60 ${row.index === active ? "bg-muted" : ""}`}
                style={{ transform: `translateY(${row.start}px)` }}
              >
                <span className="w-12 shrink-0 pt-0.5 text-xs tabular-nums text-muted-foreground">{clock(s.start)}</span>
                {editing === row.index ? (
                  <Editor
                    text={s.text}
                    onDone={(text) => {
                      setEditing(-1);
                      if (text !== null && text !== s.text) onEdit(row.index, s.start, text);
                    }}
                  />
                ) : (
                  <>
                    <span className="min-w-0 flex-1">
                      {speaker(s.speaker) && (
                        <button className="mr-2 cursor-pointer font-medium hover:underline" onClick={(e) => { e.stopPropagation(); onSpeakers(); }}>
                          {speaker(s.speaker)}
                        </button>
                      )}
                      {s.text}
                    </span>
                    <span className="flex shrink-0 gap-1 opacity-0 group-hover:opacity-100">
                      {s.text !== s.original && (
                        <IconButton size="icon-xs" variant="ghost" label={t.segmentRevert()} onClick={(e) => { e.stopPropagation(); onRevert(row.index, s.start); }}>
                          <Undo2 />
                        </IconButton>
                      )}
                      <IconButton size="icon-xs" variant="ghost" label={t.segmentEdit()} onClick={(e) => { e.stopPropagation(); setEditing(row.index); }}>
                        <Pencil />
                      </IconButton>
                    </span>
                    {s.text !== s.original && <span className="sr-only">{t.segmentEdited()}</span>}
                  </>
                )}
              </div>
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

/** One line being edited. Saves on blur or ⌘Enter, Esc drops the change. Enter alone is the IME's. */
function Editor({ text, onDone }: { text: string; onDone(text: string | null): void }) {
  const [v, setV] = useState(text);
  const done = useRef(false);
  const finish = (value: string | null) => {
    if (done.current) return;
    done.current = true;
    onDone(value);
  };
  return (
    <textarea
      autoFocus
      rows={Math.max(1, Math.ceil(text.length / 40))}
      className="min-w-0 flex-1 resize-none rounded border bg-background px-2 py-1"
      value={v}
      onClick={(e) => e.stopPropagation()}
      onChange={(e) => setV(e.target.value)}
      onBlur={() => finish(v)}
      onKeyDown={(e) => {
        if (e.key === "Escape") finish(null);
        else if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.nativeEvent.isComposing) finish(v);
      }}
    />
  );
}
