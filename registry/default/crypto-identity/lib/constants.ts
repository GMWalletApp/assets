const ASSETS_REPOSITORY = "GMWalletApp/assets";
const ASSETS_BRANCH = "main";

export const CDN_BASE_URLS = [
  `https://cdn.jsdmirror.com/gh/${ASSETS_REPOSITORY}@${ASSETS_BRANCH}`,
  `https://cdn.jsdelivr.net/gh/${ASSETS_REPOSITORY}@${ASSETS_BRANCH}`,
  `https://fastly.jsdelivr.net/gh/${ASSETS_REPOSITORY}@${ASSETS_BRANCH}`,
  `https://gcore.jsdelivr.net/gh/${ASSETS_REPOSITORY}@${ASSETS_BRANCH}`,
  `https://raw.githubusercontent.com/${ASSETS_REPOSITORY}/${ASSETS_BRANCH}`,
] as const;

export function orderedCdnBaseUrls(preferredBaseUrl?: string): string[] {
  if (!preferredBaseUrl) {
    return [...CDN_BASE_URLS];
  }
  const normalized = preferredBaseUrl.replace(/\/$/, "");
  return [normalized, ...CDN_BASE_URLS.filter((url) => url !== normalized)];
}

export const CATALOG_PATHS = {
  dapps: "support/dapps.json.zst",
  support: "support/support.json.zst",
  swapProviders: "support/swap-providers.json.zst",
  tokens: "extensions/jsonrpc/data/tokenlist.json.zst",
} as const;

export const NETWORK_SLUG_ALIASES: Readonly<Record<string, string>> = {
  "arbitrum-one": "arbitrum",
  avalanche: "avalanchec",
  "avalanche-c-chain": "avalanchec",
  "avax-c-chain": "avalanchec",
  "base-mainnet": "base",
  "binance-smart-chain": "smartchain",
  "bitcoin-mainnet": "bitcoin",
  "bnb-chain": "smartchain",
  "bnb-smart-chain": "smartchain",
  bnb: "smartchain",
  bsc: "smartchain",
  "ethereum-mainnet": "ethereum",
  matic: "polygon",
  "optimism-mainnet": "optimism",
  "polygon-mainnet": "polygon",
  "polygon-pos": "polygon",
  "solana-mainnet": "solana",
  "tron-mainnet": "tron",
} as const;

export const SUPPORT_SLUG_ALIASES: Readonly<Record<string, string>> = {
  "binance-wallet": "binance",
  "bitget-wallet": "bitget",
  "coinbase-wallet": "base",
  coinbasewallet: "base",
  "im-token": "imtoken",
  "okx-wallet": "okx",
  "one-key": "onekey",
  "phantom-wallet": "phantom",
  "rabby-wallet": "rabby",
  "rainbow-wallet": "rainbow",
  tokenpocket: "token-pocket",
  "trust-wallet": "trust",
  walletconnect: "wallet-connect",
} as const;
