import { useEffect, useRef, useState } from "react";
import { toast, Toaster } from "sonner";
import { Button } from "@/components/ui/button";
import { Onboarding } from "@/components/onboarding";
import { Detail, type Jump } from "@/components/detail";
import { Search } from "@/components/search";
import { SettingsDialog } from "@/components/settings";
import { Sidebar } from "@/components/sidebar";
import { TitleBar } from "@/components/titlebar";
import { useMessages } from "@/i18n";
import type { Messages } from "@/locales";
import { events, Importer, Library, Models, Recorder, Settings, Summarizer, type Hit, type Meta, type ModelStatus, type Recording, type RecordingState, type Settings as S, type Template } from "@/mygo";

const idle: RecordingState = { recording: false, id: "", elapsed: 0, level: 0 };
const LEVELS = 150; // bars of the live waveform
const fail = (e: unknown) => toast.error(String((e as Error)?.message ?? e));

export function App() {
  const [recordings, setRecordings] = useState<Meta[]>([]);
  const [state, setState] = useState(idle);
  const [levels, setLevels] = useState<number[]>([]);
  const wasRecording = useRef(false);
  const t = useMessages();
  const tRef = useRef(t); // the event handler below outlives a render
  tRef.current = t;

  const [selected, setSelected] = useState("");
  const [detail, setDetail] = useState<Recording | null>(null);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [model, setModel] = useState<ModelStatus>({ present: true, downloading: false });
  const [percent, setPercent] = useState(0);
  const [sidebar, setSidebar] = useState(true);
  const [searching, setSearching] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [jump, setJump] = useState<Jump | null>(null);
  const [first, setFirst] = useState<S | null>(null); // settings, while the first-run screen is up
  const selectedRef = useRef("");
  selectedRef.current = selected;

  const loadDetail = (id: string) =>
    id ? Library.get(id).then((r) => selectedRef.current === id && setDetail(r)).catch(() => setDetail(null)) : setDetail(null);
  const reload = () => {
    Library.list().then(setRecordings);
    loadDetail(selectedRef.current);
  };
  const select = (id: string) => {
    selectedRef.current = id;
    setSelected(id);
    setDetail((d) => (d?.meta.id === id ? d : null));
    loadDetail(id);
  };

  useEffect(() => {
    Settings.get().then((c) => !c.onboarded && setFirst(c)).catch(() => {});
    Models.status().then(setModel);
    Summarizer.templates().then(setTemplates).catch(() => {});
    Recorder.state().then((s) => {
      wasRecording.current = s.recording;
      setState(s);
    });
    reload();
    // The menu bar and the shortcut also start and stop recordings.
    const offs = [
      events.recordingChanged.on((s) => {
        if (wasRecording.current && !s.recording) onStopped(s.id, tRef.current);
        if (!wasRecording.current && s.recording) {
          setLevels([]);
          select(s.id);
        }
        if (s.recording) setLevels((l) => [...l.slice(-(LEVELS - 1)), s.level]);
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

  useEffect(() => {
    const on = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return;
      if (e.key === "k") {
        e.preventDefault();
        setSearching((s) => !s);
      } else if (e.key === "\\") {
        e.preventDefault();
        setSidebar((s) => !s);
      }
    };
    window.addEventListener("keydown", on);
    return () => window.removeEventListener("keydown", on);
  }, []);

  const pick = (h: Hit) => {
    select(h.id);
    setJump((j) => ({ n: (j?.n ?? 0) + 1, time: h.kind === "transcript" ? h.time : undefined, template: h.template }));
  };

  if (first) return <><Toaster position="bottom-center" /><Onboarding settings={first} onDone={() => setFirst(null)} /></>;

  return (
    <div className="flex h-screen flex-col text-sm">
      <Toaster position="bottom-center" />
      <TitleBar onToggleSidebar={() => setSidebar((s) => !s)} onSearch={() => setSearching(true)} onSettings={() => setSettingsOpen(true)} />
      <div className="flex min-h-0 flex-1">
      {sidebar && (
        <Sidebar
          recordings={recordings}
          selected={selected}
          onSelect={select}
          recording={state.recording}
          elapsed={state.elapsed}
          level={state.level}
          onRecord={() => (state.recording ? Recorder.stop() : Recorder.start()).catch(fail)}
          onImport={() => Importer.importDialog().catch(fail)}
        />
      )}
      <main className="flex min-w-0 flex-1 flex-col">
        {!model.present && (
          <div className="flex items-center gap-3 border-b bg-muted px-4 py-2">
            <span className="flex-1">{t.errNoModel()}</span>
            <Button size="sm" disabled={model.downloading} onClick={() => Models.download().catch(fail)}>
              {model.downloading ? t.modelDownloading(percent) : t.modelDownload()}
            </Button>
          </div>
        )}
        <div className="min-h-0 flex-1">
          {detail && detail.meta.id === selected ? (
            <Detail rec={detail} templates={templates} jump={jump} live={state.recording && state.id === selected ? { levels, elapsed: state.elapsed } : null} onChanged={reload} />
          ) : (
            <p className="flex h-full items-center justify-center text-muted-foreground">{t.selectPrompt()}</p>
          )}
        </div>
      </main>
      </div>
      <Search open={searching} onOpenChange={setSearching} onPick={pick} />
      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} onChanged={() => Summarizer.templates().then(setTemplates).catch(() => {})} />
    </div>
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
          .catch(fail),
    },
  });
}
