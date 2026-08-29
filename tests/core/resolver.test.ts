import { afterEach, describe, expect, it, vi } from "vitest";
import { resetCatalogCache } from "../../registry/default/crypto-identity/lib/catalog";
import { resolveIconUrls } from "../../registry/default/crypto-identity/lib/resolve-icon-urls";

describe("resolveIconUrls", () => {
  afterEach(() => {
    resetCatalogCache();
    vi.unstubAllGlobals();
  });

  it("returns an empty list for blank names", async () => {
    await expect(resolveIconUrls({ type: "wallet", name: "  " })).resolves.toEqual([]);
  });
});
