"""Synchronize wallet Swap provider metadata and logos from DefiLlama."""

from __future__ import annotations

import argparse
import io
import re
import shutil
import sys
import urllib.parse
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import Any

from sync_support import (
    DEFAULT_ASSET_BASE_URI,
    http_get,
    load_json,
    require_file,
    slugify,
    write_bytes_atomic,
    write_json,
)

REPO_ROOT = Path(__file__).resolve().parents[1]
SUPPORT_DIR = REPO_ROOT / "support"
CATALOG_NAME = "swap-providers.json"
ALIASES_NAME = "swap-provider-aliases.json"
PROVIDER_DIR_NAME = "swap-providers"
PROVIDER_ID_RE = re.compile(r"^[a-z0-9]+(?:[-_][a-z0-9]+)*$")
DEFILLAMA_ICON_HOSTS = {"icons.llama.fi", "icons.llamao.fi"}
DEFILLAMA_MAX_WORKERS = 24
SWAP_CATEGORIES = {
    "Dexs",
    "DEX Aggregator",
    "Bridge",
    "Canonical Bridge",
    "Cross Chain Bridge",
    "Bridge Aggregator",
    "Bridge Aggregators",
}


def validate_provider_id(value: str, context: str) -> str:
    if not PROVIDER_ID_RE.fullmatch(value):
        raise ValueError(f"{context} must be a lowercase slug: {value!r}")
    return value


def validate_logo_url(value: Any, context: str) -> str:
    if not isinstance(value, str) or not value:
        raise ValueError(f"{context} must be a non-empty string")
    parsed = urllib.parse.urlsplit(value)
    if (
        parsed.scheme != "https"
        or parsed.hostname not in DEFILLAMA_ICON_HOSTS
        or not parsed.path.startswith("/icons/protocols/")
    ):
        raise ValueError(f"{context} is not a supported DefiLlama icon URL: {value}")
    return value


def parse_defillama_protocols(path: Path) -> list[dict[str, Any]]:
    data = load_json(path)
    if not isinstance(data, list):
        raise ValueError("DefiLlama protocols response must be an array")
    providers: list[dict[str, str]] = []
    seen_ids: dict[str, str] = {}
    for index, row in enumerate(data):
        if not isinstance(row, dict) or row.get("category") not in SWAP_CATEGORIES:
            continue
        source_slug, name = row.get("slug"), row.get("name")
        if not isinstance(source_slug, str) or not source_slug:
            raise ValueError(f"DefiLlama protocols[{index}].slug is invalid")
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"DefiLlama protocols[{index}].name is invalid")
        provider_id = slugify(source_slug)
        validate_provider_id(provider_id, f"DefiLlama protocols[{index}] id")
        previous_slug = seen_ids.get(provider_id)
        if previous_slug is not None and previous_slug != source_slug:
            raise ValueError(
                f"DefiLlama slugs collide after normalization: "
                f"{previous_slug!r} and {source_slug!r}"
            )
        seen_ids[provider_id] = source_slug
        provider = {
            "id": provider_id,
            "name": name.strip(),
            "sourceLogoURI": validate_logo_url(
                row.get("logo"), f"DefiLlama protocols[{index}].logo"
            ),
            "url": row.get("url"),
        }
        providers.append(provider)
    if not providers:
        raise ValueError("DefiLlama returned no Swap providers")
    return sorted(providers, key=lambda provider: provider["id"])


def load_aliases(path: Path) -> list[dict[str, str]]:
    data = load_json(path)
    if (
        not isinstance(data, dict)
        or data.get("schemaVersion") != 1
        or not isinstance(data.get("aliases"), list)
    ):
        raise ValueError(f"invalid Swap provider aliases: {path}")
    aliases: list[dict[str, str]] = []
    seen: set[str] = set()
    for index, row in enumerate(data["aliases"]):
        if not isinstance(row, dict):
            raise ValueError(f"aliases[{index}] must be an object")
        alias_id, target_id, name = row.get("id"), row.get("targetId"), row.get("name")
        if not isinstance(alias_id, str):
            raise ValueError(f"aliases[{index}].id must be a string")
        validate_provider_id(alias_id, f"aliases[{index}].id")
        if alias_id in seen:
            raise ValueError(f"duplicate Swap provider alias: {alias_id}")
        seen.add(alias_id)
        if not isinstance(target_id, str):
            raise ValueError(f"aliases[{index}].targetId must be a string")
        validate_provider_id(target_id, f"aliases[{index}].targetId")
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"aliases[{index}].name must be a non-empty string")
        aliases.append({"id": alias_id, "targetId": target_id, "name": name.strip()})
    return aliases


def validate_defillama_image(data: bytes) -> bytes:
    try:
        from PIL import Image
    except ImportError as exc:
        raise RuntimeError("Pillow is required to validate DefiLlama images") from exc
    try:
        with Image.open(io.BytesIO(data)) as image:
            image.load()
            if image.format != "WEBP":
                raise ValueError(f"DefiLlama image must be WebP, got {image.format}")
            if image.width <= 0 or image.height <= 0:
                raise ValueError("DefiLlama image has invalid dimensions")
            if image.width > 8192 or image.height > 8192:
                raise ValueError("DefiLlama image dimensions exceed 8192 pixels")
    except Exception as exc:
        if isinstance(exc, ValueError):
            raise
        raise ValueError(f"invalid DefiLlama WebP image: {exc}") from exc
    return data


def download_provider_logo(provider: dict[str, Any]) -> tuple[str, bytes]:
    urls = [provider["sourceLogoURI"]]
    fallback_url = f"https://icons.llamao.fi/icons/protocols/{provider['id']}"
    if fallback_url not in urls:
        urls.append(fallback_url)
    last_error: Exception | None = None
    for index, url in enumerate(urls):
        try:
            data, content_type = http_get(url)
            break
        except Exception as exc:
            last_error = exc
            if index + 1 == len(urls):
                raise
    else:
        assert last_error is not None
        raise last_error
    if not content_type.startswith("image/"):
        raise ValueError(
            f"DefiLlama logo for {provider['id']} is not an image: {content_type}"
        )
    return provider["id"], validate_defillama_image(data)


def logo_needs_download(path: Path) -> bool:
    if not path.is_file():
        return True
    try:
        validate_defillama_image(path.read_bytes())
    except ValueError:
        return True
    return False


def sync_swap_providers(
    protocols_json: Path,
    support_dir: Path = SUPPORT_DIR,
    asset_base_uri: str = DEFAULT_ASSET_BASE_URI,
) -> None:
    source_providers = parse_defillama_protocols(protocols_json)
    aliases = load_aliases(support_dir / ALIASES_NAME)
    provider_dir = support_dir / PROVIDER_DIR_NAME
    provider_dir.mkdir(parents=True, exist_ok=True)
    missing = [
        provider
        for provider in source_providers
        if logo_needs_download(provider_dir / provider["id"] / "logo.webp")
    ]
    errors: dict[str, str] = {}
    with ThreadPoolExecutor(max_workers=DEFILLAMA_MAX_WORKERS) as executor:
        futures = {
            executor.submit(download_provider_logo, provider): provider
            for provider in missing
        }
        for future in as_completed(futures):
            provider = futures[future]
            try:
                provider_id, image = future.result()
            except Exception as exc:
                errors[provider["id"]] = str(exc)
                continue
            write_bytes_atomic(provider_dir / provider_id / "logo.webp", image)
    if errors:
        preview = "; ".join(
            f"{provider_id}: {message}"
            for provider_id, message in sorted(errors.items())[:10]
        )
        print(
            f"warning: skipped {len(errors)} unavailable DefiLlama logos: {preview}",
            file=sys.stderr,
        )
        source_providers = [
            provider for provider in source_providers if provider["id"] not in errors
        ]

    base_uri = asset_base_uri.rstrip("/")
    providers: dict[str, dict[str, Any]] = {}
    for source in source_providers:
        logo_path = provider_dir / source["id"] / "logo.webp"
        require_file(logo_path)
        validate_defillama_image(logo_path.read_bytes())
        entry = {
            "id": source["id"],
            "name": source["name"],
            "logoURI": f"{base_uri}/{PROVIDER_DIR_NAME}/{source['id']}/logo.webp",
            "url": source["url"],
        }
        providers[source["id"]] = entry

    for alias in aliases:
        if alias["id"] in providers:
            raise ValueError(f"Swap provider alias conflicts with provider: {alias['id']}")
        target = providers.get(alias["targetId"])
        if target is None:
            raise ValueError(
                f"Swap provider alias target does not exist: {alias['targetId']}"
            )
        target_logo = provider_dir / alias["targetId"] / "logo.webp"
        alias_logo = provider_dir / alias["id"] / "logo.webp"
        write_bytes_atomic(alias_logo, target_logo.read_bytes())
        entry = {
            "id": alias["id"],
            "name": alias["name"],
            "logoURI": f"{base_uri}/{PROVIDER_DIR_NAME}/{alias['id']}/logo.webp",
            "url": target["url"],
        }
        providers[alias["id"]] = entry

    expected_ids = set(providers)
    for path in provider_dir.iterdir():
        if path.is_dir() and path.name not in expected_ids:
            shutil.rmtree(path)
    for provider_id in expected_ids:
        directory = provider_dir / provider_id
        for path in directory.iterdir():
            if path.name != "logo.webp":
                if path.is_dir():
                    shutil.rmtree(path)
                else:
                    path.unlink()

    output = {
        "schemaVersion": 1,
        "assetBaseURI": base_uri,
        "providers": [providers[key] for key in sorted(providers)],
    }
    write_json(support_dir / CATALOG_NAME, output)
    validate_swap_provider_output(support_dir=support_dir, asset_base_uri=base_uri)
    print(
        f"Synced {len(source_providers)} DefiLlama Swap providers "
        f"and {len(aliases)} aliases"
    )


def validate_swap_provider_output(
    support_dir: Path = SUPPORT_DIR,
    asset_base_uri: str | None = None,
) -> None:
    catalog_path = support_dir / CATALOG_NAME
    require_file(catalog_path)
    data = load_json(catalog_path)
    if not isinstance(data, dict) or data.get("schemaVersion") != 1:
        raise ValueError(f"{CATALOG_NAME} must be a schemaVersion 1 object")
    base_uri = asset_base_uri or data.get("assetBaseURI")
    if not isinstance(base_uri, str) or not base_uri:
        raise ValueError(f"{CATALOG_NAME} assetBaseURI must be a non-empty string")
    if data.get("assetBaseURI") != base_uri.rstrip("/"):
        raise ValueError(f"{CATALOG_NAME} assetBaseURI does not match")
    providers = data.get("providers")
    if not isinstance(providers, list):
        raise ValueError(f"{CATALOG_NAME} providers must be an array")
    previous_id = ""
    for index, provider in enumerate(providers):
        if not isinstance(provider, dict):
            raise ValueError(f"providers[{index}] must be an object")
        provider_id, name, logo_uri = (
            provider.get("id"), provider.get("name"), provider.get("logoURI")
        )
        if not isinstance(provider_id, str):
            raise ValueError(f"providers[{index}].id is invalid")
        validate_provider_id(provider_id, f"providers[{index}].id")
        if provider_id <= previous_id:
            raise ValueError("Swap providers must be sorted by id")
        previous_id = provider_id
        if not isinstance(name, str) or not name:
            raise ValueError(f"providers[{index}].name is invalid")
        if set(provider) != {"id", "name", "logoURI", "url"}:
            raise ValueError(f"providers[{index}] must contain id, name, logoURI, and url")
        expected_uri = (
            f"{base_uri.rstrip('/')}/{PROVIDER_DIR_NAME}/{provider_id}/logo.webp"
        )
        if logo_uri != expected_uri:
            raise ValueError(f"providers[{index}].logoURI is invalid")
        logo_path = support_dir / PROVIDER_DIR_NAME / provider_id / "logo.webp"
        require_file(logo_path)
        validate_defillama_image(logo_path.read_bytes())


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--protocols-json", type=Path, help="DefiLlama /protocols response")
    parser.add_argument("--asset-base-uri", default=DEFAULT_ASSET_BASE_URI)
    parser.add_argument("--validate-output", action="store_true")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        if args.validate_output:
            validate_swap_provider_output()
            print(f"Validated {CATALOG_NAME}")
            return 0
        if args.protocols_json is None:
            raise ValueError("--protocols-json is required")
        sync_swap_providers(
            args.protocols_json.resolve(), SUPPORT_DIR, args.asset_base_uri
        )
        validate_swap_provider_output()
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
