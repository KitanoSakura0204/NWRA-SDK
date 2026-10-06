# nowork-sdk (Python)

标准库实现，Python 3.9+。

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
