"""Tests for official swap-provider logo overrides.

Run from the repository root (Pillow must already be installed):

    python -m unittest discover -s support -p 'test_*.py' -v
"""

from __future__ import annotations

import contextlib
import io
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock

SUPPORT_DIR = Path(__file__).resolve().parent
if str(SUPPORT_DIR) not in sys.path:
    sys.path.insert(0, str(SUPPORT_DIR))

from PIL import Image

from sync_support import load_json, write_json
from sync_swap_providers import (
    load_overrides,
    sync_swap_providers,
    validate_swap_provider_output,
)

ASSET_BASE = "https://assets.example/support"


def webp_bytes(color: tuple[int, int, int, int], size: tuple[int, int] = (8, 8)) -> bytes:
    image = Image.new("RGBA", size, color)
    buffer = io.BytesIO()
    image.save(buffer, format="WEBP", lossless=True, quality=100, method=6)
    return buffer.getvalue()


def png_bytes(color: tuple[int, int, int, int] = (1, 2, 3, 255)) -> bytes:
    image = Image.new("RGBA", (8, 8), color)
    buffer = io.BytesIO()
    image.save(buffer, format="PNG")
    return buffer.getvalue()


def protocol(slug: str, name: str, url: str, category: str = "Dexs") -> dict[str, str]:
    return {
        "category": category,
        "slug": slug,
        "name": name,
        "url": url,
        "logo": f"https://icons.llamao.fi/icons/protocols/{slug}",
    }


class SwapProviderOverrideTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.support = Path(self.tmp.name) / "support"
        self.support.mkdir()
        self.alpha_logo = webp_bytes((10, 20, 30, 255))
        self.beta_logo = webp_bytes((40, 50, 60, 255))
        self.relay_logo = webp_bytes((70, 21, 200, 255), size=(12, 12))

    def set_aliases(self, aliases: list[dict[str, str]]) -> None:
        write_json(
            self.support / "swap-provider-aliases.json",
            {"schemaVersion": 1, "aliases": aliases},
        )

    def add_override(
        self,
        provider_id: str,
        name: str,
        url: str,
        source_url: str,
        image: bytes | None,
    ) -> None:
        path = self.support / "swap-provider-overrides.json"
        if path.is_file():
            document = load_json(path)
        else:
            document = {"schemaVersion": 1, "overrides": []}
        document["overrides"].append(
            {"id": provider_id, "name": name, "url": url, "sourceURL": source_url}
        )
        write_json(path, document)
        if image is None:
            return
        master = self.support / "swap-provider-overrides" / provider_id / "logo.webp"
        master.parent.mkdir(parents=True, exist_ok=True)
        master.write_bytes(image)

    def seed_logo(self, provider_id: str, image: bytes) -> None:
        path = self.support / "swap-providers" / provider_id / "logo.webp"
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(image)

    def write_protocols(self, rows: list[dict[str, str]]) -> Path:
        path = Path(self.tmp.name) / "protocols.json"
        write_json(path, rows)
        return path

    def run_sync(self, protocols: Path) -> str:
        stdout = io.StringIO()
        with contextlib.redirect_stdout(stdout):
            sync_swap_providers(
                protocols, support_dir=self.support, asset_base_uri=ASSET_BASE
            )
        return stdout.getvalue()

    def prepare_alpha_and_relay(self) -> Path:
        self.set_aliases(
            [{"id": "alpha-alias", "targetId": "alpha", "name": "Alpha Alias"}]
        )
        self.seed_logo("alpha", self.alpha_logo)
        self.add_override(
            "relay",
            "Relay",
            "https://relay.link",
            "https://example.com/relay.svg",
            self.relay_logo,
        )
        return self.write_protocols(
            [protocol("alpha", "Alpha", "https://alpha.example")]
        )

    def catalog(self) -> dict:
        return load_json(self.support / "swap-providers.json")

    def provider(self, provider_id: str) -> dict:
        matches = [
            row for row in self.catalog()["providers"] if row["id"] == provider_id
        ]
        self.assertEqual(len(matches), 1)
        return matches[0]

    def test_a_override_written_to_catalog_and_logo_bytes(self) -> None:
        stdout = self.run_sync(self.prepare_alpha_and_relay())
        relay = self.provider("relay")
        self.assertEqual(relay["name"], "Relay")
        self.assertEqual(relay["url"], "https://relay.link")
        self.assertEqual(
            relay["logoURI"],
            f"{ASSET_BASE}/swap-providers/relay/logo.webp",
        )
        output = (self.support / "swap-providers" / "relay" / "logo.webp").read_bytes()
        master = (
            self.support / "swap-provider-overrides" / "relay" / "logo.webp"
        ).read_bytes()
        self.assertEqual(output, master)
        self.assertEqual(self.relay_logo, output)
        alias = self.provider("alpha-alias")
        self.assertEqual(alias["url"], "https://alpha.example")
        alias_logo = (
            self.support / "swap-providers" / "alpha-alias" / "logo.webp"
        ).read_bytes()
        self.assertEqual(alias_logo, self.alpha_logo)
        self.assertIn("and 1 overrides", stdout)
        validate_swap_provider_output(self.support, ASSET_BASE)

    def test_b_unknown_directory_removed_override_kept(self) -> None:
        stale = self.support / "swap-providers" / "not-in-any-source"
        stale.mkdir(parents=True)
        (stale / "logo.webp").write_bytes(self.beta_logo)
        self.run_sync(self.prepare_alpha_and_relay())
        self.assertFalse(stale.exists())
        self.assertTrue(
            (self.support / "swap-providers" / "relay" / "logo.webp").is_file()
        )
        master = self.support / "swap-provider-overrides" / "relay" / "logo.webp"
        self.assertEqual(master.read_bytes(), self.relay_logo)
        names = sorted(
            path.name for path in (self.support / "swap-providers").iterdir()
        )
        self.assertEqual(names, ["alpha", "alpha-alias", "relay"])

    def test_c_second_sync_is_idempotent(self) -> None:
        protocols = self.prepare_alpha_and_relay()
        self.run_sync(protocols)
        catalog_bytes = (self.support / "swap-providers.json").read_bytes()
        logos = {
            path.parent.name: path.read_bytes()
            for path in (self.support / "swap-providers").glob("*/logo.webp")
        }
        self.run_sync(protocols)
        self.assertEqual(
            (self.support / "swap-providers.json").read_bytes(), catalog_bytes
        )
        logos_again = {
            path.parent.name: path.read_bytes()
            for path in (self.support / "swap-providers").glob("*/logo.webp")
        }
        self.assertEqual(logos_again, logos)
        self.assertEqual(logos["relay"], self.relay_logo)

    def test_d_override_alias_conflict_raises(self) -> None:
        self.set_aliases([{"id": "relay", "targetId": "alpha", "name": "Relay"}])
        self.seed_logo("alpha", self.alpha_logo)
        self.add_override(
            "relay",
            "Relay",
            "https://relay.link",
            "https://example.com/relay.svg",
            self.relay_logo,
        )
        protocols = self.write_protocols(
            [protocol("alpha", "Alpha", "https://alpha.example")]
        )
        with mock.patch(
            "sync_swap_providers.download_provider_logo"
        ) as download, mock.patch("sync_swap_providers.http_get") as http_get:
            with self.assertRaises(ValueError) as caught:
                self.run_sync(protocols)
            download.assert_not_called()
            http_get.assert_not_called()
        self.assertIn("conflicts with alias", str(caught.exception))
        self.assertFalse((self.support / "swap-providers.json").exists())

    def test_e_override_beats_defillama_without_download(self) -> None:
        self.set_aliases(
            [{"id": "alpha-alias", "targetId": "alpha", "name": "Alpha Alias"}]
        )
        self.seed_logo("alpha", self.alpha_logo)
        self.add_override(
            "relay",
            "Relay",
            "https://relay.link",
            "https://example.com/relay.svg",
            self.relay_logo,
        )
        protocols = self.write_protocols(
            [
                protocol("alpha", "Alpha", "https://alpha.example"),
                protocol(
                    "relay",
                    "DefiLlama Relay",
                    "https://defillama.example/relay",
                    category="Bridge",
                ),
            ]
        )
        with mock.patch(
            "sync_swap_providers.download_provider_logo"
        ) as download, mock.patch(
            "sync_swap_providers.http_get"
        ) as http_get, mock.patch("sync_support.http_get") as support_http:
            stdout = self.run_sync(protocols)
            download.assert_not_called()
            http_get.assert_not_called()
            support_http.assert_not_called()
        self.assertIn("skipping download for: relay", stdout)
        relay = self.provider("relay")
        self.assertEqual(relay["name"], "Relay")
        self.assertEqual(relay["url"], "https://relay.link")
        output = (self.support / "swap-providers" / "relay" / "logo.webp").read_bytes()
        self.assertEqual(output, self.relay_logo)
        self.assertEqual(self.provider("alpha")["name"], "Alpha")
        self.assertEqual(self.provider("alpha-alias")["url"], "https://alpha.example")

    def test_f_invalid_override_inputs_raise(self) -> None:
        valid = {
            "id": "relay",
            "name": "Relay",
            "url": "https://relay.link",
            "sourceURL": "https://example.com/relay.svg",
        }
        cases = {
            "illegal id": {**valid, "id": "Relay"},
            "id with space": {**valid, "id": "bad id"},
            "http url": {**valid, "url": "http://relay.link"},
            "url without host": {**valid, "url": "https://"},
            "http sourceURL": {**valid, "sourceURL": "http://example.com/relay.svg"},
            "extra field": {**valid, "note": "nope"},
            "blank name": {**valid, "name": "   "},
        }
        for label, row in cases.items():
            with self.subTest(label):
                write_json(
                    self.support / "swap-provider-overrides.json",
                    {"schemaVersion": 1, "overrides": [row]},
                )
                with self.assertRaises(ValueError):
                    load_overrides(self.support / "swap-provider-overrides.json")

        with self.subTest("missing master"):
            write_json(
                self.support / "swap-provider-overrides.json",
                {"schemaVersion": 1, "overrides": [valid]},
            )
            with self.assertRaises(ValueError) as caught:
                load_overrides(self.support / "swap-provider-overrides.json")
            self.assertIn("missing override logo", str(caught.exception))

        with self.subTest("master is not webp"):
            master = self.support / "swap-provider-overrides" / "relay" / "logo.webp"
            master.parent.mkdir(parents=True, exist_ok=True)
            master.write_bytes(png_bytes())
            with self.assertRaises(ValueError) as caught:
                load_overrides(self.support / "swap-provider-overrides.json")
            self.assertIn("WebP", str(caught.exception))

        with self.subTest("duplicate id"):
            master.write_bytes(self.relay_logo)
            write_json(
                self.support / "swap-provider-overrides.json",
                {"schemaVersion": 1, "overrides": [valid, dict(valid)]},
            )
            with self.assertRaises(ValueError) as caught:
                load_overrides(self.support / "swap-provider-overrides.json")
            self.assertIn("duplicate", str(caught.exception))

        with self.subTest("bad schema"):
            write_json(
                self.support / "swap-provider-overrides.json",
                {"schemaVersion": 2, "overrides": []},
            )
            with self.assertRaises(ValueError):
                load_overrides(self.support / "swap-provider-overrides.json")

    def test_g_validate_detects_drift_and_missing_catalog_entry(self) -> None:
        self.run_sync(self.prepare_alpha_and_relay())
        catalog_path = self.support / "swap-providers.json"
        logo = self.support / "swap-providers" / "relay" / "logo.webp"
        master = self.support / "swap-provider-overrides" / "relay" / "logo.webp"

        with self.subTest("logo drifted"):
            logo.write_bytes(webp_bytes((9, 9, 9, 255), size=(10, 10)))
            with self.assertRaises(ValueError) as caught:
                validate_swap_provider_output(self.support, ASSET_BASE)
            self.assertIn("does not match override master", str(caught.exception))

        logo.write_bytes(master.read_bytes())
        document = load_json(catalog_path)
        with self.subTest("name and url drifted"):
            for row in document["providers"]:
                if row["id"] == "relay":
                    row["name"] = "Other"
                    row["url"] = "https://other.example"
            write_json(catalog_path, document)
            with self.assertRaises(ValueError) as caught:
                validate_swap_provider_output(self.support, ASSET_BASE)
            self.assertIn("does not match catalog", str(caught.exception))

        with self.subTest("catalog missing override"):
            document = load_json(catalog_path)
            document["providers"] = [
                row for row in document["providers"] if row["id"] != "relay"
            ]
            write_json(catalog_path, document)
            with self.assertRaises(ValueError) as caught:
                validate_swap_provider_output(self.support, ASSET_BASE)
            self.assertIn("missing override provider", str(caught.exception))

    def test_h_missing_overrides_file_matches_previous_behavior(self) -> None:
        self.assertEqual(
            load_overrides(self.support / "swap-provider-overrides.json"), []
        )
        self.set_aliases(
            [{"id": "alpha-alias", "targetId": "alpha", "name": "Alpha Alias"}]
        )
        self.seed_logo("alpha", self.alpha_logo)
        self.seed_logo("beta", self.beta_logo)
        stale = self.support / "swap-providers" / "not-in-any-source"
        stale.mkdir(parents=True)
        (stale / "logo.webp").write_bytes(webp_bytes((1, 1, 1, 255)))
        protocols = self.write_protocols(
            [
                protocol("beta", "Beta", "https://beta.example"),
                protocol("alpha", "Alpha", "https://alpha.example"),
            ]
        )
        with mock.patch("sync_swap_providers.http_get") as http_get:
            stdout = self.run_sync(protocols)
            http_get.assert_not_called()
        self.assertIn(
            "Synced 2 DefiLlama Swap providers and 1 aliases and 0 overrides",
            stdout,
        )
        self.assertFalse(stale.exists())
        self.assertFalse((self.support / "swap-provider-overrides.json").exists())
        self.assertFalse((self.support / "swap-provider-overrides").exists())
        ids = [row["id"] for row in self.catalog()["providers"]]
        self.assertEqual(ids, ["alpha", "alpha-alias", "beta"])
        self.assertEqual(self.provider("alpha")["name"], "Alpha")
        self.assertEqual(self.provider("alpha")["url"], "https://alpha.example")
        self.assertEqual(self.provider("beta")["name"], "Beta")
        self.assertEqual(self.provider("alpha-alias")["name"], "Alpha Alias")
        self.assertEqual(self.provider("alpha-alias")["url"], "https://alpha.example")
        self.assertEqual(
            (self.support / "swap-providers" / "alpha" / "logo.webp").read_bytes(),
            self.alpha_logo,
        )
        self.assertEqual(
            (self.support / "swap-providers" / "alpha-alias" / "logo.webp").read_bytes(),
            self.alpha_logo,
        )
        self.assertEqual(
            (self.support / "swap-providers" / "beta" / "logo.webp").read_bytes(),
            self.beta_logo,
        )
        validate_swap_provider_output(self.support, ASSET_BASE)

    def test_alias_target_must_be_defillama_entry(self) -> None:
        self.set_aliases([{"id": "relay-alias", "targetId": "relay", "name": "Relay"}])
        self.seed_logo("alpha", self.alpha_logo)
        self.add_override(
            "relay",
            "Relay",
            "https://relay.link",
            "https://example.com/relay.svg",
            self.relay_logo,
        )
        protocols = self.write_protocols(
            [protocol("alpha", "Alpha", "https://alpha.example")]
        )
        with self.assertRaises(ValueError) as caught:
            self.run_sync(protocols)
        self.assertIn("alias target does not exist", str(caught.exception))


if __name__ == "__main__":
    unittest.main()
