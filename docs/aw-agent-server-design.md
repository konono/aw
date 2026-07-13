# aw-agent-server 設計ドキュメント

K8s 上で常駐する AI エージェント Pod を Slack 等のチャットインターフェースから利用するためのサーバー設計。

## 1. アーキテクチャ概要

```
┌────────────┐     ┌─────────────────────────┐     ┌──────────────────┐
│   Slack    │────▶│   aw-agent-server       │────▶│  K8s Cluster     │
│   Users    │◀────│   (Deployment on K8s)   │◀────│                  │
└────────────┘     │                         │     │  ┌──────────┐   │
                   │  - Slack Bot (Bolt)     │     │  │ Pod:     │   │
                   │  - Pod Manager          │     │  │ claude   │   │
                   │  - Session Store        │     │  │ (user-A) │   │
                   └─────────────────────────┘     │  └──────────┘   │
                                                   │  ┌──────────┐   │
                                                   │  │ Pod:     │   │
                                                   │  │ claude   │   │
                                                   │  │ (user-B) │   │
                                                   │  └──────────┘   │
                                                   └──────────────────┘
```

### 技術スタック

| コンポーネント | 技術 |
|---|---|
| 言語 | Go |
| Slack 連携 | [slack-go/slack](https://github.com/slack-go/slack) (Socket Mode) |
| K8s 操作 | `k8s.io/client-go` |
| セッション管理 | Redis (推奨) or PostgreSQL |
| 設定 | 環境変数 + ConfigMap |
| デプロイ | K8s Deployment (aw-agent-server 自体も K8s 上で動作) |

### aw との関係

| aw が提供するもの | aw-agent-server が利用する方法 |
|---|---|
| `aw build --push` | ビルド済みイメージをレジストリから pull |
| `aw manifest --name <user>` | ユーザーごとの Pod manifest を生成・適用 |
| aw-init.sh / entrypoint.sh | イメージに含まれており、そのまま動作 |
| profile 設定 (.aw.yml) | manifest 生成時に参照 |

---

## 2. Slack 連携設計

### Slack App 設定

**Bot Token Scopes**:
- `app_mentions:read` — メンションの受信
- `chat:write` — メッセージの送信
- `im:history` — DM の読み取り
- `im:read` — DM チャンネルの参照
- `im:write` — DM への送信

**Event Subscriptions**:
- `app_mention` — チャンネルでの @aw-bot メンション
- `message.im` — DM での直接メッセージ

**Socket Mode**: Webhook URL 不要で Event を受信。Firewall 設定が不要なため推奨。

### イベント処理フロー

```
1. Slack Event 受信 (app_mention or message.im)
2. ユーザー ID を取得
3. セッション検索
   ├── セッションあり → 既存 Pod 名を取得
   └── セッションなし → 新規 Pod を作成、セッション登録
4. Pod が Running か確認
   ├── Running → exec 実行
   └── Not Running → Pod 再作成、セッション更新
5. kubectl exec <pod> -- claude -p --continue "<message>"
6. stdout をキャプチャ
7. Slack スレッドに応答を投稿
```

### スレッドベースの会話管理

- 初回メッセージ → 新しい Slack スレッドを作成
- 同じスレッドへの返信 → 同じ Pod、同じ claude セッション (`--continue`)
- 異なるスレッド → 同じ Pod だが新しい claude セッション

### レスポンスフォーマット

- コードブロック: `\`\`\`` でラップ
- 長文 (4000文字超): 複数メッセージに分割
- エラー: `:warning:` emoji + エラーメッセージ
- タイムアウト: 実行中のスピナー → タイムアウトメッセージ

---

## 3. Pod ライフサイクル管理

### Pod の作成

```go
func (m *PodManager) EnsurePod(ctx context.Context, userID string) (string, error) {
    // 1. 既存 Pod の検索 (label: aw.dev/user=<userID>)
    // 2. Running なら Pod 名を返す
    // 3. なければ aw manifest を実行して manifest を生成
    //    cmd: aw manifest <profile> --name <userID>
    // 4. kubectl apply -f で適用
    // 5. Pod が Running になるまで待機 (timeout: 120s)
    // 6. Pod 名を返す
}
```

### Pod のラベル設計

```yaml
labels:
  aw.dev/profile: claude-k8s
  aw.dev/tool: claude
  aw.dev/mode: chat
  aw.dev/user: U1234567        # Slack User ID
  aw.dev/managed-by: aw-agent-server
```

### アイドルタイムアウト

- 最終 exec から一定時間 (デフォルト: 1h) 経過した Pod を削除
- バックグラウンド goroutine で定期チェック (5分間隔)
- 削除前にセッションストアからも除去

### Pod のヘルスチェック

```go
func (m *PodManager) IsHealthy(ctx context.Context, podName string) bool {
    // kubectl exec <pod> -- echo ok
    // timeout: 5s
}
```

### クラッシュ時の復旧

- exec 実行時に Pod が存在しない → 自動再作成
- Pod が CrashLoopBackOff → 削除して再作成
- イメージ pull エラー → Slack にエラー通知

---

## 4. セッション管理

### データモデル

```go
type Session struct {
    UserID      string    // Slack User ID
    PodName     string    // K8s Pod name
    Namespace   string    // K8s namespace
    ThreadTS    string    // Slack thread timestamp (conversation ID)
    CreatedAt   time.Time
    LastActiveAt time.Time
}
```

### Redis スキーマ

```
# ユーザー → Pod のマッピング
session:<userID>:pod     = "aw-claude-k8s-U1234567"
session:<userID>:ns      = "aw"

# 最終アクティブ時刻 (TTL 付き)
session:<userID>:active  = "2026-07-13T10:00:00Z"  (TTL: 2h)
```

### `claude -p --continue` によるセッション継続

- claude は `~/.claude/` にセッション状態を保持
- `--continue` フラグで最新セッションを再開
- Pod が再作成されるとセッションはリセット（tool-config ConfigMap は再マウントされるが、セッションデータは emptyDir に保存されるため消失）

### セッションの期限切れ

- Redis TTL でアイドルセッションを自動期限切れ
- 期限切れ時に Pod も削除
- ユーザーが再度メッセージを送ると新しい Pod + セッションが作成

---

## 5. セキュリティ

### Slack Bot Token の管理

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: aw-agent-server-slack
  namespace: aw-system
type: Opaque
stringData:
  SLACK_BOT_TOKEN: "xoxb-..."
  SLACK_APP_TOKEN: "xapp-..."  # Socket Mode 用
```

### RBAC

aw-agent-server に必要な最小権限:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: aw-agent-server
  namespace: aw
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/exec"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch", "create", "update", "delete"]
  - apiGroups: [""]
    resources: ["configmaps", "secrets", "serviceaccounts"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["namespaces"]
    verbs: ["get", "list", "watch", "create"]
```

### Namespace 分離

```
aw-system/    — aw-agent-server 本体 + Redis
aw/           — ユーザーの AI エージェント Pod
```

---

## 6. 運用

### ログ設計

| ログレベル | 内容 |
|---|---|
| INFO | Pod 作成/削除、メッセージ受信/応答、セッション作成/期限切れ |
| WARN | Pod ヘルスチェック失敗、exec タイムアウト、イメージ pull 遅延 |
| ERROR | Pod 作成失敗、Slack API エラー、Redis 接続エラー |

構造化ログ (JSON) で出力。

### メトリクス (Prometheus)

| メトリクス | 説明 |
|---|---|
| `aw_agent_pods_active` | 現在アクティブな Pod 数 (Gauge) |
| `aw_agent_exec_duration_seconds` | exec 実行時間 (Histogram) |
| `aw_agent_exec_total` | exec 実行回数 (Counter, success/error ラベル) |
| `aw_agent_pod_create_duration_seconds` | Pod 作成～Running の時間 (Histogram) |
| `aw_agent_slack_messages_total` | 受信メッセージ数 (Counter) |

### アラート設計

| アラート | 条件 |
|---|---|
| PodCreateFailure | Pod 作成失敗が 5分間に 3回以上 |
| ExecTimeout | exec タイムアウトが 10分間に 5回以上 |
| HighPodCount | アクティブ Pod 数が閾値超過 |
| RedisDown | Redis 接続不可が 1分以上 |

### デプロイメント manifest 例

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: aw-agent-server
  namespace: aw-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: aw-agent-server
  template:
    metadata:
      labels:
        app: aw-agent-server
    spec:
      serviceAccountName: aw-agent-server
      containers:
        - name: server
          image: ghcr.io/myorg/aw-agent-server:latest
          env:
            - name: SLACK_BOT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: aw-agent-server-slack
                  key: SLACK_BOT_TOKEN
            - name: SLACK_APP_TOKEN
              valueFrom:
                secretKeyRef:
                  name: aw-agent-server-slack
                  key: SLACK_APP_TOKEN
            - name: REDIS_URL
              value: "redis://redis:6379"
            - name: AW_PROFILE
              value: "claude-k8s"
            - name: AW_NAMESPACE
              value: "aw"
            - name: IDLE_TIMEOUT
              value: "1h"
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
```

---

## 7. Known Limitations (v1)

`aw manifest` が生成する K8s manifest では、ローカル Docker フローの一部機能が対象外:

| 機能 | ローカル Docker | K8s manifest | 備考 |
|---|---|---|---|
| ワークスペースマウント | バインドマウント (双方向) | emptyDir (空) | Pod 内で `git clone` で対応 |
| SSH agent forwarding | ソケットマウント / SSH tunnel | 非対応 | SSH キーは `secrets.files` で配置可能 |
| Container socket (DooD) | ホストソケットマウント | 非対応 | K8s 内 DooD はセキュリティリスク |
| Reaper (後片付け) | パイプベースのサブプロセス | 非対応 | Pod 削除は kubectl or aw-agent-server |
| ホスト .gitconfig マウント | バインドマウント (RO) | 非対応 | Pod 内で `git config` で設定 |
| mount_gh (gh config マウント) | バインドマウント (RO) | 非対応 | `gh_token: true` で GITHUB_TOKEN を使用 |
| チームメッセージング | 共有 SQLite + MCP | 非対応 | 将来の aw-agent-server で対応予定 |
