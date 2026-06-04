# Claude Implementation Spec

## Objective

`iso8583tool` を、BASE I 向けの使いやすい `viewer`, `writer`, `validator` として実装する。

最優先は「最初の 5 分で触れること」であり、完全な BASE I private field 実装ではない。

Claude が実装時に守るべき判断は次の 3 つ。

1. まずメッセージを見られること
2. まずサンプルで往復できること
3. private field を誤って作り込み過ぎないこと

## Scope

### In Scope

- `moov-io/iso8583` を使った pack / unpack
- built-in preset `basei-starter`
- `view`, `write`, `validate`, `sample`, `init`, `version`
- テスト用 BASE I fixture の同梱
- Field 55 の EMV TLV 対応
- unknown TLV tag の保持と通知

### Out of Scope

- BASE I 全 private field の完全な構造化
- TUI / GUI editor
- socket client / server
- partner profile の自動選択
- Field 127 の完全な nested bitmap 実装

## UX Principles

### 1. Project init なしでも触れる

ユーザーはリポジトリ clone 直後に次を試せるべき。

```shell
go test ./...
go run . sample
go run . view --file examples/basei/0100-auth-request.hex
go run . write --input examples/basei/0100-auth-request.json
```

`iso8583tool.toml` が無くても `basei-starter` を使って動作すること。

### 2. コマンドはメッセージ単位

コマンドは spec 編集中心ではなく、メッセージ操作中心にする。

- `view`: raw message を読む
- `write`: JSON message document から packed message を作る
- `validate`: unpack 可否、unknown TLV、extension strategy を確認する
- `sample`: fixture を列挙・書き出しする
- `init`: workspace を初期化する

### 3. nested field は path で統一する

CLI と JSON 入力の nested field 表現はすべて dot-path に統一する。

例:

- `48`
- `48.1`
- `55.9F02`
- `127.25.1`

これで private field の実装が育っても CLI 契約を壊さない。

## Command Contract

### `view`

目的:

- packed message を human-readable に見る
- extension field の扱いを同時に把握する

入力:

- `--file PATH` または `--raw DATA`
- `--encoding hex|raw` デフォルト `hex`
- `--format describe|json` デフォルト `describe`
- `--spec PATH` は任意

出力要件:

- `describe` では MTI, bitmap, fields を見やすく表示
- active な extension field の strategy を後段に表示
- unknown TLV tag があれば一覧表示

### `write`

目的:

- JSON message document から packed message を生成する

入力:

- `--input PATH` 必須
- `--output PATH` 任意
- `--encoding hex|raw` デフォルト `hex`
- `--spec PATH` 任意

出力要件:

- `--output` なしなら stdout
- `hex` は大文字
- path ベース入力を受け付ける

### `validate`

目的:

- 「unpack できるか」「unknown TLV があるか」「extension field をどう扱うか」を一度に確認する

入力:

- `--file PATH` または `--raw DATA`
- `--encoding hex|raw`
- `--format text|json`
- `--spec PATH` 任意

出力要件:

- `Spec`
- `MTI`
- `Issues`
- `Extension Field Strategy`
- `Unknown TLV Tags`

終了コード:

- unpack/validation error があれば `1`
- warning のみなら `0`

### `sample`

目的:

- fixture を first-class に扱う

入力:

- 引数なし: sample 一覧
- `--name SAMPLE`
- `--format json|hex`
- `--output PATH`

サンプル要件:

- `0100-auth-request`
- `0110-auth-response`

### `init`

目的:

- workspace と初期 fixture をまとめて作る

出力物:

- `iso8583tool.toml`
- `specs/extensions.json`
- `examples/basei/*.json`
- `examples/basei/*.hex`
- `messages/`

## Spec Resolution

spec 解決順序は次の通り。

1. CLI `--spec`
2. `iso8583tool.toml` の `message_spec`
3. built-in `basei-starter`

`basei-starter` は「ASCII 1987 ベース + Field 55 TLV 拡張」とする。

## BASE I Starter Definition

### Field 55

Field 55 は BER-TLV composite として扱う。

最低限サポートする known tag:

- `5F2A`
- `82`
- `84`
- `8A`
- `91`
- `95`
- `9A`
- `9C`
- `9F02`
- `9F03`
- `9F09`
- `9F10`
- `9F1A`
- `9F1E`
- `9F26`
- `9F27`
- `9F33`
- `9F34`
- `9F35`
- `9F36`
- `9F37`
- `9F41`
- `71`
- `72`

挙動要件:

- known tag は decode / encode できる
- unknown tag は unpack failure にしない
- unknown tag は round-trip で保持する
- `view` / `validate` で unknown tag を表示する

### Field 48

- まず raw string
- strategy metadata は `positional`
- 将来 `48.1`, `48.2` に昇格できる前提

### Field 62

- まず raw string
- strategy metadata は `positional`

### Field 63

- まず raw string
- strategy metadata は `opaque`

### Field 127

- metadata 上は `bitmap`
- v1 では本格実装しない

## Message Document Format

JSON 形式:

```json
{
  "mti": "0100",
  "fields": {
    "2": "4761739001010010",
    "48": "LOYALTY=OFF|INSTALLMENT=00",
    "62": "ORDERID=000123|CHANNEL=ECOM"
  },
  "binary_fields": {
    "55.9F02": "000000005000",
    "55.9F36": "0034"
  }
}
```

ルール:

- `fields` はテキスト系 / 通常系
- `binary_fields` は hex string
- key は field path
- `mti` は必須

## Test Fixtures

`examples/basei/` に次の 4 ファイルを置く。

- `0100-auth-request.json`
- `0100-auth-request.hex`
- `0110-auth-response.json`
- `0110-auth-response.hex`

### 0100 Request Requirements

含めるべき field:

- `2`, `3`, `4`, `7`, `11`, `12`, `13`, `14`
- `18`, `22`, `23`, `24`, `25`
- `35`, `37`, `41`, `42`, `49`
- `48`, `55`, `62`

目的:

- purchase authorization request
- Field 55 TLV
- Field 48/62 private payload

### 0110 Response Requirements

含めるべき field:

- `2`, `3`, `4`, `7`, `11`, `12`, `13`
- `37`, `38`, `39`, `41`, `42`, `49`
- `48`, `55`, `63`

目的:

- authorization response
- issuer data in Field 55
- opaque private payload in Field 63

## Validation Rules

v1 の validate は厳しすぎないこと。

error:

- unpack 失敗
- MTI が取得できない
- spec で定義される field path が不正

warning:

- unknown TLV tag
- opaque strategy field が入っている

## Reference Inputs

Claude は必要なら `doc/reference/` を見ること。

参考にする意図:

- `doc/reference/moov-io-iso8583`
  - `cmd/iso8583/main.go`
  - `unknown_tags.go`
  - `exp/emv/spec.go`
- `doc/reference/pyiso8583`
  - `iso8583/tools.py`
- `doc/reference/jpos`
  - `jpos/src/main/resources/packager/cmfv3.xml`
  - `jpos/src/main/resources/packager/iso87ascii.xml`

ただしこれらは参考のみで、そのまま設計を移植しない。

## Git Hygiene

`doc/reference/` は必ず Git 管理対象外にすること。

期待状態:

```gitignore
doc/reference/
```

reference clone は開発者ローカル用途のみ。

## Acceptance Criteria

Claude 実装完了の判定は次。

1. `go test ./...` が通る
2. `go run . sample` で sample 一覧が出る
3. `go run . sample --name 0100-auth-request --format hex` が成功する
4. `go run . view --file examples/basei/0100-auth-request.hex` が成功する
5. `go run . validate --file examples/basei/0110-auth-response.hex` が成功する
6. `go run . write --input examples/basei/0100-auth-request.json` が成功する
7. unknown TLV tag を含む Field 55 で warning を出しつつ round-trip できる

## Implementation Order

Claude はこの順で作るのがよい。

1. `basei-starter` spec
2. JSON message document contract
3. `write` / `view` / `validate`
4. `sample`
5. `init`
6. unknown TLV preservation
7. tests and fixtures
