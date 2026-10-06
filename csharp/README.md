# NoWork.Sdk (C# / .NET 6+)

零第三方依赖，适用于 Unity 服务端 / 独立 .NET 游戏服务器 / ASP.NET 应用。

```csharp
var client = new NoworkClient("https://account.example.com", Environment.GetEnvironmentVariable("NOWORK_SERVER_KEY")!);
var r = await client.CheckWhitelistAsync(player.Name, player.Uuid);
if (!r.Allowed) Kick(player, $"未绑定账号: {r.Reason}");
await client.HeartbeatAsync(online: 12, max: 50, version: "1.0");

var oidc = new NoworkOidc(issuer, clientId, clientSecret, redirectUri);
var url = oidc.AuthorizeUrl(state, nonce, "openid profile email game_roles");
var token = await oidc.ExchangeAsync(code);
var me = await oidc.UserInfoAsync(token.GetProperty("access_token").GetString()!);
```
