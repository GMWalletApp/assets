package rpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Addr                         string
	Root                         string
	AssetBaseURL                 string
	MarketSyncEnabled            bool
	MarketSyncInterval           time.Duration
	MarketCachePath              string
	TokenListCachePath           string
	TokenListReportPath          string
	TokenListRulesPath           string
	TokenListBaseOverridesPath   string
	TokenListManualOverridesPath string
	TokenListManualTokensPath    string
	TokenListHotDefaultsPath     string
	TokenListHotCurrentPath      string
	VsCurrency                   string
	CoinGeckoAPIKey              string
	CoinGeckoBaseURL             string
	CoinGeckoKeyHeader           string
	DefiLlamaBaseURL             string
	MarketLimit                  int
}

type SyncTarget string

const (
	SyncTargetAll       SyncTarget = "all"
	SyncTargetMarket    SyncTarget = "market"
	SyncTargetTokenList SyncTarget = "tokenlist"
)

const (
	DefaultAssetBaseURL                 = "https://assets-cdn.trustwallet.com"
	DefaultMarketCachePath              = "extensions/jsonrpc/data/market.json"
	DefaultTokenListCachePath           = "extensions/jsonrpc/data/tokenlist.json"
	DefaultTokenListReportPath          = "extensions/jsonrpc/data/tokenlist-report.json"
	DefaultTokenListRulesPath           = "extensions/jsonrpc/config/tokenlist-rules.json"
	DefaultTokenListBaseOverridesPath   = "extensions/jsonrpc/config/tokenlist-base-overrides.json"
	DefaultTokenListManualOverridesPath = "extensions/jsonrpc/config/tokenlist-manual-overrides.json"
	DefaultTokenListManualTokensPath    = "extensions/jsonrpc/config/tokenlist-manual-tokens.json"
	DefaultTokenListHotDefaultsPath     = "extensions/jsonrpc/config/tokenlist-hot-defaults.json"
	DefaultTokenListHotCurrentPath      = "extensions/jsonrpc/config/tokenlist-hot-current.json"
)

type Server struct {
	config Config
	store  *Store
	cache  *CacheStore
	syncer *Syncer
}

func NewServer(config Config) *Server {
	if config.Addr == "" {
		config.Addr = ":8080"
	}
	if config.Root == "" {
		config.Root = "."
	}
	if config.AssetBaseURL == "" {
		config.AssetBaseURL = DefaultAssetBaseURL
	}
	if config.MarketCachePath == "" {
		config.MarketCachePath = DefaultMarketCachePath
	}
	if config.TokenListCachePath == "" {
		config.TokenListCachePath = DefaultTokenListCachePath
	}
	if config.TokenListReportPath == "" {
		config.TokenListReportPath = DefaultTokenListReportPath
	}
	if config.TokenListRulesPath == "" {
		config.TokenListRulesPath = DefaultTokenListRulesPath
	}
	if config.TokenListBaseOverridesPath == "" {
		config.TokenListBaseOverridesPath = DefaultTokenListBaseOverridesPath
	}
	if config.TokenListManualOverridesPath == "" {
		config.TokenListManualOverridesPath = DefaultTokenListManualOverridesPath
	}
	if config.TokenListManualTokensPath == "" {
		config.TokenListManualTokensPath = DefaultTokenListManualTokensPath
	}
	if config.TokenListHotDefaultsPath == "" {
		config.TokenListHotDefaultsPath = DefaultTokenListHotDefaultsPath
	}
	if config.TokenListHotCurrentPath == "" {
		config.TokenListHotCurrentPath = DefaultTokenListHotCurrentPath
	}
	if config.VsCurrency == "" {
		config.VsCurrency = "usd"
	}
	if config.CoinGeckoAPIKey == "" {
		config.CoinGeckoAPIKey = CoinGeckoKeyFromEnv()
	}
	if config.CoinGeckoBaseURL == "" {
		config.CoinGeckoBaseURL = CoinGeckoBaseURLFromEnv()
	}
	if config.CoinGeckoKeyHeader == "" {
		config.CoinGeckoKeyHeader = CoinGeckoKeyHeaderFromEnv()
	}
	if config.DefiLlamaBaseURL == "" {
		config.DefiLlamaBaseURL = DefiLlamaBaseURLFromEnv()
	}
	config.MarketCachePath = resolveCachePath(config.Root, config.MarketCachePath)
	config.TokenListCachePath = resolveCachePath(config.Root, config.TokenListCachePath)
	config.TokenListReportPath = resolveCachePath(config.Root, config.TokenListReportPath)
	config.TokenListRulesPath = resolveCachePath(config.Root, config.TokenListRulesPath)
	config.TokenListBaseOverridesPath = resolveCachePath(config.Root, config.TokenListBaseOverridesPath)
	config.TokenListManualOverridesPath = resolveCachePath(config.Root, config.TokenListManualOverridesPath)
	config.TokenListManualTokensPath = resolveCachePath(config.Root, config.TokenListManualTokensPath)
	config.TokenListHotDefaultsPath = resolveCachePath(config.Root, config.TokenListHotDefaultsPath)
	config.TokenListHotCurrentPath = resolveCachePath(config.Root, config.TokenListHotCurrentPath)

	store := NewStore(config.Root, config.AssetBaseURL)
	cache := NewCacheStore(config.MarketCachePath)
	syncer := NewSyncer(store, SyncConfig{
		Enabled:                      config.MarketSyncEnabled,
		Interval:                     config.MarketSyncInterval,
		MarketCachePath:              config.MarketCachePath,
		TokenListCachePath:           config.TokenListCachePath,
		TokenListReportPath:          config.TokenListReportPath,
		TokenListRulesPath:           config.TokenListRulesPath,
		TokenListBaseOverridesPath:   config.TokenListBaseOverridesPath,
		TokenListManualOverridesPath: config.TokenListManualOverridesPath,
		TokenListManualTokensPath:    config.TokenListManualTokensPath,
		TokenListHotDefaultsPath:     config.TokenListHotDefaultsPath,
		TokenListHotCurrentPath:      config.TokenListHotCurrentPath,
		VsCurrency:                   config.VsCurrency,
		CoinGeckoAPIKey:              config.CoinGeckoAPIKey,
		CoinGeckoBaseURL:             config.CoinGeckoBaseURL,
		CoinGeckoKeyHeader:           config.CoinGeckoKeyHeader,
		DefiLlamaBaseURL:             config.DefiLlamaBaseURL,
		MarketLimit:                  config.MarketLimit,
	})

	return &Server{
		config: config,
		store:  store,
		cache:  cache,
		syncer: syncer,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.Handle("/rpc", s.Handler())

	if s.config.MarketSyncEnabled {
		go s.syncer.Run(ctx)
	}

	server := &http.Server{
		Addr:              s.config.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("rpc server shutdown failed: %v", err)
		}
	}()

	log.Printf("starting JSON-RPC HTTP server on %s", s.config.Addr)
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func ParseSyncTarget(value string) (SyncTarget, error) {
	switch SyncTarget(strings.ToLower(strings.TrimSpace(value))) {
	case "", SyncTargetAll:
		return SyncTargetAll, nil
	case SyncTargetMarket:
		return SyncTargetMarket, nil
	case SyncTargetTokenList:
		return SyncTargetTokenList, nil
	default:
		return "", fmt.Errorf("unsupported sync target %q", value)
	}
}

func (s *Server) SyncOnce(ctx context.Context, target SyncTarget) error {
	switch target {
	case "", SyncTargetAll:
		if err := s.syncer.SyncMarket(ctx); err != nil {
			return fmt.Errorf("market sync failed: %w", err)
		}
		if err := s.syncer.SyncTokenList(ctx); err != nil {
			return fmt.Errorf("tokenlist sync failed: %w", err)
		}
		return nil
	case SyncTargetMarket:
		if err := s.syncer.SyncMarket(ctx); err != nil {
			return fmt.Errorf("market sync failed: %w", err)
		}
		return nil
	case SyncTargetTokenList:
		if err := s.syncer.SyncTokenList(ctx); err != nil {
			return fmt.Errorf("tokenlist sync failed: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported sync target %q", target)
	}
}

func (s *Server) call(method string, params []byte) (any, *RPCError) {
	switch method {
	case "listChains":
		return s.listChains()
	case "getChainInfo":
		return s.getChainInfo(params)
	case "getAssetByAddress":
		return s.getAssetByAddress(params)
	case "getAssetById":
		return s.getAssetByID(params)
	case "getTokenList":
		return s.getTokenList(params)
	case "getMarketRankings":
		return s.getMarketRankings(params)
	case "getAssetMarket":
		return s.getAssetMarket(params)
	case "getAppTokenList":
		return s.getAppTokenList(params)
	default:
		return nil, &RPCError{Code: ErrCodeMethodNotFound, Message: "method not found"}
	}
}

func (s *Server) listChains() (any, *RPCError) {
	result, err := s.store.ListChains()
	return result, toRPCError(err)
}

func (s *Server) getChainInfo(params []byte) (any, *RPCError) {
	var p struct {
		Chain string `json:"chain"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	result, readErr := s.store.GetChainInfo(p.Chain)
	return result, toRPCError(readErr)
}

func (s *Server) getAssetByAddress(params []byte) (any, *RPCError) {
	var p struct {
		Chain   string `json:"chain"`
		Address string `json:"address"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	result, readErr := s.store.GetAssetByAddress(p.Chain, p.Address)
	return result, toRPCError(readErr)
}

func (s *Server) getAssetByID(params []byte) (any, *RPCError) {
	var p struct {
		AssetID string `json:"assetId"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	result, readErr := s.store.GetAssetByID(p.AssetID)
	return result, toRPCError(readErr)
}

func (s *Server) getTokenList(params []byte) (any, *RPCError) {
	var p struct {
		Chain    string `json:"chain"`
		Extended bool   `json:"extended"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	result, readErr := s.store.GetTokenList(p.Chain, p.Extended)
	return result, toRPCError(readErr)
}

func (s *Server) getMarketRankings(params []byte) (any, *RPCError) {
	var p struct {
		Order          string `json:"order"`
		Limit          int    `json:"limit"`
		Offset         int    `json:"offset"`
		OnlyWithAssets bool   `json:"onlyWithAssets"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}
	if p.Order == "" {
		p.Order = "market_cap_desc"
	}
	if p.Order != "market_cap_desc" && p.Order != "volume_desc" && p.Order != "market_cap_rank_asc" {
		return nil, invalidParams("unsupported order")
	}

	cache, readErr := s.cache.ReadMarket()
	if readErr != nil {
		return nil, toRPCError(readErr)
	}
	return filterMarketRankings(cache, p.Order, p.Limit, p.Offset, p.OnlyWithAssets), nil
}

func (s *Server) getAssetMarket(params []byte) (any, *RPCError) {
	var p struct {
		Chain   string `json:"chain"`
		Address string `json:"address"`
		AssetID string `json:"assetId"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}

	var assetDetail *AssetDetail
	var err error
	if p.AssetID != "" {
		assetDetail, err = s.store.GetAssetByID(p.AssetID)
	} else {
		assetDetail, err = s.store.GetAssetByAddress(p.Chain, p.Address)
	}
	if err != nil {
		return nil, toRPCError(err)
	}

	cache, readErr := s.cache.ReadMarket()
	if readErr != nil {
		return nil, toRPCError(readErr)
	}
	matches := findMarketByAsset(cache, assetDetail.Chain, assetDetail.Address)
	if len(matches) == 0 {
		return nil, notFound("asset market data not found")
	}
	return matches, nil
}

func (s *Server) getAppTokenList(params []byte) (any, *RPCError) {
	var p struct {
		Chain          string `json:"chain"`
		Limit          int    `json:"limit"`
		Offset         int    `json:"offset"`
		MaxRank        int    `json:"maxRank"`
		OnlyWithMarket bool   `json:"onlyWithMarket"`
	}
	if err := decodeParams(params, &p); err != nil {
		return nil, err
	}

	var cache AppTokenList
	if err := readJSONFile(s.config.TokenListCachePath, &cache); err != nil {
		if os.IsNotExist(err) {
			return &AppTokenList{Source: "trustwallet+coingecko", Tokens: []AppToken{}}, nil
		}
		return nil, toRPCError(err)
	}

	return filterAppTokenList(&cache, p.Chain, p.Limit, p.Offset, p.MaxRank, p.OnlyWithMarket), nil
}

func toRPCError(err error) *RPCError {
	if err == nil {
		return nil
	}
	var rpcErr *RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr
	}
	return internalError(err.Error())
}

func stringsEqualFold(a, b string) bool {
	return len(a) == len(b) && (a == b || strings.EqualFold(a, b))
}

func decodeParams(params []byte, target any) *RPCError {
	if len(params) == 0 {
		params = []byte("{}")
	}
	if err := json.Unmarshal(params, target); err != nil {
		return invalidParams(fmt.Sprintf("invalid params: %v", err))
	}
	return nil
}

func resolveCachePath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
