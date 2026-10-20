# transflowy

[English](README.md)

会議やメモを録音して、文字起こしと要約まで行う macOS のデスクトップアプリ。[mygo](https://github.com/egoist/mygo)（Go + WebView）と React の UI で作っている。まずは個人用、Apple Silicon が第一対象。

## 機能

- **録音**: マイク、またはシステム音声も（Core Audio Tap、macOS 14.2 以上。別トラックで保存）。グローバルショートカット（既定 `⌥⌘R`）、メニューバーのアイコン、窓のボタンで開始・停止できる。停止後 5 秒は取り消せる。
- **取り込み**: 音声・動画ファイル（m4a / mp3 / wav / aac / caf / mp4 / mov）。窓や Dock アイコンへのドロップに対応。
- **文字起こし**: 録音中に先行して処理するので、停止後の待ちがほとんどない。ライブ字幕は最新の行に追従する。
  - ローカル: whisper.cpp の `whisper-server`（large-v3-turbo）。チャンクごとに言語を判定するので、日本語と英語が混ざってもよい。
  - クラウド: OpenRouter の `microsoft/mai-transcribe-2`、または OpenAI 互換の URL。
- **話者分離**: クラウドのエンジン、またはローカル（sherpa-onnx）。話者名は付け替えられる。システム音声のトラックがあるときは、マイク側を「自分」にする。
- **要約**: テンプレート（議事録 / 1on1 / 商談 / 面接 / 講義 / 汎用メモ）か、自作の Markdown テンプレート。1 件に複数の要約を持てる。手書きして編集することもできる。`[12:34]` をクリックすると文字起こしの該当行へ飛ぶ。
- **編集**: 文字起こしを行単位で直せる（元の認識結果は残り、戻せる）。一括置換と、エンジンに渡す固有名詞辞書への追加もできる。
- **再生**: 速度変更、再生中の行の字幕、文字起こしの追従、無音スキップ。
- **検索**: `⌘K` でタイトル・文字起こし・要約を横断検索。
- **書き出し**: クリップボード、`.md`、`.txt`、`.srt`。
- **ローカル限定**: オンの間はクラウドへ何も送らない。通信は `internal/netguard` の 1 か所で止める。

## データ

ファイルのみ。録音 1 件が 1 フォルダ。既定の保存先は `~/Documents/Transflowy/`、変更できる。

```
2026-10-20_1030_週次定例/
  audio.m4a          # システム音声ありなら mic.m4a / system.m4a
  transcript.json    # セグメント: start, end, speaker, text, original
  summary-<テンプレート名>.md
  meta.json
templates/
dictionary.txt
```

保存先を Obsidian の Vault にすれば、そのままノートとして読める。

## ソースから動かす

必要なもの: macOS 14.2 以上、Go、[bun](https://bun.sh)、ローカル文字起こし用の `whisper-server`（`PATH` に置く。たとえば Homebrew の `whisper-cpp`。まだ同梱していない）。API キーはアプリの設定で入れ、macOS のキーチェーンに保存する。開発中は `.env` の `OPENROUTER_API_KEY` も読む。

```sh
bun install
bun run dev          # アプリを起動
bun run typecheck
go test ./...
```

文言は `locales/ja.json` にある。編集したら `bun run generate` を実行する。生成物はコミットに含める。

再ビルドしてもマイクの許可が消えないよう、固定のローカル証明書で署名する。

```sh
bun run cert          # 初回のみ
bun run build:signed
```

## 状況

- Apple Silicon でソースから動作する。
- 未対応: `whisper-server` と sherpa-onnx の dylib の .app 同梱、Developer ID 署名と公証、自動更新、Windows。
- 予定: 会議の自動検出、テンプレートの自動選択、録音内容への質問チャット、クラウド同期フォルダの警告。

詳細: [docs/requirements.md](docs/requirements.md)、[docs/implementation-plan.md](docs/implementation-plan.md)。
