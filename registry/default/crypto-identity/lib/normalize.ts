import { NETWORK_SLUG_ALIASES, SUPPORT_SLUG_ALIASES } from "./constants";

export function normalize(value?: string | null): string {
  return value?.trim().toLowerCase() ?? "";
}

export function normalizeSlug(value?: string | null): string {
  return normalize(value).replaceAll(" ", "-");
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
  return normalized.endsWith(".png") ? normalized.slice(0, -4) : normalized;
}

export function uniqueUrls(urls: readonly string[]): string[] {
  return [...new Set(urls.filter(Boolean))];
}
