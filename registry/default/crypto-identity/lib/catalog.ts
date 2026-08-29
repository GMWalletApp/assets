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
  assetId?: string;
  chain?: string;
  id?: string;
  kind?: "native" | "token";
  logoURI?: string;
  name?: string;
  symbol?: string;
}

type CatalogNameField = "address" | "assetId" | "name" | "symbol";

interface TokenCatalogIndex {
  byAddress: Map<string, CatalogAsset>;
  byName: Map<string, CatalogAsset>;
  nativeByName: Map<string, CatalogAsset>;
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
const namedAssetIndexes = new WeakMap<CatalogAsset[], Map<string, CatalogAsset>>();
const tokenCatalogIndexes = new WeakMap<CatalogAsset[], TokenCatalogIndex>();
const textDecoder = new TextDecoder();
const TOKEN_NAME_FIELDS = ["assetId", "address", "name", "symbol"] as const;

export async function resolveTokenCatalogUrls(
  query: TokenAssetQuery,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<TokenCatalog>(CATALOG_PATHS.tokens, preferredBaseUrl);
  const network = normalizeNetworkSlug(query.network);
  const address = normalize(query.contractAddress);
  const name = normalize(query.name);
  const tokens = catalog?.tokens ?? [];
  const index = getTokenCatalogIndex(tokens);
  const key = tokenLookupKey(network, address || name);
  const match = address
    ? index.byAddress.get(key)
    : (index.nativeByName.get(key) ?? index.byName.get(key));
  return resolveCatalogAssetLogoUrls(match, preferredBaseUrl);
}

export async function resolveNetworkCatalogUrls(
  name: string,
  preferredBaseUrl?: string,
): Promise<string[]> {
  const catalog = await loadCatalog<TokenCatalog>(CATALOG_PATHS.tokens, preferredBaseUrl);
  const network = normalizeNetworkSlug(name);
  const identity = normalize(name);
  const match = catalog?.tokens?.find(
    (token) =>
      token.logoURI &&
      token.kind === "native" &&
      (normalizeNetworkSlug(token.chain) === network ||
        matchesNormalizedName(token, identity, ["assetId", "name", "symbol"])),
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
  if (!assets) {
    return undefined;
  }
  let index = namedAssetIndexes.get(assets);
  if (!index) {
    index = new Map();
    for (const asset of assets) {
      addFirst(index, normalizeName(asset.id), asset);
      addFirst(index, normalizeName(asset.name), asset);
    }
    namedAssetIndexes.set(assets, index);
  }
  return index.get(target);
}

function matchesNormalizedName(
  asset: CatalogAsset,
  target: string,
  fields: readonly CatalogNameField[],
): boolean {
  return fields.some((field) => normalize(asset[field]) === target);
}

function getTokenCatalogIndex(tokens: CatalogAsset[]): TokenCatalogIndex {
  const cached = tokenCatalogIndexes.get(tokens);
  if (cached) {
    return cached;
  }
  const index: TokenCatalogIndex = {
    byAddress: new Map(),
    byName: new Map(),
    nativeByName: new Map(),
  };
  for (const token of tokens) {
    if (!token.logoURI) {
      continue;
    }
    const network = normalizeNetworkSlug(token.chain);
    addFirst(index.byAddress, tokenLookupKey(network, normalize(token.address)), token);
    for (const field of TOKEN_NAME_FIELDS) {
      const key = tokenLookupKey(network, normalize(token[field]));
      addFirst(index.byName, key, token);
      if (token.kind === "native") {
        addFirst(index.nativeByName, key, token);
      }
    }
  }
  tokenCatalogIndexes.set(tokens, index);
  return index;
}

function tokenLookupKey(network: string, name: string): string {
  return network && name ? `${network}\u0000${name}` : "";
}

function addFirst(index: Map<string, CatalogAsset>, key: string, asset: CatalogAsset): void {
  if (key && !index.has(key)) {
    index.set(key, asset);
  }
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
