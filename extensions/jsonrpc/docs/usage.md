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

By default, `market.json` and `tokenlist.json` use the top 1000 CoinGecko market rows for market enrichment. To use a different market window or generate only one cache:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target market --market-limit 250"
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist --market-limit 1000"
```

`--market-limit` controls the CoinGecko market window used for `market.json` and tokenlist market enrichment. It does not trim the generated tokenlist; all local assets remain eligible unless filtered by `excludedStatuses`.

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
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

## Managed Token Lists

The service also exposes a SQLite-backed list manager for app-specific groups
such as `usdt`, `usdc`, `usdg`, `usds`, `stablecoin`, `eth`, `dai`, `homepage`,
`tokenlist`, and the exchange/wallet `support` list.

Default storage and packed output:

```text
extensions/jsonrpc/data/lists.sqlite
extensions/jsonrpc/data/lists/<list>.json
extensions/jsonrpc/data/lists/<list>.json.zst
extensions/jsonrpc/data/lists/manifest.json
```

On startup, `assets-rpc` seeds default managed lists from the current generated
files without overwriting existing list items:

- `tokenlist`: every token in `extensions/jsonrpc/data/tokenlist.json`; this is the generic app token list
- `usdt`: `USDT` and `USDT0`, so app surfaces can use one USDT family list
- `usdc`: native `USDC` and bridged `USDC.e` variants
- `usdg`: Global Dollar deployments
- `usds`: Sky USDS deployments; unrelated tokens that happen to reuse the `USDS` symbol are excluded
- `dai`, `eth`: exact symbol lists
- `stablecoin`: tokens tagged `stablecoin`
- `homepage`: direct homepage tokens plus enabled multi-chain list includes
- `support`: exchanges/DEXes and wallets seeded from `support/support.json`,
  plus the repository `data/static/bitget-wallet.svg` and
  `data/static/uniswap-wallet.svg` wallet entries

By default, homepage includes `usdt`, `usdc`, `usdg`, and `usds`. An include's
`tag` is the source list key. Packing reads that list directly from SQLite,
assigns the tag as each expanded item's `slot`, and replaces duplicate direct
homepage entries by normalized chain/address identity. No loopback HTTP request
is made. This keeps one family definition while the packed homepage remains a
flat array of the same item shape used by every other list.

Business lists other than the generic `tokenlist` carry homepage-style item
metadata:

- `slot`: list-specific display slot, such as `usdt`, `stablecoin`, or `native`
- `display`: app visibility flag; disabled display items can stay in the list for controlled rollout
- `displayName` and `displaySymbol`: list-level presentation overrides
- `chainName`, `chainId`, `chainLogoURI`, `explorer`, `logoURI`, and tags

Every managed list can also define shared presentation fields:

- `displayName`: common app-facing name, for example `Tether USD`
- `displaySymbol`: common app-facing symbol, for example `USDT`
- `logoURI`: common family logo applied to every item by default

The default USDT family logo is the repository-owned
`data/static/USDT.svg`. Polygon chain presentation uses
`data/static/poly.svg`; both are published through stable GitHub Raw URLs.

Items inherit those fields when packed. A contract can override the shared
name and symbol with `displayName`/`displaySymbol`, and the shared logo with
`displayLogoURI`. Base contract metadata (`name`, `symbol`, and its original
`logoURI`) remains available on each packed token.

`enabled` and `display` have different meanings. `enabled:false` excludes the
item from packed files. `display:false` keeps the item in packed files but tells
the app not to show it by default.

For the `usdt`, `usdc`, `usdg`, and `usds` family lists, every discovered item
is enabled. Items on `arbitrum`, `polygon`, `smartchain`, `ethereum`, and `tron`
default to `display:true`; items on every other chain default to
`display:false`. Display remains editable per contract through the item API.

Override paths when starting the service:

```bash
go run ./cmd/assets-rpc \
  --root ../.. \
  --managed-list-db extensions/jsonrpc/data/lists.sqlite \
  --managed-list-files-dir extensions/jsonrpc/data/lists \
  --managed-list-seed-defaults=true \
  --managed-list-pack-after-seed=false
```

CRUD endpoints:

```text
GET    /api/lists
POST   /api/lists
GET    /api/lists/{listKey}
PUT    /api/lists/{listKey}
PATCH  /api/lists/{listKey}
DELETE /api/lists/{listKey}

GET    /api/lists/{listKey}/items
POST   /api/lists/{listKey}/items
GET    /api/lists/{listKey}/items/{chain}/{address}
PUT    /api/lists/{listKey}/items/{chain}/{address}
PATCH  /api/lists/{listKey}/items/{chain}/{address}
DELETE /api/lists/{listKey}/items/{chain}/{address}

GET    /api/lists/{listKey}/includes
POST   /api/lists/{listKey}/includes
GET    /api/lists/{listKey}/includes/{tag}
PUT    /api/lists/{listKey}/includes/{tag}
PATCH  /api/lists/{listKey}/includes/{tag}
DELETE /api/lists/{listKey}/includes/{tag}

GET    /api/lists/support/{exchanges|wallets}
POST   /api/lists/support/{exchanges|wallets}
GET    /api/lists/support/{exchanges|wallets}/{id}
PUT    /api/lists/support/{exchanges|wallets}/{id}
PATCH  /api/lists/support/{exchanges|wallets}/{id}
DELETE /api/lists/support/{exchanges|wallets}/{id}

POST   /api/pack/{listKey}
POST   /api/pack/all
GET    /files/{outputName}.json
GET    /files/{outputName}.json.zst
GET    /files/manifest.json
GET    /openapi.yaml
```

The complete OpenAPI 3.1 contract is served from `/openapi.yaml`. Request
objects reject unknown JSON fields and multiple JSON values. Successful creates
return `201` with `Location`; deletes return an empty `204`; duplicate POSTs
return `409`; missing resources return `404`.

`GET /api/lists/{listKey}` returns one complete management document: the list
fields (`key`, name, shared display fields, output settings, timestamps), an
`includes` array, and an `items` array. The dedicated routes remain available
for editing an individual include or token item.

`GET /api/lists/support` instead returns the same list-level fields together
with `schemaVersion`, `assetBaseURI`, `exchanges`, and `wallets`. Support-entry
`rank`, `enabled`, and timestamps are management fields. Packing omits disabled
entries and publishes the frontend-compatible `support.json` shape containing
only `id`, `name`, optional exchange `type`, and `logoURI`.

All managed lists use the same `ManagedListItem` shape. The large `tokenlist`
therefore exposes the same token, chain, logo, explorer, tags, `hot`, market,
pairs, links, display, and membership fields as `usdt`, `stablecoin`, and
`homepage`. A frontend can select an item and post that complete JSON object to
another list without reshaping it; returned `createdAt`/`updatedAt` values are
accepted and regenerated by the target list.

For example, copy one selected tokenlist item directly into homepage:

```bash
curl -sS http://localhost:8080/api/lists/tokenlist/items/ethereum/0xdAC17F958D2ee523a2206206994597C13D831ec7 \
  | curl -sS http://localhost:8080/api/lists/homepage/items \
      -H 'content-type: application/json' \
      --data-binary @-
```

Packed `/files/{outputName}.json` files use the same aggregate boundary: list-level
metadata and the enabled, presentation-resolved entries are emitted together in
the `items` array.

Include a multi-chain family in homepage:

```bash
curl -sS http://localhost:8080/api/lists/homepage/includes \
  -H 'content-type: application/json' \
  --data '{"tag":"usdt","rank":10,"enabled":true}'
```

The source list must exist. Self-includes and indirect cycles are rejected.
Setting an include to `enabled:false` keeps the relationship in SQLite but
stops expanding it into the packed target list.

The service does not implement application-level authentication. In deployed
environments, protect `/api/lists*` and `/api/pack/*` with Caddy authentication;
`/rpc` and `/files/*` can remain read-only public endpoints when required.

Create a list:

```bash
curl -sS http://localhost:8080/api/lists \
  -H 'content-type: application/json' \
  --data '{
    "key":"usdt",
    "name":"USDT List",
    "displayName":"Tether USD",
    "displaySymbol":"USDT",
    "logoURI":"https://assets.example/usdt.png",
    "enabled":true
  }'
```

Add an existing repository asset to a list. When `chain` and `address` match
`blockchains/<chain>/assets/<address>/info.json`, the service hydrates symbol,
name, decimals, logo, status, tags, and asset ID from the local asset repository.

```bash
curl -sS http://localhost:8080/api/lists/usdt/items \
  -H 'content-type: application/json' \
  --data '{
    "token": {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955"
    },
    "slot": "usdt",
    "rank": 1,
    "enabled": true,
    "display": true,
    "displaySymbol": "USDT",
    "displayLogoURI": "https://assets.example/bsc-usdt.png"
  }'
```

Pack one list and download the compressed file:

```bash
curl -sS -X POST http://localhost:8080/api/pack/usdt
curl -O http://localhost:8080/files/usdt.json.zst
```

`POST /api/pack/all` writes every enabled list and updates `manifest.json`.
SQLite stores list membership and display overrides; the local `blockchains/**`
asset tree remains the source for base token metadata.

Default tokenlist config:

```text
extensions/jsonrpc/config/tokenlist-rules.json
extensions/jsonrpc/config/tokenlist-base-overrides.json
extensions/jsonrpc/config/tokenlist-manual-overrides.json
extensions/jsonrpc/config/tokenlist-manual-tokens.json
extensions/jsonrpc/config/tokenlist-hot-defaults.json
extensions/jsonrpc/config/tokenlist-hot-current.json
```

These JSON files are intentionally inside the repository tree, so they can be committed and read directly through GitHub Raw, a CDN, or a Worker.

Example raw URLs after pushing to GitHub:

```text
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/market.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist-report.json
```

`market.json` is generated from CoinGecko plus local Trust Wallet asset metadata.

`tokenlist.json` is the app packaging list. It contains local native coins and contract tokens after status filtering, plus:

- CoinGecko `market` and `rank`
- DefiLlama-derived `stablecoin` tag
- top-level `hot: true|false` from hot config
- base/manual override display and market binding
- manual tokens appended from Action-managed final-token entries

`tokenlist-report.json` records CoinGecko API inputs, local asset counts, market association counts, missing platform mappings, external contracts missing from this repository, filtered assets, and missing logos.

Generated files use local repository metadata as the source of truth. CoinGecko market data is used for ranking and market entity association, not as proof that a token is official, bridged, or supported for trading. `--sync-target` accepts `all`, `market`, or `tokenlist`; the default is `all`.

## Tokenlist Rules

The tokenlist configuration is split by responsibility and never writes back to `blockchains/**`:

- `tokenlist-rules.json`: generic mapping/tag/filter rules
- `tokenlist-base-overrides.json`: long-lived override entries managed by PR
- `tokenlist-manual-overrides.json`: Action-managed manual override entries
- `tokenlist-manual-tokens.json`: Action-managed final token entries appended after generated assets; only `kind=token` is supported. A manual entry may intentionally reuse a local token's chain and address (for example, separate `USDT` and `USDT0` representations), but it must use a distinct, non-empty `assetId`.
- `tokenlist-hot-defaults.json`: long-lived default hot entries managed by PR
- `tokenlist-hot-current.json`: Action-managed current-period hot entries

Use `--tokenlist-rules` to point at a different rules file:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist --tokenlist-rules extensions/jsonrpc/config/tokenlist-rules.json"
```

The rules file has five top-level sections:

```json
{
  "excludedStatuses": ["spam", "abandoned"],
  "excludedChains": ["binance"],
  "platformMappings": {
    "plasma": "plasma",
    "near-protocol": "near",
    "harmony-shard-0": "harmony"
  },
  "nativeMarketMappings": {
    "ethereum": ["ethereum", "arbitrum", "base", "optimism"],
    "polygon-ecosystem-token": ["polygon"]
  },
  "marketTagRules": [
    {
      "coingeckoId": "usd-coin",
      "addTags": ["stablecoin"]
    }
  ]
}
```

`excludedStatuses` filters local assets by Trust Wallet asset status. The default excludes `spam` and `abandoned`.

`excludedChains` filters complete local chains out of the app tokenlist output. `binance` is BNB Beacon Chain / BEP2, which has been shut down since 2024-12-03, so it is excluded from app packaging. BSC assets remain under the local `smartchain` chain handle.

`platformMappings` maps CoinGecko `platforms` keys to this repository's `blockchains/<chain>` handles. Add a rule when CoinGecko returns a valid platform/address but `tokenlist-report.json` lists it under `issues.unmappedPlatforms`.

`nativeMarketMappings` maps CoinGecko market IDs to native chain assets, which have no contract address. This is how Arbitrum, Base, or Optimism native gas ETH can inherit the Ethereum market rank. Use an empty list, such as `"arbitrum": []`, to suppress an old built-in native mapping while still letting contract token matching handle the market row.

`marketTagRules` adds tags to generated tokens that are associated with a CoinGecko market ID. For example, `tether`, `usd-coin`, and `dai` can consistently receive `stablecoin`.

Override files use this shape:

```json
{
  "assetOverrides": [
    {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955",
      "coingeckoId": "tether",
      "displayName": "Binance-Peg Tether USD",
      "addTags": ["stablecoin", "binance-peg"]
    }
  ]
}
```

Hot files use this shape:

```json
{
  "tokens": [
    {
      "chain": "smartchain",
      "address": "0x55d398326f99059fF775485246999027B3197955"
    },
    {
      "chain": "smartchain",
      "address": ""
    }
  ]
}
```

Rule priority:

```text
platformMappings and nativeMarketMappings: rules first, built-in mapping fallback second.
effective overrides = base overrides + manual overrides, with manual taking precedence on the same chain + address.
effective hot = hot defaults union hot current.
marketTagRules: applied after market association and override resolution.
```

`tokenlist-report.json` includes `rules` counters and `issues.ruleIssues`. Use these diagnostics to find unused or broken rules, such as an override pointing to a missing local asset or a CoinGecko ID that is outside the synced market window.

## GitHub Actions

The repository includes two workflows:

- `.github/workflows/jsonrpc-data.yml`
- `.github/workflows/jsonrpc-tokenlist-config.yml`

`jsonrpc-data.yml` handles generation only:

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

Manual generation runs can override `sync_target` and `market_limit`. Push-triggered runs use `sync_target=all` and `market_limit=1000`.

`jsonrpc-tokenlist-config.yml` is manual-only and manages:

- `override_upsert`
- `override_delete`
- `manual_token_upsert`
- `manual_token_delete`
- `hot_replace_current`
- `hot_add_current`
- `hot_remove_current`
- `hot_reset_current`

It updates config files, regenerates `tokenlist.json` and `tokenlist-report.json`, then commits both config and generated output.

Manual token example:

```json
{
  "kind": "token",
  "chain": "solana",
  "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
  "assetId": "solana:METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL",
  "name": "Meteora",
  "symbol": "MET",
  "decimals": 6,
  "status": "active",
  "hot": true
}
```

Delete uses only `chain` and `address`, for example:

```json
{
  "chain": "solana",
  "address": "METvsvVRapdj9cFLzq4Tr43xK4tAjQfwX76z3n6mWQL"
}
```

`manual_token_upsert` only supports contract-token style entries with `kind: "token"`. Manual native assets are intentionally not supported because every valid chain already has a generated native asset.

Generation workflow:

```bash
cd extensions/jsonrpc
make test
make sync-once
```

Then it commits generated files when they change:

```text
extensions/jsonrpc/data/market.json
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

`tokenlist.json` enrichment:

```text
DefiLlama GET https://stablecoins.llama.fi/stablecoins?includePrices=true
CoinGecko GET https://api.coingecko.com/api/v3/coins/list?include_platform=true
```

DefiLlama is used only to tag matched assets as `stablecoin` during tokenlist generation. There is no separate `stablecoins.json` output.

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

This returns the generated app packaging list from `extensions/jsonrpc/data/tokenlist.json`. `limit` controls the response size at request time; it does not fetch more market data than was synced. To make larger ranked windows available at runtime, generate caches with a larger `--market-limit`. `maxRank` is a read-time filter only; it does not change what was generated into the file.

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
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

Override them when needed:

```bash
../../bin/assets-rpc \
  --root ../.. \
  --market-cache /cache/market.json \
  --tokenlist-cache /cache/tokenlist.json \
  --tokenlist-report /cache/tokenlist-report.json
```
