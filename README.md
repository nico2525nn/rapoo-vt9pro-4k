# Rapoo VT9 Pro 4K

Rapoo VT9 Pro 4K 用の Windows ツールと HID プロトコル実装。

## ダウンロード

[Releases](https://github.com/nico2525nn/rapoo-vt9pro-4k/releases)

- `rapoo-control-v1.0.0-windows-amd64.exe` — Wails 設定アプリ（Windows）
- `rapoo-control-v1.0.0-linux-amd64` — Wails 設定アプリ（Linux）
- `rapoo-control-winui-v1.0.0-windows-amd64.zip` — WinUI 3 ネイティブアプリ一式（Windows）
- `vt9tray-v1.0.0-windows-amd64.exe` — タスクトレイ電池表示
- `vt9ctl-v1.0.0-windows-amd64.exe` / `vt9ctl-v1.0.0-linux-amd64` — CLI
- `rapoo-bridge-v1.0.0-windows-amd64.exe` — WinUI 用 HID sidecar（単体）

## 使い方

- `vt9tray.exe`: 起動するだけ。右クリックで更新モードと終了。
- `rapoo-control.exe`: 左サイドバーから DPI、センサー、ボタン、プロファイル、設定を変更。
- `vt9ctl.exe`: `list` / `battery` / `info` / `watch`

対応機能: DPI、ポーリングレート、LOD、Motion Sync、デバウンス、10ボタン、プリセット、ゲーム中ロック (0-TX)、デモモード。

### WinUI 3 版

`rapoo-control-winui-v1.0.0-windows-amd64.zip` を展開し、`RapooControl.exe` を起動してください。同じフォルダの `rapoo-bridge.exe` が HID を担当します。

- 全ページが実機の設定を読み書きします（DPI、センサー、10ボタン、プロファイル、設定）。
- ファームウェア更新は表示のみで、公式 Rapoo ソフトの起動だけを行います。本アプリは書き込みません。
- WinUI 版はサイドバー右下の「再読み込み」「適用」で変更をまとめて反映します。
- HID を開けるのは 1 プロセスだけです。WinUI 版と Wails 版 / トレイを同時に起動しないでください。

## UI

- Linux / Windows Wails UI: React 19 + Tailwind CSS v4 + 現行 shadcn/ui
- Windows ネイティブ: `winui/RapooControl`（WinUI 3 / Windows App SDK、Go sidecar 経由で実機接続）

Wails と WinUI は同じ WinUI 3 の配色トークン（背景・コンテンツ・カード・アクセント）を使用します。

## ビルド

```bash
go test ./...

# Windows トレイ / CLI
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-H=windowsgui -s -w" -o dist/vt9tray.exe ./cmd/vt9tray
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl.exe ./cmd/vt9ctl
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl ./cmd/vt9ctl

# WinUI 用 sidecar
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o build/bin/rapoo-bridge.exe ./cmd/rapoo-bridge

# Wails アプリ
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -platform windows/amd64
wails build -platform linux/amd64 -tags webkit2_41

# フロントエンド
cd frontend && npm ci && npm test && npm run build

# WinUI 3（Windows + .NET 9 SDK）
dotnet build winui/RapooControl/RapooControl.csproj -c Release -p:Platform=x64
```

`RapooControl.csproj` は `build/bin/rapoo-bridge.exe` が存在する場合、ビルド出力へコピーします。

## トラブル時

- 認識しない: 4K レシーバーまたは USB-C ケーブルを確認する。
- 設定を変更できない: ゲーム中ロック (0-TX) を解除する。
- Linux で権限エラー: hidraw の udev ルールを設定する。

  ```udev
  SUBSYSTEM=="hidraw", ATTRS{idVendor}=="24ae", MODE="0660", TAG+="uaccess"
  ```

- ファームウェア更新: 本アプリは書き込まない。公式 Rapoo ソフトで実行する。

## プロトコル

[docs/PROTOCOL.md](docs/PROTOCOL.md)

## ライセンス

MIT
