import { useEffect, useMemo, useState } from "react";
import { Captions } from "lucide-react";
import { IconButton } from "@/components/icon-button";
import { activeIndex } from "@/components/transcript";
import { useMessages } from "@/i18n";
import { speakerName } from "@/lib/speaker";
import type { Transcript } from "@/mygo";

const KEY = "transflowy.captions";
const HOLD = 1.5; // seconds a line stays after it ends, so a short one can be read

function savedOn() {
  try {
    return localStorage.getItem(KEY) !== "off";
  } catch {
    return true;
  }
}

/** The line being played, above the player. The switch at the right hides it. */
export function PlaybackCaption({ transcript, time }: { transcript: Transcript; time: number }) {
  const t = useMessages();
  const [on, setOn] = useState(savedOn);
  useEffect(() => {
    try {
      localStorage.setItem(KEY, on ? "on" : "off");
    } catch {
      // the choice then lasts until the page closes
    }
  }, [on]);
  const i = useMemo(() => activeIndex(transcript.segments, time), [transcript.segments, time]);
  const seg = i >= 0 ? transcript.segments[i] : undefined;
  const shown = on && seg && time <= Math.max(seg.end, seg.start) + HOLD ? seg : undefined;
  const name = shown ? speakerName(t, transcript, shown.speaker) : "";
  return (
    <div className="flex min-h-14 items-center gap-3 border-t px-4 py-2">
      <p className="min-w-0 flex-1 text-center text-lg leading-snug">
        {name && <span className="mr-2 text-sm font-medium text-muted-foreground">{name}</span>}
        {shown?.text}
      </p>
      <IconButton variant={on ? "secondary" : "ghost"} side="top" label={t.captionsToggle()} onClick={() => setOn(!on)}>
        <Captions />
      </IconButton>
    </div>
  );
}
