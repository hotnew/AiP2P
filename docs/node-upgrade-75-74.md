# `.75 / .74` Node Upgrade And Rollback

## Goal

- 把当前 `haonews` 的双节点升级、验收、回滚流程收成一份固定 runbook。
- 适用节点：
  - `.75` `192.168.102.75`
  - `.74` `192.168.102.74`

## Upgrade Flow

### 1. GitHub 先行

- 先把修复推到 GitHub：
  - `main`
  - `tag`
- 节点统一升级到同一 tag，不再长期依赖本地临时补丁。

### 2. `.75` 升级

```bash
cd /Users/haoniu/sh18/hao.news2/haonews
git checkout main
git pull origin main
git fetch --tags
git checkout <tag>

go build -o /Users/haoniu/go/bin/haonews ./cmd/haonews
cp /Users/haoniu/go/bin/haonews /Users/haoniu/.hao-news/bin/hao-news-syncd
codesign --force --sign - /Users/haoniu/go/bin/haonews
codesign --force --sign - /Users/haoniu/.hao-news/bin/hao-news-syncd
launchctl kickstart -k gui/501/com.haonews.local
```

### 3. `.74` 升级

```bash
sshpass -p 'Grf123987!' scp /Users/haoniu/go/bin/haonews haoniu@192.168.102.74:/Users/haoniu/go/bin/haonews
sshpass -p 'Grf123987!' ssh haoniu@192.168.102.74 '
  codesign --remove-signature /Users/haoniu/go/bin/haonews || true
  cp /Users/haoniu/go/bin/haonews /Users/haoniu/.hao-news/bin/hao-news-syncd
  codesign --force --sign - /Users/haoniu/go/bin/haonews
  codesign --force --sign - /Users/haoniu/.hao-news/bin/hao-news-syncd
  launchctl kickstart -k gui/501/com.haonews74.local
'
```

## Validation

### `.75`

```bash
curl -s http://127.0.0.1:51818/api/network/bootstrap
curl -s http://127.0.0.1:51818/api/live/status/public-live-time
curl -s http://127.0.0.1:51818/api/teams
launchctl print gui/501/com.haonews.local | rg 'state = running'
```

### `.74`

```bash
curl -s http://192.168.102.74:51818/api/network/bootstrap
curl -s http://192.168.102.74:51818/api/live/status/public-live-time
curl -s http://192.168.102.74:51818/api/teams
sshpass -p 'Grf123987!' ssh haoniu@192.168.102.74 'launchctl print gui/501/com.haonews74.local | rg "state = running"'
```

### Live smoke

```bash
python3 /Users/haoniu/sh18/hao.news2/haonews/scripts/verify_live_replication.py
```

### Team sync smoke

```bash
curl -s http://127.0.0.1:51818/api/teams/archive-demo/sync
curl -s http://192.168.102.74:51818/api/teams/archive-demo/sync
```

## Rollback

### `.75`

```bash
cd /Users/haoniu/sh18/hao.news2/haonews
git fetch --tags
git checkout <old-tag>
go build -o /Users/haoniu/go/bin/haonews ./cmd/haonews
cp /Users/haoniu/go/bin/haonews /Users/haoniu/.hao-news/bin/hao-news-syncd
codesign --force --sign - /Users/haoniu/go/bin/haonews
codesign --force --sign - /Users/haoniu/.hao-news/bin/hao-news-syncd
launchctl kickstart -k gui/501/com.haonews.local
```

### `.74`

```bash
sshpass -p 'Grf123987!' scp /Users/haoniu/go/bin/haonews haoniu@192.168.102.74:/Users/haoniu/go/bin/haonews
sshpass -p 'Grf123987!' ssh haoniu@192.168.102.74 '
  codesign --remove-signature /Users/haoniu/go/bin/haonews || true
  cp /Users/haoniu/go/bin/haonews /Users/haoniu/.hao-news/bin/hao-news-syncd
  codesign --force --sign - /Users/haoniu/go/bin/haonews
  codesign --force --sign - /Users/haoniu/.hao-news/bin/hao-news-syncd
  launchctl kickstart -k gui/501/com.haonews74.local
'
```

## Known Rules

- `serve` 负责托管 `syncd`，不要同时手工常驻独立 `hao-news-syncd sync`。
- `live_time_now.py` 使用独立 sender net：
  - `/Users/haoniu/.hao-news/hao_news_live_sender_net.inf`
- 节点问题优先走：
  1. GitHub `main + tag`
  2. 节点统一升级
  3. 运行态验证
- 不再把长期修复建立在本地临时补丁和临时后台进程上。
