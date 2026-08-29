import { createFileRoute } from "@tanstack/react-router";
import { Check, Copy, Info, PackagePlus, Puzzle, Terminal } from "lucide-react";
import type { ReactNode } from "react";
import { AppFooter, AppHeader } from "@/components/AppChrome";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCopyFeedback } from "@/hooks/use-copy-feedback";
import { CATALOG_PATHS, CDN_BASE_URLS } from "../../registry/default/crypto-identity/lib/constants";

export const Route = createFileRoute("/usage")({ component: UsagePage });

const DIRECT_COMMAND =
  "bunx shadcn@latest add https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/crypto-identity.json";
const REGISTRY_COMMAND = "bunx shadcn@latest add @gmwallet/crypto-identity";
const DIRECT_PATHS = [
  ["网络", "blockchains/{network}/info/logo.png"],
  ["Token", "blockchains/{network}/assets/{address}/logo.png"],
  ["交易平台", "support/exchanges/{id}/logo.{ext}"],
  ["钱包", "support/wallets/{id}/logo.{ext}"],
  ["DApp", "dapps/{domain}.png"],
  ["兑换服务", "support/swap-providers/{id}/logo.webp"],
] as const;
const CATALOG_FIELDS = [
  ["网络", "tokenlist.json.zst → tokens[kind=native].logoURI"],
  ["Token", "tokenlist.json.zst → tokens[].logoURI"],
  ["交易平台", "support.json.zst → exchanges[].logoURI"],
  ["钱包", "support.json.zst → wallets[].logoURI"],
  ["DApp", "dapps.json.zst → dapps[].logoURI"],
  ["兑换服务", "swap-providers.json.zst → providers[].logoURI"],
] as const;
const METAMASK_LOGO =
  "https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/wallets/metamask/logo.svg";
const ONE_INCH_LOGO =
  "https://cdn.jsdelivr.net/gh/GMWalletApp/assets@main/support/swap-providers/1inch/logo.webp";
const CATALOG_URLS = {
  tokens: `${CDN_BASE_URLS[0]}/${CATALOG_PATHS.tokens}`,
  support: `${CDN_BASE_URLS[0]}/${CATALOG_PATHS.support}`,
  dapps: `${CDN_BASE_URLS[0]}/${CATALOG_PATHS.dapps}`,
  swapProviders: `${CDN_BASE_URLS[0]}/${CATALOG_PATHS.swapProviders}`,
} as const;

function UsagePage() {
  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <AppHeader activePage="usage" />
      <main className="mx-auto w-full max-w-5xl flex-1 px-3 py-5 sm:px-6 sm:py-8 lg:px-8">
        <section className="mb-6 max-w-3xl sm:mb-8">
          <Badge className="mb-3" variant="outline">
            shadcn registry
          </Badge>
          <h1 className="text-2xl font-semibold tracking-tight sm:text-4xl">安装 CryptoIdentity</h1>
          <p className="mt-3 text-pretty text-sm leading-6 text-muted-foreground sm:text-base sm:leading-7">
            一条命令安装完整文件夹。组件统一处理网络、Token、交易平台、钱包、DApp
            和兑换服务图标，并支持角标、加载骨架与自适应背景色。
          </p>
        </section>

        <div className="grid min-w-0 gap-4 sm:gap-5">
          <Card>
            <CardHeader>
              <CardTitle>任意技术栈</CardTitle>
              <CardDescription>
                原生 Web、Vue、Svelte、移动端或服务端均从压缩目录读取 logoURI，无需安装组件。
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs className="gap-4" defaultValue="direct">
                <TabsList className="grid w-full grid-cols-2 sm:w-fit">
                  <TabsTrigger value="direct">直接使用</TabsTrigger>
                  <TabsTrigger value="catalog">目录查询</TabsTrigger>
                </TabsList>
                <TabsContent className="flex flex-col gap-4" value="direct">
                  <ResourceGrid items={DIRECT_PATHS} />
                  <Alert>
                    <Info />
                    <AlertTitle>扩展名以目录记录为准</AlertTitle>
                    <AlertDescription>
                      钱包和交易平台可能是 PNG 或 SVG。先取得完整 logoURI，再直接加载或转换 CDN。
                    </AlertDescription>
                  </Alert>
                  <div className="flex flex-col gap-2">
                    <div className="text-sm font-medium">可复制的 jsDelivr 直链</div>
                    <CopyBlock value={METAMASK_LOGO} />
                    <CopyBlock value={ONE_INCH_LOGO} />
                  </div>
                  <CodeBlock>{`<img
  src="${METAMASK_LOGO}"
  alt="MetaMask"
/>`}</CodeBlock>
                </TabsContent>
                <TabsContent className="flex flex-col gap-4" value="catalog">
                  <ResourceGrid items={CATALOG_FIELDS} />
                  <p className="text-sm leading-6 text-muted-foreground">
                    下载并解压对应目录，按 chain、address、id 或 name 匹配记录后使用 logoURI。
                    组件会将支持的来源统一为 jsDelivr 地址，同时保留镜像回退。
                  </p>
                  <CodeBlock>{`curl -sL ${CATALOG_URLS.support} \\
  | zstd -d -c \\
  | jq -r '.wallets[] | select(.id == "metamask") | .logoURI'`}</CodeBlock>
                  <div className="grid gap-2">
                    {Object.values(CATALOG_URLS).map((url) => (
                      <CopyBlock key={url} value={url} />
                    ))}
                  </div>
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="flex size-9 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                  <PackagePlus className="size-4" />
                </div>
                <div>
                  <CardTitle>直接安装</CardTitle>
                  <CardDescription>
                    不配置 registries 也可以使用，默认走 cdn.jsdmirror.com。
                  </CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <CopyBlock value={DIRECT_COMMAND} />
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <div className="flex items-center gap-3">
                <div className="flex size-9 items-center justify-center rounded-lg bg-muted">
                  <Puzzle className="size-4" />
                </div>
                <div>
                  <CardTitle>命名 Registry</CardTitle>
                  <CardDescription>项目已经配置 @gmwallet 时，安装命令更短。</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent className="space-y-4">
              <CopyBlock value={REGISTRY_COMMAND} />
              <CodeBlock>{`{
  "registries": {
    "@gmwallet": "https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/{name}.json"
  }
}`}</CodeBlock>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>一个组件，覆盖常用身份</CardTitle>
              <CardDescription>传入 icon 描述即可；Token 可追加网络角标。</CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs className="min-w-0" defaultValue="token">
                <TabsList className="h-auto w-full">
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="token">
                    Token
                  </TabsTrigger>
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="network">
                    网络
                  </TabsTrigger>
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="provider">
                    平台与 DApp
                  </TabsTrigger>
                </TabsList>
                <TabsContent value="token">
                  <CodeBlock>{`<CryptoIdentity
  icon={{
    type: "token",
    name: "USDT",
    network: "ethereum",
    contractAddress: "0xdAC17F958D2ee523a2206206994597C13D831ec7",
  }}
  cornerIcon={{ type: "network", name: "ethereum" }}
/>`}</CodeBlock>
                </TabsContent>
                <TabsContent value="network">
                  <CodeBlock>{`<CryptoIdentity
  icon={{ type: "network", name: "ethereum" }}
/>`}</CodeBlock>
                </TabsContent>
                <TabsContent value="provider">
                  <CodeBlock>{`<CryptoIdentity icon={{ type: "exchange", name: "binance" }} />
<CryptoIdentity icon={{ type: "wallet", name: "metamask" }} />
<CryptoIdentity icon={{ type: "dapp", name: "app.uniswap.org" }} />
<CryptoIdentity icon={{ type: "swap-provider", name: "1inch" }} />`}</CodeBlock>
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>组件能力</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3 sm:grid-cols-2">
              {[
                [
                  "统一 API",
                  "network、token、exchange、wallet、dapp、swap-provider 使用同一组件。",
                ],
                ["智能背景", "根据图标内容生成背景色，并兼顾明暗主题对比度。"],
                ["加载反馈", "内置 Skeleton 状态，避免图片加载时布局跳动。"],
                ["可组合角标", "cornerIcon 可展示 Token 所属网络或其他身份。"],
              ].map(([title, description]) => (
                <div className="rounded-lg bg-muted/35 p-4" key={title}>
                  <div className="font-medium">{title}</div>
                  <p className="mt-1 text-sm leading-6 text-muted-foreground">{description}</p>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </main>
      <AppFooter />
    </div>
  );
}

function ResourceGrid({ items }: { items: ReadonlyArray<readonly [string, string]> }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {items.map(([title, path]) => (
        <div className="rounded-lg bg-muted/35 p-4" key={title}>
          <div className="text-sm font-medium">{title}</div>
          <code className="mt-1 block break-all font-mono text-xs text-muted-foreground">
            {path}
          </code>
        </div>
      ))}
    </div>
  );
}

function CopyBlock({ value }: { value: string }) {
  const { copied, copy } = useCopyFeedback(value);
  return (
    <div className="flex max-w-full min-w-0 items-center gap-2 overflow-hidden rounded-lg bg-muted/50 p-2 pl-3">
      <Terminal className="size-4 shrink-0 text-muted-foreground" />
      <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs">
        {value}
      </code>
      <Button aria-label="复制链接或命令" size="icon-sm" variant="ghost" onClick={copy}>
        {copied ? <Check data-icon="inline-start" /> : <Copy data-icon="inline-start" />}
      </Button>
    </div>
  );
}

function CodeBlock({ children }: { children: ReactNode }) {
  return (
    <pre className="max-w-full overflow-x-auto rounded-lg border bg-muted/50 p-3 font-mono text-xs leading-6 text-foreground sm:p-4">
      <code>{children}</code>
    </pre>
  );
}
