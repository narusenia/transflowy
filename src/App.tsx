import { useEffect, useRef, useState } from "react";
import { toast, Toaster } from "sonner";
import { Button } from "@/components/ui/button";
import { useMessages } from "@/i18n";
import type { Messages } from "@/locales";
import { events, Importer, Library, Models, Recorder, Settings, Transcriber, type Meta, type ModelStatus, type Recording, type RecordingState } from "@/mygo";

const idle: RecordingState = { recording: false, id: "", elapsed: 0, level: 0 };

const clock = (s: number) =>
  `${Math.floor(s / 60)}:${String(Math.floor(s) % 60).padStart(2, "0")}`;

export function App() {
  const [dir, setDir] = useState("");
  const [recordings, setRecordings] = useState<Meta[]>([]);
  const [state, setState] = useState(idle);
  const wasRecording = useRef(false);
  const t = useMessages();
  const tRef = useRef(t); // the event handler below outlives a render
  tRef.current = t;

  const [selected, setSelected] = useState("");
  const [detail, setDetail] = useState<Recording | null>(null);
  const [model, setModel] = useState<ModelStatus>({ present: true, downloading: false });
  const [percent, setPercent] = useState(0);
  const selectedRef = useRef("");
  selectedRef.current = selected;

  const loadDetail = (id: string) => (id ? Library.get(id).then(setDetail).catch(() => setDetail(null)) : setDetail(null));
  const reload = () => {
    Library.list().then(setRecordings);
    loadDetail(selectedRef.current);
  };

  useEffect(() => {
    Settings.get().then((s) => setDir(s.libraryDir));
    Models.status().then(setModel);
    Recorder.state().then((s) => {
      wasRecording.current = s.recording;
      setState(s);
    });
    reload();
    // The menu bar and the shortcut also start and stop recordings.
    const offs = [
      events.recordingChanged.on((s) => {
        if (wasRecording.current && !s.recording) onStopped(s.id, tRef.current);
        wasRecording.current = s.recording;
        setState(s);
        if (!s.recording) reload();
      }),
      events.libraryChanged.on(reload),
      events.importFailed.on((p) => toast.error(p.message)),
      events.modelChanged.on((p) => {
        if (p.state === "downloading") setPercent(p.total ? Math.floor((p.done / p.total) * 100) : 0);
        if (p.state === "failed") toast.error(`${tRef.current.modelFailed()}: ${p.error ?? ""}`);
        Models.status().then(setModel);
      }),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  return (
    <main className="flex h-screen flex-col items-center justify-center gap-4 text-sm">
      <Toaster position="bottom-center" />
      <Button
        size="lg"
        variant={state.recording ? "destructive" : "default"}
        onClick={() => (state.recording ? Recorder.stop() : Recorder.start()).catch((e) => toast.error(String(e.message ?? e)))}
      >
        {state.recording ? t.recordStop(clock(state.elapsed)) : t.recordStart()}
      </Button>
      <div className="h-1.5 w-48 overflow-hidden rounded bg-muted">
        <div className="h-full bg-primary transition-[width] duration-150" style={{ width: `${Math.min(1, state.level * 2) * 100}%` }} />
      </div>
      <Button variant="outline" onClick={() => Importer.importDialog().catch((e) => toast.error(String(e.message ?? e)))}>
        {t.importButton()}
      </Button>
      {!model.present && (
        <Button variant="outline" disabled={model.downloading} onClick={() => Models.download()}>
          {model.downloading ? t.modelDownloading(percent) : t.modelDownload()}
        </Button>
      )}
      <p className="text-muted-foreground">
        {dir} · {t.recordingCount(recordings.length)}
      </p>
      {/* ponytail: placeholder list, replaced by the real list in the screen phase */}
      <ul className="max-h-48 w-96 overflow-auto text-xs text-muted-foreground">
        {recordings.map((r) => (
          <li key={r.id} className="flex cursor-pointer justify-between gap-2" onClick={() => { setSelected(r.id); loadDetail(r.id); }}>
            <span className={`truncate ${r.id === selected ? "font-semibold text-foreground" : ""}`}>{r.title || r.id}</span>
            <span title={r.error}>
              {statusLabel(t, r.status)}
              {r.status === "processing" && r.duration > 0 ? ` ${Math.floor(((r.processedUntil ?? 0) / r.duration) * 100)}%` : ""} · {clock(r.duration)}
            </span>
          </li>
        ))}
      </ul>
      {detail && (
        <div className="w-96 text-xs">
          {detail.meta.error && <p className="text-destructive">{detail.meta.error}</p>}
          <Button size="sm" variant="ghost" onClick={() => Transcriber.reprocess(detail.meta.id, "").catch((e) => toast.error(String(e.message ?? e)))}>
            {t.reprocess()}
          </Button>
          <ol className="max-h-40 overflow-auto">
            {detail.transcript.segments.map((g, i) => (
              <li key={i}>
                <span className="text-muted-foreground">{clock(g.start)}</span> {g.text}
              </li>
            ))}
          </ol>
        </div>
      )}
    </main>
  );
}

function statusLabel(t: Messages, status: string) {
  const labels: Record<string, () => string> = {
    recording: t.statusRecording, importing: t.statusImporting, queued: t.statusQueued,
    processing: t.statusProcessing, summarizing: t.statusSummarizing, done: t.statusDone, failed: t.statusFailed,
  };
  return labels[status]?.() ?? status;
}

// onStopped offers to undo a recording for five seconds.
function onStopped(id: string, t: Messages) {
  toast(t.recordingSaved(), {
    duration: 5000,
    action: {
      label: t.undo(),
      onClick: () =>
        Recorder.discard(id)
          .then(() => toast(t.recordingDeleted()))
          .catch((e) => toast.error(String(e.message ?? e))),
    },
  });
}
