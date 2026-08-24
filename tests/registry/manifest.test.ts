import { readFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";

interface Registry {
  items: Array<{
    name: string;
    type: string;
    dependencies?: string[];
    registryDependencies?: string[];
    files: Array<{ target: string }>;
  }>;
}

describe("Registry manifest", () => {
  it("installs the component and resolver folder from one item", async () => {
    const manifest = JSON.parse(await readFile("registry.json", "utf8")) as Registry;

    expect(manifest.items).toHaveLength(1);
    expect(manifest.items[0]).toMatchObject({
      name: "crypto-identity",
      type: "registry:block",
      dependencies: ["class-variance-authority", "fast-average-color", "fzstd"],
      registryDependencies: ["avatar", "skeleton"],
    });
    expect(manifest.items[0]?.files.map(({ target }) => target)).toEqual([
      "@ui/crypto-identity/index.tsx",
      "@ui/crypto-identity/hooks/use-icon-source.ts",
      "@ui/crypto-identity/hooks/use-image-background.ts",
      "@ui/crypto-identity/lib/types.ts",
      "@ui/crypto-identity/lib/index.ts",
      "@ui/crypto-identity/lib/constants.ts",
      "@ui/crypto-identity/lib/normalize.ts",
      "@ui/crypto-identity/lib/catalog.ts",
      "@ui/crypto-identity/lib/resolve-icon-urls.ts",
    ]);
  });
});
