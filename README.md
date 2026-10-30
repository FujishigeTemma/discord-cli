# discord

AIエージェントから使うための、ミニマルなDiscord CLIです。対象は1つのサーバーだけです。

- 標準出力は常に JSON Lines（1行に1オブジェクト）。空の値は省略します
- エラーは標準エラー出力に `{"error":"...","status":403}` を出し、終了コード1で終了
- REST APIだけを使い、Gatewayには接続しません（`message watch` もポーリングです）

## インストール

```bash
nix profile install github:FujishigeTemma/discord-cli
```

Nixを使わない場合は、Go 1.27以上で次を実行します。

```bash
go install github.com/FujishigeTemma/discord-cli/cmd/discord@latest
```

## Botの作成

1. [Developer Portal](https://discord.com/developers/applications) で **New Application** を押し、アプリを作成する
2. **Bot** タブで次を行う
   - **Reset Token** を押してトークンをコピーする（表示されるのは1回だけです）
   - **Message Content Intent** をオンにする（オフのままだとメッセージ本文が空で返ってきます）
3. **General Information** タブで **Application ID** をコピーする
4. 次のURLの `APP_ID` を置き換えてブラウザで開き、サーバーを選んで招待する（招待するには、そのサーバーの「サーバー管理」権限が必要です）

   ```
   https://discord.com/oauth2/authorize?client_id=APP_ID&scope=bot&permissions=309237746752
   ```

   付与する権限は、View Channels、Send Messages、Send Messages in Threads、Create Public Threads、Attach Files、Read Message History、Add Reactions です。
   他人のメッセージも削除させたい場合は、Manage Messages を加えた `permissions=309237754944` を使います。

Botは1人1つ作ります。このCLIは、Botが1つのサーバーにだけ参加している前提で動きます。

## ログイン

```bash
discord auth login     # トークンを貼り付ける（echo "$TOKEN" | discord auth login のようにパイプで渡しても可）
discord auth status    # Botの名前とサーバー名が出れば成功
```

キャッシュ（メンション用のユーザー名とIDの対応、ダウンロードした添付ファイル）は `$XDG_CACHE_HOME/discord-cli/`（未設定なら `~/.cache/discord-cli/`）に置きます。

トークンとサーバーIDは `$XDG_CONFIG_HOME/discord-cli/config.json`（未設定なら `~/.config/discord-cli/config.json`、パーミッション 0600）に保存されます。

## コマンド

```
discord auth login | status | logout

discord channel list

discord thread list    [--channel CHANNEL_ID]
discord thread create  <channel_id> --name NAME [--message MESSAGE_ID] [--text TEXT | --stdin]

discord message list   <channel_id> [--limit N] [--before ID | --after ID | --around ID]
discord message watch  <channel_id> [--after ID] [--interval 3s] [--timeout DURATION] [--max N] [--include-self]
discord message get    <channel_id> <message_id>
discord message send   <channel_id> [--text TEXT | --stdin] [--file PATH]... [--reply-to MESSAGE_ID] [--mention NAME]...
discord message edit   <channel_id> <message_id> (--text TEXT | --stdin)
discord message delete <channel_id> <message_id>

discord attachment download <channel_id> <message_id>

discord reaction list   <channel_id> <message_id> <emoji>
discord reaction add    <channel_id> <message_id> <emoji>
discord reaction remove <channel_id> <message_id> <emoji>
```

- スレッドIDはチャンネルIDとして `message` / `reaction` 系のコマンドにそのまま渡せます。
- `message list` は古い順に出力します。`--limit` は1〜100で、デフォルトは50です。
- `message watch` は新着メッセージを届いた順に1行ずつ出し続けます。開始位置はデフォルトで「今」です。続きから再開するには、最後に出た `id` を `--after` に渡します。
- `message watch` は `--max N` 件出すか `--timeout` が過ぎると終了コード0で終わります（何も来なければ出力は空）。次の1件を待つには `--max 1 --timeout 10m` のように使います。Bot自身の投稿は `--include-self` を付けない限り出しません。
- `message send` で本文が2000文字を超えると `message.txt` として添付し、出力に `"fallback":"attachment"` を付けます。
- `message edit` は2000文字を超えるとエラーになります。
- `thread create` をフォーラムチャンネルに対して使う場合は `--text` か `--stdin` が必須です。
- 絵文字は `👍` か、カスタム絵文字の `name:id` で指定します。
- メッセージの `reactions[].me` は、Bot自身もそのリアクションを付けているかを表します（`count` にBotの分が含まれます）。
- `reaction list` は、そのリアクションを付けたユーザーを1行に1人ずつ `{"id","name","bot"}` で出します（最大100人）。
- 本文中のメンションは `<@name>` で表示します（平文の `@name` と区別するため）。メンションするには `--mention name` を使います。
- サブコマンドを省略するか、存在しないサブコマンドを指定すると、ヘルプを表示して終了コード0で終了します。

## 出力例

```json
{"id":"1300","author":{"id":"1100","name":"alice"},"timestamp":"2026-10-30T12:00:00Z","content":"hello","reactions":[{"emoji":"👍","count":1}]}
```

## 開発

```bash
direnv allow     # または nix develop
go test ./...
```

テストは偽のDiscord APIに対して実行するので、トークンは不要です。
