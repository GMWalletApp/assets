package rpcserver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/trustwallet/go-primitives/asset"
	"github.com/trustwallet/go-primitives/coin"
)

type Store struct {
	root         string
	assetBaseURL string
}

type AssetIndex struct {
	bySymbol          map[string][]AssetDetail
	byStableSymbol    map[string][]AssetDetail
	byNormalizedName  map[string][]AssetDetail
	byStableName      map[string][]AssetDetail
	byCoinGeckoID     map[string][]AssetDetail
	byCoinMarketCapID map[string][]AssetDetail
	byChainAndAddress map[string]AssetDetail
	nativeAssets      []AssetDetail
	tokenAssets       []AssetDetail
}

func NewStore(root, assetBaseURL string) *Store {
	return &Store{
		root:         root,
		assetBaseURL: strings.TrimRight(assetBaseURL, "/"),
	}
}

func (s *Store) ListChains() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.root, "blockchains"))
	if err != nil {
		return nil, err
	}

	var chains []string
	for _, entry := range entries {
		if entry.IsDir() {
			chains = append(chains, entry.Name())
		}
	}
	sort.Strings(chains)

	return chains, nil
}

func (s *Store) GetChainInfo(chain string) (map[string]any, error) {
	if err := validatePathPart("chain", chain); err != nil {
		return nil, err
	}

	infoPath := filepath.Join(s.root, "blockchains", chain, "info", "info.json")
	var info map[string]any
	if err := readJSONFile(infoPath, &info); err != nil {
		if os.IsNotExist(err) {
			return nil, notFound("chain not found")
		}
		return nil, err
	}

	info["chain"] = chain
	logoPath := filepath.Join(s.root, "blockchains", chain, "info", "logo.png")
	info["logoExists"] = fileExists(logoPath)
	info["logoURI"] = fmt.Sprintf("%s/blockchains/%s/info/logo.png", s.assetBaseURL, chain)

	return info, nil
}

func (s *Store) GetAssetByID(assetID string) (*AssetDetail, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, invalidParams("assetId is required")
	}
	if nativeID, ok := parseNativeAssetID(assetID); ok {
		chain, ok := coin.Coins[nativeID]
		if !ok {
			return nil, notFound("chain not found")
		}
		return s.readNativeAssetDetail(chain.Handle, filepath.Join(s.root, "blockchains", chain.Handle, "info"))
	}

	c, tokenID, err := asset.ParseID(assetID)
	if err != nil {
		return nil, invalidParams(fmt.Sprintf("invalid assetId: %v", err))
	}

	chain, ok := coin.Coins[c]
	if !ok {
		return nil, notFound("chain not found")
	}

	return s.GetAssetByAddress(chain.Handle, tokenID)
}

func (s *Store) GetAssetByAddress(chain, address string) (*AssetDetail, error) {
	if err := validatePathPart("chain", chain); err != nil {
		return nil, err
	}
	if err := validatePathPart("address", address); err != nil {
		return nil, err
	}

	assetDir, resolvedAddress, err := s.resolveAssetDir(chain, address)
	if err != nil {
		return nil, err
	}

	return s.readAssetDetail(chain, resolvedAddress, assetDir)
}

func (s *Store) GetTokenList(chain string, extended bool) (map[string]any, error) {
	if err := validatePathPart("chain", chain); err != nil {
		return nil, err
	}

	name := "tokenlist.json"
	if extended {
		name = "tokenlist-extended.json"
	}

	tokenListPath := filepath.Join(s.root, "blockchains", chain, name)
	var data map[string]any
	if err := readJSONFile(tokenListPath, &data); err != nil {
		if os.IsNotExist(err) {
			return nil, notFound("tokenlist not found")
		}
		return nil, err
	}

	return data, nil
}

func (s *Store) TokenListPairsByAssetID() (map[string][]TokenPair, error) {
	result := map[string][]TokenPair{}
	paths, err := filepath.Glob(filepath.Join(s.root, "blockchains", "*", "tokenlist*.json"))
	if err != nil {
		return nil, err
	}

	for _, path := range paths {
		var tokenList struct {
			Tokens []struct {
				Asset string      `json:"asset"`
				Pairs []TokenPair `json:"pairs"`
			} `json:"tokens"`
		}
		if err := readJSONFile(path, &tokenList); err != nil {
			return nil, err
		}
		for _, token := range tokenList.Tokens {
			if token.Asset == "" || len(token.Pairs) == 0 {
				continue
			}
			result[token.Asset] = appendUniquePairs(result[token.Asset], token.Pairs)
		}
	}

	return result, nil
}

func (s *Store) ListStablecoinsFromLocal(chain string, limit, offset int, onlyWithAssets bool) ([]StablecoinAsset, error) {
	index, err := s.BuildAssetIndex()
	if err != nil {
		return nil, err
	}

	grouped := map[string]StablecoinAsset{}
	for _, assets := range index.byStableSymbol {
		for _, asset := range assets {
			if chain != "" && !strings.EqualFold(chain, asset.Chain) {
				continue
			}
			key := strings.ToUpper(asset.Symbol)
			item := grouped[key]
			item.Source = "local"
			item.Symbol = asset.Symbol
			item.Name = asset.Name
			item.Assets = appendUniqueAsset(item.Assets, asset)
			grouped[key] = item
		}
	}

	var items []StablecoinAsset
	for _, item := range grouped {
		if onlyWithAssets && len(item.Assets) == 0 {
			continue
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return strings.ToUpper(items[i].Symbol) < strings.ToUpper(items[j].Symbol)
	})
	for i := range items {
		items[i].Rank = i + 1
	}

	return paginateStablecoins(items, limit, offset), nil
}

func (s *Store) BuildAssetIndex() (*AssetIndex, error) {
	index := &AssetIndex{
		bySymbol:          map[string][]AssetDetail{},
		byStableSymbol:    map[string][]AssetDetail{},
		byNormalizedName:  map[string][]AssetDetail{},
		byStableName:      map[string][]AssetDetail{},
		byCoinGeckoID:     map[string][]AssetDetail{},
		byCoinMarketCapID: map[string][]AssetDetail{},
		byChainAndAddress: map[string]AssetDetail{},
		nativeAssets:      []AssetDetail{},
		tokenAssets:       []AssetDetail{},
	}

	root := filepath.Join(s.root, "blockchains")
	chains, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, chainEntry := range chains {
		if !chainEntry.IsDir() {
			continue
		}

		chain := chainEntry.Name()
		native, err := s.readNativeAssetDetail(chain, filepath.Join(root, chain, "info"))
		if err == nil {
			index.addNative(*native)
		}

		assetsDir := filepath.Join(root, chain, "assets")
		assets, err := os.ReadDir(assetsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, assetEntry := range assets {
			if !assetEntry.IsDir() {
				continue
			}

			address := assetEntry.Name()
			detail, err := s.readAssetDetail(chain, address, filepath.Join(assetsDir, address))
			if err != nil {
				continue
			}

			index.add(*detail)
		}
	}

	return index, nil
}

func (idx *AssetIndex) NativeAssets() []AssetDetail {
	return append([]AssetDetail(nil), idx.nativeAssets...)
}

func (idx *AssetIndex) TokenAssets() []AssetDetail {
	return append([]AssetDetail(nil), idx.tokenAssets...)
}

func (idx *AssetIndex) MatchMarketPlatforms(platforms map[string]string) []AssetDetail {
	return idx.MatchMarketPlatformsWithRules(platforms, nil)
}

func (idx *AssetIndex) MatchMarketPlatformsWithRules(platforms map[string]string, config *ResolvedTokenListConfig) []AssetDetail {
	var matches []AssetDetail
	for platform, address := range platforms {
		chain, _, ok := coinGeckoPlatformChainWithRules(platform, config)
		if !ok {
			continue
		}
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		if asset, ok := idx.byChainAndAddress[chainAddressKey(chain, address)]; ok {
			matches = appendUniqueAsset(matches, asset)
		}
	}

	sortAssetDetails(matches)

	return matches
}

func (idx *AssetIndex) MatchExternalMarket(coingeckoID string) []AssetDetail {
	id := normalizeExternalID(coingeckoID)
	if id == "" {
		return nil
	}

	matches := appendUniqueAssets(nil, idx.byCoinGeckoID[id])
	matches = appendUniqueAssets(matches, idx.byCoinMarketCapID[id])
	sortAssetDetails(matches)
	return matches
}

func (idx *AssetIndex) MatchNativeMarket(coingeckoID string) []AssetDetail {
	return idx.MatchNativeMarketWithRules(coingeckoID, nil)
}

func (idx *AssetIndex) MatchNativeMarketWithRules(coingeckoID string, config *ResolvedTokenListConfig) []AssetDetail {
	chains, _ := coinGeckoNativeChainsWithRules(coingeckoID, config)
	matches := make([]AssetDetail, 0, len(chains))
	for _, chain := range chains {
		if asset, ok := idx.byChainAndAddress[chainAddressKey(chain, "")]; ok {
			matches = append(matches, asset)
		}
	}
	return matches
}

func (idx *AssetIndex) addNative(asset AssetDetail) {
	idx.nativeAssets = append(idx.nativeAssets, asset)
	idx.byChainAndAddress[chainAddressKey(asset.Chain, "")] = asset
	idx.indexExternalLinks(asset)
}

func (idx *AssetIndex) add(asset AssetDetail) {
	symbolKey := strings.ToUpper(strings.TrimSpace(asset.Symbol))
	nameKey := normalizeName(asset.Name)
	key := chainAddressKey(asset.Chain, asset.Address)

	idx.byChainAndAddress[key] = asset
	idx.tokenAssets = append(idx.tokenAssets, asset)
	idx.indexExternalLinks(asset)
	if symbolKey != "" {
		idx.bySymbol[symbolKey] = append(idx.bySymbol[symbolKey], asset)
	}
	if nameKey != "" {
		idx.byNormalizedName[nameKey] = append(idx.byNormalizedName[nameKey], asset)
	}
	if hasTag(asset.Tags, "stablecoin") {
		if symbolKey != "" {
			idx.byStableSymbol[symbolKey] = append(idx.byStableSymbol[symbolKey], asset)
		}
		if nameKey != "" {
			idx.byStableName[nameKey] = append(idx.byStableName[nameKey], asset)
		}
	}
}

func (idx *AssetIndex) indexExternalLinks(asset AssetDetail) {
	for _, link := range asset.Links {
		if id := coinGeckoIDFromURL(link.URL); id != "" {
			idx.byCoinGeckoID[id] = appendUniqueAsset(idx.byCoinGeckoID[id], asset)
		}
		if id := coinMarketCapIDFromURL(link.URL); id != "" {
			idx.byCoinMarketCapID[id] = appendUniqueAsset(idx.byCoinMarketCapID[id], asset)
		}
	}
}

func (s *Store) resolveAssetDir(chain, address string) (string, string, error) {
	assetDir := filepath.Join(s.root, "blockchains", chain, "assets", address)
	if dirExists(assetDir) {
		return assetDir, address, nil
	}

	assetsDir := filepath.Join(s.root, "blockchains", chain, "assets")
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", notFound("asset not found")
		}
		return "", "", err
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.EqualFold(entry.Name(), address) {
			resolved := entry.Name()
			return filepath.Join(assetsDir, resolved), resolved, nil
		}
	}

	return "", "", notFound("asset not found")
}

func (s *Store) readNativeAssetDetail(chain, infoDir string) (*AssetDetail, error) {
	var info assetInfoFile
	if err := readJSONFile(filepath.Join(infoDir, "info.json"), &info); err != nil {
		if os.IsNotExist(err) {
			return nil, notFound("chain not found")
		}
		return nil, err
	}

	detail := &AssetDetail{
		Chain:       chain,
		Address:     "",
		AssetID:     makeNativeAssetID(chain),
		Name:        info.Name,
		Symbol:      info.Symbol,
		Type:        info.Type,
		Decimals:    info.Decimals,
		Status:      info.Status,
		Website:     info.Website,
		Description: info.Description,
		Explorer:    info.Explorer,
		Research:    info.Research,
		Tags:        info.Tags,
		Links:       info.Links,
		LogoURI:     fmt.Sprintf("%s/blockchains/%s/info/logo.png", s.assetBaseURL, chain),
		LogoExists:  fileExists(filepath.Join(infoDir, "logo.png")),
		ShortDesc:   info.ShortDesc,
		Audit:       info.Audit,
		AuditReport: info.AuditReport,
		Code:        info.Code,
		Ticker:      info.Ticker,
		ExplorerEth: info.ExplorerEth,
	}

	return detail, nil
}

func (s *Store) readAssetDetail(chain, address, assetDir string) (*AssetDetail, error) {
	var info assetInfoFile
	if err := readJSONFile(filepath.Join(assetDir, "info.json"), &info); err != nil {
		if os.IsNotExist(err) {
			return nil, notFound("asset not found")
		}
		return nil, err
	}

	if info.ID != "" {
		address = info.ID
	}

	detail := &AssetDetail{
		Chain:        chain,
		Address:      address,
		AssetID:      makeAssetID(chain, address),
		Name:         info.Name,
		Symbol:       info.Symbol,
		Type:         info.Type,
		Decimals:     info.Decimals,
		Status:       info.Status,
		Website:      info.Website,
		Description:  info.Description,
		Explorer:     info.Explorer,
		Research:     info.Research,
		Tags:         info.Tags,
		Links:        info.Links,
		LogoURI:      fmt.Sprintf("%s/blockchains/%s/assets/%s/logo.png", s.assetBaseURL, chain, address),
		LogoExists:   fileExists(filepath.Join(assetDir, "logo.png")),
		ShortDesc:    info.ShortDesc,
		Audit:        info.Audit,
		AuditReport:  info.AuditReport,
		Code:         info.Code,
		Ticker:       info.Ticker,
		ExplorerEth:  info.ExplorerEth,
		ExternalAddr: info.Address,
	}

	return detail, nil
}

func makeNativeAssetID(chainHandle string) string {
	for _, c := range coin.Coins {
		if c.Handle == chainHandle {
			return fmt.Sprintf("c%d", c.ID)
		}
	}

	return chainHandle
}

func parseNativeAssetID(assetID string) (uint, bool) {
	assetID = strings.TrimSpace(assetID)
	if len(assetID) < 2 || assetID[0] != 'c' || strings.Contains(assetID, "_") {
		return 0, false
	}
	id, err := strconv.ParseUint(assetID[1:], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

func makeAssetID(chainHandle, address string) string {
	for _, c := range coin.Coins {
		if c.Handle == chainHandle {
			return fmt.Sprintf("c%d_t%s", c.ID, address)
		}
	}

	return fmt.Sprintf("%s:%s", chainHandle, address)
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, target)
}

func validatePathPart(name, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return invalidParams(name + " is required")
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "..") {
		return invalidParams(name + " contains invalid path characters")
	}
	return nil
}

func normalizeName(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func coinGeckoIDFromURL(rawURL string) string {
	return externalIDFromURL(rawURL, "coingecko.com", "coins")
}

func coinMarketCapIDFromURL(rawURL string) string {
	return externalIDFromURL(rawURL, "coinmarketcap.com", "currencies")
}

func externalIDFromURL(rawURL, wantHost, marker string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host != wantHost {
		return ""
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if strings.EqualFold(part, marker) && i+1 < len(parts) {
			return normalizeExternalID(parts[i+1])
		}
	}
	return ""
}

func normalizeExternalID(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), "/"))
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func appendUniqueAssets(dst []AssetDetail, src []AssetDetail) []AssetDetail {
	for _, item := range src {
		dst = appendUniqueAsset(dst, item)
	}
	return dst
}

func appendUniqueAsset(dst []AssetDetail, item AssetDetail) []AssetDetail {
	key := chainAddressKey(item.Chain, item.Address)
	for _, existing := range dst {
		if chainAddressKey(existing.Chain, existing.Address) == key {
			return dst
		}
	}
	return append(dst, item)
}

func appendUniquePairs(dst []TokenPair, src []TokenPair) []TokenPair {
	for _, item := range src {
		if item.Base == "" {
			continue
		}
		exists := false
		for _, existing := range dst {
			if existing.Base == item.Base {
				exists = true
				break
			}
		}
		if !exists {
			dst = append(dst, item)
		}
	}
	return dst
}

func sortAssetDetails(items []AssetDetail) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Chain == items[j].Chain {
			return strings.ToLower(items[i].Address) < strings.ToLower(items[j].Address)
		}
		return items[i].Chain < items[j].Chain
	})
}

func chainAddressKey(chain, address string) string {
	return strings.ToLower(chain) + "\x00" + strings.ToLower(address)
}

func coinGeckoPlatformChain(platform string) (string, bool) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return "", false
	}
	if chain, ok := coinGeckoPlatformChains[platform]; ok {
		return chain, true
	}
	return platform, coinGeckoPlatformSameAsChain[platform]
}

var coinGeckoPlatformSameAsChain = map[string]bool{
	"algorand":  true,
	"aptos":     true,
	"arbitrum":  true,
	"aurora":    true,
	"base":      true,
	"blast":     true,
	"boba":      true,
	"cardano":   true,
	"celo":      true,
	"cronos":    true,
	"ethereum":  true,
	"fantom":    true,
	"harmony":   true,
	"kava":      true,
	"kcc":       true,
	"linea":     true,
	"mantle":    true,
	"metis":     true,
	"moonbeam":  true,
	"moonriver": true,
	"near":      true,
	"opbnb":     true,
	"osmosis":   true,
	"polygon":   true,
	"ronin":     true,
	"scroll":    true,
	"solana":    true,
	"sonic":     true,
	"stellar":   true,
	"sui":       true,
	"tezos":     true,
	"ton":       true,
	"tron":      true,
	"vechain":   true,
	"waves":     true,
	"wemix":     true,
	"xdai":      true,
	"zksync":    true,
	"zilliqa":   true,
}

var coinGeckoPlatformChains = map[string]string{
	"arbitrum-one":        "arbitrum",
	"avalanche":           "avalanchec",
	"avalanche-c-chain":   "avalanchec",
	"binance-smart-chain": "smartchain",
	"bnb-smart-chain":     "smartchain",
	"conflux":             "cfxevm",
	"conflux-espace":      "cfxevm",
	"crypto-com-chain":    "cryptoorg",
	"energi":              "energyweb",
	"evmos":               "nativeevmos",
	"gnosis":              "xdai",
	"gnosis-chain":        "xdai",
	"hedera-hashgraph":    "hedera",
	"injective":           "nativeinjective",
	"iotex":               "iotexevm",
	"kava-evm":            "kavaevm",
	"klay-token":          "klaytn",
	"klaytn":              "klaytn",
	"kujira":              "kujira",
	"manta-pacific":       "manta",
	"merlin-chain":        "merlin",
	"meter":               "meter",
	"neon-evm":            "neon",
	"okex-chain":          "okc",
	"optimistic-ethereum": "optimism",
	"polygon-pos":         "polygon",
	"polygon-zkevm":       "polygonzkevm",
	"rootstock":           "rootstock",
	"sei-network":         "seievm",
	"terra":               "terra",
	"terra-2":             "terrav2",
	"the-open-network":    "ton",
	"zeta-chain":          "zetaevm",
}

var coinGeckoNativeChains = map[string][]string{
	"aeternity":         {"aeternity"},
	"akash-network":     {"akash"},
	"algorand":          {"algorand"},
	"aptos":             {"aptos"},
	"arbitrum":          {"arbitrum"},
	"avalanche-2":       {"avalanchec"},
	"band-protocol":     {"band"},
	"binancecoin":       {"smartchain", "binance"},
	"bitcoin":           {"bitcoin"},
	"bitcoin-cash":      {"bitcoincash"},
	"bitcoin-gold":      {"bitcoingold"},
	"cardano":           {"cardano"},
	"celo":              {"celo"},
	"cosmos":            {"cosmos"},
	"crypto-com-chain":  {"cryptoorg"},
	"dash":              {"dash"},
	"decred":            {"decred"},
	"digibyte":          {"digibyte"},
	"dogecoin":          {"doge"},
	"ecash":             {"ecash"},
	"ethereum":          {"ethereum"},
	"ethereum-classic":  {"classic"},
	"fantom":            {"fantom"},
	"filecoin":          {"filecoin"},
	"fio-protocol":      {"fio"},
	"firo":              {"firo"},
	"harmony":           {"harmony"},
	"hedera-hashgraph":  {"hedera"},
	"icon":              {"icon"},
	"internet-computer": {"internet_computer"},
	"iostoken":          {"iost"},
	"iota":              {"iota"},
	"iotex":             {"iotex"},
	"iris-network":      {"iris"},
	"kava":              {"kava"},
	"klay-token":        {"klaytn"},
	"komodo":            {"komodo"},
	"kusama":            {"kusama"},
	"litecoin":          {"litecoin"},
	"matic-network":     {"polygon"},
	"near":              {"near"},
	"neo":               {"neo"},
	"neutron-3":         {"neutron"},
	"nimiq-2":           {"nimiq"},
	"okb":               {"okc"},
	"ontology":          {"ontology"},
	"osmosis":           {"osmosis"},
	"polkadot":          {"polkadot"},
	"qtum":              {"qtum"},
	"ripple":            {"ripple"},
	"secret":            {"secret"},
	"sei-network":       {"sei"},
	"solana":            {"solana"},
	"stellar":           {"stellar"},
	"sui":               {"sui"},
	"syscoin":           {"syscoin"},
	"terra-luna":        {"terra"},
	"terra-luna-2":      {"terrav2"},
	"tezos":             {"tezos"},
	"theta-token":       {"theta"},
	"the-open-network":  {"ton"},
	"thorchain":         {"thorchain"},
	"tron":              {"tron"},
	"vechain":           {"vechain"},
	"verge":             {"verge"},
	"waves":             {"waves"},
	"wemix-token":       {"wemix"},
	"zcash":             {"zcash"},
	"zilliqa":           {"zilliqa"},
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
