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

The installed `@/components/ui/crypto-identity/lib` module exposes one resolver for five icon types. Compressed
catalogs are loaded on demand: tokens use the complete token catalog, while exchange and wallet
names use the support catalog.

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
```

Supported types are `token`, `network`, `exchange`, `wallet`, and `dapp`. Network values use
repository directory keys such as `ethereum`, `smartchain`, and `tron`.

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

`CryptoIdentity` supports `avatar` (default), `label`, and `badge` variants. Any of the five icon
types can also be used as a bottom-right `cornerIcon`.

While an icon is loading, its avatar uses a pulse skeleton. Once loaded, its dominant color is
mixed with the active shadcn background color, so the result adapts automatically to light and dark
themes.

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
bun run check
bun run typecheck
bun run test
bun run registry:build
```
