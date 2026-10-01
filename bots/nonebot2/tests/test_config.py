import pytest
from pydantic import ValidationError

from nonebot_plugin_lounge.config import Config


def test_config_requires_base_url_and_token():
    with pytest.raises(ValidationError):
        Config()


def test_config_defaults():
    cfg = Config(lounge_base_url="https://lounge.example.com", lounge_bot_token="tok")
    assert cfg.lounge_command_prefix == "/"
    assert cfg.lounge_allowed_chats == []
    assert cfg.lounge_platform is None
