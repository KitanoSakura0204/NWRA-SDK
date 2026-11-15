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

## OIDC discovery、JWKS 与 nonce 校验

`NoworkOIDC.discover()` 和 `jwks()` 返回保留扩展字段的字典，分别读取配置 issuer 的 `/.well-known/openid-configuration` 与 `/jwks.json`。Discovery 的 `issuer` 必须与构造函数去掉末尾 `/` 后的 issuer 完全相同，且 `authorization_endpoint`、`token_endpoint`、`jwks_uri` 都必须是非空字符串；JWKS 的 `keys` 必须是对象数组。SDK 不跟随元数据公布的 URL，也不跟随这两个读取请求的 HTTP 重定向。原有授权和 token 方法继续使用固定 NoWork 路径。

这两个公开 GET 不携带 `client_secret`、Bearer token、`X-Server-Key` 或登录 cookie。每次读取使用独立的 urllib opener，隔离通过 `urllib.request.install_opener()` 安装的认证、cookie 和额外请求头；标准环境代理（包括用户配置的代理认证）继续生效。它们使用现有 `timeout`，不新增 OIDC 传输注入参数；原有 OAuth 请求仍使用 `urllib.request.urlopen()`。非 2xx、超过 1 MiB、无效 JSON、错误根类型、缺失字段和 issuer 不匹配都会抛出 `TransportError`，不会回退到其他 issuer 或未校验的文档。

ID token 的签名及 nonce 校验需要可选依赖 `cryptography`，discovery 和 JWKS 读取本身只使用标准库：

```python
from nowork_sdk import NoworkOIDC, create_pkce_pair, create_state, parse_login_callback
from nowork_sdk.delivery import verify_id_token

oidc = NoworkOIDC(issuer, client_id, client_secret, redirect_uri)
metadata = oidc.discover()
signing_keys = oidc.jwks()
pkce = create_pkce_pair()
login_context = {"state": create_state(), "nonce": create_state(), "verifier": pkce.verifier}
# 在服务端会话中保存 login_context，回调时读取，并在完成后一次性删除。
url = oidc.authorize_url(login_context["state"], login_context["nonce"], code_challenge=pkce.challenge)

# 收到浏览器回调后先验证 state，再交换授权码并验证 ID token。
code = parse_login_callback(callback_url, login_context["state"], oidc.issuer)
tokens = oidc.exchange(code, login_context["verifier"])
claims = verify_id_token(
    tokens["id_token"], signing_keys, oidc.issuer, oidc.client_id,
    expected_nonce=login_context["nonce"],
)
```

`verify_id_token(token, jwks, issuer=None, audience=None, now=None, expected_nonce=None)` 先执行 RS256 签名、issuer、audience 和 `exp` / `nbf` 校验（沿用 60 秒时钟宽限），再校验 nonce；返回原始签名声明，不改写时间。显式提供的 `expected_nonce` 必须是非空字符串，并与签名中的字符串 `nonce` 完全相同；空值、缺失声明、类型错误或不匹配都会抛出 `DecryptError`。省略它或传入 `None` 保留旧行为，原有第五个位置参数 `now` 不变。

加密 ID token 使用应用自己的解密私钥，并同样校验内层 JWT 签名和 nonce：

```python
from nowork_sdk.delivery import decrypt_id_token

claims = decrypt_id_token(
    tokens["id_token"], app_private_keys, signing_keys, oidc.issuer, oidc.client_id,
    expected_nonce=login_context["nonce"],
)
```

`decrypt_id_token(token, keys, jwks, issuer=None, audience=None, expected_nonce=None)` 的 nonce 参数追加在原有参数之后。OIDC 应用私钥、OAuth client credentials、账户 Bearer token 和游戏服务器 `X-Server-Key` 分别用于各自协议。

Public metadata reads preserve the standard environment proxy configuration and the existing timeout while isolating global urllib authentication/cookie handlers. Both fixed endpoints reject redirects, invalid documents and responses larger than 1 MiB. Pass the nonce saved for this login attempt to `verify_id_token(..., expected_nonce=nonce)` or `decrypt_id_token(..., expected_nonce=nonce)`; both validate the inner RS256 token before checking nonce, and omitted nonce arguments retain existing behavior.

Python 测试从仓库根目录运行（安装 `cryptography` 后可运行全部加密测试）：

```powershell
$env:PYTHONPATH = "$PWD/python"
python -m unittest discover -s python/tests -v
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
