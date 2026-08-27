import { createFileRoute } from "@tanstack/react-router";
import { AlertCircle, Database, Search, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { AppFooter, AppHeader } from "@/components/AppChrome";
import { AssetCard } from "@/components/asset-card";
import { AssetDetailsDialog } from "@/components/asset-details-dialog";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  type AssetEntry,
  type AssetFilter,
  assetKey,
  assetName,
  assetSecondary,
  assetSymbol,
  createAssetEntries,
  fetchAssetData,
  formatDate,
} from "@/lib/assets";

export const Route = createFileRoute("/")({ component: Home });

const INITIAL_VISIBLE_COUNT = 96;
const SKELETON_KEYS = Array.from({ length: 24 }, (_, index) => `asset-skeleton-${index + 1}`);
const FILTERS: ReadonlyArray<{ label: string; value: AssetFilter }> = [
  { label: "全部", value: "all" },
  { label: "网络", value: "native" },
  { label: "Token", value: "token" },
  { label: "交易平台", value: "exchange" },
  { label: "钱包", value: "wallet" },
  { label: "DApp", value: "dapp" },
  { label: "兑换服务", value: "swap-provider" },
];

function Home() {
  const [filter, setFilter] = useState<AssetFilter>("all");
  const [query, setQuery] = useState("");
  const [visibleCount, setVisibleCount] = useState(INITIAL_VISIBLE_COUNT);
  const [data, setData] = useState<Awaited<ReturnType<typeof fetchAssetData>> | null>(null);
  const [selected, setSelected] = useState<AssetEntry | null>(null);
  const [error, setError] = useState<string>();
  useEffect(() => {
    const controller = new AbortController();
    setData(null);
    setError(undefined);
    fetchAssetData(controller.signal)
      .then(setData)
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) {
          setError(cause instanceof Error ? cause.message : "无法加载资产索引");
        }
      });
    return () => controller.abort();
  }, []);

  const entries = useMemo(() => createAssetEntries(data), [data]);
  const searchMatches = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    if (!normalizedQuery) return entries;
    return entries.filter((entry) =>
      [assetName(entry), assetSymbol(entry), assetSecondary(entry)]
        .join(" ")
        .toLocaleLowerCase()
        .includes(normalizedQuery),
    );
  }, [entries, query]);
  const counts = useMemo(
    () =>
      searchMatches.reduce<Record<AssetFilter, number>>(
        (result, entry) => {
          result.all += 1;
          result[entry.category] += 1;
          return result;
        },
        {
          all: 0,
          native: 0,
          token: 0,
          exchange: 0,
          wallet: 0,
          dapp: 0,
          "swap-provider": 0,
        },
      ),
    [searchMatches],
  );
  const filtered = useMemo(() => {
    return filter === "all"
      ? searchMatches
      : searchMatches.filter((entry) => entry.category === filter);
  }, [filter, searchMatches]);
  const hasMore = visibleCount < filtered.length;
  const loadMoreRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const target = loadMoreRef.current;
    if (!target || !hasMore) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          setVisibleCount((count) => Math.min(count + INITIAL_VISIBLE_COUNT, filtered.length));
        }
      },
      { rootMargin: "400px 0px" },
    );
    observer.observe(target);
    return () => observer.disconnect();
  }, [filtered.length, hasMore]);

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <AppHeader activePage="icons" />
      <main className="mx-auto w-full max-w-[1536px] flex-1 px-3 py-5 sm:px-6 sm:py-8 lg:px-8">
        <section className="mb-4 grid gap-4 pb-2 sm:mb-7 sm:gap-5 sm:pb-4 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
          <div className="max-w-3xl">
            <Badge className="mb-2.5" variant="outline">
              Asset registry
            </Badge>
            <h1 className="text-balance text-2xl font-semibold tracking-tight sm:text-4xl">
              加密资产图标，一处查找
            </h1>
            <p className="mt-3 max-w-2xl text-pretty text-sm leading-6 text-muted-foreground sm:text-base">
              浏览网络、Token、交易平台、钱包、DApp 与兑换服务。预览统一由 CryptoIdentity
              解析并支持网络角标。
            </p>
          </div>
          <div className="flex items-center gap-2.5 px-0 py-0 text-sm sm:gap-3 sm:rounded-xl sm:border sm:bg-card sm:px-4 sm:py-3 sm:shadow-xs">
            <div className="flex size-8 items-center justify-center rounded-lg bg-muted sm:size-auto sm:bg-transparent">
              <Database className="size-4 text-primary" />
            </div>
            <div>
              <div className="font-medium tabular-nums">
                {data ? entries.length.toLocaleString() : "—"} 项资产
              </div>
              <div className="text-xs text-muted-foreground">六类资产类型</div>
            </div>
          </div>
        </section>

        <div className="sticky top-16 z-30 -mx-3 mb-4 bg-background/95 px-3 py-3 backdrop-blur-xl sm:-mx-6 sm:mb-6 sm:px-6 sm:py-4 lg:-mx-8 lg:px-8">
          <div className="mx-auto flex max-w-[1472px] flex-col gap-2.5 lg:flex-row lg:items-center lg:gap-3">
            <Tabs
              className="min-w-0"
              value={filter}
              onValueChange={(value) => {
                setFilter(value as AssetFilter);
                setVisibleCount(INITIAL_VISIBLE_COUNT);
              }}
            >
              <TabsList className="grid h-auto w-full grid-cols-3 justify-start lg:inline-flex lg:w-fit lg:max-w-full lg:flex-wrap">
                {FILTERS.map((item) => (
                  <TabsTrigger
                    key={item.value}
                    className="h-10 px-2 lg:h-auto lg:px-3"
                    value={item.value}
                  >
                    {item.label}
                    <span className="font-mono text-[10px] text-muted-foreground">
                      {counts[item.value]}
                    </span>
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <div className="relative min-w-0 flex-1 lg:min-w-56">
              <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                aria-label="搜索资产"
                className="pl-9 pr-9"
                placeholder="搜索名称、符号、网络或服务"
                value={query}
                onChange={(event) => {
                  setQuery(event.target.value);
                  setVisibleCount(INITIAL_VISIBLE_COUNT);
                }}
              />
              {query ? (
                <Button
                  aria-label="清除搜索"
                  className="absolute right-1 top-1/2 -translate-y-1/2"
                  size="icon-sm"
                  variant="ghost"
                  onClick={() => {
                    setQuery("");
                    setVisibleCount(INITIAL_VISIBLE_COUNT);
                  }}
                >
                  <X />
                </Button>
              ) : null}
            </div>
          </div>
        </div>

        {error ? (
          <Alert variant="destructive">
            <AlertCircle />
            <AlertTitle>资产索引加载失败</AlertTitle>
            <AlertDescription>{error}。请刷新页面后重试。</AlertDescription>
          </Alert>
        ) : null}

        {!data && !error ? <AssetSkeletons /> : null}
        {data && filtered.length === 0 ? (
          <div className="rounded-xl border border-dashed py-20 text-center text-sm text-muted-foreground">
            没有符合当前条件的资产。
          </div>
        ) : null}
        {data && filtered.length > 0 ? (
          <>
            <div className="grid grid-cols-[repeat(auto-fill,minmax(136px,1fr))] gap-2.5 sm:gap-3">
              {filtered.slice(0, visibleCount).map((asset) => (
                <AssetCard
                  key={assetKey(asset)}
                  asset={asset}
                  onSelect={() => setSelected(asset)}
                />
              ))}
            </div>
            {hasMore ? (
              <div
                ref={loadMoreRef}
                aria-label={`正在加载更多，剩余 ${(filtered.length - visibleCount).toLocaleString()} 项`}
                className="mt-6 flex justify-center py-4"
                role="status"
              >
                <Skeleton className="h-2 w-24 rounded-full" />
              </div>
            ) : null}
          </>
        ) : null}
      </main>
      <AppFooter
        meta={data ? <span>索引更新于 {formatDate(data.tokens.updatedAt)}</span> : undefined}
      />
      <AssetDetailsDialog asset={selected} onOpenChange={(open) => !open && setSelected(null)} />
    </div>
  );
}

function AssetSkeletons() {
  return (
    <div
      aria-label="正在加载资产"
      className="grid grid-cols-[repeat(auto-fill,minmax(136px,1fr))] gap-2.5 sm:gap-3"
      role="status"
    >
      {SKELETON_KEYS.map((key) => (
        <div
          className="flex min-h-36 flex-col items-center rounded-xl border border-border bg-card p-3 sm:min-h-40 sm:p-4"
          key={key}
        >
          <Skeleton className="size-12 rounded-full sm:size-14" />
          <Skeleton className="mt-3 h-4 w-16" />
          <Skeleton className="mt-2 h-3 w-24" />
          <Skeleton className="mt-auto h-5 w-14" />
        </div>
      ))}
    </div>
  );
}
