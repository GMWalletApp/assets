"""Synchronize support wallets and exchanges from WalletConnect and Web3icons."""

from __future__ import annotations

import argparse
import filecmp
import io
import json
import os
import re
import shutil
import sys
import tempfile
import time
import unicodedata
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parents[1]
SUPPORT_DIR = REPO_ROOT / "support"
SUPPORT_JSON_NAME = "support.json"
DAPPS_JSON_NAME = "dapps.json"
SOURCE_MAP_NAME = "wallet-source-map.json"
OVERRIDES_NAME = "support-overrides.json"
DEFAULT_ASSET_BASE_URI = (
    "https://raw.githubusercontent.com/GMWalletApp/assets/main/support"
)
WALLETCONNECT_API = "https://explorer-api.walletconnect.com/v3"
MAX_DOWNLOAD_BYTES = 10 * 1024 * 1024
MAX_RASTER_DIMENSION = 4096
MAX_WORKERS = 8
RETRIES = 3
SLUG_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
SOURCE_CATEGORIES = {
    "wallet": "wallets",
    "exchange": "exchanges",
    "network": "networks",
    "token": "tokens",
}
FORBIDDEN_SVG_TAGS = {"script", "style", "foreignobject", "iframe", "object", "embed"}


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(
        json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )


def require_dir(path: Path) -> None:
    if not path.is_dir():
        raise FileNotFoundError(f"required directory not found: {path}")


def require_file(path: Path) -> None:
    if not path.is_file():
        raise FileNotFoundError(f"required file not found: {path}")


def normalized_ascii(value: str) -> str:
    return (
        unicodedata.normalize("NFKD", value).encode("ascii", "ignore").decode("ascii")
    ).lower()


def slugify(value: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", normalized_ascii(value)).strip("-") or "wallet"


def normalized_name(value: str) -> str:
    return re.sub(r"[^a-z0-9]", "", normalized_ascii(value))


def validate_slug(value: str, context: str) -> str:
    if not SLUG_RE.fullmatch(value):
        raise ValueError(f"{context} must be a lowercase kebab-case slug: {value!r}")
    return value


def load_web3icons_rows(source: Path, category: str) -> list[dict[str, Any]]:
    path = source / "packages" / "common" / "src" / "metadata" / f"{category}.json"
    require_file(path)
    rows = load_json(path)
    if not isinstance(rows, list):
        raise ValueError(f"{path} must contain a JSON array")
    seen: set[str] = set()
    result: list[dict[str, Any]] = []
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise ValueError(f"{path} row {index} must be an object")
        icon_id, name, file_path = row.get("id"), row.get("name"), row.get("filePath")
        if not isinstance(icon_id, str) or not icon_id:
            raise ValueError(f"{path} row {index} has invalid id")
        validate_slug(icon_id.lower(), f"{path} row {index} id")
        if icon_id in seen:
            raise ValueError(f"{path} contains duplicate id: {icon_id}")
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"{path} row {index} has invalid name")
        if not isinstance(file_path, str) or ":" not in file_path:
            raise ValueError(f"{path} row {index} has invalid filePath")
        if category == "exchanges" and row.get("type") not in {"cex", "dex"}:
            raise ValueError(f"{path} row {index} has invalid exchange type")
        seen.add(icon_id)
        result.append(row)
    return result


def find_case_insensitive_file(directory: Path, filename: str) -> Path | None:
    direct = directory / filename
    if direct.is_file():
        return direct
    if not directory.is_dir():
        return None
    matches = [
        path
        for path in directory.iterdir()
        if path.is_file() and path.name.lower() == filename.lower()
    ]
    if len(matches) > 1:
        raise ValueError(
            f"ambiguous case-insensitive file match for {directory / filename}"
        )
    return matches[0] if matches else None


def resolve_web3icons_svg(source: Path, row: dict[str, Any]) -> Path | None:
    source_type, source_id = row["filePath"].split(":", 1)
    source_category = SOURCE_CATEGORIES.get(source_type)
    if source_category is None or not source_id:
        raise ValueError(f"unsupported Web3icons filePath: {row['filePath']}")
    available_variants = row.get("variants")
    if not isinstance(available_variants, list):
        available_variants = ["branded", "background", "mono"]
    for variant in ("branded", "background", "mono"):
        if variant not in available_variants:
            continue
        match = find_case_insensitive_file(
            source / "raw-svgs" / source_category / variant,
            f"{source_id}.svg",
        )
        if match is not None:
            return match
    return None


def validate_safe_svg(data: bytes) -> bytes:
    if not data or len(data) > MAX_DOWNLOAD_BYTES:
        raise ValueError("SVG is empty or exceeds the size limit")
    lowered = data.lower()
    if b"<!doctype" in lowered or b"<!entity" in lowered:
        raise ValueError("SVG contains a forbidden document type or entity")
    for match in re.finditer(rb"<\?([A-Za-z_:][\w:.-]*)", data):
        if match.group(1).lower() != b"xml":
            raise ValueError("SVG contains a forbidden processing instruction")
    try:
        root = ET.fromstring(data)
    except ET.ParseError as exc:
        raise ValueError(f"invalid SVG XML: {exc}") from exc
    if root.tag.rsplit("}", 1)[-1].lower() != "svg":
        raise ValueError("image XML root is not svg")
    for element in root.iter():
        local_tag = element.tag.rsplit("}", 1)[-1].lower()
        if local_tag in FORBIDDEN_SVG_TAGS:
            raise ValueError(f"SVG contains forbidden element: {local_tag}")
        for raw_name, raw_value in element.attrib.items():
            name, value = raw_name.rsplit("}", 1)[-1].lower(), raw_value.strip().lower()
            if name.startswith("on"):
                raise ValueError(f"SVG contains event handler attribute: {name}")
            forbidden_values = (
                "http:",
                "https:",
                "javascript:",
                "data:text",
                "data:image/svg",
                "//",
            )
            if any(marker in value for marker in forbidden_values):
                raise ValueError("SVG contains an external or executable reference")
            allowed_embedded = (
                "#",
                "data:image/png;base64,",
                "data:image/jpeg;base64,",
                "data:image/webp;base64,",
            )
            if (
                name in {"href", "src"}
                and value
                and not value.startswith(allowed_embedded)
            ):
                raise ValueError("SVG contains a non-local resource reference")
            if "url(" in value and "url(#" not in value.replace(" ", ""):
                raise ValueError("SVG contains a non-local URL reference")
    return data


def validate_raster_image(data: bytes) -> Any:
    if not data or len(data) > MAX_DOWNLOAD_BYTES:
        raise ValueError("raster image is empty or exceeds the size limit")
    try:
        from PIL import Image
    except ImportError as exc:
        raise RuntimeError(
            "Pillow is required to normalize WalletConnect raster images"
        ) from exc
    Image.MAX_IMAGE_PIXELS = MAX_RASTER_DIMENSION * MAX_RASTER_DIMENSION
    try:
        with Image.open(io.BytesIO(data)) as image:
            image.load()
            if image.width <= 0 or image.height <= 0:
                raise ValueError("wallet image has invalid dimensions")
            if (
                image.width > MAX_RASTER_DIMENSION
                or image.height > MAX_RASTER_DIMENSION
            ):
                raise ValueError("wallet image dimensions exceed the limit")
            return image.convert("RGBA")
    except Exception as exc:
        if isinstance(exc, ValueError):
            raise
        raise ValueError(f"invalid wallet raster image: {exc}") from exc


def normalize_walletconnect_image(data: bytes, content_type: str) -> tuple[str, bytes]:
    stripped = data.lstrip()
    if "svg" in content_type.lower() or stripped.startswith((b"<svg", b"<?xml")):
        return "svg", validate_safe_svg(data)
    image = validate_raster_image(data)
    output = io.BytesIO()
    image.save(output, format="PNG", optimize=True)
    return "png", output.getvalue()


def http_get(url: str, timeout: int = 30) -> tuple[bytes, str]:
    request = urllib.request.Request(
        url, headers={"User-Agent": "GMWalletApp-assets-sync/1.0"}
    )
    last_error: Exception | None = None
    for attempt in range(RETRIES):
        try:
            with urllib.request.urlopen(request, timeout=timeout) as response:
                length = response.headers.get("Content-Length")
                if length and int(length) > MAX_DOWNLOAD_BYTES:
                    raise ValueError("response exceeds the size limit")
                data = response.read(MAX_DOWNLOAD_BYTES + 1)
                if len(data) > MAX_DOWNLOAD_BYTES:
                    raise ValueError("response exceeds the size limit")
                return data, response.headers.get_content_type()
        except urllib.error.HTTPError as exc:
            last_error = exc
            if exc.code != 429 and not 500 <= exc.code < 600:
                raise
            retry_after = exc.headers.get("Retry-After") if exc.headers else None
            delay = (
                int(retry_after)
                if retry_after and retry_after.isdigit()
                else 2**attempt
            )
        except (urllib.error.URLError, TimeoutError) as exc:
            last_error = exc
            delay = 2**attempt
        if attempt + 1 < RETRIES:
            time.sleep(delay)
    assert last_error is not None
    raise last_error


def parse_walletconnect_listings(payload: Any) -> list[dict[str, str]]:
    if not isinstance(payload, dict):
        raise ValueError("WalletConnect listings response must be an object")
    raw_listings = payload.get("listings")
    if isinstance(raw_listings, dict):
        pairs = list(raw_listings.items())
    elif isinstance(raw_listings, list):
        pairs = [(None, value) for value in raw_listings]
    else:
        raise ValueError("WalletConnect response is missing listings")
    listings: list[dict[str, str]] = []
    seen: set[str] = set()
    for key, value in pairs:
        if not isinstance(value, dict):
            raise ValueError("WalletConnect listing must be an object")
        listing_id, name, image_id = (
            value.get("id") or key,
            value.get("name"),
            value.get("image_id"),
        )
        if not isinstance(listing_id, str) or not listing_id:
            raise ValueError("WalletConnect listing has invalid id")
        if listing_id in seen:
            raise ValueError(f"duplicate WalletConnect listing id: {listing_id}")
        if not isinstance(name, str) or not name.strip():
            raise ValueError(f"WalletConnect listing {listing_id} has invalid name")
        if not isinstance(image_id, str) or not image_id:
            raise ValueError(f"WalletConnect listing {listing_id} has invalid image_id")
        seen.add(listing_id)
        listings.append({"id": listing_id, "name": name.strip(), "image_id": image_id})
    total = payload.get("total")
    if isinstance(total, int) and total != len(listings):
        message = (
            f"incomplete WalletConnect response: expected {total}, "
            f"received {len(listings)}"
        )
        raise ValueError(message)
    if not listings:
        raise ValueError("WalletConnect returned no wallets")
    return sorted(listings, key=lambda item: item["id"])


def fetch_walletconnect_wallets(project_id: str) -> list[dict[str, Any]]:
    query = urllib.parse.urlencode({"projectId": project_id})
    payload_bytes, _ = http_get(f"{WALLETCONNECT_API}/wallets?{query}")
    try:
        listings = parse_walletconnect_listings(json.loads(payload_bytes))
    except json.JSONDecodeError as exc:
        raise ValueError("WalletConnect listings response is not valid JSON") from exc

    def download(listing: dict[str, str]) -> dict[str, Any]:
        image_id = urllib.parse.quote(listing["image_id"], safe="")
        data, content_type = http_get(f"{WALLETCONNECT_API}/logo/lg/{image_id}?{query}")
        extension, normalized = normalize_walletconnect_image(data, content_type)
        return {**listing, "extension": extension, "image": normalized}

    results: list[dict[str, Any]] = []
    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
        futures = [executor.submit(download, listing) for listing in listings]
        for future in as_completed(futures):
            results.append(future.result())
    return sorted(results, key=lambda item: item["id"])


def entry_map(document: dict[str, Any], category: str) -> dict[str, dict[str, Any]]:
    entries = document.get(category)
    if not isinstance(entries, list):
        raise ValueError(f"support document {category} must be an array")
    result: dict[str, dict[str, Any]] = {}
    for entry in entries:
        if not isinstance(entry, dict) or not isinstance(entry.get("id"), str):
            raise ValueError(f"invalid support {category} entry")
        if entry["id"] in result:
            raise ValueError(f"duplicate support {category} id: {entry['id']}")
        result[entry["id"]] = dict(entry)
    return result


def load_source_map(
    path: Path, existing_wallets: dict[str, dict[str, Any]]
) -> dict[str, dict[str, str]]:
    if not path.is_file():
        return {slug: {} for slug in sorted(existing_wallets)}
    data = load_json(path)
    if (
        not isinstance(data, dict)
        or data.get("schemaVersion") != 1
        or not isinstance(data.get("wallets"), dict)
    ):
        raise ValueError(f"invalid wallet source map: {path}")
    result: dict[str, dict[str, str]] = {}
    wc_ids: set[str] = set()
    web3_ids: set[str] = set()
    for slug, sources in data["wallets"].items():
        validate_slug(slug, "wallet source-map key")
        if not isinstance(sources, dict):
            raise ValueError(f"wallet source map entry must be an object: {slug}")
        entry: dict[str, str] = {}
        for key, seen in (("web3iconsId", web3_ids), ("walletConnectId", wc_ids)):
            value = sources.get(key)
            if value is None:
                continue
            if not isinstance(value, str) or not value:
                raise ValueError(
                    f"wallet source map {slug}.{key} must be a non-empty string"
                )
            if value in seen:
                raise ValueError(f"duplicate wallet source id for {key}: {value}")
            seen.add(value)
            entry[key] = value
        result[slug] = entry
    for slug in existing_wallets:
        result.setdefault(slug, {})
    return result


def load_overrides(path: Path) -> dict[str, dict[str, dict[str, Any]]]:
    data = load_json(path)
    if not isinstance(data, dict):
        raise ValueError("support overrides must be an object")
    result: dict[str, dict[str, dict[str, Any]]] = {}
    for category in ("wallets", "exchanges"):
        rows = data.get(category)
        if not isinstance(rows, list):
            raise ValueError(f"support overrides {category} must be an array")
        by_id: dict[str, dict[str, Any]] = {}
        for row in rows:
            if not isinstance(row, dict) or not isinstance(row.get("id"), str):
                raise ValueError(f"invalid {category} override")
            validate_slug(row["id"], f"{category} override id")
            logo_path = row.get("logoPath")
            if (
                not isinstance(logo_path, str)
                or not logo_path
                or Path(logo_path).is_absolute()
                or ".." in Path(logo_path).parts
            ):
                raise ValueError(f"invalid {category} override logoPath: {row['id']}")
            by_id[row["id"]] = row
        result[category] = by_id
    return result


def unique_slug(base: str, listing_id: str, occupied: set[str]) -> str:
    if base not in occupied:
        return base
    suffix = slugify(listing_id)[:8] or "wallet"
    candidate = f"{base}-{suffix}"
    counter = 2
    while candidate in occupied:
        candidate = f"{base}-{suffix}-{counter}"
        counter += 1
    return candidate


def copy_atomic(source: Path, destination: Path) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=destination.parent, delete=False) as handle:
        temp_path = Path(handle.name)
    try:
        shutil.copyfile(source, temp_path)
        temp_path.chmod(0o644)
        os.replace(temp_path, destination)
    finally:
        temp_path.unlink(missing_ok=True)


def copy_atomic_if_changed(source: Path, destination: Path) -> None:
    if destination.is_file() and filecmp.cmp(source, destination, shallow=False):
        destination.chmod(0o644)
        return
    copy_atomic(source, destination)


def write_bytes_atomic(destination: Path, data: bytes) -> None:
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=destination.parent, delete=False) as handle:
        handle.write(data)
        temp_path = Path(handle.name)
    try:
        temp_path.chmod(0o644)
        os.replace(temp_path, destination)
    finally:
        temp_path.unlink(missing_ok=True)


def dapps_asset_base_uri(asset_base_uri: str) -> str:
    return asset_base_uri.rstrip("/").rsplit("/", 1)[0] + "/dapps"


def sync_support(
    web3icons_source: Path,
    walletconnect_wallets: list[dict[str, Any]],
    support_dir: Path = SUPPORT_DIR,
    asset_base_uri: str = DEFAULT_ASSET_BASE_URI,
    legacy_dapps_dir: Path | None = None,
) -> None:
    require_dir(web3icons_source)
    support_json = support_dir / SUPPORT_JSON_NAME
    require_file(support_json)
    document = load_json(support_json)
    if not isinstance(document, dict):
        raise ValueError("support.json must contain an object")
    wallets, exchanges = (
        entry_map(document, "wallets"),
        entry_map(document, "exchanges"),
    )
    dapps: dict[str, dict[str, Any]] = {}
    legacy_dapps_dir = legacy_dapps_dir or support_dir.parent / "dapps"
    overrides = load_overrides(support_dir / OVERRIDES_NAME)
    source_map = load_source_map(support_dir / SOURCE_MAP_NAME, wallets)
    web3_wallet_rows = load_web3icons_rows(web3icons_source, "wallets")
    web3_exchange_rows = load_web3icons_rows(web3icons_source, "exchanges")
    slug_by_web3_id = {
        sources["web3iconsId"]: slug
        for slug, sources in source_map.items()
        if "web3iconsId" in sources
    }
    wallet_names: dict[str, set[str]] = {}
    for slug, entry in wallets.items():
        wallet_names.setdefault(normalized_name(entry["name"]), set()).add(slug)

    for row in web3_wallet_rows:
        web3_id = row["id"]
        slug = slug_by_web3_id.get(web3_id, web3_id.lower())
        validate_slug(slug, "Web3icons wallet slug")
        sources = source_map.setdefault(slug, {})
        if sources.get("web3iconsId") not in {None, web3_id}:
            raise ValueError(f"wallet slug {slug} has conflicting Web3icons id")
        sources["web3iconsId"] = web3_id
        wallet_names.setdefault(normalized_name(row["name"]), set()).add(slug)
        svg = resolve_web3icons_svg(web3icons_source, row)
        if svg is None:
            if slug not in overrides["wallets"]:
                raise FileNotFoundError(
                    f"Web3icons wallet SVG not found for {web3_id}: {row['filePath']}"
                )
            continue
        if slug not in overrides["wallets"]:
            write_bytes_atomic(
                support_dir / "wallets" / slug / "logo.svg",
                validate_safe_svg(svg.read_bytes()),
            )
            wallets[slug] = {
                "id": slug,
                "name": row["name"],
                "logoURI": f"{asset_base_uri.rstrip('/')}/wallets/{slug}/logo.svg",
            }

    slug_by_wc_id = {
        sources["walletConnectId"]: slug
        for slug, sources in source_map.items()
        if "walletConnectId" in sources
    }
    occupied = set(wallets) | set(source_map)
    for wallet in sorted(walletconnect_wallets, key=lambda item: item["id"]):
        listing_id = wallet["id"]
        slug = slug_by_wc_id.get(listing_id)
        if slug is None:
            matches = sorted(wallet_names.get(normalized_name(wallet["name"]), set()))
            if len(matches) == 1 and "walletConnectId" not in source_map[matches[0]]:
                slug = matches[0]
            else:
                slug = unique_slug(slugify(wallet["name"]), listing_id, occupied)
            source_map.setdefault(slug, {})["walletConnectId"] = listing_id
            occupied.add(slug)
        if source_map[slug].get("walletConnectId") != listing_id:
            raise ValueError(f"wallet slug {slug} has conflicting WalletConnect id")
        extension, image = wallet.get("extension"), wallet.get("image")
        if extension not in {"svg", "png"} or not isinstance(image, bytes):
            raise ValueError(
                f"WalletConnect wallet {listing_id} has invalid normalized image"
            )
        has_web3_svg = (
            "web3iconsId" in source_map[slug]
            and (support_dir / "wallets" / slug / "logo.svg").is_file()
        )
        if not has_web3_svg and slug not in overrides["wallets"]:
            write_bytes_atomic(
                support_dir / "wallets" / slug / f"logo.{extension}", image
            )
            public_logo = (
                f"{asset_base_uri.rstrip('/')}/wallets/{slug}/logo.{extension}"
            )
            wallets[slug] = {
                "id": slug,
                "name": wallet["name"],
                "logoURI": public_logo,
            }

    for row in web3_exchange_rows:
        icon_id = row["id"].lower()
        validate_slug(icon_id, "Web3icons exchange id")
        svg = resolve_web3icons_svg(web3icons_source, row)
        if svg is None:
            if icon_id not in overrides["exchanges"]:
                raise FileNotFoundError(
                    f"Web3icons exchange SVG not found for {icon_id}: {row['filePath']}"
                )
            continue
        if icon_id not in overrides["exchanges"]:
            write_bytes_atomic(
                support_dir / "exchanges" / icon_id / "logo.svg",
                validate_safe_svg(svg.read_bytes()),
            )
            exchanges[icon_id] = {
                "id": icon_id,
                "name": row["name"],
                "type": row["type"],
                "logoURI": f"{asset_base_uri.rstrip('/')}/exchanges/{icon_id}/logo.svg",
            }

    require_dir(legacy_dapps_dir)
    dapp_asset_base_uri = dapps_asset_base_uri(asset_base_uri)
    for legacy_icon in sorted(legacy_dapps_dir.glob("*.png")):
        slug = slugify(legacy_icon.stem)
        if slug in dapps:
            raise ValueError(f"duplicate normalized dApp id: {slug}")
        validate_raster_image(legacy_icon.read_bytes())
        encoded_filename = urllib.parse.quote(legacy_icon.name, safe="._-")
        dapps[slug] = {
            "id": slug,
            "name": legacy_icon.stem,
            "logoURI": f"{dapp_asset_base_uri}/{encoded_filename}",
        }

    for category, entries in (("wallets", wallets), ("exchanges", exchanges)):
        for icon_id, override in overrides[category].items():
            require_file(support_dir / override["logoPath"])
            entry = {"id": icon_id, "name": override["name"]}
            if category == "exchanges":
                entry["type"] = override["type"]
            entry["logoURI"] = f"{asset_base_uri.rstrip('/')}/{override['logoPath']}"
            entries[icon_id] = entry
            if category == "wallets":
                source_map.setdefault(icon_id, {})

    output = {
        "schemaVersion": 1,
        "assetBaseURI": asset_base_uri.rstrip("/"),
        "exchanges": [exchanges[key] for key in sorted(exchanges)],
        "wallets": [wallets[key] for key in sorted(wallets)],
    }
    dapp_output = {
        "schemaVersion": 1,
        "assetBaseURI": dapp_asset_base_uri,
        "dapps": [dapps[key] for key in sorted(dapps)],
    }
    map_output = {
        "schemaVersion": 1,
        "wallets": {slug: source_map[slug] for slug in sorted(source_map)},
    }
    write_json(support_json, output)
    write_json(support_dir / DAPPS_JSON_NAME, dapp_output)
    write_json(support_dir / SOURCE_MAP_NAME, map_output)
    validate_output(asset_base_uri, support_dir)
    validate_dapps_output(dapp_asset_base_uri, support_dir, legacy_dapps_dir)
    print(
        f"Synced {len(dapps)} dApps, {len(exchanges)} exchanges, "
        f"and {len(wallets)} wallets"
    )


def local_path_for_logo_uri(
    logo_uri: str,
    asset_base_uri: str,
    support_dir: Path,
    category: str,
    dapps_dir: Path | None = None,
) -> Path:
    if category == "dapps":
        dapp_base = asset_base_uri.rstrip("/") + "/"
        if not logo_uri.startswith(dapp_base):
            raise ValueError(f"dApp logoURI has invalid base: {logo_uri}")
        filename = urllib.parse.unquote(logo_uri[len(dapp_base) :])
        if not filename or Path(filename).name != filename:
            raise ValueError(f"dApp logoURI has invalid filename: {logo_uri}")
        return (dapps_dir or support_dir.parent / "dapps") / filename
    base = asset_base_uri.rstrip("/") + "/"
    if not logo_uri.startswith(base):
        raise ValueError(f"logoURI does not start with assetBaseURI: {logo_uri}")
    suffix = logo_uri[len(base) :]
    if suffix.startswith("/") or ".." in Path(suffix).parts:
        raise ValueError(f"logoURI has invalid local suffix: {logo_uri}")
    return support_dir / suffix


def validate_entries(
    data: dict[str, Any],
    key: str,
    asset_base_uri: str,
    support_dir: Path,
    dapps_dir: Path | None = None,
) -> None:
    entries = data.get(key)
    if not isinstance(entries, list):
        raise ValueError(f"support.json field must be an array: {key}")
    previous_id = ""
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict):
            raise ValueError(f"{key}[{index}] must be an object")
        icon_id, name, logo_uri = (
            entry.get("id"),
            entry.get("name"),
            entry.get("logoURI"),
        )
        if not isinstance(icon_id, str):
            raise ValueError(f"{key}[{index}] has invalid id")
        validate_slug(icon_id, f"{key}[{index}].id")
        if not isinstance(name, str) or not name:
            raise ValueError(f"{key}[{index}] has invalid name")
        if not isinstance(logo_uri, str) or not logo_uri:
            raise ValueError(f"{key}[{index}] has invalid logoURI")
        if icon_id <= previous_id:
            raise ValueError(f"{key} entries must be sorted by id")
        previous_id = icon_id
        if key == "exchanges" and entry.get("type") not in {"cex", "dex"}:
            raise ValueError(f"{key}[{index}] has invalid type")
        require_file(
            local_path_for_logo_uri(
                logo_uri, asset_base_uri, support_dir, key, dapps_dir
            )
        )


def validate_output(
    asset_base_uri: str | None = None, support_dir: Path = SUPPORT_DIR
) -> None:
    support_json = support_dir / SUPPORT_JSON_NAME
    require_file(support_json)
    data = load_json(support_json)
    if not isinstance(data, dict) or data.get("schemaVersion") != 1:
        raise ValueError("support.json must be a schemaVersion 1 object")
    base_uri = asset_base_uri or data.get("assetBaseURI")
    if not isinstance(base_uri, str) or not base_uri:
        raise ValueError("support.json assetBaseURI must be a non-empty string")
    if data.get("assetBaseURI") != base_uri.rstrip("/"):
        raise ValueError("support.json assetBaseURI does not match expected value")
    validate_entries(data, "exchanges", base_uri, support_dir)
    validate_entries(data, "wallets", base_uri, support_dir)
    load_source_map(support_dir / SOURCE_MAP_NAME, entry_map(data, "wallets"))


def validate_dapps_output(
    dapp_asset_base_uri: str | None = None,
    support_dir: Path = SUPPORT_DIR,
    dapps_dir: Path | None = None,
) -> None:
    dapps_json = support_dir / DAPPS_JSON_NAME
    require_file(dapps_json)
    data = load_json(dapps_json)
    if not isinstance(data, dict) or data.get("schemaVersion") != 1:
        raise ValueError("dapps.json must be a schemaVersion 1 object")
    base_uri = dapp_asset_base_uri or data.get("assetBaseURI")
    if not isinstance(base_uri, str) or not base_uri:
        raise ValueError("dapps.json assetBaseURI must be a non-empty string")
    if data.get("assetBaseURI") != base_uri.rstrip("/"):
        raise ValueError("dapps.json assetBaseURI does not match expected value")
    validate_entries(data, "dapps", base_uri, support_dir, dapps_dir)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--web3icons-source",
        type=Path,
        help="Path to a checked out 0xa3k5/web3icons repository",
    )
    parser.add_argument(
        "--walletconnect-project-id",
        default=os.environ.get("WALLETCONNECT_PROJECT_ID", ""),
    )
    parser.add_argument(
        "--skip-walletconnect",
        action="store_true",
        help="Sync only Web3icons for local maintenance",
    )
    parser.add_argument("--asset-base-uri", default=DEFAULT_ASSET_BASE_URI)
    parser.add_argument("--validate-output", action="store_true")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        if args.validate_output and args.web3icons_source is None:
            validate_output(args.asset_base_uri)
            validate_dapps_output(dapps_asset_base_uri(args.asset_base_uri))
            print(f"Validated {SUPPORT_JSON_NAME} and {DAPPS_JSON_NAME}")
            return 0
        if args.web3icons_source is None:
            raise ValueError("--web3icons-source is required")
        if not args.skip_walletconnect and not args.walletconnect_project_id:
            message = "WALLETCONNECT_PROJECT_ID is required"
            raise ValueError(f"{message} unless --skip-walletconnect is used")
        walletconnect_wallets = (
            []
            if args.skip_walletconnect
            else fetch_walletconnect_wallets(args.walletconnect_project_id)
        )
        with tempfile.TemporaryDirectory(prefix="support-sync-") as temp_dir:
            staged = Path(temp_dir) / "support"
            shutil.copytree(SUPPORT_DIR, staged)
            sync_support(
                args.web3icons_source.resolve(),
                walletconnect_wallets,
                staged,
                args.asset_base_uri,
                REPO_ROOT / "dapps",
            )
            for directory in ("wallets", "exchanges"):
                for source_file in (staged / directory).glob("*/*"):
                    if source_file.is_file():
                        copy_atomic_if_changed(
                            source_file, SUPPORT_DIR / source_file.relative_to(staged)
                        )
            for name in (
                SUPPORT_JSON_NAME,
                DAPPS_JSON_NAME,
                SOURCE_MAP_NAME,
            ):
                copy_atomic_if_changed(staged / name, SUPPORT_DIR / name)
        validate_output(args.asset_base_uri)
        validate_dapps_output(dapps_asset_base_uri(args.asset_base_uri))
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
