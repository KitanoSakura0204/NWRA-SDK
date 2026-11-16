#!/usr/bin/env python3
"""Check local links in the maintained SDK integration guides and language READMEs."""
from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]


def main():
    files = [ROOT / "README.md", ROOT / "CHANGELOG.md"]
    files += sorted((ROOT / "docs").rglob("*.md"))
    files += [ROOT / language / "README.md" for language in ("go", "python", "typescript", "java", "csharp", "php")]
    files += [ROOT / "examples/README.md"]
    files += sorted((ROOT / "examples/oidc-rp-go").glob("README.md"))
    errors = []
    count = 0
    for path in files:
        if not path.exists():
            errors.append(f"missing document: {path.relative_to(ROOT)}")
            continue
        text = path.read_text(encoding="utf-8-sig")
        text = re.sub(r"^(`{3,}|~{3,}).*?^\1\s*$", "", text, flags=re.M | re.S)
        for match in re.finditer(r"(?<!!)\[[^\]\n]*\]\((<?[^)\n]+>?)\)", text):
            target = match.group(1).strip().strip("<>")
            if urlsplit(target).scheme or target.startswith(("#", "//")):
                continue
            target = unquote(target.split("#", 1)[0].split("?", 1)[0])
            if not target:
                continue
            count += 1
            if not (path.parent / target).exists():
                errors.append(f"{path.relative_to(ROOT)}: missing link {target}")
        if "\ufffd" in text:
            errors.append(f"{path.relative_to(ROOT)}: Unicode replacement character")
    if errors:
        print("\n".join(errors))
        return 1
    print(f"PASS {len(files)} documents and {count} local links")
    return 0


if __name__ == "__main__":
    sys.exit(main())
