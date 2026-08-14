package rpcserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	_ "modernc.org/sqlite"
)

type ManagedListService struct {
	dbPath            string
	filesDir          string
	tokenListSeedPath string
	manualTokensPath  string
	homepageSeedPath  string
	publicBaseURL     string
	store             *Store
}

type ManagedList struct {
	Key           string `json:"key"`
	Name          string `json:"name,omitempty"`
	Description   string `json:"description,omitempty"`
	DisplayName   string `json:"displayName,omitempty"`
	DisplaySymbol string `json:"displaySymbol,omitempty"`
	LogoURI       string `json:"logoURI,omitempty"`
	OutputPath    string `json:"outputPath,omitempty"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

type ManagedToken struct {
	Kind         string              `json:"kind"`
	Chain        string              `json:"chain"`
	ChainName    string              `json:"chainName,omitempty"`
	ChainID      int                 `json:"chainId,omitempty"`
	ChainLogoURI string              `json:"chainLogoURI,omitempty"`
	Address      string              `json:"address"`
	AssetID      string              `json:"assetId,omitempty"`
	Type         string              `json:"type,omitempty"`
	Name         string              `json:"name,omitempty"`
	Symbol       string              `json:"symbol,omitempty"`
	Decimals     int                 `json:"decimals"`
	Status       string              `json:"status,omitempty"`
	LogoURI      string              `json:"logoURI,omitempty"`
	LogoExists   bool                `json:"logoExists"`
	Explorer     string              `json:"explorer,omitempty"`
	Tags         []string            `json:"tags,omitempty"`
	Hot          bool                `json:"hot"`
	Market       *ManagedTokenMarket `json:"market,omitempty"`
	Pairs        []TokenPair         `json:"pairs,omitempty"`
	Links        []Link              `json:"links,omitempty"`
}

type ManagedTokenMarket struct {
	CoinGeckoID   string  `json:"coingeckoId,omitempty"`
	MarketCapRank int     `json:"marketCapRank,omitempty"`
	MarketCap     float64 `json:"marketCap,omitempty"`
	TotalVolume   float64 `json:"totalVolume,omitempty"`
	CurrentPrice  float64 `json:"currentPrice,omitempty"`
	LastUpdated   string  `json:"lastUpdated,omitempty"`
}

type ManagedListItem struct {
	Token          ManagedToken `json:"token"`
	Slot           string       `json:"slot,omitempty"`
	Rank           int          `json:"rank,omitempty"`
	Enabled        bool         `json:"enabled"`
	Display        bool         `json:"display"`
	DisplayName    string       `json:"displayName,omitempty"`
	DisplaySymbol  string       `json:"displaySymbol,omitempty"`
	DisplayLogoURI string       `json:"displayLogoURI,omitempty"`
	CreatedAt      string       `json:"createdAt,omitempty"`
	UpdatedAt      string       `json:"updatedAt,omitempty"`
}

// ManagedListInclude links a list to another managed list. Tag is both the
// source list key and the slot assigned to the source items when they are
// expanded into the target's packed output.
type ManagedListInclude struct {
	Tag       string `json:"tag"`
	Rank      int    `json:"rank,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ManagedListDocument is the complete management view of a list. Items remain
// independently editable, but readers do not need a second request to assemble
// the list.
type ManagedListDocument struct {
	ManagedList
	Includes []ManagedListInclude `json:"includes"`
	Items    []ManagedListItem    `json:"items"`
}

// ManagedSupportEntry is an exchange or wallet shown by clients. Rank,
// enabled and timestamps are management-only fields and are omitted from the
// published support.json document.
type ManagedSupportEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	LogoURI   string `json:"logoURI"`
	Rank      int    `json:"rank,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

type ManagedSupportDocument struct {
	ManagedList
	SchemaVersion int                   `json:"schemaVersion"`
	AssetBaseURI  string                `json:"assetBaseURI"`
	Exchanges     []ManagedSupportEntry `json:"exchanges"`
	Wallets       []ManagedSupportEntry `json:"wallets"`
}

type PublishedSupportEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type,omitempty"`
	LogoURI string `json:"logoURI"`
}

type PublishedSupportDocument struct {
	SchemaVersion int                     `json:"schemaVersion"`
	AssetBaseURI  string                  `json:"assetBaseURI"`
	Exchanges     []PublishedSupportEntry `json:"exchanges"`
	Wallets       []PublishedSupportEntry `json:"wallets"`
}

type ManagedListOutput struct {
	Key           string             `json:"key"`
	Name          string             `json:"name,omitempty"`
	Description   string             `json:"description,omitempty"`
	DisplayName   string             `json:"displayName,omitempty"`
	DisplaySymbol string             `json:"displaySymbol,omitempty"`
	LogoURI       string             `json:"logoURI,omitempty"`
	OutputPath    string             `json:"outputPath,omitempty"`
	Enabled       bool               `json:"enabled"`
	CreatedAt     string             `json:"createdAt,omitempty"`
	UpdatedAt     string             `json:"updatedAt,omitempty"`
	GeneratedAt   string             `json:"generatedAt"`
	Version       int                `json:"version"`
	Items         []ManagedListToken `json:"items"`
}

type ManagedListToken struct {
	ID            string              `json:"id,omitempty"`
	Slot          string              `json:"slot,omitempty"`
	Kind          string              `json:"kind"`
	Chain         string              `json:"chain"`
	ChainName     string              `json:"chainName,omitempty"`
	ChainID       int                 `json:"chainId,omitempty"`
	ChainLogoURI  string              `json:"chainLogoURI,omitempty"`
	Address       string              `json:"address"`
	AssetID       string              `json:"assetId,omitempty"`
	Type          string              `json:"type,omitempty"`
	Display       bool                `json:"display"`
	DisplayName   string              `json:"displayName,omitempty"`
	DisplaySymbol string              `json:"displaySymbol,omitempty"`
	Name          string              `json:"name,omitempty"`
	Symbol        string              `json:"symbol,omitempty"`
	Decimals      int                 `json:"decimals"`
	Status        string              `json:"status,omitempty"`
	LogoURI       string              `json:"logoURI,omitempty"`
	LogoExists    bool                `json:"logoExists"`
	Explorer      string              `json:"explorer,omitempty"`
	Rank          int                 `json:"rank,omitempty"`
	Tags          []string            `json:"tags,omitempty"`
	Hot           bool                `json:"hot"`
	Market        *ManagedTokenMarket `json:"market,omitempty"`
	Pairs         []TokenPair         `json:"pairs,omitempty"`
	Links         []Link              `json:"links,omitempty"`
}

type PackFile struct {
	ListKey     string `json:"listKey"`
	JSONPath    string `json:"jsonPath"`
	ZstdPath    string `json:"zstdPath"`
	JSONURL     string `json:"jsonUrl"`
	ZstdURL     string `json:"zstdUrl"`
	JSONSize    int64  `json:"jsonSize"`
	ZstdSize    int64  `json:"zstdSize"`
	JSONSHA256  string `json:"jsonSha256"`
	ZstdSHA256  string `json:"zstdSha256"`
	TokenCount  int    `json:"tokenCount"`
	GeneratedAt string `json:"generatedAt"`
}

type PackManifest struct {
	GeneratedAt string     `json:"generatedAt"`
	Files       []PackFile `json:"files"`
}

func NewManagedListService(dbPath, filesDir, tokenListSeedPath, manualTokensPath, homepageSeedPath, publicBaseURL string, store *Store) *ManagedListService {
	return &ManagedListService{
		dbPath:            dbPath,
		filesDir:          filesDir,
		tokenListSeedPath: tokenListSeedPath,
		manualTokensPath:  manualTokensPath,
		homepageSeedPath:  homepageSeedPath,
		publicBaseURL:     strings.TrimRight(strings.TrimSpace(publicBaseURL), "/"),
		store:             store,
	}
}

func (s *ManagedListService) Init() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return initializeManagedListSchema(db)
}

func (s *ManagedListService) SeedDefaultLists() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()

	if err := s.seedFromAppTokenList(db); err != nil {
		return err
	}
	if err := s.seedFromManualTokenList(db); err != nil {
		return err
	}
	if err := s.seedFromHomepage(db); err != nil {
		return err
	}
	if err := seedHomepageIncludes(db); err != nil {
		return err
	}
	return s.seedSupportList(db)
}

func (s *ManagedListService) ListLists() ([]ManagedList, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`select key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at from lists order by key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	lists := []ManagedList{}
	for rows.Next() {
		list, err := scanManagedList(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, list)
	}
	return lists, rows.Err()
}

func (s *ManagedListService) GetList(key string) (*ManagedList, error) {
	key = normalizeListKey(key)
	if key == "" {
		return nil, invalidParams("list key is required")
	}

	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	row := db.QueryRow(`select key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at from lists where key = ?`, key)
	list, err := scanManagedList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list not found")
	}
	return &list, err
}

func (s *ManagedListService) UpsertList(input ManagedList) (*ManagedList, error) {
	input.Key = normalizeListKey(input.Key)
	if input.Key == "" {
		return nil, invalidParams("list key is required")
	}
	if !validListKey(input.Key) {
		return nil, invalidParams("list key may contain only lowercase letters, numbers, '.', '_' and '-'")
	}
	if input.Name == "" {
		input.Name = input.Key
	}
	if input.OutputPath == "" {
		input.OutputPath = input.Key + ".json"
	}
	outputPath, err := safePackOutputPath(input.Key, input.OutputPath)
	if err != nil {
		return nil, err
	}
	input.OutputPath = filepath.ToSlash(outputPath)
	now := time.Now().UTC().Format(time.RFC3339)

	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var conflictingKey string
	if err := db.QueryRow(`select key from lists where lower(output_path) = lower(?) and key <> ? limit 1`, input.OutputPath, input.Key).Scan(&conflictingKey); err == nil {
		return nil, conflict("outputPath is already used by list " + conflictingKey)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var previous *ManagedList
	if current, scanErr := scanManagedList(db.QueryRow(`select key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at from lists where key = ?`, input.Key)); scanErr == nil {
		previous = &current
	} else if !errors.Is(scanErr, sql.ErrNoRows) {
		return nil, scanErr
	}

	_, err = db.Exec(`
		insert into lists(key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(key) do update set
			name = excluded.name,
			description = excluded.description,
			display_name = excluded.display_name,
			display_symbol = excluded.display_symbol,
			logo_uri = excluded.logo_uri,
			output_path = excluded.output_path,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, input.Key, input.Name, input.Description, input.DisplayName, input.DisplaySymbol, input.LogoURI, input.OutputPath, boolToInt(input.Enabled), now, now)
	if err != nil {
		return nil, err
	}
	if previous != nil && (!input.Enabled || previous.OutputPath != input.OutputPath) {
		if err := s.prunePackedArtifacts(previous.Key, previous.OutputPath); err != nil {
			return nil, err
		}
	}
	return s.GetList(input.Key)
}

func (s *ManagedListService) DeleteList(key string) error {
	key = normalizeListKey(key)
	if key == "" {
		return invalidParams("list key is required")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	row := db.QueryRow(`select key, name, description, display_name, display_symbol, logo_uri, output_path, enabled, created_at, updated_at from lists where key = ?`, key)
	list, err := scanManagedList(row)
	if errors.Is(err, sql.ErrNoRows) {
		return notFound("list not found")
	}
	if err != nil {
		return err
	}
	result, err := db.Exec(`delete from lists where key = ?`, key)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return notFound("list not found")
	}
	return s.prunePackedArtifacts(list.Key, list.OutputPath)
}

func (s *ManagedListService) ListItems(listKey string) ([]ManagedListItem, error) {
	listKey = normalizeListKey(listKey)
	if listKey == "" {
		return nil, invalidParams("list key is required")
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var exists int
	if err := db.QueryRow(`select 1 from lists where key = ?`, listKey).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list not found")
	} else if err != nil {
		return nil, err
	}

	rows, err := db.Query(`
		select t.kind, t.chain, t.chain_name, t.chain_id, t.chain_logo_uri, t.address, t.asset_id, t.type,
			t.name, t.symbol, t.decimals, t.status, t.logo_uri, t.logo_exists, t.explorer, t.tags_json,
			t.hot, t.market_json, t.pairs_json, t.links_json,
			li.slot, li.rank, li.enabled, li.display, li.display_name, li.display_symbol, li.display_logo_uri, li.created_at, li.updated_at
		from list_items li
		join lists l on l.id = li.list_id
		join tokens t on t.id = li.token_id
		where l.key = ?
		order by li.rank asc, t.chain asc, t.symbol asc, t.address asc
	`, listKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []ManagedListItem{}
	for rows.Next() {
		item, err := scanManagedListItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ManagedListService) GetListDocument(listKey string) (*ManagedListDocument, error) {
	list, err := s.GetList(listKey)
	if err != nil {
		return nil, err
	}
	items, err := s.ListItems(list.Key)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []ManagedListItem{}
	}
	includes, err := s.ListIncludes(list.Key)
	if err != nil {
		return nil, err
	}
	if includes == nil {
		includes = []ManagedListInclude{}
	}
	return &ManagedListDocument{ManagedList: *list, Includes: includes, Items: items}, nil
}

func (s *ManagedListService) ListIncludes(listKey string) ([]ManagedListInclude, error) {
	listKey = normalizeListKey(listKey)
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var exists int
	if err := db.QueryRow(`select 1 from lists where key = ?`, listKey).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list not found")
	} else if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		select source.key, li.rank, li.enabled, li.created_at, li.updated_at
		from list_includes li
		join lists target on target.id = li.list_id
		join lists source on source.id = li.source_list_id
		where target.key = ?
		order by li.rank asc, source.key asc
	`, listKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	includes := []ManagedListInclude{}
	for rows.Next() {
		include, err := scanManagedListInclude(rows)
		if err != nil {
			return nil, err
		}
		includes = append(includes, include)
	}
	return includes, rows.Err()
}

func (s *ManagedListService) GetInclude(listKey, tag string) (*ManagedListInclude, error) {
	listKey, tag = normalizeListKey(listKey), normalizeListKey(tag)
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	include, err := scanManagedListInclude(db.QueryRow(`
		select source.key, li.rank, li.enabled, li.created_at, li.updated_at
		from list_includes li
		join lists target on target.id = li.list_id
		join lists source on source.id = li.source_list_id
		where target.key = ? and source.key = ?
	`, listKey, tag))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list include not found")
	}
	return &include, err
}

func (s *ManagedListService) SaveInclude(listKey string, input ManagedListInclude) (*ManagedListInclude, error) {
	listKey, input.Tag = normalizeListKey(listKey), normalizeListKey(input.Tag)
	if !validListKey(listKey) || !validListKey(input.Tag) {
		return nil, invalidParams("list key and include tag must be valid list keys")
	}
	if listKey == input.Tag {
		return nil, invalidParams("a list cannot include itself")
	}
	if input.Rank < 0 {
		return nil, invalidParams("rank must be greater than or equal to zero")
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var targetID, sourceID int64
	if err := tx.QueryRow(`select id from lists where key = ?`, listKey).Scan(&targetID); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list not found")
	} else if err != nil {
		return nil, err
	}
	if err := tx.QueryRow(`select id from lists where key = ?`, input.Tag).Scan(&sourceID); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("included list not found")
	} else if err != nil {
		return nil, err
	}
	var cycle int
	err = tx.QueryRow(`
		with recursive reachable(id) as (
			select source_list_id from list_includes where list_id = ?
			union
			select li.source_list_id from list_includes li join reachable r on li.list_id = r.id
		)
		select 1 from reachable where id = ? limit 1
	`, sourceID, targetID).Scan(&cycle)
	if err == nil {
		return nil, conflict("list include would create a cycle")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(`
		insert into list_includes(list_id, source_list_id, rank, enabled, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?)
		on conflict(list_id, source_list_id) do update set
			rank = excluded.rank,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, targetID, sourceID, input.Rank, boolToInt(input.Enabled), now, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetInclude(listKey, input.Tag)
}

func (s *ManagedListService) DeleteInclude(listKey, tag string) error {
	listKey, tag = normalizeListKey(listKey), normalizeListKey(tag)
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`
		delete from list_includes
		where list_id = (select id from lists where key = ?)
		  and source_list_id = (select id from lists where key = ?)
	`, listKey, tag)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return notFound("list include not found")
	}
	return err
}

func (s *ManagedListService) UpsertItem(listKey string, input ManagedListItem) (*ManagedListItem, error) {
	return s.saveItem(listKey, input, true)
}

// SaveItem persists the supplied token metadata as-is. It is used by PUT and
// PATCH after the API layer has merged and validated the requested changes.
func (s *ManagedListService) SaveItem(listKey string, input ManagedListItem) (*ManagedListItem, error) {
	return s.saveItem(listKey, input, false)
}

func (s *ManagedListService) saveItem(listKey string, input ManagedListItem, hydrate bool) (*ManagedListItem, error) {
	listKey = normalizeListKey(listKey)
	if listKey == "" {
		return nil, invalidParams("list key is required")
	}
	if input.Rank < 0 {
		return nil, invalidParams("rank must be greater than or equal to zero")
	}
	var token ManagedToken
	var err error
	if hydrate {
		token, err = s.resolveManagedToken(input.Token)
	} else {
		token, err = validateManagedToken(input.Token)
		if err == nil {
			token = s.enrichChainContext(token)
		}
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)

	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var listID int64
	if err := tx.QueryRow(`select id from lists where key = ?`, listKey).Scan(&listID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, notFound("list not found")
		}
		return nil, err
	}

	tagsJSON, err := json.Marshal(token.Tags)
	if err != nil {
		return nil, err
	}
	marketJSON, err := json.Marshal(token.Market)
	if err != nil {
		return nil, err
	}
	pairsJSON, err := json.Marshal(token.Pairs)
	if err != nil {
		return nil, err
	}
	linksJSON, err := json.Marshal(token.Links)
	if err != nil {
		return nil, err
	}
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
		return nil, err
	}

	var tokenID int64
	if err := tx.QueryRow(`select id from tokens where chain = ? and address = ?`, token.Chain, token.Address).Scan(&tokenID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(`
		insert into list_items(list_id, token_id, slot, rank, enabled, display, display_name, display_symbol, display_logo_uri, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(list_id, token_id) do update set
			slot = excluded.slot,
			rank = excluded.rank,
			enabled = excluded.enabled,
			display = excluded.display,
			display_name = excluded.display_name,
			display_symbol = excluded.display_symbol,
			display_logo_uri = excluded.display_logo_uri,
			updated_at = excluded.updated_at
	`, listID, tokenID, normalizeSlot(input.Slot), input.Rank, boolToInt(input.Enabled), boolToInt(input.Display), input.DisplayName, input.DisplaySymbol, input.DisplayLogoURI, now, now)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetItem(listKey, token.Chain, token.Address)
}

func (s *ManagedListService) GetItem(listKey, chain, address string) (*ManagedListItem, error) {
	listKey = normalizeListKey(listKey)
	chain = normalizeChain(chain)
	address = strings.TrimSpace(address)
	if listKey == "" || chain == "" {
		return nil, invalidParams("list key and chain are required")
	}

	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	row := db.QueryRow(`
		select t.kind, t.chain, t.chain_name, t.chain_id, t.chain_logo_uri, t.address, t.asset_id, t.type,
			t.name, t.symbol, t.decimals, t.status, t.logo_uri, t.logo_exists, t.explorer, t.tags_json,
			t.hot, t.market_json, t.pairs_json, t.links_json,
			li.slot, li.rank, li.enabled, li.display, li.display_name, li.display_symbol, li.display_logo_uri, li.created_at, li.updated_at
		from list_items li
		join lists l on l.id = li.list_id
		join tokens t on t.id = li.token_id
		where l.key = ? and t.chain = ? and lower(t.address) = lower(?)
	`, listKey, chain, address)
	item, err := scanManagedListItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("list item not found")
	}
	return &item, err
}

func (s *ManagedListService) DeleteItem(listKey, chain, address string) error {
	listKey = normalizeListKey(listKey)
	chain = normalizeChain(chain)
	address = strings.TrimSpace(address)
	if listKey == "" || chain == "" {
		return invalidParams("list key and chain are required")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`
		delete from list_items
		where id in (
			select li.id
			from list_items li
			join lists l on l.id = li.list_id
			join tokens t on t.id = li.token_id
			where l.key = ? and t.chain = ? and lower(t.address) = lower(?)
		)
	`, listKey, chain, address)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return notFound("list item not found")
	}
	return nil
}

func (s *ManagedListService) PackList(listKey string) (*PackFile, error) {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	if normalizeListKey(listKey) == "support" {
		return s.packSupportList(generatedAt)
	}
	output, err := s.buildExpandedManagedListOutput(listKey, generatedAt, map[string]bool{})
	if err != nil {
		return nil, err
	}
	return s.writePackedList(output, output.OutputPath)
}

func (s *ManagedListService) PackAll() (*PackManifest, error) {
	lists, err := s.ListLists()
	if err != nil {
		return nil, err
	}
	manifest := &PackManifest{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, list := range lists {
		if !list.Enabled {
			continue
		}
		var packed *PackFile
		if list.Key == "support" {
			packed, err = s.packSupportList(manifest.GeneratedAt)
		} else {
			var output ManagedListOutput
			output, err = s.buildExpandedManagedListOutput(list.Key, manifest.GeneratedAt, map[string]bool{})
			if err == nil {
				packed, err = s.writePackedList(output, list.OutputPath)
			}
		}
		if err != nil {
			return nil, err
		}
		manifest.Files = append(manifest.Files, *packed)
	}
	if err := writeJSONAtomic(filepath.Join(s.filesDir, "manifest.json"), manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func (s *ManagedListService) open() (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(s.dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	for _, pragma := range []string{
		`pragma busy_timeout = 5000`,
		`pragma journal_mode = wal`,
		`pragma foreign_keys = on`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := initializeManagedListSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func (s *ManagedListService) resolveManagedToken(input ManagedToken) (ManagedToken, error) {
	input.Chain = normalizeChain(input.Chain)
	input.Address = strings.TrimSpace(input.Address)
	if input.Chain == "" {
		return ManagedToken{}, invalidParams("token chain is required")
	}
	if input.Address == "" && strings.ToLower(input.Kind) != "native" {
		return ManagedToken{}, invalidParams("token address is required unless kind is native")
	}
	if input.Address != "" {
		if detail, err := s.store.GetAssetByAddress(input.Chain, input.Address); err == nil {
			return s.enrichChainContext(mergeManagedTokenUI(managedTokenFromAsset(*detail), input)), nil
		}
	}
	if input.Kind == "native" {
		if detail, err := s.store.readNativeAssetDetail(input.Chain, filepath.Join(s.store.root, "blockchains", input.Chain, "info")); err == nil {
			return s.enrichChainContext(mergeManagedTokenUI(managedTokenFromAsset(*detail), input)), nil
		}
	}
	input, err := validateManagedToken(input)
	if err != nil {
		return ManagedToken{}, err
	}
	return s.enrichChainContext(input), nil
}

func mergeManagedTokenUI(base, input ManagedToken) ManagedToken {
	base.Hot = input.Hot
	base.Market = input.Market
	base.Pairs = append([]TokenPair(nil), input.Pairs...)
	base.Links = append([]Link(nil), input.Links...)
	if base.Explorer == "" {
		base.Explorer = input.Explorer
	}
	return base
}

func (s *ManagedListService) enrichChainContext(token ManagedToken) ManagedToken {
	if token.Chain == "" || s.store == nil {
		return token
	}
	if normalizeChain(token.Chain) == "polygon" {
		token.ChainLogoURI = DefaultPolygonLogoURI
	}
	info, err := s.store.GetChainInfo(token.Chain)
	if err != nil {
		return token
	}
	if token.ChainName == "" {
		if value, ok := info["name"].(string); ok {
			token.ChainName = value
		}
	}
	if token.ChainLogoURI == "" {
		if value, ok := info["logoURI"].(string); ok {
			token.ChainLogoURI = value
		}
	}
	if token.ChainID == 0 {
		switch value := info["chainId"].(type) {
		case float64:
			token.ChainID = int(value)
		case int:
			token.ChainID = value
		}
	}
	return token
}

func (s *ManagedListService) writePackedList(output ManagedListOutput, outputPath string) (*PackFile, error) {
	return s.writePackedDocument(output.Key, outputPath, output, len(output.Items), output.GeneratedAt)
}

func (s *ManagedListService) writePackedDocument(listKey, outputPath string, document any, itemCount int, generatedAt string) (*PackFile, error) {
	if err := os.MkdirAll(s.filesDir, 0o755); err != nil {
		return nil, err
	}
	relativePath, err := safePackOutputPath(listKey, outputPath)
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(s.filesDir, relativePath)
	zstdPath := jsonPath + ".zst"
	if err := writeJSONAtomic(jsonPath, document); err != nil {
		return nil, err
	}
	jsonBytes, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return nil, err
	}
	zstdBytes := encoder.EncodeAll(jsonBytes, nil)
	encoder.Close()
	if err := writeBytesAtomic(zstdPath, zstdBytes); err != nil {
		return nil, err
	}
	jsonInfo, err := os.Stat(jsonPath)
	if err != nil {
		return nil, err
	}
	zstdInfo, err := os.Stat(zstdPath)
	if err != nil {
		return nil, err
	}
	relativePath = filepath.ToSlash(relativePath)
	publicBaseURL := s.publicBaseURL
	if publicBaseURL == "" {
		publicBaseURL = DefaultManagedListPublicBaseURL
	}
	jsonURL := strings.TrimRight(publicBaseURL, "/") + "/" + relativePath
	return &PackFile{
		ListKey:     listKey,
		JSONPath:    relativePath,
		ZstdPath:    relativePath + ".zst",
		JSONURL:     jsonURL,
		ZstdURL:     jsonURL + ".zst",
		JSONSize:    jsonInfo.Size(),
		ZstdSize:    zstdInfo.Size(),
		JSONSHA256:  sha256Hex(jsonBytes),
		ZstdSHA256:  sha256Hex(zstdBytes),
		TokenCount:  itemCount,
		GeneratedAt: generatedAt,
	}, nil
}

func (s *ManagedListService) prunePackedArtifacts(listKey, outputPath string) error {
	relativePath, err := safePackOutputPath(listKey, outputPath)
	if err != nil {
		return err
	}
	for _, path := range []string{
		filepath.Join(s.filesDir, relativePath),
		filepath.Join(s.filesDir, relativePath) + ".zst",
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	manifestPath := filepath.Join(s.filesDir, "manifest.json")
	var manifest PackManifest
	if err := readJSONFile(manifestPath, &manifest); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	files := manifest.Files[:0]
	for _, file := range manifest.Files {
		if file.ListKey != listKey {
			files = append(files, file)
		}
	}
	if len(files) == len(manifest.Files) {
		return nil
	}
	manifest.Files = files
	manifest.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONAtomic(manifestPath, manifest)
}

func safePackOutputPath(listKey, outputPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		outputPath = normalizeListKey(listKey) + ".json"
	}
	if strings.ContainsAny(outputPath, "\\:\x00") {
		return "", invalidParams("outputPath must be a portable relative path using '/' separators")
	}
	outputPath = path.Clean(outputPath)
	if strings.HasPrefix(outputPath, "/") || outputPath == "." || strings.HasPrefix(outputPath, "../") || outputPath == ".." {
		return "", invalidParams("outputPath must stay inside managed list files directory")
	}
	if !strings.HasSuffix(outputPath, ".json") {
		outputPath += ".json"
	}
	return filepath.FromSlash(outputPath), nil
}

func writeBytesAtomic(filePath string, data []byte) error {
	directory := filepath.Dir(filePath)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".tmp-*.zst")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, filePath)
}

func initializeManagedListSchema(db *sql.DB) error {
	stmts := []string{
		`create table if not exists lists (
			id integer primary key autoincrement,
			key text not null unique,
			name text not null default '',
			description text not null default '',
			display_name text not null default '',
			display_symbol text not null default '',
			logo_uri text not null default '',
			output_path text not null default '',
			enabled integer not null default 1,
			created_at text not null,
			updated_at text not null
		)`,
		`create table if not exists list_includes (
			id integer primary key autoincrement,
			list_id integer not null references lists(id) on delete cascade,
			source_list_id integer not null references lists(id) on delete cascade,
			rank integer not null default 0,
			enabled integer not null default 1,
			created_at text not null,
			updated_at text not null,
			unique(list_id, source_list_id)
		)`,
		`create table if not exists tokens (
			id integer primary key autoincrement,
			kind text not null default 'token',
			chain text not null,
			chain_name text not null default '',
			chain_id integer not null default 0,
			chain_logo_uri text not null default '',
			address text not null,
			asset_id text not null default '',
			type text not null default '',
			name text not null default '',
			symbol text not null default '',
			decimals integer not null default 0,
			status text not null default '',
			logo_uri text not null default '',
			logo_exists integer not null default 0,
			explorer text not null default '',
			tags_json text not null default '[]',
			hot integer not null default 0,
			market_json text not null default 'null',
			pairs_json text not null default '[]',
			links_json text not null default '[]',
			created_at text not null,
			updated_at text not null,
			unique(chain, address)
		)`,
		`create table if not exists list_items (
			id integer primary key autoincrement,
			list_id integer not null references lists(id) on delete cascade,
			token_id integer not null references tokens(id) on delete cascade,
			slot text not null default '',
			rank integer not null default 0,
			enabled integer not null default 1,
			display integer not null default 1,
			display_name text not null default '',
			display_symbol text not null default '',
			display_logo_uri text not null default '',
			created_at text not null,
			updated_at text not null,
			unique(list_id, token_id)
		)`,
		`create table if not exists support_entries (
			id integer primary key autoincrement,
			list_id integer not null references lists(id) on delete cascade,
			category text not null check(category in ('exchanges', 'wallets')),
			entry_id text not null,
			name text not null,
			type text not null default '',
			logo_uri text not null,
			rank integer not null default 0,
			enabled integer not null default 1,
			created_at text not null,
			updated_at text not null,
			unique(list_id, category, entry_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func buildManagedListOutput(list ManagedList, items []ManagedListItem, generatedAt string) ManagedListOutput {
	packedItems := make([]ManagedListToken, 0, len(items))
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		token := item.Token
		name := token.Name
		if item.DisplayName != "" {
			name = item.DisplayName
		}
		symbol := token.Symbol
		if item.DisplaySymbol != "" {
			symbol = item.DisplaySymbol
		} else if list.DisplaySymbol != "" {
			symbol = list.DisplaySymbol
		}
		if item.DisplayName == "" && list.DisplayName != "" {
			name = list.DisplayName
		}
		logoURI := token.LogoURI
		if list.LogoURI != "" {
			logoURI = list.LogoURI
		}
		if item.DisplayLogoURI != "" {
			logoURI = item.DisplayLogoURI
		}
		packedItems = append(packedItems, ManagedListToken{
			ID:            managedListTokenID(list.Key, item.Slot, token),
			Slot:          item.Slot,
			Kind:          token.Kind,
			Chain:         token.Chain,
			ChainName:     token.ChainName,
			ChainID:       token.ChainID,
			ChainLogoURI:  token.ChainLogoURI,
			Address:       token.Address,
			AssetID:       token.AssetID,
			Type:          token.Type,
			Display:       item.Display,
			DisplayName:   name,
			DisplaySymbol: symbol,
			Name:          token.Name,
			Symbol:        token.Symbol,
			Decimals:      token.Decimals,
			Status:        token.Status,
			LogoURI:       logoURI,
			LogoExists:    logoURI != "",
			Explorer:      token.Explorer,
			Rank:          item.Rank,
			Tags:          token.Tags,
			Hot:           token.Hot,
			Market:        token.Market,
			Pairs:         token.Pairs,
			Links:         token.Links,
		})
	}
	return ManagedListOutput{
		Key:           list.Key,
		Name:          list.Name,
		Description:   list.Description,
		DisplayName:   list.DisplayName,
		DisplaySymbol: list.DisplaySymbol,
		LogoURI:       list.LogoURI,
		OutputPath:    list.OutputPath,
		Enabled:       list.Enabled,
		CreatedAt:     list.CreatedAt,
		UpdatedAt:     list.UpdatedAt,
		GeneratedAt:   generatedAt,
		Version:       1,
		Items:         packedItems,
	}
}

func (s *ManagedListService) buildExpandedManagedListOutput(listKey, generatedAt string, visiting map[string]bool) (ManagedListOutput, error) {
	listKey = normalizeListKey(listKey)
	if visiting[listKey] {
		return ManagedListOutput{}, conflict("list include cycle detected")
	}
	visiting[listKey] = true
	defer delete(visiting, listKey)

	list, err := s.GetList(listKey)
	if err != nil {
		return ManagedListOutput{}, err
	}
	items, err := s.ListItems(listKey)
	if err != nil {
		return ManagedListOutput{}, err
	}
	output := buildManagedListOutput(*list, items, generatedAt)
	includes, err := s.ListIncludes(listKey)
	if err != nil {
		return ManagedListOutput{}, err
	}
	if len(includes) == 0 {
		return output, nil
	}

	included := make([]ManagedListToken, 0)
	includedIdentities := map[string]struct{}{}
	for _, include := range includes {
		if !include.Enabled {
			continue
		}
		source, err := s.buildExpandedManagedListOutput(include.Tag, generatedAt, visiting)
		if err != nil {
			return ManagedListOutput{}, err
		}
		if !source.Enabled {
			continue
		}
		for _, token := range source.Items {
			identity := managedListTokenIdentity(token)
			if _, exists := includedIdentities[identity]; exists {
				continue
			}
			includedIdentities[identity] = struct{}{}
			token.Slot = include.Tag
			if include.Rank > 0 {
				token.Rank = include.Rank
			}
			included = append(included, token)
		}
	}

	combined := make([]ManagedListToken, 0, len(output.Items)+len(included))
	for _, token := range output.Items {
		if _, replaced := includedIdentities[managedListTokenIdentity(token)]; replaced {
			continue
		}
		combined = append(combined, token)
	}
	combined = append(combined, included...)
	sort.SliceStable(combined, func(i, j int) bool {
		if combined[i].Rank != combined[j].Rank {
			return combined[i].Rank < combined[j].Rank
		}
		if combined[i].Slot != combined[j].Slot {
			return combined[i].Slot < combined[j].Slot
		}
		if combined[i].Chain != combined[j].Chain {
			return combined[i].Chain < combined[j].Chain
		}
		return strings.ToLower(combined[i].Address) < strings.ToLower(combined[j].Address)
	})
	output.Items = combined
	return output, nil
}

func managedListTokenIdentity(token ManagedListToken) string {
	address := strings.TrimSpace(token.Address)
	if strings.HasPrefix(strings.ToLower(address), "0x") {
		address = strings.ToLower(address)
	}
	return normalizeChain(token.Chain) + "\x00" + address
}

func managedTokenFromAsset(asset AssetDetail) ManagedToken {
	return ManagedToken{
		Kind:       defaultString(assetKind(asset), "token"),
		Chain:      asset.Chain,
		Address:    asset.Address,
		AssetID:    asset.AssetID,
		Type:       asset.Type,
		Name:       asset.Name,
		Symbol:     asset.Symbol,
		Decimals:   asset.Decimals,
		Status:     asset.Status,
		LogoURI:    asset.LogoURI,
		LogoExists: asset.LogoExists,
		Explorer:   asset.Explorer,
		Tags:       appendUniqueStrings(nil, asset.Tags...),
		Links:      append([]Link(nil), asset.Links...),
	}
}

func assetKind(asset AssetDetail) string {
	if asset.Address == "" {
		return "native"
	}
	return "token"
}

type managedListScanner interface {
	Scan(dest ...any) error
}

func scanManagedList(scanner managedListScanner) (ManagedList, error) {
	var list ManagedList
	var enabled int
	err := scanner.Scan(&list.Key, &list.Name, &list.Description, &list.DisplayName, &list.DisplaySymbol, &list.LogoURI, &list.OutputPath, &enabled, &list.CreatedAt, &list.UpdatedAt)
	list.Enabled = enabled != 0
	return list, err
}

func scanManagedListItem(scanner managedListScanner) (ManagedListItem, error) {
	var item ManagedListItem
	var tagsJSON, marketJSON, pairsJSON, linksJSON string
	var logoExists, hot, enabled, display int
	err := scanner.Scan(
		&item.Token.Kind, &item.Token.Chain, &item.Token.ChainName, &item.Token.ChainID, &item.Token.ChainLogoURI,
		&item.Token.Address, &item.Token.AssetID, &item.Token.Type, &item.Token.Name, &item.Token.Symbol,
		&item.Token.Decimals, &item.Token.Status, &item.Token.LogoURI, &logoExists, &item.Token.Explorer,
		&tagsJSON, &hot, &marketJSON, &pairsJSON, &linksJSON, &item.Slot, &item.Rank, &enabled, &display, &item.DisplayName,
		&item.DisplaySymbol, &item.DisplayLogoURI, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.Token.LogoExists = logoExists != 0
	item.Token.Hot = hot != 0
	item.Enabled = enabled != 0
	item.Display = display != 0
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &item.Token.Tags)
	}
	if marketJSON != "" && marketJSON != "null" {
		_ = json.Unmarshal([]byte(marketJSON), &item.Token.Market)
	}
	if pairsJSON != "" {
		_ = json.Unmarshal([]byte(pairsJSON), &item.Token.Pairs)
	}
	if linksJSON != "" {
		_ = json.Unmarshal([]byte(linksJSON), &item.Token.Links)
	}
	return item, nil
}

func scanManagedListInclude(scanner managedListScanner) (ManagedListInclude, error) {
	var include ManagedListInclude
	var enabled int
	err := scanner.Scan(&include.Tag, &include.Rank, &enabled, &include.CreatedAt, &include.UpdatedAt)
	include.Enabled = enabled != 0
	return include, err
}

func normalizeListKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, " ", "-")
	return key
}

func validListKey(key string) bool {
	if key == "" {
		return false
	}
	for _, char := range key {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '_' || char == '-' {
			continue
		}
		return false
	}
	return true
}

func validateManagedToken(token ManagedToken) (ManagedToken, error) {
	token.Chain = normalizeChain(token.Chain)
	token.Address = strings.TrimSpace(token.Address)
	token.Kind = strings.ToLower(strings.TrimSpace(token.Kind))
	if token.Chain == "" {
		return ManagedToken{}, invalidParams("token chain is required")
	}
	if token.Kind == "" {
		if token.Address == "" {
			token.Kind = "native"
		} else {
			token.Kind = "token"
		}
	}
	if token.Kind != "native" && token.Kind != "token" {
		return ManagedToken{}, invalidParams("token kind must be 'native' or 'token'")
	}
	if token.Kind == "native" && token.Address != "" {
		return ManagedToken{}, invalidParams("native token address must be empty")
	}
	if token.Kind == "token" && token.Address == "" {
		return ManagedToken{}, invalidParams("token address is required")
	}
	if token.Decimals < 0 || token.Decimals > 255 {
		return ManagedToken{}, invalidParams("token decimals must be between 0 and 255")
	}
	token.Tags = appendUniqueStrings(nil, token.Tags...)
	return token, nil
}

func normalizeChain(chain string) string {
	return strings.ToLower(strings.TrimSpace(chain))
}

func normalizeSlot(slot string) string {
	return strings.ToLower(strings.TrimSpace(slot))
}

func managedListTokenID(listKey, slot string, token ManagedToken) string {
	slot = normalizeSlot(slot)
	if slot != "" {
		return token.Chain + ":" + slot
	}
	if token.AssetID != "" {
		return token.AssetID
	}
	if token.Address == "" {
		return token.Chain
	}
	return token.Chain + ":" + token.Address
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortPackFiles(files []PackFile) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].ListKey < files[j].ListKey
	})
}

func sqliteError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
