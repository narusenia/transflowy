import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useMessages } from "@/i18n";
import { events, Models, Onboarding as Ob, Settings, type ModelStatus, type Settings as S } from "@/mygo";

const fail = (e: unknown) => toast.error(String((e as Error)?.message ?? e));

/** "Alt+Cmd+R" → "⌥⌘R" */
const keys = (s: string) =>
  s.split("+").map((k) => ({ Alt: "⌥", Cmd: "⌘", Ctrl: "⌃", Shift: "⇧" })[k] ?? k).join("");

function Choice({ on, title, body, onClick }: { on: boolean; title: string; body: string; onClick(): void }) {
  return (
    <button onClick={onClick} className={`w-full cursor-pointer rounded-lg border p-3 text-left ${on ? "border-primary bg-muted" : "hover:bg-muted/50"}`}>
      <div className="font-medium">{title}</div>
      <div className="text-xs text-muted-foreground">{body}</div>
    </button>
  );
}

/** First run: microphone, engine, shortcut. Everything can be changed later in the settings. */
export function Onboarding({ settings, onDone }: { settings: S; onDone(): void }) {
  const t = useMessages();
  const [step, setStep] = useState(0);
  const [s, setS] = useState(settings);
  const [mic, setMic] = useState<"" | "busy" | "ok" | "silent">("");
  const [key, setKey] = useState("");
  const [hasKey, setHasKey] = useState(false);
  const [trial, setTrial] = useState<"" | "recording" | "working">("");
  const [heard, setHeard] = useState<string | null>(null);
  const [model, setModel] = useState<ModelStatus>({ present: false, downloading: false, speakerPresent: false, speakerDownloading: false });
  const [pct, setPct] = useState(0);

  useEffect(() => {
    Models.status().then(setModel);
    Settings.hasKey("openrouter").then(setHasKey).catch(() => {});
    return events.modelChanged.on((p) => {
      if (p.kind !== "whisper") return;
      if (p.state === "downloading") setPct(p.total ? Math.floor((p.done / p.total) * 100) : 0);
      Models.status().then(setModel);
    });
  }, []);

  const save = (next: S) => {
    setS(next);
    Settings.set(next).catch(fail);
  };
  const finish = () => Settings.set({ ...s, onboarded: true }).then(onDone).catch(fail);
  const cloud = s.transcribe.kind !== "local";

  const body = [
    <>
      <p className="text-sm">{t.obStep1Body()}</p>
      <Button
        disabled={mic === "busy"}
        onClick={() => {
          setMic("busy");
          Ob.checkMic().then((r) => setMic(r.heard ? "ok" : "silent")).catch((e) => { setMic(""); fail(e); });
        }}
      >
        {t.obMicButton()}
      </Button>
      {mic === "ok" && <p className="text-sm text-primary">{t.obMicOk()}</p>}
      {mic === "silent" && <p className="text-sm text-muted-foreground">{t.obMicSilent()}</p>}
    </>,
    <>
      <p className="text-sm text-muted-foreground">{t.obStep2Body()}</p>
      <Choice on={cloud} title={t.obCloud()} body={t.obCloudBody()} onClick={() => save({ ...s, transcribe: { kind: "openrouter", model: "" } })} />
      <Choice on={!cloud} title={t.obLocal()} body={t.obLocalBody()} onClick={() => save({ ...s, transcribe: { kind: "local", model: "" } })} />
      {!cloud && (
        <div className="space-y-1">
          {model.present ? null : model.downloading ? (
            <p className="text-sm">{t.obModelProgress(pct)}</p>
          ) : (
            <Button variant="outline" onClick={() => Models.download().catch(fail)}>{t.obModelStart()}</Button>
          )}
          {!model.present && <p className="text-xs text-muted-foreground">{t.obModelBackground()}</p>}
        </div>
      )}
      <label className="block space-y-1 text-sm">
        {t.obKeyLabel()}
        <div className="flex gap-2">
          <Input type="password" autoComplete="off" value={key} placeholder={hasKey ? t.keySaved() : t.keyNotSet()} onChange={(e) => setKey(e.target.value)} />
          <Button variant="outline" disabled={!key} onClick={() => Settings.setKey("openrouter", key).then(() => { setKey(""); setHasKey(true); }).catch(fail)}>{t.keySave()}</Button>
        </div>
      </label>
      <label className="block space-y-1 text-sm">
        {t.obSummaryModel()}
        <Input value={s.summarize.model} placeholder="anthropic/claude-haiku-4.5" onChange={(e) => setS({ ...s, summarize: { ...s.summarize, model: e.target.value } })} onBlur={() => save(s)} />
        <span className="block text-xs text-muted-foreground">{t.obSummaryModelHint()}</span>
      </label>
      {cloud && (
        <div className="space-y-2">
          <Button
            variant="outline"
            disabled={trial !== "" || !hasKey}
            onClick={() => {
              setHeard(null);
              setTrial("recording");
              setTimeout(() => setTrial((x) => (x === "recording" ? "working" : x)), 10000);
              Ob.trial().then((x) => setHeard(x || t.obTrialEmpty())).catch(fail).finally(() => setTrial(""));
            }}
          >
            {t.obTrial()}
          </Button>
          <p className="text-xs text-muted-foreground">
            {trial === "recording" ? t.obTrialRecording() : trial === "working" ? t.obTrialWorking() : t.obTrialHint()}
          </p>
          {heard && <p className="rounded-md bg-muted p-2 text-sm whitespace-pre-wrap">{heard}</p>}
        </div>
      )}
    </>,
    <>
      <p className="text-sm">{t.obStep3Body()}</p>
      <div className="flex justify-center py-4">
        <kbd className="rounded-lg border bg-muted px-4 py-2 text-2xl">{keys(s.shortcut)}</kbd>
      </div>
    </>,
  ];
  const titles = [t.obStep1Title(), t.obStep2Title(), t.obStep3Title()];

  return (
    <div className="flex h-screen items-center justify-center bg-background p-6">
      <div className="w-full max-w-md space-y-4">
        <div className="flex items-start justify-between">
          <div>
            <p className="text-xs text-muted-foreground">{t.obWelcome()} · {step + 1} / 3</p>
            <h1 className="text-xl font-semibold">{titles[step]}</h1>
          </div>
          <Button variant="ghost" size="sm" onClick={finish}>{t.obSkip()}</Button>
        </div>
        <div className="flex gap-1">
          {[0, 1, 2].map((i) => <div key={i} className={`h-1 flex-1 rounded ${i <= step ? "bg-primary" : "bg-muted"}`} />)}
        </div>
        <div className="space-y-3">{body[step]}</div>
        <div className="flex justify-between pt-2">
          <Button variant="ghost" disabled={step === 0} onClick={() => setStep(step - 1)}>{t.obBack()}</Button>
          {step < 2 ? <Button onClick={() => setStep(step + 1)}>{t.obNext()}</Button> : <Button onClick={finish}>{t.obStart()}</Button>}
        </div>
      </div>
    </div>
  );
}
