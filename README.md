# transflowy

[日本語](README_ja.md)

A desktop app for macOS that records, transcribes and summarizes meetings and notes. Built with [mygo](https://github.com/egoist/mygo) (Go + WebView) and a React UI. Personal use first; Apple Silicon is the main target.

## Features

- **Record** from the microphone, or the system audio too (Core Audio Tap, macOS 14.2+; saved as a separate track). Start and stop with a global shortcut (default `⌥⌘R`), the menu bar icon or the window. A five-second undo follows a stop.
- **Import** audio and video files (m4a, mp3, wav, aac, caf, mp4, mov) by dropping them on the window or the Dock icon.
- **Transcribe** while you record, so there is almost no wait after you stop. Live captions follow the newest line.
  - Local: whisper.cpp `whisper-server` (large-v3-turbo), language detected per chunk, so Japanese and English can mix.
  - Cloud: OpenRouter `microsoft/mai-transcribe-2`, or any OpenAI-compatible URL.
- **Tell speakers apart** with the cloud engine or locally (sherpa-onnx), then rename them. With a system track, your microphone is "自分".
- **Summarize** with templates (minutes, 1on1, sales, interview, lecture, memo), or your own Markdown templates. Keep several summaries per recording, or write and edit one by hand. `[12:34]` links jump to the transcript line.
- **Edit** the transcript line by line (the original is kept and can be restored), replace text in bulk, and add words to a dictionary that the engines receive.
- **Play** with speed control, a caption of the playing line, the playhead following the transcript, and an option to skip silence.
- **Search** titles, transcripts and summaries with `⌘K`.
- **Export** to the clipboard, `.md`, `.txt` or `.srt`.
- **Local only** switch: while it is on, nothing is sent to the cloud. One guard (`internal/netguard`) covers every request.

## Data

Plain files, one folder per recording. The default location is `~/Documents/Transflowy/` and can be changed.

```
2026-10-20_1030_weekly/
  audio.m4a          # mic.m4a / system.m4a with a system track
  transcript.json    # segments: start, end, speaker, text, original
  summary-<template>.md
  meta.json
templates/
dictionary.txt
```

Point it at an Obsidian vault and the notes are already there.

## Run from source

Requirements: macOS 14.2+, Go, [bun](https://bun.sh), and `whisper-server` for local transcription (on `PATH`, for example from Homebrew's `whisper-cpp`; it is not bundled yet). API keys go in the app's settings and are kept in the macOS keychain. In development, `OPENROUTER_API_KEY` in `.env` is also read.

```sh
bun install
bun run dev          # run the app
bun run typecheck
go test ./...
```

Messages live in `locales/ja.json`. After editing it, run `bun run generate`; the generated files are committed.

To keep the microphone permission across rebuilds, sign with a fixed local identity:

```sh
bun run cert          # once
bun run build:signed
```

## Status

- Works from source on Apple Silicon.
- Not done yet: bundling `whisper-server` and the sherpa-onnx dylibs into the `.app`, Developer ID signing and notarization, auto update, Windows.
- Planned: meeting detection, automatic template choice, chat about a recording, a warning for cloud-synced folders.

Details: [docs/requirements.md](docs/requirements.md), [docs/implementation-plan.md](docs/implementation-plan.md) (Japanese).
