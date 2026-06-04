# iso8583tool

`iso8583tool` is a usability-first scaffold for a BASE I oriented ISO 8583 `viewer`, `writer`, and `validator`.

![demo](./docs/demo.gif)

> デモの再生成: `make build && vhs docs/demo.tape`（[charmbracelet/vhs](https://github.com/charmbracelet/vhs) が必要）

現在のデフォルトは `basei-starter` です。これは `moov-io/iso8583` の ASCII 1987 spec を土台にしつつ、BASE I で実務上重要になりやすい部分だけ先に扱います。

- Field 55: EMV TLV として閲覧・生成
- Field 48 / 62: 将来の positional overlay を見越して path ベースで拡張
- Field 63: まず opaque のまま保持
- `sample` コマンドと `examples/basei` で、最初からテスト用メッセージを触れる
- `view` / `validate` の出力はカラー表示し、数値コードを意味に変換する（MTI・応答コード・通貨・POS entry mode・EMV タグなど）

数値→意味の変換例:

```text
MTI..........: 0100  → Authorization Request from Acquirer (ISO8583:1987)
F39  Response Code.....: 00  → Approved
F49  Transaction Currency Code..: 392  → JPY (Japanese yen)
F9F27 Cryptogram Information Data..: 80  → ARQC (online authorization requested)
```

色は端末では自動で有効になり、パイプやリダイレクト時は無効になります。`--color auto|always|never` で上書きでき、`NO_COLOR` 環境変数も尊重します。`--format json` は常に色なしで `decoded` 配列に意味を含めます。

## Quick Start

```shell
go test ./...
go run . sample
```

テスト用 BASE I サンプルを JSON で見る:

```shell
go run . sample --name 0100-auth-request
go run . sample --name 0110-auth-response
```

Packed hex を出す:

```shell
go run . sample --name 0100-auth-request --format hex
```

同梱サンプルを検査する:

```shell
go run . view --file examples/basei/0100-auth-request.hex
go run . validate --file examples/basei/0110-auth-response.hex
```

JSON からメッセージを組み立てる:

```shell
go run . write --input examples/basei/0100-auth-request.json
```

## Message Document Format

固定長や通常の可変長フィールドは `fields`、EMV などのバイナリ系は `binary_fields` に分けます。

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

path 形式を使うので、将来 private field を本格的に subfield 化しても、CLI の入力契約は変えずに済みます。

## Included BASE I Samples

- [0100-auth-request.json](./examples/basei/0100-auth-request.json)
- [0100-auth-request.hex](./examples/basei/0100-auth-request.hex)
- [0110-auth-response.json](./examples/basei/0110-auth-response.json)
- [0110-auth-response.hex](./examples/basei/0110-auth-response.hex)
- [0100-auth-request-unknown-tlv.json](./examples/basei/0100-auth-request-unknown-tlv.json)
- [0100-auth-request-unknown-tlv.hex](./examples/basei/0100-auth-request-unknown-tlv.hex)

これらは Field 48 / 55 / 62 / 63 を含むので、BASE I 向けの viewer / writer / validator の最初の回帰テストとして使えます。

`0100-auth-request-unknown-tlv` は Field 55 に既知タグと未知タグ (`DF8129`) を混在させたサンプルです。`view` / `validate` が未知タグを warning として通知しつつ、unpack → re-pack で落とさずに保持することを確認できます。

```shell
go run . view --file examples/basei/0100-auth-request-unknown-tlv.hex
go run . validate --file examples/basei/0100-auth-request-unknown-tlv.hex
```

## Config

`iso8583tool.toml` で spec を差し替えられます。

```toml
[spec]
preset = "basei-starter"
message_spec = "./specs/basei.json"
extension_catalog = "./specs/extensions.json"
```

実際のネットワーク仕様が固まったら、ここに JSON spec を差して `basei-starter` を卒業させる想定です。

## Notes

- `view` と `validate` は unknown TLV tag を検出すると、落とさずに通知します
- 参考実装をローカル確認するための clone は `doc/reference/` 配下に置き、Git のコミット対象から外しています

実装仕様として渡すなら [docs/claude-implementation-spec.md](./docs/claude-implementation-spec.md) を使い、設計意図の補足として [docs/tool-design.md](./docs/tool-design.md) と [docs/extension-field-strategy.md](./docs/extension-field-strategy.md) を参照してください。
