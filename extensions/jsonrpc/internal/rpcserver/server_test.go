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

func TestStablecoinMethodsAreRemoved(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{Root: root, AssetBaseURL: "https://cdn.example"})

	for _, method := range []string{"listStablecoins", "getStablecoinRankings", "getStablecoinBySymbol"} {
		var response rpcResponse
		doRPC(t, server, map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
			"params":  map[string]any{},
		}, &response)
		if response.Error == nil {
			t.Fatalf("expected rpc error for %s", method)
		}
		if response.Error.Code != ErrCodeMethodNotFound {
			t.Fatalf("unexpected error for %s: %+v", method, response.Error)
		}
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

func TestSyncerWritesMarketAndTokenListWithStablecoinTags(t *testing.T) {
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
	baseOverridesPath := filepath.Join(root, DefaultTokenListBaseOverridesPath)
	mustWriteJSON(t, rulesPath, TokenListRules{})
	mustWriteJSON(t, baseOverridesPath, TokenListAssetOverridesFile{
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
		Enabled:                    true,
		MarketCachePath:            filepath.Join(root, "data", "market.json"),
		TokenListCachePath:         filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:        filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:         rulesPath,
		TokenListBaseOverridesPath: baseOverridesPath,
		VsCurrency:                 "usd",
		CoinGeckoAPIKey:            "test-key",
		CoinGeckoBaseURL:           "https://coingecko.test",
		CoinGeckoKeyHeader:         "x-test-key",
		DefiLlamaBaseURL:           "https://defillama.test",
		MarketLimit:                100,
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
	if err := syncer.SyncTokenList(ctx); err != nil {
		t.Fatalf("sync tokenlist: %v", err)
	}

	cache := NewCacheStore(filepath.Join(root, "data", "market.json"))
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
	if usdt == nil || !hasTag(usdt.Tags, "stablecoin") || !hasTag(usdt.Tags, "binance-peg") {
		t.Fatalf("expected USDT stablecoin/binance-peg tags, got %+v", usdt)
	}
	if findAppToken(tokenList.Tokens, "smartchain", noLogoAddress) == nil {
		t.Fatalf("expected unranked active token to remain in output, got %+v", tokenList.Tokens)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Source != "trustwallet+coingecko" || report.Local.OutputTokens != 3 || report.Local.Filtered != 1 || report.Local.MissingLogos != 1 || report.Market.NativeMatches != 1 || report.Market.TokenMatches != 1 || report.Market.RankedAssets != 2 || report.Stablecoin.TaggedAssets != 1 {
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

func TestTokenListSyncKeepsUnrankedAssets(t *testing.T) {
	root := newFixtureRoot(t)
	const extraAddress = "0x00000000000000000000000000000000000000cc"
	addAsset(t, root, "smartchain", extraAddress, map[string]any{
		"name":     "No Rank Token",
		"type":     "BEP20",
		"symbol":   "NRK",
		"decimals": 18,
		"status":   "active",
		"id":       extraAddress,
	})
	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		DefiLlamaBaseURL:    "https://defillama.test",
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
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
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
	if findAppToken(tokenList.Tokens, "smartchain", extraAddress) == nil {
		t.Fatalf("expected unranked asset to remain in tokenlist, got %+v", tokenList.Tokens)
	}
	if got := findAppToken(tokenList.Tokens, "smartchain", extraAddress); got != nil && got.Rank != 0 {
		t.Fatalf("expected unranked asset with zero rank, got %+v", got)
	}
}

func TestTokenListSyncUsesConfigurableExcludedStatuses(t *testing.T) {
	root := newFixtureRoot(t)
	const spamAddress = "0x00000000000000000000000000000000000000dd"
	const abandonedAddress = "0x00000000000000000000000000000000000000ee"
	addAsset(t, root, "smartchain", spamAddress, map[string]any{
		"name":     "Spam Token",
		"type":     "BEP20",
		"symbol":   "SPAM",
		"decimals": 18,
		"status":   "spam",
		"id":       spamAddress,
	})
	addAsset(t, root, "smartchain", abandonedAddress, map[string]any{
		"name":     "Abandoned Token",
		"type":     "BEP20",
		"symbol":   "ABD",
		"decimals": 18,
		"status":   "abandoned",
		"id":       abandonedAddress,
	})

	rulesPath := filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json")
	mustWriteJSON(t, rulesPath, TokenListRules{
		ExcludedStatuses: []string{"spam"},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:  rulesPath,
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		DefiLlamaBaseURL:    "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
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
	if findAppToken(tokenList.Tokens, "smartchain", spamAddress) != nil {
		t.Fatalf("expected spam token to be excluded, got %+v", tokenList.Tokens)
	}
	if findAppToken(tokenList.Tokens, "smartchain", abandonedAddress) == nil {
		t.Fatalf("expected abandoned token to remain when not excluded, got %+v", tokenList.Tokens)
	}
}

func TestTokenListSyncAllowsEmptyExcludedStatuses(t *testing.T) {
	root := newFixtureRoot(t)
	const spamAddress = "0x00000000000000000000000000000000000000fd"
	const abandonedAddress = "0x00000000000000000000000000000000000000fe"
	addAsset(t, root, "smartchain", spamAddress, map[string]any{
		"name":     "Spam Token",
		"type":     "BEP20",
		"symbol":   "SPAM",
		"decimals": 18,
		"status":   "spam",
		"id":       spamAddress,
	})
	addAsset(t, root, "smartchain", abandonedAddress, map[string]any{
		"name":     "Abandoned Token",
		"type":     "BEP20",
		"symbol":   "ABD",
		"decimals": 18,
		"status":   "abandoned",
		"id":       abandonedAddress,
	})

	rulesPath := filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json")
	mustWriteJSON(t, rulesPath, TokenListRules{
		ExcludedStatuses: []string{},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:  filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath: filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:  rulesPath,
		CoinGeckoAPIKey:     "test-key",
		CoinGeckoBaseURL:    "https://coingecko.test",
		DefiLlamaBaseURL:    "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
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
	if findAppToken(tokenList.Tokens, "smartchain", spamAddress) == nil {
		t.Fatalf("expected spam token to remain when excludedStatuses is empty, got %+v", tokenList.Tokens)
	}
	if findAppToken(tokenList.Tokens, "smartchain", abandonedAddress) == nil {
		t.Fatalf("expected abandoned token to remain when excludedStatuses is empty, got %+v", tokenList.Tokens)
	}
}

func TestApplyTokenListConfigOperationUpdatesManualOverridesAndHotCurrent(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})

	manualPath := filepath.Join(root, DefaultTokenListManualOverridesPath)
	hotPath := filepath.Join(root, DefaultTokenListHotCurrentPath)
	mustWriteJSON(t, manualPath, TokenListManualOverrides{
		AssetOverrides: []TokenListAssetOverride{{
			Chain:       "smartchain",
			Address:     testUSDTAddress,
			DisplayName: "Old Name",
			AddTags:     []string{"old-tag"},
		}},
	})
	mustWriteJSON(t, hotPath, TokenListHotList{
		Tokens: []TokenListHotEntry{{
			Chain:   "smartchain",
			Address: "0x0000000000000000000000000000000000000001",
		}},
	})

	overrideResult, err := ApplyTokenListConfigOperation(
		root,
		DefaultTokenListManualOverridesPath,
		DefaultTokenListManualTokensPath,
		DefaultTokenListHotCurrentPath,
		TokenListConfigOperationOverrideUpsert,
		`{"assetOverrides":[{"chain":"smartchain","address":"`+testUSDTAddress+`","displayName":"New Name","addTags":["manual-tag"]},{"chain":"plasma","address":"0x00000000000000000000000000000000000000aa","displaySymbol":"USDE"}]}`,
	)
	if err != nil {
		t.Fatalf("apply override upsert: %v", err)
	}
	if !overrideResult.ManualOverridesUpdated || overrideResult.HotCurrentUpdated {
		t.Fatalf("expected only manual overrides to be updated, got %+v", overrideResult)
	}

	hotResult, err := ApplyTokenListConfigOperation(
		root,
		DefaultTokenListManualOverridesPath,
		DefaultTokenListManualTokensPath,
		DefaultTokenListHotCurrentPath,
		TokenListConfigOperationHotReplaceCurrent,
		`{"tokens":[{"chain":"plasma","address":"0x00000000000000000000000000000000000000aa"},{"chain":"smartchain","address":""}]}`,
	)
	if err != nil {
		t.Fatalf("apply hot replace: %v", err)
	}
	if hotResult.ManualOverridesUpdated || !hotResult.HotCurrentUpdated {
		t.Fatalf("expected only hot current to be updated, got %+v", hotResult)
	}

	var manual TokenListManualOverrides
	if err := readJSONFile(manualPath, &manual); err != nil {
		t.Fatalf("read manual overrides: %v", err)
	}
	if len(manual.AssetOverrides) != 2 {
		t.Fatalf("expected two manual overrides, got %+v", manual.AssetOverrides)
	}
	smartchainOverride := findAssetOverride(manual.AssetOverrides, "smartchain", testUSDTAddress)
	if smartchainOverride == nil || smartchainOverride.DisplayName != "New Name" || hasTag(smartchainOverride.AddTags, "old-tag") || !hasTag(smartchainOverride.AddTags, "manual-tag") {
		t.Fatalf("expected manual override replacement, got %+v", smartchainOverride)
	}
	plasmaOverride := findAssetOverride(manual.AssetOverrides, "plasma", "0x00000000000000000000000000000000000000aa")
	if plasmaOverride == nil || plasmaOverride.DisplaySymbol != "USDE" {
		t.Fatalf("expected plasma override to be appended, got %+v", plasmaOverride)
	}

	var hotList TokenListHotList
	if err := readJSONFile(hotPath, &hotList); err != nil {
		t.Fatalf("read current hot list: %v", err)
	}
	if len(hotList.Tokens) != 2 {
		t.Fatalf("expected two current hot entries after replacement, got %+v", hotList.Tokens)
	}
	if !strings.EqualFold(hotList.Tokens[0].Chain, "plasma") || !strings.EqualFold(hotList.Tokens[0].Address, "0x00000000000000000000000000000000000000aa") {
		t.Fatalf("expected current hot replacement, got %+v", hotList.Tokens)
	}
	if !strings.EqualFold(hotList.Tokens[1].Chain, "smartchain") || hotList.Tokens[1].Address != "" {
		t.Fatalf("expected native hot entry with empty address, got %+v", hotList.Tokens)
	}
}

func TestApplyTokenListConfigOperationRejectsInvalidInput(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationOverrideUpsert, `{"chain":"unknown","address":"0x1"}`); err == nil {
		t.Fatal("expected unknown chain override to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"unknown","kind":"token","address":"0x1"}`); err == nil {
		t.Fatal("expected unknown chain manual token to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"smartchain","kind":"token"}`); err == nil {
		t.Fatal("expected token manual token without address to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"tokens":[{"chain":"plasma","kind":"token","address":"0x00000000000000000000000000000000000000aa"},{"chain":"plasma","kind":"token","address":"0x00000000000000000000000000000000000000aa"}]}`); err == nil {
		t.Fatal("expected duplicate manual token payload to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"smartchain","kind":"token","address":"`+testUSDTAddress+`"}`); err == nil {
		t.Fatal("expected manual token local asset conflict to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"plasma","kind":"native","assetId":"plasma"}`); err == nil {
		t.Fatal("expected native manual token to be rejected")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenDelete, `{"address":"0x1"}`); err == nil {
		t.Fatal("expected manual token delete without chain to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationHotAddCurrent, `{"address":"0x1"}`); err == nil {
		t.Fatal("expected hot token without chain to fail")
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationHotResetCurrent, `{"tokens":[]}`); err == nil {
		t.Fatal("expected hot_reset_current with payload to fail")
	}
}

func TestApplyTokenListConfigOperationDeleteAddRemoveAndReset(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})

	manualPath := filepath.Join(root, DefaultTokenListManualOverridesPath)
	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	hotCurrentPath := filepath.Join(root, DefaultTokenListHotCurrentPath)
	mustWriteJSON(t, manualPath, TokenListAssetOverridesFile{
		AssetOverrides: []TokenListAssetOverride{
			{
				Chain:       "smartchain",
				Address:     testUSDTAddress,
				DisplayName: "Keep Me",
			},
			{
				Chain:       "smartchain",
				Address:     "0x00000000000000000000000000000000000000aa",
				DisplayName: "Delete Me",
			},
		},
	})
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{
		Tokens: []AppToken{{
			Kind:    "token",
			Chain:   "plasma",
			Address: "0x00000000000000000000000000000000000000aa",
			AssetID: "plasma:0x00000000000000000000000000000000000000aa",
			Name:    "Delete Me",
		}},
	})
	mustWriteJSON(t, hotCurrentPath, TokenListHotList{
		Tokens: []TokenListHotEntry{{
			Chain:   "smartchain",
			Address: testUSDTAddress,
		}},
	})

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationOverrideDelete, `{"chain":"smartchain","address":"0x00000000000000000000000000000000000000aa"}`); err != nil {
		t.Fatalf("apply override delete: %v", err)
	}
	var manual TokenListAssetOverridesFile
	if err := readJSONFile(manualPath, &manual); err != nil {
		t.Fatalf("read manual overrides: %v", err)
	}
	if len(manual.AssetOverrides) != 1 || !strings.EqualFold(manual.AssetOverrides[0].Chain, "smartchain") {
		t.Fatalf("expected override delete to keep only smartchain entry, got %+v", manual.AssetOverrides)
	}

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenDelete, `{"chain":"plasma","address":"0x00000000000000000000000000000000000000aa"}`); err != nil {
		t.Fatalf("apply manual token delete: %v", err)
	}
	var manualTokens TokenListManualTokensFile
	if err := readJSONFile(manualTokensPath, &manualTokens); err != nil {
		t.Fatalf("read manual tokens: %v", err)
	}
	if len(manualTokens.Tokens) != 0 {
		t.Fatalf("expected manual token delete to clear file, got %+v", manualTokens.Tokens)
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenDelete, `{"chain":"plasma","address":"0x00000000000000000000000000000000000000aa"}`); err != nil {
		t.Fatalf("apply idempotent manual token delete: %v", err)
	}

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationHotAddCurrent, `{"tokens":[{"chain":"smartchain","address":"`+testUSDTAddress+`"},{"chain":"smartchain","address":""}]}`); err != nil {
		t.Fatalf("apply hot add: %v", err)
	}
	var hotCurrent TokenListHotList
	if err := readJSONFile(hotCurrentPath, &hotCurrent); err != nil {
		t.Fatalf("read hot current: %v", err)
	}
	if len(hotCurrent.Tokens) != 2 {
		t.Fatalf("expected deduped hot current add result, got %+v", hotCurrent.Tokens)
	}

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationHotRemoveCurrent, `{"chain":"smartchain","address":"`+testUSDTAddress+`"}`); err != nil {
		t.Fatalf("apply hot remove: %v", err)
	}
	if err := readJSONFile(hotCurrentPath, &hotCurrent); err != nil {
		t.Fatalf("read hot current after remove: %v", err)
	}
	if len(hotCurrent.Tokens) != 1 || hotCurrent.Tokens[0].Address != "" {
		t.Fatalf("expected only native hot entry to remain, got %+v", hotCurrent.Tokens)
	}

	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationHotResetCurrent, ""); err != nil {
		t.Fatalf("apply hot reset: %v", err)
	}
	if err := readJSONFile(hotCurrentPath, &hotCurrent); err != nil {
		t.Fatalf("read hot current after reset: %v", err)
	}
	if len(hotCurrent.Tokens) != 0 {
		t.Fatalf("expected hot current to be cleared, got %+v", hotCurrent.Tokens)
	}
}

func TestApplyTokenListConfigOperationUpsertsManualTokens(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})

	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{
		Tokens: []AppToken{{
			Kind:       "token",
			Chain:      "plasma",
			Address:    "0x00000000000000000000000000000000000000aa",
			AssetID:    "plasma:0x00000000000000000000000000000000000000aa",
			Name:       "Old Token",
			Symbol:     "OLD",
			Decimals:   18,
			Status:     "active",
			LogoURI:    "https://example.com/old.png",
			LogoExists: true,
		}},
	})

	result, err := ApplyTokenListConfigOperation(
		root,
		DefaultTokenListManualOverridesPath,
		DefaultTokenListManualTokensPath,
		DefaultTokenListHotCurrentPath,
		TokenListConfigOperationManualTokenUpsert,
		`{"tokens":[{"kind":"token","chain":"plasma","address":"0x00000000000000000000000000000000000000aa","assetId":"plasma:0x00000000000000000000000000000000000000aa","name":"New Token","symbol":"NEW","decimals":18,"status":"active","logoURI":"https://example.com/new.png","logoExists":true,"hot":true},{"kind":"token","chain":"plasma","address":"0x00000000000000000000000000000000000000bb","assetId":"plasma:0x00000000000000000000000000000000000000bb","name":"Second Token","symbol":"TWO","decimals":18,"status":"active","hot":false}]}`,
	)
	if err != nil {
		t.Fatalf("apply manual token upsert: %v", err)
	}
	if result.ManualOverridesUpdated || !result.ManualTokensUpdated || result.HotCurrentUpdated {
		t.Fatalf("expected only manual tokens to be updated, got %+v", result)
	}

	var manualTokens TokenListManualTokensFile
	if err := readJSONFile(manualTokensPath, &manualTokens); err != nil {
		t.Fatalf("read manual tokens: %v", err)
	}
	if len(manualTokens.Tokens) != 2 {
		t.Fatalf("expected two manual tokens, got %+v", manualTokens.Tokens)
	}
	replaced := findAppToken(manualTokens.Tokens, "plasma", "0x00000000000000000000000000000000000000aa")
	if replaced == nil || replaced.Name != "New Token" || replaced.Symbol != "NEW" || !replaced.Hot {
		t.Fatalf("expected existing manual token to be replaced, got %+v", replaced)
	}
	second := findAppToken(manualTokens.Tokens, "plasma", "0x00000000000000000000000000000000000000bb")
	if second == nil || second.Symbol != "TWO" {
		t.Fatalf("expected new manual token to be appended, got %+v", manualTokens.Tokens)
	}
}

func TestTokenListSyncAppliesManualOverridesAndHotList(t *testing.T) {
	root := newFixtureRoot(t)
	rulesPath := filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json")
	baseOverridesPath := filepath.Join(root, DefaultTokenListBaseOverridesPath)
	manualPath := filepath.Join(root, DefaultTokenListManualOverridesPath)
	hotDefaultsPath := filepath.Join(root, DefaultTokenListHotDefaultsPath)
	hotCurrentPath := filepath.Join(root, DefaultTokenListHotCurrentPath)

	mustWriteJSON(t, rulesPath, TokenListRules{})
	mustWriteJSON(t, baseOverridesPath, TokenListAssetOverridesFile{
		AssetOverrides: []TokenListAssetOverride{{
			Chain:       "smartchain",
			Address:     testUSDTAddress,
			DisplayName: "Base Override Name",
			AddTags:     []string{"base-tag"},
		}},
	})
	mustWriteJSON(t, manualPath, TokenListManualOverrides{
		AssetOverrides: []TokenListAssetOverride{{
			Chain:       "smartchain",
			Address:     testUSDTAddress,
			DisplayName: "Manual Override Name",
			AddTags:     []string{"manual-tag"},
		}},
	})
	mustWriteJSON(t, hotDefaultsPath, TokenListHotList{
		Tokens: []TokenListHotEntry{
			{
				Chain:   "smartchain",
				Address: testUSDTAddress,
			},
		},
	})
	mustWriteJSON(t, hotCurrentPath, TokenListHotList{
		Tokens: []TokenListHotEntry{
			{
				Chain:   "smartchain",
				Address: "",
			},
		},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:           filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:          filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:           rulesPath,
		TokenListBaseOverridesPath:   baseOverridesPath,
		TokenListManualOverridesPath: manualPath,
		TokenListHotDefaultsPath:     hotDefaultsPath,
		TokenListHotCurrentPath:      hotCurrentPath,
		CoinGeckoAPIKey:              "test-key",
		CoinGeckoBaseURL:             "https://coingecko.test",
		DefiLlamaBaseURL:             "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
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
	usdt := findAppToken(tokenList.Tokens, "smartchain", testUSDTAddress)
	if usdt == nil || usdt.Name != "Manual Override Name" || !hasTag(usdt.Tags, "manual-tag") || hasTag(usdt.Tags, "base-tag") || hasTag(usdt.Tags, "hot") || !usdt.Hot {
		t.Fatalf("expected manual override precedence plus hot bool, got %+v", usdt)
	}
	native := findAppToken(tokenList.Tokens, "smartchain", "")
	if native == nil || !native.Hot || hasTag(native.Tags, "hot") {
		t.Fatalf("expected native asset to accept empty-address hot entry, got %+v", native)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Rules.ConfiguredAssetOverrides != 1 {
		t.Fatalf("expected merged asset override count, got %+v", report.Rules)
	}
	if report.Rules.BaseAssetOverrides != 1 || report.Rules.ManualAssetOverrides != 1 {
		t.Fatalf("expected split override stats, got %+v", report.Rules)
	}
	if report.Hot.DefaultEntries != 1 || report.Hot.CurrentEntries != 1 {
		t.Fatalf("expected split hot stats, got %+v", report.Hot)
	}
	if report.Hot.EnabledAssets != 2 {
		t.Fatalf("expected hot enabled assets count, got %+v", report.Hot)
	}
	if len(report.Issues.MissingHotAssets) != 0 {
		t.Fatalf("unexpected missing hot assets: %+v", report.Issues.MissingHotAssets)
	}
}

func TestTokenListSyncAppendsManualTokens(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "plasma", map[string]any{
		"name":     "Plasma",
		"symbol":   "XPL",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
	})
	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{
		Tokens: []AppToken{{
			Kind:       "token",
			Chain:      "plasma",
			Address:    "0x00000000000000000000000000000000000000aa",
			AssetID:    "plasma:0x00000000000000000000000000000000000000aa",
			Name:       "Manual USDM",
			Symbol:     "USDM",
			Decimals:   6,
			Status:     "active",
			LogoURI:    "https://example.com/usdm.png",
			LogoExists: true,
			Rank:       88,
			Market: &AppTokenMarket{
				Source:        "manual",
				CoinGeckoID:   "manual-usdm",
				MarketCapRank: 88,
			},
			Tags: []string{"stablecoin", "defi"},
			Hot:  true,
		}},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:        filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:       filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:        filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json"),
		TokenListManualTokensPath: manualTokensPath,
		CoinGeckoAPIKey:           "test-key",
		CoinGeckoBaseURL:          "https://coingecko.test",
		DefiLlamaBaseURL:          "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
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
	manual := findAppToken(tokenList.Tokens, "plasma", "0x00000000000000000000000000000000000000aa")
	if manual == nil || manual.Name != "Manual USDM" || manual.Market == nil || manual.Market.CoinGeckoID != "manual-usdm" || !manual.Hot || !hasTag(manual.Tags, "stablecoin") {
		t.Fatalf("expected manual token to be appended verbatim, got %+v", manual)
	}
	if tokenList.Tokens[len(tokenList.Tokens)-1].Chain != "plasma" || !strings.EqualFold(tokenList.Tokens[len(tokenList.Tokens)-1].Address, "0x00000000000000000000000000000000000000aa") {
		t.Fatalf("expected manual token to be appended after generated tokens, got %+v", tokenList.Tokens[len(tokenList.Tokens)-1])
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Local.OutputTokens != len(tokenList.Tokens) {
		t.Fatalf("expected output token count to include manual tokens, got %+v", report.Local)
	}
	if report.Market.RankedAssets != 1 {
		t.Fatalf("expected manual token rank to contribute to rankedAssets, got %+v", report.Market)
	}
	if report.Hot.EnabledAssets != 1 {
		t.Fatalf("expected manual hot token to contribute to hot count, got %+v", report.Hot)
	}
}

func TestTokenListSyncRejectsManualTokenLocalConflicts(t *testing.T) {
	root := newFixtureRoot(t)
	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{
		Tokens: []AppToken{{
			Kind:    "token",
			Chain:   "smartchain",
			Address: testUSDTAddress,
			AssetID: "manual-usdt",
			Name:    "Manual USDT",
		}},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:        filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:       filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:        filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json"),
		TokenListManualTokensPath: manualTokensPath,
		CoinGeckoAPIKey:           "test-key",
		CoinGeckoBaseURL:          "https://coingecko.test",
		DefiLlamaBaseURL:          "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := syncer.SyncTokenList(ctx); err == nil || !strings.Contains(err.Error(), "conflicts with a local asset") {
		t.Fatalf("expected manual token conflict to fail sync, got %v", err)
	}
}

func TestTokenListSyncReportsMissingHotAssets(t *testing.T) {
	root := newFixtureRoot(t)
	hotCurrentPath := filepath.Join(root, DefaultTokenListHotCurrentPath)
	mustWriteJSON(t, hotCurrentPath, TokenListHotList{
		Tokens: []TokenListHotEntry{{
			Chain:   "smartchain",
			Address: "0x00000000000000000000000000000000000000ff",
		}},
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:      filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:     filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:      filepath.Join(root, "extensions", "jsonrpc", "config", "tokenlist-rules.json"),
		TokenListHotCurrentPath: hotCurrentPath,
		CoinGeckoAPIKey:         "test-key",
		CoinGeckoBaseURL:        "https://coingecko.test",
		DefiLlamaBaseURL:        "https://defillama.test",
	})
	syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[]`), nil
		case "coingecko.test/coins/list":
			return jsonResponse(`[]`), nil
		case "defillama.test/stablecoins":
			return jsonResponse(`{"peggedAssets":[]}`), nil
		default:
			return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewReader(nil)), Header: make(http.Header)}, nil
		}
	})}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := syncer.SyncTokenList(ctx); err != nil {
		t.Fatalf("sync tokenlist: %v", err)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Hot.DefaultEntries != 0 || report.Hot.CurrentEntries != 1 {
		t.Fatalf("expected split hot counts, got %+v", report.Hot)
	}
	if report.Hot.EnabledAssets != 0 {
		t.Fatalf("expected no hot enabled assets, got %+v", report.Hot)
	}
	if len(report.Issues.MissingHotAssets) != 1 || !strings.EqualFold(report.Issues.MissingHotAssets[0].Address, "0x00000000000000000000000000000000000000ff") {
		t.Fatalf("expected missing hot asset to be reported, got %+v", report.Issues.MissingHotAssets)
	}
}

func TestAppTokenJSONAlwaysIncludesHotField(t *testing.T) {
	data, err := json.Marshal(AppToken{
		Kind:    "token",
		Chain:   "smartchain",
		Hot:     false,
		Address: testUSDTAddress,
		AssetID: "c20000714_t" + testUSDTAddress,
		Symbol:  "USDT",
	})
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	value, ok := payload["hot"]
	if !ok {
		t.Fatalf("expected hot field to be present, payload=%s", string(data))
	}
	hot, ok := value.(bool)
	if !ok || hot {
		t.Fatalf("expected hot=false, payload=%s", string(data))
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
	baseOverridesPath := filepath.Join(root, DefaultTokenListBaseOverridesPath)
	mustWriteJSON(t, rulesPath, TokenListRules{
		PlatformMappings: map[string]string{
			"plasma": "plasma",
		},
		NativeMarketMappings: map[string][]string{
			"ethereum": []string{"base"},
		},
		MarketTagRules: []TokenListMarketTagRule{
			{CoinGeckoID: "tether", AddTags: []string{"stablecoin"}},
			{CoinGeckoID: "ethena-usde", AddTags: []string{"stablecoin", "defi"}},
		},
	})
	mustWriteJSON(t, baseOverridesPath, TokenListAssetOverridesFile{
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
	})

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{
		TokenListCachePath:         filepath.Join(root, "data", "tokenlist.json"),
		TokenListReportPath:        filepath.Join(root, "data", "tokenlist-report.json"),
		TokenListRulesPath:         rulesPath,
		TokenListBaseOverridesPath: baseOverridesPath,
		VsCurrency:                 "usd",
		CoinGeckoAPIKey:            "test-key",
		CoinGeckoBaseURL:           "https://coingecko.test",
		CoinGeckoKeyHeader:         "x-test-key",
		DefiLlamaBaseURL:           "https://defillama.test",
		MarketLimit:                100,
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
				}, {
					"id": 2,
					"gecko_id": "ethena-usde",
					"name": "Ethena USDe",
					"symbol": "USDE",
					"pegType": "peggedUSD",
					"priceSource": "defillama",
					"circulating": {"peggedUSD": 100},
					"chainCirculating": {"Plasma": {}}
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
	if report.Rules.ConfiguredPlatformMappings != 1 || report.Rules.ConfiguredAssetOverrides != 2 || report.Rules.BaseAssetOverrides != 2 || report.Rules.ManualAssetOverrides != 0 || report.Rules.PlatformMappingHits != 1 || report.Rules.NativeMarketMappingHits != 1 {
		t.Fatalf("unexpected rule stats: %+v", report.Rules)
	}
	if report.Stablecoin.TaggedAssets != 2 {
		t.Fatalf("expected stablecoin tagged assets, got %+v", report.Stablecoin)
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
	config := &ResolvedTokenListConfig{
		NativeMarketMappings: rules.NativeMarketMappings,
	}

	chains, usedRule := coinGeckoNativeChainsWithRules("arbitrum", config)
	if !usedRule {
		t.Fatal("expected explicit native rule to override built-in mapping")
	}
	if len(chains) != 0 {
		t.Fatalf("expected empty native mapping override, got %v", chains)
	}
}

func TestParseSyncTarget(t *testing.T) {
	tests := map[string]SyncTarget{
		"":          SyncTargetAll,
		"all":       SyncTargetAll,
		"market":    SyncTargetMarket,
		"tokenlist": SyncTargetTokenList,
		" MARKET ":  SyncTargetMarket,
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

func TestSyncOnceTargetSkipsMarketCacheForTokenList(t *testing.T) {
	root := newFixtureRoot(t)
	marketPath := filepath.Join(root, "data", "market.json")
	tokenListPath := filepath.Join(root, "data", "tokenlist.json")
	server := NewServer(Config{
		Root:               root,
		AssetBaseURL:       "https://cdn.example",
		MarketCachePath:    marketPath,
		TokenListCachePath: tokenListPath,
		CoinGeckoAPIKey:    "test-key",
		CoinGeckoBaseURL:   "https://coingecko.test",
		DefiLlamaBaseURL:   "https://defillama.test",
		MarketLimit:        100,
	})
	server.syncer.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host + r.URL.Path {
		case "coingecko.test/coins/markets":
			return jsonResponse(`[{
				"id": "tether",
				"symbol": "usdt",
				"name": "Tether",
				"market_cap_rank": 3
			}]`), nil
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

	if err := server.SyncOnce(ctx, SyncTargetTokenList); err != nil {
		t.Fatalf("sync once tokenlist: %v", err)
	}
	if fileExists(marketPath) {
		t.Fatal("market cache should not be written for tokenlist-only sync")
	}
	if !fileExists(tokenListPath) {
		t.Fatal("tokenlist cache should be written")
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

func findAssetOverride(overrides []TokenListAssetOverride, chain, address string) *TokenListAssetOverride {
	for i := range overrides {
		if strings.EqualFold(overrides[i].Chain, chain) && strings.EqualFold(overrides[i].Address, address) {
			return &overrides[i]
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
