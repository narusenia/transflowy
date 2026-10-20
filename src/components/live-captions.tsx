import { useMessages } from "@/i18n";
import { speakerName } from "@/lib/speaker";
import type { Recording } from "@/mygo";

/**
 * The newest lines of a recording in large type, while it runs. Lines arrive
 * as the pipeline finishes each chunk: seconds behind the speech with the
 * local engine, minutes with the cloud one (it sends long chunks, so that its
 * speakers stay consistent).
 */
export function LiveCaptions({ rec, elapsed }: { rec: Recording; elapsed: number }) {
  const t = useMessages();
  const segs = rec.transcript.segments;
  const last = segs.slice(-3);
  const behind = elapsed - (rec.meta.processedUntil ?? 0) > 3; // audio the engine has not reached
  return (
    <div className="flex h-full flex-col justify-end gap-3 overflow-hidden px-6 py-6">
      {last.length === 0 && (
        <div className="space-y-2 text-center text-muted-foreground">
          <p className="text-lg">{t.liveWaiting()}</p>
          <p className="text-xs">{t.liveCloudHint()}</p>
        </div>
      )}
      {last.map((s, i) => {
        const name = speakerName(t, rec.transcript, s.speaker);
        return (
          <p key={`${s.start}-${i}`} className={`leading-relaxed transition-opacity ${i === last.length - 1 ? "text-2xl" : "text-lg opacity-50"}`}>
            {name && <span className="mr-2 text-sm font-medium text-muted-foreground">{name}</span>}
            {s.text}
          </p>
        );
      })}
      {behind && last.length > 0 && <p className="animate-pulse text-sm text-muted-foreground">{t.liveRecognizing()}</p>}
    </div>
  );
}
