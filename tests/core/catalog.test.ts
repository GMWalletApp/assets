import { afterEach, describe, expect, it, vi } from "vitest";
import { resetCatalogCache } from "../../registry/default/crypto-identity/lib/catalog";
import { resolveIconUrls } from "../../registry/default/crypto-identity/lib/resolve-icon-urls";

describe("compressed catalogs", () => {
  afterEach(() => {
    resetCatalogCache();
    vi.unstubAllGlobals();
  });

  it("resolves tokens from the complete token catalog", async () => {
    const fetchMock = mockCatalogFetch({
      "extensions/jsonrpc/data/tokenlist.json.zst": {
        tokens: [
          {
            chain: "ethereum",
            symbol: "USDT",
            address: "0x1",
            logoURI:
              "https://cdn.jsdelivr.net/gh/trustwallet/assets@master/blockchains/ethereum/assets/0x1/logo.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "token",
      name: "USDT",
      network: "ethereum",
      contractAddress: "0x1",
    });

    expect(urls).toEqual([
      "https://cdn.jsdmirror.com/gh/trustwallet/assets@master/blockchains/ethereum/assets/0x1/logo.png",
      "https://cdn.jsdelivr.net/gh/trustwallet/assets@master/blockchains/ethereum/assets/0x1/logo.png",
    ]);
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("prefers the catalog format when resolving wallet display names", async () => {
    mockCatalogFetch({
      "support/support.json.zst": {
        exchanges: [],
        wallets: [
          {
            id: "rainbow",
            name: "Rainbow App",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "wallet",
      name: "Rainbow App",
    });
    expect(urls[0]).toBe(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/wallets/rainbow/logo.png",
    );
    expect(urls.every((url) => url.endsWith("/support/wallets/rainbow/logo.png"))).toBe(true);
  });

  it("preserves exchange catalog file formats", async () => {
    mockCatalogFetch({
      "support/support.json.zst": {
        exchanges: [
          {
            id: "binance",
            name: "Binance",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/exchanges/binance/logo.svg",
          },
        ],
        wallets: [],
      },
    });

    const urls = await resolveIconUrls({ type: "exchange", name: "Binance" });

    expect(urls[0]).toBe(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/exchanges/binance/logo.svg",
    );
    expect(urls.every((url) => url.endsWith("/support/exchanges/binance/logo.svg"))).toBe(true);
  });

  it("resolves DApp catalog IDs to their domain-based logo paths", async () => {
    mockCatalogFetch({
      "support/dapps.json.zst": {
        dapps: [
          {
            id: "app-uniswap-org",
            name: "app.uniswap.org",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/dapps/app.uniswap.org.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "dapp",
      name: "app-uniswap-org",
    });

    expect(urls).toContain(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/dapps/app.uniswap.org.png",
    );
  });

  it("resolves swap provider display names from the provider catalog", async () => {
    mockCatalogFetch({
      "support/swap-providers.json.zst": {
        providers: [
          {
            id: "1inch",
            name: "1inch",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/swap-providers/1inch/logo.webp",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "swap-provider",
      name: "1inch",
    });

    expect(urls).toContain(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/swap-providers/1inch/logo.webp",
    );
  });

  it("keeps the preferred mirror first for catalog-resolved support icons", async () => {
    mockCatalogFetch({
      "support/support.json.zst": {
        exchanges: [],
        wallets: [
          {
            id: "rainbow",
            name: "Rainbow App",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls(
      { type: "wallet", name: "Rainbow App" },
      "https://mirror.example/assets",
    );

    expect(urls[0]).toBe("https://mirror.example/assets/support/wallets/rainbow/logo.png");
    expect(urls.every((url) => url.endsWith("/support/wallets/rainbow/logo.png"))).toBe(true);
  });

  it("prefers the native token for a symbol lookup", async () => {
    mockCatalogFetch({
      "extensions/jsonrpc/data/tokenlist.json.zst": {
        tokens: [
          {
            chain: "ethereum",
            kind: "token",
            symbol: "ETH",
            logoURI: "https://example.com/wrapped.png",
          },
          {
            chain: "ethereum",
            kind: "native",
            symbol: "ETH",
            logoURI: "https://example.com/native.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "token",
      name: "ETH",
      network: "ethereum",
    });

    expect(urls).toEqual(["https://example.com/native.png"]);
  });

  it("resolves network icons from native token catalog entries", async () => {
    const fetchMock = mockCatalogFetch({
      "extensions/jsonrpc/data/tokenlist.json.zst": {
        tokens: [
          {
            chain: "ethereum",
            kind: "native",
            symbol: "ETH",
            logoURI: "https://example.com/ethereum.png",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({ type: "network", name: "ethereum" });

    expect(urls).toEqual(["https://example.com/ethereum.png"]);
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("deduplicates concurrent catalog requests", async () => {
    const fetchMock = mockCatalogFetch({
      "support/support.json.zst": { exchanges: [], wallets: [] },
    });
    await Promise.all([
      resolveIconUrls({ type: "wallet", name: "unknown" }),
      resolveIconUrls({ type: "wallet", name: "unknown" }),
    ]);
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("retries after every mirror fails", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: false });
    vi.stubGlobal("fetch", fetchMock);
    const query = { type: "wallet", name: "unknown" } as const;

    await resolveIconUrls(query);
    await resolveIconUrls(query);

    expect(fetchMock).toHaveBeenCalledTimes(10);
  });
});

function mockCatalogFetch(payloads: Record<string, unknown>) {
  const fetchMock = vi.fn(async (input: string | URL | Request) => {
    const url = String(input);
    const entry = Object.entries(payloads).find(([path]) => url.endsWith(path));
    if (!entry) {
      return { ok: false };
    }
    const bytes = new TextEncoder().encode(JSON.stringify(entry[1]));
    return {
      ok: true,
      arrayBuffer: async () => bytes.buffer,
    };
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}
