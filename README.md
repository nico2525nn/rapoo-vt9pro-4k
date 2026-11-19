# Rapoo VT9 Pro 4K

Rapoo VT9 Pro 4K 用の Windows ツールと HID プロトコル実装。

## ダウンロード

[Releases](https://github.com/nico2525nn/rapoo-vt9pro-4k/releases)

- `rapoo-control.exe` — 設定アプリ
- `vt9tray.exe` — タスクトレイ電池表示
- `vt9ctl.exe` — CLI

## 使い方

- `vt9tray.exe`: 起動するだけ。右クリックで更新モードと終了。
- `rapoo-control.exe`: 左サイドバーから DPI、センサー、ボタン、プリセットを設定。
- `vt9ctl.exe`: `list` / `battery` / `info` / `watch`

対応機能: DPI、ポーリングレート、LOD、Motion Sync、デバウンス、10ボタン、プリセット、ゲーム中ロック (0-TX)、デモモード。

## ビルド

```bash
go test ./...

# Windows トレイ / CLI
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-H=windowsgui -s -w" -o dist/vt9tray.exe ./cmd/vt9tray
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/vt9ctl.exe ./cmd/vt9ctl

# Wails アプリ
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails build -platform windows/amd64
wails build -platform linux/amd64 -tags webkit2_41

# フロントエンド
cd frontend && npm ci && npm test && npm run build
```

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
