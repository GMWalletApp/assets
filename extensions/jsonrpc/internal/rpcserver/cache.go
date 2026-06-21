package rpcserver

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
)

type CacheStore struct {
	marketPath     string
	stablecoinPath string
}

func NewCacheStore(marketPath, stablecoinPath string) *CacheStore {
	return &CacheStore{marketPath: marketPath, stablecoinPath: stablecoinPath}
}

func (c *CacheStore) ReadMarket() (*MarketCache, error) {
	var cache MarketCache
	if err := readJSONFile(c.marketPath, &cache); err != nil {
		if os.IsNotExist(err) {
			return &MarketCache{Source: "coingecko", Assets: []MarketAsset{}}, nil
		}
		return nil, err
	}
	return &cache, nil
}

func (c *CacheStore) ReadStablecoins() (*StablecoinCache, error) {
	var cache StablecoinCache
	if err := readJSONFile(c.stablecoinPath, &cache); err != nil {
		if os.IsNotExist(err) {
			return &StablecoinCache{Source: "defillama", Assets: []StablecoinAsset{}}, nil
		}
		return nil, err
	}
	return &cache, nil
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepathDir(path), ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}

func filterMarketRankings(cache *MarketCache, order string, limit, offset int, onlyWithAssets bool) []MarketAsset {
	items := make([]MarketAsset, 0, len(cache.Assets))
	for _, item := range cache.Assets {
		if onlyWithAssets && len(item.Assets) == 0 {
			continue
		}
		item.Source = defaultString(item.Source, cache.Source)
		item.UpdatedAt = defaultString(item.UpdatedAt, cache.UpdatedAt)
		items = append(items, item)
	}

	switch order {
	case "volume_desc":
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].TotalVolume > items[j].TotalVolume
		})
	case "market_cap_rank_asc":
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].MarketCapRank == 0 {
				return false
			}
			if items[j].MarketCapRank == 0 {
				return true
			}
			return items[i].MarketCapRank < items[j].MarketCapRank
		})
	default:
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].MarketCap > items[j].MarketCap
		})
	}

	return paginateMarket(items, limit, offset)
}

func filterStablecoinRankings(cache *StablecoinCache, limit, offset int, onlyWithAssets bool) []StablecoinAsset {
	items := make([]StablecoinAsset, 0, len(cache.Assets))
	for _, item := range cache.Assets {
		if onlyWithAssets && len(item.Assets) == 0 {
			continue
		}
		item.Source = defaultString(item.Source, cache.Source)
		item.UpdatedAt = defaultString(item.UpdatedAt, cache.UpdatedAt)
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Circulating > items[j].Circulating
	})

	return paginateStablecoins(items, limit, offset)
}

func filterAppTokenList(cache *AppTokenList, chain string, limit, offset, maxRank int, onlyWithMarket bool) *AppTokenList {
	filtered := &AppTokenList{
		Source:    cache.Source,
		UpdatedAt: cache.UpdatedAt,
		Tokens:    make([]AppToken, 0, len(cache.Tokens)),
	}

	for _, token := range cache.Tokens {
		if chain != "" && !stringsEqualFold(token.Chain, chain) {
			continue
		}
		if onlyWithMarket && token.Market == nil {
			continue
		}
		if maxRank > 0 && (token.Rank == 0 || token.Rank > maxRank) {
			continue
		}
		filtered.Tokens = append(filtered.Tokens, token)
	}

	filtered.Tokens = paginateAppTokens(filtered.Tokens, limit, offset)
	return filtered
}

func findMarketByAsset(cache *MarketCache, chain, address string) []MarketAsset {
	var matches []MarketAsset
	key := chainAddressKey(chain, address)
	for _, item := range cache.Assets {
		for _, asset := range item.Assets {
			if chainAddressKey(asset.Chain, asset.Address) == key {
				matches = append(matches, item)
				break
			}
		}
	}
	return matches
}

func findStablecoinBySymbol(cache *StablecoinCache, symbol string) []StablecoinAsset {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	var matches []StablecoinAsset
	for _, item := range cache.Assets {
		if strings.ToUpper(item.Symbol) == symbol {
			matches = append(matches, item)
		}
	}
	return matches
}

func paginateMarket(items []MarketAsset, limit, offset int) []MarketAsset {
	limit, offset = normalizePagination(limit, offset)
	if offset >= len(items) {
		return []MarketAsset{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func paginateStablecoins(items []StablecoinAsset, limit, offset int) []StablecoinAsset {
	limit, offset = normalizePagination(limit, offset)
	if offset >= len(items) {
		return []StablecoinAsset{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func paginateAppTokens(items []AppToken, limit, offset int) []AppToken {
	limit, offset = normalizePagination(limit, offset)
	if offset >= len(items) {
		return []AppToken{}
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end]
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func filepathDir(path string) string {
	idx := strings.LastIndexAny(path, `/\`)
	if idx < 0 {
		return "."
	}
	return path[:idx]
}
