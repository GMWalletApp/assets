package rpcserver

import (
	"os"
	"strings"
)

var defaultExcludedStatuses = []string{"spam", "abandoned"}

type ResolvedTokenListConfig struct {
	PlatformMappings     map[string]string
	NativeMarketMappings map[string][]string
	MarketTagRules       []TokenListMarketTagRule
	ExcludedStatuses     []string

	BaseOverrides   []TokenListAssetOverride
	ManualOverrides []TokenListAssetOverride
	AssetOverrides  []TokenListAssetOverride

	HotDefaults []TokenListHotEntry
	HotCurrent  []TokenListHotEntry
	HotEntries  []TokenListHotEntry
}

func loadTokenListRules(path string) (*TokenListRules, error) {
	rules := &TokenListRules{}
	if strings.TrimSpace(path) != "" {
		if err := readJSONFile(path, rules); err != nil {
			if !os.IsNotExist(err) {
				return nil, err
			}
		}
	}
	rules.normalize()
	return rules, nil
}

func loadResolvedTokenListConfig(rulesPath, baseOverridesPath, manualOverridesPath, hotDefaultsPath, hotCurrentPath string) (*ResolvedTokenListConfig, error) {
	rules, err := loadTokenListRules(rulesPath)
	if err != nil {
		return nil, err
	}
	baseOverrides, err := loadTokenListAssetOverrides(baseOverridesPath)
	if err != nil {
		return nil, err
	}
	manualOverrides, err := loadTokenListAssetOverrides(manualOverridesPath)
	if err != nil {
		return nil, err
	}
	hotDefaults, err := loadTokenListHotEntries(hotDefaultsPath)
	if err != nil {
		return nil, err
	}
	hotCurrent, err := loadTokenListHotEntries(hotCurrentPath)
	if err != nil {
		return nil, err
	}

	return &ResolvedTokenListConfig{
		PlatformMappings:     rules.PlatformMappings,
		NativeMarketMappings: rules.NativeMarketMappings,
		MarketTagRules:       rules.MarketTagRules,
		ExcludedStatuses:     rules.ExcludedStatuses,
		BaseOverrides:        mergeTokenListAssetOverrides(nil, baseOverrides),
		ManualOverrides:      mergeTokenListAssetOverrides(nil, manualOverrides),
		AssetOverrides:       mergeTokenListAssetOverrides(baseOverrides, manualOverrides),
		HotDefaults:          dedupeTokenListHotEntries(hotDefaults),
		HotCurrent:           dedupeTokenListHotEntries(hotCurrent),
		HotEntries:           dedupeTokenListHotEntries(append(append([]TokenListHotEntry{}, hotDefaults...), hotCurrent...)),
	}, nil
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

	for i := range rules.MarketTagRules {
		rules.MarketTagRules[i].CoinGeckoID = normalizeExternalID(rules.MarketTagRules[i].CoinGeckoID)
		rules.MarketTagRules[i].AddTags = appendUniqueStrings(nil, rules.MarketTagRules[i].AddTags...)
		rules.MarketTagRules[i].AddTags = removeStringTag(rules.MarketTagRules[i].AddTags, "hot")
	}

	if rules.ExcludedStatuses == nil {
		rules.ExcludedStatuses = append([]string(nil), defaultExcludedStatuses...)
	} else if len(rules.ExcludedStatuses) == 0 {
		rules.ExcludedStatuses = []string{}
	} else {
		rules.ExcludedStatuses = appendUniqueStrings(nil, rules.ExcludedStatuses...)
	}
}

func (config *ResolvedTokenListConfig) ruleStats() ReportRuleStats {
	if config == nil {
		return ReportRuleStats{}
	}
	return ReportRuleStats{
		ConfiguredPlatformMappings:     len(config.PlatformMappings),
		ConfiguredNativeMarketMappings: len(config.NativeMarketMappings),
		ConfiguredAssetOverrides:       len(config.AssetOverrides),
		BaseAssetOverrides:             len(config.BaseOverrides),
		ManualAssetOverrides:           len(config.ManualOverrides),
		ConfiguredMarketTagRules:       len(config.MarketTagRules),
	}
}

func coinGeckoPlatformChainWithRules(platform string, config *ResolvedTokenListConfig) (string, bool, bool) {
	platform = normalizeExternalID(platform)
	if platform == "" {
		return "", false, false
	}
	if config != nil {
		if chain, ok := config.PlatformMappings[platform]; ok {
			return chain, true, chain != ""
		}
	}
	chain, ok := coinGeckoPlatformChain(platform)
	return chain, false, ok
}

func coinGeckoNativeChainsWithRules(coingeckoID string, config *ResolvedTokenListConfig) ([]string, bool) {
	coingeckoID = normalizeExternalID(coingeckoID)
	if coingeckoID == "" {
		return nil, false
	}
	if config != nil {
		if chains, ok := config.NativeMarketMappings[coingeckoID]; ok {
			return append([]string(nil), chains...), true
		}
	}
	return append([]string(nil), coinGeckoNativeChains[coingeckoID]...), false
}

func (config *ResolvedTokenListConfig) assetOverride(chain, address string) (*TokenListAssetOverride, bool) {
	if config == nil {
		return nil, false
	}
	key := chainAddressKey(chain, address)
	for i := len(config.AssetOverrides) - 1; i >= 0; i-- {
		override := &config.AssetOverrides[i]
		if chainAddressKey(override.Chain, override.Address) == key {
			return override, true
		}
	}
	return nil, false
}

func (config *ResolvedTokenListConfig) marketTagRules(coingeckoID string) []TokenListMarketTagRule {
	if config == nil {
		return nil
	}
	coingeckoID = normalizeExternalID(coingeckoID)
	var matches []TokenListMarketTagRule
	for _, rule := range config.MarketTagRules {
		if rule.CoinGeckoID == coingeckoID {
			matches = append(matches, rule)
		}
	}
	return matches
}

func (config *ResolvedTokenListConfig) isExcludedStatus(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return false
	}
	statuses := defaultExcludedStatuses
	if config != nil && config.ExcludedStatuses != nil {
		statuses = config.ExcludedStatuses
	}
	for _, excluded := range statuses {
		if status == excluded {
			return true
		}
	}
	return false
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

func removeStringTag(values []string, tag string) []string {
	if len(values) == 0 {
		return values
	}
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		return values
	}
	filtered := values[:0]
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), tag) {
			continue
		}
		filtered = append(filtered, value)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}
