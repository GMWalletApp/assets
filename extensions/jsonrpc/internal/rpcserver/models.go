package rpcserver

import "encoding/json"

const (
	ErrCodeParseError     = -32700
	ErrCodeInvalidRequest = -32600
	ErrCodeMethodNotFound = -32601
	ErrCodeInvalidParams  = -32602
	ErrCodeInternal       = -32603
	ErrCodeNotFound       = -32004
)

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return e.Message
}

func invalidParams(message string) *RPCError {
	return &RPCError{Code: ErrCodeInvalidParams, Message: message}
}

func notFound(message string) *RPCError {
	return &RPCError{Code: ErrCodeNotFound, Message: message}
}

func internalError(message string) *RPCError {
	return &RPCError{Code: ErrCodeInternal, Message: message}
}

type Link struct {
	Name string `json:"name,omitempty"`
	URL  string `json:"url,omitempty"`
}

type AssetDetail struct {
	Chain        string   `json:"chain"`
	Address      string   `json:"address"`
	AssetID      string   `json:"assetId"`
	Name         string   `json:"name,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	Type         string   `json:"type,omitempty"`
	Decimals     int      `json:"decimals"`
	Status       string   `json:"status,omitempty"`
	Website      string   `json:"website,omitempty"`
	Description  string   `json:"description,omitempty"`
	Explorer     string   `json:"explorer,omitempty"`
	Research     string   `json:"research,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Links        []Link   `json:"links,omitempty"`
	LogoURI      string   `json:"logoURI,omitempty"`
	LogoExists   bool     `json:"logoExists"`
	ShortDesc    string   `json:"short_desc,omitempty"`
	Audit        string   `json:"audit,omitempty"`
	AuditReport  string   `json:"audit_report,omitempty"`
	Code         string   `json:"code,omitempty"`
	Ticker       string   `json:"ticker,omitempty"`
	ExplorerEth  string   `json:"explorer-ETH,omitempty"`
	ExternalAddr string   `json:"addressField,omitempty"`
}

type TokenPair struct {
	Base string `json:"base,omitempty"`
}

type MarketCache struct {
	Source     string        `json:"source"`
	VsCurrency string        `json:"vsCurrency,omitempty"`
	UpdatedAt  string        `json:"updatedAt,omitempty"`
	Assets     []MarketAsset `json:"assets"`
}

type MarketAsset struct {
	Rank          int           `json:"rank,omitempty"`
	Source        string        `json:"source,omitempty"`
	CoinGeckoID   string        `json:"coingeckoId,omitempty"`
	Symbol        string        `json:"symbol,omitempty"`
	Name          string        `json:"name,omitempty"`
	MarketCapRank int           `json:"marketCapRank,omitempty"`
	MarketCap     float64       `json:"marketCap,omitempty"`
	TotalVolume   float64       `json:"totalVolume,omitempty"`
	CurrentPrice  float64       `json:"currentPrice,omitempty"`
	LastUpdated   string        `json:"lastUpdated,omitempty"`
	UpdatedAt     string        `json:"updatedAt,omitempty"`
	Assets        []AssetDetail `json:"assets"`
}

type StablecoinCache struct {
	Source    string            `json:"source"`
	UpdatedAt string            `json:"updatedAt,omitempty"`
	Assets    []StablecoinAsset `json:"assets"`
}

type StablecoinAsset struct {
	Rank        int           `json:"rank,omitempty"`
	Source      string        `json:"source,omitempty"`
	DefiLlamaID string        `json:"defillamaId,omitempty"`
	CoinGeckoID string        `json:"coingeckoId,omitempty"`
	Symbol      string        `json:"symbol,omitempty"`
	Name        string        `json:"name,omitempty"`
	Circulating float64       `json:"circulating,omitempty"`
	Chains      []string      `json:"chains,omitempty"`
	PegType     string        `json:"pegType,omitempty"`
	PriceSource string        `json:"priceSource,omitempty"`
	UpdatedAt   string        `json:"updatedAt,omitempty"`
	Assets      []AssetDetail `json:"assets"`
}

type AppTokenList struct {
	Source    string     `json:"source"`
	UpdatedAt string     `json:"updatedAt,omitempty"`
	Tokens    []AppToken `json:"tokens"`
}

type AppToken struct {
	Kind       string          `json:"kind"`
	Chain      string          `json:"chain"`
	Hot        bool            `json:"hot"`
	Address    string          `json:"address"`
	AssetID    string          `json:"assetId"`
	Type       string          `json:"type,omitempty"`
	Name       string          `json:"name,omitempty"`
	Symbol     string          `json:"symbol,omitempty"`
	Decimals   int             `json:"decimals"`
	Status     string          `json:"status,omitempty"`
	LogoURI    string          `json:"logoURI,omitempty"`
	LogoExists bool            `json:"logoExists"`
	Rank       int             `json:"rank,omitempty"`
	Market     *AppTokenMarket `json:"market,omitempty"`
	Pairs      []TokenPair     `json:"pairs,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Links      []Link          `json:"links,omitempty"`
}

type AppTokenMarket struct {
	Source        string  `json:"source,omitempty"`
	CoinGeckoID   string  `json:"coingeckoId,omitempty"`
	MarketCapRank int     `json:"marketCapRank,omitempty"`
	MarketCap     float64 `json:"marketCap,omitempty"`
	TotalVolume   float64 `json:"totalVolume,omitempty"`
	CurrentPrice  float64 `json:"currentPrice,omitempty"`
	LastUpdated   string  `json:"lastUpdated,omitempty"`
}

type TokenListReport struct {
	Source     string                `json:"source"`
	UpdatedAt  string                `json:"updatedAt,omitempty"`
	APIs       []ReportAPIRequest    `json:"apis"`
	Local      ReportLocalStats      `json:"local"`
	Market     ReportMarketStats     `json:"market"`
	Stablecoin ReportStablecoinStats `json:"stablecoin"`
	Hot        ReportHotStats        `json:"hot"`
	Rules      ReportRuleStats       `json:"rules"`
	Issues     ReportIssues          `json:"issues"`
}

type ReportAPIRequest struct {
	Source string            `json:"source"`
	URL    string            `json:"url"`
	Params map[string]string `json:"params,omitempty"`
}

type ReportLocalStats struct {
	NativeAssets int `json:"nativeAssets"`
	TokenAssets  int `json:"tokenAssets"`
	OutputTokens int `json:"outputTokens"`
	Filtered     int `json:"filtered"`
	MissingLogos int `json:"missingLogos"`
}

type ReportMarketStats struct {
	Rows             int `json:"rows"`
	NativeMatches    int `json:"nativeMatches"`
	TokenMatches     int `json:"tokenMatches"`
	RankedAssets     int `json:"rankedAssets"`
	UnmatchedRows    int `json:"unmatchedRows"`
	UnmappedPlatform int `json:"unmappedPlatform"`
	MissingAssets    int `json:"missingAssets"`
}

type ReportStablecoinStats struct {
	TaggedAssets int `json:"taggedAssets"`
}

type ReportHotStats struct {
	DefaultEntries int `json:"defaultEntries"`
	CurrentEntries int `json:"currentEntries"`
	EnabledAssets  int `json:"enabledAssets"`
}

type ReportIssues struct {
	FilteredAssets        []ReportAssetRef    `json:"filteredAssets,omitempty"`
	MissingLogos          []ReportAssetRef    `json:"missingLogos,omitempty"`
	MissingHotAssets      []ReportAssetRef    `json:"missingHotAssets,omitempty"`
	UnmappedPlatforms     []ReportPlatformRef `json:"unmappedPlatforms,omitempty"`
	MissingPlatformAssets []ReportPlatformRef `json:"missingPlatformAssets,omitempty"`
	UnmatchedMarketRows   []ReportMarketRef   `json:"unmatchedMarketRows,omitempty"`
	RuleIssues            []ReportRuleIssue   `json:"ruleIssues,omitempty"`
}

type ReportRuleStats struct {
	ConfiguredPlatformMappings     int `json:"configuredPlatformMappings"`
	ConfiguredNativeMarketMappings int `json:"configuredNativeMarketMappings"`
	ConfiguredAssetOverrides       int `json:"configuredAssetOverrides"`
	BaseAssetOverrides             int `json:"baseAssetOverrides"`
	ManualAssetOverrides           int `json:"manualAssetOverrides"`
	ConfiguredMarketTagRules       int `json:"configuredMarketTagRules"`
	ConfiguredExcludedChains       int `json:"configuredExcludedChains"`
	PlatformMappingHits            int `json:"platformMappingHits"`
	NativeMarketMappingHits        int `json:"nativeMarketMappingHits"`
	AssetOverrideMarketHits        int `json:"assetOverrideMarketHits"`
	AssetOverrideHits              int `json:"assetOverrideHits"`
	MarketTagRuleHits              int `json:"marketTagRuleHits"`
	ExcludedChainHits              int `json:"excludedChainHits"`
}

type ReportAssetRef struct {
	Kind    string `json:"kind,omitempty"`
	Chain   string `json:"chain,omitempty"`
	Address string `json:"address,omitempty"`
	AssetID string `json:"assetId,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
}

type ReportPlatformRef struct {
	CoinGeckoID string `json:"coingeckoId,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Name        string `json:"name,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Chain       string `json:"chain,omitempty"`
	Address     string `json:"address,omitempty"`
}

type ReportMarketRef struct {
	CoinGeckoID string `json:"coingeckoId,omitempty"`
	Symbol      string `json:"symbol,omitempty"`
	Name        string `json:"name,omitempty"`
	Rank        int    `json:"rank,omitempty"`
}

type ReportRuleIssue struct {
	Rule        string `json:"rule,omitempty"`
	Reason      string `json:"reason,omitempty"`
	Chain       string `json:"chain,omitempty"`
	Address     string `json:"address,omitempty"`
	CoinGeckoID string `json:"coingeckoId,omitempty"`
}

type TokenListRules struct {
	PlatformMappings     map[string]string        `json:"platformMappings,omitempty"`
	NativeMarketMappings map[string][]string      `json:"nativeMarketMappings,omitempty"`
	MarketTagRules       []TokenListMarketTagRule `json:"marketTagRules,omitempty"`
	ExcludedStatuses     []string                 `json:"excludedStatuses"`
	ExcludedChains       []string                 `json:"excludedChains,omitempty"`
}

type TokenListAssetOverridesFile struct {
	AssetOverrides []TokenListAssetOverride `json:"assetOverrides"`
}

type TokenListManualOverrides = TokenListAssetOverridesFile

type TokenListAssetOverride struct {
	Chain         string   `json:"chain,omitempty"`
	Address       string   `json:"address,omitempty"`
	CoinGeckoID   string   `json:"coingeckoId,omitempty"`
	DisplayName   string   `json:"displayName,omitempty"`
	DisplaySymbol string   `json:"displaySymbol,omitempty"`
	AddTags       []string `json:"addTags,omitempty"`
	Note          string   `json:"note,omitempty"`
}

type TokenListMarketTagRule struct {
	CoinGeckoID string   `json:"coingeckoId,omitempty"`
	AddTags     []string `json:"addTags,omitempty"`
}

type TokenListHotList struct {
	Tokens []TokenListHotEntry `json:"tokens"`
}

type TokenListHotEntry struct {
	Chain   string `json:"chain,omitempty"`
	Address string `json:"address,omitempty"`
}

type TokenListManualTokensFile struct {
	Tokens []AppToken `json:"tokens"`
}

type TokenListManualTokens = TokenListManualTokensFile

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
	ID      json.RawMessage `json:"id"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id"`
}

type assetInfoFile struct {
	Name          string   `json:"name"`
	Symbol        string   `json:"symbol"`
	Type          string   `json:"type"`
	Decimals      int      `json:"decimals"`
	Description   string   `json:"description"`
	Website       string   `json:"website"`
	Explorer      string   `json:"explorer"`
	Research      string   `json:"research"`
	Status        string   `json:"status"`
	ID            string   `json:"id"`
	Links         []Link   `json:"links"`
	ShortDesc     string   `json:"short_desc"`
	Audit         string   `json:"audit"`
	AuditReport   string   `json:"audit_report"`
	Tags          []string `json:"tags"`
	Code          string   `json:"code"`
	Ticker        string   `json:"ticker"`
	ExplorerEth   string   `json:"explorer-ETH"`
	Address       string   `json:"address"`
	CoinMarketcap string   `json:"coinmarketcap"`
	DataSource    string   `json:"data_source"`
}
