"""Protocol fixtures only: never require provider credentials or network access."""
import io
import json
import urllib.error
import urllib.parse
from unittest.mock import MagicMock, patch

import pytest

from nowork_sdk.errors import ApiError, EncryptionError, TransportError
from nowork_sdk.gamedata import (
    MAX_ARCHIVE_EXPORT_BYTES, MAX_RESPONSE_BYTES, AccountGameClient, AdminGameClient, GameSnapshot,
    PaginationError, PublicGameClient, ServerGameClient, archive_field_path,
)


def response(data, *, status=200, headers=None):
    result = MagicMock()
    result.__enter__.return_value = result
    result.status, result.headers = status, headers or {}
    raw = json.dumps(data, ensure_ascii=False).encode("utf-8")
    result.read.side_effect = lambda count: raw[:count]
    return result


def opener_for(data, *, status=200, headers=None):
    opener = MagicMock()
    opener.open.return_value = response({"code": 0, "data": data}, status=status, headers=headers)
    return opener


def game(game_id="arknights", provider="hypergryph"):
    return {"id": game_id, "provider": provider, "name": {"en": "Game", "zh_CN": "游戏"},
            "capabilities": [{"id": "characters", "supported": True, "auth_scope": "role", "status": "ok"}]}


def snapshot(game_id="arknights", next_cursor=""):
    return {"game_id": game_id, "fetched_at": "2026-10-08T00:00:00Z", "next_cursor": next_cursor,
            "sections": [{"id": "characters", "status": "partial", "reason": "public reference only",
                "source": {"source_id": "fixture", "kind": "community", "auth_scope": "public"},
                "entities": [{"id": "char_1", "kind": "character", "name": {"en": "Amiya"},
                    "icon": {"url": "https://assets.example/amiya.png", "source_id": "fixture"},
                    "fields": [{"key": "quantity", "kind": "number", "value": 0, "status": "ok"},
                               {"key": "owned", "kind": "boolean", "value": False, "status": "ok"},
                               {"key": "unknown", "value": None, "status": "private"}]}]},
                {"id": "inventory", "status": "unsupported", "entities": None, "fields": None}]}


def catalog(page=1, count=1, entries=None):
    return {"provider": {"id": "arknights-data", "game_id": "arknights", "source": {"auth_scope": "public"}},
            "cache": {"provider": "arknights-data", "locale": "en", "version": "latest",
                      "fetched_at": "2026-10-08T00:00:00Z", "last_error_code": "source_unavailable"},
            "status": "stale", "list": entries if entries is not None else [{"id": "char_1", "kind": "character"}],
            "count": count, "page": page, "limit": 1}


def sent(opener):
    return opener.open.call_args.args[0]


def query(opener):
    return urllib.parse.parse_qs(urllib.parse.urlsplit(sent(opener).full_url).query)


def test_public_descriptors_send_no_auth_and_use_finite_timeout():
    opener = opener_for({"games": [game()]})
    result = PublicGameClient("https://account.example", timeout=7, opener=opener).games()
    assert result[0].capabilities[0].auth_scope == "role"
    assert result[0].name["zh_CN"] == "游戏"
    assert sent(opener).full_url == "https://account.example/api/hypergryph/games"
    assert sent(opener).get_header("Authorization") is None
    assert sent(opener).get_header("X-server-key") is None
    assert opener.open.call_args.kwargs["timeout"] == 7
    opener.open.return_value.read.assert_called_once_with(MAX_RESPONSE_BYTES + 1)


def test_nested_snapshot_retains_provenance_zero_false_null_and_unavailable_sections():
    opener = opener_for(snapshot())
    result = AccountGameClient("https://account.example", "site-jwt", opener=opener).snapshot(
        "hypergryph", "arknights", role_id="arknights:role:7", sections=["characters", "inventory"], locale="zh_CN")
    assert result.sections[0].source.auth_scope == "public"
    assert result.sections[0].entities[0].icon.url.startswith("https://")
    assert [f.value for f in result.sections[0].entities[0].fields] == [0, False, None]
    assert type(result.sections[0].entities[0].fields[1].value) is bool
    assert result.sections[1].status == "unsupported"
    assert result.sections[1].entities == []
    assert result.role is None
    assert result.sections[0].entities[0].policy.visibility == "owner"
    assert query(opener)["roleId"] == ["arknights:role:7"]
    assert query(opener)["sections"] == ["characters,inventory"]
    assert sent(opener).get_header("Authorization") == "Bearer site-jwt"
    assert sent(opener).get_header("X-server-key") is None


def test_mihoyo_uses_array_games_data_route_and_stored_numeric_role_id():
    opener = opener_for([game("genshin", "mihoyo")])
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    assert client.games("mihoyo")[0].id == "genshin"
    opener.open.return_value = response({"code": 0, "data": snapshot("genshin")})
    client.snapshot("mihoyo", "genshin", role_id=14, cursor="opaque/a+b=?&x=2")
    assert urllib.parse.urlsplit(sent(opener).full_url).path == "/api/account/mihoyo/games/genshin/data"
    assert query(opener)["roleId"] == ["14"]
    assert query(opener)["cursor"] == ["opaque/a+b=?&x=2"]
    with pytest.raises(ValueError):
        client.snapshot("mihoyo", "genshin", role_id="upstream-uid")


def test_provider_role_discovery_preserves_existing_wrapper_shapes():
    opener = opener_for({"games": [game()], "roles": [{"id": "r7", "game_id": "arknights", "level": None}],
                         "discovery": {"status": "partial", "updatedAt": "2026-10-08T00:00:00Z"}, "selectedRoleId": "r7"})
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    roles = client.hypergryph_games()
    assert roles.roles[0].id == roles.selected_role_id == "r7"
    assert roles.discovery.status == "partial"
    assert roles.roles[0].level is None
    client.select_hypergryph_role("r7")
    assert sent(opener).method == "PUT"
    assert json.loads(sent(opener).data) == {"roleId": "r7"}
    opener.open.return_value = response({"code": 0, "data": {"enabled": False, "bound": True,
        "state": "verify", "roles": [{"id": 14, "gameId": "genshin", "gameUid": "100000001", "gameBiz": "hk4e_cn"}]}})
    status = client.mihoyo_status()
    assert status.roles[0].id == 14
    assert status.roles[0].game_uid == "100000001"
    assert status.state == "verify"
    assert status.enabled is False


@pytest.mark.parametrize("provider,role", [("mihoyo", None), ("hypergryph", "r7")])
def test_public_client_refuses_private_scope_before_network(provider, role):
    opener = MagicMock()
    with pytest.raises(ValueError):
        PublicGameClient("https://account.example", opener=opener).snapshot(provider, "game", role_id=role)
    opener.open.assert_not_called()


def test_snapshot_pagination_stops_at_end_and_detects_cycle_and_budget():
    opener = MagicMock()
    opener.open.side_effect = [response({"code": 0, "data": snapshot(next_cursor="next")}),
                               response({"code": 0, "data": snapshot()})]
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    assert len(list(client.snapshot_pages("hypergryph", "arknights", max_pages=2))) == 2
    assert query(opener)["cursor"] == ["next"]
    opener.open.side_effect = None
    opener.open.return_value = response({"code": 0, "data": snapshot(next_cursor="repeat")})
    pages = client.snapshot_pages("hypergryph", "arknights", max_pages=3)
    next(pages)
    next(pages)
    with pytest.raises(PaginationError, match="repeated"):
        next(pages)
    pages = client.snapshot_pages("hypergryph", "arknights", max_pages=1)
    next(pages)
    with pytest.raises(PaginationError, match="budget"):
        next(pages)


def test_catalog_requires_jwt_exposes_stale_cache_and_paginates():
    opener = MagicMock()
    opener.open.side_effect = [response({"code": 0, "data": catalog(page=1, count=2)}),
                               response({"code": 0, "data": catalog(page=2, count=2)})]
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    pages = list(client.asset_catalog_pages("arknights-data", search="Amiya & 罗德岛", kind="character", limit=1))
    assert len(pages) == 2
    assert pages[0].cache.last_error_code == "source_unavailable"
    assert pages[0].provider.source.auth_scope == "public"
    assert pages[0].status == "stale"
    assert query(opener)["q"] == ["Amiya & 罗德岛"]
    assert query(opener)["page"] == ["2"]
    assert sent(opener).get_header("Authorization") == "Bearer jwt"


def test_admin_assets_and_account_snapshot_use_scoped_routes():
    opener = opener_for({"provider": "arknights-data", "disabled": True})
    client = AdminGameClient("https://account.example", "admin-jwt", opener=opener)
    assert client.set_asset_disabled("arknights-data", True, locale="en", version="latest").disabled is True
    assert sent(opener).full_url.endswith("/api/admin/game-assets/providers/arknights-data/state")
    assert json.loads(sent(opener).data) == {"locale": "en", "version": "latest", "disabled": True}
    client.refresh_asset_provider("arknights-data")
    assert sent(opener).method == "POST"
    assert sent(opener).full_url.endswith("/providers/arknights-data/refresh")
    opener.open.return_value = response({"code": 0, "data": snapshot()})
    client.hypergryph_snapshot(9, "arknights", role_id="r7")
    assert urllib.parse.urlsplit(sent(opener).full_url).path == "/api/admin/hypergryph/accounts/9/games/arknights/snapshot"
    assert sent(opener).get_header("Authorization") == "Bearer admin-jwt"


def test_server_key_is_only_used_for_existing_shared_role_endpoint():
    opener = opener_for({"uid": 42, "status": "ok", "serverTime": 123, "roles": [
        {"uid": "10001", "nickname": "Doctor", "apNow": 8, "summary": {"roster": [{"id": "char_1", "level": 80}]}}]})
    result = ServerGameClient("https://account.example", "server-secret", opener=opener).hypergryph_roles(42)
    assert sent(opener).full_url == "https://account.example/server/v1/hypergryph/roles?uid=42"
    assert sent(opener).get_header("X-server-key") == "server-secret"
    assert sent(opener).get_header("Authorization") is None
    assert result.roles[0].ap_now == 8
    assert result.roles[0].summary["roster"][0]["level"] == 80


@pytest.mark.parametrize("timeout", [0, -1, float("inf"), float("nan"), 61, True])
def test_invalid_timeouts_are_rejected(timeout):
    with pytest.raises(ValueError):
        PublicGameClient("https://account.example", timeout=timeout)


@pytest.mark.parametrize("credential", ["", "jwt\nother", "jwt other", None])
def test_authenticated_clients_require_safe_nonempty_credentials(credential):
    for client in (AccountGameClient, AdminGameClient, ServerGameClient):
        with pytest.raises(ValueError):
            client("https://account.example", credential)


@pytest.mark.parametrize("base", ["file:///tmp/secrets", "https://user:pass@example.test", "https://example.test?jwt=secret",
                                    "https://@example.test", "https://example.test#fragment", "https://example.test:wrong", "https://example.test\n"])
def test_invalid_base_urls_are_rejected(base):
    with pytest.raises(ValueError):
        AccountGameClient(base, "jwt")


@pytest.mark.parametrize("kwargs", [{"limit": 0}, {"limit": 101}, {"limit": True}, {"cursor": "a" * 2049},
                                    {"sections": "characters"}, {"sections": ["a,b"]}, {"locale": "invalid"}])
def test_snapshot_input_bounds_fail_before_network(kwargs):
    opener = MagicMock()
    with pytest.raises(ValueError):
        AccountGameClient("https://account.example", "jwt", opener=opener).snapshot("hypergryph", "arknights", **kwargs)
    opener.open.assert_not_called()


@pytest.mark.parametrize("envelope,status,error", [([], 200, TransportError), ({"code": True, "data": {}}, 200, TransportError),
    ({"code": 0}, 200, TransportError), ({"code": 0, "data": {}}, 503, TransportError),
    ({"code": 429, "message": "rate limited"}, 429, ApiError), ({"code": 0, "data": {"games": "wrong"}}, 200, TransportError)])
def test_envelope_and_http_errors_are_not_empty_success(envelope, status, error):
    opener = MagicMock()
    opener.open.return_value = response(envelope, status=status)
    with pytest.raises(error):
        PublicGameClient("https://account.example", opener=opener).games()
    assert opener.open.call_count == 1


def test_http_error_envelope_preserves_business_code_without_retry():
    opener = MagicMock()
    body = io.BytesIO(b'{"code":403,"message":"data is private"}')
    opener.open.side_effect = urllib.error.HTTPError("https://account.example", 403, "Forbidden", {}, body)
    with pytest.raises(ApiError) as error:
        AccountGameClient("https://account.example", "jwt", opener=opener).games("mihoyo")
    assert error.value.code == 403
    assert body.closed
    assert opener.open.call_count == 1


def test_response_size_and_encryption_are_fail_closed():
    opener = MagicMock()
    oversized = response({})
    oversized.read.side_effect = None
    oversized.read.return_value = b" " * (MAX_RESPONSE_BYTES + 1)
    opener.open.return_value = oversized
    with pytest.raises(TransportError, match="8 MiB"):
        PublicGameClient("https://account.example", opener=opener).games()
    opener.open.return_value = response({"code": 0, "data": {"jwe": "encrypted-payload"}})
    with pytest.raises(EncryptionError):
        ServerGameClient("https://account.example", "key", opener=opener).hypergryph_roles(42)


def test_catalog_inconsistent_page_and_bounded_iteration_are_not_silent_success():
    opener = opener_for(catalog(page=2))
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    with pytest.raises(TransportError, match="requested page"):
        client.asset_catalog("arknights-data", limit=1)
    opener.open.return_value = response({"code": 0, "data": catalog(count=2)})
    pages = client.asset_catalog_pages("arknights-data", limit=1, max_pages=1)
    assert next(pages).has_more
    with pytest.raises(PaginationError, match="budget"):
        next(pages)
    opener.open.return_value = response({"code": 0, "data": catalog(count=2, entries=[])})
    pages = client.asset_catalog_pages("arknights-data", limit=1)
    next(pages)
    with pytest.raises(PaginationError, match="empty page"):
        next(pages)


def test_catalog_default_variant_is_selected_by_the_manifest_not_an_assumed_language():
    opener = opener_for(catalog())
    AccountGameClient("https://account.example", "jwt", opener=opener).asset_catalog("arknights-data", limit=1)
    sent_query = urllib.parse.parse_qs(urllib.parse.urlsplit(sent(opener).full_url).query, keep_blank_values=True)
    assert sent_query["locale"] == [""]
    assert sent_query["version"] == [""]


def test_response_cursor_bounds_and_nonfinite_json_are_transport_failures():
    opener = opener_for(snapshot(next_cursor="a" * 2049))
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    with pytest.raises(TransportError, match="cursor"):
        client.snapshot("hypergryph", "arknights")
    opener.open.return_value = response({"code": 0, "data": {"value": float("nan")}})
    with pytest.raises(TransportError, match="decoding"):
        client.games("mihoyo")


def test_default_opener_refuses_redirects_before_forwarding_credentials():
    with patch("urllib.request.build_opener") as build:
        AccountGameClient("https://account.example", "jwt")
    handler = build.call_args.args[0]
    assert handler.redirect_request(None, None, 302, "Found", {}, "https://other.example") is None


@pytest.mark.parametrize("change", [{"game_id": "other"}, {"sections": "not a list"},
                                   {"sections": [{"id": "x", "fields": [{"key": "x", "value": {"token": "not allowed"}}]}]}])
def test_malformed_typed_snapshots_are_rejected(change):
    payload = snapshot()
    payload.update(change)
    opener = opener_for(payload)
    with pytest.raises(TransportError):
        AccountGameClient("https://account.example", "jwt", opener=opener).snapshot("hypergryph", "arknights")


def test_null_lists_are_supported_but_boolean_role_ids_are_not_numbers():
    assert GameSnapshot.from_dict({"game_id": "arknights", "fetched_at": "now", "sections": None}).sections == []
    opener = MagicMock()
    with pytest.raises(ValueError):
        AccountGameClient("https://account.example", "jwt", opener=opener).snapshot("mihoyo", "genshin", role_id=True)
    opener.open.assert_not_called()


def archive_entry(provider="hypergryph"):
    return {"provider": provider, "role_id": "", "version": 1, "revision": 17, "status": "stale",
            "stale": True, "expires_at": "2026-10-08T01:00:00Z", "snapshot": snapshot()}


def archive_policy(revision=17):
    return {"id": 4, "game_id": "arknights", "path": "/sections/profile/fields/uid", "visibility": "secret",
            "exportable": False, "revision": revision, "updated_at": "2026-10-08T01:00:00Z"}


def archive_history_entry(entry_id=19):
    return {"id": entry_id, "game_id": "arknights", "role_id": "", "revision": 17,
            "status": "stale", "bytes": 600, "created_at": "2026-10-08T01:00:00Z"}


def archive_audit(entry_id=19):
    return {"id": entry_id, "game_id": "arknights", "path": "/sections/profile/fields/uid", "revision": 17,
            "previous": "null", "current": '{"visibility":"secret","exportable":false}',
            "created_at": "2026-10-08T01:00:00Z"}


def archive_export():
    return {"scope": {"site_id": 0, "user_id": 42, "account_id": 7}, "provider": "hypergryph",
            "exported_at": "2026-10-08T02:00:00Z", "snapshots": [archive_entry()],
            "history": [archive_history_entry()], "policies": [archive_policy()], "policy_audit": [archive_audit()]}


@pytest.mark.parametrize("client_class,prefix,token", [
    (AccountGameClient, "/api/account", "site-jwt"), (AdminGameClient, "/api/admin", "admin-jwt")])
def test_archive_lists_entries_and_redaction_use_only_the_scoped_jwt(client_class, prefix, token):
    metadata = {"game_id": "arknights", "role_id": "", "version": 1, "revision": 17, "status": "stale",
                "stale": True, "bytes": 600, "fetched_at": "2026-10-08T00:00:00Z", "expires_at": "2026-10-08T01:00:00Z"}
    opener = opener_for([metadata])
    client = client_class("https://account.example", token, opener=opener)
    rows = client.archive_snapshots("hypergryph", 7)
    assert rows[0].role_id == ""
    assert rows[0].stale is True
    assert sent(opener).full_url == f"https://account.example{prefix}/hypergryph/accounts/7/archive/snapshots"
    assert sent(opener).get_header("Authorization") == "Bearer " + token
    assert sent(opener).get_header("X-server-key") is None
    opener.open.return_value = response({"code": 0, "data": archive_entry()})
    entry = client.archived_snapshot("hypergryph", 7, "arknights", "arknights:42")
    assert query(opener) == {"game_id": ["arknights"], "role_id": ["arknights:42"]}
    assert entry.role_id == ""  # Server redaction must not be replaced by the requested identity.
    assert entry.snapshot.role is None
    assert entry.snapshot.sections[0].entities[0].fields[0].value == 0


def test_archive_status_exposes_internal_binding_ids_without_scope_override():
    opener = opener_for({"enabled": False, "account": {"id": 7, "status": "expired", "sklandUid": "upstream"}})
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    assert client.hypergryph_status().account.id == 7
    assert sent(opener).full_url.endswith("/api/account/hypergryph")
    opener.open.return_value = response({"code": 0, "data": {"enabled": True, "bound": True, "roles": [], "accountId": 11}})
    assert client.mihoyo_status().account_id == 11


def test_archive_history_paginates_backwards_without_coercing_large_ids():
    first_id = 2**63 + 15
    opener = MagicMock()
    opener.open.side_effect = [response({"code": 0, "data": [archive_history_entry(first_id)]}),
                               response({"code": 0, "data": [archive_history_entry(first_id - 1)]}),
                               response({"code": 0, "data": []})]
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    pages = list(client.archive_history_pages("hypergryph", 7, limit=1, max_pages=3))
    assert [page[0].id for page in pages] == [first_id, first_id - 1]
    assert query(opener) == {"before": [str(first_id - 1)], "limit": ["1"]}
    opener.open.side_effect = None
    opener.open.return_value = response({"code": 0, "data": [archive_history_entry(19)]})
    with pytest.raises(PaginationError, match="descend"):
        client.archive_history("hypergryph", 7, before=19, limit=1)
    pages = client.archive_history_pages("hypergryph", 7, limit=1, max_pages=1)
    next(pages)
    with pytest.raises(PaginationError, match="budget"):
        next(pages)


def test_archive_policy_reads_audit_and_revision_writes_match_backend():
    opener = opener_for([archive_policy()])
    client = AdminGameClient("https://account.example", "admin", opener=opener)
    policy = client.archive_policies("hypergryph", 7, "arknights")[0]
    assert policy.revision == 17
    assert query(opener) == {"game_id": ["arknights"]}
    opener.open.return_value = response({"code": 0, "data": [archive_audit()]})
    audit = client.archive_policy_audit("hypergryph", 7, before=20, limit=10)[0]
    assert audit.previous == "null"
    assert json.loads(audit.current)["exportable"] is False
    assert sent(opener).full_url.endswith("/archive/policy-audit?before=20&limit=10")
    opener.open.return_value = response({"code": 0, "data": archive_policy(18)})
    updated = client.set_archive_policy("hypergryph", 7, "arknights", policy.path,
                                        visibility="secret", exportable=False, revision=policy.revision)
    assert updated.revision == 18
    assert sent(opener).method == "PUT"
    assert sent(opener).full_url.endswith("/api/admin/hypergryph/accounts/7/archive/policies")
    assert json.loads(sent(opener).data) == {"game_id": "arknights", "path": policy.path,
        "visibility": "secret", "exportable": False, "revision": 17}
    client.set_archive_policy("hypergryph", 7, "arknights", policy.path, visibility="secret", exportable=False, revision=0)
    assert json.loads(sent(opener).data)["revision"] == 0
    opener.open.reset_mock()
    opener.open.return_value = response({"code": 409, "message": "conflict"})
    with pytest.raises(ApiError) as error:
        client.set_archive_policy("hypergryph", 7, "arknights", policy.path, visibility="secret", exportable=False, revision=17)
    assert error.value.code == 409
    assert opener.open.call_count == 1


def test_archive_field_path_escapes_upstream_ids_and_invalid_updates_never_send():
    path = archive_field_path("profile/region", "level~max", entity_id="char/~1")
    assert path == "/sections/profile~1region/entities/char~1~01/fields/level~0max"
    opener = MagicMock()
    client = AdminGameClient("https://account.example", "jwt", opener=opener)
    for changes in ({"path": "/sections/a/fields/b~2"}, {"revision": True}, {"revision": -1},
                    {"revision": 2**64}, {"exportable": 0}, {"visibility": "anyone"}):
        args = {"path": path, "visibility": "owner", "exportable": False, "revision": 0}
        args.update(changes)
        with pytest.raises(ValueError):
            client.set_archive_policy("hypergryph", 7, "arknights", **args)
    opener.open.assert_not_called()


@pytest.mark.parametrize("client_class,prefix", [(AccountGameClient, "account"), (AdminGameClient, "admin")])
def test_archive_export_is_typed_server_redacted_and_clear_accepts_null_only_as_success(client_class, prefix):
    opener = opener_for(archive_export())
    client = client_class("https://account.example", "jwt", opener=opener)
    result = client.export_archive("hypergryph", 7)
    assert result.scope.site_id == 0
    assert result.scope.account_id == 7
    assert result.snapshots[0].role_id == ""
    assert result.policies[0].visibility == "secret"
    assert sent(opener).full_url.endswith(f"/api/{prefix}/hypergryph/accounts/7/archive/export")
    opener.open.return_value.read.assert_called_once_with(MAX_ARCHIVE_EXPORT_BYTES + 1)
    opener.open.return_value = response({"code": 0, "data": None})
    assert client.clear_archive("hypergryph", 7) is None
    assert sent(opener).method == "DELETE"
    assert sent(opener).full_url.endswith(f"/api/{prefix}/hypergryph/accounts/7/archive")
    opener.open.return_value.read.assert_called_once_with(MAX_RESPONSE_BYTES + 1)
    opener.open.return_value = response({"code": 0})
    with pytest.raises(TransportError):
        client.clear_archive("hypergryph", 7)
    opener.open.return_value = response({"code": 403, "message": "denied"})
    with pytest.raises(ApiError):
        client.clear_archive("hypergryph", 7)


def test_large_archive_export_limit_is_per_request_and_still_bounded():
    # A valid JSON envelope can exceed 16 MiB with metadata while remaining <20 MiB.
    raw = json.dumps({"code": 0, "data": archive_export()}).encode() + b" " * (16 * 1024 * 1024)
    large = response({})
    large.read.side_effect = lambda count: raw[:count]
    opener = MagicMock()
    opener.open.return_value = large
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    assert client.export_archive("hypergryph", 7).scope.account_id == 7
    assert large.read.call_args.args == (MAX_ARCHIVE_EXPORT_BYTES + 1,)
    with pytest.raises(TransportError, match="8 MiB"):
        client.archive_snapshots("hypergryph", 7)
    assert large.read.call_args.args == (MAX_RESPONSE_BYTES + 1,)
    raw = b" " * (MAX_ARCHIVE_EXPORT_BYTES + 1)
    with pytest.raises(TransportError, match="20 MiB"):
        client.export_archive("hypergryph", 7)


def test_archive_methods_are_absent_from_public_and_server_clients_and_policy_write_is_admin_only():
    for client in (PublicGameClient, ServerGameClient):
        for method in ("archive_snapshots", "archived_snapshot", "archive_history", "archive_policies",
                       "archive_policy_audit", "export_archive", "clear_archive", "set_archive_policy"):
            assert not hasattr(client, method)
    assert not hasattr(AccountGameClient, "set_archive_policy")


@pytest.mark.parametrize("account_id", [0, -1, True, 2**63, "7"])
def test_invalid_archive_binding_ids_fail_before_network(account_id):
    opener = MagicMock()
    with pytest.raises(ValueError):
        AccountGameClient("https://account.example", "jwt", opener=opener).archive_snapshots("mihoyo", account_id)
    opener.open.assert_not_called()


def test_archive_wrong_scope_and_null_read_fail_instead_of_returning_empty_data():
    payload = archive_export()
    payload["scope"]["account_id"] = 8
    opener = opener_for(payload)
    client = AccountGameClient("https://account.example", "jwt", opener=opener)
    with pytest.raises(TransportError, match="scope"):
        client.export_archive("hypergryph", 7)
    opener.open.return_value = response({"code": 0, "data": None})
    with pytest.raises(TransportError):
        client.archive_history("hypergryph", 7)
    for before, limit in ((-1, 50), (True, 50), (2**64, 50), (0, 101)):
        with pytest.raises(ValueError):
            client.archive_history("hypergryph", 7, before=before, limit=limit)
