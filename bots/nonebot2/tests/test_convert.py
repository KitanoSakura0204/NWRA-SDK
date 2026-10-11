from nonebot_plugin_nowork.convert import nowork_message_to_text
from nonebot_plugin_nowork.platform import platform_of


def test_nowork_message_to_text_renders_segments():
    message = [
        {"type": "text", "data": {"text": "hello "}},
        {"type": "mention", "data": {"userId": "42"}},
        {"type": "link", "data": {"label": "docs", "url": "https://example.com"}},
        {"type": "image", "data": {"url": "https://example.com/a.png"}},
    ]
    text = nowork_message_to_text(message)
    assert text == "hello @42docs (https://example.com)[image: https://example.com/a.png]"


def test_nowork_message_to_text_empty():
    assert nowork_message_to_text(None) == ""
    assert nowork_message_to_text([]) == ""


def test_canonical_server_keys_and_markdown_keep_readable_content():
    message = [
        {"type": "markdown", "data": {"text": "**Security alert** "}},
        {"type": "mention", "data": {"user_id": "42", "userId": "legacy"}},
        {"type": "text", "data": {"text": " "}},
        {"type": "link", "data": {"title": "Support", "label": "legacy", "url": "https://account.example/help"}},
    ]
    assert nowork_message_to_text(message) == "**Security alert** @42 Support (https://account.example/help)"


def test_platform_of_known_adapter():
    assert platform_of("OneBot V11", None) == "onebot11"
    assert platform_of("Telegram", None) == "telegram"


def test_platform_of_unknown_adapter_falls_back_to_generic():
    assert platform_of("SomeNewAdapter", None) == "generic"


def test_platform_of_override_wins():
    assert platform_of("OneBot V11", "qq_bot") == "qq_bot"
