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
--tokenlist-manual-tokens extensions/jsonrpc/config/tokenlist-manual-tokens.json
--tokenlist-hot-defaults extensions/jsonrpc/config/tokenlist-hot-defaults.json
--tokenlist-hot-current extensions/jsonrpc/config/tokenlist-hot-current.json
--managed-list-db extensions/jsonrpc/data/lists.sqlite
--managed-list-files-dir extensions/jsonrpc/data/lists
--managed-list-public-base-url /files
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
<assets-root>/extensions/jsonrpc/config/tokenlist-manual-tokens.json
<assets-root>/extensions/jsonrpc/config/tokenlist-hot-defaults.json
<assets-root>/extensions/jsonrpc/config/tokenlist-hot-current.json
```

These files are maintained separately from upstream Trust Wallet asset data. They are used only while generating extension caches; they do not modify `blockchains/**/info.json`, logos, tokenlist files, or other upstream asset files.

Managed-list mode additionally writes:

```text
<managed-list-db>
<managed-list-files-dir>/<listKey>.json
<managed-list-files-dir>/<listKey>.json.zst
<managed-list-files-dir>/manifest.json
```

`<managed-list-db>` is the SQLite file path, not a directory. Both its parent
directory and the complete managed-list files
directory must be writable by the service account. The assets repository itself
can remain read-only when cache and managed-list paths point to a separate
writable directory.

## Production Managed-List Deployment

The following layout keeps the checked-out repository read-only at runtime and
stores all mutable state under `/var/lib/assets-rpc`:

```text
/srv/assets/                         assets repository
/srv/assets/bin/assets-rpc          installed binary
/var/lib/assets-rpc/market.json     generated market cache
/var/lib/assets-rpc/tokenlist.json  generated large token list
/var/lib/assets-rpc/report.json     generation report
/var/lib/assets-rpc/lists.sqlite    managed-list database
/var/lib/assets-rpc/lists/          public JSON, Zstd, and manifest files
/etc/assets-rpc/env                 server-only environment variables
```

Create the service account and writable directories. On Debian/Ubuntu, for
example:

```bash
sudo useradd --system --home /var/lib/assets-rpc --shell /usr/sbin/nologin assets-rpc
sudo install -d -o assets-rpc -g assets-rpc -m 0750 \
  /var/lib/assets-rpc \
  /var/lib/assets-rpc/lists
sudo install -d -o root -g assets-rpc -m 0750 /etc/assets-rpc
```

Then build and install the binary:

```bash
cd /srv/assets/extensions/jsonrpc
go test ./...
make build BIN_DIR=/tmp/assets-rpc-build
sudo install -d -o root -g root -m 0755 /srv/assets/bin
sudo install -m 0755 /tmp/assets-rpc-build/assets-rpc /srv/assets/bin/assets-rpc
```

Store secrets in `/etc/assets-rpc/env` and restrict the file to root and the
service account:

```dotenv
COINGECKO_API_KEY=replace-me
```

Set its ownership and mode after creating it:

```bash
sudo chown root:assets-rpc /etc/assets-rpc/env
sudo chmod 0640 /etc/assets-rpc/env
```

Before the first service start, generate the large tokenlist used to seed
`tokenlist`, stablecoin families, and other managed lists:

```bash
sudo -u assets-rpc sh -c '
  set -a
  . /etc/assets-rpc/env
  exec /srv/assets/bin/assets-rpc \
    --root /srv/assets \
    --sync-once \
    --sync-target all \
    --market-cache /var/lib/assets-rpc/market.json \
    --tokenlist-cache /var/lib/assets-rpc/tokenlist.json \
    --tokenlist-report /var/lib/assets-rpc/report.json
'
```

The first normal start should use `--managed-list-seed-defaults=true` and
`--managed-list-pack-after-seed=true`. This creates the initial SQLite schema, seeds the
default lists, and immediately publishes JSON/Zstd artifacts plus the manifest.
Later starts are idempotent: seeding does not overwrite existing list
memberships, and schema creation safely skips tables that already exist.

Background market synchronization refreshes the source tokenlist cache. Newly
discovered tokens are imported into managed lists on the next service start;
restart the unit after a desired sync and the configured pack-after-seed step
will republish the enabled lists.

The main HTTP surfaces are:

```text
Authenticated administration:
  /api/lists
  /api/lists/*
  /api/pack/*

Public read-only publication:
  /files/<outputName>.json
  /files/<outputName>.json.zst
  /files/manifest.json
  /openapi.yaml

Existing read-only RPC:
  /rpc
```

Calling `POST /api/pack/<listKey>` republishes one list. Calling
`POST /api/pack/all` republishes every enabled list and atomically refreshes
`manifest.json`. A published file returns `404` until its list has been packed.

## Manual GitHub Action: Root `output/`

The manual workflow is:

```text
.github/workflows/build-managed-list-output.yml
```

Run **Build Managed List Output** from the Actions tab and enter only the public
domain, including its scheme but without a path, for example:

```text
https://assets.example.com
```

Every run deletes and rebuilds the fixed repository-root `output/` directory
from the currently committed large tokenlist, manual-token config, and homepage
list. It creates and commits:

```text
output/manifest.json
output/tokenlist.json
output/tokenlist.json.zst
output/usdt.json
output/usdt.json.zst
output/usdc.json
output/usdc.json.zst
output/usdg.json
output/usdg.json.zst
output/usds.json
output/usds.json.zst
output/stablecoin.json
output/stablecoin.json.zst
output/homepage.json
output/homepage.json.zst
output/eth.json
output/eth.json.zst
output/dai.json
output/dai.json.zst
output/support.json
output/support.json.zst
```

The workflow automatically includes any additional enabled default lists in the
future. It validates every JSON file, tests every Zstd file, checks required
lists, and verifies that manifest paths are portable. With the example domain,
manifest URLs are generated as:

```text
https://assets.example.com/output/usdt.json
https://assets.example.com/output/usdt.json.zst
```

The workflow then commits only changes under `output/` with the message
`chore: update managed list output` and pushes to the manually selected branch.
It does not call CoinGecko; run **Update JSON-RPC Data** first when the committed
large tokenlist itself needs refreshing, and run **Build Homepage Tokenlist**
first when homepage inputs changed.

The same generator can be run locally without starting the HTTP server:

```bash
cd extensions/jsonrpc
make pack-lists-once \
  PACK_LIST_ARGS="--managed-list-db /tmp/assets-lists.sqlite \
  --managed-list-files-dir ../../output \
  --managed-list-public-base-url https://assets.example.com/output"
```

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
manual_token_upsert
manual_token_delete
hot_replace_current
hot_add_current
hot_remove_current
hot_reset_current
```

Each config run updates the relevant config file, regenerates `tokenlist.json` and `tokenlist-report.json`, then commits both config and output.

`manual_token_upsert` accepts a final token object, array, or `{ "tokens": [...] }` wrapper, but only supports `kind="token"`. Manual native assets are intentionally rejected. `manual_token_delete` reads only `chain` and `address`.

## Security Notes

- Store `COINGECKO_API_KEY` in GitHub Secrets or a server-local environment file that is not committed.
- Do not commit generated `.env` files, API keys, private keys, wallet seed phrases, or personal paths.
- The JSON-RPC endpoint, `/files/`, and `/openapi.yaml` are read-only. Every `/api/lists*` and `/api/pack/*` operation is administrative and must not be exposed without authentication.
- The service intentionally delegates administrative authentication to Caddy. Bind the service to localhost with `--addr 127.0.0.1:8080`, protect the administrative paths with Caddy `basic_auth`, and expose the service through Caddy.
- The GitHub workflow only passes `COINGECKO_API_KEY` to the key-check and JSON generation steps, and automatic push runs are limited to `main` and `master`.

Example Caddy configuration:

```caddyfile
assets.example.com {
	@managed path /api/lists /api/lists/* /api/pack/*
	basic_auth @managed {
		admin $2a$14$REPLACE_WITH_CADDY_HASH_PASSWORD_OUTPUT
	}

	reverse_proxy 127.0.0.1:8080
}
```

Generate the password hash with `caddy hash-password`. Keep `/files/*` outside
the authenticated matcher when applications need to download packed JSON,
Zstandard files, and `manifest.json` without credentials. `/openapi.yaml` is
also read-only and can remain public for API clients and documentation tools.

Validate and reload Caddy after installing the configuration:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## systemd Example

Save the following unit as `/etc/systemd/system/assets-rpc.service`:

```ini
[Unit]
Description=Assets JSON-RPC sidecar
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=assets-rpc
Group=assets-rpc
WorkingDirectory=/srv/assets/extensions/jsonrpc
EnvironmentFile=/etc/assets-rpc/env
ExecStart=/srv/assets/bin/assets-rpc \
  --addr 127.0.0.1:8080 \
  --root /srv/assets \
  --market-sync-enabled=true \
  --market-sync-interval=6h \
  --market-cache /var/lib/assets-rpc/market.json \
  --tokenlist-cache /var/lib/assets-rpc/tokenlist.json \
  --tokenlist-report /var/lib/assets-rpc/report.json \
  --managed-list-db /var/lib/assets-rpc/lists.sqlite \
  --managed-list-files-dir /var/lib/assets-rpc/lists \
  --managed-list-seed-defaults=true \
  --managed-list-pack-after-seed=true
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/assets-rpc

[Install]
WantedBy=multi-user.target
```

Install and start the unit:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now assets-rpc
sudo systemctl status assets-rpc
sudo journalctl -u assets-rpc -n 100 --no-pager
```

## Container Pattern

Build the binary in a Go builder image, then run it with the assets repository mounted read-only plus a writable `data` directory:

```bash
docker run --rm \
  -p 127.0.0.1:8080:8080 \
  -e COINGECKO_API_KEY=xxx \
  -v /srv/assets:/srv/assets:ro \
  -v /srv/assets-cache:/cache \
  assets-rpc \
  --addr :8080 \
  --root /srv/assets \
  --market-cache /cache/market.json \
  --tokenlist-cache /cache/tokenlist.json \
  --tokenlist-report /cache/tokenlist-report.json \
  --managed-list-db /cache/lists.sqlite \
  --managed-list-files-dir /cache/lists \
  --managed-list-seed-defaults=true \
  --managed-list-pack-after-seed=true
```

The managed-list database and generated list files must use the writable cache
mount when the assets repository is mounted read-only. The loopback-only port
mapping is intended to be placed behind the authenticated Caddy configuration
above. Before the first long-running container start, run the same image once
with `--sync-once` and the `/cache` market/tokenlist/report paths so the large
tokenlist exists before managed-list seeding.

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

Before replacing the binary, back up the SQLite database and generated manifest.
The safest simple procedure is to stop the service briefly so the database copy
is consistent:

```bash
sudo systemctl stop assets-rpc
sudo cp -a /var/lib/assets-rpc/lists.sqlite /var/lib/assets-rpc/lists.sqlite.backup
sudo cp -a /var/lib/assets-rpc/lists/manifest.json /var/lib/assets-rpc/manifest.json.backup
sudo systemctl start assets-rpc
```

This release introduces the initial SQLite schema and intentionally contains no
legacy-schema compatibility layer. To roll back, stop the service, restore the
previous binary and database backup, then start it again. Generated JSON/Zstd
artifacts can always be rebuilt from SQLite with `POST /api/pack/all`.

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

For a complete managed-list deployment check through Caddy:

```bash
# OpenAPI is public.
curl -fsS https://assets.example.com/openapi.yaml >/dev/null

# Administrative reads and writes require Caddy credentials.
curl -fsS -u admin:password https://assets.example.com/api/lists >/dev/null
curl -fsS -u admin:password -X POST https://assets.example.com/api/pack/all

# Published files are public and have the expected formats.
curl -fsS https://assets.example.com/files/manifest.json | jq .
curl -fsS https://assets.example.com/files/usdt.json | jq '.key, (.items | length)'
curl -fsS https://assets.example.com/files/usdt.json.zst -o /tmp/usdt.json.zst
zstd -t /tmp/usdt.json.zst
```

Also verify that unauthenticated administrative access is rejected:

```bash
test "$(curl -sS -o /dev/null -w '%{http_code}' https://assets.example.com/api/lists)" = "401"
```

## Failure Behavior

- If CoinGecko sync fails, the service keeps serving the previous `market.json`.
- If `COINGECKO_API_KEY` is missing, market sync is skipped.
- If DefiLlama sync fails, tokenlist generation keeps using the previous `tokenlist.json` until the next successful sync.
- If caches do not exist yet, ranking methods return empty lists; local asset lookup still works.
