# board MCP on Lambda Function URL

[lambroll](https://github.com/fujiwara/lambroll) + [Lambda Web Adapter (LWA)](https://github.com/awslabs/aws-lambda-web-adapter) を使って board MCP サーバーを Lambda Function URL にデプロイし、[idproxy](https://github.com/youyo/idproxy) で OIDC 認証を被せる構成です。

| 項目 | 値 |
|---|---|
| MCP 認証 | OIDC (idproxy) |
| BOARD API 認証 | 共有 API キー（Lambda 環境変数） |
| 状態保存 | DynamoDB (idproxy Store) |

> board は単一の API キー / トークンで BOARD API にアクセスする設計のため、
> per-user OAuth は使いません（[logvalet](https://github.com/youyo/logvalet) の Mode A 相当）。

## Prerequisites

- [lambroll](https://github.com/fujiwara/lambroll)（`brew install fujiwara/tap/lambroll`）
- [mise](https://mise.jdx.dev/)
- AWS CLI（認証済み）
- OIDC プロバイダ（Google / Microsoft Entra ID / Auth0 など）で登録できる OAuth クライアント
- BOARD API キー / トークン（[the-board.jp](https://the-board.jp/) で発行）

## 1. IAM Role 作成

```bash
aws iam create-role \
  --role-name board-lambda-role \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": { "Service": "lambda.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy \
  --role-name board-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole

aws iam put-role-policy \
  --role-name board-lambda-role \
  --policy-name board-mcp-idproxy-dynamodb \
  --policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Action": ["dynamodb:GetItem","dynamodb:PutItem","dynamodb:DeleteItem","dynamodb:UpdateItem","dynamodb:Query"],
      "Resource": "arn:aws:dynamodb:*:*:table/board-mcp-idproxy"
    }]
  }'

aws iam get-role --role-name board-lambda-role --query 'Role.Arn' --output text
```

## 2. OIDC クライアントの作成

利用する OIDC プロバイダで OAuth 2.0 / OIDC クライアントを作成。

- **Redirect URI**: Function URL 確定までは仮値で登録。確定後 `<FUNCTION_URL>/callback` に更新。
- 取得する値: `Issuer URL`, `Client ID`, `Client Secret`

## 3. ECDSA 署名鍵の生成

idproxy の DynamoDB Store を Lambda マルチコンテナで安全に動かすには、
ランダム生成ではなく固定の ECDSA P-256 鍵が必要です。

```bash
mise run generate-signing-key
# 出力された awk コマンドで signing.pem を 1 行に整形し、
# .env の BOARD_MCP_SIGNING_KEY に設定する
```

## 4. `.env` を作成

```bash
cp .env.example .env
# .env を編集（ROLE_ARN, BOARD_API_*, OIDC*, BOARD_MCP_SIGNING_KEY）
# COOKIE_SECRET 生成: openssl rand -hex 32
```

## 5. DynamoDB テーブル作成

```bash
mise run dynamodb-create
```

## 6. デプロイ

```bash
mise run deploy
```

## 7. Function URL を確定して再デプロイ

```bash
aws lambda get-function-url-config --function-name board-mcp \
  --query 'FunctionUrl' --output text
```

- `.env` の `BOARD_MCP_EXTERNAL_URL` を確定値に更新
- OIDC プロバイダの redirect_uri を `<FUNCTION_URL>/callback` に更新
- `mise run deploy` で再デプロイ

## 8. 動作確認

ブラウザで Function URL → OIDC ログイン → `/healthz` または MCP エンドポイントが応答すれば成功。

MCP クライアントから接続する場合は、OAuth 2.1 AS フロー（Bearer Token）を踏むため
idproxy が発行する access token を使ってください（[idproxy README](https://github.com/youyo/idproxy/blob/main/README.md) 参照）。

## クリーンアップ

```bash
lambroll delete --function function.json
mise run dynamodb-delete
aws iam delete-role-policy --role-name board-lambda-role --policy-name board-mcp-idproxy-dynamodb
aws iam detach-role-policy --role-name board-lambda-role \
  --policy-arn arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole
aws iam delete-role --role-name board-lambda-role
mise run clean
```

## 環境変数リファレンス

### 必須

| 変数 | 説明 |
|------|------|
| `ROLE_ARN` | Lambda 実行ロール ARN |
| `BOARD_API_KEY` | BOARD API key |
| `BOARD_API_TOKEN` | BOARD API token |
| `BOARD_MCP_EXTERNAL_URL` | Function URL（deploy 後に確定値へ更新） |
| `BOARD_MCP_OIDC_ISSUER` | OIDC Issuer URL |
| `BOARD_MCP_OIDC_CLIENT_ID` | OIDC Client ID |
| `BOARD_MCP_OIDC_CLIENT_SECRET` | OIDC Client Secret |
| `BOARD_MCP_COOKIE_SECRET` | Cookie 暗号鍵（hex, 32 bytes 以上） |
| `BOARD_MCP_SIGNING_KEY` | ECDSA P-256 署名鍵（PEM、改行は `\n`）|
| `BOARD_MCP_IDPROXY_STORE_DYNAMODB_TABLE` | DynamoDB テーブル名 |

### 任意

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `AWS_REGION` | `ap-northeast-1` | デプロイ先リージョン |
| `BOARD_VERSION` | `0.10.0` | GitHub Release バージョン |
| `LAMBDA_ARCH` | `arm64` | `arm64` または `x86_64` |
| `BOARD_MCP_ALLOWED_DOMAINS` | （無制限） | 許可するメールドメイン（カンマ区切り） |
| `BOARD_MCP_ALLOWED_EMAILS` | （無制限） | 許可するメールアドレス（カンマ区切り） |
| `BOARD_MCP_IDPROXY_STORE_DYNAMODB_REGION` | `ap-northeast-1` | DynamoDB リージョン |
