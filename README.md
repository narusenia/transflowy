<p align="center"><img src="resources/icon.png" alt="transflowy" width="128"></p>

<h1 align="center">transflowy</h1>

[日本語](README_ja.md)

A desktop app for macOS that records, transcribes and summarizes meetings and notes. Built with [mygo](https://github.com/egoist/mygo) (Go + WebView) and a React UI. Personal use first; Apple Silicon is the main target.

## Features

- **Record** from the microphone, or the system audio too (Core Audio Tap, macOS 14.2+; saved as a separate track). Start and stop with a global shortcut (default `⌥⌘R`), the menu bar icon or the window. A five-second undo follows a stop.
- **Import** audio and video files (m4a, mp3, wav, aac, caf, mp4, mov) by dropping them on the window or the Dock icon.
- **Transcribe** while you record, so there is almost no wait after you stop. Live captions follow the newest line.
  - Local: whisper.cpp `whisper-server`, bundled (large-v3-turbo model, downloaded on first use), language detected per chunk, so Japanese and English can mix.
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

## Install

There is no prebuilt download yet (no Developer ID signing), so you build the app yourself. Apple Silicon and macOS 14.2+ only.

Requirements: Xcode command line tools, Go, [bun](https://bun.sh), cmake, git.

```sh
git clone git@github.com:narusenia/transflowy.git
cd transflowy
bun install
bun run bundle        # builds whisper-server, copies the sherpa-onnx dylibs (first time and after updates)
bun run cert          # once: a local signing identity, so macOS keeps the microphone permission across rebuilds
bun run build:signed  # makes build/darwin-arm64/transflowy.app
```

`bun run build:dmg` also makes `build/darwin-arm64/transflowy 0.1.0 arm64.dmg`, a disk image to hand to another Mac. Copy `transflowy.app` to `/Applications` (or drag it from the image). It is signed with your own local identity, so it opens on the Mac that built it. To use it on another Mac, build there, or copy the app and run `xattr -dr com.apple.quarantine /Applications/transflowy.app` once.

On first launch:

1. Allow the microphone. System audio recording asks for its own permission the first time you turn it on.
2. Choose an engine: paste an OpenRouter API key, or download the local model (large-v3-turbo, about 0.6 GB, in the background). Recording works before either is ready; processing starts when it is.
3. Check the shortcut (default `⌥⌘R`).

Local speaker separation downloads its models from the settings. The API key is kept in the macOS keychain.

## Develop

```sh
bun run dev          # run the app (needs whisper-server: bun run bundle, or one on PATH)
bun run typecheck
go test ./...
```

In development, `OPENROUTER_API_KEY` in `.env` is also read. Messages live in `locales/ja.json`; after editing it run `bun run generate` (the generated files are committed).

## Status

- Builds and runs on Apple Silicon. `whisper-server` and the sherpa-onnx dylibs are bundled in the `.app`.
- Not done yet: Developer ID signing and notarization (so no prebuilt download), auto update, Windows.
- Planned: meeting detection, automatic template choice, chat about a recording, a warning for cloud-synced folders.

Details: [docs/requirements.md](docs/requirements.md), [docs/implementation-plan.md](docs/implementation-plan.md) (Japanese).

## License

[MIT](LICENSE)
