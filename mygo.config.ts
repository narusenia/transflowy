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
  macos: {
    // Core Audio Tap (system audio, a later stage) needs 14.2.
    minimumSystemVersion: "14.2",
    // Without this key macOS refuses microphone access without asking.
    infoPlist: { NSMicrophoneUsageDescription: "会議やメモを録音して文字起こしするために、マイクを使います。" },
  },
});
