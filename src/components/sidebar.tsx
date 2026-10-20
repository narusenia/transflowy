import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { clock } from "@/lib/media";
import { useMessages } from "@/i18n";
import { progressText } from "@/lib/status";
import type { Messages } from "@/locales";
import { Library, type Meta } from "@/mygo";
import { Mic, Square, Upload, X } from "lucide-react";

function statusBadge(t: Messages, m: Meta) {
  const text = progressText(t, m) ?? (m.status === "recording" ? t.statusRecording() : m.status === "failed" ? t.statusFailed() : null);
  if (!text) return null; // done: no badge
  return { text, variant: m.status === "failed" ? ("destructive" as const) : m.status === "recording" ? ("default" as const) : ("secondary" as const) };
}

/** "今日" / "昨日" / the date, for the list's group headings. */
function dayLabel(t: Messages, iso: string) {
  const d = new Date(iso);
  const day = (x: Date) => new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
  const diff = Math.round((day(new Date()) - day(d)) / 86400000);
  if (diff === 0) return t.dateToday();
  if (diff === 1) return t.dateYesterday();
  return d.toLocaleDateString(undefined, { year: "numeric", month: "long", day: "numeric", weekday: "short" });
}

export function Sidebar({ recordings, selected, onSelect, recording, elapsed, level, onRecord, onImport, canCapture, capture, onCapture }: {
  recordings: Meta[];
  selected: string;
  onSelect(id: string): void;
  recording: boolean;
  elapsed: number;
  level: number;
  onRecord(): void;
  onImport(): void;
  canCapture: boolean; // this build can record system audio
  capture: boolean;
  onCapture(on: boolean): void;
}) {
  const t = useMessages();
  const groups: { label: string; items: Meta[] }[] = [];
  for (const m of recordings) {
    const label = dayLabel(t, m.createdAt);
    const g = groups[groups.length - 1];
    if (g && g.label === label) g.items.push(m);
    else groups.push({ label, items: [m] });
  }
  return (
    <aside className="flex h-full w-72 shrink-0 flex-col border-r bg-sidebar text-sidebar-foreground">
      <div className="space-y-2 p-3">
        <Button className="w-full" variant={recording ? "destructive" : "default"} onClick={onRecord}>
          {recording ? <Square /> : <Mic />}
          {recording ? t.recordStop(clock(elapsed)) : t.recordStart()}
        </Button>
        {recording && (
          <div className="h-1 overflow-hidden rounded bg-muted">
            <div className="h-full bg-primary transition-[width] duration-150" style={{ width: `${Math.min(1, level * 2) * 100}%` }} />
          </div>
        )}
        {canCapture && (
          <label className="flex items-center justify-between gap-2 px-1 text-xs text-muted-foreground" title={t.captureSystemHint()}>
            {t.captureSystem()}
            <Switch size="sm" checked={capture} disabled={recording} onCheckedChange={onCapture} />
          </label>
        )}
        <Button className="w-full" variant="outline" size="sm" onClick={onImport}>
          <Upload /> {t.importButton()}
        </Button>
      </div>
      <nav className="min-h-0 flex-1 overflow-auto px-2 pb-3">
        {recordings.length === 0 && <p className="p-3 text-xs text-muted-foreground">{t.listEmpty()}</p>}
        {groups.map((g) => (
          <section key={g.label}>
            <h2 className="px-2 pt-3 pb-1 text-xs font-medium text-muted-foreground">{g.label}</h2>
            {g.items.map((m) => {
              const badge = statusBadge(t, m);
              return (
                <div key={m.id} className="group relative">
                <button
                  onClick={() => onSelect(m.id)}
                  onContextMenu={(e) => {
                    e.preventDefault();
                    onSelect(m.id);
                    void Library.showMenu(m.id);
                  }}
                  title={m.error}
                  className={`flex w-full cursor-pointer flex-col gap-0.5 rounded-md px-2 py-1.5 text-left text-sm hover:bg-sidebar-accent ${m.id === selected ? "bg-sidebar-accent" : ""}`}
                >
                  <span className="truncate font-medium">{m.title || new Date(m.createdAt).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" })}</span>
                  <span className="flex items-center gap-2 text-xs text-muted-foreground">
                    {m.duration > 0 && <span className="tabular-nums">{clock(m.duration)}</span>}
                    {badge && <Badge variant={badge.variant}>{badge.text}</Badge>}
                  </span>
                </button>
                {m.status !== "recording" && m.status !== "importing" && (
                  <button
                    aria-label={t.menuDelete()}
                    title={t.menuDelete()}
                    onClick={() => void Library.confirmDelete(m.id)}
                    className="absolute top-1.5 right-1.5 hidden size-5 cursor-pointer items-center justify-center rounded text-muted-foreground hover:bg-background hover:text-destructive group-hover:flex"
                  >
                    <X className="size-3.5" />
                  </button>
                )}
                </div>
              );
            })}
          </section>
        ))}
      </nav>
    </aside>
  );
}
