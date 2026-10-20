import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import WaveSurfer from "wavesurfer.js";
import { IconButton } from "@/components/icon-button";
import { FastForward, Pause, Play } from "lucide-react";
import { clock, mediaUrl } from "@/lib/media";
import { useMessages } from "@/i18n";
import { useDark } from "@/lib/theme";

export interface PlayerHandle {
  /** Moves the playhead; plays from there when play is set. */
  seek(seconds: number, play?: boolean): void;
}

const speeds = [0.75, 1, 1.25, 1.5, 2];
// Shown while a recording has no peaks.json (made before peaks existed).
const flat = Array.from({ length: 300 }, () => 0.25);

const SILENT = 0.02; // a peak below this is silence
const MIN_SILENCE = 1.5; // seconds: shorter pauses are left alone

/** [start, end] seconds of the runs of quiet bars. Resolution is one bar: a bar is long in a long recording. */
function silences(peaks: number[], duration: number): [number, number][] {
  const bar = duration / peaks.length;
  const out: [number, number][] = [];
  let from = -1;
  for (let i = 0; i <= peaks.length; i++) {
    const quiet = i < peaks.length && (peaks[i] ?? 1) < SILENT;
    if (quiet && from < 0) from = i;
    if (!quiet && from >= 0) {
      if ((i - from) * bar >= MIN_SILENCE) out.push([from * bar, i * bar]);
      from = -1;
    }
  }
  return out;
}

/** The bottom player: wavesurfer.js draws the saved peaks and never decodes the audio. */
export const Player = forwardRef<PlayerHandle, { id: string; duration: number; done: boolean; onTime(seconds: number): void }>(
  function Player({ id, duration, done, onTime }, ref) {
    const t = useMessages();
    const dark = useDark(); // the waveform is drawn with colors read once: draw it again when they change
    const box = useRef<HTMLDivElement>(null);
    const ws = useRef<WaveSurfer | null>(null);
    const onTimeRef = useRef(onTime);
    onTimeRef.current = onTime;
    const [playing, setPlaying] = useState(false);
    const [time, setTime] = useState(0);
    const [speed, setSpeed] = useState(1);
    const [skip, setSkip] = useState(false);
    const skipRef = useRef(false);
    skipRef.current = skip;
    const quiet = useRef<[number, number][]>([]);

    useImperativeHandle(ref, () => ({
      seek(seconds, play) {
        const w = ws.current;
        if (!w || duration <= 0) return;
        w.setTime(Math.min(seconds, duration));
        setTime(seconds);
        onTimeRef.current(seconds);
        if (play) void w.play();
      },
    }));

    useEffect(() => {
      if (!box.current) return;
      let dead = false;
      const style = getComputedStyle(document.documentElement);
      const audio = new Audio(mediaUrl(id, "audio.m4a"));
      audio.preload = "metadata";
      fetch(mediaUrl(id, "peaks.json"))
        .then((r) => (r.ok ? (r.json() as Promise<number[]>) : flat))
        .catch(() => flat)
        .then((peaks) => {
          if (dead || !box.current) return;
          const w = WaveSurfer.create({
            container: box.current,
            media: audio,
            peaks: [peaks],
            duration,
            height: 48,
            barWidth: 2,
            barGap: 1,
            barRadius: 2,
            waveColor: style.getPropertyValue("--muted-foreground") || "#888",
            progressColor: style.getPropertyValue("--primary") || "#000",
            // A red bar at the playhead: the played part alone is hard to tell from the rest.
            cursorWidth: 2,
            cursorColor: style.getPropertyValue("--destructive") || "#e11d48",
          });
          quiet.current = silences(peaks, duration);
          w.on("timeupdate", (s) => {
            const gap = skipRef.current && quiet.current.find(([a, b]) => s >= a && s < b);
            if (gap) w.setTime(gap[1]); // jump over the silence
            setTime(s);
            onTimeRef.current(s);
          });
          w.on("play", () => setPlaying(true));
          w.on("pause", () => setPlaying(false));
          ws.current = w;
        });
      return () => {
        dead = true;
        ws.current?.destroy();
        ws.current = null;
        setPlaying(false);
        setTime(0);
      };
    }, [id, duration, dark, done]); // peaks.json and the m4a appear when processing ends

    // Space plays and pauses, unless it types a space or presses a control of a dialog.
    useEffect(() => {
      const on = (e: KeyboardEvent) => {
        if (e.key !== " " || e.repeat || e.isComposing || e.metaKey || e.ctrlKey || e.altKey) return;
        const el = e.target as HTMLElement;
        if (el.closest("input, textarea, select, [contenteditable=true], [role=dialog]")) return;
        e.preventDefault(); // also stops a focused row or button from being "clicked"
        ws.current?.playPause();
      };
      window.addEventListener("keydown", on);
      return () => window.removeEventListener("keydown", on);
    }, []);

    return (
      <div className="flex items-center gap-3 border-t px-4 py-2">
        <IconButton size="icon" side="top" variant="ghost" label={playing ? t.pause() : t.play()} onClick={() => ws.current?.playPause()}>
          {playing ? <Pause /> : <Play />}
        </IconButton>
        <span className="w-24 shrink-0 text-xs tabular-nums text-muted-foreground">
          {clock(time)} / {clock(duration)}
        </span>
        <div ref={box} className="min-w-0 flex-1" />
        <IconButton size="icon" side="top" variant={skip ? "secondary" : "ghost"} label={t.skipSilence()} onClick={() => setSkip(!skip)}>
          <FastForward />
        </IconButton>
        <select
          aria-label={t.playbackSpeed()}
          className="h-7 rounded-md border bg-transparent px-1 text-xs"
          value={speed}
          onChange={(e) => {
            const v = Number(e.target.value);
            setSpeed(v);
            ws.current?.setPlaybackRate(v, true);
          }}
        >
          {speeds.map((s) => (
            <option key={s} value={s}>
              {s}x
            </option>
          ))}
        </select>
      </div>
    );
  },
);
