# Codex 指纹收敛与 WS mode 配置调研

核对日期：2026-09-19。项目基线：sub2api **0.2.5**，提交 `efe9aab1e4ec89a42ba45e8dac20e882c5409a6a`。适用场景：一个上游账号、固定出口 IP、5–6 位使用者各持一个站点 API Key。本文提供配置建议，没有修改线上账号或部署。

## 1. 三类配置各自解决什么问题

| 项目 | 配置位置 | 实际作用 |
| --- | --- | --- |
| Codex 指纹收敛 | sub2api → 账号编辑 → OpenAI OAuth/Setup Token 设置 | 改写应用层设备、会话、线程等标识。不是 Codex 官方客户端的同名设置，也不是访问权限隔离。 |
| TLS 指纹模板 | sub2api 的 TLS 配置及实际调用该模板的 HTTP 传输路径 | 改变 TLS 握手特征。与应用层标识收敛是不同机制；不能由“启用了指纹收敛”推断 HTTP 与 WS 的 TLS 握手完全相同。 |
| WS mode | Codex 客户端能力声明 + sub2api 全局路由 + 账号模式 | 决定客户端至网关、网关至上游分别使用 WebSocket 还是 HTTP/SSE，以及是否复用上游连接。 |

依据：[OpenAI Codex 配置参考][codex-config]、[sub2api 指纹实现][fingerprint]、[WS 请求头构造][ws-headers]、[部署配置][deploy]。

## 2. 指纹收敛怎么选

0.2.5 原本已有这四档，默认 `off`，本次改动不会替你切换：

| 模式 | 设备 installation_id | session_id | thread_id | 对当前场景的建议 |
| --- | --- | --- | --- | --- |
| `off` | 不执行本功能的收敛 | 不执行本功能的收敛 | 不执行本功能的收敛 | 先保留，建立稳定性基线。网关其他既有的账号/API Key 命名空间处理仍然存在。 |
| `device` | 收敛为账号的稳定设备标识 | 保留各会话语义 | 保留各线程语义 | 有明确需要时，优先小范围尝试这一档。 |
| `session` | 稳定 | 账号级统一 | 由原始客户端 session 派生 | 改写范围较大，不作为多人共用的默认推荐。 |
| `full` | 稳定 | 统一 | 统一 | 不建议作为 5–6 人共享时的起步配置。 |

`session` / `full` 的 turn_id 按请求重新生成，window_id 与派生线程关联。种子由服务端生成和维护，不需要手工填入，也不应频繁改动。可用范围与细节见[源码][fingerprint]及[账号设置文案][account-ui]。

操作：编辑目标 OpenAI OAuth / Setup Token 账号，找到“Codex 指纹收敛”，保留“关闭”或明确选择“仅设备”，保存即可。官方 Codex 的 `config.toml` 不要填写 `codex_fingerprint_mode`；它是 sub2api 的账号 Extra 字段。官方配置参考没有把该字段定义为客户端配置项。[codex-config] [fingerprint]

固定 IP 不等于已经解决会话隔离。建议每人使用单独的站点 API Key，并让各客户端生成自己的 session。`account_proxy` 只按账号和代理隔离连接池，不能保证代理供应商出口永远不换 IP。指纹收敛也不保证减少封禁、提高模型质量或增加额度；原项目设置提示本身记录了收敛后额度表现变化的反馈，属于项目反馈，不是 OpenAI 的效果承诺。[deploy] [account-ui]

## 3. sub2api 的四种 WS mode

| 模式 | 客户端 → 网关 | 网关 → 上游 | 适用情形 |
| --- | --- | --- | --- |
| `off` | 该账号不启用 WS 能力 | 正常 HTTP 路径 | 暂时不使用 WS；客户端也应选择 HTTP。不要假设服务器一定能替客户端自动降级。 |
| `ctx_pool` | WS | 池化的 WS 连接 | 现有池化路径，作为常规起点。共享池不表示可忽略会话兼容性。 |
| `passthrough` | WS | 每个客户端会话独立建立上游 WS | 希望减少池化复用变量，代价是更多长连接。 |
| `http_bridge` | WS | HTTP/SSE | 客户端希望使用 WS，但代理或上游 WS 不稳定时尝试。不能获得上游 WS 连接缓存的全部收益。 |

这些是 **sub2api 自定义的模式名称**；不是 OpenAI 官方 `config.toml` 的枚举。旧 `shared` / `dedicated` 在本项目会归一化为 `ctx_pool`。上述描述来自[账号模式解析][ws-mode]、[账号设置文案][account-ui]与[部署配置][deploy]。

### 服务端示例

以下是应合并进现有 YAML 的配置片段，不要覆盖已有数据库等配置：

```yaml
gateway:
  connection_pool_isolation: account_proxy
  openai_ws:
    enabled: true
    oauth_enabled: true
    apikey_enabled: true
    mode_router_v2_enabled: true
    ingress_mode_default: ctx_pool
    force_http: false
    responses_websockets: false
    responses_websockets_v2: true
    store_disabled_conn_mode: strict
```

**关键开关是 `mode_router_v2_enabled: true`**。0.2.5 示例默认是 `false`；关闭时不按新版账号模式分流，仍受 legacy 行为和其他能力开关约束。对应环境变量为 `GATEWAY_OPENAI_WS_MODE_ROUTER_V2_ENABLED=true`。修改服务启动配置后需按部署方式重启生效。[deploy]

然后在目标账号的“WS mode”选择 `ctx_pool`。OAuth 使用 Extra 字段 `openai_oauth_responses_websockets_v2_mode`，API Key 账号使用 `openai_apikey_responses_websockets_v2_mode`；优先通过管理界面保存。需要排查连接复用问题时，再对单个账号试 `passthrough`；代理不支持稳定 WS 时试 `http_bridge`。[ws-mode] [account-ui]

保持 `store_disabled_conn_mode: strict`。原版已按账号并发与系数计算 WS 池上限；默认 OAuth/API Key 系数均为 5.0。**WS 存活会话数与正在执行的请求数不是一回事**：`ctx_pool` 在轮次之间仍持有连接，所以不要简单把池上限压成 2–3 条来代替业务并发限制。[deploy]

### Codex 客户端示例

每个人的 Codex 用户配置通常在 `~/.codex/config.toml`。保留已有的可用模型设置，按实际域名配置 provider：

```toml
model_provider = "sub2api"

[model_providers.sub2api]
name = "sub2api"
base_url = "https://YOUR-SUB2API-DOMAIN/v1"
wire_api = "responses"
env_key = "SUB2API_API_KEY"
requires_openai_auth = false
supports_websockets = true
```

`SUB2API_API_KEY` 是该使用者自己的站点 API Key，通过启动 Codex 的环境提供，不是上游账号的 OAuth token。`supports_websockets` 是官方文档定义的 provider WebSocket 能力开关；服务端、代理及账号也必须支持。不要把网上旧示例的实验性 `features.responses_websockets*` 开关照搬到所有 Codex 版本。以所装版本的配置校验和当前[官方配置参考][codex-config]为准。

若暂不使用 WS，把 provider 的 `supports_websockets` 设为 `false`，再重新建立客户端会话。`wire_api` 仍为 `responses`；它并不因采用 WebSocket 就变成 `websocket`。[codex-config]

反向代理必须转发 WebSocket Upgrade。若使用 Nginx，相关 location 需支持 `proxy_http_version 1.1`、`Upgrade` 和 `Connection` 请求头，并给长连接合适的读取超时；不要把它误解成 sub2api 上游 HTTP/2 的开关。此处仅为部署检查建议，未修改代理配置。

## 4. 官方 WebSocket 文档对当前配置的启示

OpenAI 官方 Responses WebSocket 使用持续连接，每轮发送 `response.create`，可通过 `previous_response_id` 延续上下文。文档明确兼容 `store=false` / ZDR，但连接内存缓存并不等于持久化存储；断线或 `previous_response_not_found` 时，可能需要重连并重发完整输入上下文。[official-ws]

本次核对到的官方文档还支持 `stream_id` 多路会话：同一 lane 按顺序处理，不同 lane 可并行，服务端事件带对应 `stream_id`。文档给出的当前连接限制是 60 分钟、最多 16 个活动响应与 32 个不同的命名 lane。**这些是官方 API 当前文档，不应直接当作 ChatGPT OAuth 后端或 sub2api 所有模式已经支持的承诺**。本次流量统计对透传路径按 lane 分开记录，避免观察功能自行强加单轮并发限制；并未宣称实现所有官方多路调度能力。[official-ws]

## 5. 对当前运营方式的建议顺序

1. 每人独立站点 API Key，账号绑定固定代理；保留 `account_proxy`。
2. 指纹先保持 `off`；若要试收敛，单账号尝试 `device`，观察 429、断流、连接恢复和额度变化，不直接上 `session/full`。
3. WS 先保持现有可工作的路径；需要启用新版模式选择时，打开全局 mode router，并选 `ctx_pool`。对比问题时一次只换一个模式。
4. 业务并发可从 2–3 的保守值实测；它不是上游官方安全值，按实际套餐、请求耗时与失败率调整。
5. 先启用“并发观察建议”。严格 RPM / burst 仍默认关闭，需要限速时再设置；界面的初始 60 RPM / burst 5 是可编辑初值，不代表实际额度或安全保证。5–6 位用户共用这个账号预算，每次重试也消耗预算。
6. 本次新增的观察建议不自动改并发。成功测试不解除其他模型或并发失败产生的限制；需恢复账号时使用明确的恢复操作。

本次补丁保留 0.2.5 原有 WS 保活机制；补齐 device 模式的会话兼容性检查、HTTP/WS 应用层身份一致性、Retry-After / 冷却保护、共享测试并发、按账号原子 RPM 和观测界面。它没有启用任何线上策略，没有新增 Node.js TLS 模板。

## 来源

[codex-config]: https://developers.openai.com/codex/config-reference/
[official-ws]: https://developers.openai.com/api/docs/guides/websocket-mode
[fingerprint]: https://github.com/jihtsan/sub2api/blob/efe9aab1e4ec89a42ba45e8dac20e882c5409a6a/backend/internal/service/openai_codex_fingerprint.go
[ws-headers]: https://github.com/jihtsan/sub2api/blob/efe9aab1e4ec89a42ba45e8dac20e882c5409a6a/backend/internal/service/openai_ws_forwarder_payload.go
[deploy]: https://github.com/jihtsan/sub2api/blob/efe9aab1e4ec89a42ba45e8dac20e882c5409a6a/deploy/config.example.yaml
[ws-mode]: https://github.com/jihtsan/sub2api/blob/efe9aab1e4ec89a42ba45e8dac20e882c5409a6a/frontend/src/utils/openaiWsMode.ts
[account-ui]: https://github.com/jihtsan/sub2api/blob/efe9aab1e4ec89a42ba45e8dac20e882c5409a6a/frontend/src/i18n/locales/zh/admin/accounts.ts
