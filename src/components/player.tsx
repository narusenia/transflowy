import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from "react";
import WaveSurfer from "wavesurfer.js";
import { Button } from "@/components/ui/button";
import { Pause, Play } from "lucide-react";
import { clock, mediaUrl } from "@/lib/media";
import { useMessages } from "@/i18n";

export interface PlayerHandle {
  /** Moves the playhead; plays from there when play is set. */
  seek(seconds: number, play?: boolean): void;
}

const speeds = [0.75, 1, 1.25, 1.5, 2];
// Shown while a recording has no peaks.json (made before peaks existed).
const flat = Array.from({ length: 300 }, () => 0.25);

/** The bottom player: wavesurfer.js draws the saved peaks and never decodes the audio. */
export const Player = forwardRef<PlayerHandle, { id: string; duration: number; onTime(seconds: number): void }>(
  function Player({ id, duration, onTime }, ref) {
    const t = useMessages();
    const box = useRef<HTMLDivElement>(null);
    const ws = useRef<WaveSurfer | null>(null);
    const onTimeRef = useRef(onTime);
    onTimeRef.current = onTime;
    const [playing, setPlaying] = useState(false);
    const [time, setTime] = useState(0);
    const [speed, setSpeed] = useState(1);

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
            cursorWidth: 0,
          });
          w.on("timeupdate", (s) => {
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
    }, [id, duration]);

    return (
      <div className="flex items-center gap-3 border-t px-4 py-2">
        <Button size="icon" variant="ghost" onClick={() => ws.current?.playPause()}>
          {playing ? <Pause /> : <Play />}
        </Button>
        <span className="w-24 shrink-0 text-xs tabular-nums text-muted-foreground">
          {clock(time)} / {clock(duration)}
        </span>
        <div ref={box} className="min-w-0 flex-1" />
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
