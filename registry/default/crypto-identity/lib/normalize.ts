import { NETWORK_SLUG_ALIASES, SUPPORT_SLUG_ALIASES } from "./constants";
import type { AssetType } from "./types";

export function normalize(value?: string | null): string {
  return value?.trim().toLowerCase() ?? "";
}

export function normalizeSlug(value?: string | null): string {
  return normalize(value).replace(/[\s_]+/g, "-");
}

export function normalizeNetworkSlug(value?: string | null): string {
  const slug = normalizeSlug(value);
  return NETWORK_SLUG_ALIASES[slug] ?? slug;
}

export function normalizeSupportSlug(value?: string | null): string {
  const slug = normalizeSlug(value);
  return SUPPORT_SLUG_ALIASES[slug] ?? slug;
}

export function normalizeDapp(value?: string | null): string {
  const normalized = normalize(value);
  const name = normalized.endsWith(".png") ? normalized.slice(0, -4) : normalized;
  return name.replace(/[.\s_]+/g, "-");
}

export function normalizeAssetName(type: AssetType, value?: string | null): string {
  switch (type) {
    case "network":
      return normalizeNetworkSlug(value);
    case "dapp":
      return normalizeDapp(value);
    case "exchange":
    case "wallet":
      return normalizeSupportSlug(value);
    case "swap-provider":
      return normalizeSlug(value);
    case "token":
      return normalize(value);
  }
}

export function uniqueUrls(urls: readonly string[]): string[] {
  return [...new Set(urls.filter(Boolean))];
}
