import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { useMessages } from "@/i18n";
import { getMode, setMode, type Mode } from "@/lib/theme";
import { Library, Recorder, Settings, Summarizer, type Provider, type Settings as S, type Template } from "@/mygo";

const fail = (e: unknown) => toast.error(String((e as Error)?.message ?? e));
const select = "h-8 rounded-md border bg-transparent px-2 text-sm";
const OLLAMA = "http://localhost:11434/v1";

function Row({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-[10rem_1fr] items-start gap-3 py-2">
      <div className="pt-1.5 text-sm">{label}</div>
      <div className="space-y-1">
        {children}
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
      </div>
    </div>
  );
}

/** A text field that saves when it loses focus. */
function Text({ value, onSave, placeholder, type }: { value: string; onSave(v: string): void; placeholder?: string; type?: string }) {
  const [v, setV] = useState(value);
  useEffect(() => setV(value), [value]);
  return <Input type={type} value={v} placeholder={placeholder} onChange={(e) => setV(e.target.value)} onBlur={() => v !== value && onSave(v)} />;
}

/** One API key: its value never reaches the page, only whether it is stored. */
function Key({ kind, label }: { kind: string; label: string }) {
  const t = useMessages();
  const [has, setHas] = useState(false);
  const [v, setV] = useState("");
  const refresh = () => Settings.hasKey(kind).then(setHas).catch(() => setHas(false));
  useEffect(() => { void refresh(); }, [kind]); // eslint-disable-line react-hooks/exhaustive-deps
  return (
    <Row label={label}>
      <div className="flex gap-2">
        <Input type="password" autoComplete="off" value={v} placeholder={has ? t.keySaved() : t.keyNotSet()} onChange={(e) => setV(e.target.value)} />
        <Button variant="outline" disabled={!v} onClick={() => Settings.setKey(kind, v).then(() => { setV(""); return refresh(); }).catch(fail)}>{t.keySave()}</Button>
        {has && <Button variant="ghost" onClick={() => Settings.deleteKey(kind).then(refresh).catch(fail)}>{t.keyDelete()}</Button>}
      </div>
    </Row>
  );
}

function ProviderFields({ p, kinds, onChange, modelHint }: { p: Provider; kinds: string[]; onChange(p: Provider): void; modelHint?: string }) {
  const t = useMessages();
  const names: Record<string, string> = { local: t.engineLocal(), openrouter: t.engineOpenRouter(), "openai-compat": t.engineCompat() };
  return (
    <>
      <Row label={t.engine()}>
        <select className={select} value={p.kind} onChange={(e) => onChange({ ...p, kind: e.target.value })}>
          {kinds.map((k) => <option key={k} value={k}>{names[k]}</option>)}
        </select>
      </Row>
      {p.kind === "openai-compat" && (
        <Row label={t.baseUrl()}>
          <Text value={p.baseUrl ?? ""} placeholder="http://localhost:11434/v1" onSave={(v) => onChange({ ...p, baseUrl: v })} />
        </Row>
      )}
      {p.kind !== "local" && (
        <Row label={t.model()}>
          <Text value={p.model} placeholder={modelHint} onSave={(v) => onChange({ ...p, model: v })} />
        </Row>
      )}
    </>
  );
}

export function SettingsDialog({ open, onOpenChange, onChanged }: { open: boolean; onOpenChange(o: boolean): void; onChanged(): void }) {
  const t = useMessages();
  const [s, setS] = useState<S | null>(null);
  const [templates, setTemplates] = useState<Template[]>([]);
  const [ollama, setOllama] = useState<string[] | null>(null);
  const [devices, setDevices] = useState<string[]>([]);
  const [mode, setModeState] = useState<Mode>(getMode);

  useEffect(() => {
    if (!open) return;
    Settings.get().then(setS).catch(fail);
    Summarizer.templates().then(setTemplates).catch(() => setTemplates([]));
    Recorder.devices().then(setDevices).catch(() => setDevices([]));
    setOllama(null);
  }, [open]);

  const save = (next: S) => {
    setS(next);
    Settings.set(next).then(onChanged).catch(fail);
  };
  const detect = () =>
    Summarizer.detectOllama().then((m) => {
      setOllama(m);
      if (m.length === 0) toast(t.ollamaNone());
      else if (s) save({ ...s, summarize: { kind: "openai-compat", baseUrl: OLLAMA, model: s.summarize.model && m.includes(s.summarize.model) ? s.summarize.model : m[0]! } });
    });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-2xl overflow-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t.settings()}</DialogTitle>
        </DialogHeader>
        {s && (
          <div className="divide-y">
            <section className="pb-2">
              <Row label={t.appearance()}>
                <select className={select} value={mode} onChange={(e) => { const m = e.target.value as Mode; setModeState(m); setMode(m); }}>
                  <option value="system">{t.themeSystem()}</option>
                  <option value="light">{t.themeLight()}</option>
                  <option value="dark">{t.themeDark()}</option>
                </select>
              </Row>
              <Row label={t.localOnly()} hint={t.localOnlyHint()}>
                <Switch checked={s.localOnly} onCheckedChange={(v) => save({ ...s, localOnly: v })} />
              </Row>
              <Row label={t.inputDevice()}>
                <select className={select} value={s.inputDevice} onChange={(e) => save({ ...s, inputDevice: e.target.value })}>
                  <option value="">{t.inputDeviceDefault()}</option>
                  {/* A saved device that is unplugged stays listed, so the choice is not lost. */}
                  {(s.inputDevice && !devices.includes(s.inputDevice) ? [s.inputDevice, ...devices] : devices).map((d) => <option key={d}>{d}</option>)}
                </select>
              </Row>
              <Row label={t.libraryDir()}>
                <div className="flex gap-2">
                  <Text value={s.libraryDir} onSave={(v) => save({ ...s, libraryDir: v })} />
                  <Button variant="outline" onClick={() => Library.chooseFolder().then((d) => d && save({ ...s, libraryDir: d })).catch(fail)}>{t.chooseFolder()}</Button>
                </div>
              </Row>
            </section>
            <section className="py-2">
              <h3 className="pt-2 text-sm font-semibold">{t.settingsTranscribe()}</h3>
              <ProviderFields p={s.transcribe} kinds={["local", "openrouter", "openai-compat"]} modelHint={t.modelDefault()} onChange={(p) => save({ ...s, transcribe: p })} />
            </section>
            <section className="py-2">
              <h3 className="pt-2 text-sm font-semibold">{t.settingsSummarize()}</h3>
              <ProviderFields p={s.summarize} kinds={["openrouter", "openai-compat"]} onChange={(p) => save({ ...s, summarize: p })} />
              <Row label="Ollama">
                <div className="flex items-center gap-2">
                  <Button variant="outline" size="sm" onClick={() => detect().catch(fail)}>{t.detectOllama()}</Button>
                  {ollama && ollama.length > 0 && (
                    <select aria-label={t.ollamaPick()} className={select} value={s.summarize.model} onChange={(e) => save({ ...s, summarize: { ...s.summarize, model: e.target.value } })}>
                      {ollama.map((m) => <option key={m}>{m}</option>)}
                    </select>
                  )}
                </div>
              </Row>
              <Row label={t.defaultTemplate()}>
                <select className={select} value={s.template} onChange={(e) => save({ ...s, template: e.target.value })}>
                  {templates.map((x) => <option key={x.name}>{x.name}</option>)}
                </select>
              </Row>
            </section>
            <section className="py-2">
              <h3 className="pt-2 text-sm font-semibold">{t.settingsKeys()}</h3>
              <Key kind="openrouter" label={t.apiKeyOpenRouter()} />
              <Key kind="openai-compat" label={t.apiKeyCompat()} />
            </section>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
