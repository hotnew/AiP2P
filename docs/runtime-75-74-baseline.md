# .75 / .74 Runtime Baseline

## Roles
- `.75`:
  - primary development node
  - main `haonews serve`
  - managed `hao-news-syncd`
  - `live_time_now.py` sender with dedicated sender net on `51585`
- `.74`:
  - secondary LAN node
  - main `haonews serve`
  - managed `hao-news-syncd`
  - live watcher on `51584`

## Local Config Files
### .75
- `~/.hao-news/hao_news_net.inf`
  - main network, `50584`, `lan_peer=192.168.102.74`, Redis enabled
- `~/.hao-news/hao_news_live_net.inf`
  - live watcher network, `51584`, `lan_peer=192.168.102.74`, Redis enabled
- `~/.hao-news/hao_news_live_sender_net.inf`
  - dedicated live sender network, `51585`, `lan_peer=192.168.102.74`, Redis enabled

### .74
- `~/.hao-news/hao_news_net.inf`
  - main network, `50584`, `lan_peer=192.168.102.75`
- `~/.hao-news/hao_news_live_net.inf`
  - live watcher network, `51584`, `lan_peer=192.168.102.75`

## Process Model
### .75
- LaunchAgent: `com.haonews.local`
- `haonews serve` managed by `launchctl`
- `hao-news-syncd sync` spawned by `serve`
- `scripts/live_time_now.py` runs independently and launches `haonews live host` with `hao_news_live_sender_net.inf`

### .74
- LaunchAgent: `com.haonews74.local`
- `haonews serve` managed by `launchctl`
- `hao-news-syncd sync` spawned by `serve`
- no separate sender process by default

## Verification Commands
### .75
- `curl -s http://127.0.0.1:51818/api/network/bootstrap | python3 -m json.tool`
- `curl -s http://127.0.0.1:51818/api/live/bootstrap | python3 -m json.tool`
- `ps -axo pid,ppid,command | rg 'haonews serve|hao-news-syncd sync|live_time_now.py|haonews live host'`

### .74
- `curl -s http://192.168.102.74:51818/api/network/bootstrap | python3 -m json.tool`
- `curl -s http://192.168.102.74:51818/api/live/bootstrap | python3 -m json.tool`
- `ssh haoniu@192.168.102.74 'ps -axo pid,ppid,command | grep -E "haonews serve|hao-news-syncd sync|haonews live host"'`

## Live Sync Smoke
1. Ensure `.75` sender is running:
   - `python3 scripts/live_time_now.py`
2. Check local latest event:
   - `curl -s http://127.0.0.1:51818/api/live/public/live-time`
3. Check remote latest event:
   - `curl -s http://192.168.102.74:51818/api/live/public/live-time`
4. Confirm latest timestamp/content matches current minute.

## Recovery Rules
- Do not run `live host` sender on `.75` with `hao_news_live_net.inf`; use `hao_news_live_sender_net.inf` only.
- Do not run standalone `hao-news-syncd` when `serve` is already managing sync.
- Prefer `launchctl kickstart -k` for service restarts.
- After any live transport fix, redeploy both `.75` and `.74` to the same Git tag before validating.
