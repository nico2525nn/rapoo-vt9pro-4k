# Rapoo VT9 Pro 4K — Windows Battery Tray & Protocol SDK (`rapoo-vt9pro-4k`)

**Rapoo VT9 Pro 4K**（4Kワイヤレスドングル `0x24AE:0x1211` / USB-C有線 `0x24AE:0x4411`）のバッテリー残量を Windows のタスクトレイに**くっきりとした数字＋残量バー**で常時表示する超軽量ネイティブ Go アプリ（`vt9tray.exe`）と、将来的な **Wails 製クロスプラットフォーム制御アプリ（Windows / Linux 両対応）** を見据えたプロトコル SDK & CLI（`vt9ctl`）です。

- **外部ライブラリ・CGO 依存ゼロ**（標準ライブラリ＋OS ネイティブ API のみ、単一 `.exe` 約 2MB）
- **ゲーム中のマウス操作に一切影響しない設計**:
  - 4000Hz のマウス入力インターフェース（`MI_00`）には**列挙時のメタデータ取得を含め一切アクセスしません**（パス文字列の段階で `MI_01` のみをフィルタ）。
  - `FILE_SHARE_READ | FILE_SHARE_WRITE` で開くため、公式ソフト（`RapooGameDev.exe` / `A-HUB`）を起動したままでも競合せず共存可能です。
  - 右クリックメニューまたは `-passive` フラグで **「ゲーム優先・完全受動モード（送信パケットゼロ、COL05 の非同期 `0xBB` 通知のみ受信）」** にいつでも切り替えられます。

---

## ディレクトリ構成（Wails 移植を見据えたレイヤ分離）

```text
rapoo-vt9pro-4k/
├── cmd/
│   ├── vt9tray/          # Windows 常駐タスクトレイアプリ (Win32 Shell_NotifyIconW, HiDPI 対応)
│   └── vt9ctl/           # Windows / Linux 両対応 CLI ツール
├── internal/
│   ├── icon/             # 純 Go ピクセルアライン数字アイコン描画エンジン (16/20/24/32px HiDPI)
│   ├── service/          # Wails にそのまま Bind 可能な MouseService (状態管理・自動再接続・スリープ保持)
│   └── tray/             # Windows ネイティブトレイ UI (USB ホットプラグ・Explorer 再起動復旧・自動起動登録)
├── pkg/
│   ├── hid/              # CGO 不要の OS 抽象 HID レイヤ (Windows SetupAPI/Overlapped & Linux hidraw)
│   └── rapoo/            # Rapoo Realtek 4K (Chip 0x0E) プロトコル実装・ドメインモデル
└── docs/
    └── PROTOCOL.md       # 公式ドライバ (RapooDevice.dll) から解析した完全なバイト単位プロトコル仕様書
```

プロトコルの詳細（`0xBA` コマンドフレーム、`0xBB`/`0xBD` パッシブ通知、オンボードメモリマップ `0x0880`〜`0x2200` 等）は [docs/PROTOCOL.md](docs/PROTOCOL.md) にまとめています。

---

## 使い方

### 1. Windows タスクトレイアプリ (`vt9tray.exe`)
`vt9tray.exe` を起動するだけで、タスクトレイに現在のバッテリー容量が数字（`0`〜`100`）で表示されます。
（※ Windows 11 では初回のみタスクトレイ右端の `^`（隠れているインジケーター）からタスクバーへドラッグすると常時表示になります）

- **左クリック**: 今すぐバッテリー残量を再取得
- **右クリックメニュー**:
  - デバイス名・接続方式（`4K無線` / `USB有線`）・充電状態・最終更新時刻の表示
  - **通常モード（60秒更新 + 自動通知）**
  - **低頻度モード（5分更新 + 自動通知）**
  - **ゲーム優先・完全受動（送信ゼロ・通知のみ）**
  - **Windows 起動時に自動起動**（`HKCU\...\Run` レジストリの ON/OFF）
  - **終了**
- **スマートな挙動**:
  - ワイヤレスマウスが省電力スリープに入っても直前のバッテリー％を消さずに保持し、スリープ色（落ち着いたグレー）で表示します。
  - 4K ドングルを挿したまま USB-C ケーブルを接続して有線充電プレイに切り替えた場合、自動的に有線側（`0x4411`）を優先して充電状態を反映します。

### 2. CLI ツール (`vt9ctl` / `vt9ctl.exe`)
```powershell
# 接続中のベンダ HID エンドポイント確認（マウスへの送信ゼロ）
.\vt9ctl.exe list

# バッテリー残量の単発取得
.\vt9ctl.exe battery

# 完全受動（送信ゼロ）で 0xBB / 0xBD 非同期イベントをリアルタイム監視
.\vt9ctl.exe watch

# FW バージョン・DPI ステージ・ポーリングレート・LOD・MotionSync・スリープ時間の一括確認
.\vt9ctl.exe info
```

---

## ビルド方法（Linux からのクロスコンパイル / Windows ネイティブ共通）

```bash
# ユニットテスト実行
go test -v ./...

# Windows 用タスクトレイアプリ (コンソール非表示 GUI バイナリ)
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-H=windowsgui -s -w" -o dist/vt9tray.exe ./cmd/vt9tray

# Windows 用 CLI
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o dist/vt9ctl.exe ./cmd/vt9ctl

# Linux 用 CLI
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl ./cmd/vt9ctl
```

> **Linux で使う場合の udev ルール例** (`/etc/udev/rules.d/99-rapoo.rules`):
> ```udev
> SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"
> ```

---

## 将来の Wails アプリへの組み込みイメージ

`internal/service.MouseService` と `pkg/rapoo` は UI から完全に独立しており、Wails の `Bind` にそのまま渡せる構造になっています。

```go
mouseSvc := service.NewMouseService(nil, service.Config{
    OnEventEmit: func(name string, payload any) {
        runtime.EventsEmit(appCtx, name, payload)
    },
})

err := wails.Run(&options.App{
    Title:     "Rapoo Control",
    OnStartup: func(ctx context.Context) { appCtx = ctx; mouseSvc.Startup(ctx) },
    OnShutdown: func(ctx context.Context) { mouseSvc.Shutdown() },
    Bind:      []any{mouseSvc},
})
```
