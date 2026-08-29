import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { __unstable__loadDesignSystem } from "@tailwindcss/node";
import { describe, expect, it } from "vitest";
import { canonicalizeSource, splitClassList } from "../../scripts/tailwind-canonicalize";

describe("tailwind canonicalize CLI", () => {
  it("keeps spaces inside arbitrary values", () => {
    expect(splitClassList("before:content-['hello world'] size-[18px]")).toEqual([
      "before:content-['hello world']",
      "size-[18px]",
    ]);
  });

  it("targets static JSX and class helper strings only", () => {
    const source = `
      const label = "size-[18px]";
      const root = cn("size-[18px]", active && "mt-[16px]");
      const variants = cva("p-[8px]", { variants: { size: { lg: "w-[16px] h-[16px]" } } });
      const staticNode = <div className="size-[18px]" />;
      const dynamicNode = <div className={\`size-[\${size}px]\`} />;
    `;
    const replacements = new Map([
      ["size-[18px]", ["size-4.5"]],
      ["mt-[16px]", ["mt-4"]],
      ["p-[8px]", ["p-2"]],
      ["w-[16px] h-[16px]", ["size-4"]],
    ]);

    const result = canonicalizeSource(
      source,
      "fixture.tsx",
      (candidates) => replacements.get(candidates.join(" ")) ?? candidates,
    );

    expect(result.edits).toHaveLength(5);
    expect(result.output).toContain('const label = "size-[18px]"');
    expect(result.output).toContain('cn("size-4.5", active && "mt-4")');
    expect(result.output).toContain('cva("p-2", { variants: { size: { lg: "size-4" } } })');
    expect(result.output).toContain('className="size-4.5"');
    expect(result.output).toContain("className={" + "`size-[" + "$" + "{size}px]`}");
  });

  it("uses the project Tailwind theme for canonical values", async () => {
    const cssPath = resolve("src/styles.css");
    const css = await readFile(cssPath, "utf8");
    const designSystem = await __unstable__loadDesignSystem(css, { base: resolve("src") });
    const result = canonicalizeSource(
      '<span className="size-[18px] w-4 h-4 mt-[16px]" />',
      "fixture.tsx",
      (candidates, options) => designSystem.canonicalizeCandidates(candidates, options),
    );

    expect(result.output).toBe('<span className="mt-4 size-4" />');
  });

  it("stabilizes multi-pass Tailwind replacements", () => {
    const result = canonicalizeSource(
      '<div className="translate-x-[-50%] translate-y-[-50%]" />',
      "fixture.tsx",
      (candidates) => {
        if (candidates.join(" ") === "translate-x-[-50%] translate-y-[-50%]") {
          return ["-translate-[50%]"];
        }
        if (candidates[0] === "-translate-[50%]") return ["translate-[-50%]"];
        return candidates;
      },
    );

    expect(result.output).toBe('<div className="translate-[-50%]" />');
  });
});
