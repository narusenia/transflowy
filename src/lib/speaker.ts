import type { Messages } from "@/locales";
import type { Transcript } from "@/mygo";

// With a system audio track the microphone is the user and the track's own
// lines are the others (library.SpeakerMe / SpeakerOthers in Go).
const ME = 100;
const OTHERS = 101;

/** The default label of a speaker: 話者1 for speaker 0, 自分 and 相手 for the two tracks. */
export function defaultSpeaker(t: Messages, n: number) {
  return n === ME ? t.speakerMe() : n === OTHERS ? t.speakerOthers() : t.speakerLabel(n + 1);
}

/** The name the user gave to a speaker, or the default label; "" for a line without a speaker. */
export function speakerName(t: Messages, tr: Transcript, n: number) {
  return n < 0 ? "" : tr.speakers?.[String(n)] || defaultSpeaker(t, n);
}
