package rpcserver

import (
	"os"
	"strings"
)

func loadTokenListRules(path string) (*TokenListRules, error) {
	rules := &TokenListRules{}
	if strings.TrimSpace(path) == "" {
		return rules, nil
	}
	if err := readJSONFile(path, rules); err != nil {
		if os.IsNotExist(err) {
			return rules, nil
		}
		return nil, err
	}
	rules.normalize()
	return rules, nil
}

func (rules *TokenListRules) normalize() {
	if rules == nil {
		return
	}

	platformMappings := map[string]string{}
	for platform, chain := range rules.PlatformMappings {
		platform = normalizeExternalID(platform)
		chain = strings.ToLower(strings.TrimSpace(chain))
		if platform == "" || chain == "" {
			continue
		}
		platformMappings[platform] = chain
	}
	rules.PlatformMappings = platformMappings

	nativeMappings := map[string][]string{}
	for coinGeckoID, chains := range rules.NativeMarketMappings {
		coinGeckoID = normalizeExternalID(coinGeckoID)
		if coinGeckoID == "" {
			continue
		}
		nativeMappings[coinGeckoID] = appendUniqueStrings(nil, chains...)
	}
	rules.NativeMarketMappings = nativeMappings

	for i := range rules.AssetOverrides {
		rules.AssetOverrides[i].Chain = strings.ToLower(strings.TrimSpace(rules.AssetOverrides[i].Chain))
		rules.AssetOverrides[i].Address = strings.TrimSpace(rules.AssetOverrides[i].Address)
		rules.AssetOverrides[i].CoinGeckoID = normalizeExternalID(rules.AssetOverrides[i].CoinGeckoID)
		rules.AssetOverrides[i].DisplayName = strings.TrimSpace(rules.AssetOverrides[i].DisplayName)
		rules.AssetOverrides[i].DisplaySymbol = strings.TrimSpace(rules.AssetOverrides[i].DisplaySymbol)
		rules.AssetOverrides[i].AddTags = appendUniqueStrings(nil, rules.AssetOverrides[i].AddTags...)
	}

	for i := range rules.MarketTagRules {
		rules.MarketTagRules[i].CoinGeckoID = normalizeExternalID(rules.MarketTagRules[i].CoinGeckoID)
		rules.MarketTagRules[i].AddTags = appendUniqueStrings(nil, rules.MarketTagRules[i].AddTags...)
	}
}

func (rules *TokenListRules) ruleStats() ReportRuleStats {
	if rules == nil {
		return ReportRuleStats{}
	}
	return ReportRuleStats{
		ConfiguredPlatformMappings:     len(rules.PlatformMappings),
		ConfiguredNativeMarketMappings: len(rules.NativeMarketMappings),
		ConfiguredAssetOverrides:       len(rules.AssetOverrides),
		ConfiguredMarketTagRules:       len(rules.MarketTagRules),
	}
}

func coinGeckoPlatformChainWithRules(platform string, rules *TokenListRules) (string, bool, bool) {
	platform = normalizeExternalID(platform)
	if platform == "" {
		return "", false, false
	}
	if rules != nil {
		if chain, ok := rules.PlatformMappings[platform]; ok {
			return chain, true, chain != ""
		}
	}
	chain, ok := coinGeckoPlatformChain(platform)
	return chain, false, ok
}

func coinGeckoNativeChainsWithRules(coingeckoID string, rules *TokenListRules) ([]string, bool) {
	coingeckoID = normalizeExternalID(coingeckoID)
	if coingeckoID == "" {
		return nil, false
	}
	if rules != nil {
		if chains, ok := rules.NativeMarketMappings[coingeckoID]; ok {
			return append([]string(nil), chains...), true
		}
	}
	return append([]string(nil), coinGeckoNativeChains[coingeckoID]...), false
}

func (rules *TokenListRules) assetOverride(chain, address string) (*TokenListAssetOverride, bool) {
	if rules == nil {
		return nil, false
	}
	key := chainAddressKey(chain, address)
	for i := len(rules.AssetOverrides) - 1; i >= 0; i-- {
		override := &rules.AssetOverrides[i]
		if chainAddressKey(override.Chain, override.Address) == key {
			return override, true
		}
	}
	return nil, false
}

func (rules *TokenListRules) marketTagRules(coingeckoID string) []TokenListMarketTagRule {
	if rules == nil {
		return nil
	}
	coingeckoID = normalizeExternalID(coingeckoID)
	var matches []TokenListMarketTagRule
	for _, rule := range rules.MarketTagRules {
		if rule.CoinGeckoID == coingeckoID {
			matches = append(matches, rule)
		}
	}
	return matches
}

func appendUniqueStrings(dst []string, values ...string) []string {
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		exists := false
		for _, existing := range dst {
			if strings.EqualFold(existing, value) {
				exists = true
				break
			}
		}
		if !exists {
			dst = append(dst, value)
		}
	}
	return dst
}
