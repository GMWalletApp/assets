import { describe, expect, it } from "vitest";
import {
  canonicalLogoUrl,
  resolveLogoUrls,
} from "../../registry/default/crypto-identity/lib/logo-urls";

describe("resolveLogoUrls", () => {
  it("uses all repository mirrors in order and preserves the catalog URL", () => {
    const original =
      "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.png";

    const urls = resolveLogoUrls(original, "https://mirror.example/assets/");

    expect(urls[0]).toBe("https://mirror.example/assets/support/wallets/rainbow/logo.png");
    expect(urls).toContain(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/wallets/rainbow/logo.png",
    );
    expect(urls.at(-1)).toBe(original);
  });

  it("rewrites Trust Wallet and jsDelivr URLs through jsdmirror", () => {
    expect(
      resolveLogoUrls("https://assets-cdn.trustwallet.com/blockchains/ethereum/info/logo.png")[0],
    ).toBe(
      "https://cdn.jsdmirror.com/gh/trustwallet/assets@master/blockchains/ethereum/info/logo.png",
    );
    expect(resolveLogoUrls("https://cdn.jsdelivr.net/gh/example/assets@main/logo.svg")[0]).toBe(
      "https://cdn.jsdmirror.com/gh/example/assets@main/logo.svg",
    );
  });

  it("normalizes supported sources to one jsDelivr URL", () => {
    expect(
      canonicalLogoUrl(
        "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/metamask/logo.svg",
      ),
    ).toBe("https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/wallets/metamask/logo.svg");
    expect(
      canonicalLogoUrl("https://assets-cdn.trustwallet.com/blockchains/ethereum/info/logo.png"),
    ).toBe(
      "https://cdn.jsdelivr.net/gh/trustwallet/assets@master/blockchains/ethereum/info/logo.png",
    );
  });

  it("leaves unknown and invalid URLs unchanged", () => {
    expect(resolveLogoUrls("https://example.com/logo.png")).toEqual([
      "https://example.com/logo.png",
    ]);
    expect(resolveLogoUrls("not-a-url")).toEqual(["not-a-url"]);
  });
});
