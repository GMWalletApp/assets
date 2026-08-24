import { resolveSupportCatalogUrls, resolveTokenCatalogUrls } from "./catalog";
import { CDN_BASE_URLS } from "./constants";
import {
  normalizeDapp,
  normalizeNetworkSlug,
  normalizeSlug,
  normalizeSupportSlug,
  uniqueUrls,
} from "./normalize";
import type { AssetQuery } from "./types";

/** Resolve ordered icon URL candidates for a token, network, exchange, wallet, or dapp. */
export async function resolveIconUrls(query: AssetQuery): Promise<string[]> {
  if (query.type === "token") {
    const contractAddress = query.contractAddress?.trim();
    const network = normalizeNetworkSlug(query.network);
    const directUrls =
      contractAddress && network
        ? urlsForPath(`blockchains/${network}/assets/${contractAddress}/logo.png`)
        : [];

    if (!query.includeCatalog) {
      return directUrls;
    }
    return uniqueUrls([...directUrls, ...(await resolveTokenCatalogUrls(query))]);
  }

  const name = query.type === "dapp" ? normalizeDapp(query.name) : normalizeSlug(query.name);
  if (!name) {
    return [];
  }

  switch (query.type) {
    case "network":
      return urlsForPath(`blockchains/${normalizeNetworkSlug(name)}/info/logo.png`);
    case "dapp":
      return urlsForPath(`dapps/${name}.png`);
    case "exchange":
    case "wallet": {
      const supportName = normalizeSupportSlug(name);
      const directUrls = urlsForPath(`support/${query.type}s/${supportName}/logo.svg`);
      if (!query.includeCatalog) {
        return directUrls;
      }
      return uniqueUrls([
        ...directUrls,
        ...(await resolveSupportCatalogUrls(query.type, query.name)),
      ]);
    }
  }
}

function urlsForPath(path: string): string[] {
  return CDN_BASE_URLS.map((baseUrl) => `${baseUrl}/${path}`);
}
