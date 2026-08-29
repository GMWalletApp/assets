import {
  resolveDappCatalogUrls,
  resolveNetworkCatalogUrls,
  resolveSupportCatalogUrls,
  resolveSwapProviderCatalogUrls,
  resolveTokenCatalogUrls,
} from "./catalog";
import { normalizeDapp, normalizeSlug } from "./normalize";
import type { AssetQuery } from "./types";

/** Resolve ordered icon URL candidates for a token, network, exchange, wallet, DApp, or swap provider. */
export async function resolveIconUrls(
  query: AssetQuery,
  preferredBaseUrl?: string,
): Promise<string[]> {
  if (query.type === "token") {
    return resolveTokenCatalogUrls(query, preferredBaseUrl);
  }

  const name = query.type === "dapp" ? normalizeDapp(query.name) : normalizeSlug(query.name);
  if (!name) {
    return [];
  }

  switch (query.type) {
    case "network":
      return resolveNetworkCatalogUrls(query.name, preferredBaseUrl);
    case "dapp":
      return resolveDappCatalogUrls(query.name, preferredBaseUrl);
    case "swap-provider":
      return resolveSwapProviderCatalogUrls(query.name, preferredBaseUrl);
    case "exchange":
    case "wallet":
      return resolveSupportCatalogUrls(query.type, query.name, preferredBaseUrl);
  }
}
