package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sparkdance/assets-jsonrpc/internal/rpcserver"
)

func main() {
	var config rpcserver.Config
	var syncOnce bool
	var syncTarget string

	coinGeckoBaseURL := rpcserver.CoinGeckoBaseURLFromEnv()
	if coinGeckoBaseURL == "" {
		coinGeckoBaseURL = rpcserver.DefaultCoinGeckoBaseURL
	}
	coinGeckoKeyHeader := rpcserver.CoinGeckoKeyHeaderFromEnv()
	if coinGeckoKeyHeader == "" {
		coinGeckoKeyHeader = rpcserver.DefaultCoinGeckoKeyHeader
	}
	defiLlamaBaseURL := rpcserver.DefiLlamaBaseURLFromEnv()
	if defiLlamaBaseURL == "" {
		defiLlamaBaseURL = rpcserver.DefaultDefiLlamaBaseURL
	}

	flag.StringVar(&config.Addr, "addr", ":8080", "JSON-RPC HTTP listen address")
	flag.StringVar(&config.Root, "root", "../..", "Trust Wallet assets repository root")
	flag.StringVar(&config.AssetBaseURL, "asset-base-url", rpcserver.DefaultAssetBaseURL, "base URL used for logoURI fields")
	flag.BoolVar(&config.MarketSyncEnabled, "market-sync-enabled", true, "enable background market and stablecoin sync")
	flag.DurationVar(&config.MarketSyncInterval, "market-sync-interval", 6*time.Hour, "background market sync interval")
	flag.StringVar(&config.MarketCachePath, "market-cache", rpcserver.DefaultMarketCachePath, "market ranking cache path, relative to --root unless absolute")
	flag.StringVar(&config.StablecoinCachePath, "stablecoin-cache", rpcserver.DefaultStablecoinCachePath, "stablecoin ranking cache path, relative to --root unless absolute")
	flag.StringVar(&config.TokenListCachePath, "tokenlist-cache", rpcserver.DefaultTokenListCachePath, "app tokenlist cache path, relative to --root unless absolute")
	flag.StringVar(&config.TokenListReportPath, "tokenlist-report", rpcserver.DefaultTokenListReportPath, "app tokenlist sync report path, relative to --root unless absolute")
	flag.StringVar(&config.TokenListRulesPath, "tokenlist-rules", rpcserver.DefaultTokenListRulesPath, "tokenlist rules path, relative to --root unless absolute")
	flag.StringVar(&config.VsCurrency, "coingecko-vs-currency", "usd", "CoinGecko quote currency")
	flag.StringVar(&config.CoinGeckoBaseURL, "coingecko-base-url", coinGeckoBaseURL, "CoinGecko API base URL")
	flag.StringVar(&config.CoinGeckoKeyHeader, "coingecko-api-key-header", coinGeckoKeyHeader, "CoinGecko API key header name")
	flag.StringVar(&config.DefiLlamaBaseURL, "defillama-base-url", defiLlamaBaseURL, "DefiLlama stablecoin API base URL")
	flag.IntVar(&config.MarketLimit, "market-limit", rpcserver.DefaultMarketLimit, "maximum CoinGecko market rows to sync")
	flag.IntVar(&config.TokenListMaxRank, "tokenlist-max-rank", rpcserver.DefaultTokenListMaxRank, "only include tokenlist assets with rank <= this value; 0 keeps all active assets")
	flag.BoolVar(&syncOnce, "sync-once", false, "sync market/stablecoin/tokenlist JSON caches and exit without starting HTTP")
	flag.StringVar(&syncTarget, "sync-target", string(rpcserver.SyncTargetAll), "sync-once target: all, market, stablecoins, or tokenlist")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := rpcserver.NewServer(config)
	if syncOnce {
		target, err := rpcserver.ParseSyncTarget(syncTarget)
		if err != nil {
			log.Fatalf("invalid --sync-target: %v", err)
		}
		if err := server.SyncOnce(ctx, target); err != nil {
			log.Fatalf("sync failed: %v", err)
		}
		return
	}

	if err := server.Start(ctx); err != nil {
		log.Fatalf("JSON-RPC server failed: %v", err)
	}
}
