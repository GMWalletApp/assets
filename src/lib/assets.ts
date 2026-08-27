import { decompress } from "fzstd";
import type { CryptoIdentityIcon } from "../../registry/default/crypto-identity";
import {
  CATALOG_PATHS,
  orderedCdnBaseUrls,
} from "../../registry/default/crypto-identity/lib/constants";

type AssetCategory = "native" | "token" | "exchange" | "wallet" | "dapp" | "swap-provider";
export type AssetFilter = "all" | AssetCategory;

interface Token {
  address: string;
  assetId: string;
  chain: string;
  decimals: number;
  hot?: boolean;
  kind: "native" | "token";
  logoURI?: string;
  name: string;
  rank?: number;
  status: string;
  symbol: string;
  type: string;
}

interface SupportItem {
  id: string;
  logoURI: string;
  name: string;
  type?: string;
  url?: string;
}

export type AssetEntry =
  | { category: "native" | "token"; token: Token }
  | { category: "exchange" | "wallet" | "dapp" | "swap-provider"; item: SupportItem };

interface TokenList {
  source: string;
  tokens: Token[];
  updatedAt: string;
}

interface SupportList {
  exchanges: SupportItem[];
  schemaVersion: number;
  wallets: SupportItem[];
}

interface DappList {
  dapps: SupportItem[];
  schemaVersion: number;
}

interface SwapProviderList {
  providers: SupportItem[];
  schemaVersion: number;
}

interface AssetData {
  dapps: DappList;
  support: SupportList;
  swapProviders: SwapProviderList;
  tokens: TokenList;
}

const DEFAULT_CDN_BASE_URL = orderedCdnBaseUrls()[0] ?? "";
const DEFAULT_CDN_ORIGIN = new URL(DEFAULT_CDN_BASE_URL).origin;
const textDecoder = new TextDecoder();

export async function fetchAssetData(signal: AbortSignal): Promise<AssetData> {
  const [tokens, support, dapps, swapProviders] = await Promise.all([
    fetchCompressedJsonFromMirrors<TokenList>(CATALOG_PATHS.tokens, signal),
    fetchCompressedJsonFromMirrors<SupportList>(CATALOG_PATHS.support, signal),
    fetchCompressedJsonFromMirrors<DappList>(CATALOG_PATHS.dapps, signal),
    fetchCompressedJsonFromMirrors<SwapProviderList>(CATALOG_PATHS.swapProviders, signal),
  ]);
  return { dapps, support, swapProviders, tokens };
}

export function createAssetEntries(data: AssetData | null): AssetEntry[] {
  if (!data) {
    return [];
  }
  return [
    ...data.tokens.tokens
      .filter((token) => token.logoURI)
      .map((token): AssetEntry => ({ category: token.kind, token })),
    ...data.support.exchanges.map((item): AssetEntry => ({ category: "exchange", item })),
    ...data.support.wallets.map((item): AssetEntry => ({ category: "wallet", item })),
    ...data.dapps.dapps.map((item): AssetEntry => ({ category: "dapp", item })),
    ...data.swapProviders.providers.map(
      (item): AssetEntry => ({ category: "swap-provider", item }),
    ),
  ];
}

export function assetKey(entry: AssetEntry): string {
  return "token" in entry
    ? `${entry.category}:${entry.token.chain}:${entry.token.address || entry.token.assetId}:${entry.token.assetId}`
    : `${entry.category}:${entry.item.id}`;
}

export function assetName(entry: AssetEntry): string {
  return "token" in entry ? entry.token.name : entry.item.name;
}

export function assetSymbol(entry: AssetEntry): string {
  return "token" in entry ? entry.token.symbol || entry.token.name : entry.item.name;
}

export function assetSecondary(entry: AssetEntry): string {
  return "token" in entry ? entry.token.chain : entry.item.id;
}

export function assetBadge(entry: AssetEntry): string {
  if ("token" in entry) {
    return entry.category === "native" ? "network" : "token";
  }
  if (entry.category === "exchange") {
    return entry.item.type ?? "exchange";
  }
  return entry.category;
}

export function assetIcon(entry: AssetEntry): CryptoIdentityIcon {
  if ("item" in entry) {
    return {
      type: entry.category,
      name: entry.category === "dapp" ? entry.item.name : entry.item.id,
    };
  }
  if (entry.category === "native") {
    return { type: "network", name: entry.token.chain };
  }
  const { address, chain, symbol } = entry.token;
  return address
    ? { type: "token", name: symbol, network: chain, contractAddress: address }
    : { type: "token", name: symbol, network: chain };
}

export function assetCornerIcon(entry: AssetEntry): CryptoIdentityIcon | undefined {
  return entry.category === "token" && "token" in entry
    ? { type: "network", name: entry.token.chain }
    : undefined;
}

export function assetLogoUrl(entry: AssetEntry): string | undefined {
  const logoUrl = "token" in entry ? entry.token.logoURI : entry.item.logoURI;
  if (!logoUrl) {
    return logoUrl;
  }
  try {
    const url = new URL(logoUrl);
    if (url.hostname === "assets-cdn.trustwallet.com") {
      return `${DEFAULT_CDN_ORIGIN}/gh/trustwallet/assets@master${url.pathname}`;
    }
    if (url.hostname === "cdn.jsdelivr.net") {
      return `${DEFAULT_CDN_ORIGIN}${url.pathname}`;
    }
    const repositoryPath = "/GMWalletApp/assets/main/";
    if (url.hostname === "raw.githubusercontent.com" && url.pathname.startsWith(repositoryPath)) {
      return `${DEFAULT_CDN_BASE_URL}/${url.pathname.slice(repositoryPath.length)}`;
    }
  } catch {
    return logoUrl;
  }
  return logoUrl;
}

export function identityBaseUrl(): string {
  return DEFAULT_CDN_BASE_URL;
}

export function formatDate(value?: string): string {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("zh-CN", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

async function fetchCompressedJson<T>(url: string, signal: AbortSignal): Promise<T> {
  const response = await fetch(url, {
    signal: AbortSignal.any([signal, AbortSignal.timeout(5_000)]),
    headers: { Accept: "application/zstd, application/json" },
  });
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`);
  }
  const compressed = new Uint8Array(await response.arrayBuffer());
  return JSON.parse(textDecoder.decode(decompress(compressed))) as T;
}

async function fetchCompressedJsonFromMirrors<T>(path: string, signal: AbortSignal): Promise<T> {
  let lastError: unknown;
  for (const baseUrl of orderedCdnBaseUrls()) {
    try {
      return await fetchCompressedJson<T>(`${baseUrl}/${path}`, signal);
    } catch (error) {
      if (signal.aborted) {
        throw error;
      }
      lastError = error;
    }
  }
  throw lastError instanceof Error ? lastError : new Error("无法加载资产索引");
}
