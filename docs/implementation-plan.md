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

- [x] i18n 基盤（追加）: `locales/ja.json`（ソース言語 = 日本語）、`mygo generate` が `locales/catalog_gen.go` と `src/locales.ts` を生成。Go は `locales.Xxx()`、画面は `useMessages()`。メニューバーは言語変更時に再設定。**文言は今後ここに追加する**（直書きしない）。他言語は `locales/<tag>.json` を足すだけ（未訳は日本語に fall back）。未対応: マイク権限の説明文（Info.plist）の多言語化は `resources/darwin/<lang>.lproj/InfoPlist.strings` で行う

## Phase 2: 録音と取り込み

- [x] malgo でマイク録音、数秒ごとの WAV 追記（2 秒ごとに追記・ヘッダ更新。起動時に前回クラッシュした録音を復旧）
  - **`mygo build` は既定で cgo 無効** → malgo が未定義でビルド失敗。`CGO_ENABLED=1` を dev / build / generate のスクリプトに設定済み。Windows は C コンパイラが要る（Windows 着手時に確認）
  - 録音中は `recording.wav` を残す（Phase 3 の処理で使う。処理完了後に削除する）
- [x] 停止後の m4a 変換（afconvert、`audio.m4a`。アプリ終了時・クラッシュ時にも変換を完了させる）
- [x] ファイル取り込み（ドロップ → afconvert で 16kHz WAV 化）（`Importer`。経路: ウィンドウへのドロップ、Dock アイコン / 「このアプリで開く」、ボタンのファイル選択。`open -a` で wav と mp4 の取り込みを確認、対象外の拡張子は無視して通知。ウィンドウへのドロップと Dock 上のドロップ自体は未確認）
- [x] 動画ファイル（mp4 / mov）の取り込み（mp4 で確認、音声のみ抽出）
- [x] グローバルショートカット `⌥⌘R`（設定の `shortcut`。起動時にエラーなく登録できることを確認、押下での動作は未確認）
- [x] メニューバーアイコン（開始 / 停止、経過時間。窓を閉じても録音継続）。残り: ローカル限定の鍵マーク（Phase 3 のガードと同時）
- [x] 停止直後 5 秒の「取り消し」トースト（直前に止めた録音だけ削除可）

## Phase 3: 文字起こし

- [x] 無音区間チャンク化（`transcribe.Plan`、録音中も実行）。**VAD は純 Go のエネルギー判定**（silero は話者分離の段階で導入。依存の dylib を .app に同梱する手間を避けた）
  - 最短 4 秒・最長 25 秒（ローカル）/ 5〜10 分（クラウド）。無音が無ければ直前 5 秒で最も静かな所で切る
  - 実機検証で判明: 言語は窓ごとに 1 つしか判定されないため、窓に別言語が混ざると片方が落ちる。**窓の末尾 3 秒以上が未認識で、その部分に音声があれば、そこから切り直して再送**する（結果が 1 秒未満なら追わない）
  - 無音だけのチャンクは送らない（幻覚防止）。afconvert の WAV は 4KB のパディングチャンクがあるため、データ位置はチャンクをたどって探す（44 バイト固定は誤り）
- [x] 処理キュー（`internal/pipeline`。チャンクごとに保存、再起動で続きから再開。一時的な失敗は最大 5 回、オフラインは回数に数えず待機。401/403 は即失敗、キー・モデル未準備と「ローカル限定 × クラウド」は失敗にせず待機（設定変更・DL 完了で自動再開）。録音中から先行処理、停止後に続行）
- [x] プロバイダ: OpenRouter（MAI-Transcribe-2）（実 API で確認: 日英混在、話者 0/1、文単位の行分割。英語の単語間の空白を補う）
- [x] プロバイダ: OpenAI 互換 URL（multipart。単体テストのみ、実サーバは未確認）
- [x] プロバイダ: whisper-server サイドカー（起動・5 分アイドルで停止・次回呼び出しで再起動。Homebrew 版を使用、**配布用の同梱は未対応**: 公開配布の段階で .app に入れる）
- [x] ローカルモデル DL（`internal/models`。途中再開、SHA-256 照合、`Models` サービス + 進捗イベント。単体テストのみ、**実際の Hugging Face からの DL は未確認**）
- [x] 「ローカル限定」ガード（`internal/netguard`。録音・文字起こしを送る全リクエストがこのクライアントを通り、ループバック以外を拒否。メニューバーに鍵マーク）。要約の送信（Phase 4）も同じクライアントを使うこと
- [x] 固有名詞辞書の受け渡し（ライブラリ直下の `dictionary.txt`、1 行 1 語、`#` はコメント。管理 UI は Phase 6）
- [x] 言語の自動判定と録音ごとの上書き（`Transcriber.SetLanguage`、UI は Phase 5）

**Phase 3 で見つかった追加タスク**

- [ ] 設定画面（エンジン選択、API キー、OpenAI 互換 URL、保存先、ローカル限定トグル）。いまは `settings.json` の手編集か、開発時の `.env`（`OPENROUTER_API_KEY`）でしか設定できない → Phase 5 で作る
- [ ] whisper-server と依存ライブラリを .app に同梱し署名（公開配布の段階）
- [ ] 実際の録音（マイク）での先行処理と、実モデル DL の動作確認

## Phase 4: 要約

- [x] プロバイダ: OpenRouter / OpenAI 互換 URL、Ollama 自動検出（`summarize.Chat` が `/chat/completions` を呼ぶ。送信は `netguard` のクライアント経由。Ollama は `Summarizer.DetectOllama` がモデル一覧を返し、画面が openai-compat の `http://localhost:11434/v1` として設定する。モデルの既定は空で、未選択は待機）
  - 実 API（OpenRouter、`anthropic/claude-haiku-4.5`）で、合成の文字起こしからタイトル行・`[mm:ss]`・節構成が出ることを確認。Ollama と OpenAI 互換の実サーバは未確認
- [x] 組み込みテンプレート 6 種とユーザーテンプレート読み込み（`internal/summarize/templates/*.md` を埋め込み。ユーザー分は `<保存先>/templates/*.md`、同名は置き換え。frontmatter は `name` / `description`）
- [x] 要約の自動実行、別テンプレートでの再生成、複数要約の保持（文字起こし完了で `Settings.Template`（既定「汎用メモ」）を自動実行。`Summarizer.Summarize(id, template)` で再生成。状態 `summarizing` = 要約待ち。失敗は文字起こしを `done` のまま `meta.error` に出す）
- [x] タイトル自動生成（要約と同時。`meta.title` が空のときだけ設定し、取り込みやユーザーが付けた題は上書きしない）
- [x] 要約中のタイムスタンプ `[mm:ss]` の出力（文字起こしを `[mm:ss] 話者N: 本文` で渡し、根拠の時刻を行末に付けさせる。1 時間超は `[75:00]`）

**Phase 4 の制約・追加タスク**

- 長い文字起こしは分割せず 1 回で送る。モデルの文脈長を超える長時間録音は失敗する（分割要約は必要になったら足す）
- 文字起こしの再処理で要約も既定テンプレートで作り直される（他テンプレートの要約は古いまま残る → Phase 6 の「再生成」バッジ）
- テンプレート選択・要約表示の画面は Phase 5。設定画面（要約モデル・テンプレート）も Phase 5

## Phase 5: 画面

画面はモック runtime（vite）でブラウザ描画して確認した。**実アプリ（WebView）での動作は未確認**: `media://` の再生・シーク、ウィンドウ幅 1100px 未満のタブ切替、通知、フォルダ選択ダイアログ。

- [x] 2 ペインレイアウト、日付グループの録音一覧、`⌘\` で折りたたみ
- [x] 詳細: 要約と文字起こしの左右分割、1100px 未満でタブ（要約は小さな Markdown 描画。表・リンク・入れ子リストは未対応）
- [x] 下部プレーヤー（wavesurfer.js、速度変更。`media://localhost/<id>/audio.m4a` を再生し、`peaks.json` が無い古い録音は平坦な波形）
- [x] 波形: 処理完了時に `recording.wav` から約 2,000 点のピークを `peaks.json` へ保存（wav を消す前）。録音中のライブ波形は `level` イベントから描く
- [x] 再生位置の行ハイライトと自動追従（TanStack Virtual。ホイール操作で追従を切り、「再生位置へ戻る」で復帰）
- [x] 要約の `[12:34]` クリック → 該当行へ移動して再生
- [x] 状態バッジと処理完了通知（要約完成時に通知。通知のクリックでウィンドウを開く動作は未対応）
- [x] 失敗バッジからの「ローカルで処理」
- [x] 録音ごとの「再処理」（エンジン選択）、録音ごとの言語の上書き
- [x] `⌘K` 横断検索（cmdk。検索は Go が毎回ファイルを走査する。千件を超えて遅くなったら索引）
- [x] 設定画面（Phase 3 から繰り越し）: ローカル限定、保存先、文字起こし / 要約のエンジン・モデル・URL、Ollama 検出、既定テンプレート、API キー（値は表示しない）。保存先の変更は既存録音を移動しない
- [x] 入力デバイスの選択（設定画面。名前で保存し、未接続・未選択はシステム既定で録音。実機で列挙は確認、選んだデバイスでの録音は未確認）

**Phase 5 で見つかったこと**

- 古い `Meta.title` が空の録音は一覧で時刻のみ表示（日付グループの下）。タイトル編集は Phase 6
- 検索ダイアログは開いた直後の入力が落ちることがあった（フォーカスのタイミング。要実機確認）

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
