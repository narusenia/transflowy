import { defineConfig } from "mygo-cli";

export default defineConfig({
  name: "transflowy",
  identifier: "one.nxeu.transflowy",
  version: "0.1.0",
  // `mygo dev` runs devCommand and loads devUrl; `mygo build` runs
  // buildCommand and embeds frontendDist into the app.
  devUrl: "http://localhost:5173",
  devCommand: "bun run dev:web",
  buildCommand: "bun run build:web",
  frontendDist: "dist",
  bindings: "src/mygo.ts",
  out: "build",
  // Files dropped on the Dock icon, or opened with the app, are imported.
  fileAssociations: [
    { ext: ["m4a", "mp3", "wav", "aac", "caf", "aiff", "aif", "flac", "mp4", "mov", "m4v"], name: "Audio / Video", role: "Viewer" },
  ],
  // Messages live in locales/<tag>.json; Japanese is the source, others fall back to it.
  i18n: { dir: "locales", source: "ja" },
  macos: {
    // Core Audio Tap (system audio, a later stage) needs 14.2.
    minimumSystemVersion: "14.2",
    // Without this key macOS refuses microphone access without asking.
    infoPlist: {
      NSMicrophoneUsageDescription: "会議やメモを録音して文字起こしするために、マイクを使います。",
      // Without this key macOS refuses system audio capture without asking.
      NSAudioCaptureUsageDescription: "会議の相手の声など、Mac から出る音を録音して文字起こしするために、システム音声を使います。",
    },
  },
});
