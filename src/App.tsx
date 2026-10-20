import { useEffect, useRef, useState } from "react";
import { toast, Toaster } from "sonner";
import { Button } from "@/components/ui/button";
import { useMessages } from "@/i18n";
import type { Messages } from "@/locales";
import { events, Importer, Library, Recorder, Settings, type Meta, type RecordingState } from "@/mygo";

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

  const reload = () => Library.list().then(setRecordings);

  useEffect(() => {
    Settings.get().then((s) => setDir(s.libraryDir));
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
      <p className="text-muted-foreground">
        {dir} · {t.recordingCount(recordings.length)}
      </p>
      {/* ponytail: placeholder list, replaced by the real list in the screen phase */}
      <ul className="max-h-48 w-96 overflow-auto text-xs text-muted-foreground">
        {recordings.map((r) => (
          <li key={r.id} className="flex justify-between gap-2">
            <span className="truncate">{r.title || r.id}</span>
            <span>
              {r.status} · {clock(r.duration)}
            </span>
          </li>
        ))}
      </ul>
    </main>
  );
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
