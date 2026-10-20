/** A file in a recording's folder, served by the app's media protocol. */
export const mediaUrl = (id: string, file: string) => `media://localhost/${encodeURIComponent(id)}/${file}`;

/** 75.4 → "1:15"; an hour or more → "1:02:03". */
export function clock(s: number) {
  const t = Math.floor(s);
  const h = Math.floor(t / 3600);
  const mm = Math.floor((t % 3600) / 60);
  const ss = String(t % 60).padStart(2, "0");
  return h ? `${h}:${String(mm).padStart(2, "0")}:${ss}` : `${mm}:${ss}`;
}

/** "[75:30]" → 4530. The summaries write minutes without wrapping at 60. */
export const stamp = /\[(\d+):(\d{2})\]/g;
export const stampSeconds = (m: string, s: string) => Number(m) * 60 + Number(s);
