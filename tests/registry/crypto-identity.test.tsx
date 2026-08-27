// @vitest-environment jsdom

import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { FastAverageColor } from "fast-average-color";
import { afterEach, describe, expect, it, vi } from "vitest";
import { CryptoIdentity } from "../../registry/default/crypto-identity";

const core = vi.hoisted(() => ({
  resolveIconUrls: vi.fn(),
}));

vi.mock("../../registry/default/crypto-identity/lib/resolve-icon-urls", () => core);

describe("CryptoIdentity", () => {
  afterEach(() => {
    cleanup();
    core.resolveIconUrls.mockReset();
    vi.restoreAllMocks();
  });

  it.each(["avatar", "label", "badge"] as const)("renders the %s variant", async (variant) => {
    core.resolveIconUrls.mockResolvedValue(["https://example.com/icon.png"]);
    const props =
      variant === "avatar"
        ? { variant, icon: { type: "network" as const, name: "ethereum" } }
        : {
            variant,
            icon: { type: "network" as const, name: "ethereum" },
            children: "Ethereum",
          };
    const { container } = render(<CryptoIdentity {...props} />);
    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    expect(container.querySelector("img")?.getAttribute("alt")).toBe(
      variant === "avatar" ? "ethereum" : "",
    );
    if (variant !== "avatar") {
      expect(screen.getByText("Ethereum")).toBeTruthy();
    }
  });

  it("loads the token catalog only after direct mirrors fail", async () => {
    core.resolveIconUrls
      .mockResolvedValueOnce(["one.png", "two.png"])
      .mockResolvedValueOnce(["one.png", "two.png", "catalog.png"]);
    const { container } = render(
      <CryptoIdentity icon={{ type: "token", name: "UNKNOWN", network: "ethereum" }} />,
    );

    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    const first = container.querySelector("img");
    expect(first).not.toBeNull();
    fireEvent.error(first as HTMLImageElement);
    expect(container.querySelector("img")?.getAttribute("src")).toBe("two.png");
    await act(async () => fireEvent.error(container.querySelector("img") as HTMLImageElement));
    expect(core.resolveIconUrls).toHaveBeenLastCalledWith(
      expect.objectContaining({ includeCatalog: true }),
    );
    await waitFor(() =>
      expect(container.querySelector("img")?.getAttribute("src")).toBe("catalog.png"),
    );
  });

  it("renders an independent corner icon", async () => {
    core.resolveIconUrls
      .mockResolvedValueOnce(["token.png"])
      .mockResolvedValueOnce(["network.png"]);
    const { container } = render(
      <CryptoIdentity
        icon={{ type: "token", name: "USDT", network: "ethereum" }}
        cornerIcon={{ type: "network", name: "ethereum" }}
      />,
    );
    await waitFor(() => expect(container.querySelectorAll("img")).toHaveLength(2));
    const images = [...container.querySelectorAll("img")];
    expect(images.map((image) => image.getAttribute("src"))).toEqual(["token.png", "network.png"]);
    expect(container.querySelector('[data-slot="crypto-identity-corner-skeleton"]')).not.toBeNull();
    fireEvent.load(images[1] as HTMLImageElement);
    expect(container.querySelector('[data-slot="crypto-identity-corner-skeleton"]')).toBeNull();
  });

  it("uses the image dominant color as an adaptive background", async () => {
    core.resolveIconUrls.mockResolvedValue(["https://cdn.jsdmirror.com/icon.png"]);
    vi.spyOn(FastAverageColor.prototype, "getColor").mockReturnValue({
      hex: "#627eea",
      hexa: "#627eeaff",
      isDark: false,
      isLight: true,
      rgb: "rgb(98,126,234)",
      rgba: "rgba(98,126,234,1)",
      value: [98, 126, 234, 255],
    });
    const { container } = render(<CryptoIdentity icon={{ type: "network", name: "ethereum" }} />);

    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    fireEvent.load(container.querySelector("img") as HTMLImageElement);

    const style = container
      .querySelector('[data-slot="crypto-identity-image"]')
      ?.getAttribute("style");
    expect(style).toContain("--crypto-identity-color: #627eea");
    expect(style).toContain("--crypto-identity-light-surface: rgb(227 232 251)");
    expect(style).toContain("--crypto-identity-dark-surface: rgb(34 43 78)");
  });

  it("darkens pale icons in light mode to preserve contrast", async () => {
    core.resolveIconUrls.mockResolvedValue(["https://cdn.jsdmirror.com/pale-icon.png"]);
    vi.spyOn(FastAverageColor.prototype, "getColor").mockReturnValue({
      hex: "#e8edf5",
      hexa: "#e8edf5ff",
      isDark: false,
      isLight: true,
      rgb: "rgb(232,237,245)",
      rgba: "rgba(232,237,245,1)",
      value: [232, 237, 245, 255],
    });
    const { container } = render(<CryptoIdentity icon={{ type: "network", name: "pale" }} />);

    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    fireEvent.load(container.querySelector("img") as HTMLImageElement);

    expect(
      container.querySelector('[data-slot="crypto-identity-image"]')?.getAttribute("style"),
    ).toContain("--crypto-identity-light-surface: rgb(102 104 108)");
  });

  it("shows a skeleton until the image loads", async () => {
    core.resolveIconUrls.mockResolvedValue(["https://example.com/icon.png"]);
    const { container } = render(<CryptoIdentity icon={{ type: "network", name: "ethereum" }} />);
    const avatar = container.querySelector('[data-slot="crypto-identity-image"]');

    expect(container.querySelector('[data-slot="crypto-identity-skeleton"]')).not.toBeNull();
    expect(avatar?.getAttribute("aria-busy")).toBe("true");
    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    fireEvent.load(container.querySelector("img") as HTMLImageElement);

    expect(container.querySelector('[data-slot="crypto-identity-skeleton"]')).toBeNull();
    expect(avatar?.hasAttribute("aria-busy")).toBe(false);
  });

  it("stops the skeleton when no image can be resolved", async () => {
    core.resolveIconUrls.mockResolvedValue([]);
    const { container } = render(
      <CryptoIdentity fallback="ETH" icon={{ type: "network", name: "unknown" }} />,
    );
    const avatar = container.querySelector('[data-slot="crypto-identity-image"]');

    await waitFor(() => expect(avatar?.hasAttribute("aria-busy")).toBe(false));
    expect(container.querySelector('[data-slot="crypto-identity-skeleton"]')).toBeNull();
    expect(screen.getByText("ETH")).toBeTruthy();
  });

  it("falls back to the catalog after the final image error", async () => {
    core.resolveIconUrls.mockResolvedValue(["one.png"]);
    const { container } = render(<CryptoIdentity icon={{ type: "wallet", name: "metamask" }} />);
    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    fireEvent.error(container.querySelector("img") as HTMLImageElement);
    await waitFor(() =>
      expect(core.resolveIconUrls).toHaveBeenLastCalledWith(
        expect.objectContaining({ includeCatalog: true }),
      ),
    );
  });

  it("falls back to the DApp catalog after a direct path fails", async () => {
    core.resolveIconUrls
      .mockResolvedValueOnce(["direct.png"])
      .mockResolvedValueOnce(["direct.png", "catalog.png"]);
    const { container } = render(
      <CryptoIdentity icon={{ type: "dapp", name: "app-uniswap-org" }} />,
    );

    await waitFor(() => expect(container.querySelector("img")).not.toBeNull());
    fireEvent.error(container.querySelector("img") as HTMLImageElement);

    await waitFor(() =>
      expect(core.resolveIconUrls).toHaveBeenLastCalledWith(
        expect.objectContaining({ includeCatalog: true }),
      ),
    );
    await waitFor(() =>
      expect(container.querySelector("img")?.getAttribute("src")).toBe("catalog.png"),
    );
  });
});
