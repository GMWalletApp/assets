import { afterEach, describe, expect, it, vi } from "vitest";
import { resetCatalogCache } from "../../registry/default/crypto-identity/lib/catalog";
import { resolveIconUrls } from "../../registry/default/crypto-identity/lib/resolve-icon-urls";

describe("resolveIconUrls", () => {
  afterEach(() => {
    resetCatalogCache();
    vi.unstubAllGlobals();
  });

  it.each(["wallet", "token"] as const)(
    "returns an empty list for blank %s names",
    async (type) => {
      const query =
        type === "token"
          ? ({ type, name: "  ", network: "ethereum" } as const)
          : ({ type, name: "  " } as const);

      await expect(resolveIconUrls(query)).resolves.toEqual([]);
    },
  );

  it("returns an empty list for a blank token network", async () => {
    await expect(resolveIconUrls({ type: "token", name: "USDT", network: "  " })).resolves.toEqual(
      [],
    );
  });
});
