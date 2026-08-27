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
      includeCatalog: true,
    });

    expect(urls).toContain(
      "https://cdn.jsdmirror.com/gh/trustwallet/assets@master/blockchains/ethereum/assets/0x1/logo.png",
    );
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("resolves wallet display names from the support catalog", async () => {
    mockCatalogFetch({
      "support/support.json.zst": {
        exchanges: [],
        wallets: [
          {
            id: "rainbow",
            name: "Rainbow App",
            logoURI:
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.svg",
          },
        ],
      },
    });

    const urls = await resolveIconUrls({
      type: "wallet",
      name: "Rainbow App",
      includeCatalog: true,
    });
    expect(urls[0]).toContain("cdn.jsdmirror.com/gh/GMWalletApp/assets@main");
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
      includeCatalog: true,
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
      includeCatalog: true,
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
              "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.svg",
          },
        ],
      },
    });

    const urls = await resolveIconUrls(
      { type: "wallet", name: "Rainbow App", includeCatalog: true },
      "https://mirror.example/assets",
    );

    expect(urls[0]).toBe("https://mirror.example/assets/support/wallets/rainbow-app/logo.svg");
    expect(urls).toContain("https://mirror.example/assets/support/wallets/rainbow/logo.svg");
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
      includeCatalog: true,
    });

    expect(urls).toEqual(["https://example.com/native.png"]);
  });

  it("does not load a catalog for deterministic network paths", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    await resolveIconUrls({ type: "network", name: "ethereum" });
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("deduplicates concurrent catalog requests", async () => {
    const fetchMock = mockCatalogFetch({
      "support/support.json.zst": { exchanges: [], wallets: [] },
    });
    await Promise.all([
      resolveIconUrls({ type: "wallet", name: "unknown", includeCatalog: true }),
      resolveIconUrls({ type: "wallet", name: "unknown", includeCatalog: true }),
    ]);
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("retries after every mirror fails", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: false });
    vi.stubGlobal("fetch", fetchMock);
    const query = { type: "wallet", name: "unknown", includeCatalog: true } as const;

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
