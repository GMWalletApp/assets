import { decompress } from "fzstd";
import { CATALOG_PATHS, orderedCdnBaseUrls } from "./constants";
import { resolveLogoUrls } from "./logo-urls";
import {
  normalize,
  normalizeDapp,
  normalizeNetworkSlug,
  normalizeSlug,
  normalizeSupportSlug,
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

interface SwapProviderCatalog {
  providers?: CatalogAsset[];
}

const catalogRequests = new Map<string, Promise<unknown>>();
const textDecoder = new TextDecoder();

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
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

export async function resolveNetworkCatalogUrls(
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<TokenCatalog>(CATALOG_PATHS.tokens, preferredBaseUrl);
  const network = normalizeNetworkSlug(name);
  const match = catalog?.tokens?.find(
    (token) =>
      token.logoURI && token.kind === "native" && normalizeNetworkSlug(token.chain) === network,
  );
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

export async function resolveSupportCatalogUrls(
  type: "exchange" | "wallet",
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const support = await loadCatalog<SupportCatalog>(CATALOG_PATHS.support, preferredBaseUrl);
  const items = type === "exchange" ? support?.exchanges : support?.wallets;
  const match = findNamedCatalogAsset(items, name, normalizeSupportSlug);
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

export async function resolveDappCatalogUrls(
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<DappCatalog>(CATALOG_PATHS.dapps, preferredBaseUrl);
  const match = findNamedCatalogAsset(catalog?.dapps, name, normalizeDapp);
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

export async function resolveSwapProviderCatalogUrls(
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<SwapProviderCatalog>(
    CATALOG_PATHS.swapProviders,
    preferredBaseUrl,
  );
  const match = findNamedCatalogAsset(catalog?.providers, name, normalizeSlug);
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

function findNamedCatalogAsset(
  assets: CatalogAsset[] | undefined,
  name: string,
  normalizeName: (value?: string | null) => string,
): CatalogAsset | undefined {
  const target = normalizeName(name);
  return assets?.find(
    (asset) => normalizeName(asset.id) === target || normalizeName(asset.name) === target,
  );
}

function resolveCatalogAssetLogoUrls(
  asset: CatalogAsset | undefined,
  preferredBaseUrl?: string,
): string[] {
  return asset?.logoURI ? resolveLogoUrls(asset.logoURI, preferredBaseUrl) : [];
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
      return JSON.parse(textDecoder.decode(decompress(compressed))) as T;
    } catch {
      // Try the next mirror.
    }
  }
  return null;
}

export function resetCatalogCache(): void {
  catalogRequests.clear();
}
