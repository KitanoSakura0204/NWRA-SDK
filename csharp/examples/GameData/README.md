# C# game data example

This dependency-free .NET 6+ example reads the site's actual game APIs. It prints capabilities, availability, provenance and counts without dumping role identities, raw fields or tokens. It does not call publisher APIs directly, retry challenges, change a binding or modify policy.

Set `NOWORK_BASE_URL` to the site's HTTP(S) base URL. Then run the project with one of these modes:

```text
dotnet run --project SDK/csharp/examples/GameData/GameData.csproj -- public
dotnet run --project SDK/csharp/examples/GameData/GameData.csproj -- account
dotnet run --project SDK/csharp/examples/GameData/GameData.csproj -- admin
```

- `public` reads Hypergryph descriptors with no credentials. miHoYo has no public route.
- `account` requires `NOWORK_SITE_JWT`, a site login JWT. It reads owned discovery and capabilities.
- `admin` requires `NOWORK_ADMIN_JWT`, a site login JWT with the relevant provider and asset permissions. The backend also checks which accounts the administrator may access.

Optional environment variables:

| Variable | Meaning |
| --- | --- |
| `NOWORK_GAME_PROVIDER` | `hypergryph` (default) or `mihoyo` |
| `NOWORK_GAME_ID` | ID returned by descriptors; no SDK game allowlist |
| `NOWORK_GAME_SECTION` | Capability ID; set with game ID to fetch up to three pages |
| `NOWORK_GAME_ROLE_ID` | miHoYo positive role row ID from `MihoyoStatusAsync`; Hypergryph opaque key from `HypergryphGamesAsync`; leave unset in public mode |
| `NOWORK_GAME_ACCOUNT_ID` | Positive provider binding ID for archive metadata and admin Hypergryph reads |
| `NOWORK_GAME_LOCALE` | `en`, `ja`, `zh_CN`, or `zh_TW` |

Ctrl+C cancels in-flight headers and body reads. The example also has a 60-second total budget. A page-budget exception explicitly marks incomplete traversal. Account and admin clients have separate immutable route scopes. OAuth access tokens, chat sessions and `X-Server-Key` are separate protocols; use `NoworkClient` for the existing Server API.

`GameDataClient` and `GameDataAdminClient` expose archive list/detail/history/policy/audit/export/clear plus asset provider/catalog reads. Admin alone exposes revision-checked `SetFieldPolicyAsync`, refresh and disabled-state writes. Archive export calls the backend's policy-filtered route; serializing a live owner snapshot is not a substitute. Clearing archive data retains administrator policy restrictions. Public catalogs describe reference entries and never imply owned inventory.

Responses are read incrementally with byte ceilings (8 MiB normal / 24 MiB export by default, configurable up to 32 MiB). `GameDataClientOptions.Timeout` covers the response body. Streams are disposed on cancellation or failure, redirects are rejected, and requests do not inherit `HttpClient` default headers or cookies. An optional caller-owned handler must not inject credentials, cookies or redirects; ordinary handler configurations with these features enabled are rejected.

`ApiException`, `TransportException` and `EncryptionException` are shared with the existing SDK. `GameDataErrors.Details(exception)` adds typed HTTP status, raw Retry-After and safe backend data; transport reasons include `timeout`, `network`, `response_too_large`, and malformed-response categories. API refusals and challenges are surfaced once without automatic retries. `GameField.Value` is `JsonElement`, preserving missing/null, zero/false and uint64 precision.

The dependency-free offline fixture executable is at `SDK/csharp/Nowork.Sdk.GameData.Tests`. Run it through the root agent's isolated validation workflow; it needs no publisher credentials or network services. No build or test result is claimed in this delivery.
