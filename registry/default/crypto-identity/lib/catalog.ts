import { decompress } from "fzstd";
import { CATALOG_PATHS, orderedCdnBaseUrls } from "./constants";
import {
  normalize,
  normalizeDapp,
  normalizeNetworkSlug,
  normalizeSupportSlug,
  uniqueUrls,
} from "./normalize";
import type { TokenAssetQuery } from "./types";

interface CatalogAsset {
  address?: string | null;
  chain?: string;
  id?: string;
  kind?: "native" | "token";
  logoURI?: string;
  name?: string;
  symbol?: string;
}

interface TokenCatalog {
  tokens?: CatalogAsset[];
}

interface SupportCatalog {
  exchanges?: CatalogAsset[];
  wallets?: CatalogAsset[];
}

interface DappCatalog {
  dapps?: CatalogAsset[];
}

const catalogRequests = new Map<string, Promise<unknown>>();

export async function resolveTokenCatalogUrls(
  query: TokenAssetQuery,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<TokenCatalog>(CATALOG_PATHS.tokens, preferredBaseUrl);
  const network = normalizeNetworkSlug(query.network);
  const address = normalize(query.contractAddress);
  const symbol = normalize(query.name);
  const tokens = catalog?.tokens ?? [];
  const matches = (token: CatalogAsset) => {
    if (!token.logoURI || normalizeNetworkSlug(token.chain) !== network) {
      return false;
    }
    return address ? normalize(token.address) === address : normalize(token.symbol) === symbol;
  };
  const match = address
    ? tokens.find(matches)
    : (tokens.find((token) => token.kind === "native" && matches(token)) ?? tokens.find(matches));
  return match?.logoURI ? mirrorLogoUrls(match.logoURI, preferredBaseUrl) : [];
}

export async function resolveSupportCatalogUrls(
  type: "exchange" | "wallet",
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const support = await loadCatalog<SupportCatalog>(CATALOG_PATHS.support, preferredBaseUrl);
  const target = normalizeSupportSlug(name);
  const items = type === "exchange" ? support?.exchanges : support?.wallets;
  const match = items?.find((item) => {
    return [item.id, item.name].some((value) => normalizeSupportSlug(value) === target);
  });
  return match?.logoURI ? mirrorLogoUrls(match.logoURI, preferredBaseUrl) : [];
}

export async function resolveDappCatalogUrls(
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<DappCatalog>(CATALOG_PATHS.dapps, preferredBaseUrl);
  const target = normalizeDapp(name);
  const match = catalog?.dapps?.find((item) => {
    return [item.id, item.name].some((value) => normalizeDapp(value ?? "") === target);
  });
  return match?.logoURI ? mirrorLogoUrls(match.logoURI, preferredBaseUrl) : [];
}

async function loadCatalog<T>(path: string, preferredBaseUrl?: string): Promise<T | null> {
  const key = `${preferredBaseUrl ?? "default"}:${path}`;
  let request = catalogRequests.get(key);
  if (!request) {
    request = fetchCatalog<T>(path, preferredBaseUrl);
    catalogRequests.set(key, request);
  }
  const result = (await request) as T | null;
  if (result === null) {
    catalogRequests.delete(key);
  }
  return result;
}

async function fetchCatalog<T>(path: string, preferredBaseUrl?: string): Promise<T | null> {
  for (const baseUrl of orderedCdnBaseUrls(preferredBaseUrl)) {
    try {
      const response = await fetch(`${baseUrl}/${path}`, {
        signal: AbortSignal.timeout(5_000),
      });
      if (!response.ok) {
        continue;
      }
      const compressed = new Uint8Array(await response.arrayBuffer());
      return JSON.parse(new TextDecoder().decode(decompress(compressed))) as T;
    } catch {
      // Try the next mirror.
    }
  }
  return null;
}

function mirrorLogoUrls(logoUrl: string, preferredBaseUrl?: string): string[] {
  const urls = [logoUrl];
  try {
    const parsed = new URL(logoUrl);
    if (parsed.hostname === "assets-cdn.trustwallet.com") {
      urls.unshift(`https://cdn.jsdmirror.com/gh/trustwallet/assets@master${parsed.pathname}`);
    } else if (parsed.hostname === "cdn.jsdelivr.net") {
      urls.unshift(`https://cdn.jsdmirror.com${parsed.pathname}`);
    } else if (parsed.hostname === "raw.githubusercontent.com") {
      const [owner, repository, branch, ...path] = parsed.pathname.split("/").filter(Boolean);
      if (owner && repository && branch && path.length > 0) {
        urls.unshift(
          `https://cdn.jsdmirror.com/gh/${owner}/${repository}@${branch}/${path.join("/")}`,
        );
        if (preferredBaseUrl && owner === "GMWalletApp" && repository === "assets") {
          urls.unshift(`${preferredBaseUrl.replace(/\/$/, "")}/${path.join("/")}`);
        }
      }
    }
  } catch {
    // Keep the original URL when it is not parseable.
  }
  return uniqueUrls(urls);
}

export function resetCatalogCache(): void {
  catalogRequests.clear();
}
