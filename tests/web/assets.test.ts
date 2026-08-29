import { afterEach, describe, expect, it, vi } from "vitest";
import {
  type AssetEntry,
  assetCanonicalLogoUrl,
  assetIcon,
  assetKey,
  assetLogoUrl,
  fetchAssetData,
} from "../../src/lib/assets";

afterEach(() => vi.unstubAllGlobals());

describe("asset helpers", () => {
  it("resolves DApp raw URLs through the preferred CDN", () => {
    const dapp: AssetEntry = {
      category: "dapp",
      item: {
        id: "app.uniswap.org",
        name: "app.uniswap.org",
        logoURI:
          "https://raw.githubusercontent.com/GMWalletApp/assets/main/dapps/app.uniswap.org.png",
      },
    };

    expect(assetLogoUrl(dapp)).toBe(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/dapps/app.uniswap.org.png",
    );
    expect(assetCanonicalLogoUrl(dapp)).toBe(
      "https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/dapps/app.uniswap.org.png",
    );
    expect(assetIcon(dapp)).toEqual({ type: "dapp", name: "app.uniswap.org" });
  });

  it("uses token addresses to distinguish otherwise identical assets", () => {
    const token = (address: string): AssetEntry => ({
      category: "token",
      token: {
        address,
        assetId: "USDC",
        chain: "zksync",
        decimals: 6,
        kind: "token",
        name: "USD Coin",
        status: "active",
        symbol: "USDC",
        type: "ERC20",
      },
    });

    expect(assetKey(token("0x1"))).not.toBe(assetKey(token("0x2")));
  });

  it("maps swap providers to the component icon contract", () => {
    const provider: AssetEntry = {
      category: "swap-provider",
      item: {
        id: "1inch",
        name: "1inch",
        logoURI:
          "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/swap-providers/1inch/logo.webp",
        url: "https://1inch.io/",
      },
    };

    expect(assetIcon(provider)).toEqual({ type: "swap-provider", name: "1inch" });
  });

  it("falls back to the next mirror without calling the GitHub API", async () => {
    const fetchMock = vi.fn(async (input: string | URL | Request) => {
      const url = String(input);
      if (url.includes("cdn.jsdmirror.com")) {
        return new Response(null, { status: 503 });
      }
      const payload = url.includes("tokenlist")
        ? { source: "test", tokens: [], updatedAt: "2026-08-24T00:00:00Z" }
        : url.includes("swap-providers")
          ? { providers: [], schemaVersion: 1 }
          : url.includes("dapps")
            ? { dapps: [], schemaVersion: 1 }
            : { exchanges: [], schemaVersion: 1, wallets: [] };
      return new Response(JSON.stringify(payload));
    });
    vi.stubGlobal("fetch", fetchMock);

    const data = await fetchAssetData(new AbortController().signal);
    const requestedUrls = fetchMock.mock.calls.map(([input]) => String(input));

    expect(data.tokens.source).toBe("test");
    expect(data.dapps.dapps).toEqual([]);
    expect(data.swapProviders.providers).toEqual([]);
    expect(requestedUrls.some((url) => url.includes("cdn.jsdelivr.net"))).toBe(true);
    expect(requestedUrls.every((url) => !url.includes("api.github.com"))).toBe(true);
  });
});
