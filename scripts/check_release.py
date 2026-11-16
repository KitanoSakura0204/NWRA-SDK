#!/usr/bin/env python3
"""Keep the six core SDK release declarations aligned without bumping other packages."""
import json
from pathlib import Path
import re
import sys

ROOT = Path(__file__).resolve().parents[1]
DECLARATIONS = {
    "go/client.go": r'const Version = "([^"]+)"',
    "java/pom.xml": r"<version>([^<]+)</version>",
    "java/src/main/java/cyou/nowork/sdk/NoworkClient.java": r'VERSION = "([^"]+)"',
    "python/pyproject.toml": r'^version = "([^"]+)"',
    "python/nowork_sdk/__init__.py": r'__version__ = "([^"]+)"',
    "typescript/package.json": r'"version":\s*"([^"]+)"',
    "typescript/src/index.ts": r"SDK_VERSION = '([^']+)'",
    "csharp/Nowork.Sdk/Nowork.Sdk.csproj": r"<Version>([^<]+)</Version>",
    "csharp/Nowork.Sdk/NoworkClient.cs": r'const string Version = "([^"]+)"',
    "php/composer.json": r'"version":\s*"([^"]+)"',
    "php/src/NoworkClient.php": r"const VERSION = '([^']+)'",
}


def main():
    match = re.search(r"^## (\d+\.\d+\.\d+)\b", (ROOT / "CHANGELOG.md").read_text(encoding="utf-8"), re.M)
    if not match:
        raise SystemExit("missing latest semantic version in CHANGELOG")
    version = match.group(1)
    errors = []
    for name, pattern in DECLARATIONS.items():
        found = re.search(pattern, (ROOT / name).read_text(encoding="utf-8"), re.M)
        if not found or found.group(1) != version:
            errors.append(f"{name}: expected {version}, got {found.group(1) if found else 'missing'}")
    manifest = json.loads((ROOT / "docs/reference/manifest.json").read_text(encoding="utf-8"))
    if manifest.get("sdkVersion") != version:
        errors.append("reference manifest SDK version is stale")
    if f"SDK {version}" not in (ROOT / "README.md").read_text(encoding="utf-8").splitlines()[0]:
        errors.append("README release title is stale")
    if errors:
        print("\n".join(errors))
        return 1
    print(f"PASS core SDK {version}: {len(DECLARATIONS)} declarations, manifest and README")
    return 0


if __name__ == "__main__":
    sys.exit(main())
