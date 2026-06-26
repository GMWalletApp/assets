package rpcserver

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type TokenListConfigOperation string

const (
	TokenListConfigOperationOverrideUpsert    TokenListConfigOperation = "override_upsert"
	TokenListConfigOperationOverrideDelete    TokenListConfigOperation = "override_delete"
	TokenListConfigOperationManualTokenUpsert TokenListConfigOperation = "manual_token_upsert"
	TokenListConfigOperationManualTokenDelete TokenListConfigOperation = "manual_token_delete"
	TokenListConfigOperationHotReplaceCurrent TokenListConfigOperation = "hot_replace_current"
	TokenListConfigOperationHotAddCurrent     TokenListConfigOperation = "hot_add_current"
	TokenListConfigOperationHotRemoveCurrent  TokenListConfigOperation = "hot_remove_current"
	TokenListConfigOperationHotResetCurrent   TokenListConfigOperation = "hot_reset_current"
)

type TokenListConfigUpdateResult struct {
	ManualOverridesUpdated bool
	ManualTokensUpdated    bool
	HotCurrentUpdated      bool
}

func ParseTokenListConfigOperation(value string) (TokenListConfigOperation, error) {
	switch TokenListConfigOperation(strings.ToLower(strings.TrimSpace(value))) {
	case TokenListConfigOperationOverrideUpsert:
		return TokenListConfigOperationOverrideUpsert, nil
	case TokenListConfigOperationOverrideDelete:
		return TokenListConfigOperationOverrideDelete, nil
	case TokenListConfigOperationManualTokenUpsert:
		return TokenListConfigOperationManualTokenUpsert, nil
	case TokenListConfigOperationManualTokenDelete:
		return TokenListConfigOperationManualTokenDelete, nil
	case TokenListConfigOperationHotReplaceCurrent:
		return TokenListConfigOperationHotReplaceCurrent, nil
	case TokenListConfigOperationHotAddCurrent:
		return TokenListConfigOperationHotAddCurrent, nil
	case TokenListConfigOperationHotRemoveCurrent:
		return TokenListConfigOperationHotRemoveCurrent, nil
	case TokenListConfigOperationHotResetCurrent:
		return TokenListConfigOperationHotResetCurrent, nil
	default:
		return "", fmt.Errorf("unsupported tokenlist config operation %q", value)
	}
}

func ApplyTokenListConfigOperation(root, manualOverridesPath, manualTokensPath, hotCurrentPath string, operation TokenListConfigOperation, payloadJSON string) (TokenListConfigUpdateResult, error) {
	result := TokenListConfigUpdateResult{}
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	manualOverridesPath = resolveCachePath(root, defaultString(strings.TrimSpace(manualOverridesPath), DefaultTokenListManualOverridesPath))
	manualTokensPath = resolveCachePath(root, defaultString(strings.TrimSpace(manualTokensPath), DefaultTokenListManualTokensPath))
	hotCurrentPath = resolveCachePath(root, defaultString(strings.TrimSpace(hotCurrentPath), DefaultTokenListHotCurrentPath))
	payloadJSON = strings.TrimSpace(payloadJSON)

	switch operation {
	case TokenListConfigOperationOverrideUpsert:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListAssetOverrides(manualOverridesPath)
		if err != nil {
			return result, err
		}
		updates, err := parseInlineTokenListAssetOverrides(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		for i := range updates {
			if err := validateActionTokenListAssetOverride(root, &updates[i]); err != nil {
				return result, err
			}
		}
		merged := mergeTokenListAssetOverrides(existing, updates)
		sortTokenListAssetOverrides(merged)
		if err := writeJSONAtomic(manualOverridesPath, TokenListAssetOverridesFile{AssetOverrides: merged}); err != nil {
			return result, err
		}
		result.ManualOverridesUpdated = true
	case TokenListConfigOperationOverrideDelete:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListAssetOverrides(manualOverridesPath)
		if err != nil {
			return result, err
		}
		deletes, err := parseInlineTokenListAssetOverrides(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		keys := map[string]struct{}{}
		for i := range deletes {
			if err := validateActionTokenListAssetOverride(root, &deletes[i]); err != nil {
				return result, err
			}
			keys[chainAddressKey(deletes[i].Chain, deletes[i].Address)] = struct{}{}
		}
		filtered := filterTokenListAssetOverrides(existing, keys)
		sortTokenListAssetOverrides(filtered)
		if err := writeJSONAtomic(manualOverridesPath, TokenListAssetOverridesFile{AssetOverrides: filtered}); err != nil {
			return result, err
		}
		result.ManualOverridesUpdated = true
	case TokenListConfigOperationManualTokenUpsert:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListManualTokens(manualTokensPath)
		if err != nil {
			return result, err
		}
		updates, err := parseInlineTokenListManualTokens(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		localAssetKeys, err := buildTokenListLocalAssetKeySet(root)
		if err != nil {
			return result, err
		}
		if err := validateTokenListManualTokens(root, updates, localAssetKeys, true, "payload_json"); err != nil {
			return result, err
		}
		merged := mergeTokenListManualTokens(existing, updates)
		if err := validateTokenListManualTokens(root, merged, localAssetKeys, false, manualTokensPath); err != nil {
			return result, err
		}
		sortTokenListManualTokens(merged)
		if err := writeJSONAtomic(manualTokensPath, TokenListManualTokensFile{Tokens: merged}); err != nil {
			return result, err
		}
		result.ManualTokensUpdated = true
	case TokenListConfigOperationManualTokenDelete:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListManualTokens(manualTokensPath)
		if err != nil {
			return result, err
		}
		deletes, err := parseInlineTokenListManualTokens(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		keys := map[string]struct{}{}
		for i := range deletes {
			if err := validateActionTokenListManualTokenDeleteRef(root, &deletes[i]); err != nil {
				return result, err
			}
			keys[chainAddressKey(deletes[i].Chain, deletes[i].Address)] = struct{}{}
		}
		filtered := filterTokenListManualTokens(existing, keys)
		localAssetKeys, err := buildTokenListLocalAssetKeySet(root)
		if err != nil {
			return result, err
		}
		if err := validateTokenListManualTokens(root, filtered, localAssetKeys, false, manualTokensPath); err != nil {
			return result, err
		}
		sortTokenListManualTokens(filtered)
		if err := writeJSONAtomic(manualTokensPath, TokenListManualTokensFile{Tokens: filtered}); err != nil {
			return result, err
		}
		result.ManualTokensUpdated = true
	case TokenListConfigOperationHotReplaceCurrent:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		entries, err := parseInlineTokenListHotEntries(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		for i := range entries {
			if err := validateActionTokenListHotEntry(root, &entries[i]); err != nil {
				return result, err
			}
		}
		entries = dedupeTokenListHotEntries(entries)
		sortTokenListHotEntries(entries)
		if err := writeJSONAtomic(hotCurrentPath, TokenListHotList{Tokens: entries}); err != nil {
			return result, err
		}
		result.HotCurrentUpdated = true
	case TokenListConfigOperationHotAddCurrent:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListHotEntries(hotCurrentPath)
		if err != nil {
			return result, err
		}
		updates, err := parseInlineTokenListHotEntries(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		for i := range updates {
			if err := validateActionTokenListHotEntry(root, &updates[i]); err != nil {
				return result, err
			}
		}
		entries := dedupeTokenListHotEntries(append(existing, updates...))
		sortTokenListHotEntries(entries)
		if err := writeJSONAtomic(hotCurrentPath, TokenListHotList{Tokens: entries}); err != nil {
			return result, err
		}
		result.HotCurrentUpdated = true
	case TokenListConfigOperationHotRemoveCurrent:
		if payloadJSON == "" {
			return result, fmt.Errorf("%s requires payload_json", operation)
		}
		existing, err := loadTokenListHotEntries(hotCurrentPath)
		if err != nil {
			return result, err
		}
		deletes, err := parseInlineTokenListHotEntries(payloadJSON, "payload_json")
		if err != nil {
			return result, err
		}
		keys := map[string]struct{}{}
		for i := range deletes {
			if err := validateActionTokenListHotEntry(root, &deletes[i]); err != nil {
				return result, err
			}
			keys[chainAddressKey(deletes[i].Chain, deletes[i].Address)] = struct{}{}
		}
		filtered := filterTokenListHotEntries(existing, keys)
		sortTokenListHotEntries(filtered)
		if err := writeJSONAtomic(hotCurrentPath, TokenListHotList{Tokens: filtered}); err != nil {
			return result, err
		}
		result.HotCurrentUpdated = true
	case TokenListConfigOperationHotResetCurrent:
		if payloadJSON != "" {
			return result, fmt.Errorf("%s does not accept payload_json", operation)
		}
		if err := writeJSONAtomic(hotCurrentPath, TokenListHotList{Tokens: []TokenListHotEntry{}}); err != nil {
			return result, err
		}
		result.HotCurrentUpdated = true
	default:
		return result, fmt.Errorf("unsupported tokenlist config operation %q", operation)
	}

	return result, nil
}

func loadTokenListAssetOverrides(path string) ([]TokenListAssetOverride, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := parseTokenListAssetOverridesJSON(data, path)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		normalizeTokenListAssetOverride(&entries[i])
	}
	return mergeTokenListAssetOverrides(nil, entries), nil
}

func loadTokenListHotEntries(path string) ([]TokenListHotEntry, error) {
	if strings.TrimSpace(path) == "" {
		return []TokenListHotEntry{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TokenListHotEntry{}, nil
		}
		return nil, err
	}
	entries, err := parseTokenListHotEntriesJSON(data, path)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		normalizeTokenListHotEntry(&entries[i])
	}
	return dedupeTokenListHotEntries(entries), nil
}

func loadTokenListManualTokens(path string) ([]AppToken, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	entries, err := parseTokenListManualTokensJSON(data, path)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		normalizeTokenListManualToken(&entries[i])
	}
	return append([]AppToken(nil), entries...), nil
}

func parseInlineTokenListAssetOverrides(input, origin string) ([]TokenListAssetOverride, error) {
	return parseTokenListAssetOverridesJSON([]byte(input), origin)
}

func parseInlineTokenListHotEntries(input, origin string) ([]TokenListHotEntry, error) {
	return parseTokenListHotEntriesJSON([]byte(input), origin)
}

func parseInlineTokenListManualTokens(input, origin string) ([]AppToken, error) {
	return parseTokenListManualTokensJSON([]byte(input), origin)
}

func parseTokenListAssetOverridesJSON(data []byte, origin string) ([]TokenListAssetOverride, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, nil
	}

	var wrapped TokenListAssetOverridesFile
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.AssetOverrides != nil {
		return wrapped.AssetOverrides, nil
	}

	var entries []TokenListAssetOverride
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries, nil
	}

	var entry TokenListAssetOverride
	if err := json.Unmarshal(data, &entry); err == nil {
		return []TokenListAssetOverride{entry}, nil
	}

	return nil, fmt.Errorf("%s: expected an override object, an override array, or an object with assetOverrides[]", origin)
}

func parseTokenListHotEntriesJSON(data []byte, origin string) ([]TokenListHotEntry, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, nil
	}

	var wrapped TokenListHotList
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Tokens != nil {
		return wrapped.Tokens, nil
	}

	var entries []TokenListHotEntry
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries, nil
	}

	var entry TokenListHotEntry
	if err := json.Unmarshal(data, &entry); err == nil {
		return []TokenListHotEntry{entry}, nil
	}

	return nil, fmt.Errorf("%s: expected a hot token object, a hot token array, or an object with tokens[]", origin)
}

func parseTokenListManualTokensJSON(data []byte, origin string) ([]AppToken, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return nil, nil
	}

	var wrapped TokenListManualTokensFile
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.Tokens != nil {
		return wrapped.Tokens, nil
	}

	var entries []AppToken
	if err := json.Unmarshal(data, &entries); err == nil {
		return entries, nil
	}

	var entry AppToken
	if err := json.Unmarshal(data, &entry); err == nil {
		return []AppToken{entry}, nil
	}

	return nil, fmt.Errorf("%s: expected a manual token object, a manual token array, or an object with tokens[]", origin)
}

func validateActionTokenListAssetOverride(root string, entry *TokenListAssetOverride) error {
	normalizeTokenListAssetOverride(entry)
	if entry.Chain == "" {
		return fmt.Errorf("asset override missing chain")
	}
	if !tokenListChainExists(root, entry.Chain) {
		return fmt.Errorf("asset override uses unknown chain %q", entry.Chain)
	}
	if entry.Address == "" {
		return fmt.Errorf("asset override %s missing address", entry.Chain)
	}
	return nil
}

func validateActionTokenListHotEntry(root string, entry *TokenListHotEntry) error {
	normalizeTokenListHotEntry(entry)
	if entry.Chain == "" {
		return fmt.Errorf("hot token missing chain")
	}
	if !tokenListChainExists(root, entry.Chain) {
		return fmt.Errorf("hot token uses unknown chain %q", entry.Chain)
	}
	return nil
}

func validateActionTokenListManualTokenDeleteRef(root string, entry *AppToken) error {
	normalizeTokenListManualToken(entry)
	if entry.Chain == "" {
		return fmt.Errorf("manual token delete missing chain")
	}
	if !tokenListChainExists(root, entry.Chain) {
		return fmt.Errorf("manual token delete uses unknown chain %q", entry.Chain)
	}
	return nil
}

func tokenListChainExists(root, chain string) bool {
	infoPath := filepath.Join(root, "blockchains", chain, "info", "info.json")
	info, err := os.Stat(infoPath)
	return err == nil && !info.IsDir()
}

func normalizeTokenListAssetOverride(entry *TokenListAssetOverride) {
	if entry == nil {
		return
	}
	entry.Chain = strings.ToLower(strings.TrimSpace(entry.Chain))
	entry.Address = strings.TrimSpace(entry.Address)
	entry.CoinGeckoID = normalizeExternalID(entry.CoinGeckoID)
	entry.DisplayName = strings.TrimSpace(entry.DisplayName)
	entry.DisplaySymbol = strings.TrimSpace(entry.DisplaySymbol)
	entry.AddTags = appendUniqueStrings(nil, entry.AddTags...)
	entry.AddTags = removeStringTag(entry.AddTags, "hot")
	entry.Note = strings.TrimSpace(entry.Note)
}

func normalizeTokenListHotEntry(entry *TokenListHotEntry) {
	if entry == nil {
		return
	}
	entry.Chain = strings.ToLower(strings.TrimSpace(entry.Chain))
	entry.Address = strings.TrimSpace(entry.Address)
}

func normalizeTokenListManualToken(entry *AppToken) {
	if entry == nil {
		return
	}
	entry.Kind = strings.ToLower(strings.TrimSpace(entry.Kind))
	entry.Chain = strings.ToLower(strings.TrimSpace(entry.Chain))
	entry.Address = strings.TrimSpace(entry.Address)
}

func validateActionTokenListManualToken(root string, entry *AppToken, localAssetKeys map[string]struct{}, origin string) error {
	normalizeTokenListManualToken(entry)
	if entry.Chain == "" {
		return fmt.Errorf("%s: manual token missing chain", origin)
	}
	if !tokenListChainExists(root, entry.Chain) {
		return fmt.Errorf("%s: manual token uses unknown chain %q", origin, entry.Chain)
	}
	switch entry.Kind {
	case "token":
		if entry.Address == "" {
			return fmt.Errorf("%s: manual token %s missing address", origin, entry.Chain)
		}
	default:
		return fmt.Errorf("%s: manual token %s/%s has unsupported kind %q; only kind=token is allowed", origin, entry.Chain, entry.Address, entry.Kind)
	}
	if _, ok := localAssetKeys[chainAddressKey(entry.Chain, entry.Address)]; ok {
		return fmt.Errorf("%s: manual token %s/%s conflicts with a local asset", origin, entry.Chain, entry.Address)
	}
	return nil
}

func validateTokenListManualTokens(root string, entries []AppToken, localAssetKeys map[string]struct{}, rejectDuplicateKeys bool, origin string) error {
	seen := map[string]struct{}{}
	for i := range entries {
		if err := validateActionTokenListManualToken(root, &entries[i], localAssetKeys, origin); err != nil {
			return err
		}
		key := chainAddressKey(entries[i].Chain, entries[i].Address)
		if _, ok := seen[key]; ok && rejectDuplicateKeys {
			return fmt.Errorf("%s: duplicate manual token key %s/%s", origin, entries[i].Chain, entries[i].Address)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func buildTokenListLocalAssetKeySet(root string) (map[string]struct{}, error) {
	index, err := NewStore(root, DefaultAssetBaseURL).BuildAssetIndex()
	if err != nil {
		return nil, err
	}
	keys := make(map[string]struct{}, len(index.nativeAssets)+len(index.tokenAssets))
	for _, asset := range index.NativeAssets() {
		keys[chainAddressKey(asset.Chain, asset.Address)] = struct{}{}
	}
	for _, asset := range index.TokenAssets() {
		keys[chainAddressKey(asset.Chain, asset.Address)] = struct{}{}
	}
	return keys, nil
}

func mergeTokenListAssetOverrides(base, updates []TokenListAssetOverride) []TokenListAssetOverride {
	merged := make([]TokenListAssetOverride, 0, len(base)+len(updates))
	indexByKey := map[string]int{}
	appendEntry := func(entry TokenListAssetOverride) {
		normalizeTokenListAssetOverride(&entry)
		if entry.Chain == "" || entry.Address == "" {
			merged = append(merged, entry)
			return
		}
		key := chainAddressKey(entry.Chain, entry.Address)
		if idx, ok := indexByKey[key]; ok {
			merged[idx] = entry
			return
		}
		indexByKey[key] = len(merged)
		merged = append(merged, entry)
	}

	for _, entry := range base {
		appendEntry(entry)
	}
	for _, entry := range updates {
		appendEntry(entry)
	}
	return merged
}

func dedupeTokenListHotEntries(entries []TokenListHotEntry) []TokenListHotEntry {
	deduped := make([]TokenListHotEntry, 0, len(entries))
	indexByKey := map[string]int{}
	for _, entry := range entries {
		normalizeTokenListHotEntry(&entry)
		if entry.Chain == "" {
			continue
		}
		key := chainAddressKey(entry.Chain, entry.Address)
		if idx, ok := indexByKey[key]; ok {
			deduped[idx] = entry
			continue
		}
		indexByKey[key] = len(deduped)
		deduped = append(deduped, entry)
	}
	return deduped
}

func mergeTokenListManualTokens(base, updates []AppToken) []AppToken {
	merged := make([]AppToken, 0, len(base)+len(updates))
	indexByKey := map[string]int{}
	appendEntry := func(entry AppToken) {
		normalizeTokenListManualToken(&entry)
		key := chainAddressKey(entry.Chain, entry.Address)
		if idx, ok := indexByKey[key]; ok {
			merged[idx] = entry
			return
		}
		indexByKey[key] = len(merged)
		merged = append(merged, entry)
	}

	for _, entry := range base {
		appendEntry(entry)
	}
	for _, entry := range updates {
		appendEntry(entry)
	}
	return merged
}

func filterTokenListAssetOverrides(entries []TokenListAssetOverride, dropKeys map[string]struct{}) []TokenListAssetOverride {
	if len(dropKeys) == 0 {
		return mergeTokenListAssetOverrides(nil, entries)
	}
	filtered := make([]TokenListAssetOverride, 0, len(entries))
	for _, entry := range entries {
		if _, ok := dropKeys[chainAddressKey(entry.Chain, entry.Address)]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	return mergeTokenListAssetOverrides(nil, filtered)
}

func filterTokenListHotEntries(entries []TokenListHotEntry, dropKeys map[string]struct{}) []TokenListHotEntry {
	if len(dropKeys) == 0 {
		return dedupeTokenListHotEntries(entries)
	}
	filtered := make([]TokenListHotEntry, 0, len(entries))
	for _, entry := range entries {
		if _, ok := dropKeys[chainAddressKey(entry.Chain, entry.Address)]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	return dedupeTokenListHotEntries(filtered)
}

func filterTokenListManualTokens(entries []AppToken, dropKeys map[string]struct{}) []AppToken {
	if len(dropKeys) == 0 {
		return mergeTokenListManualTokens(nil, entries)
	}
	filtered := make([]AppToken, 0, len(entries))
	for _, entry := range entries {
		if _, ok := dropKeys[chainAddressKey(entry.Chain, entry.Address)]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	return mergeTokenListManualTokens(nil, filtered)
}

func sortTokenListAssetOverrides(entries []TokenListAssetOverride) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Chain != entries[j].Chain {
			return entries[i].Chain < entries[j].Chain
		}
		return strings.ToLower(entries[i].Address) < strings.ToLower(entries[j].Address)
	})
}

func sortTokenListHotEntries(entries []TokenListHotEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Chain != entries[j].Chain {
			return entries[i].Chain < entries[j].Chain
		}
		return strings.ToLower(entries[i].Address) < strings.ToLower(entries[j].Address)
	})
}

func sortTokenListManualTokens(entries []AppToken) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Chain != entries[j].Chain {
			return entries[i].Chain < entries[j].Chain
		}
		return strings.ToLower(entries[i].Address) < strings.ToLower(entries[j].Address)
	})
}
