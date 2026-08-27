import { createFileRoute } from "@tanstack/react-router";
import { Check, Copy, PackagePlus, Puzzle, Terminal } from "lucide-react";
import type { ReactNode } from "react";
import { AppFooter, AppHeader } from "@/components/AppChrome";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCopyFeedback } from "@/hooks/use-copy-feedback";

export const Route = createFileRoute("/usage")({ component: UsagePage });

const DIRECT_COMMAND =
  "bunx shadcn@latest add https://cdn.jsdmirror.com/gh/GMWalletApp/assets@package/registry/crypto-identity.json";
const REGISTRY_COMMAND = "bunx shadcn@latest add @gmwallet/crypto-identity";

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
                原生 Web、Vue、Svelte、移动端或服务端都可以直接使用 CDN 资源，无需安装组件。
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                {[
                  ["网络", "blockchains/{network}/info/logo.png"],
                  ["Token", "blockchains/{network}/assets/{address}/logo.png"],
                  ["交易平台", "support/exchanges/{name}/logo.svg"],
                  ["钱包", "support/wallets/{name}/logo.svg"],
                  ["DApp", "dapps/{name}.png"],
                  ["兑换服务", "support/swap-providers/{name}/logo.webp"],
                ].map(([title, path]) => (
                  <div className="rounded-lg bg-muted/35 p-4" key={title}>
                    <div className="text-sm font-medium">{title}</div>
                    <code className="mt-1 block break-all font-mono text-xs text-muted-foreground">
                      {path}
                    </code>
                  </div>
                ))}
              </div>
              <CopyBlock value="https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/blockchains/ethereum/info/logo.png" />
              <p className="text-sm leading-6 text-muted-foreground">
                批量查找时，解压 tokenlist.json.zst、support.json.zst、dapps.json.zst 或
                swap-providers.json.zst，并优先使用记录中的 logoURI。切换镜像时只需替换 URL 的 CDN
                前缀。
              </p>
              <div className="grid gap-3">
                <CopyBlock value="https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/extensions/jsonrpc/data/tokenlist.json.zst" />
                <CopyBlock value="https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/support.json.zst" />
                <CopyBlock value="https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/dapps.json.zst" />
                <CopyBlock value="https://cdn.jsdmirror.com/gh/GMWalletApp/assets@main/support/swap-providers.json.zst" />
              </div>
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

function CopyBlock({ value }: { value: string }) {
  const { copied, copy } = useCopyFeedback(value);
  return (
    <div className="flex max-w-full min-w-0 items-center gap-2 overflow-hidden rounded-lg bg-muted/50 p-2 pl-3">
      <Terminal className="size-4 shrink-0 text-muted-foreground" />
      <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs">
        {value}
      </code>
      <Button aria-label="复制安装命令" size="icon-sm" variant="ghost" onClick={copy}>
        {copied ? <Check /> : <Copy />}
      </Button>
    </div>
  );
}

function CodeBlock({ children }: { children: ReactNode }) {
  return (
    <pre className="max-w-full overflow-x-auto rounded-lg bg-foreground p-3 text-xs leading-6 text-background sm:p-4">
      <code>{children}</code>
    </pre>
  );
}
