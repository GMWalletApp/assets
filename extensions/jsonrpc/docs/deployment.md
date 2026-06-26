# Deploying The Assets JSON-RPC Sidecar

## Build Artifact

Build from `extensions/jsonrpc`:

```bash
make build
```

The binary should run with the Trust Wallet assets repository mounted or checked out locally.

Cross-platform artifacts:

```bash
make build-all
```

Artifacts are written to `extensions/jsonrpc/dist/`.

## Runtime Configuration

Required:

```bash
--root /path/to/assets
```

Common flags:

```bash
--addr :8080
--asset-base-url https://assets-cdn.trustwallet.com
--market-sync-enabled true
--market-sync-interval 6h
--market-cache extensions/jsonrpc/data/market.json
--tokenlist-cache extensions/jsonrpc/data/tokenlist.json
--tokenlist-report extensions/jsonrpc/data/tokenlist-report.json
--tokenlist-rules extensions/jsonrpc/config/tokenlist-rules.json
--tokenlist-base-overrides extensions/jsonrpc/config/tokenlist-base-overrides.json
--tokenlist-manual-overrides extensions/jsonrpc/config/tokenlist-manual-overrides.json
--tokenlist-hot-defaults extensions/jsonrpc/config/tokenlist-hot-defaults.json
--tokenlist-hot-current extensions/jsonrpc/config/tokenlist-hot-current.json
--coingecko-vs-currency usd
--coingecko-base-url https://api.coingecko.com/api/v3
--coingecko-api-key-header x-cg-demo-api-key
--defillama-base-url https://stablecoins.llama.fi
--market-limit 1000
```

Environment:

```bash
COINGECKO_API_KEY=xxx
COINGECKO_API_BASE_URL=https://api.coingecko.com/api/v3
COINGECKO_API_KEY_HEADER=x-cg-demo-api-key
DEFILLAMA_STABLECOIN_BASE_URL=https://stablecoins.llama.fi
```

Only `COINGECKO_API_KEY` is required for market sync. The URL/header environment variables are optional overrides. DefiLlama stablecoin tag enrichment does not require an API key.

For CoinGecko Pro:

```bash
COINGECKO_API_BASE_URL=https://pro-api.coingecko.com/api/v3
COINGECKO_API_KEY_HEADER=x-cg-pro-api-key
```

`--asset-base-url` is only used to construct `logoURI` fields. The default matches upstream `.github/assets.config.yaml` `urls.assets_app`.

## Files Written By The Service

By default, the service writes:

```text
<assets-root>/extensions/jsonrpc/data/market.json
<assets-root>/extensions/jsonrpc/data/tokenlist.json
<assets-root>/extensions/jsonrpc/data/tokenlist-report.json
```

These files are local derived caches. They are not required for the upstream Trust Wallet assets repository to function.

They are placed under `extensions/jsonrpc/data/` so they can also be committed and consumed directly through GitHub Raw or another static file host.

The service also reads extension-local rules from:

```text
<assets-root>/extensions/jsonrpc/config/tokenlist-rules.json
<assets-root>/extensions/jsonrpc/config/tokenlist-base-overrides.json
<assets-root>/extensions/jsonrpc/config/tokenlist-manual-overrides.json
<assets-root>/extensions/jsonrpc/config/tokenlist-hot-defaults.json
<assets-root>/extensions/jsonrpc/config/tokenlist-hot-current.json
```

These files are maintained separately from upstream Trust Wallet asset data. They are used only while generating extension caches; they do not modify `blockchains/**/info.json`, logos, tokenlist files, or other upstream asset files.

## One-Shot Static JSON Deployment

Use this mode when you only need JSON files, for example for a Worker or static CDN:

```bash
cd /srv/assets/extensions/jsonrpc
COINGECKO_API_KEY=xxx make sync-once
```

`market.json` and tokenlist market enrichment default to the top 1000 CoinGecko rows. For a different one-shot market window or a single cache target:

```bash
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target market --market-limit 250"
COINGECKO_API_KEY=xxx make sync-once SYNC_ARGS="--sync-target tokenlist"
```

`--market-limit` limits the CoinGecko market rows fetched for `market.json` and tokenlist market enrichment. It does not trim `tokenlist.json`; status filtering and config rules decide inclusion.

Then publish or commit:

```text
extensions/jsonrpc/data/market.json
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

Example raw URLs:

```text
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/market.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist.json
https://raw.githubusercontent.com/<owner>/<repo>/<branch>/extensions/jsonrpc/data/tokenlist-report.json
```

The generated files use local chain/address/decimals/logo/explorer metadata as the source of truth. `tokenlist.json` ranks local native coins and contract tokens by CoinGecko market capitalization when they can be associated with a CoinGecko market row through an explicit native mapping, local CoinGecko/CoinMarketCap links, or CoinGecko platform contract addresses. This association does not mean the token is official, bridged, or supported for trading; local tags such as `stablecoin` or `binance-peg` remain the token metadata for that distinction.

`extensions/jsonrpc/config/tokenlist-rules.json` only holds generic rules. Asset-level overrides and hot lists live in the four companion config files. Keep all five config files when syncing or merging upstream Trust Wallet asset updates. If upstream changes a token address or removes an asset, the next `tokenlist-report.json` will list the affected rule under `issues.ruleIssues`.

## GitHub Actions Static JSON Generation

Generation workflow:

```text
.github/workflows/jsonrpc-data.yml
```

It supports:

```text
push to main or master
workflow_dispatch
```

Manual `workflow_dispatch` runs accept `sync_target` (`all` or `tokenlist`) and `market_limit`. Push-triggered runs use `all` and `market_limit=1000`.

Before enabling it, configure this repository secret:

```text
COINGECKO_API_KEY
```

Optional repository variables:

```text
COINGECKO_API_BASE_URL
COINGECKO_API_KEY_HEADER
DEFILLAMA_STABLECOIN_BASE_URL
```

On each run, it:

```text
1. Checks out the repository
2. Sets up Go from extensions/jsonrpc/go.mod
3. Runs sidecar tests
4. Runs make sync-once
5. Validates market.json, tokenlist.json, and tokenlist-report.json
6. Commits changed JSON files back to the same branch
```

Generated files:

```text
extensions/jsonrpc/data/market.json
extensions/jsonrpc/data/tokenlist.json
extensions/jsonrpc/data/tokenlist-report.json
```

These files can be served directly from GitHub Raw, a static CDN, or a Worker without running the RPC server.

Tokenlist config CRUD workflow:

```text
.github/workflows/jsonrpc-tokenlist-config.yml
```

It is manual-only and supports:

```text
override_upsert
override_delete
hot_replace_current
hot_add_current
hot_remove_current
hot_reset_current
```

Each config run updates the relevant config file, regenerates `tokenlist.json` and `tokenlist-report.json`, then commits both config and output.

## Security Notes

- Store `COINGECKO_API_KEY` in GitHub Secrets or a server-local environment file that is not committed.
- Do not commit generated `.env` files, API keys, private keys, wallet seed phrases, or personal paths.
- The JSON-RPC endpoint is read-only and rejects request bodies larger than 1 MiB, but public deployments should still use normal HTTP protection such as a reverse proxy, TLS, and rate limiting.
- For private/internal deployments, bind the service to localhost with `--addr 127.0.0.1:8080` and expose it through your own gateway.
- The GitHub workflow only passes `COINGECKO_API_KEY` to the key-check and JSON generation steps, and automatic push runs are limited to `main` and `master`.

## systemd Example

```ini
[Unit]
Description=Assets JSON-RPC sidecar
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/srv/assets/extensions/jsonrpc
Environment=COINGECKO_API_KEY=replace-me
ExecStart=/srv/assets/bin/assets-rpc \
  --addr :8080 \
  --root /srv/assets \
  --market-sync-enabled=true \
  --market-sync-interval=6h
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Container Pattern

Build the binary in a Go builder image, then run it with the assets repository mounted read-only plus a writable `data` directory:

```bash
docker run --rm \
  -p 8080:8080 \
  -e COINGECKO_API_KEY=xxx \
  -v /srv/assets:/srv/assets:ro \
  -v /srv/assets-cache:/cache \
  assets-rpc \
  --addr :8080 \
  --root /srv/assets \
  --market-cache /cache/market.json \
  --tokenlist-cache /cache/tokenlist.json \
  --tokenlist-report /cache/tokenlist-report.json
```

If the repository is writable and you want GitHub Raw-compatible paths, use the defaults instead:

```bash
--market-cache extensions/jsonrpc/data/market.json
--tokenlist-cache extensions/jsonrpc/data/tokenlist.json
--tokenlist-report extensions/jsonrpc/data/tokenlist-report.json
```

## Upstream Sync Workflow

Recommended workflow:

```bash
cd /srv/assets
git pull --ff-only upstream master
cd extensions/jsonrpc
make build
systemctl restart assets-rpc
```

Because the sidecar is isolated under `extensions/jsonrpc`, upstream changes to `blockchains/`, `internal/`, `cmd/`, or `Makefile` should not conflict with the service code.

Automated upstream sync is possible, but direct auto-merge is usually riskier than opening a PR. A safe pattern is:

```text
schedule/manual workflow
  -> git fetch upstream
  -> git merge upstream/master or upstream/main
  -> run checks
  -> create PR
```

Directly pushing an automatic upstream merge can break the branch if upstream changes conflict. Keeping custom code under `extensions/jsonrpc` keeps the conflict surface small, so either manual fast-forward sync or a scheduled PR workflow is usually enough.

## Health Check

Use `listChains` as a simple readiness check:

```bash
curl -sS http://127.0.0.1:8080/rpc \
  -H 'Content-Type: application/json' \
  --data '{"jsonrpc":"2.0","id":1,"method":"listChains","params":{}}'
```

## Failure Behavior

- If CoinGecko sync fails, the service keeps serving the previous `market.json`.
- If `COINGECKO_API_KEY` is missing, market sync is skipped.
- If DefiLlama sync fails, tokenlist generation keeps using the previous `tokenlist.json` until the next successful sync.
- If caches do not exist yet, ranking methods return empty lists; local asset lookup still works.
