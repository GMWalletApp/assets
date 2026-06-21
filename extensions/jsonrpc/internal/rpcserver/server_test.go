package rpcserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testUSDTAddress = "0x55d398326f99059fF775485246999027B3197955"

func TestGetAssetByAddressReturnsFullAssetDetail(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:              root,
		AssetBaseURL:      "https://cdn.example",
		MarketSyncEnabled: false,
	})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAssetByAddress",
		"params": map[string]any{
			"chain":   "smartchain",
			"address": testUSDTAddress,
		},
	}, &response)

	if response.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", response.Error)
	}

	var detail AssetDetail
	mustRemarshal(t, response.Result, &detail)

	if detail.Chain != "smartchain" {
		t.Fatalf("unexpected chain: %s", detail.Chain)
	}
	if detail.Address != testUSDTAddress {
		t.Fatalf("unexpected address: %s", detail.Address)
	}
	if detail.AssetID != "c20000714_t"+testUSDTAddress {
		t.Fatalf("unexpected asset id: %s", detail.AssetID)
	}
	if detail.Decimals != 18 {
		t.Fatalf("unexpected decimals: %d", detail.Decimals)
	}
	if detail.LogoURI == "" || !detail.LogoExists {
		t.Fatalf("expected logo data, got uri=%q exists=%v", detail.LogoURI, detail.LogoExists)
	}
	if detail.Website == "" || detail.Explorer == "" {
		t.Fatalf("expected website and explorer in detail: %+v", detail)
	}
}

func TestGetAssetByIDReturnsNativeAssetDetail(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:              root,
		AssetBaseURL:      "https://cdn.example",
		MarketSyncEnabled: false,
	})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAssetById",
		"params":  map[string]any{"assetId": "c20000714"},
	}, &response)

	if response.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", response.Error)
	}

	var detail AssetDetail
	mustRemarshal(t, response.Result, &detail)
	if detail.Chain != "smartchain" || detail.Address != "" || detail.Symbol != "BNB" || detail.AssetID != "c20000714" {
		t.Fatalf("unexpected native asset detail: %+v", detail)
	}
}

func TestMarketRankingsReturnEmbeddedAssetDetails(t *testing.T) {
	root := newFixtureRoot(t)
	detail := mustAssetDetail(t, root)
	mustWriteJSON(t, filepath.Join(root, DefaultMarketCachePath), MarketCache{
		Source:     "coingecko",
		VsCurrency: "usd",
		UpdatedAt:  "2026-06-20T00:00:00Z",
		Assets: []MarketAsset{
			{
				Rank:          3,
				Source:        "coingecko",
				CoinGeckoID:   "tether",
				Symbol:        "USDT",
				Name:          "Tether",
				MarketCapRank: 3,
				MarketCap:     100,
				TotalVolume:   50,
				CurrentPrice:  1,
				Assets:        []AssetDetail{detail},
			},
		},
	})

	server := NewServer(Config{Root: root, AssetBaseURL: "https://cdn.example"})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getMarketRankings",
		"params":  map[string]any{"limit": 10, "onlyWithAssets": true},
	}, &response)

	if response.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", response.Error)
	}

	var rankings []MarketAsset
	mustRemarshal(t, response.Result, &rankings)
	if len(rankings) != 1 {
		t.Fatalf("expected one ranking, got %d", len(rankings))
	}
	if len(rankings[0].Assets) != 1 {
		t.Fatalf("expected embedded asset detail, got %+v", rankings[0])
	}
	if rankings[0].Assets[0].Decimals != 18 || rankings[0].Assets[0].LogoURI == "" {
		t.Fatalf("expected full embedded asset detail, got %+v", rankings[0].Assets[0])
	}
}

func TestStablecoinRankingsReturnEmbeddedAssetDetails(t *testing.T) {
	root := newFixtureRoot(t)
	detail := mustAssetDetail(t, root)
	mustWriteJSON(t, filepath.Join(root, DefaultStablecoinCachePath), StablecoinCache{
		Source:    "defillama",
		UpdatedAt: "2026-06-20T00:00:00Z",
		Assets: []StablecoinAsset{
			{
				Rank:        1,
				Source:      "defillama",
				DefiLlamaID: "1",
				Symbol:      "USDT",
				Name:        "Tether",
				Circulating: 100,
				Chains:      []string{"BSC"},
				PegType:     "peggedUSD",
				Assets:      []AssetDetail{detail},
			},
		},
	})

	server := NewServer(Config{Root: root, AssetBaseURL: "https://cdn.example"})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getStablecoinRankings",
		"params":  map[string]any{"limit": 10, "onlyWithAssets": true},
	}, &response)

	if response.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", response.Error)
	}

	var rankings []StablecoinAsset
	mustRemarshal(t, response.Result, &rankings)
	if len(rankings) != 1 || len(rankings[0].Assets) != 1 {
		t.Fatalf("expected embedded asset detail, got %+v", rankings)
	}
	if rankings[0].Assets[0].Explorer == "" || rankings[0].Assets[0].Decimals != 18 {
		t.Fatalf("expected full asset detail, got %+v", rankings[0].Assets[0])
	}
}

func TestGetAppTokenListSupportsRuntimePaginationAndRankFilter(t *testing.T) {
	root := newFixtureRoot(t)
	tokenListPath := filepath.Join(root, "data", "tokenlist.json")
	mustWriteJSON(t, tokenListPath, AppTokenList{
		Source:    "trustwallet+coingecko",
		UpdatedAt: "2026-06-20T00:00:00Z",
		Tokens: []AppToken{
			{Kind: "token", Chain: "smartchain", Address: testUSDTAddress, AssetID: "c20000714_t" + testUSDTAddress, Symbol: "USDT", Rank: 3, Market: &AppTokenMarket{CoinGeckoID: "tether", MarketCapRank: 3}},
			{Kind: "native", Chain: "smartchain", AssetID: "c20000714", Symbol: "BNB", Rank: 4, Market: &AppTokenMarket{CoinGeckoID: "binancecoin", MarketCapRank: 4}},
			{Kind: "token", Chain: "ethereum", Address: "0x0000000000000000000000000000000000000001", AssetID: "c60_t0x0000000000000000000000000000000000000001", Symbol: "NOPE", Rank: 200, Market: &AppTokenMarket{CoinGeckoID: "nope", MarketCapRank: 200}},
		},
	})
	server := NewServer(Config{
		Root:               root,
		AssetBaseURL:       "https://cdn.example",
		TokenListCachePath: tokenListPath,
	})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAppTokenList",
		"params":  map[string]any{"limit": 1, "offset": 1, "maxRank": 100, "onlyWithMarket": true},
	}, &response)

	if response.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", response.Error)
	}

	var result AppTokenList
	mustRemarshal(t, response.Result, &result)
	if len(result.Tokens) != 1 || result.Tokens[0].Symbol != "BNB" {
		t.Fatalf("expected second top-100 app token by runtime pagination, got %+v", result.Tokens)
	}
}

func TestUnknownMethodReturnsJSONRPCError(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{Root: root, AssetBaseURL: "https://cdn.example"})

	var response rpcResponse
	doRPC(t, server, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "unknown",
		"params":  map[string]any{},
	}, &response)

	if response.Error == nil {
		t.Fatal("expected rpc error")
	}
	if response.Error.Code != ErrCodeMethodNotFound {
		t.Fatalf("unexpected error code: %d", response.Error.Code)
	}
}

func TestOversizedRequestReturnsJSONRPCError(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{Root: root, AssetBaseURL: "https://cdn.example"})

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(bytes.Repeat([]byte(" "), maxRequestBodyBytes+1)))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}

	var response rpcResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	if response.Error == nil {
		t.Fatal("expected rpc error")
	}
}

func TestSyncerWritesMarketAndStablecoinCachesWithAssetDetails(t *testing.T) {
	root := newFixtureRoot(t)
	spamAddress := "0x00000000000000000000000000000000000000aa"
	spamDir := filepath.Join(root, "blockchains", "smartchain", "assets", spamAddress)
	if err := os.MkdirAll(spamDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(spamDir, "info.json"), map[string]any{
		"name":     "Spam Token",
		"type":     "BEP20",
		"symbol":   "SPAM",
		"decimals": 18,
		"status":   "spam",
		"id":       spamAddress,
	})

	noLogoAddress := "0x00000000000000000000000000000000000000bb"
	noLogoDir := filepath.Join(root, "blockchains", "smartchain", "assets", noLogoAddress)
	if err := os.MkdirAll(noLogoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(noLogoDir, "info.json"), map[string]any{
		"name":     "No Logo Token",
		"type":     "BEP20",
		"symbol":   "NLG",
		"decimals": 18,
		"status":   "active",
		"id":       noLogoAddress,
	})

	rulesPath := filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json")
	mustWriteJSON(t, rulesPath, TokenListRules{
		AssetOverrides: []TokenListAssetOverride{
			{
				Chain:       "smartchain",
				Address:     testUSDTAddress,
				CoinGeckoID: "tether",
				DisplayName: "Binance-Peg Tether USD",
				AddTags:     []string{"binance-peg"},
			},
		},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		Enabled:             true,
		MarketCachePath:     filepath.Join(root, "data", "market.json"),
		StablecoinCachePath: filepath.Join(root, "data", "stablecoins.json"),
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:  rulesPath,
		VsCurrency:          "usd",
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		CoinGeckoKeyHeader:  "x-test-key",
		DefiLlamaBaseURL:    "https://defillama.test",
		MarketLimit:         100,
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			if got := r.Header.Get("x-test-key"); got != "test-key" {
				t.Fatalf("unexpected coingecko api key header: %q", got)
			}
			return jsonResponse(`[{
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"current_price": 1,
				"market_cap": 100,
				"market_cap_rank": 3,
				"total_volume": 50,
				"last_updated": "2026-06-20T00:00:00Z"
			}, {
				"id": "fake-usdt",
				"symbol": "usdt",
				"name": "Fake USDT",
				"current_price": 1,
				"market_cap": 1,
				"market_cap_rank": 999,
				"total_volume": 1,
				"last_updated": "2026-06-20T00:00:00Z"
			}, {
				"id": "binancecoin",
				"symbol": "bnb",
				"name": "BNB",
				"current_price": 500,
				"market_cap": 5000,
				"market_cap_rank": 4,
				"total_volume": 500,
				"last_updated": "2026-06-20T00:00:00Z"
			}]`), nil
		case "coingecko.test/coins/list":
			if got := r.Header.Get("x-test-key"); got != "test-key" {
				t.Fatalf("unexpected coingecko api key header: %q", got)
			}
			if got := r.URL.Query().Get("include_platform"); got != "true" {
				t.Fatalf("expected include_platform=true, got %q", got)
			}
			return jsonResponse(`[{
					"id": "tether",
					"symbol": "usdt",
					"name": "Tether",
					"platforms": {}
				}, {
					"id": "fake-usdt",
					"symbol": "usdt",
				"name": "Fake USDT",
				"platforms": {
					"binance-smart-chain": "0x0000000000000000000000000000000000000001"
				}
			}]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{
				"peggedAssets": [{
					"id": 1,
					"gecko_id": "tether",
					"name": "Tether",
					"symbol": "USDT",
					"pegType": "peggedUSD",
					"priceSource": "defillama",
					"circulating": {"peggedUSD": 100},
					"chainCirculating": {"BSC": {}}
				}]
			}`), nil
		default:
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(bytes.NewReader(nil)),
				Header:     make(http.Header),
			}, nil
		}
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := syncer.SyncMarket(ctx); err != nil {
		t.Fatalf("sync market: %v", err)
	}
	if err := syncer.SyncStablecoins(ctx); err != nil {
		t.Fatalf("sync stablecoins: %v", err)
	}
	if err := syncer.SyncTokenList(ctx); err != nil {
		t.Fatalf("sync tokenlist: %v", err)
	}

	cache := NewCacheStore(filepath.Join(root, "data", "market.json"), filepath.Join(root, "data", "stablecoins.json"))
	market, err := cache.ReadMarket()
	if err != nil {
		t.Fatalf("read market cache: %v", err)
	}
	if len(market.Assets) != 3 || len(market.Assets[0].Assets) != 1 {
		t.Fatalf("expected synced market asset details, got %+v", market)
	}
	if len(market.Assets[1].Assets) != 0 {
		t.Fatalf("expected no symbol fallback for unmatched platform address, got %+v", market.Assets[1])
	}
	if len(market.Assets[2].Assets) != 1 || market.Assets[2].Assets[0].Address != "" {
		t.Fatalf("expected native BNB market asset, got %+v", market.Assets[2])
	}
	if market.Assets[0].Assets[0].Name != "Binance-Peg Tether USD" || !hasTag(market.Assets[0].Assets[0].Tags, "binance-peg") {
		t.Fatalf("expected market embedded asset rules, got %+v", market.Assets[0].Assets[0])
	}

	stablecoins, err := cache.ReadStablecoins()
	if err != nil {
		t.Fatalf("read stablecoin cache: %v", err)
	}
	if len(stablecoins.Assets) != 1 || len(stablecoins.Assets[0].Assets) != 1 {
		t.Fatalf("expected synced stablecoin asset details, got %+v", stablecoins)
	}
	if stablecoins.Assets[0].Assets[0].Name != "Binance-Peg Tether USD" || !hasTag(stablecoins.Assets[0].Assets[0].Tags, "binance-peg") {
		t.Fatalf("expected stablecoin embedded asset rules, got %+v", stablecoins.Assets[0].Assets[0])
	}

	var tokenList AppTokenList
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist.json"), &tokenList); err != nil {
		t.Fatalf("read tokenlist: %v", err)
	}
	if len(tokenList.Tokens) != 3 {
		t.Fatalf("expected native BNB and USDT token, got %+v", tokenList.Tokens)
	}
	var nativeBNB, usdt *AppToken
	for i := range tokenList.Tokens {
		token := &tokenList.Tokens[i]
		if token.Kind == "native" && token.Chain == "smartchain" {
			nativeBNB = token
		}
		if token.Kind == "token" && token.Address == testUSDTAddress {
			usdt = token
		}
	}
	if nativeBNB == nil || nativeBNB.Market == nil || nativeBNB.Market.CoinGeckoID != "binancecoin" || nativeBNB.Address != "" {
		t.Fatalf("expected native BNB with market data, got %+v", nativeBNB)
	}
	if usdt == nil || usdt.Market == nil || usdt.Market.CoinGeckoID != "tether" {
		t.Fatalf("expected USDT with market data from local external link, got %+v", usdt)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Source != "trustwallet+coingecko" || report.Local.OutputTokens != 3 || report.Local.Filtered != 1 || report.Local.MissingLogos != 1 || report.Market.NativeMatches != 1 || report.Market.TokenMatches != 1 {
		t.Fatalf("unexpected tokenlist report: %+v", report)
	}
}

func TestMarketLimitControlsCoinGeckoPageSize(t *testing.T) {
	root := newFixtureRoot(t)
	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		MarketCachePath:  filepath.Join(root, "data", "market.json"),
		CoinGeckoAPIKey:  "test-key",
		CoinGeckoBaseURL: "https://coingecko.test",
		MarketLimit:      100,
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			if got := r.URL.Query().Get("per_page"); got != "100" {
				t.Fatalf("expected per_page=100, got %q", got)
			}
			if got := r.URL.Query().Get("page"); got != "1" {
				t.Fatalf("expected page=1, got %q", got)
			}
			return jsonResponse(`[{
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"market_cap_rank": 3
			}]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[{"id": "tether", "symbol": "usdt", "name": "Tether", "platforms": {}}]`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := syncer.SyncMarket(ctx); err != nil {
		t.Fatalf("sync market: %v", err)
	}
}

func TestTokenListMaxRankFiltersOutput(t *testing.T) {
	root := newFixtureRoot(t)
	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		TokenListMaxRank:    3,
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[{
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"market_cap_rank": 3
			}, {
				"id": "binancecoin",
				"symbol": "bnb",
				"name": "BNB",
				"market_cap_rank": 4
			}]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[{"id": "tether", "symbol": "usdt", "name": "Tether", "platforms": {}}]`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := syncer.SyncTokenList(ctx); err != nil {
		t.Fatalf("sync tokenlist: %v", err)
	}

	var tokenList AppTokenList
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist.json"), &tokenList); err != nil {
		t.Fatalf("read tokenlist: %v", err)
	}
	if len(tokenList.Tokens) != 1 || tokenList.Tokens[0].Address != testUSDTAddress || tokenList.Tokens[0].Rank != 3 {
		t.Fatalf("expected only rank<=3 USDT token, got %+v", tokenList.Tokens)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Local.RankFiltered == 0 {
		t.Fatalf("expected rank filtered count, got %+v", report.Local)
	}
}

func TestTokenListRulesApplyOnlyToGeneratedTokenList(t *testing.T) {
	root := newFixtureRoot(t)
	const plasmaUSDe = "0x5d3a1Ff2b6BAb83b63cd9AD0787074081a52ef34"
	addNativeChain(t, root, "base", map[string]any{
		"name":     "Base",
		"symbol":   "ETH",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})
	addAsset(t, root, "plasma", plasmaUSDe, map[string]any{
		"name":     "USDe",
		"type":     "PLASMA",
		"symbol":   "USDe",
		"decimals": 18,
		"status":   "active",
		"id":       plasmaUSDe,
		"tags":     []string{"stablecoin"},
	})

	rulesPath := filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json")
	mustWriteJSON(t, rulesPath, TokenListRules{
		PlatformMappings: map[string]string{
			"plasma": "plasma",
		},
		NativeMarketMappings: map[string][]string{
			"ethereum": []string{"base"},
		},
		AssetOverrides: []TokenListAssetOverride{
			{
				Chain:       "smartchain",
				Address:     testUSDTAddress,
				CoinGeckoID: "tether",
				DisplayName: "Binance-Peg Tether USD",
				AddTags:     []string{"stablecoin", "binance-peg"},
			},
			{
				Chain:       "plasma",
				Address:     plasmaUSDe,
				CoinGeckoID: "ethena-usde",
				AddTags:     []string{"defi"},
			},
		},
		MarketTagRules: []TokenListMarketTagRule{
			{CoinGeckoID: "tether", AddTags: []string{"stablecoin"}},
			{CoinGeckoID: "ethena-usde", AddTags: []string{"stablecoin", "defi"}},
		},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:  rulesPath,
		VsCurrency:          "usd",
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		CoinGeckoKeyHeader:  "x-test-key",
		MarketLimit:         100,
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[{
				"id": "ethereum",
				"symbol": "eth",
				"name": "Ethereum",
				"market_cap_rank": 2
			}, {
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"market_cap_rank": 3
			}, {
				"id": "ethena-usde",
				"symbol": "usde",
				"name": "Ethena USDe",
				"market_cap_rank": 23
			}]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[
				{"id": "ethereum", "symbol": "eth", "name": "Ethereum", "platforms": {}},
				{"id": "tether", "symbol": "usdt", "name": "Tether", "platforms": {}},
				{"id": "ethena-usde", "symbol": "usde", "name": "Ethena USDe", "platforms": {
					"plasma": "` + plasmaUSDe + `"
				}}
			]`), nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := syncer.SyncTokenList(ctx); err != nil {
		t.Fatalf("sync tokenlist: %v", err)
	}

	var tokenList AppTokenList
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist.json"), &tokenList); err != nil {
		t.Fatalf("read tokenlist: %v", err)
	}
	usdt := findAppToken(tokenList.Tokens, "smartchain", testUSDTAddress)
	if usdt == nil || usdt.Name != "Binance-Peg Tether USD" || !hasTag(usdt.Tags, "binance-peg") || usdt.Decimals != 18 {
		t.Fatalf("expected generated BSC USDT override without immutable field changes, got %+v", usdt)
	}
	baseNative := findAppToken(tokenList.Tokens, "base", "")
	if baseNative == nil || baseNative.Market == nil || baseNative.Market.CoinGeckoID != "ethereum" || baseNative.Rank != 2 {
		t.Fatalf("expected base native ETH to inherit ethereum rank, got %+v", baseNative)
	}
	usde := findAppToken(tokenList.Tokens, "plasma", plasmaUSDe)
	if usde == nil || usde.Market == nil || usde.Market.CoinGeckoID != "ethena-usde" || !hasTag(usde.Tags, "defi") {
		t.Fatalf("expected plasma USDe platform mapping and tags, got %+v", usde)
	}

	sourceDetail, err := NewStore(root, "https://cdn.example").GetAssetByAddress("smartchain", testUSDTAddress)
	if err != nil {
		t.Fatal(err)
	}
	if sourceDetail.Name != "Tether USD" || hasTag(sourceDetail.Tags, "binance-peg") {
		t.Fatalf("rules should not modify source asset detail, got %+v", sourceDetail)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Rules.ConfiguredPlatformMappings != 1 || report.Rules.ConfiguredAssetOverrides != 2 || report.Rules.PlatformMappingHits != 1 || report.Rules.NativeMarketMappingHits != 1 {
		t.Fatalf("unexpected rule stats: %+v", report.Rules)
	}
	if len(report.Issues.RuleIssues) != 0 {
		t.Fatalf("unexpected rule issues: %+v", report.Issues.RuleIssues)
	}
}

func TestExternalMarketLinkParsing(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		cg   string
		cmc  string
	}{
		{
			name: "coingecko localized",
			raw:  "https://www.coingecko.com/en/coins/tether/",
			cg:   "tether",
		},
		{
			name: "coingecko direct",
			raw:  "https://coingecko.com/coins/usd-coin/",
			cg:   "usd-coin",
		},
		{
			name: "coinmarketcap localized",
			raw:  "https://coinmarketcap.com/ru/currencies/tether/",
			cmc:  "tether",
		},
		{
			name: "coinmarketcap direct",
			raw:  "https://coinmarketcap.com/currencies/usd-coin/",
			cmc:  "usd-coin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := coinGeckoIDFromURL(tt.raw); got != tt.cg {
				t.Fatalf("coingecko id = %q, want %q", got, tt.cg)
			}
			if got := coinMarketCapIDFromURL(tt.raw); got != tt.cmc {
				t.Fatalf("coinmarketcap id = %q, want %q", got, tt.cmc)
			}
		})
	}
}

func TestTokenListRulesCanSuppressBuiltInNativeMapping(t *testing.T) {
	rules := &TokenListRules{
		NativeMarketMappings: map[string][]string{
			"arbitrum": []string{},
		},
	}
	rules.normalize()

	chains, usedRule := coinGeckoNativeChainsWithRules("arbitrum", rules)
	if !usedRule {
		t.Fatal("expected explicit native rule to override built-in mapping")
	}
	if len(chains) != 0 {
		t.Fatalf("expected empty native mapping override, got %v", chains)
	}
}

func TestParseSyncTarget(t *testing.T) {
	tests := map[string]SyncTarget{
		"":            SyncTargetAll,
		"all":         SyncTargetAll,
		"market":      SyncTargetMarket,
		"stablecoins": SyncTargetStablecoins,
		"tokenlist":   SyncTargetTokenList,
		" MARKET ":    SyncTargetMarket,
	}

	for input, want := range tests {
		got, err := ParseSyncTarget(input)
		if err != nil {
			t.Fatalf("parse sync target %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("parse sync target %q: got %q want %q", input, got, want)
		}
	}

	if _, err := ParseSyncTarget("prices"); err == nil {
		t.Fatal("expected invalid sync target error")
	}
}

func TestSyncOnceTargetSkipsUnselectedCache(t *testing.T) {
	root := newFixtureRoot(t)
	marketPath := filepath.Join(root, "data", "market.json")
	stablecoinPath := filepath.Join(root, "data", "stablecoins.json")
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		MarketCachePath:     marketPath,
		StablecoinCachePath: stablecoinPath,
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		DefiLlamaBaseURL:    "https://defillama.test",
		MarketLimit:         100,
	})
	server.syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			t.Fatalf("unexpected market sync request")
		case "coingecko.test/coins/list":
			return jsonResponse(`[{
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"platforms": {
					"binance-smart-chain": "` + testUSDTAddress + `"
				}
			}]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{
				"peggedAssets": [{
					"id": 1,
					"gecko_id": "tether",
					"name": "Tether",
					"symbol": "USDT",
					"pegType": "peggedUSD",
					"priceSource": "defillama",
					"circulating": {"peggedUSD": 100},
					"chainCirculating": {"BSC": {}}
				}]
			}`), nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.SyncOnce(ctx, SyncTargetStablecoins); err != nil {
		t.Fatalf("sync once stablecoins: %v", err)
	}
	if fileExists(marketPath) {
		t.Fatal("market cache should not be written for stablecoin-only sync")
	}
	if !fileExists(stablecoinPath) {
		t.Fatal("stablecoin cache should be written")
	}
}

func newFixtureRoot(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	assetDir := filepath.Join(root, "blockchains", "smartchain", "assets", testUSDTAddress)
	chainInfoDir := filepath.Join(root, "blockchains", "smartchain", "info")
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(chainInfoDir, 0o755); err != nil {
		t.Fatal(err)
	}

	mustWriteJSON(t, filepath.Join(chainInfoDir, "info.json"), map[string]any{
		"name":        "BNB Smart Chain",
		"symbol":      "BNB",
		"type":        "coin",
		"decimals":    18,
		"description": "BNB Smart Chain",
		"website":     "https://bnbchain.org",
		"explorer":    "https://bscscan.com",
		"status":      "active",
	})
	if err := os.WriteFile(filepath.Join(chainInfoDir, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	mustWriteJSON(t, filepath.Join(assetDir, "info.json"), map[string]any{
		"name":        "Tether USD",
		"website":     "https://tether.to",
		"description": "Tether gives you the joint benefits of open blockchain technology and traditional currency.",
		"explorer":    "https://bscscan.com/token/" + testUSDTAddress,
		"type":        "BEP20",
		"symbol":      "USDT",
		"decimals":    18,
		"status":      "active",
		"id":          testUSDTAddress,
		"tags":        []string{"stablecoin"},
		"links": []map[string]string{
			{"name": "coingecko", "url": "https://coingecko.com/en/coins/tether/"},
		},
	})
	if err := os.WriteFile(filepath.Join(assetDir, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}

	mustWriteJSON(t, filepath.Join(root, "blockchains", "smartchain", "tokenlist.json"), map[string]any{
		"name": "Trust Wallet: Smartchain",
		"tokens": []map[string]any{
			{"chainId": 56, "address": testUSDTAddress, "symbol": "USDT"},
		},
	})

	return root
}

func addNativeChain(t *testing.T, root, chain string, info map[string]any) {
	t.Helper()

	infoDir := filepath.Join(root, "blockchains", chain, "info")
	if err := os.MkdirAll(infoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(infoDir, "info.json"), info)
	if err := os.WriteFile(filepath.Join(infoDir, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func addAsset(t *testing.T, root, chain, address string, info map[string]any) {
	t.Helper()

	assetDir := filepath.Join(root, "blockchains", chain, "assets", address)
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteJSON(t, filepath.Join(assetDir, "info.json"), info)
	if err := os.WriteFile(filepath.Join(assetDir, "logo.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findAppToken(tokens []AppToken, chain, address string) *AppToken {
	for i := range tokens {
		if strings.EqualFold(tokens[i].Chain, chain) && strings.EqualFold(tokens[i].Address, address) {
			return &tokens[i]
		}
	}
	return nil
}

func mustAssetDetail(t *testing.T, root string) AssetDetail {
	t.Helper()

	detail, err := NewStore(root, "https://cdn.example").GetAssetByAddress("smartchain", testUSDTAddress)
	if err != nil {
		t.Fatal(err)
	}
	return *detail
}

func doRPC(t *testing.T, server *Server, request any, response *rpcResponse) {
	t.Helper()

	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), response); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
}

func mustWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRemarshal(t *testing.T, input any, output any) {
	t.Helper()
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		t.Fatal(err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
