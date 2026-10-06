"""Typed GM action schema (PROTOCOL 2.4). Plain dicts work everywhere these types are accepted."""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional


def _drop_empty(d: Dict[str, Any]) -> Dict[str, Any]:
    return {k: v for k, v in d.items() if v is not None and v != "" and v != [] and v != {} and v is not False}


@dataclass
class GMOption:
    """One choice of a select parameter or of a named option list."""

    value: str
    label: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return {"value": self.value, "label": self.label} if self.label else {"value": self.value}


@dataclass
class GMActionParam:
    """One input of a GM action. Only name / type / required / label existed before 2026-09-28;
    every other field is optional. type: string | text | number | boolean | select | player
    (aliases int / integer / bool / textarea)."""

    name: str
    type: str = "string"
    required: bool = False
    label: str = ""
    description: str = ""
    placeholder: str = ""
    default: Any = None  # ignored by the server when it does not fit the definition
    min: Optional[float] = None  # number only
    max: Optional[float] = None
    step: Optional[float] = None
    integer: bool = False
    max_length: int = 0  # server default 1000, text 4000, player 64
    options: List[GMOption] = field(default_factory=list)  # inline select choices (1-200)
    source: str = ""  # names a list published with publish_gm_catalog / publish_gm_options

    def to_dict(self) -> Dict[str, Any]:
        extra = _drop_empty({
            "description": self.description,
            "placeholder": self.placeholder,
            "integer": self.integer,
            "maxLength": self.max_length or None,
            "options": [_plain(o) for o in self.options],
            "source": self.source,
        })
        for k in ("default", "min", "max", "step"):  # 0 and False are real values here
            if getattr(self, k) is not None:
                extra[k] = getattr(self, k)
        return {"name": self.name, "type": self.type, "required": self.required, "label": self.label, **extra}


@dataclass
class GMAction:
    """A GM action the plugin offers to the admin panel. key: no colon, no whitespace, <= 64 bytes.
    danger: normal | confirm | critical (the last two need a confirmation before sending)."""

    key: str
    title: str
    description: str = ""
    params: List[GMActionParam] = field(default_factory=list)
    labels: Dict[str, str] = field(default_factory=dict)
    category: str = ""  # catalogue group
    danger: str = ""

    def to_dict(self) -> Dict[str, Any]:
        return {"key": self.key, "title": self.title, **_drop_empty({
            "description": self.description,
            "params": [_plain(p) for p in self.params],
            "labels": dict(self.labels),
            "category": self.category,
            "danger": self.danger,
        })}


@dataclass
class GMPublishResult:
    """What a publish stored; entries the server could not accept are skipped and listed
    ([{"key": ..., "reason": ...}]), never a failure for the whole call."""

    count: int = 0
    skipped: List[Dict[str, str]] = field(default_factory=list)

    @classmethod
    def from_dict(cls, d: Dict[str, Any]) -> "GMPublishResult":
        return cls(int(d.get("count", 0) or 0), [dict(s) for s in (d.get("skipped") or []) if isinstance(s, dict)])


def _plain(x: Any) -> Any:
    """A type from this module as its wire dict; dicts and other values pass through."""
    return x.to_dict() if hasattr(x, "to_dict") else x
