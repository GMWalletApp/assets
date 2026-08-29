# GMWallet Assets Registry

[English](README.md) | **简体中文**

通过 shadcn Registry 分发的类型安全图标解析器与可编辑 React 组件。

## 安装

无需配置命名 Registry，可直接安装：

```bash
bunx shadcn@latest add https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/crypto-identity.json
```

如需重复安装，可将 Registry 添加到 `components.json`：

```json
{
  "registries": {
    "@gmwallet": "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/{name}.json"
  }
}
```

然后通过名称安装：

```bash
bunx shadcn@latest add @gmwallet/crypto-identity
```

该命令会将组件、相关 Hooks 和解析器库安装到同一个目录中。

## 核心解析器

安装后的 `@/components/ui/crypto-identity/lib` 模块提供一个支持六种图标类型的统一解析器。每次查询都会按需加载压缩目录。Token 和网络使用完整 Token 目录，交易平台和钱包使用 Support 目录，DApp 使用 DApp 目录，Swap 服务商使用专用服务商目录。解析器始终读取匹配记录中的 `logoURI`，不会猜测文件名或扩展名。

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

支持的类型包括 `token`、`network`、`exchange`、`wallet`、`dapp` 和 `swap-provider`。网络名称、显示名称、符号、仓库键和常见别名均由内部统一归一化。

目录匹配会经过规范化处理，并且不区分大小写：

| 资源类型 | 匹配字段 |
| --- | --- |
| Token | 在指定网络内优先使用 `contractAddress`；否则 `name` 可以匹配目录资源的 ID、地址、显示名称或 Symbol。Symbol 存在歧义时优先选择原生 Token。 |
| 网络 | `name` 可以匹配链键，或原生资源的 ID、显示名称或 Symbol。 |
| 交易平台、钱包、DApp、Swap 服务商 | `name` 可以匹配目录记录的 ID 或显示名称。 |

## 任意技术栈

不安装 React 组件的应用应下载对应的压缩目录，使用 Zstandard 解压，匹配记录并直接使用其 `logoURI`：

| 资源类型 | 目录与字段 |
| --- | --- |
| 网络 | `extensions/jsonrpc/data/tokenlist.json.zst` → `tokens[kind=native].logoURI` |
| Token | `extensions/jsonrpc/data/tokenlist.json.zst` → `tokens[].logoURI` |
| 交易平台 | `support/support.json.zst` → `exchanges[].logoURI` |
| 钱包 | `support/support.json.zst` → `wallets[].logoURI` |
| DApp | `support/dapps.json.zst` → `dapps[].logoURI` |
| Swap 服务商 | `support/swap-providers.json.zst` → `providers[].logoURI` |

例如：

```bash
curl -sL https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/support.json.zst \
  | zstd -d -c \
  | jq -r '.wallets[] | select(.id == "metamask") | .logoURI'
```

应将 `logoURI` 作为唯一可信来源。资源可能是 PNG、SVG 或 WebP，因此使用方不应自行构造路径或假设扩展名。安装后的解析器会提取其仓库相对路径，并映射为 `GMWalletApp/assets@main` 的有序镜像地址。

### 直接访问图片

从目录获取 `logoURI` 后即可直接加载图片。以下当前示例说明不同资源的格式可能不同：

| 资源类型 | 仓库路径格式 |
| --- | --- |
| 网络 | `blockchains/{network}/info/logo.png` |
| Token | `blockchains/{network}/assets/{address}/logo.png` |
| 交易平台 | `support/exchanges/{id}/logo.{ext}` |
| 钱包 | `support/wallets/{id}/logo.{ext}` |
| DApp | `dapps/{domain}.png` |
| Swap 服务商 | `support/swap-providers/{id}/logo.webp` |

交易平台和钱包的 `{ext}` 应从 `support.json.zst` 获取，它可能是 PNG 或 SVG。

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

应使用目录为所请求资源返回的 URL；以上示例并非路径模板。`canonicalLogoUrl` 会将支持的目录路径映射到配置的 jsDelivr 仓库，`resolveLogoUrls` 则按顺序返回配置仓库的镜像地址。

## React 组件

调用方只需提供业务语义上的 `type` 与 `name`。大小写、常见网络别名、空格、下划线、短横线、DApp 域名和可选的 `.png` 后缀均由解析器内部处理。

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

`CryptoIdentity` 支持 `avatar`（默认）、`label` 和 `badge` 三种变体。六种图标类型均可作为右下角的 `cornerIcon` 使用。

图标加载期间，头像会显示脉冲骨架屏。加载完成后，组件会根据图标主色为浅色和深色主题分别生成高对比度表面。浅色图标会在浅色主题中使用更深的表面，深色单色图标则在两种主题中都能保持清晰可见。

使用 `className` 设置根元素样式。内部样式钩子可通过 `crypto-identity-avatar`、`crypto-identity-image`、`crypto-identity-corner` 和 `crypto-identity-label` data slot 获取。

### Registry 镜像

在 `components.json` 中使用一个 Registry 镜像：

```text
https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/{name}.json
https://cdn.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
https://fastly.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
https://gcore.jsdelivr.net/gh/GMWalletApp/assets@package/registry/{name}.json
```

如需固定版本快照，请将 `@package` 替换为 Git commit SHA。

## 开发

```bash
bun install
bun run dev
bun run check
bun run tailwind:write
bun run typecheck
bun run test
bun run build
bun run registry:build
```

`bun run check` 也会根据 `src/styles.css` 检查静态 Tailwind 类名是否为规范形式。运行
`bun run tailwind:write` 可自动改写 JSX 属性，以及 `cn`、`clsx`、`cva` 等常用辅助函数中的
静态类名；包含动态插值的模板字符串会保留原样。CLI 使用仓库锁定版本的 Tailwind v4
设计系统 API。

预览站点是一个为 Cloudflare Workers Static Assets 配置的客户端渲染 Vite SPA。构建产物写入 `dist`，未知路由会返回 `index.html`，因此 `/usage` 和未来新增的客户端路由都可以直接打开。

使用 Cloudflare Workers Builds 时，将 `bun run build` 设为构建命令，将 `npx wrangler deploy` 设为部署命令。
