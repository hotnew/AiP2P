# Aip2p Redis 整改与落地计划

日期：2026-04-01

## 目标

把 `/Users/haoniu/sh18/docs/aip2p-redis-20260401.md` 收成一条可逐步落地、可随时回退的实现链。

这次不做“大改存储架构”，只做：

1. 在现有 `aip2p_net.inf` 上增加 Redis 配置解析
2. 增加核心 Redis 客户端封装
3. 先把 Redis 用在 `Live` 读缓存
4. 保留文件存储为唯一持久层和最终回退路径

## 不做的事

这轮明确不做：

- 不引入新的 `workspace.json` / `net.json` 配置分支
- 不让 Redis 变成唯一持久层
- 不把 `topics` / `sync` 写路径直接改成 Redis-first
- 不做 Redis 异步 flush worker
- 不做 Redis 依赖下的行为强绑定

## 设计原则

1. Redis 是一级热缓存，不是权威存储。
2. 文件仍然是权威写路径。
3. 先做 `Live`，因为热点更明显，改动面更可控。
4. 任何 Redis 错误都必须自动回退到现有文件逻辑。
5. 所有缓存 key 必须带前缀，默认 `aip2p-`

## 配置策略

不采用原方案里的 JSON 配置文件。

直接扩展现有 `aip2p_net.inf`：

```ini
redis_enabled=true
redis_addr=127.0.0.1:6379
redis_password=
redis_db=0
redis_key_prefix=aip2p-
redis_max_retries=3
redis_dial_timeout_ms=3000
redis_read_timeout_ms=2000
redis_write_timeout_ms=2000
redis_pool_size=10
redis_min_idle_conns=2
redis_hot_window_days=7
```

原因：

- 当前 `aip2p` 真实部署就是围绕 `aip2p_net.inf`
- 改这一条比再引入新配置体系安全得多
- `.75/.76` 运维链更简单

## Phase R1

目标：只落基础设施，不改现有页面行为。

内容：

1. `internal/aip2p/redis_config.go`
2. `internal/aip2p/redis_client.go`
3. `internal/aip2p/network.go` 增加 Redis 配置解析
4. `internal/plugins/aip2p/network.go` 同步解析
5. `go.mod` 增加 `github.com/redis/go-redis/v9`

验收：

- 没配置 Redis 时，行为完全不变
- 配了 Redis 但连不上时，行为完全回退
- `go test ./internal/aip2p ./internal/plugins/aip2p`

## Phase R2

目标：给 `Live` 加只读缓存，不动持久语义。

内容：

1. `internal/aip2p/live/store.go`
   - `LoadRoom()`
   - `ReadEvents()`
   - `LoadArchiveResult()`
   增加 Redis read-through
2. `ListRooms()` 增加短 TTL 列表缓存
3. 现有写操作：
   - `SaveRoom()`
   - `SaveRoomAuthoritative()`
   - `AppendEvent()`
   - `SaveArchiveResult()`
   只做缓存更新/失效，不改文件权威写逻辑

验收：

- Redis 命中时减少磁盘 JSON 反序列化
- Redis 出错时自动回退文件
- `go test ./internal/aip2p/live ./internal/plugins/aip2plive`

## Phase R3

目标：把 Redis 状态暴露到运维层，但不扩大写路径。

内容：

1. `/network` 或运行时状态里显示：
   - Redis enabled
   - Redis addr
   - Redis reachable
2. 提供最小 Redis 健康检查

验收：

- 不影响主功能
- 状态可见

## Phase R4

目标：把 `topics/sync` 的热元数据先接到 Redis，但不改首页主读链。

这轮只做低风险部分：

1. `sync announcement` 缓存
2. `channel/topic` 热索引
3. `queue refs` Redis 镜像

暂时不做：

- 首页 `/` / `/topics` 直接从 Redis 渲染
- Redis-first 写路径
- flush / write-behind

## 当前执行顺序

这次先做：

1. `R1`
2. `R2`
3. `R3`
4. 补测试和说明

当前继续推进：

- `R4` 的热元数据部分

## 回退策略

如果 Redis 路线任何一步不稳，立即回退到：

- Redis client 可为 `nil`
- 所有存取都走现有文件路径
- 不改任何现有文件格式

这保证 `.75/.76` 可以随时撤回，不需要迁移数据。

## 当前已实施状态

已完成：

1. `R1`
   - `internal/aip2p/redis_config.go`
   - `internal/aip2p/redis_client.go`
   - `internal/aip2p/network.go`
   - `internal/plugins/aip2p/network.go`
   - `go.mod` / `go.sum`

2. `R2`
   - `internal/aip2p/live/store.go`
   - `internal/aip2p/live/archive.go`
   - `internal/aip2p/live/discovery.go`
   - `internal/aip2p/live/room.go`
   - `internal/plugins/aip2plive/plugin.go`

3. `R3`
   - `/network` 通过 `NodeStatus` 暴露 Redis enabled / addr / reachable / prefix
   - 使用轻量 `ProbeRedis()` 探针，不把 Redis 状态探测绑死在主读写路径
   - `sync status` 落盘后同步镜像到 `aip2p-meta:node_status`
   - 插件读 `sync status` 时优先命中 Redis 镜像，失败再回退 `status.json`
   - `sync supervisor` 健康检查也改为优先读 Redis 镜像
   - `/api/network/bootstrap` 也返回 Redis 摘要：
     - `redis.enabled`
     - `redis.online`
     - `redis.addr`
     - `redis.prefix`
     - `redis.db`
     - `announcement_count`
     - `channel_index_count`
     - `topic_index_count`
     - `realtime_queue_refs`
     - `history_queue_refs`

4. `R4`（已完成热元数据部分）
   - `sync announcement` 生成/导入时同步镜像到 Redis
   - `channel` 热索引：
     - `aip2p-sync:channel:<channel>`
   - `topic` 热索引：
     - `aip2p-sync:topic:<topic>`
   - `queue refs` 运行时镜像：
     - `aip2p-sync:queue:refs:realtime`
     - `aip2p-sync:queue:refs:history`
   - 说明：
     - 文件队列仍是权威来源
     - Redis 只是热镜像，不参与最终判定
   - 已补读取 helper：
     - 按 channel/topic 读取 announcement 热索引
     - 读取 realtime/history 队列镜像

4. 测试
   - `internal/aip2p/network_test.go`
   - `internal/aip2p/live/redis_store_test.go`
   - `internal/aip2p/status_redis_test.go`
    - `internal/plugins/aip2p/network_test.go`
    - `internal/plugins/aip2p/ops_status_test.go`
    - `internal/plugins/aip2p/sync_status_test.go`
    - `internal/plugins/aip2p/sync_supervisor_test.go`

当前明确未做：

- 首页 `/` / `/topics` 主索引 Redis 化
- Redis write-behind / flush worker
