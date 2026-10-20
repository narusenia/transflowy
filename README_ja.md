<p align="center"><img src="resources/icon.png" alt="transflowy" width="128"></p>

<h1 align="center">transflowy</h1>

[English](README.md)

会議やメモを録音して、文字起こしと要約まで行う macOS のデスクトップアプリ。[mygo](https://github.com/egoist/mygo)（Go + WebView）と React の UI で作っている。まずは個人用、Apple Silicon が第一対象。

## 機能

- **録音**: マイク、またはシステム音声も（Core Audio Tap、macOS 14.2 以上。別トラックで保存）。グローバルショートカット（既定 `⌥⌘R`）、メニューバーのアイコン、窓のボタンで開始・停止できる。停止後 5 秒は取り消せる。
- **取り込み**: 音声・動画ファイル（m4a / mp3 / wav / aac / caf / mp4 / mov）。窓や Dock アイコンへのドロップに対応。
- **文字起こし**: 録音中に先行して処理するので、停止後の待ちがほとんどない。ライブ字幕は最新の行に追従する。
  - ローカル: 同梱の whisper.cpp `whisper-server`（モデルは large-v3-turbo、初回に取得）。チャンクごとに言語を判定するので、日本語と英語が混ざってもよい。
  - クラウド: OpenRouter の `microsoft/mai-transcribe-2`、または OpenAI 互換の URL。
- **話者分離**: クラウドのエンジン、またはローカル（sherpa-onnx）。話者名は付け替えられる。システム音声のトラックがあるときは、マイク側を「自分」にする。
- **要約**: テンプレート（議事録 / 1on1 / 商談 / 面接 / 講義 / 汎用メモ）か、自作の Markdown テンプレート。1 件に複数の要約を持てる。手書きして編集することもできる。`[12:34]` をクリックすると文字起こしの該当行へ飛ぶ。
- **編集**: 文字起こしを行単位で直せる（元の認識結果は残り、戻せる）。一括置換と、エンジンに渡す固有名詞辞書への追加もできる。
- **再生**: 速度変更、再生中の行の字幕、文字起こしの追従、無音スキップ。
- **検索**: `⌘K` でタイトル・文字起こし・要約を横断検索。
- **書き出し**: クリップボード、`.md`、`.txt`、`.srt`。
- **日本語と英語**の画面。システムの言語に合わせ、設定で選べる。ビルトインの要約テンプレートも両方ある。
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

## インストール

Apple Silicon、macOS 14.2 以上のみ。

### ダウンロード

[Releases](https://github.com/narusenia/transflowy/releases) から `transflowy <バージョン> arm64.dmg` を取り、開いてアプリを `/Applications` にドラッグする。Developer ID で署名していないので、macOS が初回の起動を止める。次のコマンドを 1 回実行してから開く。

```sh
xattr -dr com.apple.quarantine /Applications/transflowy.app
```

### 自分でビルドする

必要なもの: Xcode のコマンドラインツール、Go、[bun](https://bun.sh)、cmake、git。

```sh
git clone git@github.com:narusenia/transflowy.git
cd transflowy
bun install
bun run bundle        # whisper-server のビルドと sherpa-onnx の dylib のコピー（初回と更新後）
bun run cert          # 初回のみ。ローカルの署名用証明書。再ビルドしてもマイクの許可が消えなくなる
bun run build:signed  # build/darwin-arm64/transflowy.app ができる
```

`bun run build:dmg` なら `build/darwin-arm64/transflowy 0.1.0 arm64.dmg` も作る。ほかの Mac へ渡すためのディスクイメージ。`transflowy.app` を `/Applications` にコピーする（イメージからドラッグでもよい）。署名は自分のローカル証明書なので、ビルドした Mac ではそのまま開ける。ほかの Mac では、上の `xattr` コマンドを 1 回実行する。

初回起動の流れ:

1. マイクを許可する。システム音声の録音は、初めてオンにしたときに別の許可を求められる。
2. エンジンを選ぶ。OpenRouter の API キーを貼るか、ローカルモデル（large-v3-turbo、約 0.6 GB）をダウンロードする。ダウンロードは裏で進む。どちらが整う前でも録音はでき、整い次第、処理が始まる。
3. ショートカット（既定 `⌥⌘R`）を確認する。

ローカルの話者分離は、設定からモデルをダウンロードする。API キーは macOS のキーチェーンに保存する。

## 開発

```sh
bun run dev          # アプリを起動（whisper-server が要る。bun run bundle か、PATH 上のもの）
bun run typecheck
go test ./...
```

開発中は `.env` の `OPENROUTER_API_KEY` も読む。文言は `locales/ja.json` にある。編集したら `bun run generate` を実行する（生成物はコミットに含める）。

## 状況

- Apple Silicon でビルドして動作する。`whisper-server` と sherpa-onnx の dylib は .app に同梱している。
- 未対応: Developer ID 署名と公証（ダウンロード版は `xattr` の手順が要る）、自動更新、Windows。
- 予定: 会議の自動検出、テンプレートの自動選択、録音内容への質問チャット、クラウド同期フォルダの警告。

詳細: [docs/requirements.md](docs/requirements.md)、[docs/implementation-plan.md](docs/implementation-plan.md)。

## ライセンス

[MIT](LICENSE)。含まれるソフトウェアと、ダウンロードするモデルのライセンスは [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)（アプリ内でも、情報ボタンの「オープンソースライセンス」で見られる）。依存を変えたら `bun run notices` を実行する。
