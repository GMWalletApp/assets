package rpcserver

import (
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"time"
)

type seedListSpec struct {
	Key         string
	Name        string
	Description string
	Match       func(ManagedToken) bool
}

var appTokenSeedLists = []seedListSpec{
	{
		Key:         "tokenlist",
		Name:        "App Token List",
		Description: "All app-packaged tokens from extensions/jsonrpc/data/tokenlist.json.",
		Match:       func(ManagedToken) bool { return true },
	},
	{
		Key:         "usdt",
		Name:        "USDT List",
		Description: "Multi-chain USDT family list, including USDT0 variants where the app should treat them as the USDT slot.",
		Match: func(token ManagedToken) bool {
			symbol := strings.ToUpper(token.Symbol)
			return symbol == "USDT" || symbol == "USDT0"
		},
	},
	{
		Key:         "usdc",
		Name:        "USDC List",
		Description: "Multi-chain USDC list.",
		Match:       matchSymbol("USDC"),
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
		Key:         "usds",
		Name:        "USDS List",
		Description: "Multi-chain USDS list.",
		Match:       matchSymbol("USDS"),
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
		if err := seedList(db, spec.Key, spec.Name, spec.Description, spec.Key+".json"); err != nil {
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
			if err := seedListItem(db, spec.Key, s.enrichChainContext(token), slot, rank, true, "", "", "seed:"+s.tokenListSeedPath); err != nil {
				return err
			}
			rank++
		}
	}
	return nil
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
			ID            string   `json:"id"`
			Chain         string   `json:"chain"`
			Slot          string   `json:"slot"`
			Kind          string   `json:"kind"`
			DisplaySymbol string   `json:"displaySymbol"`
			DisplayName   string   `json:"displayName"`
			ChainName     string   `json:"chainName"`
			ChainID       int      `json:"chainId"`
			ChainLogoURI  string   `json:"chainLogoURI"`
			Tags          []string `json:"tags"`
			Symbol        string   `json:"symbol"`
			Name          string   `json:"name"`
			Address       *string  `json:"address"`
			Decimals      int      `json:"decimals"`
			LogoURI       string   `json:"logoURI"`
			Explorer      string   `json:"explorer"`
			Source        string   `json:"source"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}
	if err := seedList(db, "homepage", "Homepage List", "Curated homepage token list from data/tokenlists/out/homepage.json.", "homepage.json"); err != nil {
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
			Source:       defaultString(token.Source, "seed-homepage"),
		}
		if err := seedListItem(db, "homepage", s.enrichChainContext(managed), token.Slot, i+1, true, token.DisplayName, token.DisplaySymbol, "seed:"+s.homepageSeedPath); err != nil {
			return err
		}
	}
	return nil
}

func seedList(db *sql.DB, key, name, description, outputPath string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		insert into lists(key, name, description, output_path, enabled, created_at, updated_at)
		values(?, ?, ?, ?, 1, ?, ?)
		on conflict(key) do nothing
	`, normalizeListKey(key), name, description, outputPath, now, now)
	return err
}

func seedListItem(db *sql.DB, listKey string, token ManagedToken, slot string, rank int, display bool, displayName, displaySymbol, note string) error {
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
	if token.Source == "" {
		token.Source = "seed"
	}
	token.Tags = appendUniqueStrings(nil, token.Tags...)
	tagsJSON, err := json.Marshal(token.Tags)
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
		insert into tokens(kind, chain, chain_name, chain_id, chain_logo_uri, address, asset_id, type, name, symbol, decimals, status, logo_uri, logo_exists, explorer, tags_json, source, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
			source = excluded.source,
			updated_at = excluded.updated_at
	`, token.Kind, token.Chain, token.ChainName, token.ChainID, token.ChainLogoURI, token.Address, token.AssetID, token.Type, token.Name, token.Symbol, token.Decimals, token.Status, token.LogoURI, boolToInt(token.LogoExists), token.Explorer, string(tagsJSON), token.Source, now, now)
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
		insert into list_items(list_id, token_id, slot, rank, enabled, display, display_name, display_symbol, note, created_at, updated_at)
		values(?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?)
		on conflict(list_id, token_id) do nothing
	`, listID, tokenID, normalizeSlot(slot), rank, boolToInt(display), displayName, displaySymbol, note, now, now)
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
		Tags:       appendUniqueStrings(nil, token.Tags...),
		Source:     "seed-tokenlist",
	}
}

func matchSymbol(symbol string) func(ManagedToken) bool {
	want := strings.ToUpper(symbol)
	return func(token ManagedToken) bool {
		return strings.ToUpper(token.Symbol) == want
	}
}
