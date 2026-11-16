#!/usr/bin/env python3
"""Generate a portable SDK API snapshot and index from the backend's checked contract."""
from __future__ import annotations

import argparse
import hashlib
import json
from collections import defaultdict
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT_SOURCE = ROOT.parent / "Backend/internal/modules/devdocs/openapi_full.json"
SNAPSHOT = ROOT / "docs/reference/openapi.json"
MANIFEST = ROOT / "docs/reference/manifest.json"
INDEX = ROOT / "docs/API_REFERENCE.md"
METHODS = ("get", "post", "put", "patch", "delete", "head", "options", "trace")
AS_OF = "2026-11-16"


def operations(document):
    return [(path, method, item[method]) for path, item in sorted(document["paths"].items())
            for method in METHODS if method in item]


def resolve_pointer(document, reference):
    if not reference.startswith("#/"):
        raise ValueError(f"external reference is not portable: {reference}")
    node = document
    for part in reference[2:].split("/"):
        part = part.replace("~1", "/").replace("~0", "~")
        node = node[int(part)] if isinstance(node, list) else node[part]
    return node


def validate(document):
    if not str(document.get("openapi", "")).startswith("3.") or not isinstance(document.get("paths"), dict):
        raise ValueError("expected an OpenAPI 3 document with paths")
    pending = [document]
    while pending:
        node = pending.pop()
        if isinstance(node, dict):
            # A schema's properties map can itself declare a field named $ref.
            if isinstance(node.get("$ref"), str):
                resolve_pointer(document, node["$ref"])
            pending.extend(node.values())
        elif isinstance(node, list):
            pending.extend(node)
    schemes = document.get("components", {}).get("securitySchemes", {})
    for path, method, operation in operations(document):
        for alternative in operation.get("security", document.get("security", [])):
            for name in alternative:
                if name not in schemes:
                    raise ValueError(f"undefined auth scheme {name}: {method} {path}")


def cell(text):
    return str(text).replace("|", "&#124;").replace("\r", " ").replace("\n", " ")


def auth_text(document, operation):
    alternatives = operation.get("security", document.get("security", []))
    if not alternatives:
        return "公开；仍受站点、限流及业务条件约束"
    return " 或 ".join(" + ".join(f"`{cell(name)}`" for name in alternative)
                       if alternative else "可匿名" for alternative in alternatives)


def request_text(document, path_item, operation):
    parts = []
    for raw in path_item.get("parameters", []) + operation.get("parameters", []):
        parameter = resolve_pointer(document, raw["$ref"]) if "$ref" in raw else raw
        parts.append(f"{parameter.get('in', '?')}:`{cell(parameter.get('name', '?'))}`"
                     + ("*" if parameter.get("required") else ""))
    if "requestBody" in operation:
        body = operation["requestBody"]
        if "$ref" in body:
            body = resolve_pointer(document, body["$ref"])
        parts.extend("body:" + cell(media) for media in body.get("content", {}))
    return "; ".join(parts) or "—"


def render_index(document, digest):
    ops = operations(document)
    registered = sum("x-go-source" in operation for _, _, operation in ops)
    groups = defaultdict(list)
    for path, method, operation in ops:
        groups[(operation.get("tags") or ["未分组"])[0]].append((path, method, operation))
    schemas = document.get("components", {}).get("schemas", {})
    lines = ["# 全项目接口索引", "", f"契约核对日期：{AS_OF}。共 **{len(ops)} 个文档操作、{len(document['paths'])} 个路径、{len(schemas)} 个数据结构**；其中 {registered} 个操作有当前源码注册位置，{len(ops)-registered} 个仅作为已弃用契约保留。", "",
             "本文由 `scripts/generate_integration_reference.py` 生成。每个接口的全部参数、请求体、响应、错误、权限说明和数据结构见同目录内可离线使用的 [完整 OpenAPI](reference/openapi.json)。不要将该完整管理接口文档放入网页公开静态目录；SDK 包默认不发布这个参考目录。", "",
             "先读 [游戏服务器指南](GAME_SERVER_INTEGRATION.md)、[OIDC 指南](OIDC_INTEGRATION.md) 或 [项目对接总指南](PROJECT_INTEGRATION.md)。表中的鉴权列只是协议凭据入口，角色权限、数据范围、站点与用户状态仍以操作的 `description` 和后端检查为准；接口存在不表示当前账号或站点可使用。`*` 表示必填参数。", "",
             f"源契约 SHA-256：`{digest}`。版本和来源见 [manifest](reference/manifest.json)。", "", "## 鉴权方案", "",
             "| 方案 | 类型 | 说明 |", "| --- | --- | --- |"]
    for name, scheme in sorted(document.get("components", {}).get("securitySchemes", {}).items()):
        lines.append(f"| `{cell(name)}` | {cell(scheme.get('type', ''))} | {cell(scheme.get('description', ''))} |")
    lines += ["", "## 分组目录", ""]
    ordered = sorted(groups)
    for i, tag in enumerate(ordered, 1):
        lines.append(f"- [{tag}](#group-{i:03d})（{len(groups[tag])} 个操作）")
    descriptions = {item["name"]: item.get("description", "") for item in document.get("tags", [])}
    for i, tag in enumerate(ordered, 1):
        lines += ["", f'<a id="group-{i:03d}"></a>', "", f"### {tag}", "", descriptions.get(tag, ""), "",
                  "| 方法 | 路径 | 摘要 | 鉴权 | 输入入口 |", "| --- | --- | --- | --- | --- |"]
        for path, method, operation in groups[tag]:
            summary = cell(operation.get('summary', '')) + ("（已弃用）" if operation.get("deprecated") else "")
            lines.append(f"| {method.upper()} | `{cell(path)}` | {summary} | {auth_text(document, operation)} | {request_text(document, document['paths'][path], operation)} |")
    lines += ["", "## 更新和校验", "", "在 SDK 根目录执行以下命令；第一条需要同级 Backend checkout：", "", "```powershell",
              "python ../Backend/scripts/gen_openapi.py --check", "python scripts/generate_integration_reference.py", "python scripts/generate_integration_reference.py --check", "python scripts/check_docs.py", "```", "",
              "仅有 SDK checkout 时，`--check` 会检查已有快照、索引与内部引用的一致性，并明确说明没有与实时后端源契约比较；生成更新仍需要通过 `--source` 指定经过核对的后端 JSON。", ""]
    return "\n".join(lines)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, default=DEFAULT_SOURCE)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    source = args.source
    standalone = args.check and source == DEFAULT_SOURCE and not source.exists()
    if standalone:
        source = SNAPSHOT
    raw = source.read_bytes()
    document = json.loads(raw.decode("utf-8-sig"))
    validate(document)
    digest = hashlib.sha256(raw).hexdigest()
    manifest = {"asOf": AS_OF, "sdkVersion": "1.3.0", "source": "Backend/internal/modules/devdocs/openapi_full.json",
                "sha256": digest, "operations": len(operations(document)), "paths": len(document["paths"]),
                "sourceRegisteredOperations": sum("x-go-source" in operation for _, _, operation in operations(document)),
                "schemas": len(document.get("components", {}).get("schemas", {}))}
    outputs = {SNAPSHOT: raw, MANIFEST: (json.dumps(manifest, ensure_ascii=False, indent=2) + "\n").encode("utf-8"),
               INDEX: render_index(document, digest).encode("utf-8")}
    if args.check:
        changed = [str(path.relative_to(ROOT)) for path, content in outputs.items()
                   if not path.exists() or path.read_bytes() != content]
        if changed:
            raise SystemExit("stale generated reference: " + ", ".join(changed))
        print("PASS API snapshot, index and JSON references" + (" (standalone; live backend not compared)" if standalone else " (matches backend source)"))
    else:
        for path, content in outputs.items():
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        print(f"Generated {manifest['operations']} operations and {manifest['schemas']} schemas")


if __name__ == "__main__":
    main()
