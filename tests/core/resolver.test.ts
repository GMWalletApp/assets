import { afterEach, describe, expect, it, vi } from "vitest";
import { resetCatalogCache } from "../../registry/default/crypto-identity/lib/catalog";
import { resolveIconUrls } from "../../registry/default/crypto-identity/lib/resolve-icon-urls";

describe("resolveIconUrls", () => {
  afterEach(() => {
    resetCatalogCache();
    vi.unstubAllGlobals();
  });

  it.each([
    [{ type: "network", name: "ethereum" }, "/blockchains/ethereum/info/logo.png"],
    [{ type: "exchange", name: "Binance" }, "/support/exchanges/binance/logo.svg"],
    [{ type: "wallet", name: "MetaMask" }, "/support/wallets/metamask/logo.svg"],
    [{ type: "dapp", name: "app.uniswap.org" }, "/dapps/app.uniswap.org.png"],
  ] as const)("resolves $type icons", async (query, path) => {
    const urls = await resolveIconUrls(query);
    expect(urls).toHaveLength(5);
    expect(urls.every((url) => url.endsWith(path))).toBe(true);
  });

  it("resolves a token contract on its network", async () => {
    const urls = await resolveIconUrls({
      type: "token",
      name: "USDT",
      network: "tron",
      contractAddress: "TXYZ",
    });
    expect(urls[0]).toContain("/blockchains/tron/assets/TXYZ/logo.png");
  });

  it("does not load the catalog unless requested", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const urls = await resolveIconUrls({ type: "token", name: "USDT", network: "ethereum" });
    expect(urls).toEqual([]);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("returns an empty list for blank names", async () => {
    await expect(resolveIconUrls({ type: "wallet", name: "  " })).resolves.toEqual([]);
  });

  it("accepts dapp keys with or without the png extension", async () => {
    const withExtension = await resolveIconUrls({ type: "dapp", name: "www.example.com.png" });
    expect(withExtension[0]?.endsWith("/dapps/www.example.com.png")).toBe(true);
  });
});
