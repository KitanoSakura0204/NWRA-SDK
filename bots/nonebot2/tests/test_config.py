import pytest
from pydantic import ValidationError

from nonebot_plugin_nowork.config import Config


def test_config_requires_base_url_and_token():
    with pytest.raises(ValidationError):
        Config()


def test_config_defaults():
    cfg = Config(nowork_base_url="https://nowork.example.com", nowork_bot_token="tok")
    assert cfg.nowork_command_prefix == "/"
    assert cfg.nowork_allowed_chats == []
    assert cfg.nowork_platform is None
