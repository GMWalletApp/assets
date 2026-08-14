package rpcserver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"
)

type seedListSpec struct {
	Key           string
	Name          string
	Description   string
	DisplayName   string
	DisplaySymbol string
	LogoURI       string
	Match         func(ManagedToken) bool
}

var appTokenSeedLists = []seedListSpec{
	{
		Key:         "tokenlist",
		Name:        "App Token List",
		Description: "All app-packaged tokens from extensions/jsonrpc/data/tokenlist.json.",
		Match:       func(ManagedToken) bool { return true },
	},
	{
		Key:           "usdt",
		Name:          "USDT List",
		Description:   "Multi-chain USDT family list, including USDT0 variants where the app should treat them as the USDT slot.",
		DisplayName:   "Tether USD",
		DisplaySymbol: "USDT",
		LogoURI:       DefaultUSDTFamilyLogoURI,
		Match: func(token ManagedToken) bool {
			symbol := strings.ToUpper(token.Symbol)
			return symbol == "USDT" || symbol == "USDT0"
		},
	},
	{
		Key:           "usdc",
		Name:          "USDC List",
		Description:   "Multi-chain native and bridged USDC family list.",
		DisplayName:   "USD Coin",
		DisplaySymbol: "USDC",
		LogoURI:       "https://assets-cdn.trustwallet.com/blockchains/ethereum/assets/0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48/logo.png",
		Match: func(token ManagedToken) bool {
			symbol := strings.ToUpper(token.Symbol)
			return symbol == "USDC" || symbol == "USDC.E"
		},
	},
	{
		Key:           "usdg",
		Name:          "USDG List",
		Description:   "Multi-chain Global Dollar list.",
		DisplayName:   "Global Dollar",
		DisplaySymbol: "USDG",
		LogoURI:       "https://assets-cdn.trustwallet.com/blockchains/ethereum/assets/0xe343167631d89B6Ffc58B88d6b7fB0228795491D/logo.png",
		Match:         matchSymbol("USDG"),
	},
	{
		Key:         "stablecoin",
		Name:        "Stablecoin List",
		Description: "Multi-chain stablecoin list derived from token tags.",
		Match: func(token ManagedToken) bool {
			return hasTag(token.Tags, "stablecoin")
		},
	},
	{
		Key:         "eth",
		Name:        "ETH List",
		Description: "Multi-chain ETH and wrapped ETH-family entries represented with symbol ETH.",
		Match:       matchSymbol("ETH"),
	},
	{
		Key:           "usds",
		Name:          "USDS List",
		Description:   "Multi-chain Sky USDS list.",
		DisplayName:   "USDS",
		DisplaySymbol: "USDS",
		LogoURI:       "https://assets-cdn.trustwallet.com/blockchains/ethereum/assets/0xdC035D45d973E3EC169d2276DDab16f1e407384F/logo.png",
		Match: func(token ManagedToken) bool {
			return strings.EqualFold(token.Symbol, "USDS") && strings.Contains(strings.ToUpper(token.Name), "USDS")
		},
	},
	{
		Key:         "dai",
		Name:        "DAI List",
		Description: "Multi-chain DAI list.",
		Match:       matchSymbol("DAI"),
	},
}

func (s *ManagedListService) seedFromAppTokenList(db *sql.DB) error {
	if strings.TrimSpace(s.tokenListSeedPath) == "" {
		return nil
	}
	data, err := os.ReadFile(s.tokenListSeedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var source AppTokenList
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}
	tokens := make([]ManagedToken, 0, len(source.Tokens))
	for _, token := range source.Tokens {
		tokens = append(tokens, managedTokenFromAppToken(token))
	}

	for _, spec := range appTokenSeedLists {
		if err := seedList(db, spec.Key, spec.Name, spec.Description, spec.DisplayName, spec.DisplaySymbol, spec.LogoURI, spec.Key+".json"); err != nil {
			return err
		}
		rank := 1
		for _, token := range tokens {
			if !spec.Match(token) {
				continue
			}
			slot := ""
			if spec.Key != "tokenlist" {
				slot = spec.Key
			}
			display := defaultManagedListDisplay(spec.Key, token.Chain)
			if err := seedListItem(db, spec.Key, s.enrichChainContext(token), slot, rank, display, "", "", ""); err != nil {
				return err
			}
			rank++
		}
	}
	return nil
}

func (s *ManagedListService) seedFromManualTokenList(db *sql.DB) error {
	if strings.TrimSpace(s.manualTokensPath) == "" {
		return nil
	}
	tokens, err := loadTokenListManualTokens(s.manualTokensPath)
	if err != nil {
		return err
	}
	for _, spec := range appTokenSeedLists {
		if err := seedList(db, spec.Key, spec.Name, spec.Description, spec.DisplayName, spec.DisplaySymbol, spec.LogoURI, spec.Key+".json"); err != nil {
			return err
		}
		rank, err := nextSeedRank(db, spec.Key)
		if err != nil {
			return err
		}
		for _, appToken := range tokens {
			token := managedTokenFromAppToken(appToken)
			if !spec.Match(token) {
				continue
			}
			slot := ""
			if spec.Key != "tokenlist" {
				slot = spec.Key
			}
			display := defaultManagedListDisplay(spec.Key, token.Chain)
			if err := seedListItem(db, spec.Key, s.enrichChainContext(token), slot, rank, display, "", "", ""); err != nil {
				return err
			}
			rank++
		}
	}
	return nil
}

func nextSeedRank(db *sql.DB, listKey string) (int, error) {
	var rank int
	err := db.QueryRow(`
		select coalesce(max(li.rank), 0) + 1
		from list_items li
		join lists l on l.id = li.list_id
		where l.key = ?
	`, normalizeListKey(listKey)).Scan(&rank)
	return rank, err
}

func (s *ManagedListService) seedFromHomepage(db *sql.DB) error {
	if strings.TrimSpace(s.homepageSeedPath) == "" {
		return nil
	}
	data, err := os.ReadFile(s.homepageSeedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var source struct {
		Tokens []struct {
			ID             string          `json:"id"`
			Chain          string          `json:"chain"`
			Slot           string          `json:"slot"`
			Kind           string          `json:"kind"`
			DisplaySymbol  string          `json:"displaySymbol"`
			DisplayName    string          `json:"displayName"`
			DisplayLogoURI string          `json:"displayLogoURI"`
			ChainName      string          `json:"chainName"`
			ChainID        int             `json:"chainId"`
			ChainLogoURI   string          `json:"chainLogoURI"`
			Tags           []string        `json:"tags"`
			Symbol         string          `json:"symbol"`
			Name           string          `json:"name"`
			Address        *string         `json:"address"`
			Decimals       int             `json:"decimals"`
			LogoURI        string          `json:"logoURI"`
			Explorer       string          `json:"explorer"`
			Hot            bool            `json:"hot"`
			Market         *AppTokenMarket `json:"market"`
			Pairs          []TokenPair     `json:"pairs"`
			Links          []Link          `json:"links"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}
	if err := seedList(db, "homepage", "Homepage List", "Curated homepage token list from data/tokenlists/out/homepage.json.", "", "", "", "homepage.json"); err != nil {
		return err
	}
	for i, token := range source.Tokens {
		address := ""
		if token.Address != nil {
			address = *token.Address
		}
		managed := ManagedToken{
			Kind:         defaultString(token.Kind, "token"),
			Chain:        normalizeChain(token.Chain),
			ChainName:    token.ChainName,
			ChainID:      token.ChainID,
			ChainLogoURI: token.ChainLogoURI,
			Address:      strings.TrimSpace(address),
			AssetID:      token.ID,
			Name:         token.Name,
			Symbol:       token.Symbol,
			Decimals:     token.Decimals,
			Status:       "active",
			LogoURI:      token.LogoURI,
			LogoExists:   token.LogoURI != "",
			Explorer:     token.Explorer,
			Tags:         appendUniqueStrings(nil, token.Tags...),
			Hot:          token.Hot,
			Market:       managedTokenMarketFromAppToken(token.Market),
			Pairs:        append([]TokenPair(nil), token.Pairs...),
			Links:        append([]Link(nil), token.Links...),
		}
		managed, err = preserveManagedTokenUIFromDB(db, managed)
		if err != nil {
			return err
		}
		if err := seedListItem(db, "homepage", s.enrichChainContext(managed), token.Slot, i+1, true, token.DisplayName, token.DisplaySymbol, token.DisplayLogoURI); err != nil {
			return err
		}
	}
	return nil
}

func preserveManagedTokenUIFromDB(db *sql.DB, token ManagedToken) (ManagedToken, error) {
	var hot int
	var marketJSON, pairsJSON, linksJSON string
	err := db.QueryRow(`select hot, market_json, pairs_json, links_json from tokens where chain = ? and address = ?`, token.Chain, token.Address).Scan(&hot, &marketJSON, &pairsJSON, &linksJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return token, nil
	}
	if err != nil {
		return ManagedToken{}, err
	}
	token.Hot = hot != 0
	if marketJSON != "" && marketJSON != "null" {
		if err := json.Unmarshal([]byte(marketJSON), &token.Market); err != nil {
			return ManagedToken{}, err
		}
	}
	if pairsJSON != "" {
		if err := json.Unmarshal([]byte(pairsJSON), &token.Pairs); err != nil {
			return ManagedToken{}, err
		}
	}
	if linksJSON != "" {
		if err := json.Unmarshal([]byte(linksJSON), &token.Links); err != nil {
			return ManagedToken{}, err
		}
	}
	return token, nil
}

func seedList(db *sql.DB, key, name, description, displayName, displaySymbol, logoURI, outputPath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		insert into lists(key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
		on conflict(key) do update set
			display_name = case when lists.display_name = '' then excluded.display_name else lists.display_name end,
			display_symbol = case when lists.display_symbol = '' then excluded.display_symbol else lists.display_symbol end,
			logo_uri = case when lists.logo_uri = '' then excluded.logo_uri else lists.logo_uri end
	`, normalizeListKey(key), name, description, displayName, displaySymbol, logoURI, outputPath, now, now)
	return err
}

func seedListItem(db *sql.DB, listKey string, token ManagedToken, slot string, rank int, display bool, displayName, displaySymbol, displayLogoURI string) error {
	token.Chain = normalizeChain(token.Chain)
	token.Address = strings.TrimSpace(token.Address)
	if token.Chain == "" {
		return nil
	}
	if token.Kind == "" {
		if token.Address == "" {
			token.Kind = "native"
		} else {
			token.Kind = "token"
		}
	}
	token.Tags = appendUniqueStrings(nil, token.Tags...)
	tagsJSON, err := json.Marshal(token.Tags)
	if err != nil {
		return err
	}
	marketJSON, err := json.Marshal(token.Market)
	if err != nil {
		return err
	}
	pairsJSON, err := json.Marshal(token.Pairs)
	if err != nil {
		return err
	}
	linksJSON, err := json.Marshal(token.Links)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		insert into tokens(kind, chain, chain_name, chain_id, chain_logo_uri, address, asset_id, type, name, symbol, decimals, status, logo_uri, logo_exists, explorer, tags_json, hot, market_json, pairs_json, links_json, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(chain, address) do update set
			kind = excluded.kind,
			chain_name = excluded.chain_name,
			chain_id = excluded.chain_id,
			chain_logo_uri = excluded.chain_logo_uri,
			asset_id = excluded.asset_id,
			type = excluded.type,
			name = excluded.name,
			symbol = excluded.symbol,
			decimals = excluded.decimals,
			status = excluded.status,
			logo_uri = excluded.logo_uri,
			logo_exists = excluded.logo_exists,
			explorer = excluded.explorer,
			tags_json = excluded.tags_json,
			hot = excluded.hot,
			market_json = excluded.market_json,
			pairs_json = excluded.pairs_json,
			links_json = excluded.links_json,
			updated_at = excluded.updated_at
	`, token.Kind, token.Chain, token.ChainName, token.ChainID, token.ChainLogoURI, token.Address, token.AssetID, token.Type, token.Name, token.Symbol, token.Decimals, token.Status, token.LogoURI, boolToInt(token.LogoExists), token.Explorer, string(tagsJSON), boolToInt(token.Hot), string(marketJSON), string(pairsJSON), string(linksJSON), now, now)
	if err != nil {
		return err
	}

	var listID, tokenID int64
	if err := tx.QueryRow(`select id from lists where key = ?`, normalizeListKey(listKey)).Scan(&listID); err != nil {
		return err
	}
	if err := tx.QueryRow(`select id from tokens where chain = ? and address = ?`, token.Chain, token.Address).Scan(&tokenID); err != nil {
		return err
	}
	_, err = tx.Exec(`
		insert into list_items(list_id, token_id, slot, rank, enabled, display, display_name, display_symbol, display_logo_uri, created_at, updated_at)
		values(?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?)
		on conflict(list_id, token_id) do nothing
	`, listID, tokenID, normalizeSlot(slot), rank, boolToInt(display), displayName, displaySymbol, displayLogoURI, now, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func managedTokenFromAppToken(token AppToken) ManagedToken {
	return ManagedToken{
		Kind:       token.Kind,
		Chain:      normalizeChain(token.Chain),
		Address:    strings.TrimSpace(token.Address),
		AssetID:    token.AssetID,
		Type:       token.Type,
		Name:       token.Name,
		Symbol:     token.Symbol,
		Decimals:   token.Decimals,
		Status:     token.Status,
		LogoURI:    token.LogoURI,
		LogoExists: token.LogoExists,
		Explorer:   appTokenLink(token, "explorer"),
		Tags:       appendUniqueStrings(nil, token.Tags...),
		Hot:        token.Hot,
		Market:     managedTokenMarketFromAppToken(token.Market),
		Pairs:      append([]TokenPair(nil), token.Pairs...),
		Links:      append([]Link(nil), token.Links...),
	}
}

func managedTokenMarketFromAppToken(market *AppTokenMarket) *ManagedTokenMarket {
	if market == nil {
		return nil
	}
	return &ManagedTokenMarket{
		CoinGeckoID:   market.CoinGeckoID,
		MarketCapRank: market.MarketCapRank,
		MarketCap:     market.MarketCap,
		TotalVolume:   market.TotalVolume,
		CurrentPrice:  market.CurrentPrice,
		LastUpdated:   market.LastUpdated,
	}
}

func appTokenLink(token AppToken, name string) string {
	for _, link := range token.Links {
		if strings.EqualFold(strings.TrimSpace(link.Name), name) {
			return strings.TrimSpace(link.URL)
		}
	}
	return ""
}

func matchSymbol(symbol string) func(ManagedToken) bool {
	want := strings.ToUpper(symbol)
	return func(token ManagedToken) bool {
		return strings.ToUpper(token.Symbol) == want
	}
}

func defaultManagedListDisplay(listKey, chain string) bool {
	switch normalizeListKey(listKey) {
	case "usdt", "usdc", "usdg", "usds":
		switch normalizeChain(chain) {
		case "arbitrum", "polygon", "smartchain", "ethereum", "tron":
			return true
		default:
			return false
		}
	default:
		return true
	}
}
