# Homepage Tokenlist

`build_homepage.py` builds `out/homepage.json` from the local Trust Wallet `blockchains/*` data.

## Manual overrides

When Trust Wallet does not contain a homepage token you want, add it to
`homepage_overrides.json`. The script merges overrides after the automatic build.
If `homepage_overrides.json` stays empty, the build falls back to the automatic
Trust Wallet asset selection only.

Supported root shapes:

- `{ "tokens": [ ... ] }`
- `[ ... ]`
- a single token object

Each override token should use the same shape as a `homepage.json` token item.

Required fields:

- `chain`: canonical chain key like `zksync` or export key like `bsc`
- `slot`: `usdt`, `usdc`, or `dai`
- `kind`: must be `token`
- `symbol`
- `name`
- `address`
- `decimals`
- `logoURI`
- `explorer`

Optional fields:

- `id`: if present, it must match the final computed id; the script recomputes it
- `chainName`
- `chainId`
- `chainLogoURI`
- `displaySymbol`
- `displayName`
- `source`

The script always recomputes these fields from `chain` and `slot` to keep output
consistent:

- `id`
- `chain`
- `chainName`
- `chainId`
- `chainLogoURI`
- `slot`
- `kind`
- `displaySymbol`
- `displayName`

Example:

```json
{
  "tokens": [
    {
      "id": "zksync:usdt",
      "chain": "zksync",
      "chainName": "zkSync Era",
      "chainId": 324,
      "chainLogoURI": "https://assets-cdn.trustwallet.com/blockchains/zksync/info/logo.png",
      "slot": "usdt",
      "kind": "token",
      "displaySymbol": "USDT",
      "displayName": "Tether",
      "symbol": "USDT",
      "name": "Bridged USDT",
      "address": "0x493257fD37EDB34451f62EDf8D2a0C418852bA4C",
      "decimals": 6,
      "logoURI": "https://assets.coingecko.com/coins/images/35001/thumb/logo.png?1706959346",
      "explorer": "https://explorer.zksync.io/address/0x493257fD37EDB34451f62EDf8D2a0C418852bA4C",
      "source": "manual-override:coingecko"
    }
  ]
}
```

## GitHub Actions

The `Build Homepage Tokenlist` workflow is manual-only and supports:

- tracked overrides from `homepage_overrides.json`
- manual inline homepage token JSON via `workflow_dispatch`

When manual inline JSON is provided, the workflow:

1. merges it into `homepage_overrides.json`
2. rebuilds `out/homepage.json`
3. commits `homepage_overrides.json` back to the repo
4. uploads `out/homepage.json` as a workflow artifact

If no manual override is provided, the manually triggered workflow just rebuilds
`out/homepage.json` from Trust Wallet assets plus any existing saved overrides.
