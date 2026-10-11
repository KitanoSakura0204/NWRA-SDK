# nowork-sdk (Python)

Python 3.9+，普通请求使用标准库；加密交付可选安装 `cryptography`。

```bash
pip install nowork-sdk   # 或 pip install -e ./python
```

```python
from nowork_sdk import NoworkClient, NoworkOIDC, TransportError

client = NoworkClient("https://account.example.com", server_key="lsk_xxx")
r = client.check_whitelist(name="Steve", uuid="069a79f4-...")
if not r.allowed:
    kick(f"未绑定账号: {r.reason}")
client.heartbeat(online=12, max_players=50, version="1.21.1")

oidc = NoworkOIDC(issuer, client_id, client_secret, redirect_uri)
url = oidc.authorize_url(state, nonce, "openid profile email game_roles")
token = oidc.exchange(code)
me = oidc.userinfo(token["access_token"])
```

## 服务器响应加密

服务器开启交付加密后，`whitelist` / `whitelist/check` 返回 JWE。未配置私钥时，SDK 抛出 `EncryptionError`，不能将其当成网络异常而放行玩家。配置私钥后，这两个接口必须返回密文，明文响应也会被拒绝；心跳、GM、玩家数据和其他当前未加密接口继续接受正常明文响应。

```bash
pip install cryptography
```

```python
import json
from pathlib import Path
from nowork_sdk import EncryptionError, NoworkClient

keys = [json.loads(Path("/etc/nowork/server-current.jwk").read_text(encoding="utf-8"))]
# 轮换过渡期可同时加入旧私钥；必须是私有 JWK 对象，不能直接传文件名或 JSON 字符串。
client = NoworkClient(
    "https://account.example.com", "lsk_xxx", 10.0,
    decryption_keys=keys,
    server_code="survival",  # 后台的服务器代码，不是显示名称或 X-Server-Key
)
try:
    result = client.check_whitelist(name="Steve")
except EncryptionError:
    kick("无法验证加密白名单，请稍后重试")
else:
    if not result.allowed:
        kick(result.message or result.reason)
```

`decryption_keys` 接受一个私有 JWK 字典或多个 JWK 的可迭代对象，沿用 [DELIVERY 协议](../docs/DELIVERY.md) 的 RSA-OAEP-256、EC P-256 和 X25519 格式。客户端复制配置，后续请求可重复使用生成器提供的密钥；更换配置时创建新客户端。密文的 `kid` 用于选择轮换中的密钥，无 `kid` 时逐一尝试。

所有解密结果都检查经过认证的 `ctx: server:<服务器代码>:<请求路由>`；配置 `server_code` 时还必须与指定代码完全相等。建议始终配置它，尤其多个服务器共用密钥时。省略它只核对 `server` 类型、非空服务器代码和精确路由，不能验证响应属于哪个服务器。私钥不会发给服务器；`NoworkClient` 仍只使用 `X-Server-Key` 认证。账户 API 的 JWT 客户端独立使用 Bearer token。

解密后的白名单必须明确给出整数 `mode`（`0`、`1` 或 `2`），入服检查必须给出布尔型 `allowed`。缺失或类型错误会抛出 `EncryptionError`，不会回退为关闭白名单，也不会把字符串 `"false"` 当成允许。合法的 `mode: 0` 仍表示关闭白名单。

无法解密、密钥不匹配、上下文错误、损坏的 JWE 或无效的解密 JSON 都抛出 `EncryptionError`（带 `route`、`reason` 和可选原始异常 `__cause__`）。它不继承 `TransportError`。业务拒绝仍为 `ApiError`；HTTP、网络、外层响应格式或超限错误仍为 `TransportError`。主服务器传输最多读取 8 MiB 响应，超限即关闭响应并拒绝，不返回部分数据；游戏数据客户端的 8 MiB 限制和存档导出的 20 MiB 限制独立保留。原有第三个位置参数 `timeout` 默认 10 秒，行为不变。

`raw(method, path, query=None, body=None)` 始终保留原始 `body`、`envelope`、`data` 和 `encrypted` 标记，不自动解密，也不执行明文降级检查；业务、HTTP、JSON 和大小检查仍生效。手动使用时可将 `raw.envelope` 交给 `nowork_sdk.delivery.open_server_data(..., ctx="server:survival:whitelist")`。单独的 `jwe` 字段即使值损坏也会标为加密响应。

离线测试或自定义传输可通过关键词参数 `opener` 注入提供 `open(request, timeout=...)` 的对象，响应须支持上下文管理、`read(size)`、`status` 和 `headers`；不传时继续使用 `urllib.request.urlopen`。

## Encrypted Server API responses

Pass `decryption_keys` as a private JWK mapping or an iterable of mappings, and set `server_code` to the expected server code. Install `cryptography` only when opening encrypted replies. The existing delivery helpers handle RSA-OAEP-256, P-256 and X25519, including overlapping rotation keys. Every authenticated context must name the exact request route and, when configured, the expected server.

Encrypted replies without usable keys raise `EncryptionError`. With keys configured, only the two Backend-encrypted whitelist routes also reject plaintext; heartbeat, GM and other ordinary plaintext routes keep working. Treat `EncryptionError` as a refusal, never as permission to join. Private keys stay local and ServerKey authentication remains separate from account JWT authentication.

Decrypted whitelist data requires an explicit integer `mode` from 0 to 2, and join checks require a boolean `allowed`. Missing or invalid fields raise `EncryptionError`; a valid explicit `mode: 0` remains supported.

`raw()` preserves the original envelope without decryption or downgrade checks. Wire responses are bounded to 8 MiB; oversized replies raise `TransportError` before decryption, and no partial result is returned. The separate game archive export limit remains 20 MiB. Existing positional timeout arguments and API/network error categories remain compatible.
