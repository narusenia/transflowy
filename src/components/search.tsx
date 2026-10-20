import { useEffect, useState } from "react";
import { Command, CommandDialog, CommandEmpty, CommandInput, CommandItem, CommandList } from "@/components/ui/command";
import { clock } from "@/lib/media";
import { useMessages } from "@/i18n";
import { Library } from "@/mygo";
import type { Hit } from "@/mygo";

/** ⌘K: search titles, transcripts and summaries of every recording. */
export function Search({ open, onOpenChange, onPick }: { open: boolean; onOpenChange(o: boolean): void; onPick(h: Hit): void }) {
  const t = useMessages();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<Hit[]>([]);

  useEffect(() => {
    if (!q.trim()) return setHits([]);
    const id = setTimeout(() => Library.search(q).then(setHits).catch(() => setHits([])), 150);
    return () => clearTimeout(id);
  }, [q]);
  useEffect(() => {
    if (!open) setQ("");
  }, [open]);

  const kind = { title: t.hitTitle(), transcript: t.hitTranscript(), summary: t.hitSummary() } as Record<string, string>;
  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title={t.searchTitle()} description={t.searchPlaceholder()}>
      {/* The match is done in Go: the list is not filtered again here. */}
      <Command shouldFilter={false}>
      <CommandInput value={q} onValueChange={setQ} placeholder={t.searchPlaceholder()} />
      <CommandList>
        {q.trim() && <CommandEmpty>{t.searchEmpty()}</CommandEmpty>}
        {hits.map((h, i) => (
          <CommandItem key={i} value={`${i}`} onSelect={() => { onPick(h); onOpenChange(false); }}>
            <div className="min-w-0">
              <div className="truncate text-sm">{h.snippet}</div>
              <div className="truncate text-xs text-muted-foreground">
                {kind[h.kind]}{h.template ? ` · ${h.template}` : ""}{h.kind === "transcript" ? ` · ${clock(h.time ?? 0)}` : ""} · {h.title || h.id}
              </div>
            </div>
          </CommandItem>
        ))}
      </CommandList>
      </Command>
    </CommandDialog>
  );
}
