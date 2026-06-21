# Assets JSON-RPC Usage

## Build

From `extensions/jsonrpc`:

```bash
make build
```

The binary is written to:

```text
../../bin/assets-rpc
```

Cross-platform release builds:

```bash
make build-all
```

## Generate Static JSON Caches

The service can run as a one-shot sync job without starting HTTP:

```bash
COINGECKO_API_KEY=xxx make sync-once
```

By default, `market.json` fetches up to 100 CoinGecko market rows before local asset matching, and `tokenlist.json` keeps assets with `rank <= 100`. To use a different market window or generate only one cache:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target market --market-limit 250"
make sync-once SYNC_ARGS="--sync-target stablecoins"
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist"
```

To generate an app tokenlist containing only assets associated with the top 100 market rows:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist --market-limit 100 --tokenlist-max-rank 100"
```

`--market-limit` controls the CoinGecko market window used for `market.json`, `market` enrichment, and rank matching. `--tokenlist-max-rank` controls whether `tokenlist.json` keeps all active local assets or only assets with `rank <= N`; the default is `100`, and `0` keeps all active assets.

CoinGecko Demo API is used by default:

```text
COINGECKO_API_BASE_URL=https://api.coingecko.com/api/v3
COINGECKO_API_KEY_HEADER=x-cg-demo-api-key
```

For a CoinGecko Pro key, set:

```bash
COINGECKO_API_BASE_URL=https://pro-api.coingecko.com/api/v3 \
COINGECKO_API_KEY_HEADER=x-cg-pro-api-key \
COINGECKO_API_KEY=xxx \
make sync-once
```

Default output:

```text
extensions/jsonrpc/data/market.json
extensions/jsonrpc/data/stablecoins.json
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

Default tokenlist rules:

```text
extensions/jsonrpc/config/tokenlist-rules.json
```

These JSON files are intentionally inside the repository tree, so they can be committed and read directly through GitHub Raw, a CDN, or a Worker.

Example raw URLs after pushing to GitHub:

```text
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/market.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/stablecoins.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist-report.json
```

`market.json` is generated from CoinGecko plus local Trust Wallet asset metadata.

`stablecoins.json` is an independent DefiLlama ranking cache plus local Trust Wallet asset metadata.

`tokenlist.json` is the app packaging list. It contains active native coins and contract tokens with local name/symbol/decimals/logo metadata, ordered by CoinGecko market capitalization when a local asset can be associated with a CoinGecko market row.

`tokenlist-report.json` records CoinGecko API inputs, local asset counts, market association counts, missing platform mappings, external contracts missing from this repository, filtered assets, and missing logos.

Generated files use local repository metadata as the source of truth. CoinGecko market data is used for ranking and market entity association, not as proof that a token is official, bridged, or supported for trading. `--sync-target` accepts `all`, `market`, `stablecoins`, or `tokenlist`; the default is `all`.

## Tokenlist Rules

`extensions/jsonrpc/config/tokenlist-rules.json` is an extension-local rules file. It is not part of upstream Trust Wallet asset metadata and it is never written back to `blockchains/**`. It only affects generated caches such as `tokenlist.json`, `market.json`, `stablecoins.json`, and diagnostics in `tokenlist-report.json`.

Use `--tokenlist-rules` to point at a different rules file:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist --tokenlist-rules extensions/jsonrpc/config/tokenlist-rules.json"
```

The rules file has four top-level sections:

```json
{
  "platformMappings": {
    "plasma": "plasma",
    "near-protocol": "near",
    "harmony-shard-0": "harmony"
  },
  "nativeMarketMappings": {
    "ethereum": ["ethereum", "arbitrum", "base", "optimism"],
    "polygon-ecosystem-token": ["polygon"]
  },
  "assetOverrides": [
    {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955",
      "coingeckoId": "tether",
      "displayName": "Binance-Peg Tether USD",
      "addTags": ["stablecoin", "binance-peg"]
    }
  ],
  "marketTagRules": [
    {
      "coingeckoId": "usd-coin",
      "addTags": ["stablecoin"]
    }
  ]
}
```

`platformMappings` maps CoinGecko `platforms` keys to this repository's `blockchains/<chain>` handles. Add a rule when CoinGecko returns a valid platform/address but `tokenlist-report.json` lists it under `issues.unmappedPlatforms`.

`nativeMarketMappings` maps CoinGecko market IDs to native chain assets, which have no contract address. This is how Arbitrum, Base, or Optimism native gas ETH can inherit the Ethereum market rank. Use an empty list, such as `"arbitrum": []`, to suppress an old built-in native mapping while still letting contract token matching handle the market row.

`assetOverrides` targets one local token by `chain + address`. It can bind a generated token to a `coingeckoId`, override generated `name` or `symbol` through `displayName`/`displaySymbol`, and add app tags through `addTags`. It cannot change `address`, `decimals`, `type`, `logoURI`, or any source `blockchains/**/info.json` file.

`marketTagRules` adds tags to generated tokens that are associated with a CoinGecko market ID. For example, `tether`, `usd-coin`, and `dai` can consistently receive `stablecoin`.

Rule priority:

```text
platformMappings and nativeMarketMappings: rules first, built-in mapping fallback second.
assetOverrides: applied after market association and before final sort.
marketTagRules: applied after assetOverrides.
```

`tokenlist-report.json` includes `rules` counters and `issues.ruleIssues`. Use these diagnostics to find unused or broken rules, such as an override pointing to a missing local asset or a CoinGecko ID that is outside the synced market window.

## GitHub Action

The repository includes `.github/workflows/jsonrpc-data.yml`.

It runs on:

```text
push to main or master
workflow_dispatch
```

Required repository secret:

```text
COINGECKO_API_KEY
```

Optional repository variables:

```text
COINGECKO_API_BASE_URL
COINGECKO_API_KEY_HEADER
DEFILLAMA_STABLECOIN_BASE_URL
```

Manual workflow runs can override `sync_target`, `market_limit`, and `tokenlist_max_rank`. Push-triggered runs use `sync_target=all`, `market_limit=100`, and `tokenlist_max_rank=100`.

The workflow runs:

```bash
cd extensions/jsonrpc
make test
make sync-once
```

Then it commits generated files when they change:

```text
extensions/jsonrpc/data/market.json
extensions/jsonrpc/data/stablecoins.json
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

The workflow ignores pushes that only change `extensions/jsonrpc/data/**` to avoid a commit loop.

## External Sync Sources

`market.json`:

```text
CoinGecko GET https://api.coingecko.com/api/v3/coins/markets
CoinGecko GET https://api.coingecko.com/api/v3/coins/list?include_platform=true
```

Used market fields include price, market cap, market cap rank, total volume, symbol, and name. Native coins are associated through an explicit CoinGecko ID to local chain mapping. Contract tokens are associated through local `coingecko`/`coinmarketcap` links and, when available, CoinGecko platform contract addresses by chain and address. These associations do not imply official issuance or trading support. `--market-limit` can reduce the synced market window, for example `--market-limit 100` for the top 100 rows. `COINGECKO_API_KEY` is required for sync. Demo keys use `https://api.coingecko.com/api/v3` with `x-cg-demo-api-key`; Pro keys use `https://pro-api.coingecko.com/api/v3` with `x-cg-pro-api-key`.

`stablecoins.json`:

```text
DefiLlama GET https://stablecoins.llama.fi/stablecoins?includePrices=true
CoinGecko GET https://api.coingecko.com/api/v3/coins/list?include_platform=true
```

Used fields include stablecoin name, symbol, peg type, circulating value, and chain distribution. DefiLlama does not require an API key.

`tokenlist.json` does not call DefiLlama. Stablecoin sync uses DefiLlama only for the separate `stablecoins.json` cache. Neither tokenlist nor stablecoin sync binds assets by symbol/name.

## Start JSON-RPC HTTP

```bash
COINGECKO_API_KEY=xxx ../../bin/assets-rpc \
  --root ../.. \
  --addr :8080
```

`--asset-base-url` defaults to `https://assets-cdn.trustwallet.com`. This is only used to build `logoURI` values and matches upstream `.github/assets.config.yaml` `urls.assets_app`; it is not a JSON-RPC or market-data API.

For local testing without external sync:

```bash
../../bin/assets-rpc \
  --root ../.. \
  --addr :8080 \
  --market-sync-enabled=false
```

## Endpoint

```text
POST /rpc
Content-Type: application/json
```

The server supports JSON-RPC 2.0 single requests and batch requests.

## Asset Lookup

### `getAssetByAddress`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getAssetByAddress",
  "params": {
    "chain": "smartchain",
    "address": "0x55d398326f99059fF775485246999027B3197955"
  }
}
```

Returns local asset details:

```json
{
  "chain": "smartchain",
  "address": "0x55d398326f99059fF775485246999027B3197955",
  "assetId": "c20000714_t0x55d398326f99059fF775485246999027B3197955",
  "name": "Tether USD",
  "symbol": "USDT",
  "type": "BEP20",
  "decimals": 18,
  "status": "active",
  "website": "https://tether.to",
  "explorer": "https://bscscan.com/token/0x55d398326f99059fF775485246999027B3197955",
  "tags": ["stablecoin"],
  "links": [],
  "logoURI": "https://assets-cdn.trustwallet.com/blockchains/smartchain/assets/0x55d398326f99059fF775485246999027B3197955/logo.png",
  "logoExists": true
}
```

### `getAssetById`

```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "getAssetById",
  "params": {
    "assetId": "c20000714_t0x55d398326f99059fF775485246999027B3197955"
  }
}
```

`assetId` is the Trust Wallet internal asset identifier. For external callers, prefer `getAssetByAddress`.

## Chain And Token Lists

### `listChains`

```json
{"jsonrpc":"2.0","id":1,"method":"listChains","params":{}}
```

### `getChainInfo`

```json
{"jsonrpc":"2.0","id":1,"method":"getChainInfo","params":{"chain":"smartchain"}}
```

### `getTokenList`

```json
{"jsonrpc":"2.0","id":1,"method":"getTokenList","params":{"chain":"smartchain","extended":false}}
```

This returns the upstream per-chain Trust Wallet tokenlist file.

### `getAppTokenList`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getAppTokenList",
  "params": {
    "chain": "smartchain",
    "limit": 50,
    "offset": 0,
    "maxRank": 100,
    "onlyWithMarket": true
  }
}
```

This returns the generated app packaging list from `extensions/jsonrpc/data/tokenlist.json`. `limit` controls the response size at request time; it does not fetch more market data than was synced. To make larger ranked windows available at runtime, generate caches with a larger `--market-limit` and `--tokenlist-max-rank`.

## Rankings

Ranking methods embed full local asset details in `assets[]`. Clients should not need to loop over `getAssetByAddress`.

### `getMarketRankings`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getMarketRankings",
  "params": {
    "order": "market_cap_desc",
    "limit": 100,
    "offset": 0,
    "onlyWithAssets": true
  }
}
```

Supported `order` values:

```text
market_cap_desc
volume_desc
market_cap_rank_asc
```

### `getStablecoinRankings`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getStablecoinRankings",
  "params": {
    "limit": 100,
    "offset": 0,
    "onlyWithAssets": true
  }
}
```

### `listStablecoins`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "listStablecoins",
  "params": {
    "chain": "smartchain",
    "limit": 100,
    "offset": 0,
    "onlyWithAssets": true
  }
}
```

If `data/stablecoins.json` exists, this reads DefiLlama-enriched cache. Otherwise it falls back to local assets tagged `stablecoin`.

### `getStablecoinBySymbol`

```json
{"jsonrpc":"2.0","id":1,"method":"getStablecoinBySymbol","params":{"symbol":"USDT"}}
```

### `getAssetMarket`

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "getAssetMarket",
  "params": {
    "chain": "smartchain",
    "address": "0x55d398326f99059fF775485246999027B3197955"
  }
}
```

## JSON-RPC Errors

```text
-32700 parse error
-32600 invalid request
-32601 method not found
-32602 invalid params
-32603 internal error
-32004 not found
```

## Cache Files

Default cache paths are relative to `--root`:

```text
extensions/jsonrpc/data/market.json
extensions/jsonrpc/data/stablecoins.json
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

Override them when needed:

```bash
../../bin/assets-rpc \
  --root ../.. \
  --market-cache /cache/market.json \
  --stablecoin-cache /cache/stablecoins.json \
  --tokenlist-cache /cache/tokenlist.json \
  --tokenlist-report /cache/tokenlist-report.json
```
