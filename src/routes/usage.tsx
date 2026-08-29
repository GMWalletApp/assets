import { createFileRoute } from "@tanstack/react-router";
import { Check, Copy, Info, PackagePlus, Puzzle, Terminal } from "lucide-react";
import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
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
  ["network", "blockchains/{network}/info/logo.png"],
  ["token", "blockchains/{network}/assets/{address}/logo.png"],
  ["exchange", "support/exchanges/{id}/logo.{ext}"],
  ["wallet", "support/wallets/{id}/logo.{ext}"],
  ["dapp", "dapps/{domain}.png"],
  ["swapProvider", "support/swap-providers/{id}/logo.webp"],
] as const;
const CATALOG_FIELDS = [
  ["network", "tokenlist.json.zst → tokens[kind=native].logoURI"],
  ["token", "tokenlist.json.zst → tokens[].logoURI"],
  ["exchange", "support.json.zst → exchanges[].logoURI"],
  ["wallet", "support.json.zst → wallets[].logoURI"],
  ["dapp", "dapps.json.zst → dapps[].logoURI"],
  ["swapProvider", "swap-providers.json.zst → providers[].logoURI"],
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
const FEATURES = ["api", "background", "loading", "badge"] as const;

function UsagePage() {
  const { t } = useTranslation();

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <AppHeader activePage="usage" />
      <main className="mx-auto w-full max-w-5xl flex-1 px-3 py-5 sm:px-6 sm:py-8 lg:px-8">
        <section className="mb-6 max-w-3xl sm:mb-8">
          <Badge className="mb-3" variant="outline">
            {t("usage.badge")}
          </Badge>
          <h1 className="text-2xl font-semibold tracking-tight sm:text-4xl">{t("usage.title")}</h1>
          <p className="mt-3 text-pretty text-muted-foreground text-sm/6 sm:text-base/7">
            {t("usage.description")}
          </p>
        </section>

        <div className="grid min-w-0 gap-4 sm:gap-5">
          <Card>
            <CardHeader>
              <CardTitle>{t("usage.anyStackTitle")}</CardTitle>
              <CardDescription>{t("usage.anyStackDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs className="gap-4" defaultValue="direct">
                <TabsList className="grid w-full grid-cols-2 sm:w-fit">
                  <TabsTrigger value="direct">{t("usage.directTab")}</TabsTrigger>
                  <TabsTrigger value="catalog">{t("usage.catalogTab")}</TabsTrigger>
                </TabsList>
                <TabsContent className="flex flex-col gap-4" value="direct">
                  <ResourceGrid items={DIRECT_PATHS} />
                  <Alert>
                    <Info />
                    <AlertTitle>{t("usage.extensionTitle")}</AlertTitle>
                    <AlertDescription>{t("usage.extensionDescription")}</AlertDescription>
                  </Alert>
                  <div className="flex flex-col gap-2">
                    <div className="text-sm font-medium">{t("usage.directLinks")}</div>
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
                  <p className="text-muted-foreground text-sm/6">{t("usage.catalogDescription")}</p>
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
                  <CardTitle>{t("usage.directInstallTitle")}</CardTitle>
                  <CardDescription>{t("usage.directInstallDescription")}</CardDescription>
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
                  <CardTitle>{t("usage.namedRegistryTitle")}</CardTitle>
                  <CardDescription>{t("usage.namedRegistryDescription")}</CardDescription>
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
              <CardTitle>{t("usage.componentTitle")}</CardTitle>
              <CardDescription>{t("usage.componentDescription")}</CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs className="min-w-0" defaultValue="token">
                <TabsList className="h-auto w-full">
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="token">
                    {t("usage.resources.token")}
                  </TabsTrigger>
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="network">
                    {t("usage.networkTab")}
                  </TabsTrigger>
                  <TabsTrigger className="min-h-9 text-xs sm:text-sm" value="provider">
                    {t("usage.providersTab")}
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
              <CardTitle>{t("usage.featuresTitle")}</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3 sm:grid-cols-2">
              {FEATURES.map((feature) => (
                <div className="rounded-lg bg-muted/35 p-4" key={feature}>
                  <div className="font-medium">{t(`usage.features.${feature}Title`)}</div>
                  <p className="mt-1 text-muted-foreground text-sm/6">
                    {t(`usage.features.${feature}Description`)}
                  </p>
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
  const { t } = useTranslation();

  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {items.map(([title, path]) => (
        <div className="rounded-lg bg-muted/35 p-4" key={title}>
          <div className="text-sm font-medium">{t(`usage.resources.${title}`)}</div>
          <code className="mt-1 block break-all font-mono text-xs text-muted-foreground">
            {path}
          </code>
        </div>
      ))}
    </div>
  );
}

function CopyBlock({ value }: { value: string }) {
  const { t } = useTranslation();
  const { copied, copy } = useCopyFeedback(value);
  return (
    <div className="flex max-w-full min-w-0 items-center gap-2 overflow-hidden rounded-lg bg-muted/50 p-2 pl-3">
      <Terminal className="size-4 shrink-0 text-muted-foreground" />
      <code className="min-w-0 flex-1 overflow-x-auto whitespace-nowrap font-mono text-xs">
        {value}
      </code>
      <Button
        aria-label={t("actions.copyLinkOrCommand")}
        size="icon-sm"
        variant="ghost"
        onClick={copy}
      >
        {copied ? <Check data-icon="inline-start" /> : <Copy data-icon="inline-start" />}
      </Button>
    </div>
  );
}

function CodeBlock({ children }: { children: ReactNode }) {
  return (
    <pre className="max-w-full overflow-x-auto rounded-lg border bg-muted/50 p-3 font-mono text-foreground text-xs/6 sm:p-4">
      <code>{children}</code>
    </pre>
  );
}
