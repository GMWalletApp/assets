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

func TestTokenListSyncExcludesConfiguredChains(t *testing.T) {
	root := newFixtureRoot(t)
	const bep2Address = "YFII-061"
	const manualBEP2Address = "MANUAL-001"

	addNativeChain(t, root, "binance", map[string]any{
		"name":     "BNB Beacon Chain",
		"symbol":   "BNB",
		"type":     "coin",
		"decimals": 8,
		"status":   "active",
	})
	addAsset(t, root, "binance", bep2Address, map[string]any{
		"name":     "YFIIBEP2",
		"type":     "BEP2",
		"symbol":   "YFII",
		"decimals": 8,
		"status":   "active",
		"id":       bep2Address,
	})

	index, err := NewStore(root, "https://cdn.example").BuildAssetIndex()
	if err != nil {
		t.Fatalf("build asset index: %v", err)
	}
	config := &ResolvedTokenListConfig{ExcludedChains: []string{"binance"}}
	manualTokens := []AppToken{{
		Kind:     "token",
		Chain:    "binance",
		Address:  manualBEP2Address,
		AssetID:  "c714_t" + manualBEP2Address,
		Name:     "Manual BEP2",
		Symbol:   "MBEP2",
		Decimals: 8,
		Status:   "active",
	}}

	syncer := NewSyncer(NewStore(root, "https://cdn.example"), SyncConfig{})
	tokenList, report := syncer.buildAppTokenList(index, nil, nil, nil, nil, config, manualTokens, "2026-07-01T00:00:00Z")

	if findAppToken(tokenList.Tokens, "smartchain", testUSDTAddress) == nil {
		t.Fatalf("expected smartchain token to remain, got %+v", tokenList.Tokens)
	}
	if findAppToken(tokenList.Tokens, "binance", "") != nil {
		t.Fatalf("expected binance native to be excluded, got %+v", tokenList.Tokens)
	}
	if findAppToken(tokenList.Tokens, "binance", bep2Address) != nil {
		t.Fatalf("expected generated BEP2 token to be excluded, got %+v", tokenList.Tokens)
	}
	if findAppToken(tokenList.Tokens, "binance", manualBEP2Address) != nil {
		t.Fatalf("expected manual BEP2 token to be excluded, got %+v", tokenList.Tokens)
	}
	if report.Rules.ConfiguredExcludedChains != 1 || report.Rules.ExcludedChainHits != 3 {
		t.Fatalf("unexpected excluded chain stats: %+v", report.Rules)
	}
	if report.Local.Filtered != 3 || report.Local.OutputTokens != len(tokenList.Tokens) {
		t.Fatalf("unexpected local stats: %+v", report.Local)
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
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"smartchain","kind":"token","address":"`+testUSDTAddress+`","assetId":"usdt0:smartchain:`+testUSDTAddress+`","symbol":"USDT0"}`); err != nil {
		t.Fatalf("expected distinct manual representation of local asset to succeed: %v", err)
	}
	if _, err := ApplyTokenListConfigOperation(root, DefaultTokenListManualOverridesPath, DefaultTokenListManualTokensPath, DefaultTokenListHotCurrentPath, TokenListConfigOperationManualTokenUpsert, `{"chain":"smartchain","kind":"token","address":"`+testUSDTAddress+`","assetId":"c20000714_t`+testUSDTAddress+`","symbol":"USDT0"}`); err == nil {
		t.Fatal("expected reuse of local assetId to fail")
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
		AssetOverrides: []TokenListAssetOverride{
			{
				Chain:       "smartchain",
				Address:     testUSDTAddress,
				DisplayName: "Manual Override Name",
				AddTags:     []string{"manual-tag"},
				ReceiveList: boolPtr(true),
			},
			{
				Chain:       "smartchain",
				ReceiveList: boolPtr(true),
			},
		},
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
	if usdt == nil || usdt.Name != "Manual Override Name" || !hasTag(usdt.Tags, "manual-tag") || hasTag(usdt.Tags, "base-tag") || hasTag(usdt.Tags, "hot") || !usdt.Hot || !usdt.ReceiveList {
		t.Fatalf("expected manual override precedence plus hot bool, got %+v", usdt)
	}
	native := findAppToken(tokenList.Tokens, "smartchain", "")
	if native == nil || !native.Hot || !native.ReceiveList || hasTag(native.Tags, "hot") {
		t.Fatalf("expected native asset to accept empty-address hot and receive override, got %+v", native)
	}

	var report TokenListReport
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist-report.json"), &report); err != nil {
		t.Fatalf("read tokenlist report: %v", err)
	}
	if report.Rules.ConfiguredAssetOverrides != 2 {
		t.Fatalf("expected merged asset override count, got %+v", report.Rules)
	}
	if report.Rules.BaseAssetOverrides != 1 || report.Rules.ManualAssetOverrides != 2 {
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
	if len(report.Issues.RuleIssues) != 0 {
		t.Fatalf("unexpected native asset override issue: %+v", report.Issues.RuleIssues)
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

func TestTokenListSyncAllowsDistinctManualRepresentationOfLocalAsset(t *testing.T) {
	root := newFixtureRoot(t)
	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{
		Tokens: []AppToken{{
			Kind:     "token",
			Chain:    "smartchain",
			Address:  testUSDTAddress,
			AssetID:  "usdt0:smartchain:" + testUSDTAddress,
			Name:     "USDT0",
			Symbol:   "USDT0",
			Decimals: 6,
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
		t.Fatalf("expected distinct manual representation to sync: %v", err)
	}

	var tokenList AppTokenList
	if err := readJSONFile(filepath.Join(root, "data", "tokenlist.json"), &tokenList); err != nil {
		t.Fatalf("read tokenlist: %v", err)
	}
	matching := 0
	foundUSDT0 := false
	for _, token := range tokenList.Tokens {
		if token.Chain == "smartchain" && strings.EqualFold(token.Address, testUSDTAddress) {
			matching++
			foundUSDT0 = foundUSDT0 || token.AssetID == "usdt0:smartchain:"+testUSDTAddress && token.Symbol == "USDT0"
		}
	}
	if matching != 2 || !foundUSDT0 {
		t.Fatalf("expected generated USDT plus manual USDT0 representation, got %+v", tokenList.Tokens)
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

func TestLoadTokenListRulesNormalizesExcludedChains(t *testing.T) {
	rulesPath := filepath.Join(t.TempDir(), "rules.json")
	mustWriteJSON(t, rulesPath, TokenListRules{
		ExcludedChains: []string{" Binance ", "BINANCE", "", "SmartChain "},
	})

	rules, err := loadTokenListRules(rulesPath)
	if err != nil {
		t.Fatalf("load tokenlist rules: %v", err)
	}
	if got := strings.Join(rules.ExcludedChains, ","); got != "binance,smartchain" {
		t.Fatalf("unexpected excluded chains: %v", rules.ExcludedChains)
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

func TestManagedListCRUDAndPack(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                     root,
		AssetBaseURL:             "https://cdn.example",
		ManagedListDBPath:        filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir:      filepath.Join(root, "files"),
		ManagedListPublicBaseURL: "https://assets.example/output/",
	})

	list, err := server.lists.UpsertList(ManagedList{
		Key:           "USDT",
		Name:          "USDT List",
		Description:   "All supported USDT variants",
		DisplayName:   "Tether USD",
		DisplaySymbol: "USDT",
		LogoURI:       "https://cdn.example/usdt-family.png",
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("upsert list: %v", err)
	}
	if list.Key != "usdt" || !list.Enabled {
		t.Fatalf("unexpected managed list: %+v", list)
	}

	item, err := server.lists.UpsertItem("usdt", ManagedListItem{
		Token: ManagedToken{
			Chain:   "smartchain",
			Address: testUSDTAddress,
		},
		Rank:           1,
		Enabled:        true,
		Display:        true,
		Slot:           "usdt",
		DisplaySymbol:  "USDT",
		DisplayLogoURI: "https://cdn.example/bsc-usdt.png",
	})
	if err != nil {
		t.Fatalf("upsert item: %v", err)
	}
	if item.Token.Symbol != "USDT" || !item.Token.LogoExists {
		t.Fatalf("expected item to be hydrated from local assets, got %+v", item)
	}

	items, err := server.lists.ListItems("usdt")
	if err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected one item, got %+v", items)
	}

	packed, err := server.lists.PackList("usdt")
	if err != nil {
		t.Fatalf("pack list: %v", err)
	}
	if packed.TokenCount != 1 || packed.JSONSize == 0 || packed.ZstdSize == 0 || packed.ZstdSHA256 == "" {
		t.Fatalf("unexpected pack result: %+v", packed)
	}
	if packed.JSONPath != "usdt.json" || packed.ZstdPath != "usdt.json.zst" || packed.JSONURL != "https://assets.example/output/usdt.json" || packed.ZstdURL != "https://assets.example/output/usdt.json.zst" {
		t.Fatalf("expected portable artifact paths and configured public URLs, got %+v", packed)
	}
	if !fileExists(filepath.Join(root, "files", "usdt.json")) || !fileExists(filepath.Join(root, "files", "usdt.json.zst")) {
		t.Fatalf("expected packed files to exist: %+v", packed)
	}

	var output ManagedListOutput
	if err := readJSONFile(filepath.Join(root, "files", "usdt.json"), &output); err != nil {
		t.Fatalf("read packed json: %v", err)
	}
	if output.Key != "usdt" || len(output.Items) != 1 || output.Items[0].Symbol != "USDT" {
		t.Fatalf("unexpected packed output: %+v", output)
	}
	if !output.Items[0].Display || output.Items[0].Slot != "usdt" || output.Items[0].DisplaySymbol != "USDT" || output.Items[0].ChainName == "" || output.Items[0].Explorer == "" {
		t.Fatalf("expected homepage-style fields in packed output, got %+v", output.Items[0])
	}
	if output.DisplayName != "Tether USD" || output.DisplaySymbol != "USDT" || output.LogoURI != "https://cdn.example/usdt-family.png" {
		t.Fatalf("expected shared list presentation in packed output, got %+v", output)
	}
	if output.Items[0].LogoURI != "https://cdn.example/bsc-usdt.png" {
		t.Fatalf("expected contract-level logo to override shared logo, got %+v", output.Items[0])
	}
	rawOutput, err := os.ReadFile(filepath.Join(root, "files", "usdt.json"))
	if err != nil {
		t.Fatalf("read raw packed json: %v", err)
	}
	if !bytes.Contains(rawOutput, []byte(`"items"`)) || bytes.Contains(rawOutput, []byte(`"tokens"`)) || bytes.Contains(rawOutput, []byte(`"source"`)) || bytes.Contains(rawOutput, []byte(`"note"`)) {
		t.Fatalf("packed managed list must use items and omit tokens/source/note: %s", rawOutput)
	}

	manifest, err := server.lists.PackAll()
	if err != nil {
		t.Fatalf("pack all: %v", err)
	}
	if len(manifest.Files) != 1 || manifest.Files[0].ListKey != "usdt" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if !fileExists(filepath.Join(root, "files", "manifest.json")) {
		t.Fatal("expected manifest file to exist")
	}
}

func TestManagedSupportListSeedCRUDAndPack(t *testing.T) {
	root := newFixtureRoot(t)
	mustWriteJSON(t, filepath.Join(root, "support", "support.json"), PublishedSupportDocument{
		SchemaVersion: 1,
		AssetBaseURI:  supportAssetBaseURI,
		Exchanges: []PublishedSupportEntry{
			{ID: "uniswap", Name: "Uniswap", Type: "dex", LogoURI: supportAssetBaseURI + "/exchanges/uniswap/logo.svg"},
		},
		Wallets: []PublishedSupportEntry{
			{ID: "metamask", Name: "MetaMask", LogoURI: supportAssetBaseURI + "/wallets/metamask/logo.svg"},
		},
	})
	server := NewServer(Config{
		Root:                root,
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	if err := server.lists.SeedDefaultLists(); err != nil {
		t.Fatalf("seed support list: %v", err)
	}
	if err := server.lists.SeedDefaultLists(); err != nil {
		t.Fatalf("support seed should be idempotent: %v", err)
	}

	rec := doHTTP(t, server.listAPIHandler(), http.MethodGet, "/api/lists/support", "")
	var document ManagedSupportDocument
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &document) != nil {
		t.Fatalf("get support document status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(document.Exchanges) != 1 || len(document.Wallets) != 3 {
		t.Fatalf("expected source plus static wallet seeds, got %+v", document)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/support/exchanges", `{"id":"curve","name":"Curve","type":"dex","logoURI":"https://cdn.example/curve.svg","rank":2}`)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/lists/support/exchanges/curve" {
		t.Fatalf("create exchange status=%d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodPatch, "/api/lists/support/exchanges/curve", `{"enabled":false,"name":"Curve Finance"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("patch exchange status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodPut, "/api/lists/support/wallets/rabby", `{"name":"Rabby","logoURI":"https://cdn.example/rabby.svg","rank":4,"enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("put wallet status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodDelete, "/api/lists/support/wallets/metamask", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete wallet status=%d body=%s", rec.Code, rec.Body.String())
	}

	packed, err := server.lists.PackList("support")
	if err != nil {
		t.Fatalf("pack support: %v", err)
	}
	if packed.TokenCount != 4 {
		t.Fatalf("expected one enabled exchange and three wallets, got %+v", packed)
	}
	var output PublishedSupportDocument
	if err := readJSONFile(filepath.Join(root, "files", "support.json"), &output); err != nil {
		t.Fatalf("read packed support: %v", err)
	}
	if len(output.Exchanges) != 1 || output.Exchanges[0].ID != "uniswap" || len(output.Wallets) != 3 {
		t.Fatalf("packed support should omit disabled/deleted entries: %+v", output)
	}
	for _, entry := range append(output.Exchanges, output.Wallets...) {
		if entry.ID == "curve" || entry.ID == "metamask" {
			t.Fatalf("disabled/deleted entry leaked into packed support: %+v", output)
		}
	}
}

func TestManagedListIncludesCRUDAndExpandedPack(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                root,
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	for _, list := range []ManagedList{
		{Key: "homepage", Name: "Homepage", Enabled: true},
		{Key: "usdt", Name: "USDT", DisplayName: "Tether USD", DisplaySymbol: "USDT", LogoURI: "https://cdn.example/usdt.svg", Enabled: true},
		{Key: "collection", Name: "Collection", Enabled: true},
	} {
		if _, err := server.lists.UpsertList(list); err != nil {
			t.Fatalf("create %s: %v", list.Key, err)
		}
	}
	sourceItems := []ManagedListItem{
		{Token: ManagedToken{Kind: "token", Chain: "ethereum", Address: "0x111", Name: "Ethereum Tether", Symbol: "USDT", Decimals: 6}, Slot: "usdt", Rank: 1, Enabled: true, Display: true},
		{Token: ManagedToken{Kind: "token", Chain: "tron", Address: "T111", Name: "Tron Tether", Symbol: "USDT", Decimals: 6}, Slot: "usdt", Rank: 2, Enabled: true, Display: false},
	}
	for _, item := range sourceItems {
		if _, err := server.lists.SaveItem("usdt", item); err != nil {
			t.Fatalf("save USDT source item: %v", err)
		}
	}
	if _, err := server.lists.SaveItem("homepage", ManagedListItem{
		Token: ManagedToken{Kind: "token", Chain: "ethereum", Address: "0x111", Name: "Old homepage USDT", Symbol: "USDT", Decimals: 6},
		Slot:  "usdt", Rank: 5, Enabled: true, Display: true,
	}); err != nil {
		t.Fatalf("save duplicate homepage item: %v", err)
	}
	if _, err := server.lists.SaveItem("homepage", ManagedListItem{
		Token: ManagedToken{Kind: "native", Chain: "solana", Address: "", Name: "Solana", Symbol: "SOL", Decimals: 9},
		Slot:  "native", Rank: 1, Enabled: true, Display: true,
	}); err != nil {
		t.Fatalf("save native homepage item: %v", err)
	}

	rec := doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/homepage/includes", `{"tag":"usdt","rank":10,"enabled":true}`)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/lists/homepage/includes/usdt" {
		t.Fatalf("create include status=%d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/homepage/includes", `{"tag":"usdt"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate include status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodGet, "/api/lists/homepage", "")
	var document ManagedListDocument
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &document) != nil || len(document.Includes) != 1 || document.Includes[0].Tag != "usdt" {
		t.Fatalf("list document should contain includes: status=%d body=%s", rec.Code, rec.Body.String())
	}

	packed, err := server.lists.PackList("homepage")
	if err != nil {
		t.Fatalf("pack expanded homepage: %v", err)
	}
	var output ManagedListOutput
	if err := readJSONFile(filepath.Join(root, "files", packed.JSONPath), &output); err != nil {
		t.Fatalf("read expanded homepage: %v", err)
	}
	if len(output.Items) != 3 {
		t.Fatalf("expected native plus two deduplicated USDT items, got %+v", output.Items)
	}
	var foundEthereum, foundTron bool
	for _, item := range output.Items {
		if item.Chain == "ethereum" && item.Address == "0x111" {
			foundEthereum = item.Slot == "usdt" && item.Rank == 10 && item.DisplayName == "Tether USD" && item.DisplaySymbol == "USDT" && item.LogoURI == "https://cdn.example/usdt.svg"
		}
		if item.Chain == "tron" && item.Address == "T111" {
			foundTron = item.Slot == "usdt" && item.Rank == 10 && !item.Display
		}
	}
	if !foundEthereum || !foundTron {
		t.Fatalf("expanded source presentation/display was not preserved: %+v", output.Items)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPatch, "/api/lists/homepage/includes/usdt", `{"enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable include status=%d body=%s", rec.Code, rec.Body.String())
	}
	packed, err = server.lists.PackList("homepage")
	if err != nil {
		t.Fatalf("pack homepage with disabled include: %v", err)
	}
	if err := readJSONFile(filepath.Join(root, "files", packed.JSONPath), &output); err != nil || len(output.Items) != 2 {
		t.Fatalf("disabled include should leave direct homepage items: err=%v output=%+v", err, output)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/collection/includes", `{"tag":"homepage","enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create collection include status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodPut, "/api/lists/homepage/includes/collection", `{"tag":"collection","enabled":true}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cyclic include should conflict: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/homepage/includes", `{"tag":"homepage"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("self include should be rejected: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodDelete, "/api/lists/homepage/includes/usdt", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete include status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, server.listAPIHandler(), http.MethodGet, "/api/lists/homepage/includes/usdt", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted include should be missing: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestManagedListRESTAPI(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})

	rec := doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists", `{"key":"usdc","name":"USDC List","enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create list status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/usdc/items", `{"token":{"chain":"smartchain","address":"`+testUSDTAddress+`"},"slot":"usdc","rank":7,"enabled":true,"displaySymbol":"USDC"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item status=%d body=%s", rec.Code, rec.Body.String())
	}

	var item ManagedListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if item.Token.Symbol != "USDT" || item.DisplaySymbol != "USDC" || item.Rank != 7 || !item.Display || item.Slot != "usdc" {
		t.Fatalf("unexpected REST item: %+v", item)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPatch, "/api/lists/usdc", `{"description":"Updated description"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var list ManagedList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode patched list: %v", err)
	}
	if list.Name != "USDC List" || list.Description != "Updated description" || list.OutputPath != "usdc.json" || !list.Enabled {
		t.Fatalf("PATCH should preserve omitted list fields, got %+v", list)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPatch, "/api/lists/usdc/items/smartchain/"+testUSDTAddress, `{"displayName":"Reviewed USDC"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch item status=%d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode patched item: %v", err)
	}
	if item.DisplayName != "Reviewed USDC" || item.DisplaySymbol != "USDC" || item.Rank != 7 || !item.Enabled || !item.Display || item.Slot != "usdc" {
		t.Fatalf("PATCH should preserve omitted item fields, got %+v", item)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodGet, "/api/lists/usdc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get aggregate list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var document ManagedListDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatalf("decode aggregate list: %v", err)
	}
	if document.Key != "usdc" || document.Description != "Updated description" || len(document.Items) != 1 || document.Items[0].DisplayName != "Reviewed USDC" {
		t.Fatalf("GET list should include top-level metadata and items, got %+v", document)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"items"`)) || bytes.Contains(rec.Body.Bytes(), []byte(`"tokens"`)) {
		t.Fatalf("aggregate list must use one items array: %s", rec.Body.String())
	}

	rec = doHTTP(t, server.packAPIHandler(), http.MethodPost, "/api/pack/usdc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pack status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !fileExists(filepath.Join(root, "files", "usdc.json.zst")) {
		t.Fatal("expected REST pack to write zstd output")
	}
}

func TestManagedListRESTCRUDValidation(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	handler := server.listAPIHandler()

	rec := doHTTP(t, handler, http.MethodPost, "/api/lists", `{"key":"custom","name":"Custom","enabled":true}`)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/lists/custom" {
		t.Fatalf("create list status=%d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPost, "/api/lists", `{"key":"custom","name":"Duplicate"}`)
	if rec.Code != http.StatusConflict || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"conflict"`)) {
		t.Fatalf("duplicate list should return structured conflict: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/custom", `{"displayName":"Custom USD","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch list status=%d body=%s", rec.Code, rec.Body.String())
	}
	var document ManagedListDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Name != "Custom" || document.DisplayName != "Custom USD" || document.Enabled || document.Items == nil {
		t.Fatalf("patch list did not preserve and update individual fields: %+v", document)
	}
	rec = doHTTP(t, handler, http.MethodPut, "/api/lists/custom", `{"name":"Replaced Custom","displaySymbol":"CUS","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace list status=%d body=%s", rec.Code, rec.Body.String())
	}
	document = ManagedListDocument{}
	if err := json.Unmarshal(rec.Body.Bytes(), &document); err != nil {
		t.Fatal(err)
	}
	if document.Name != "Replaced Custom" || document.DisplayName != "" || document.DisplaySymbol != "CUS" || !document.Enabled {
		t.Fatalf("PUT should replace list-level fields: %+v", document)
	}
	rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/custom", `{"unknown":true}`)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("unknown field")) {
		t.Fatalf("unknown list field should fail: status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, invalidOutputPath := range []string{"../escape.json", `nested\\escape.json`, "C:/escape.json"} {
		rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/custom", `{"outputPath":`+recBodyJSON(t, invalidOutputPath)+`}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("unsafe output path %q should fail: status=%d body=%s", invalidOutputPath, rec.Code, rec.Body.String())
		}
	}
	rec = doHTTP(t, handler, http.MethodPost, "/api/lists", `{"key":"other","outputPath":"custom.json","enabled":true}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate output path should conflict: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodGet, "/api/lists/missing/items", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown list items should return 404, got status=%d body=%s", rec.Code, rec.Body.String())
	}

	createItemBody := `{"token":{"kind":"token","chain":"custom-chain","address":"0xabc","name":"Original","symbol":"OLD","decimals":6},"rank":2,"enabled":true,"display":true}`
	rec = doHTTP(t, handler, http.MethodPost, "/api/lists/custom/items", createItemBody)
	if rec.Code != http.StatusCreated || rec.Header().Get("Location") != "/api/lists/custom/items/custom-chain/0xabc" {
		t.Fatalf("create item status=%d location=%q body=%s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPost, "/api/lists/custom/items", createItemBody)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate item should return conflict: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodGet, "/api/lists/custom/items/custom-chain/0xabc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get item status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/custom/items/custom-chain/0xabc", `{"token":{"name":"Renamed","symbol":"NEW"},"display":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch item status=%d body=%s", rec.Code, rec.Body.String())
	}
	var item ManagedListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Token.Name != "Renamed" || item.Token.Symbol != "NEW" || item.Token.Decimals != 6 || item.Display || item.Rank != 2 {
		t.Fatalf("patch item did not deep-merge individual fields: %+v", item)
	}
	rec = doHTTP(t, handler, http.MethodPut, "/api/lists/custom/items/custom-chain/0xabc", `{"token":{"kind":"token","chain":"custom-chain","address":"0xabc","name":"Replacement","symbol":"RPL","decimals":8},"enabled":true,"display":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("replace item status=%d body=%s", rec.Code, rec.Body.String())
	}
	item = ManagedListItem{}
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Token.Name != "Replacement" || item.Token.Symbol != "RPL" || item.Token.Decimals != 8 || item.Rank != 0 || !item.Enabled || !item.Display {
		t.Fatalf("PUT should replace the complete item: %+v", item)
	}
	rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/custom/items/custom-chain/0xabc", `{"rank":-1}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative rank should fail: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPut, "/api/lists/custom/items/custom-chain/0xabc", `{"token":{"chain":"other-chain","address":"0xabc","kind":"token"}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("path/body identity mismatch should fail: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodPut, "/api/lists/custom/items/custom-chain/0xdef", `{"token":{"kind":"token","chain":"custom-chain","address":"0xdef","name":"PUT Created","symbol":"NEW","decimals":6},"enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("PUT should create a missing item: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodDelete, "/api/lists/custom/items/custom-chain/0xdef", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete PUT-created item status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodDelete, "/api/lists/custom/items/custom-chain/0xabc", "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete item should return empty 204: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodGet, "/api/lists/custom/items/custom-chain/0xabc", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("deleted item should return 404: status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, handler, http.MethodDelete, "/api/lists/custom", "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("delete list should return empty 204: status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestManagedListItemCanBeCopiedDirectlyBetweenLists(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	handler := server.listAPIHandler()
	for _, key := range []string{"tokenlist", "homepage", "stablecoin"} {
		rec := doHTTP(t, handler, http.MethodPost, "/api/lists", `{"key":"`+key+`","name":"`+key+`","enabled":true}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s status=%d body=%s", key, rec.Code, rec.Body.String())
		}
	}

	itemJSON := `{
		"token":{
			"chain":"smartchain",
			"address":"` + testUSDTAddress + `",
			"hot":true,
			"market":{"coingeckoId":"tether","marketCapRank":3,"currentPrice":1},
			"pairs":[{"base":"BNB"}],
			"links":[{"name":"website","url":"https://tether.to"}]
		},
		"rank":3,
		"enabled":true,
		"display":true
	}`
	rec := doHTTP(t, handler, http.MethodPost, "/api/lists/tokenlist/items", itemJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create tokenlist item status=%d body=%s", rec.Code, rec.Body.String())
	}
	var source ManagedListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &source); err != nil {
		t.Fatal(err)
	}
	if !source.Token.Hot || source.Token.Market == nil || source.Token.Market.CoinGeckoID != "tether" || len(source.Token.Pairs) != 1 || len(source.Token.Links) != 1 || source.Token.ChainName == "" {
		t.Fatalf("large tokenlist item lost UI metadata: %+v", source)
	}

	// The complete item returned by tokenlist includes timestamps. It must be
	// accepted unchanged by any target list so a UI can forward its selection.
	for _, target := range []string{"homepage", "stablecoin"} {
		rec = doHTTP(t, handler, http.MethodPost, "/api/lists/"+target+"/items", recBodyJSON(t, source))
		if rec.Code != http.StatusCreated {
			t.Fatalf("copy item to %s status=%d body=%s", target, rec.Code, rec.Body.String())
		}
		var copied ManagedListItem
		if err := json.Unmarshal(rec.Body.Bytes(), &copied); err != nil {
			t.Fatal(err)
		}
		if copied.Token.Chain != source.Token.Chain || !strings.EqualFold(copied.Token.Address, source.Token.Address) || copied.Token.Name != source.Token.Name || !copied.Token.Hot || copied.Token.Market == nil || copied.Token.Market.CoinGeckoID != "tether" || len(copied.Token.Pairs) != 1 || len(copied.Token.Links) != 1 {
			t.Fatalf("copied %s item does not match unified item structure: %+v", target, copied)
		}
	}
}

func recBodyJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestManagedListOpenAPIContractIsServed(t *testing.T) {
	rec := doHTTP(t, openAPIHandler(), http.MethodGet, "/openapi.yaml", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/yaml") {
		t.Fatalf("openapi response status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, required := range []string{
		"openapi: 3.1.0",
		"/api/lists/{listKey}:",
		"/api/lists/{listKey}/items/{chain}/{address}:",
		"/api/lists/{listKey}/includes/{tag}:",
		"/files/{outputName}.json:",
		"/files/{outputName}.json.zst:",
		"/files/manifest.json:",
		"authoritative public URL",
		"operationId: patchManagedListItem",
		"operationId: createManagedListInclude",
		"operationId: downloadPackedListZstd",
		"additionalProperties: false",
	} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(required)) {
			t.Fatalf("openapi document is missing %q", required)
		}
	}

	rec = doHTTP(t, openAPIHandler(), http.MethodHead, "/openapi.yaml", "")
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("openapi HEAD status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = doHTTP(t, openAPIHandler(), http.MethodPost, "/openapi.yaml", "")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("openapi method check status=%d allow=%q body=%s", rec.Code, rec.Header().Get("Allow"), rec.Body.String())
	}
}

func TestManagedFilesPublishJSONAndZstdContentTypes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "usdt.json"), []byte(`{"key":"usdt","items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "usdt.json.zst"), []byte{0x28, 0xb5, 0x2f, 0xfd}, 0o644); err != nil {
		t.Fatal(err)
	}
	handler := managedFilesHandler(dir)
	rec := doHTTP(t, handler, http.MethodGet, "/files/usdt.json", "")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("JSON publish status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	rec = doHTTP(t, handler, http.MethodGet, "/files/usdt.json.zst", "")
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/zstd" {
		t.Fatalf("zstd publish status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
}

func TestManagedListDisableAndDeletePrunePackedArtifacts(t *testing.T) {
	root := newFixtureRoot(t)
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	list, err := server.lists.UpsertList(ManagedList{Key: "usdt", Name: "USDT", Enabled: true})
	if err != nil {
		t.Fatalf("create list: %v", err)
	}
	if _, err := server.lists.PackAll(); err != nil {
		t.Fatalf("pack all: %v", err)
	}
	jsonPath := filepath.Join(root, "files", "usdt.json")
	if !fileExists(jsonPath) || !fileExists(jsonPath+".zst") {
		t.Fatal("expected packed artifacts")
	}

	list.Enabled = false
	if _, err := server.lists.UpsertList(*list); err != nil {
		t.Fatalf("disable list: %v", err)
	}
	if fileExists(jsonPath) || fileExists(jsonPath+".zst") {
		t.Fatal("disabled list left stale packed artifacts")
	}
	var manifest PackManifest
	if err := readJSONFile(filepath.Join(root, "files", "manifest.json"), &manifest); err != nil {
		t.Fatalf("read pruned manifest: %v", err)
	}
	if len(manifest.Files) != 0 {
		t.Fatalf("disabled list remained in manifest: %+v", manifest)
	}

	list.Enabled = true
	if _, err := server.lists.UpsertList(*list); err != nil {
		t.Fatalf("re-enable list: %v", err)
	}
	if _, err := server.lists.PackAll(); err != nil {
		t.Fatalf("repack list: %v", err)
	}
	if err := server.lists.DeleteList("usdt"); err != nil {
		t.Fatalf("delete list: %v", err)
	}
	if fileExists(jsonPath) || fileExists(jsonPath+".zst") {
		t.Fatal("deleted list left stale packed artifacts")
	}
}

func TestManagedListRESTAPIPreservesDisplayFalseAndNativeItems(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "ethereum", map[string]any{
		"name":     "Ethereum",
		"symbol":   "ETH",
		"type":     "coin",
		"decimals": 18,
		"status":   "active",
		"explorer": "https://etherscan.io/",
	})
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})

	rec := doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists", `{"key":"eth","name":"ETH List","enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create list status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodPost, "/api/lists/eth/items", `{"token":{"kind":"native","chain":"ethereum","address":""},"slot":"native","rank":1,"enabled":true,"display":false,"displaySymbol":"ETH"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create native item status=%d body=%s", rec.Code, rec.Body.String())
	}
	var item ManagedListItem
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	if item.Display || item.Token.Kind != "native" || item.Token.Address != "" || item.Token.ChainName != "Ethereum" {
		t.Fatalf("expected native item with display=false, got %+v", item)
	}

	rec = doHTTP(t, server.packAPIHandler(), http.MethodPost, "/api/pack/eth", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("pack eth status=%d body=%s", rec.Code, rec.Body.String())
	}
	var output ManagedListOutput
	if err := readJSONFile(filepath.Join(root, "files", "eth.json"), &output); err != nil {
		t.Fatalf("read eth output: %v", err)
	}
	if len(output.Items) != 1 || output.Items[0].Display || output.Items[0].Slot != "native" || output.Items[0].ChainLogoURI == "" {
		t.Fatalf("expected packed native display=false item, got %+v", output.Items)
	}

	rec = doHTTP(t, server.listAPIHandler(), http.MethodDelete, "/api/lists/eth/items/ethereum/native", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete native item status=%d body=%s", rec.Code, rec.Body.String())
	}
	items, err := server.lists.ListItems("eth")
	if err != nil {
		t.Fatalf("list eth after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected native item to be deleted, got %+v", items)
	}
}

func TestManagedListSeedsDefaultMultichainLists(t *testing.T) {
	root := newFixtureRoot(t)
	addNativeChain(t, root, "near", map[string]any{
		"name": "NEAR Protocol", "symbol": "NEAR", "type": "coin", "decimals": 24, "status": "active", "explorer": "https://nearblocks.io/",
	})
	tokenListPath := filepath.Join(root, DefaultTokenListCachePath)
	manualTokensPath := filepath.Join(root, DefaultTokenListManualTokensPath)
	homepagePath := filepath.Join(root, "data", "tokenlists", "out", "homepage.json")
	mustWriteJSON(t, tokenListPath, AppTokenList{
		Source:    "test",
		UpdatedAt: "2026-08-14T00:00:00Z",
		Tokens: []AppToken{
			{
				Kind:       "token",
				Chain:      "ethereum",
				Address:    "0xdAC17F958D2ee523a2206206994597C13D831ec7",
				AssetID:    "c60_t0xdAC17F958D2ee523a2206206994597C13D831ec7",
				Name:       "Tether",
				Symbol:     "USDT",
				Decimals:   6,
				Status:     "active",
				LogoURI:    "https://cdn.example/eth-usdt.png",
				LogoExists: true,
				Rank:       1,
				Tags:       []string{"stablecoin"},
				Hot:        true,
				Market:     &AppTokenMarket{CoinGeckoID: "tether", MarketCapRank: 3, CurrentPrice: 1},
				Pairs:      []TokenPair{{Base: "ETH"}},
				Links:      []Link{{Name: "website", URL: "https://tether.to"}},
			},
			{
				Kind:       "token",
				Chain:      "arbitrum",
				Address:    "0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9",
				AssetID:    "c10042221_t0xfd086bc7cd5c481dcc9c85ebe478a1c0b69fcbb9",
				Name:       "USDT0",
				Symbol:     "USDT0",
				Decimals:   6,
				Status:     "active",
				LogoURI:    "https://cdn.example/arb-usdt0.png",
				LogoExists: true,
				Rank:       2,
				Tags:       []string{"stablecoin"},
			},
			{
				Kind:       "native",
				Chain:      "ethereum",
				Address:    "",
				AssetID:    "c60",
				Name:       "Ethereum",
				Symbol:     "ETH",
				Decimals:   18,
				Status:     "active",
				LogoURI:    "https://cdn.example/eth.png",
				LogoExists: true,
				Rank:       3,
			},
			{
				Kind:       "token",
				Chain:      "polygon",
				Address:    "0x8f3cf7ad23cd3cadbd9735aff958023239c6a063",
				AssetID:    "c966_t0x8f3cf7ad23cd3cadbd9735aff958023239c6a063",
				Name:       "Dai Stablecoin",
				Symbol:     "DAI",
				Decimals:   18,
				Status:     "active",
				LogoExists: true,
				Tags:       []string{"stablecoin"},
			},
		},
	})
	mustWriteJSON(t, manualTokensPath, TokenListManualTokensFile{Tokens: []AppToken{{
		Kind: "token", Chain: "near", Address: "usdt.tether-token.near", AssetID: "near:usdt.tether-token.near",
		Name: "Tether USD", Symbol: "USDT", Decimals: 6, Status: "active", LogoURI: "https://cdn.example/usdt.png", LogoExists: true,
		Tags: []string{"stablecoin"}, Links: []Link{{Name: "explorer", URL: "https://nearblocks.io/token/usdt.tether-token.near"}},
	}}})
	mustWriteJSON(t, homepagePath, map[string]any{
		"tokens": []map[string]any{
			{
				"id":            "ethereum:usdt",
				"chain":         "ethereum",
				"slot":          "usdt",
				"kind":          "token",
				"displaySymbol": "USDT",
				"displayName":   "Tether",
				"symbol":        "USDT",
				"name":          "Tether",
				"address":       "0xdAC17F958D2ee523a2206206994597C13D831ec7",
				"decimals":      6,
				"logoURI":       "https://cdn.example/eth-usdt.png",
				"tags":          []string{"stablecoin"},
				"source":        "trustwallet-asset",
			},
		},
	})

	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	if err := server.lists.SeedDefaultLists(); err != nil {
		t.Fatalf("seed default lists: %v", err)
	}
	if err := server.lists.SeedDefaultLists(); err != nil {
		t.Fatalf("seed should be idempotent: %v", err)
	}

	usdt, err := server.lists.ListItems("usdt")
	if err != nil {
		t.Fatalf("list usdt: %v", err)
	}
	if len(usdt) != 3 || !usdt[0].Display || usdt[0].Slot != "usdt" {
		t.Fatalf("expected USDT list to include generated and manual USDT/USDT0, got %+v", usdt)
	}
	if !usdt[0].Token.Hot || usdt[0].Token.Market == nil || usdt[0].Token.Market.CoinGeckoID != "tether" || len(usdt[0].Token.Pairs) != 1 || len(usdt[0].Token.Links) != 1 {
		t.Fatalf("expected family list to preserve the unified tokenlist item metadata, got %+v", usdt[0])
	}
	tokenlistItems, err := server.lists.ListItems("tokenlist")
	if err != nil {
		t.Fatalf("list tokenlist: %v", err)
	}
	if len(tokenlistItems) != 5 || !tokenlistItems[0].Token.Hot || tokenlistItems[0].Token.Market == nil || len(tokenlistItems[0].Token.Pairs) != 1 || len(tokenlistItems[0].Token.Links) != 1 {
		t.Fatalf("large tokenlist items should expose complete unified metadata, got %+v", tokenlistItems)
	}
	for _, item := range usdt {
		wantDisplay := item.Token.Chain == "ethereum" || item.Token.Chain == "arbitrum"
		if item.Display != wantDisplay {
			t.Fatalf("unexpected default display for USDT on %s: got %v want %v", item.Token.Chain, item.Display, wantDisplay)
		}
	}
	usdtList, err := server.lists.GetList("usdt")
	if err != nil {
		t.Fatalf("get usdt list: %v", err)
	}
	if usdtList.DisplayName != "Tether USD" || usdtList.DisplaySymbol != "USDT" || usdtList.LogoURI != DefaultUSDTFamilyLogoURI {
		t.Fatalf("expected shared USDT presentation defaults, got %+v", usdtList)
	}
	packed, err := server.lists.PackList("usdt")
	if err != nil {
		t.Fatalf("pack usdt: %v", err)
	}
	var usdtOutput ManagedListOutput
	if err := readJSONFile(filepath.Join(root, "files", packed.JSONPath), &usdtOutput); err != nil {
		t.Fatalf("read packed usdt: %v", err)
	}
	for _, token := range usdtOutput.Items {
		if token.DisplaySymbol != "USDT" || token.LogoURI != usdtList.LogoURI {
			t.Fatalf("expected shared USDT presentation to apply to %s/%s: %+v", token.Chain, token.Address, token)
		}
	}
	for _, key := range []string{"usdc", "usdg", "usds"} {
		if _, err := server.lists.GetList(key); err != nil {
			t.Fatalf("expected default %s list: %v", key, err)
		}
	}

	stablecoins, err := server.lists.ListItems("stablecoin")
	if err != nil {
		t.Fatalf("list stablecoin: %v", err)
	}
	if len(stablecoins) != 4 {
		t.Fatalf("expected four stablecoin seed items, got %+v", stablecoins)
	}
	for _, item := range stablecoins {
		if item.Token.Chain == "polygon" && item.Token.ChainLogoURI != DefaultPolygonLogoURI {
			t.Fatalf("expected repository Polygon logo, got %+v", item.Token)
		}
	}

	ethItems, err := server.lists.ListItems("eth")
	if err != nil {
		t.Fatalf("list eth: %v", err)
	}
	if len(ethItems) != 1 || ethItems[0].Token.Kind != "native" || ethItems[0].Token.Address != "" {
		t.Fatalf("expected native ETH seed item, got %+v", ethItems)
	}

	homepage, err := server.lists.ListItems("homepage")
	if err != nil {
		t.Fatalf("list homepage: %v", err)
	}
	if len(homepage) != 1 || homepage[0].DisplaySymbol != "USDT" || homepage[0].Slot != "usdt" || !homepage[0].Display {
		t.Fatalf("expected homepage seed item with display symbol, got %+v", homepage)
	}
	homepageIncludes, err := server.lists.ListIncludes("homepage")
	if err != nil {
		t.Fatalf("list default homepage includes: %v", err)
	}
	if len(homepageIncludes) != len(defaultHomepageIncludeTags) {
		t.Fatalf("expected default homepage family includes %v, got %+v", defaultHomepageIncludeTags, homepageIncludes)
	}
	manifest, err := server.PackManagedListsOnce()
	if err != nil {
		t.Fatalf("one-shot managed list pack: %v", err)
	}
	if len(manifest.Files) != 9 {
		t.Fatalf("expected one-shot pack to publish all nine default lists, got %+v", manifest.Files)
	}
	for _, file := range manifest.Files {
		if filepath.IsAbs(file.JSONPath) || filepath.IsAbs(file.ZstdPath) || !strings.HasPrefix(file.JSONURL, "/files/") || !strings.HasPrefix(file.ZstdURL, "/files/") {
			t.Fatalf("one-shot manifest should use portable paths and default public URLs: %+v", file)
		}
	}
}

func TestManagedFamilyListMatchingAndDisplayDefaults(t *testing.T) {
	specs := map[string]seedListSpec{}
	for _, spec := range appTokenSeedLists {
		specs[spec.Key] = spec
	}
	for _, key := range []string{"usdt", "usdc", "usdg", "usds"} {
		if _, ok := specs[key]; !ok {
			t.Fatalf("missing managed family list spec %s", key)
		}
	}
	if !specs["usdc"].Match(ManagedToken{Symbol: "USDC.e"}) {
		t.Fatal("USDC family should include bridged USDC.e")
	}
	if !specs["usdg"].Match(ManagedToken{Symbol: "USDG"}) {
		t.Fatal("USDG family should include USDG")
	}
	if !specs["usds"].Match(ManagedToken{Symbol: "USDS", Name: "USDS Stablecoin"}) {
		t.Fatal("USDS family should include Sky USDS")
	}
	if specs["usds"].Match(ManagedToken{Symbol: "USDS", Name: "StableUSD"}) {
		t.Fatal("USDS family should exclude unrelated tokens that reuse the symbol")
	}
	for _, chain := range []string{"arbitrum", "polygon", "smartchain", "ethereum", "tron"} {
		for _, key := range []string{"usdt", "usdc", "usdg", "usds"} {
			if !defaultManagedListDisplay(key, chain) {
				t.Fatalf("expected %s/%s to display by default", key, chain)
			}
		}
	}
	for _, chain := range []string{"base", "solana", "near", "optimism"} {
		for _, key := range []string{"usdt", "usdc", "usdg", "usds"} {
			if defaultManagedListDisplay(key, chain) {
				t.Fatalf("expected %s/%s to be hidden by default", key, chain)
			}
		}
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

func doHTTP(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
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

func boolPtr(value bool) *bool {
	return &value
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
