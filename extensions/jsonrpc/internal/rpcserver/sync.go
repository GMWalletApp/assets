package rpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultCoinGeckoBaseURL   = "https://api.coingecko.com/api/v3"
	DefaultCoinGeckoKeyHeader = "x-cg-demo-api-key"
	DefaultDefiLlamaBaseURL   = "https://stablecoins.llama.fi"
	DefaultMarketLimit        = 1000
	defaultHTTPClientTimeout  = 30 * time.Second
	defaultMarketPerPage      = 250
)

type SyncConfig struct {
	Enabled                      bool
	Interval                     time.Duration
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
	CoinGeckoKeyHeader           string
	CoinGeckoBaseURL             string
	DefiLlamaBaseURL             string
	MarketLimit                  int
}

type Syncer struct {
	store  *Store
	config SyncConfig
	client *http.Client
}

type coinGeckoDataset struct {
	Markets       []coinGeckoMarket
	Coins         []coinGeckoListItem
	CoinByID      map[string]coinGeckoListItem
	PlatformsByID map[string]map[string]string
}

func NewSyncer(store *Store, config SyncConfig) *Syncer {
	if config.Interval <= 0 {
		config.Interval = 6 * time.Hour
	}
	if config.VsCurrency == "" {
		config.VsCurrency = "usd"
	}
	if config.CoinGeckoBaseURL == "" {
		config.CoinGeckoBaseURL = DefaultCoinGeckoBaseURL
	}
	if config.CoinGeckoKeyHeader == "" {
		config.CoinGeckoKeyHeader = DefaultCoinGeckoKeyHeader
	}
	if config.DefiLlamaBaseURL == "" {
		config.DefiLlamaBaseURL = DefaultDefiLlamaBaseURL
	}
	if config.MarketLimit <= 0 {
		config.MarketLimit = DefaultMarketLimit
	}

	return &Syncer{
		store:  store,
		config: config,
		client: &http.Client{Timeout: defaultHTTPClientTimeout},
	}
}

func (s *Syncer) loadTokenListConfig() (*ResolvedTokenListConfig, error) {
	return loadResolvedTokenListConfig(
		s.config.TokenListRulesPath,
		s.config.TokenListBaseOverridesPath,
		s.config.TokenListManualOverridesPath,
		s.config.TokenListHotDefaultsPath,
		s.config.TokenListHotCurrentPath,
	)
}

func (s *Syncer) Run(ctx context.Context) {
	if !s.config.Enabled {
		return
	}

	s.syncOnce(ctx)

	ticker := time.NewTicker(s.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncOnce(ctx)
		}
	}
}

func (s *Syncer) syncOnce(ctx context.Context) {
	if err := s.SyncMarket(ctx); err != nil {
		log.Printf("market sync failed: %v", err)
	}
	if err := s.SyncTokenList(ctx); err != nil {
		log.Printf("tokenlist sync failed: %v", err)
	}
}

func (s *Syncer) SyncMarket(ctx context.Context) error {
	if s.config.CoinGeckoAPIKey == "" {
		log.Print("COINGECKO_API_KEY is not set; skipping market sync")
		return nil
	}

	index, err := s.store.BuildAssetIndex()
	if err != nil {
		return err
	}
	config, err := s.loadTokenListConfig()
	if err != nil {
		return err
	}

	coingecko, err := s.fetchCoinGeckoDataset(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cache := MarketCache{
		Source:     "coingecko",
		VsCurrency: s.config.VsCurrency,
		UpdatedAt:  now,
		Assets:     make([]MarketAsset, 0, len(coingecko.Markets)),
	}

	for i, market := range coingecko.Markets {
		rank := market.MarketCapRank
		if rank == 0 {
			rank = i + 1
		}
		assets := index.MatchNativeMarketWithRules(market.ID, config)
		assets = appendUniqueAssets(assets, index.MatchMarketPlatformsWithRules(coingecko.PlatformsByID[market.ID], config))
		assets = appendUniqueAssets(assets, index.MatchExternalMarket(market.ID))
		assets = applyAssetDetailRulesToAssets(assets, market.ID, config)
		cache.Assets = append(cache.Assets, MarketAsset{
			Rank:          rank,
			Source:        "coingecko",
			CoinGeckoID:   market.ID,
			Symbol:        strings.ToUpper(market.Symbol),
			Name:          market.Name,
			MarketCapRank: market.MarketCapRank,
			MarketCap:     market.MarketCap,
			TotalVolume:   market.TotalVolume,
			CurrentPrice:  market.CurrentPrice,
			LastUpdated:   market.LastUpdated,
			UpdatedAt:     now,
			Assets:        assets,
		})
	}

	return writeJSONAtomic(s.config.MarketCachePath, cache)
}

func (s *Syncer) SyncTokenList(ctx context.Context) error {
	index, err := s.store.BuildAssetIndex()
	if err != nil {
		return err
	}
	config, err := s.loadTokenListConfig()
	if err != nil {
		return err
	}
	manualTokens, err := loadTokenListManualTokens(s.config.TokenListManualTokensPath)
	if err != nil {
		return err
	}
	localAssetKeys := make(map[string]struct{}, len(index.nativeAssets)+len(index.tokenAssets))
	for _, asset := range index.NativeAssets() {
		localAssetKeys[chainAddressKey(asset.Chain, asset.Address)] = struct{}{}
	}
	for _, asset := range index.TokenAssets() {
		localAssetKeys[chainAddressKey(asset.Chain, asset.Address)] = struct{}{}
	}
	if err := validateTokenListManualTokens(s.store.root, manualTokens, localAssetKeys, true, s.config.TokenListManualTokensPath); err != nil {
		return err
	}
	pairsByAssetID, err := s.store.TokenListPairsByAssetID()
	if err != nil {
		return err
	}

	var coingecko *coinGeckoDataset
	if s.config.CoinGeckoAPIKey == "" {
		log.Print("COINGECKO_API_KEY is not set; tokenlist sync will skip market enrichment")
	} else {
		coingecko, err = s.fetchCoinGeckoDataset(ctx)
		if err != nil {
			return err
		}
	}
	coinList := []coinGeckoListItem(nil)
	if coingecko != nil {
		coinList = coingecko.Coins
	} else {
		coinList, err = s.fetchCoinGeckoCoinList(ctx)
		if err != nil {
			return err
		}
	}
	stablecoins, err := s.fetchDefiLlamaStablecoins(ctx)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	tokenList, report := s.buildAppTokenList(index, coingecko, coinList, stablecoins, pairsByAssetID, config, manualTokens, now)
	if err := writeJSONAtomic(s.config.TokenListCachePath, tokenList); err != nil {
		return err
	}
	return writeJSONAtomic(s.config.TokenListReportPath, report)
}

func (s *Syncer) buildAppTokenList(index *AssetIndex, coingecko *coinGeckoDataset, coinList []coinGeckoListItem, stablecoins *defiLlamaStablecoinsResponse, pairsByAssetID map[string][]TokenPair, config *ResolvedTokenListConfig, manualTokens []AppToken, now string) (*AppTokenList, *TokenListReport) {
	report := &TokenListReport{
		Source:    "trustwallet+coingecko",
		UpdatedAt: now,
		APIs:      s.tokenListReportAPIs(coingecko != nil),
		Rules:     config.ruleStats(),
	}
	report.Local.NativeAssets = len(index.nativeAssets)
	report.Local.TokenAssets = len(index.tokenAssets)
	report.Hot.DefaultEntries = len(config.HotDefaults)
	report.Hot.CurrentEntries = len(config.HotCurrent)
	if coingecko != nil {
		report.Market.Rows = len(coingecko.Markets)
	}
	reportTokenListRuleIssues(index, coingecko, config, report)

	marketByAsset := s.buildMarketByAsset(index, coingecko, config, report)
	stablecoinAssetKeys := buildStablecoinAssetKeys(index, coinList, stablecoins, config)
	hotAssetKeys := buildHotAssetKeys(index, config.HotEntries, report)

	tokens := make([]AppToken, 0, len(index.nativeAssets)+len(index.tokenAssets))
	for _, asset := range append(index.NativeAssets(), index.TokenAssets()...) {
		kind := "token"
		if asset.Address == "" {
			kind = "native"
		}
		if config.isExcludedStatus(asset.Status) {
			report.Local.Filtered++
			report.Issues.FilteredAssets = append(report.Issues.FilteredAssets, reportAssetRef(kind, asset))
			continue
		}
		key := assetLookupKey(asset)
		token := AppToken{
			Kind:       kind,
			Chain:      asset.Chain,
			Hot:        false,
			Address:    asset.Address,
			AssetID:    asset.AssetID,
			Type:       asset.Type,
			Name:       asset.Name,
			Symbol:     asset.Symbol,
			Decimals:   asset.Decimals,
			Status:     asset.Status,
			LogoURI:    asset.LogoURI,
			LogoExists: asset.LogoExists,
			Pairs:      pairsByAssetID[asset.AssetID],
			Tags:       removeStringTag(append([]string(nil), asset.Tags...), "hot"),
			Links:      asset.Links,
		}
		if market := marketByAsset[key]; market != nil {
			token.Market = market
			token.Rank = market.MarketCapRank
		}
		applyTokenListRules(&token, config, report)
		if _, ok := stablecoinAssetKeys[key]; ok {
			token.Tags = appendUniqueStrings(token.Tags, "stablecoin")
		}
		if _, ok := hotAssetKeys[key]; ok {
			token.Hot = true
		}
		token.Tags = removeStringTag(token.Tags, "hot")
		if !asset.LogoExists {
			report.Local.MissingLogos++
			report.Issues.MissingLogos = append(report.Issues.MissingLogos, reportAssetRef(kind, asset))
		}
		tokens = append(tokens, token)
		if token.Rank > 0 {
			report.Market.RankedAssets++
		}
		if hasTag(token.Tags, "stablecoin") {
			report.Stablecoin.TaggedAssets++
		}
		if token.Hot {
			report.Hot.EnabledAssets++
		}
	}

	sort.SliceStable(tokens, func(i, j int) bool {
		if tokens[i].Rank != 0 && tokens[j].Rank != 0 && tokens[i].Rank != tokens[j].Rank {
			return tokens[i].Rank < tokens[j].Rank
		}
		if tokens[i].Rank != 0 {
			return true
		}
		if tokens[j].Rank != 0 {
			return false
		}
		if tokens[i].Chain != tokens[j].Chain {
			return tokens[i].Chain < tokens[j].Chain
		}
		if tokens[i].Symbol != tokens[j].Symbol {
			return tokens[i].Symbol < tokens[j].Symbol
		}
		if tokens[i].Name != tokens[j].Name {
			return tokens[i].Name < tokens[j].Name
		}
		return strings.ToLower(tokens[i].Address) < strings.ToLower(tokens[j].Address)
	})

	for _, token := range manualTokens {
		normalizeTokenListManualToken(&token)
		tokens = append(tokens, token)
		if token.Rank > 0 {
			report.Market.RankedAssets++
		}
		if hasTag(token.Tags, "stablecoin") {
			report.Stablecoin.TaggedAssets++
		}
		if token.Hot {
			report.Hot.EnabledAssets++
		}
	}

	report.Local.OutputTokens = len(tokens)
	return &AppTokenList{
		Source:    report.Source,
		UpdatedAt: now,
		Tokens:    tokens,
	}, report
}

func (s *Syncer) tokenListReportAPIs(includeMarket bool) []ReportAPIRequest {
	apis := make([]ReportAPIRequest, 0, 3)
	if includeMarket {
		apis = append(apis, ReportAPIRequest{
			Source: "coingecko",
			URL:    strings.TrimRight(s.config.CoinGeckoBaseURL, "/") + "/coins/markets",
			Params: map[string]string{
				"vs_currency": s.config.VsCurrency,
				"order":       "market_cap_desc",
				"limit":       strconv.Itoa(s.marketLimit()),
				"sparkline":   "false",
			},
		})
	}
	apis = append(apis,
		ReportAPIRequest{
			Source: "coingecko",
			URL:    strings.TrimRight(s.config.CoinGeckoBaseURL, "/") + "/coins/list",
			Params: map[string]string{"include_platform": "true"},
		},
		ReportAPIRequest{
			Source: "defillama",
			URL:    strings.TrimRight(s.config.DefiLlamaBaseURL, "/") + "/stablecoins",
			Params: map[string]string{"includePrices": "true"},
		},
	)
	return apis
}

func (s *Syncer) buildMarketByAsset(index *AssetIndex, coingecko *coinGeckoDataset, config *ResolvedTokenListConfig, report *TokenListReport) map[string]*AppTokenMarket {
	result := map[string]*AppTokenMarket{}
	if coingecko == nil {
		return result
	}
	for i, market := range coingecko.Markets {
		rank := market.MarketCapRank
		if rank == 0 {
			rank = i + 1
		}
		enrichment := &AppTokenMarket{
			Source:        "coingecko",
			CoinGeckoID:   market.ID,
			MarketCapRank: rank,
			MarketCap:     market.MarketCap,
			TotalVolume:   market.TotalVolume,
			CurrentPrice:  market.CurrentPrice,
			LastUpdated:   market.LastUpdated,
		}

		matched := false
		seen := map[string]struct{}{}
		nativeMatches, usedNativeRule := matchNativeMarketWithRules(index, market.ID, config)
		if count := setBestMarketMatches(result, seen, nativeMatches, enrichment); count > 0 {
			matched = true
			report.Market.NativeMatches += count
			if usedNativeRule {
				report.Rules.NativeMarketMappingHits += count
			}
		}

		tokenMatches := matchCoinGeckoPlatforms(index, market.ID, market.Symbol, market.Name, coingecko.PlatformsByID[market.ID], config, report)
		if count := setBestMarketMatches(result, seen, tokenMatches, enrichment); count > 0 {
			matched = true
			report.Market.TokenMatches += count
		}

		linkMatches := index.MatchExternalMarket(market.ID)
		if count := setBestMarketMatches(result, seen, linkMatches, enrichment); count > 0 {
			matched = true
			report.Market.TokenMatches += count
		}

		overrideMatches := matchAssetOverrideMarket(index, config, market.ID)
		if count := setBestMarketMatches(result, seen, overrideMatches, enrichment); count > 0 {
			matched = true
			report.Market.TokenMatches += count
			report.Rules.AssetOverrideMarketHits += count
		}

		if !matched {
			report.Market.UnmatchedRows++
			report.Issues.UnmatchedMarketRows = append(report.Issues.UnmatchedMarketRows, ReportMarketRef{
				CoinGeckoID: market.ID,
				Symbol:      strings.ToUpper(market.Symbol),
				Name:        market.Name,
				Rank:        rank,
			})
		}
	}
	return result
}

func buildStablecoinAssetKeys(index *AssetIndex, coinList []coinGeckoListItem, stablecoins *defiLlamaStablecoinsResponse, config *ResolvedTokenListConfig) map[string]struct{} {
	keys := map[string]struct{}{}
	if index == nil || stablecoins == nil {
		return keys
	}

	platformsByID := platformsByCoinID(coinList)
	for _, stablecoin := range stablecoins.PeggedAssets {
		coinGeckoID := stablecoin.CoinGeckoID()
		if coinGeckoID == "" {
			continue
		}

		assets := appendUniqueAssets(index.MatchMarketPlatformsWithRules(platformsByID[coinGeckoID], config), index.MatchExternalMarket(coinGeckoID))
		assets = appendUniqueAssets(assets, matchAssetOverrideMarket(index, config, coinGeckoID))
		for _, asset := range assets {
			keys[assetLookupKey(asset)] = struct{}{}
		}
	}

	return keys
}

func buildHotAssetKeys(index *AssetIndex, hotEntries []TokenListHotEntry, report *TokenListReport) map[string]struct{} {
	keys := map[string]struct{}{}
	if index == nil {
		return keys
	}
	for _, item := range hotEntries {
		asset, ok := index.byChainAndAddress[chainAddressKey(item.Chain, item.Address)]
		if !ok {
			if report != nil {
				kind := "token"
				if strings.TrimSpace(item.Address) == "" {
					kind = "native"
				}
				report.Issues.MissingHotAssets = append(report.Issues.MissingHotAssets, ReportAssetRef{
					Kind:    kind,
					Chain:   item.Chain,
					Address: item.Address,
				})
			}
			continue
		}
		keys[assetLookupKey(asset)] = struct{}{}
	}
	return keys
}

func matchCoinGeckoPlatforms(index *AssetIndex, coinGeckoID, symbol, name string, platforms map[string]string, config *ResolvedTokenListConfig, report *TokenListReport) []AssetDetail {
	var matches []AssetDetail
	for platform, address := range platforms {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		chain, usedRule, ok := coinGeckoPlatformChainWithRules(platform, config)
		if !ok {
			if report != nil {
				report.Market.UnmappedPlatform++
				report.Issues.UnmappedPlatforms = append(report.Issues.UnmappedPlatforms, ReportPlatformRef{
					CoinGeckoID: coinGeckoID,
					Symbol:      strings.ToUpper(symbol),
					Name:        name,
					Platform:    platform,
					Address:     address,
				})
			}
			continue
		}
		asset, ok := index.byChainAndAddress[chainAddressKey(chain, address)]
		if !ok {
			if report != nil {
				report.Market.MissingAssets++
				report.Issues.MissingPlatformAssets = append(report.Issues.MissingPlatformAssets, ReportPlatformRef{
					CoinGeckoID: coinGeckoID,
					Symbol:      strings.ToUpper(symbol),
					Name:        name,
					Platform:    platform,
					Chain:       chain,
					Address:     address,
				})
			}
			continue
		}
		if usedRule && report != nil {
			report.Rules.PlatformMappingHits++
		}
		matches = appendUniqueAsset(matches, asset)
	}
	return matches
}

func matchNativeMarketWithRules(index *AssetIndex, coingeckoID string, config *ResolvedTokenListConfig) ([]AssetDetail, bool) {
	chains, usedRule := coinGeckoNativeChainsWithRules(coingeckoID, config)
	matches := make([]AssetDetail, 0, len(chains))
	for _, chain := range chains {
		if asset, ok := index.byChainAndAddress[chainAddressKey(chain, "")]; ok {
			matches = append(matches, asset)
		}
	}
	return matches, usedRule
}

func matchAssetOverrideMarket(index *AssetIndex, config *ResolvedTokenListConfig, coingeckoID string) []AssetDetail {
	if config == nil {
		return nil
	}
	coingeckoID = normalizeExternalID(coingeckoID)
	var matches []AssetDetail
	for _, override := range config.AssetOverrides {
		if override.CoinGeckoID != coingeckoID {
			continue
		}
		if asset, ok := index.byChainAndAddress[chainAddressKey(override.Chain, override.Address)]; ok {
			matches = appendUniqueAsset(matches, asset)
		}
	}
	sortAssetDetails(matches)
	return matches
}

func applyTokenListRules(token *AppToken, config *ResolvedTokenListConfig, report *TokenListReport) {
	if token == nil || config == nil {
		return
	}

	coinGeckoID := ""
	if token.Market != nil {
		coinGeckoID = token.Market.CoinGeckoID
	}

	if override, ok := config.assetOverride(token.Chain, token.Address); ok {
		report.Rules.AssetOverrideHits++
		if override.DisplayName != "" {
			token.Name = override.DisplayName
		}
		if override.DisplaySymbol != "" {
			token.Symbol = override.DisplaySymbol
		}
		token.Tags = appendUniqueStrings(token.Tags, override.AddTags...)
		if coinGeckoID == "" {
			coinGeckoID = override.CoinGeckoID
		}
	}

	for _, rule := range config.marketTagRules(coinGeckoID) {
		if len(rule.AddTags) == 0 {
			continue
		}
		before := len(token.Tags)
		token.Tags = appendUniqueStrings(token.Tags, rule.AddTags...)
		if len(token.Tags) != before {
			report.Rules.MarketTagRuleHits++
		}
	}
}

func applyAssetDetailRulesToAssets(assets []AssetDetail, coingeckoID string, config *ResolvedTokenListConfig) []AssetDetail {
	if config == nil || len(assets) == 0 {
		return assets
	}
	out := append([]AssetDetail(nil), assets...)
	for i := range out {
		applyAssetDetailRules(&out[i], coingeckoID, config)
	}
	return out
}

func applyAssetDetailRules(asset *AssetDetail, coingeckoID string, config *ResolvedTokenListConfig) {
	if asset == nil || config == nil {
		return
	}

	if override, ok := config.assetOverride(asset.Chain, asset.Address); ok {
		if override.DisplayName != "" {
			asset.Name = override.DisplayName
		}
		if override.DisplaySymbol != "" {
			asset.Symbol = override.DisplaySymbol
		}
		asset.Tags = appendUniqueStrings(asset.Tags, override.AddTags...)
		if coingeckoID == "" {
			coingeckoID = override.CoinGeckoID
		}
	}

	for _, rule := range config.marketTagRules(coingeckoID) {
		asset.Tags = appendUniqueStrings(asset.Tags, rule.AddTags...)
	}
}

func reportTokenListRuleIssues(index *AssetIndex, coingecko *coinGeckoDataset, config *ResolvedTokenListConfig, report *TokenListReport) {
	if config == nil || report == nil {
		return
	}

	marketIDs := map[string]struct{}{}
	if coingecko != nil {
		for _, market := range coingecko.Markets {
			marketIDs[normalizeExternalID(market.ID)] = struct{}{}
		}
	}

	for platform, chain := range config.PlatformMappings {
		if _, ok := index.byChainAndAddress[chainAddressKey(chain, "")]; !ok {
			report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
				Rule:   "platformMappings",
				Reason: "target chain not found",
				Chain:  chain,
			})
		}
		if platform == "" {
			report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
				Rule:   "platformMappings",
				Reason: "empty platform",
				Chain:  chain,
			})
		}
	}

	for coingeckoID, chains := range config.NativeMarketMappings {
		if len(marketIDs) > 0 {
			if _, ok := marketIDs[coingeckoID]; !ok {
				report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
					Rule:        "nativeMarketMappings",
					Reason:      "coingecko market not in synced market window",
					CoinGeckoID: coingeckoID,
				})
			}
		}
		for _, chain := range chains {
			if _, ok := index.byChainAndAddress[chainAddressKey(chain, "")]; !ok {
				report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
					Rule:        "nativeMarketMappings",
					Reason:      "target chain not found",
					Chain:       chain,
					CoinGeckoID: coingeckoID,
				})
			}
		}
	}

	for _, override := range config.AssetOverrides {
		if override.Chain == "" || override.Address == "" {
			report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
				Rule:        "assetOverrides",
				Reason:      "chain and address are required",
				Chain:       override.Chain,
				Address:     override.Address,
				CoinGeckoID: override.CoinGeckoID,
			})
			continue
		}
		if _, ok := index.byChainAndAddress[chainAddressKey(override.Chain, override.Address)]; !ok {
			report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
				Rule:        "assetOverrides",
				Reason:      "asset not found",
				Chain:       override.Chain,
				Address:     override.Address,
				CoinGeckoID: override.CoinGeckoID,
			})
		}
		if override.CoinGeckoID != "" {
			if len(marketIDs) > 0 {
				if _, ok := marketIDs[override.CoinGeckoID]; !ok {
					report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
						Rule:        "assetOverrides",
						Reason:      "coingecko market not in synced market window",
						Chain:       override.Chain,
						Address:     override.Address,
						CoinGeckoID: override.CoinGeckoID,
					})
				}
			}
		}
	}

	for _, rule := range config.MarketTagRules {
		if rule.CoinGeckoID == "" {
			report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
				Rule:   "marketTagRules",
				Reason: "coingeckoId is required",
			})
			continue
		}
		if len(marketIDs) > 0 {
			if _, ok := marketIDs[rule.CoinGeckoID]; !ok {
				report.Issues.RuleIssues = append(report.Issues.RuleIssues, ReportRuleIssue{
					Rule:        "marketTagRules",
					Reason:      "coingecko market not in synced market window",
					CoinGeckoID: rule.CoinGeckoID,
				})
			}
		}
	}
}

func setBestMarket(markets map[string]*AppTokenMarket, key string, market *AppTokenMarket) {
	existing := markets[key]
	if existing == nil || existing.MarketCapRank == 0 || (market.MarketCapRank != 0 && market.MarketCapRank < existing.MarketCapRank) {
		copy := *market
		markets[key] = &copy
	}
}

func setBestMarketMatches(markets map[string]*AppTokenMarket, seen map[string]struct{}, matches []AssetDetail, market *AppTokenMarket) int {
	count := 0
	for _, asset := range matches {
		key := assetLookupKey(asset)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		setBestMarket(markets, key, market)
		count++
	}
	return count
}

func assetLookupKey(asset AssetDetail) string {
	return chainAddressKey(asset.Chain, asset.Address)
}

func reportAssetRef(kind string, asset AssetDetail) ReportAssetRef {
	return ReportAssetRef{
		Kind:    kind,
		Chain:   asset.Chain,
		Address: asset.Address,
		AssetID: asset.AssetID,
		Symbol:  asset.Symbol,
		Name:    asset.Name,
		Status:  asset.Status,
	}
}

func (s *Syncer) fetchCoinGeckoMarkets(ctx context.Context, page, perPage int) ([]coinGeckoMarket, error) {
	u, err := url.Parse(strings.TrimRight(s.config.CoinGeckoBaseURL, "/") + "/coins/markets")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("vs_currency", s.config.VsCurrency)
	q.Set("order", "market_cap_desc")
	q.Set("per_page", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))
	q.Set("sparkline", "false")
	u.RawQuery = q.Encode()

	var result []coinGeckoMarket
	if err := s.getJSON(ctx, u.String(), true, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Syncer) fetchCoinGeckoDataset(ctx context.Context) (*coinGeckoDataset, error) {
	var markets []coinGeckoMarket
	limit := s.marketLimit()
	perPage := marketPerPageForLimit(limit)
	pages := marketPageCount(limit, perPage)
	for page := 1; page <= pages; page++ {
		pageMarkets, err := s.fetchCoinGeckoMarkets(ctx, page, perPage)
		if err != nil {
			return nil, err
		}
		if len(pageMarkets) == 0 {
			break
		}
		markets = append(markets, pageMarkets...)
	}
	if len(markets) > limit {
		markets = markets[:limit]
	}

	coins, err := s.fetchCoinGeckoCoinList(ctx)
	if err != nil {
		return nil, err
	}

	return &coinGeckoDataset{
		Markets:       markets,
		Coins:         coins,
		CoinByID:      coinListByID(coins),
		PlatformsByID: platformsByCoinID(coins),
	}, nil
}

func (s *Syncer) marketLimit() int {
	return s.config.MarketLimit
}

func marketPerPageForLimit(limit int) int {
	if limit > 0 && limit < defaultMarketPerPage {
		return limit
	}
	return defaultMarketPerPage
}

func marketPageCount(limit, perPage int) int {
	if limit <= 0 {
		return 0
	}
	if perPage <= 0 {
		perPage = defaultMarketPerPage
	}
	return (limit + perPage - 1) / perPage
}

func (s *Syncer) fetchCoinGeckoPlatformsByID(ctx context.Context, markets []coinGeckoMarket) (map[string]map[string]string, error) {
	marketIDs := make(map[string]struct{}, len(markets))
	for _, market := range markets {
		if market.ID != "" {
			marketIDs[market.ID] = struct{}{}
		}
	}
	if len(marketIDs) == 0 {
		return map[string]map[string]string{}, nil
	}

	coins, err := s.fetchCoinGeckoCoinList(ctx)
	if err != nil {
		return nil, err
	}

	platformsByID := make(map[string]map[string]string, len(marketIDs))
	for _, coin := range coins {
		if _, ok := marketIDs[coin.ID]; !ok || len(coin.Platforms) == 0 {
			continue
		}
		platformsByID[coin.ID] = coin.Platforms
	}

	return platformsByID, nil
}

func coinListByID(coins []coinGeckoListItem) map[string]coinGeckoListItem {
	result := make(map[string]coinGeckoListItem, len(coins))
	for _, coin := range coins {
		if coin.ID != "" {
			result[coin.ID] = coin
		}
	}
	return result
}

func platformsByCoinID(coins []coinGeckoListItem) map[string]map[string]string {
	result := make(map[string]map[string]string, len(coins))
	for _, coin := range coins {
		if coin.ID != "" && len(coin.Platforms) > 0 {
			result[coin.ID] = coin.Platforms
		}
	}
	return result
}

func (s *Syncer) fetchCoinGeckoCoinList(ctx context.Context) ([]coinGeckoListItem, error) {
	u, err := url.Parse(strings.TrimRight(s.config.CoinGeckoBaseURL, "/") + "/coins/list")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("include_platform", "true")
	u.RawQuery = q.Encode()

	var result []coinGeckoListItem
	if err := s.getJSON(ctx, u.String(), true, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Syncer) fetchDefiLlamaStablecoins(ctx context.Context) (*defiLlamaStablecoinsResponse, error) {
	endpoint := strings.TrimRight(s.config.DefiLlamaBaseURL, "/") + "/stablecoins?includePrices=true"
	var result defiLlamaStablecoinsResponse
	if err := s.getJSON(ctx, endpoint, false, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Syncer) getJSON(ctx context.Context, endpoint string, coinGecko bool, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	if coinGecko && s.config.CoinGeckoAPIKey != "" {
		req.Header.Set(s.config.CoinGeckoKeyHeader, s.config.CoinGeckoAPIKey)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d from %s", res.StatusCode, endpoint)
	}

	return json.NewDecoder(res.Body).Decode(target)
}

func CoinGeckoKeyFromEnv() string {
	return strings.TrimSpace(os.Getenv("COINGECKO_API_KEY"))
}

func CoinGeckoBaseURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("COINGECKO_API_BASE_URL"))
}

func CoinGeckoKeyHeaderFromEnv() string {
	return strings.TrimSpace(os.Getenv("COINGECKO_API_KEY_HEADER"))
}

func DefiLlamaBaseURLFromEnv() string {
	return strings.TrimSpace(os.Getenv("DEFILLAMA_STABLECOIN_BASE_URL"))
}

type coinGeckoMarket struct {
	ID            string  `json:"id"`
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	CurrentPrice  float64 `json:"current_price"`
	MarketCap     float64 `json:"market_cap"`
	MarketCapRank int     `json:"market_cap_rank"`
	TotalVolume   float64 `json:"total_volume"`
	LastUpdated   string  `json:"last_updated"`
}

type coinGeckoListItem struct {
	ID        string            `json:"id"`
	Symbol    string            `json:"symbol"`
	Name      string            `json:"name"`
	Platforms map[string]string `json:"platforms"`
}

type defiLlamaStablecoinsResponse struct {
	PeggedAssets []defiLlamaStablecoin `json:"peggedAssets"`
}

type defiLlamaStablecoin struct {
	ID               any                        `json:"id"`
	GeckoID          string                     `json:"gecko_id"`
	GeckoIDCamel     string                     `json:"geckoId"`
	Name             string                     `json:"name"`
	Symbol           string                     `json:"symbol"`
	PegType          string                     `json:"pegType"`
	PriceSource      string                     `json:"priceSource"`
	Circulating      map[string]float64         `json:"circulating"`
	ChainCirculating map[string]json.RawMessage `json:"chainCirculating"`
}

func (d defiLlamaStablecoin) CoinGeckoID() string {
	if strings.TrimSpace(d.GeckoID) != "" {
		return strings.TrimSpace(d.GeckoID)
	}
	return strings.TrimSpace(d.GeckoIDCamel)
}

func (d defiLlamaStablecoin) IDString() string {
	switch value := d.ID.(type) {
	case string:
		return value
	case float64:
		return strconv.FormatInt(int64(value), 10)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func (d defiLlamaStablecoin) CirculatingUSD() float64 {
	if value, ok := d.Circulating["peggedUSD"]; ok {
		return value
	}
	var total float64
	for _, value := range d.Circulating {
		total += value
	}
	return total
}

func (d defiLlamaStablecoin) Chains() []string {
	chains := make([]string, 0, len(d.ChainCirculating))
	for chain := range d.ChainCirculating {
		chains = append(chains, chain)
	}
	sort.Strings(chains)
	return chains
}
