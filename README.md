# GMWallet Assets Registry

Typed icon resolution and editable React components distributed through a shadcn Registry.

## Setup

Install directly without configuring a named Registry:

```bash
bunx shadcn@latest add https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/crypto-identity.json
```

For repeated installs, add the Registry to `components.json`:

```json
{
  "registries": {
    "@gmwallet": "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/{name}.json"
  }
}
```

Then install it by name:

```bash
bunx shadcn@latest add @gmwallet/crypto-identity
```

This command installs the component, its hooks and resolver library as one folder.

## Core resolver

The installed `@/components/ui/crypto-identity/lib` module exposes one resolver for six icon types. Compressed
catalogs are loaded on demand for every lookup. Tokens and networks use the complete token catalog,
exchanges and wallets use the support catalog, DApps use the DApp catalog, and swap providers use
the dedicated provider catalog. The resolver always reads the matched record's `logoURI`; it never
guesses a filename or extension.

```ts
import { resolveIconUrls } from "@/components/ui/crypto-identity/lib";

const tokenUrls = await resolveIconUrls({
  type: "token",
  name: "USDT",
  network: "ethereum",
});

const dappUrls = await resolveIconUrls({
  type: "dapp",
  name: "app.uniswap.org",
});

const providerUrls = await resolveIconUrls({
  type: "swap-provider",
  name: "1inch",
});
```

Supported types are `token`, `network`, `exchange`, `wallet`, `dapp`, and `swap-provider`. Network values use
repository directory keys such as `ethereum`, `smartchain`, and `tron`.

## Any technology stack

Applications that do not install the React component should download the relevant compressed catalog,
decompress it with Zstandard, match a record, and use its `logoURI` directly:

| Asset type | Catalog and field |
| --- | --- |
| Network | `extensions/jsonrpc/data/tokenlist.json.zst` → `tokens[kind=native].logoURI` |
| Token | `extensions/jsonrpc/data/tokenlist.json.zst` → `tokens[].logoURI` |
| Exchange | `support/support.json.zst` → `exchanges[].logoURI` |
| Wallet | `support/support.json.zst` → `wallets[].logoURI` |
| DApp | `support/dapps.json.zst` → `dapps[].logoURI` |
| Swap provider | `support/swap-providers.json.zst` → `providers[].logoURI` |

For example:

```bash
curl -sL https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/support.json.zst \
  | zstd -d -c \
  | jq -r '.wallets[] | select(.id == "metamask") | .logoURI'
```

Treat `logoURI` as the source of truth. Assets may be PNG, SVG, or WebP, so consumers should not
construct paths or assume an extension. The installed resolver also exports `resolveLogoUrls` to
convert a catalog URL into ordered CDN candidates while retaining the original URL as the final fallback.

### Direct image access

After obtaining `logoURI` from a catalog, the image can be loaded directly. These current examples
demonstrate that formats can differ:

| Asset type | Repository path format |
| --- | --- |
| Network | `blockchains/{network}/info/logo.png` |
| Token | `blockchains/{network}/assets/{address}/logo.png` |
| Exchange | `support/exchanges/{id}/logo.{ext}` |
| Wallet | `support/wallets/{id}/logo.{ext}` |
| DApp | `dapps/{domain}.png` |
| Swap provider | `support/swap-providers/{id}/logo.webp` |

For exchanges and wallets, obtain `{ext}` from `support.json.zst`; it can be PNG or SVG.

```text
https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/wallets/metamask/logo.svg
https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/swap-providers/1inch/logo.webp
```

```html
<img
  src="https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/wallets/metamask/logo.svg"
  alt="MetaMask"
/>
```

Use the URL returned by the catalog for the requested asset; the examples above are not path templates.
`canonicalLogoUrl` normalizes supported catalog sources to `https://cdn.jsdelivr.net`, while
`resolveLogoUrls` adds the preferred mirrors and retains the original source as the final fallback.

## React component

```tsx
import { CryptoIdentity } from "@/components/ui/crypto-identity";

export function CryptoIdentities() {
  return (
    <div className="flex items-center gap-4">
      <CryptoIdentity icon={{ type: "network", name: "ethereum" }} />

      <CryptoIdentity
        variant="label"
        icon={{ type: "token", name: "USDT", network: "ethereum" }}
        cornerIcon={{ type: "network", name: "ethereum" }}
      >
        Tether
      </CryptoIdentity>

      <CryptoIdentity variant="badge" icon={{ type: "wallet", name: "metamask" }}>
        MetaMask
      </CryptoIdentity>
    </div>
  );
}
```

`CryptoIdentity` supports `avatar` (default), `label`, and `badge` variants. Any of the six icon
types can also be used as a bottom-right `cornerIcon`.

While an icon is loading, its avatar uses a pulse skeleton. Once loaded, its dominant color is used
to derive separate high-contrast surfaces for light and dark themes. Pale icons receive a darker
light-theme surface, while dark monochrome icons remain legible in both themes.

Use `className` for the root element. Internal styling hooks are available through the
`crypto-identity-avatar`, `crypto-identity-image`, `crypto-identity-corner`, and
`crypto-identity-label` data slots.

### Registry mirrors

Use one registry mirror in `components.json`:

```text
https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/{name}.json
https://cdn.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
https://fastly.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
https://gcore.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
```

For a stable snapshot, replace `@package` with a Git commit SHA.

## Development

```bash
bun install
bun run dev
bun run check
bun run typecheck
bun run test
bun run build
bun run registry:build
```

The preview is a client-rendered Vite SPA configured for Cloudflare Workers Static Assets. Build
output is written to `dist`, and unknown routes return `index.html` so `/usage` and future client
routes can be opened directly.

For Cloudflare Workers Builds, use `bun run build` as the build command and `npx wrangler deploy`
as the deploy command.
