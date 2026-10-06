# Webhook 接收端示例（Python）

> English version below.

只用标准库（`http.server`）+ 仓库内的 Python SDK：校验签名、解密加密投递（JWE）、按事件 `id` 幂等，处理 `game_role.reviewed` / `user.deleted` / `server.offline` / `user.level_changed` / `connection.bound` / `connection.unbound`。只有启用加密投递时才需要第三方包 `cryptography`。

## 目录

```
webhook-receiver-python/
├── server.py          入口：读取环境变量、ThreadingHTTPServer、/healthz
├── receiver.py        端点本身：验签、解密、幂等、状态码（Receiver.handle），与 HTTP 框架无关
├── events.py          业务逻辑：事件分发（dispatch）
└── test_receiver.py   unittest 测试（直接调用 Receiver.handle，不监听端口）
```

`receiver.py` 会把仓库内的 `SDK/python` 加入 `sys.path`，所以不用安装 SDK；单独拷走本目录时 `pip install` SDK 即可（找不到仓库目录时自动跳过）。

## 运行

需要 Python 3.9+。

```bash
cd SDK/examples/webhook-receiver-python
export NOWORK_WEBHOOK_SECRET=whsec_xxx   # 后台「开放平台 → Webhook」新建 Webhook 时显示（只显示一次，可「重置密钥」）
python server.py
# ... INFO listening on http://0.0.0.0:8092/nowork/webhook (timestamp tolerance 300s, 0 decryption key(s))
```

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `NOWORK_WEBHOOK_SECRET` | 必填 | 端点签名密钥 |
| `HOST` / `PORT` | `0.0.0.0` / `8092` | 监听地址 |
| `WEBHOOK_PATH` | `/nowork/webhook` | 接收路径（后台端点 URL 需与之一致） |
| `WEBHOOK_TOLERANCE` | `300` | 允许的时间戳偏差（秒），`0` 关闭检查 |
| `SEEN_CAPACITY` | `10000` | 幂等缓存保留的事件 id 数量 |
| `NOWORK_DECRYPTION_KEYS` | 空 | 私钥 JWK 文件路径，逗号分隔（轮换期间填新旧两把）；设置后需要 `pip install cryptography`，缺少时启动即报错 |

生产环境请放在 HTTPS 反向代理之后，用 `systemd` / `supervisor` 托管。

## 接入已有框架

`Receiver.handle(method, headers, body)` 返回 `(状态码, JSON 字典)`，任何框架都能直接用，例如 Flask：

```python
from events import dispatch
from receiver import Receiver

rc = Receiver(secret, dispatch)

@app.post("/nowork/webhook")
def nowork_webhook():
    status, body = rc.handle("POST", request.headers, request.get_data())  # 必须是原始字节
    return body, status
```

## 行为约定

| 情况 | 响应 | 用户中心的处理 |
| --- | --- | --- |
| 签名正确、首次收到 | `200 {"status":"ok"}` | 标记已投递 |
| 同一 `id` 再次收到 | `200 {"status":"duplicate"}` | 标记已投递（不重复处理） |
| 签名错误 / 缺签名 / 时间戳偏差过大 | `401` | 按 30s / 2m / 10m / 30m / 2h / 6h 退避重试 |
| 请求体不是 JSON、没有 `id` 或字段类型不对 | `400` | 同上（重试也不会成功，请检查中间代理是否改写了请求体） |
| `dispatch` 抛错 | `500` | 同上，重试时 `id` 不变 |
| 签名正确但无法解密（缺私钥、密钥不符，或配置了私钥却收到明文） | `500` | 同上；补上私钥后重试即可成功 |
| 没有 `Content-Length` / 超过 1 MiB | `411` / `413` | 同上 |

- 签名覆盖 `<X-NoWork-Timestamp>.<原始请求体>`：一定要把原始字节交给 `verify_webhook`，不要先解析再序列化。
- 处理逻辑必须在 10 秒内返回；耗时任务先入队再响应。
- 幂等缓存是进程内 LRU；多实例部署时换成 Redis `SET NX EX` 或数据库唯一索引。同一 `id` 的两次投递几乎不会同时到达（只有失败或超时后才重试），如果业务不能容忍重复执行，请在 `dispatch` 内用数据库唯一约束兜底。

## 加密投递（JWE）与成对匿名标识（pairwise）

Webhook 绑定的投递配置开启 `webhook.encrypt` 后（后台「开放平台 → 数据投递」，见 `SDK/docs/DELIVERY.md` 第 6 节），请求体变为 `{"id","event","timestamp","jwe"}` 并带 `X-NoWork-Encryption: JWE`。`receiver.py` 先对外层原始请求体验签，再用 SDK 的 `open_webhook` 解密（核对内层 `id` / `event` / `timestamp` 与外层一致、受保护头 `ctx` 为 `webhook:<端点 ID>`）；配置了私钥却收到明文时拒绝。私钥只放在接收端，后台只保存公钥 JWK；轮换时先把新私钥加进 `NOWORK_DECRYPTION_KEYS=new.jwk,old.jwk`，再在后台启用新公钥，过渡期结束后删掉旧私钥。

投递配置的 `subject` 为 `pairwise` 时，事件里不再有 `userId` / `uid` / `username` / `ip` / `externalId` / `externalName` / `email` / `phone`，改由 `data.user.sub` 标识账号。`events.py` 的 `who()` 两种都能打印；自己的业务按配置选一种作为本地主键。事件字段见 `../webhook-receiver-go/README.md` 与 `docs/PROTOCOL.md` 第 5 节。

## 本地自测与检查

后台「开放平台 → Webhook」里点“测试”会推送一条 `event = ping`；`../discord-role-sync/scripts/send-event.mjs` 可以手工签名发送任意事件。

```bash
python -m unittest -v   # 在本目录执行
```

测试覆盖：明文投递与重复、错误签名 / 篡改 / 过期 / 缺签名 / 非 JSON / 缺 `id` / 时间戳类型错误 / 非 POST、处理失败后重试、LRU 淘汰，以及用仓库共享测试向量（`SDK/go/testdata/vectors.json`）验证加密投递（需要 `cryptography`，没有时跳过）、缺私钥、"配置了私钥却收到明文"和误填公钥。

---

# Webhook receiver example (Python)

A webhook endpoint on the standard library's `http.server` plus the Python SDK in this repo. It verifies signatures, opens encrypted deliveries (JWE), is idempotent by event `id`, and logs `game_role.reviewed`, `user.deleted`, `server.offline`, `user.level_changed`, `connection.bound` and `connection.unbound`. The third-party `cryptography` package is needed only for encrypted deliveries.

`server.py` is the entry point (environment, `ThreadingHTTPServer`, `/healthz`), `receiver.py` the endpoint (`Receiver.handle`: signature, decryption, idempotency, status codes; framework independent), `events.py` the business logic (`dispatch`), `test_receiver.py` the tests. `receiver.py` puts the repo's `SDK/python` on `sys.path`; copied out of the repo, `pip install` the SDK instead.

## Run

Python 3.9+.

```bash
cd SDK/examples/webhook-receiver-python
export NOWORK_WEBHOOK_SECRET=whsec_xxx   # shown once when you create the webhook in 开放平台 → Webhook (Open Platform → Webhooks)
python server.py
```

Variables: `NOWORK_WEBHOOK_SECRET` (required), `HOST` / `PORT` (`0.0.0.0` / `8092`), `WEBHOOK_PATH` (`/nowork/webhook`, must match the URL in the console), `WEBHOOK_TOLERANCE` (`300` seconds, `0` turns the check off), `SEEN_CAPACITY` (`10000`), `NOWORK_DECRYPTION_KEYS` (comma-separated private JWK files, new and old during a rotation; needs `pip install cryptography`, checked at start). Run it behind an HTTPS reverse proxy under `systemd` / `supervisor`.

`Receiver.handle(method, headers, body)` returns `(status, dict)`, so it drops into any framework; the Chinese section above shows Flask. Always pass the raw request bytes.

## Responses

`200 {"status":"ok"}` on first delivery, `200 {"status":"duplicate"}` for an `id` already handled, `401` for a bad or missing signature or a timestamp too far off, `400` for a body that is not JSON, has no `id` or has wrongly typed fields, `500` when `dispatch` raises or a signed delivery cannot be opened (no key, wrong key, or plaintext while keys are set), `411` / `413` without `Content-Length` or over 1 MiB. Anything but 2xx is retried after 30s / 2m / 10m / 30m / 2h / 6h with the same `id`. Return within 10 s and queue slow work. The LRU is per process: with several instances use Redis `SET NX EX` or a unique DB column keyed by `id`, and guard `dispatch` with a unique constraint if a double run would hurt.

## Encrypted deliveries and pairwise subjects

With `webhook.encrypt` on the delivery profile (开放平台 → 数据投递, `SDK/docs/DELIVERY.md` section 6) the body becomes `{"id","event","timestamp","jwe"}` with `X-NoWork-Encryption: JWE`. The receiver verifies the outer body, then opens it with `open_webhook`, which checks that the inner `id` / `event` / `timestamp` repeat the outer ones and that the header `ctx` is `webhook:<endpoint id>`; with keys configured a plaintext delivery is refused. To rotate, add the new private key first (`new.jwk,old.jwk`), activate the new public key in the console, then drop the old one. With `subject: pairwise`, `data.user.sub` replaces `userId` / `uid` / `username` / `externalId` and the other identifying fields; `who()` prints either.

## Test and check

`python -m unittest -v` in this directory: a plain delivery and its duplicate; wrong secret, tampered, stale, unsigned, non-JSON, missing `id`, non-numeric timestamp and non-POST; a failing handler and its retry; LRU eviction; and, with the shared vectors in `SDK/go/testdata/vectors.json`, an encrypted delivery (skipped without `cryptography`), the same without a key, plaintext while keys are set, and a public JWK refused at start. The console's 测试 (Test) button sends `event = ping`; `../discord-role-sync/scripts/send-event.mjs` signs any event by hand.
