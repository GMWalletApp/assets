// @vitest-environment jsdom

import { afterEach, describe, expect, it } from "vitest";
import i18n from "../../src/lib/i18n";
import enUS from "../../src/locales/en-US.json";
import zhCN from "../../src/locales/zh-CN.json";

function leafKeys(value: unknown, prefix = ""): string[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return [prefix];
  }

  return Object.entries(value).flatMap(([key, child]) =>
    leafKeys(child, prefix ? `${prefix}.${key}` : key),
  );
}

function leafValues(value: unknown): unknown[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return [value];
  }

  return Object.values(value).flatMap(leafValues);
}

describe("site translations", () => {
  const initialLanguage = i18n.resolvedLanguage ?? "en-US";

  afterEach(async () => {
    await i18n.changeLanguage(initialLanguage);
  });

  it("provides complete English and Simplified Chinese resources", () => {
    expect(enUS.nav.icons).toBe("Icons");
    expect(enUS.actions.switchLanguage).toBe("Switch to Chinese");
    expect(zhCN.nav.icons).toBe("图标");
    expect(zhCN.actions.switchLanguage).toBe("切换为英文");

    expect(leafValues(enUS).every((value) => typeof value === "string" && value.length > 0)).toBe(
      true,
    );
    expect(leafValues(zhCN).every((value) => typeof value === "string" && value.length > 0)).toBe(
      true,
    );
  });

  it("keeps both locale structures in sync", () => {
    expect(leafKeys(zhCN).sort()).toEqual(leafKeys(enUS).sort());
  });

  it.each([
    ["en-US", "en-US"],
    ["zh-CN", "zh-CN"],
  ] as const)("updates document.lang when changing to %s", async (language, expectedLang) => {
    await i18n.changeLanguage(language);

    expect(i18n.resolvedLanguage).toBe(language);
    expect(document.documentElement.lang).toBe(expectedLang);
  });
});
