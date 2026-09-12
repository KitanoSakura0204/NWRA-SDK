# lounge-sdk-go

```bash
go get github.com/wangework/lounge-sdk-go
```

```go
import lounge "github.com/wangework/lounge-sdk-go"

c := lounge.New("https://account.example.com", lounge.WithServerKey(os.Getenv("LOUNGE_SERVER_KEY")))
res, err := c.CheckWhitelist(ctx, player.Name, player.UUID.String())
switch {
case err != nil:            // 网络异常: 按服务器策略决定放行/拒绝, 建议使用本地缓存
case !res.Allowed:          // 拒绝, res.Reason 给出原因
default:                    // 放行, res.Player 含 UID / 昵称 / 信任等级
}
```

OIDC：

```go
o := lounge.NewOIDC(issuer, clientID, clientSecret, redirectURI)
http.Redirect(w, r, o.AuthorizeURL(state, nonce, "openid profile email game_roles"), 302)
// 回调
tok, _ := o.Exchange(ctx, r.URL.Query().Get("code"))
me, _  := o.UserInfo(ctx, tok.AccessToken)
```

示例：`go run ./examples/paper-check`（需要 `LOUNGE_SERVER_KEY`）。
