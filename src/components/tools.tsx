import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { useMessages } from "@/i18n";
import { Exporter, Library, type Transcript } from "@/mygo";

const fail = (e: unknown) => toast.error(String((e as Error)?.message ?? e));
const select = "h-8 rounded-md border bg-transparent px-2 text-sm";

/** Replace a text in the whole transcript; offers to add the new word to the dictionary. */
export function ReplaceDialog({ id, open, onOpenChange, onChanged }: { id: string; open: boolean; onOpenChange(o: boolean): void; onChanged(): void }) {
  const t = useMessages();
  const [find, setFind] = useState("");
  const [to, setTo] = useState("");
  const run = () =>
    Library.replace(id, find, to)
      .then((n) => {
        onOpenChange(false);
        onChanged();
        toast(t.replaceDone(n), {
          duration: 8000,
          action: to ? { label: t.addDictionary(), onClick: () => Library.addToDictionary(to).then(() => toast(t.dictionaryAdded())).catch(fail) } : undefined,
        });
      })
      .catch(fail);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t.replaceTitle()}</DialogTitle>
        </DialogHeader>
        <label className="space-y-1 text-sm">
          {t.replaceFind()}
          <Input value={find} onChange={(e) => setFind(e.target.value)} autoFocus />
        </label>
        <label className="space-y-1 text-sm">
          {t.replaceWith()}
          <Input value={to} onChange={(e) => setTo(e.target.value)} onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && find && run()} />
        </label>
        <Button disabled={!find} onClick={run}>{t.replaceRun()}</Button>
      </DialogContent>
    </Dialog>
  );
}

/** Copy to the clipboard or save as .md / .txt / .srt. */
export function ExportDialog({ id, summaries, template, open, onOpenChange }: { id: string; summaries: string[]; template: string; open: boolean; onOpenChange(o: boolean): void }) {
  const t = useMessages();
  const [chosen, setInclude] = useState("both");
  const include = summaries.length ? chosen : "transcript"; // no summary yet: only the transcript
  const [stamps, setStamps] = useState(true);
  const [tpl, setTpl] = useState(template);
  const needsSummary = include !== "transcript";
  const name = summaries.includes(tpl) ? tpl : (summaries[0] ?? "");
  const blocked = needsSummary && !name;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t.exportButton()}</DialogTitle>
        </DialogHeader>
        <label className="flex items-center justify-between gap-3 text-sm">
          {t.exportInclude()}
          <select className={select} value={include} onChange={(e) => setInclude(e.target.value)}>
            <option value="summary" disabled={!summaries.length}>{t.includeSummary()}</option>
            <option value="both" disabled={!summaries.length}>{t.includeBoth()}</option>
            <option value="transcript">{t.includeTranscript()}</option>
          </select>
        </label>
        {needsSummary && summaries.length > 1 && (
          <label className="flex items-center justify-between gap-3 text-sm">
            {t.summaryTemplate()}
            <select className={select} value={name} onChange={(e) => setTpl(e.target.value)}>
              {summaries.map((s) => <option key={s}>{s}</option>)}
            </select>
          </label>
        )}
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={stamps} onChange={(e) => setStamps(e.target.checked)} />
          {t.exportStamps()}
        </label>
        <Button disabled={blocked} onClick={() => Exporter.text(id, name, include, stamps).then((x) => navigator.clipboard.writeText(x)).then(() => { toast(t.exportCopied()); onOpenChange(false); }).catch(fail)}>
          {t.exportCopy()}
        </Button>
        <div className="space-y-1">
          <div className="text-xs text-muted-foreground">{t.exportSaveAs()}</div>
          <div className="flex gap-2">
            {["md", "txt", "srt"].map((f) => (
              <Button key={f} variant="outline" disabled={blocked && f === "md"} onClick={() => Exporter.save(id, name, f, include, stamps).then((p) => { if (p) { toast(t.exportSaved(p)); onOpenChange(false); } }).catch(fail)}>
                .{f}
              </Button>
            ))}
          </div>
          <p className="text-xs text-muted-foreground">{t.exportNote()}</p>
        </div>
      </DialogContent>
    </Dialog>
  );
}

/** The speakers of a transcript with how many lines each has and a sample. */
export function speakersOf(t: Transcript) {
  const found = new Map<number, { count: number; sample: string }>();
  for (const s of t.segments) {
    if (s.speaker < 0) continue;
    const f = found.get(s.speaker) ?? { count: 0, sample: s.text };
    found.set(s.speaker, { count: f.count + 1, sample: f.sample });
  }
  return [...found].sort((a, b) => a[0] - b[0]).map(([n, v]) => ({ n, ...v }));
}

/** Give the speakers of a recording names. A blank name goes back to "話者N". */
export function SpeakersDialog({ id, transcript, open, onOpenChange, onChanged }: { id: string; transcript: Transcript; open: boolean; onOpenChange(o: boolean): void; onChanged(): void }) {
  const t = useMessages();
  const speakers = speakersOf(transcript);
  const [names, setNames] = useState<Record<string, string>>({});
  useEffect(() => {
    if (open) setNames({ ...transcript.speakers });
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps
  const save = () =>
    Library.setSpeakers(id, names)
      .then(() => {
        onOpenChange(false);
        onChanged();
      })
      .catch(fail);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{t.speakersTitle()}</DialogTitle>
        </DialogHeader>
        <p className="text-xs text-muted-foreground">{t.speakersHint()}</p>
        <div className="min-w-0 space-y-3">
          {speakers.map((s, i) => (
            <label key={s.n} className="block min-w-0 space-y-1 text-sm">
              <span className="flex items-baseline justify-between gap-2">
                <span className="font-medium">{t.speakerLabel(s.n + 1)}</span>
                <span className="text-xs text-muted-foreground">{t.speakerLines(s.count)}</span>
              </span>
              <Input
                autoFocus={i === 0}
                value={names[String(s.n)] ?? ""}
                placeholder={t.speakerLabel(s.n + 1)}
                onChange={(e) => setNames({ ...names, [String(s.n)]: e.target.value })}
                onKeyDown={(e) => e.key === "Enter" && !e.nativeEvent.isComposing && save()}
              />
              <span className="block truncate text-xs text-muted-foreground">{s.sample}</span>
            </label>
          ))}
        </div>
        <Button onClick={save}>{t.speakersSave()}</Button>
      </DialogContent>
    </Dialog>
  );
}
