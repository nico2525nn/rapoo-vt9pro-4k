# Rapoo 更新サーバー調査メモ

最終更新: 2026-11-22

## 確認できたこと

公式 `RapooGameDev` は次の API を使用しています。

- ベース URL: `http://api.driver.rapoo.cn`
- ドライバ/バージョン確認: `POST /v1/driver/version`
- クラウド設定: `/v1/driver/config2/get`, `/v1/driver/config2/save`
- マクロ設定: `/v1/driver/macro/config/get`, `/v1/driver/macro/config/save`
- ユーザー: `/v1/user/*`
- アップロード: `/v1/file/upload`

`POST /v1/driver/version` はパラメータ不足時に `{"code":7,"data":null,"msg":"缺少必要的参数"}` を返します。署名付きリクエストが必要で、パラメータ仕様は未解析です。

## ファームウェアパッケージ

公式ソフトのインストール先に以下が同梱されています。

```text
C:\Program Files (x86)\Rapoo\RapooGameDev\Update\Local\
├── vt9pro\VT9PRO_Update.zip
├── V+4K\4K_Update.zip
└── V+Dongle\Dongle_Update.zip
```

各 ZIP は `manifest.json`, `.bin`, `.dat` の Nordic DFU 形式です。
`Update\Clouds` は公式アプリが取得したパッケージのキャッシュです。

## 未解決

- `api.driver.rapoo.cn` の署名パラメータ（timestamp/nonce/sign 等）が未特定。
- `rapooweb` S3 バケットは一覧取得不可、ファームウェア直リンクの推測は 403。
- 公式アプリを SSH から起動しても、セッション 0 のため更新チェック通信を再現できなかった。

## 次の手順

1. Windows のデスクトップセッションで公式アプリの「Check for updates」を実行する。
2. `pktmon` または Fiddler で `api.driver.rapoo.cn` へのリクエストを採取する。
3. 署名パラメータとファームウェア URL を特定する。
4. ダウンロードのみを自動化し、書き込みは公式アップデータに渡す。

本アプリは現時点でもファームウェアのバージョン表示と公式アップデータ起動まで対応しています。実書き込みは行いません。
