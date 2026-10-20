import { useEffect, useRef, useState } from "react";
import { toast, Toaster } from "sonner";
import { Button } from "@/components/ui/button";
import { events, Library, Recorder, Settings, type Meta, type RecordingState } from "@/mygo";

const idle: RecordingState = { recording: false, id: "", elapsed: 0, level: 0 };

const clock = (s: number) =>
  `${Math.floor(s / 60)}:${String(Math.floor(s) % 60).padStart(2, "0")}`;

export function App() {
  const [dir, setDir] = useState("");
  const [recordings, setRecordings] = useState<Meta[]>([]);
  const [state, setState] = useState(idle);
  const wasRecording = useRef(false);

  const reload = () => Library.list().then(setRecordings);

  useEffect(() => {
    Settings.get().then((s) => setDir(s.libraryDir));
    Recorder.state().then((s) => {
      wasRecording.current = s.recording;
      setState(s);
    });
    reload();
    // The menu bar and the shortcut also start and stop recordings.
    return events.recordingChanged.on((s) => {
      if (wasRecording.current && !s.recording) onStopped(s.id);
      wasRecording.current = s.recording;
      setState(s);
      if (!s.recording) reload();
    });
  }, []);

  return (
    <main className="flex h-screen flex-col items-center justify-center gap-4 text-sm">
      <Toaster position="bottom-center" />
      <Button
        size="lg"
        variant={state.recording ? "destructive" : "default"}
        onClick={() => (state.recording ? Recorder.stop() : Recorder.start()).catch((e) => toast.error(String(e.message ?? e)))}
      >
        {state.recording ? `停止 ${clock(state.elapsed)}` : "録音を開始"}
      </Button>
      <div className="h-1.5 w-48 overflow-hidden rounded bg-muted">
        <div className="h-full bg-primary transition-[width] duration-150" style={{ width: `${Math.min(1, state.level * 2) * 100}%` }} />
      </div>
      <p className="text-muted-foreground">
        {dir} · {recordings.length} 件
      </p>
    </main>
  );
}

// onStopped offers to undo a recording for five seconds.
function onStopped(id: string) {
  toast("録音を保存しました", {
    duration: 5000,
    action: {
      label: "取り消し",
      onClick: () =>
        Recorder.discard(id)
          .then(() => toast("録音を削除しました"))
          .catch((e) => toast.error(String(e.message ?? e))),
    },
  });
}
