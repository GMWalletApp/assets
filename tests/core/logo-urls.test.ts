import { describe, expect, it } from "vitest";
import {
  canonicalLogoUrl,
  resolveLogoUrls,
} from "../../registry/default/crypto-identity/lib/logo-urls";

describe("resolveLogoUrls", () => {
  it("uses all configured repository mirrors in order", () => {
    const original =
      "https://raw.githubusercontent.com/GMWalletApp/assets/main/support/wallets/rainbow/logo.png";

    const urls = resolveLogoUrls(original, "https://mirror.example/assets/");

    expect(urls[0]).toBe("https://mirror.example/assets/support/wallets/rainbow/logo.png");
    expect(urls).toContain(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/wallets/rainbow/logo.png",
    );
    expect(urls.at(-1)).toBe(original);
  });

  it("maps Trust Wallet paths to the configured assets repository", () => {
    const source = "https://assets-cdn.trustwallet.com/blockchains/ethereum/info/logo.png";
    const urls = resolveLogoUrls(source);

    expect(urls[0]).toBe(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/blockchains/ethereum/info/logo.png",
    );
    expect(urls).not.toContain(source);
  });

  it("maps generated Trust Wallet paths to the configured assets repository", () => {
    const replaced =
      "https://cdn.jsdelivr.net/gh/trustwallet/assets@master/blockchains/base/assets/0x1/logo.png";
    const urls = resolveLogoUrls(replaced, "https://mirror.example/assets/");

    expect(urls[0]).toBe("https://mirror.example/assets/blockchains/base/assets/0x1/logo.png");
    expect(urls).not.toContain(replaced);
  });

  it("maps other supported repository URLs to the configured assets repository", () => {
    expect(resolveLogoUrls("https://cdn.jsdelivr.net/gh/example/assets@main/logo.svg")[0]).toBe(
      "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/logo.svg",
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
      "https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/blockchains/ethereum/info/logo.png",
    );
  });

  it("leaves unknown and invalid URLs unchanged", () => {
    expect(resolveLogoUrls("https://example.com/logo.png")).toEqual([
      "https://example.com/logo.png",
    ]);
    expect(resolveLogoUrls("not-a-url")).toEqual(["not-a-url"]);
  });
});
