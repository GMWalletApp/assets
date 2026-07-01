#!/usr/bin/env python3
"""Sync support exchange and wallet branded logos from crypto-icons.

This script intentionally copies upstream SVG files as-is. It only changes the
directory layout and regenerates support/support.json for frontend lookup.
"""

from __future__ import annotations

import argparse
import json
import shutil
import sys
from pathlib import Path
from typing import Any, Optional


REPO_ROOT = Path(__file__).resolve().parents[1]
SUPPORT_DIR = REPO_ROOT / "support"
SUPPORT_JSON = SUPPORT_DIR / "support.json"
DEFAULT_ASSET_BASE_URI = "https://raw.githubusercontent.com/GMWalletApp/assets/main/support"


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, data: Any) -> None:
    path.write_text(
        json.dumps(data, ensure_ascii=False, indent=2, sort_keys=False) + "\n",
        encoding="utf-8",
    )


def require_dir(path: Path) -> None:
    if not path.is_dir():
        raise FileNotFoundError(f"required directory not found: {path}")


def require_file(path: Path) -> None:
    if not path.is_file():
        raise FileNotFoundError(f"required file not found: {path}")


def branded_svg_ids(source: Path, category: str) -> set[str]:
    branded_dir = source / "assets" / category / "branded"
    require_dir(branded_dir)
    return {path.stem for path in branded_dir.glob("*.svg") if path.is_file()}


def map_rows_by_id(source: Path, category: str) -> dict[str, dict[str, Any]]:
    map_file = source / "maps" / f"{category}.json"
    require_file(map_file)
    rows = load_json(map_file)
    if not isinstance(rows, list):
        raise ValueError(f"{map_file} must contain a JSON array")

    by_id: dict[str, dict[str, Any]] = {}
    for index, row in enumerate(rows):
        if not isinstance(row, dict):
            raise ValueError(f"{map_file} row {index} must be an object")

        icon_id = row.get("id")
        name = row.get("name")
        if not isinstance(icon_id, str) or not icon_id:
            raise ValueError(f"{map_file} row {index} has invalid id")
        if not isinstance(name, str) or not name:
            raise ValueError(f"{map_file} row {index} has invalid name")
        if icon_id in by_id:
            raise ValueError(f"{map_file} contains duplicate id: {icon_id}")

        by_id[icon_id] = row
    return by_id


def copy_category(
    source: Path,
    category: str,
    output_category: str,
    asset_base_uri: str,
) -> list[dict[str, Any]]:
    icon_ids = branded_svg_ids(source, category)
    rows_by_id = map_rows_by_id(source, category)
    missing_map_rows = sorted(icon_ids - set(rows_by_id))
    if missing_map_rows:
        raise ValueError(
            f"{category} branded SVGs missing from maps/{category}.json: "
            + ", ".join(missing_map_rows)
        )

    output_dir = SUPPORT_DIR / output_category
    if output_dir.exists():
        shutil.rmtree(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    entries: list[dict[str, Any]] = []
    branded_dir = source / "assets" / category / "branded"
    for icon_id in sorted(icon_ids):
        source_svg = branded_dir / f"{icon_id}.svg"
        target_dir = output_dir / icon_id
        target_dir.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(source_svg, target_dir / "logo.svg")

        row = rows_by_id[icon_id]
        logo_path = f"{output_category}/{icon_id}/logo.svg"
        entry: dict[str, Any] = {
            "id": icon_id,
            "name": row["name"],
        }
        if category == "exchanges":
            exchange_type = row.get("type")
            if not isinstance(exchange_type, str) or not exchange_type:
                raise ValueError(f"exchange map row has invalid type: {icon_id}")
            entry["type"] = exchange_type

        entry["logoURI"] = f"{asset_base_uri.rstrip('/')}/{logo_path}"
        entries.append(entry)

    return entries


def sync_support(source: Path, asset_base_uri: str) -> None:
    require_dir(source)
    exchanges = copy_category(source, "exchanges", "exchanges", asset_base_uri)
    wallets = copy_category(source, "wallets", "wallets", asset_base_uri)

    output = {
        "schemaVersion": 1,
        "assetBaseURI": asset_base_uri.rstrip("/"),
        "exchanges": exchanges,
        "wallets": wallets,
    }
    write_json(SUPPORT_JSON, output)
    validate_output(asset_base_uri)
    print(
        f"Synced {len(exchanges)} exchanges and {len(wallets)} wallets to "
        f"{SUPPORT_JSON.relative_to(REPO_ROOT)}"
    )


def local_path_for_logo_uri(logo_uri: str, asset_base_uri: str) -> Path:
    base = asset_base_uri.rstrip("/") + "/"
    if not logo_uri.startswith(base):
        raise ValueError(f"logoURI does not start with assetBaseURI: {logo_uri}")

    suffix = logo_uri[len(base) :]
    if suffix.startswith("/") or ".." in Path(suffix).parts:
        raise ValueError(f"logoURI has invalid local suffix: {logo_uri}")

    return SUPPORT_DIR / suffix


def validate_entries(data: dict[str, Any], key: str, asset_base_uri: str) -> None:
    entries = data.get(key)
    if not isinstance(entries, list):
        raise ValueError(f"support.json field must be an array: {key}")

    previous_id = ""
    for index, entry in enumerate(entries):
        if not isinstance(entry, dict):
            raise ValueError(f"{key}[{index}] must be an object")

        icon_id = entry.get("id")
        name = entry.get("name")
        logo_uri = entry.get("logoURI")
        if not isinstance(icon_id, str) or not icon_id:
            raise ValueError(f"{key}[{index}] has invalid id")
        if not isinstance(name, str) or not name:
            raise ValueError(f"{key}[{index}] has invalid name")
        if not isinstance(logo_uri, str) or not logo_uri:
            raise ValueError(f"{key}[{index}] has invalid logoURI")
        if icon_id <= previous_id:
            raise ValueError(f"{key} entries must be sorted by id")
        previous_id = icon_id

        if key == "exchanges":
            exchange_type = entry.get("type")
            if not isinstance(exchange_type, str) or not exchange_type:
                raise ValueError(f"{key}[{index}] has invalid type")

        local_path = local_path_for_logo_uri(logo_uri, asset_base_uri)
        require_file(local_path)


def validate_output(asset_base_uri: Optional[str] = None) -> None:
    require_file(SUPPORT_JSON)
    data = load_json(SUPPORT_JSON)
    if not isinstance(data, dict):
        raise ValueError("support.json must contain an object")
    if data.get("schemaVersion") != 1:
        raise ValueError("support.json schemaVersion must be 1")

    base_uri = asset_base_uri or data.get("assetBaseURI")
    if not isinstance(base_uri, str) or not base_uri:
        raise ValueError("support.json assetBaseURI must be a non-empty string")
    if data.get("assetBaseURI") != base_uri.rstrip("/"):
        raise ValueError("support.json assetBaseURI does not match expected value")

    validate_entries(data, "exchanges", base_uri)
    validate_entries(data, "wallets", base_uri)


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--source",
        type=Path,
        help="Path to a checked out GMWalletApp/crypto-icons repository.",
    )
    parser.add_argument(
        "--asset-base-uri",
        default=DEFAULT_ASSET_BASE_URI,
        help="Base raw URL used when generating support/support.json.",
    )
    parser.add_argument(
        "--validate-output",
        action="store_true",
        help="Validate the existing support/support.json and referenced logos.",
    )
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    try:
        if args.validate_output and args.source is None:
            validate_output(args.asset_base_uri)
            print(f"Validated {SUPPORT_JSON.relative_to(REPO_ROOT)}")
            return 0

        if args.source is None:
            raise ValueError("--source is required unless --validate-output is used")

        sync_support(args.source.resolve(), args.asset_base_uri)
        return 0
    except Exception as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
