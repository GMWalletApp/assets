"""Build the wallet homepage token list directly from this Trust Wallet assets repo.

Output:
  out/homepage.json = {
    "generatedAt": "...",
    "chainOrder": [...],
    "slotOrder": ["native", "usdt", "usdc", "dai"],
    "chains": [...],
    "tokens": [...]
  }

Rules:
  - include one native coin per configured chain
  - include BTC only as bitcoin native
  - for each non-bitcoin chain, include at most one USDT, one USDC, and one DAI
  - if multiple candidates exist for a slot, rank them and choose the top-ranked one
  - if a slot has no matching asset in this repo, skip it and report it

Data sources:
  - native chain metadata: blockchains/<chain>/info/info.json
  - token metadata: blockchains/<chain>/assets/<address>/info.json

Notes:
  - token explorer URLs come directly from Trust Wallet info.json
  - chain explorer URLs come directly from Trust Wallet info.json
  - logo URIs are derived from the canonical Trust Wallet CDN path
  - export aliases such as smartchain -> bsc apply only to homepage output fields
  - chain display metadata in CHAIN_CONFIG is intentionally curated for wallet use
  - manual homepage overrides are merged after the automatic build
"""

from __future__ import annotations

import argparse
import json
import os
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

HERE = Path(__file__).resolve().parent
REPO_ROOT = HERE.parents[1]
OUT_DIR = HERE / "out"
BLOCKCHAINS_DIR = REPO_ROOT / "blockchains"
TRUSTWALLET_CDN = "https://assets-cdn.trustwallet.com"
DEFAULT_OVERRIDES_FILE = HERE / "homepage_overrides.json"
INLINE_OVERRIDES_ENV = "HOMEPAGE_INLINE_OVERRIDES_JSON"

EXPORT_CHAIN_KEYS = {
    "smartchain": "bsc",
}
CANONICAL_CHAIN_KEYS = {export: canonical for canonical, export in EXPORT_CHAIN_KEYS.items()}

CHAIN_ORDER = [
    "ethereum",
    "bitcoin",
    "smartchain",
    "polygon",
    "arbitrum",
    "optimism",
    "base",
    "avalanchec",
    "fantom",
    "linea",
    "zksync",
    "solana",
    "tron",
]

# Trust Wallet chain metadata is not always wallet-friendly at the chain level
# (for example, some L2 native symbols are ARETH/OETH in the repo). Keep the
# homepage chain header fields explicit here, while token metadata still comes
# directly from blockchains/<chain>/assets/<address>/info.json.
CHAIN_CONFIG = {
    "ethereum": {"chainName": "Ethereum", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 1},
    "bitcoin": {"chainName": "Bitcoin", "nativeSymbol": "BTC", "nativeName": "Bitcoin", "chainId": None},
    "smartchain": {"chainName": "BNB Smart Chain", "nativeSymbol": "BNB", "nativeName": "BNB", "chainId": 56},
    "polygon": {"chainName": "Polygon", "nativeSymbol": "POL", "nativeName": "Polygon", "chainId": 137},
    "arbitrum": {"chainName": "Arbitrum", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 42161},
    "optimism": {"chainName": "Optimism", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 10},
    "base": {"chainName": "Base", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 8453},
    "avalanchec": {"chainName": "Avalanche", "nativeSymbol": "AVAX", "nativeName": "Avalanche", "chainId": 43114},
    "fantom": {"chainName": "Fantom", "nativeSymbol": "FTM", "nativeName": "Fantom", "chainId": 250},
    "linea": {"chainName": "Linea", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 59144},
    "zksync": {"chainName": "zkSync Era", "nativeSymbol": "ETH", "nativeName": "Ethereum", "chainId": 324},
    "solana": {"chainName": "Solana", "nativeSymbol": "SOL", "nativeName": "Solana", "chainId": None},
    "tron": {"chainName": "TRON", "nativeSymbol": "TRX", "nativeName": "TRON", "chainId": None},
}

SLOT_ORDER = ["native", "usdt", "usdc", "dai"]
TARGET_SLOTS = ["usdt", "usdc", "dai"]
TOKENLESS_CHAINS = {"bitcoin"}

SLOT_META = {
    "native": {"displayName": None, "displaySymbol": None},
    "usdt": {"displayName": "Tether", "displaySymbol": "USDT"},
    "usdc": {"displayName": "USD Coin", "displaySymbol": "USDC"},
    "dai": {"displayName": "Dai", "displaySymbol": "DAI"},
}

NAME_PENALTY_TERMS = (
    "wormhole",
    "bridged",
    "multichain",
    "portal",
    "wrapped",
)


@dataclass(frozen=True)
class SlotRule:
    preferred_symbols: tuple[str, ...]
    fallback_symbols: tuple[str, ...] = ()
    banned_symbols: tuple[str, ...] = ()
    preferred_name_terms: tuple[str, ...] = ()


@dataclass(frozen=True)
class AssetCandidate:
    address: str
    symbol: str
    name: str
    explorer: str
    decimals: int | None
    status: str
    tags: tuple[str, ...]


SLOT_RULES = {
    "usdt": SlotRule(
        preferred_symbols=("USDT",),
        fallback_symbols=("USDT",),
        banned_symbols=("USDT0", "USDT+"),
        preferred_name_terms=("tether", "usdt"),
    ),
    "usdc": SlotRule(
        preferred_symbols=("USDC",),
        fallback_symbols=("USDC.E",),
        preferred_name_terms=("usd coin", "usdc"),
    ),
    "dai": SlotRule(
        preferred_symbols=("DAI",),
        fallback_symbols=("DAI.E",),
        preferred_name_terms=("dai",),
    ),
}


def load_json(path: Path) -> Any:
    return json.loads(path.read_text(encoding="utf-8"))


def export_chain_key(chain: str) -> str:
    return EXPORT_CHAIN_KEYS.get(chain, chain)


def canonical_chain_key(chain: str) -> str:
    return CANONICAL_CHAIN_KEYS.get(chain, chain)


def canonical_symbol(symbol: str | None) -> str:
    return (symbol or "").strip().upper()


def build_chain_logo_uri(chain: str) -> str:
    return f"{TRUSTWALLET_CDN}/blockchains/{chain}/info/logo.png"


def build_token_logo_uri(chain: str, address: str) -> str:
    return f"{TRUSTWALLET_CDN}/blockchains/{chain}/assets/{address}/logo.png"


class AssetCatalog:
    def __init__(self) -> None:
        self.asset_dirs: dict[str, dict[str, Path]] = {}
        self.candidates: dict[str, list[AssetCandidate]] = {}

    def _asset_index(self, chain: str) -> dict[str, Path]:
        if chain not in self.asset_dirs:
            base = BLOCKCHAINS_DIR / chain / "assets"
            index: dict[str, Path] = {}
            if base.exists():
                for item in base.iterdir():
                    if item.is_dir():
                        index[item.name.lower()] = item
            self.asset_dirs[chain] = index
        return self.asset_dirs[chain]

    def resolve_token(self, chain: str, address: str) -> dict[str, Any]:
        asset_dir = self._asset_index(chain).get(address.lower())
        if asset_dir is None:
            raise KeyError(f"asset metadata not found for {chain}:{address}")

        info = load_json(asset_dir / "info.json")
        return {
            "address": asset_dir.name,
            "symbol": info.get("symbol"),
            "name": info.get("name"),
            "decimals": info.get("decimals"),
            "logoURI": build_token_logo_uri(chain, asset_dir.name),
            "explorer": info.get("explorer"),
            "source": "trustwallet-asset",
        }

    def candidate_by_address(self, chain: str, address: str) -> AssetCandidate:
        needle = address.lower()
        for candidate in self.iter_candidates(chain):
            if candidate.address.lower() == needle:
                return candidate
        raise KeyError(f"candidate not found for {chain}:{address}")

    def iter_candidates(self, chain: str) -> list[AssetCandidate]:
        if chain not in self.candidates:
            rows: list[AssetCandidate] = []
            base = BLOCKCHAINS_DIR / chain / "assets"
            if base.exists():
                for item in base.iterdir():
                    if not item.is_dir():
                        continue
                    info_path = item / "info.json"
                    if not info_path.exists():
                        continue
                    info = load_json(info_path)
                    rows.append(AssetCandidate(
                        address=item.name,
                        symbol=(info.get("symbol") or "").strip(),
                        name=(info.get("name") or "").strip(),
                        explorer=(info.get("explorer") or "").strip(),
                        decimals=info.get("decimals"),
                        status=(info.get("status") or "").strip().lower(),
                        tags=tuple(info.get("tags") or []),
                    ))
            self.candidates[chain] = rows
        return self.candidates[chain]


def load_chain_meta(chain: str) -> dict[str, Any]:
    info = load_json(BLOCKCHAINS_DIR / chain / "info" / "info.json")
    cfg = CHAIN_CONFIG[chain]
    return {
        "canonicalKey": chain,
        "exportKey": export_chain_key(chain),
        "name": cfg["chainName"],
        "symbol": cfg["nativeSymbol"],
        "nativeName": cfg["nativeName"],
        "decimals": info.get("decimals"),
        "chainId": cfg["chainId"],
        "logoURI": build_chain_logo_uri(chain),
        "explorer": info.get("explorer"),
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build homepage.json from Trust Wallet assets plus manual overrides.")
    parser.add_argument(
        "--overrides-file",
        default=str(DEFAULT_OVERRIDES_FILE),
        help="JSON file with manual homepage override tokens. Defaults to data/tokenlists/homepage_overrides.json.",
    )
    parser.add_argument(
        "--inline-overrides-json",
        default=os.environ.get(INLINE_OVERRIDES_ENV, ""),
        help=f"Inline JSON override payload. Defaults to ${INLINE_OVERRIDES_ENV} when set.",
    )
    parser.add_argument(
        "--save-inline-overrides",
        action="store_true",
        help="Merge inline overrides into the overrides file before building homepage.json.",
    )
    return parser.parse_args()


def candidate_sort_key(slot: str, candidate: AssetCandidate) -> tuple:
    rule = SLOT_RULES[slot]
    symbol = canonical_symbol(candidate.symbol)
    name = candidate.name.lower()
    tags = {tag.lower() for tag in candidate.tags}

    symbol_rank = 0
    if symbol in rule.preferred_symbols:
        symbol_rank = 2
    elif symbol in rule.fallback_symbols:
        symbol_rank = 1

    preferred_name_hits = sum(1 for term in rule.preferred_name_terms if term in name)
    penalty_hits = sum(1 for term in NAME_PENALTY_TERMS if term in name)
    stablecoin_bonus = 1 if "stablecoin" in tags else 0
    clean_name = 1 if penalty_hits == 0 else 0

    return (
        symbol_rank,
        clean_name,
        stablecoin_bonus,
        preferred_name_hits,
        -penalty_hits,
        -(candidate.decimals or 0),
        candidate.address.lower(),
    )


def pick_slot_address(chain: str, slot: str, catalog: AssetCatalog) -> str | None:
    rule = SLOT_RULES[slot]
    ranked: list[tuple[tuple, AssetCandidate]] = []

    for candidate in catalog.iter_candidates(chain):
        if candidate.status != "active":
            continue
        symbol = canonical_symbol(candidate.symbol)
        if symbol in rule.banned_symbols:
            continue
        if symbol not in rule.preferred_symbols and symbol not in rule.fallback_symbols:
            continue
        ranked.append((candidate_sort_key(slot, candidate), candidate))

    if not ranked:
        return None

    ranked.sort(key=lambda item: item[0], reverse=True)
    return ranked[0][1].address


def variant_note(slot: str, candidate: AssetCandidate) -> str | None:
    rule = SLOT_RULES[slot]
    symbol = canonical_symbol(candidate.symbol)
    name = candidate.name.lower()

    reasons: list[str] = []
    if symbol not in rule.preferred_symbols:
        reasons.append(f"symbol={candidate.symbol}")
    if any(term in name for term in NAME_PENALTY_TERMS):
        reasons.append(f"name={candidate.name}")
    if not reasons:
        return None
    return f"{slot} -> {candidate.address} ({', '.join(reasons)})"


def normalize_override_entries(raw: Any, origin: str) -> list[dict[str, Any]]:
    if raw is None:
        return []
    if isinstance(raw, dict) and "tokens" in raw:
        raw = raw["tokens"]
    elif isinstance(raw, dict):
        raw = [raw]

    if not isinstance(raw, list):
        raise ValueError(f"{origin}: expected a token object, a token array, or an object with tokens[]")

    rows: list[dict[str, Any]] = []
    for idx, item in enumerate(raw):
        if not isinstance(item, dict):
            raise ValueError(f"{origin}: token override #{idx + 1} must be an object")
        rows.append(item)
    return rows


def load_override_entries(path_text: str, inline_json: str) -> list[dict[str, Any]]:
    rows: list[dict[str, Any]] = []

    path = Path(path_text)
    if path.exists():
        rows.extend(normalize_override_entries(load_json(path), str(path)))

    if inline_json.strip():
        rows.extend(normalize_override_entries(json.loads(inline_json), f"inline overrides via {INLINE_OVERRIDES_ENV}"))

    return rows


def validate_override_entry(entry: dict[str, Any]) -> dict[str, Any]:
    chain_value = str(entry.get("chain") or "").strip()
    chain = canonical_chain_key(chain_value)
    export_chain = export_chain_key(chain)
    slot = str(entry.get("slot") or "").strip().lower()
    address = str(entry.get("address") or "").strip()
    symbol = str(entry.get("symbol") or "").strip()
    name = str(entry.get("name") or "").strip()
    explorer = str(entry.get("explorer") or "").strip()
    source = str(entry.get("source") or "manual-override").strip()
    logo_uri = str(entry.get("logoURI") or "").strip() or None
    decimals = entry.get("decimals")
    kind = str(entry.get("kind") or "token").strip()
    token_id = str(entry.get("id") or "").strip()

    if chain not in CHAIN_CONFIG:
        raise ValueError(f"unknown override chain: {entry.get('chain')}")
    if slot not in TARGET_SLOTS:
        raise ValueError(f"override slot must be one of {', '.join(TARGET_SLOTS)}: {slot}")
    if kind != "token":
        raise ValueError(f"override {chain}:{slot} kind must be token")
    if token_id and token_id != f"{export_chain}:{slot}":
        raise ValueError(f"override {chain}:{slot} id must match {export_chain}:{slot}")
    if not address:
        raise ValueError(f"override {chain}:{slot} missing address")
    if not symbol:
        raise ValueError(f"override {chain}:{slot} missing symbol")
    if not name:
        raise ValueError(f"override {chain}:{slot} missing name")
    if explorer == "":
        raise ValueError(f"override {chain}:{slot} missing explorer")
    if not isinstance(decimals, int):
        raise ValueError(f"override {chain}:{slot} decimals must be an integer")
    if logo_uri is None:
        raise ValueError(f"override {chain}:{slot} missing logoURI")

    return {
        "chain": chain,
        "slot": slot,
        "address": address,
        "symbol": symbol,
        "name": name,
        "explorer": explorer,
        "decimals": decimals,
        "logoURI": logo_uri,
        "source": source,
    }


def build_manual_token(chain_meta: dict[str, Any], entry: dict[str, Any]) -> dict[str, Any]:
    slot_meta = SLOT_META[entry["slot"]]
    return {
        "id": f"{chain_meta['exportKey']}:{entry['slot']}",
        "chain": chain_meta["exportKey"],
        "chainName": chain_meta["name"],
        "chainId": chain_meta["chainId"],
        "chainLogoURI": chain_meta["logoURI"],
        "slot": entry["slot"],
        "kind": "token",
        "displaySymbol": slot_meta["displaySymbol"],
        "displayName": slot_meta["displayName"],
        "symbol": entry["symbol"],
        "name": entry["name"],
        "address": entry["address"],
        "decimals": entry["decimals"],
        "logoURI": entry["logoURI"],
        "explorer": entry["explorer"],
        "source": entry["source"],
    }


def override_sort_key(entry: dict[str, Any]) -> tuple[int, int]:
    chain_rank = {export_chain_key(chain): idx for idx, chain in enumerate(CHAIN_ORDER)}
    slot_rank = {slot: idx for idx, slot in enumerate(SLOT_ORDER)}
    token = entry if "slot" in entry and "chain" in entry else {}
    return (
        chain_rank.get(str(token.get("chain") or ""), len(chain_rank)),
        slot_rank.get(str(token.get("slot") or ""), len(slot_rank)),
    )


def write_json(path: Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def persist_override_entries(
    path_text: str,
    entries: list[dict[str, Any]],
    chain_meta_by_canonical: dict[str, dict[str, Any]],
) -> list[dict[str, Any]]:
    merged_by_slot: dict[tuple[str, str], dict[str, Any]] = {}
    for entry in entries:
        row = validate_override_entry(entry)
        merged_by_slot[(row["chain"], row["slot"])] = row

    stored_tokens = [
        build_manual_token(chain_meta_by_canonical[row["chain"]], row)
        for row in merged_by_slot.values()
    ]
    stored_tokens.sort(key=override_sort_key)
    write_json(Path(path_text), {"tokens": stored_tokens})
    return stored_tokens


def sort_tokens(tokens: list[dict[str, Any]]) -> list[dict[str, Any]]:
    chain_rank = {export_chain_key(chain): idx for idx, chain in enumerate(CHAIN_ORDER)}
    slot_rank = {slot: idx for idx, slot in enumerate(SLOT_ORDER)}
    return sorted(
        tokens,
        key=lambda token: (
            chain_rank.get(token["chain"], len(chain_rank)),
            slot_rank.get(token["slot"], len(slot_rank)),
        ),
    )


def build_chain_summaries(tokens: list[dict[str, Any]], chain_meta_by_canonical: dict[str, dict[str, Any]]) -> list[dict[str, Any]]:
    by_chain: dict[str, list[dict[str, Any]]] = {}
    for token in tokens:
        by_chain.setdefault(token["chain"], []).append(token)

    chains: list[dict[str, Any]] = []
    for canonical in CHAIN_ORDER:
        chain_meta = chain_meta_by_canonical[canonical]
        export_key = chain_meta["exportKey"]
        chain_tokens = by_chain.get(export_key, [])
        chains.append({
            "key": export_key,
            "name": chain_meta["name"],
            "symbol": chain_meta["symbol"],
            "chainId": chain_meta["chainId"],
            "logoURI": chain_meta["logoURI"],
            "explorer": chain_meta["explorer"],
            "availableSlots": [token["slot"] for token in chain_tokens],
            "tokenCount": len(chain_tokens),
        })
    return chains


def compute_missing_slots(tokens: list[dict[str, Any]], chain_meta_by_canonical: dict[str, dict[str, Any]]) -> dict[str, list[str]]:
    by_chain: dict[str, set[str]] = {}
    for token in tokens:
        by_chain.setdefault(token["chain"], set()).add(token["slot"])

    missing: dict[str, list[str]] = {}
    for canonical in CHAIN_ORDER:
        if canonical in TOKENLESS_CHAINS:
            continue
        export_key = chain_meta_by_canonical[canonical]["exportKey"]
        present = by_chain.get(export_key, set())
        slots = [slot for slot in TARGET_SLOTS if slot not in present]
        if slots:
            missing[canonical] = slots
    return missing


def apply_manual_overrides(
    tokens: list[dict[str, Any]],
    entries: list[dict[str, Any]],
    chain_meta_by_canonical: dict[str, dict[str, Any]],
) -> tuple[list[dict[str, Any]], list[str]]:
    if not entries:
        return sort_tokens(tokens), []

    validated: list[dict[str, Any]] = []
    seen: set[tuple[str, str]] = set()
    for entry in entries:
        row = validate_override_entry(entry)
        key = (row["chain"], row["slot"])
        if key in seen:
            raise ValueError(f"duplicate manual override for {row['chain']}:{row['slot']}")
        seen.add(key)
        validated.append(row)

    by_id = {token["id"]: token for token in tokens}
    notes: list[str] = []
    for entry in validated:
        chain_meta = chain_meta_by_canonical[entry["chain"]]
        token = build_manual_token(chain_meta, entry)
        by_id[token["id"]] = token
        notes.append(f"{token['id']} -> {entry['address']} ({entry['source']})")

    return sort_tokens(list(by_id.values())), notes


def build_native_token(chain_meta: dict[str, Any]) -> dict[str, Any]:
    return {
        "id": f"{chain_meta['exportKey']}:native",
        "chain": chain_meta["exportKey"],
        "chainName": chain_meta["name"],
        "chainId": chain_meta["chainId"],
        "chainLogoURI": chain_meta["logoURI"],
        "slot": "native",
        "kind": "native",
        "displaySymbol": chain_meta["symbol"],
        "displayName": chain_meta["nativeName"],
        "symbol": chain_meta["symbol"],
        "name": chain_meta["nativeName"],
        "address": None,
        "decimals": chain_meta["decimals"],
        "logoURI": chain_meta["logoURI"],
        "explorer": chain_meta["explorer"],
        "source": "trustwallet-chain",
    }


def build_slot_token(
    chain_meta: dict[str, Any],
    slot: str,
    address: str,
    catalog: AssetCatalog,
) -> dict[str, Any]:
    token = catalog.resolve_token(chain_meta["canonicalKey"], address)
    slot_meta = SLOT_META[slot]
    return {
        "id": f"{chain_meta['exportKey']}:{slot}",
        "chain": chain_meta["exportKey"],
        "chainName": chain_meta["name"],
        "chainId": chain_meta["chainId"],
        "chainLogoURI": chain_meta["logoURI"],
        "slot": slot,
        "kind": "token",
        "displaySymbol": slot_meta["displaySymbol"],
        "displayName": slot_meta["displayName"],
        "symbol": token.get("symbol"),
        "name": token.get("name"),
        "address": token.get("address"),
        "decimals": token.get("decimals"),
        "logoURI": token.get("logoURI"),
        "explorer": token.get("explorer"),
        "source": token.get("source"),
    }


def main() -> int:
    args = parse_args()
    catalog = AssetCatalog()
    chain_meta_by_canonical = {chain: load_chain_meta(chain) for chain in CHAIN_ORDER}

    if args.save_inline_overrides and args.inline_overrides_json.strip():
        combined_entries = load_override_entries(args.overrides_file, args.inline_overrides_json)
        persist_override_entries(args.overrides_file, combined_entries, chain_meta_by_canonical)
        args.inline_overrides_json = ""

    tokens: list[dict[str, Any]] = []
    variant_notes: dict[str, tuple[str, str]] = {}

    for chain in CHAIN_ORDER:
        chain_meta = chain_meta_by_canonical[chain]
        chain_tokens = [build_native_token(chain_meta)]

        if chain not in TOKENLESS_CHAINS:
            for slot in TARGET_SLOTS:
                address = pick_slot_address(chain, slot, catalog)
                if address is None:
                    continue
                candidate = catalog.candidate_by_address(chain, address)
                note = variant_note(slot, candidate)
                token = build_slot_token(chain_meta, slot, address, catalog)
                if note is not None:
                    variant_notes[token["id"]] = (token["address"], note)
                chain_tokens.append(token)

        tokens.extend(chain_tokens)

    override_entries = load_override_entries(args.overrides_file, args.inline_overrides_json)
    tokens, applied_override_notes = apply_manual_overrides(tokens, override_entries, chain_meta_by_canonical)
    chains = build_chain_summaries(tokens, chain_meta_by_canonical)
    missing_slots = compute_missing_slots(tokens, chain_meta_by_canonical)

    payload = {
        "generatedAt": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "chainOrder": [export_chain_key(chain) for chain in CHAIN_ORDER],
        "slotOrder": SLOT_ORDER,
        "chains": chains,
        "tokens": tokens,
    }

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    out_path = OUT_DIR / "homepage.json"
    out_path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n")

    print()
    print(f"{'chain':<12} {'tokens':>6}  slots")
    print("-" * 72)
    for chain in chains:
        slots = ",".join(chain["availableSlots"])
        print(f"{chain['key']:<12} {chain['tokenCount']:>6}  {slots}")
    if missing_slots:
        print("\nMissing homepage slots:")
        for chain in CHAIN_ORDER:
            slots = missing_slots.get(chain)
            if slots:
                print(f"  {export_chain_key(chain)}: {','.join(slots)}")
    if applied_override_notes:
        print("\nApplied homepage overrides:")
        for note in applied_override_notes:
            print(f"  {note}")
    if variant_notes:
        print("\nSelected variant homepage slots:")
        for token in tokens:
            variant = variant_notes.get(token["id"])
            if variant is None:
                continue
            variant_address, note = variant
            if token.get("source") == "trustwallet-asset" and token.get("address") == variant_address:
                print(f"  {token['chain']}: {note}")
    print(f"\nWrote {out_path} ({len(tokens)} tokens)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
