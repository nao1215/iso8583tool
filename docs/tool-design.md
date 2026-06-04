# Tool Design

## Goal

`iso8583tool` の最優先は「BASE I を触る最初の 5 分が軽いこと」です。

そのために v1 では次の 4 つだけを強くします。

1. すぐ見られる
2. すぐ書ける
3. すぐ検証できる
4. private field で破綻しない

## Usability Decisions

### 1. CLI はメッセージ単位で完結させる

主要コマンドは `view`, `write`, `validate`, `sample`, `init` です。

- `view`: 人が読むための出力
- `write`: JSON から packed message を作る
- `validate`: unpack と extension strategy をまとめて確認する
- `sample`: テストデータをすぐ取り出す
- `init`: 新しい作業ディレクトリを最短で作る

この形にした理由は、spec 操作より先に「メッセージを触る」需要が来るからです。

### 2. 入力契約は path ベースで固定する

private field をいきなり完全モデル化しない代わりに、入力の形だけ先に固定します。

- top-level: `48`, `55`, `62`
- positional: `48.1`, `62.3`
- TLV: `55.9F02`, `55.9F36`
- nested bitmap: `127.25.1`

これで spec が育っても CLI を壊しません。

### 3. unknown TLV は失敗より保存を優先する

Field 55 は実運用で「未知タグを消さない」ことの方が重要です。

そのため `basei-starter` は:

- known tag は decode する
- unknown tag は warning として見せる
- round-trip では保持する

を基本動作にしています。

### 4. サンプルを first-class にする

`sample` コマンドと `examples/basei` を用意し、最初から次を揃えています。

- `0100` authorization request
- `0110` authorization response
- JSON
- packed hex

「まず 1 本 unpack したい」「JSON を直して pack し直したい」をすぐ試せます。

## BASE I Strategy

`basei-starter` はあえて限定的です。

- Field 55 は EMV TLV として扱う
- Field 48 / 62 は raw だが positional に昇格しやすい前提で扱う
- Field 63 は opaque
- Field 127 はまだ metadata のみ

これは private grammar が未確定な状態で、見かけ上の完成度だけ高い誤仕様を入れないためです。

## Reference Survey

ローカルに clone した参照実装は `doc/reference/` 配下に置いています。ここは `.gitignore` でコミット対象から外しています。

見たポイントは次の通りです。

- `moov-io/iso8583`
  - path marshaling
  - built-in describe output
  - unknown TLV tag preservation
- `pyiso8583`
  - spec 駆動の encode/decode
  - pretty-print を前提にした出力
- `jPOS`
  - external packager 定義
  - subfield / dataset / ICC data の分離

この比較から、`iso8583tool` では次を採用しました。

- message 操作中心の CLI
- external spec 差し替え前提
- nested field を path で表す統一 contract
- unknown private data を落とさない挙動

## Non-Goals

今はまだやらないこと:

- BASE I 全 private field の完全な構造化
- interactive editor / TUI
- network channel / socket client
- partner ごとの proprietary profile 自動切り替え

## Next Steps

次に進めるなら順番はこれです。

1. 実ネットワークの BASE I spec を JSON 化する
2. Field 48 / 62 / 127 のどれを positional / bitmap に昇格させるか決める
3. `edit` コマンドか TUI を入れて subfield 操作を対話的にする
