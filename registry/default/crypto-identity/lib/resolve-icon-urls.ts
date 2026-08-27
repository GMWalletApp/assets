import {
  resolveDappCatalogUrls,
  resolveSupportCatalogUrls,
  resolveTokenCatalogUrls,
} from "./catalog";
import { orderedCdnBaseUrls } from "./constants";
import {
  normalizeDapp,
  normalizeNetworkSlug,
  normalizeSlug,
  normalizeSupportSlug,
  uniqueUrls,
} from "./normalize";
import type { AssetQuery } from "./types";

/** Resolve ordered icon URL candidates for a token, network, exchange, wallet, or dapp. */
export async function resolveIconUrls(
  query: AssetQuery,
  preferredBaseUrl?: string,
): Promise<string[]> {
  if (query.type === "token") {
    const contractAddress = query.contractAddress?.trim();
    const network = normalizeNetworkSlug(query.network);
    const directUrls =
      contractAddress && network
        ? urlsForPath(`blockchains/${network}/assets/${contractAddress}/logo.png`, preferredBaseUrl)
        : [];

    if (!query.includeCatalog) {
      return directUrls;
    }
    return uniqueUrls([...directUrls, ...(await resolveTokenCatalogUrls(query, preferredBaseUrl))]);
  }

  const name = query.type === "dapp" ? normalizeDapp(query.name) : normalizeSlug(query.name);
  if (!name) {
    return [];
  }

  switch (query.type) {
    case "network":
      return urlsForPath(
        `blockchains/${normalizeNetworkSlug(name)}/info/logo.png`,
        preferredBaseUrl,
      );
    case "dapp": {
      const directUrls = urlsForPath(`dapps/${name}.png`, preferredBaseUrl);
      if (!query.includeCatalog) {
        return directUrls;
      }
      return uniqueUrls([
        ...directUrls,
        ...(await resolveDappCatalogUrls(query.name, preferredBaseUrl)),
      ]);
    }
    case "exchange":
    case "wallet": {
      const supportName = normalizeSupportSlug(name);
      const directUrls = urlsForPath(
        `support/${query.type}s/${supportName}/logo.svg`,
        preferredBaseUrl,
      );
      if (!query.includeCatalog) {
        return directUrls;
      }
      return uniqueUrls([
        ...directUrls,
        ...(await resolveSupportCatalogUrls(query.type, query.name, preferredBaseUrl)),
      ]);
    }
  }
}

function urlsForPath(path: string, preferredBaseUrl?: string): string[] {
  return orderedCdnBaseUrls(preferredBaseUrl).map((baseUrl) => `${baseUrl}/${path}`);
}
