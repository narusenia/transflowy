# transflowy 実装計画

要件は [requirements.md](requirements.md)。完了したらチェックを入れる。各タスクは 1 コミット以上の論理単位。

## Phase 0: PoC

結果は各項目の下に 1〜2 行で追記する。決定が変わる場合は requirements.md も直す。

- [x] P1 OpenRouter 文字起こし: 1 分 / 5 分 / 30 分の音声で制約確認、MAI-Transcribe-2 の diarization・phraseList の渡し方確認
  - `POST /api/v1/audio/transcriptions`（JSON、`input_audio.data` は base64、`provider.options.azure.diarization.enabled` / `phraseList.phrases`）が動作。料金は約 $0.10/時（297 秒で $0.00825）
  - **処理速度**: 5 分（297 秒）の音声が 8.7〜9.3 秒、約 34 倍速。60 秒タイムアウトに当たるのは約 30 分超の見込み。**クラウドは長めのチャンク（最大 10 分）で送れる**（P2 のローカルは 15 秒）
  - **日英混在**: 日本語 + 英語の 1 チャンクでも英語が落ちず、1.1 秒で完了（Whisper と違う）
  - **話者分離**: `say` の 2 声（Kyoko / Reed）を 4 ターンとも正しく speaker 0 / 1 に分けた（sherpa-onnx では不可だった）。長いチャンクで送れば話者 ID がチャンク内で一貫する
  - **受け付ける形式は wav / mp3 / flac**。m4a / aac / webm は 400。→ クラウドへは WAV で送る（16kHz モノラルで 10 分 ≈ 19MB、上限 25MB 内）。保存形式（m4a）はそのまま送れない
  - 話者分離なしだと segment が非常に長い（最大 177 秒）。表示用の行は VAD 区間か句読点で自前分割する。単語タイムスタンプは日本語だと 1 文字単位
  - 未確認: 実際の会議音声での精度、`prompt` 相当（phraseList）の効果、`mai-transcribe-2` の同時リクエスト上限
- [x] P2 whisper-server: large-v3-turbo の日本語 + 英語混在精度と処理速度（実時間比）
  - `ggml-large-v3-turbo-q5_0.bin`（574MB）+ `whisper-server`（Homebrew 導入済み）。起動 9 秒、22.8 秒の音声を約 1.7 秒で処理（約 13 倍速、M 系 Metal）
  - 日本語は正確。文中の英単語（API、スプリント）も問題なし。`prompt` に辞書語を渡す方式も動いた（読点の入り方が変わる程度）
  - **問題**: 言語は処理窓（約 30 秒）ごとに 1 つしか判定されない。日本語 15 秒 + 英語 8 秒を 1 チャンクで渡すと英語が丸ごと落ちた。英語だけのチャンクは正常 → VAD で短く切り、チャンクごとに `language=auto` で処理する（上限 15 秒の設計と一致）。要件のモデルは q5_0 に変更し、初回 DL を約 0.6GB に修正
- 未確認: 実際の会議音声（雑音・早口・重なり）での精度
- [x] P3 sherpa-onnx Go: silero VAD と話者分離が mac で動くか（Windows はビルドのみ）
  - `github.com/k2-fsa/sherpa-onnx-go-macos`（cgo、dylib 同梱）で動作。Windows は別モジュール `sherpa-onnx-go-windows`（未検証）
  - VAD（silero、最大 15 秒で強制分割）: 4 ターンの会話を正解どおりに 4 区間へ分割、30 秒の音声で 65ms
  - 話者分離（pyannote-3.0 + 3dspeaker / wespeaker）: 30 秒の音声で約 3 秒（約 10 倍速、1 時間で約 6 分）。**macOS TTS の 2 声（Kyoko / Reed）は分離できなかった**（人工音声は話者特徴が薄く、モデルの優劣は判定不能）
  - 残課題: 実際の 2 人音声で分離精度を確認する（2 段目の着手前）。ローカル分離の所要時間は録音時間の約 1/10 になるので、バックグラウンド処理前提
- [x] P4 malgo: 16kHz モノラル録音、固定署名でマイク権限ダイアログ
  - `github.com/gen2brain/malgo` v0.11.26。S16 / 16kHz / モノラルで 5.01 秒を録音、音量あり（無音ではない）。デバイス列挙も可
  - ターミナル経由の実行では権限が通った。**アプリ本体（固定署名の .app）でのマイク権限ダイアログは未確認** → Phase 2 の冒頭で確認（`Info.plist` の `NSMicrophoneUsageDescription` が mygo の設定で出せるかも含む）
- [x] P5 afconvert: 各形式 → 16kHz WAV、動画の可否
  - m4a / mp3 / aac / wav / caf / mp4 / mov すべて `afconvert in -f WAVE -d LEI16@16000 -c 1 out.wav` で変換できた。動画（mp4 / mov）も音声が取れる → 動画取り込みを MVP に前倒しできる
  - WAV → m4a は `afconvert in.wav -f m4af -d aac -b 32000 out.m4a`。`-c 1` を付けると失敗し、16kHz モノラルで 64kbps も失敗する。32kbps なら 1 時間約 14MB
- [x] P6 WebView での保存フォルダ内 m4a 再生手段
  - `mygo.Protocol.Handle("media", mygo.FileServer(os.DirFS(保存先)))` で配信し、`<audio src="media://localhost/…">` で再生する。FileServer は Range 対応でシークできる見込み（実機での確認は Phase 5 で行う）。Windows は `http://media.localhost/`
  - 付随して分かったこと: キーチェーンは `mygo.Secrets`、グローバルショートカットは `mygo.GlobalShortcut`、通知は `mygo.NewNotification`、ファイルドロップは `onFileDrop`（mygo-runtime）。`go-keyring` は不要
  - 分からないこと: マイク権限（P4 で実機確認）、サイドカー起動（標準の `os/exec` を使う）

## Phase 1: 土台

- [x] identifier を `one.nxeu.transflowy` に変更
- [x] React + Tailwind + shadcn/ui 導入、Greeter サンプル削除
- [x] 固定の自己署名証明書でのビルド手順
  - 用意済み: `bun run cert`（証明書作成。ログインキーチェーンに追加し、ログインパスワードを聞かれる）→ `bun run build:signed`。実行済み。ログインキーチェーンに `transflowy Local Signing` を作成し、`bun run build:signed` で署名・検証できた（designated requirement が identifier と証明書に固定）。マイク権限ダイアログの実機確認は Phase 2 で行う
  - 済み: Info.plist に `NSMicrophoneUsageDescription`（無いとマイクが黙って拒否される）、最低 macOS 14.2
- [x] データモデル（`meta.json` / `transcript.json`、`speaker` / `original` を含む）と録音フォルダの読み書き（`internal/library`、原子的書き込み、テストあり）
- [x] 設定の保存（保存先、既定プロバイダ、ローカル限定、ショートカット）と API キーのキーチェーン保存（`internal/settings`、`mygo.Secrets`。キーの値はページに返さない）

## Phase 2: 録音と取り込み

- [ ] malgo でマイク録音、数秒ごとの WAV 追記
- [ ] 停止後の m4a 変換（afconvert）
- [ ] ファイル取り込み（ドロップ → afconvert で 16kHz WAV 化）
- [ ] 動画ファイル（mp4 / mov）の取り込み
- [ ] グローバルショートカット `⌥⌘R`
- [ ] メニューバーアイコン（開始 / 停止、経過時間、ローカル限定の鍵マーク）
- [ ] 停止直後 5 秒の「取り消し」トースト

## Phase 3: 文字起こし

- [ ] VAD による無音区間チャンク化（録音中に実行）
- [ ] 処理キュー（チャンク単位、指数バックオフ、オフライン待機と自動再開）
- [ ] プロバイダ: OpenRouter（MAI-Transcribe-2）
- [ ] プロバイダ: OpenAI 互換 URL
- [ ] プロバイダ: whisper-server サイドカー（起動・停止・クラッシュ時再起動）
- [ ] ローカルモデル DL（進捗表示、裏で継続）
- [ ] 「ローカル限定」ガード（クラウド送信の唯一の出口で拒否）
- [ ] 固有名詞辞書の受け渡し（phraseList / initial prompt）
- [ ] 言語の自動判定と録音ごとの上書き

## Phase 4: 要約

- [ ] プロバイダ: OpenRouter / OpenAI 互換 URL、Ollama 自動検出
- [ ] 組み込みテンプレート 6 種とユーザーテンプレート読み込み
- [ ] 要約の自動実行、別テンプレートでの再生成、複数要約の保持
- [ ] タイトル自動生成
- [ ] 要約中のタイムスタンプ `[mm:ss]` の出力

## Phase 5: 画面

- [ ] 2 ペインレイアウト、日付グループの録音一覧、`⌘\` で折りたたみ
- [ ] 詳細: 要約と文字起こしの左右分割、1100px 未満でタブ
- [ ] 下部プレーヤー（wavesurfer.js、速度変更）
- [ ] 再生位置の行ハイライトと自動追従（TanStack Virtual）
- [ ] 要約の `[12:34]` クリック → 該当行へ移動して再生
- [ ] 状態バッジと処理完了通知
- [ ] 失敗バッジからの「ローカルで処理」
- [ ] 録音ごとの「再処理」（エンジン変更）
- [ ] `⌘K` 横断検索（cmdk）

## Phase 6: 編集と書き出し

- [ ] セグメントのインライン編集と元に戻す
- [ ] 一括置換と辞書追加の提案
- [ ] 要約の「再生成」バッジ
- [ ] クリップボードコピー（形式選択）
- [ ] .md / .txt / .srt 書き出し

## Phase 7: 初回起動

- [ ] 3 ステップのオンボーディング（権限 → エンジン → ショートカット）
- [ ] 準備前に録音したものを準備完了後に自動処理
- [ ] クラウド選択時の 10 秒試し録音

MVP 完了。

## 2 段目

- [ ] システム音声録音（mac: Core Audio Tap、別トラック保存）
- [ ] 会議自動検出と録音提案の通知
- [ ] 話者分離（クラウド / sherpa-onnx）と話者名の付け替え
- [ ] ライブ字幕
- [ ] テンプレート自動選択
- [ ] 録音内容への質問チャット
- [ ] 無音スキップ再生
- [ ] ローカル限定 ON 中のクラウド同期フォルダ警告
- [ ] 検索が遅くなったら SQLite FTS 索引
- [ ] Windows: WASAPI loopback、音声変換手段
- [ ] 公開配布: Developer ID 署名・公証、GitHub Releases、自動更新
