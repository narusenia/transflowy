import type { Messages } from "@/locales";
import type { Meta } from "@/mygo";

/** What a recording is doing right now, for the states that take time; null otherwise. */
export function progressText(t: Messages, m: Meta): string | null {
  switch (m.status) {
    case "importing": return t.statusImporting();
    case "queued": return t.statusQueued();
    case "diarizing": return t.statusDiarizing();
    case "summarizing": return t.statusSummarizing();
    case "processing": {
      const pct = m.duration > 0 ? Math.min(99, Math.floor(((m.processedUntil ?? 0) / m.duration) * 100)) : 0;
      return t.statusProcessingPercent(pct);
    }
    default: return null;
  }
}
