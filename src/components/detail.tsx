import { useEffect, useRef, useState } from "react";
import { AudioLines, Download, Pencil, Replace, RotateCw, Users } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/icon-button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Player, type PlayerHandle } from "@/components/player";
import { Transcript } from "@/components/transcript";
import { Markdown } from "@/components/summary";
import { ExportDialog, ReplaceDialog, SpeakersDialog, speakersOf } from "@/components/tools";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { clock } from "@/lib/media";
import { useMessages } from "@/i18n";
import { Library, Summarizer, Transcriber, type Recording, type Template } from "@/mygo";

/** Where a search hit or a link should land in the recording that opens. */
export interface Jump {
  n: number; // changes on every request, so the same target can be asked twice
  time?: number;
  template?: string;
}

export interface Live {
  levels: number[];
  elapsed: number;
}

const fail = (e: unknown) => toast.error(String((e as Error)?.message ?? e));
const select = "h-8 rounded-md border bg-transparent px-2 text-sm";

function useWide() {
  const q = "(min-width: 1100px)";
  const [wide, setWide] = useState(() => matchMedia(q).matches);
  useEffect(() => {
    const m = matchMedia(q);
    const on = () => setWide(m.matches);
    m.addEventListener("change", on);
    return () => m.removeEventListener("change", on);
  }, []);
  return wide;
}

export function Detail({ rec, templates, jump, live, onChanged, renaming, onRenameHandled }: {
  rec: Recording;
  templates: Template[];
  jump: Jump | null;
  live: Live | null;
  onChanged(): void;
  renaming: boolean; // the context menu asked to rename this recording
  onRenameHandled(): void;
}) {
  const t = useMessages();
  const wide = useWide();
  const { meta } = rec;
  const recording = meta.status === "recording";
  const player = useRef<PlayerHandle>(null);
  const [time, setTime] = useState(0);
  const [follow, setFollow] = useState(true);
  const [tab, setTab] = useState("summary");
  const [name, setName] = useState(""); // the summary shown
  const [tpl, setTpl] = useState(""); // the template the button uses
  const [text, setText] = useState("");
  const [replacing, setReplacing] = useState(false);
  const [naming, setNaming] = useState(false);
  const [draft, setDraft] = useState<{ name: string; text: string } | null>(null); // a summary being written by hand
  const [exporting, setExporting] = useState(false);

  useEffect(() => {
    setTime(0);
    setFollow(true);
  }, [meta.id]);

  // Keep the shown summary valid as summaries appear.
  const wanted = useRef(""); // a summary just saved: show it once the reloaded list has it
  useEffect(() => {
    if (wanted.current) {
      if (rec.summaries.includes(wanted.current)) {
        setName(wanted.current);
        wanted.current = "";
      }
      return;
    }
    if (!rec.summaries.includes(name)) setName(rec.summaries[0] ?? "");
  }, [rec.summaries, name]);
  useEffect(() => {
    if (!tpl) setTpl(name || templates[0]?.name || "");
  }, [name, templates, tpl]);
  useEffect(() => {
    let dead = false;
    if (!name) return setText("");
    Library.summary(meta.id, name).then((s) => !dead && setText(s)).catch(() => !dead && setText(""));
    return () => { dead = true; };
  }, [meta.id, name, meta.status, rec.summaries]);

  useEffect(() => {
    if (!jump) return;
    if (jump.template && rec.summaries.includes(jump.template)) {
      setName(jump.template);
      setTab("summary");
    }
    if (jump.time !== undefined) {
      setTab("transcript");
      setFollow(true);
      setTime(jump.time);
      setTimeout(() => player.current?.seek(jump.time!, false), 0);
    }
  }, [jump?.n]); // eslint-disable-line react-hooks/exhaustive-deps

  const seek = (s: number) => {
    setFollow(true);
    if (!wide) setTab("transcript");
    player.current?.seek(s, true);
  };

  const summaryPane = (
    <div className="flex h-full flex-col">
      <div className="flex flex-wrap items-center gap-2 border-b px-4 py-2">
        {rec.summaries.length > 1 &&
          rec.summaries.map((s) => (
            <Button key={s} size="sm" variant={s === name ? "secondary" : "ghost"} onClick={() => setName(s)}>
              {s}
            </Button>
          ))}
        {rec.stale.includes(name) && (
          <button title={t.staleHint()} className="cursor-pointer" onClick={() => Summarizer.summarize(meta.id, name).then(onChanged).catch(fail)}>
            <Badge variant="destructive">{t.staleSummary()}</Badge>
          </button>
        )}
        <span className="flex-1" />
        {name && !draft && (
          <IconButton variant="ghost" label={t.summaryEdit()} onClick={() => setDraft({ name, text })}>
            <Pencil />
          </IconButton>
        )}
        <select aria-label={t.summaryTemplate()} className={select} value={tpl} onChange={(e) => setTpl(e.target.value)}>
          {templates.map((x) => (
            <option key={x.name} value={x.name} title={x.description}>
              {x.name}
            </option>
          ))}
        </select>
        <Button
          size="sm"
          variant="outline"
          title={rec.summaries.includes(tpl) ? t.summaryRegenerateKeepsEdits() : undefined}
          disabled={!!draft || !tpl || meta.status === "summarizing" || meta.status === "recording" || rec.transcript.segments.length === 0}
          onClick={() => Summarizer.summarize(meta.id, tpl).then(onChanged).catch(fail)}
        >
          {rec.summaries.includes(tpl) ? t.summaryRemake() : t.summaryMake()}
        </Button>
      </div>
      {draft ? (
        <SummaryEditor
          draft={draft}
          onCancel={() => setDraft(null)}
          onSave={(d) =>
            Library.saveSummary(meta.id, d.name, d.text)
              .then(() => {
                setDraft(null);
                wanted.current = d.text.trim() ? d.name : "";
                onChanged();
              })
              .catch(fail)
          }
        />
      ) : (
      <div className="min-h-0 flex-1 overflow-auto px-4 py-2">
        {meta.status === "summarizing" && <p className="text-sm text-muted-foreground">{meta.error || t.summaryWaiting()}</p>}
        {text ? (
          <Markdown text={text} onSeek={seek} />
        ) : (
          meta.status !== "summarizing" && (
            <div className="space-y-3">
              <p className="text-sm text-muted-foreground">{t.summaryEmpty()}</p>
              {meta.status === "done" && (
                <Button size="sm" variant="outline" onClick={() => setDraft({ name: t.manualSummaryName(), text: "" })}>
                  {t.summaryWrite()}
                </Button>
              )}
            </div>
          )
        )}
      </div>
      )}
    </div>
  );
  const transcriptPane = <Transcript
      transcript={rec.transcript}
      time={time}
      onSeek={seek}
      follow={follow}
      setFollow={setFollow}
      onEdit={(i, start, text) => Library.editSegment(meta.id, i, start, text).then(onChanged).catch(fail)}
      onRevert={(i, start) => Library.resetSegment(meta.id, i, start).then(onChanged).catch(fail)}
      onSpeakers={() => setNaming(true)}
    />;

  return (
    <div className="flex h-full flex-col">
      <header className="space-y-2 border-b px-4 py-3">
        <Title key={meta.id} id={meta.id} title={meta.title} renaming={renaming} onRenameHandled={onRenameHandled} fallback={new Date(meta.createdAt).toLocaleString()} onChanged={onChanged} />
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
          <p className="text-xs text-muted-foreground">
            {[new Date(meta.createdAt).toLocaleString(), meta.duration > 0 && clock(meta.duration), meta.engine].filter(Boolean).join(" · ")}
          </p>
          <div className="flex items-center gap-1.5">
          {!recording && rec.transcript.segments.length > 0 && (
            <>
              {speakersOf(rec.transcript).length > 0 && (
                <IconButton variant="outline" label={t.speakersButton()} onClick={() => setNaming(true)}>
                  <Users />
                </IconButton>
              )}
              <IconButton variant="outline" label={t.diarizeAgain()} disabled={meta.status !== "done"} onClick={() => Transcriber.diarize(meta.id).then(onChanged).catch(fail)}>
                <AudioLines />
              </IconButton>
              <IconButton variant="outline" label={t.replaceButton()} onClick={() => setReplacing(true)}>
                <Replace />
              </IconButton>
              <IconButton variant="outline" label={t.exportButton()} onClick={() => setExporting(true)}>
                <Download />
              </IconButton>
            </>
          )}
          {!recording && meta.status !== "importing" && (
            <>
              <span className="mx-1 h-4 w-px bg-border" />
              <Reprocess id={meta.id} language={meta.language ?? ""} detected={meta.detected} onChanged={onChanged} />
            </>
          )}
          </div>
        </div>
        {(meta.status === "failed" || (meta.status === "done" && meta.error)) && (
          <div className="flex flex-wrap items-center gap-2 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
            <span className="flex-1">{meta.error}</span>
            {meta.status === "failed" && (
              <>
                <Button size="sm" variant="outline" onClick={() => Transcriber.reprocess(meta.id, "").then(onChanged).catch(fail)}>{t.retry()}</Button>
                <Button size="sm" variant="outline" onClick={() => Transcriber.reprocess(meta.id, "local").then(onChanged).catch(fail)}>{t.processLocal()}</Button>
              </>
            )}
          </div>
        )}
      </header>
      <div className="min-h-0 flex-1">
        {wide ? (
          <div className="grid h-full grid-cols-2 divide-x">
            <div className="min-h-0">{summaryPane}</div>
            <div className="min-h-0">{transcriptPane}</div>
          </div>
        ) : (
          <Tabs value={tab} onValueChange={(v) => setTab(String(v))} className="h-full gap-0">
            <TabsList className="m-2 self-start">
              <TabsTrigger value="summary">{t.tabSummary()}</TabsTrigger>
              <TabsTrigger value="transcript">{t.tabTranscript()}</TabsTrigger>
            </TabsList>
            <TabsContent value="summary" className="min-h-0">{summaryPane}</TabsContent>
            <TabsContent value="transcript" className="min-h-0">{transcriptPane}</TabsContent>
          </Tabs>
        )}
      </div>
      <SpeakersDialog id={meta.id} transcript={rec.transcript} open={naming} onOpenChange={setNaming} onChanged={onChanged} />
      <ReplaceDialog id={meta.id} open={replacing} onOpenChange={setReplacing} onChanged={onChanged} />
      <ExportDialog key={meta.id} id={meta.id} summaries={rec.summaries} template={name} open={exporting} onOpenChange={setExporting} />
      {recording ? (
        <LiveWave live={live} />
      ) : meta.duration > 0 ? (
        <Player ref={player} id={meta.id} duration={meta.duration} onTime={setTime} />
      ) : null}
    </div>
  );
}

/** The language of this recording and "process again with this engine". */
function Reprocess({ id, language, detected, onChanged }: { id: string; language: string; detected?: string; onChanged(): void }) {
  const t = useMessages();
  const [kind, setKind] = useState("");
  const small = "h-7 rounded-md border bg-transparent px-2 text-xs";
  return (
    <>
      <select
        title={t.language()}
        aria-label={t.language()}
        className={small}
        value={language}
        onChange={(e) => Transcriber.setLanguage(id, e.target.value).then(onChanged).catch(fail)}
      >
        <option value="">{t.language()}: {t.languageAuto()}{detected ? ` (${detected})` : ""}</option>
        <option value="ja">{t.languageJa()}</option>
        <option value="en">{t.languageEn()}</option>
      </select>
      <select title={t.processWith()} aria-label={t.processWith()} className={small} value={kind} onChange={(e) => setKind(e.target.value)}>
        <option value="">{t.processWith()}: {t.languageAuto()}</option>
        <option value="local">{t.engineLocal()}</option>
        <option value="openrouter">{t.engineOpenRouter()}</option>
        <option value="openai-compat">{t.engineCompat()}</option>
      </select>
      <IconButton variant="outline" label={t.reprocess()} onClick={() => Transcriber.reprocess(id, kind).then(onChanged).catch(fail)}>
        <RotateCw />
      </IconButton>
    </>
  );
}

/** The input level over the last seconds, drawn from the recorder's level events. */
function LiveWave({ live }: { live: Live | null }) {
  const t = useMessages();
  return (
    <div className="flex items-center gap-3 border-t px-4 py-2">
      <span className="w-24 shrink-0 text-xs tabular-nums text-destructive">● {clock(live?.elapsed ?? 0)}</span>
      <div className="flex h-12 min-w-0 flex-1 items-center gap-px overflow-hidden">
        {(live?.levels ?? []).map((l, i) => (
          <div key={i} className="w-0.5 shrink-0 rounded bg-primary" style={{ height: `${Math.max(4, Math.min(1, l * 2) * 100)}%` }} />
        ))}
      </div>
      <span className="text-xs text-muted-foreground">{t.liveRecording()}</span>
    </div>
  );
}

/** The title: click to rename; an empty title goes back to the date. */
function Title({ id, title, fallback, onChanged, renaming, onRenameHandled }: { id: string; title: string; fallback: string; onChanged(): void; renaming: boolean; onRenameHandled(): void }) {
  const t = useMessages();
  const [editing, setEditing] = useState(false);
  const [v, setV] = useState(title);
  useEffect(() => {
    if (renaming) {
      setV(title);
      setEditing(true);
      onRenameHandled();
    }
  }, [renaming]); // eslint-disable-line react-hooks/exhaustive-deps
  const save = () => {
    setEditing(false);
    if (v.trim() !== title) Library.setTitle(id, v).then(onChanged).catch(fail);
  };
  if (!editing)
    return (
      <h1 className="cursor-text truncate text-lg font-semibold" title={t.segmentEdit()} onClick={() => { setV(title); setEditing(true); }}>
        {title || fallback}
      </h1>
    );
  return (
    <Input
      autoFocus
      className="h-8 text-lg font-semibold"
      value={v}
      placeholder={t.titlePlaceholder()}
      onChange={(e) => setV(e.target.value)}
      onBlur={save}
      onKeyDown={(e) => {
        if (e.key === "Escape") setEditing(false);
        else if (e.key === "Enter" && !e.nativeEvent.isComposing) save();
      }}
    />
  );
}

/** Write or fix a summary by hand: Markdown, with [12:34] stamps that stay links. */
function SummaryEditor({ draft, onSave, onCancel }: { draft: { name: string; text: string }; onSave(d: { name: string; text: string }): void; onCancel(): void }) {
  const t = useMessages();
  const [text, setText] = useState(draft.text);
  const save = () => onSave({ name: draft.name, text });
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-2 px-4 py-2">
      <textarea
        autoFocus
        className="min-h-0 flex-1 resize-none rounded-md border bg-background p-3 font-mono text-sm leading-relaxed"
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Escape") onCancel();
          else if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !e.nativeEvent.isComposing) save();
        }}
      />
      <p className="text-xs text-muted-foreground">{t.summaryEditHint()}</p>
      <div className="flex gap-2">
        <Button size="sm" onClick={save}>{t.summarySave()}</Button>
        <Button size="sm" variant="ghost" onClick={onCancel}>{t.cancel()}</Button>
      </div>
    </div>
  );
}
