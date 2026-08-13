package rpcserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
	_ "github.com/mattn/go-sqlite3"
)

type ManagedListService struct {
	dbPath            string
	filesDir          string
	tokenListSeedPath string
	homepageSeedPath  string
	store             *Store
}

type ManagedList struct {
	Key         string `json:"key"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	OutputPath  string `json:"outputPath,omitempty"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

type ManagedToken struct {
	Kind         string   `json:"kind"`
	Chain        string   `json:"chain"`
	ChainName    string   `json:"chainName,omitempty"`
	ChainID      int      `json:"chainId,omitempty"`
	ChainLogoURI string   `json:"chainLogoURI,omitempty"`
	Address      string   `json:"address"`
	AssetID      string   `json:"assetId,omitempty"`
	Type         string   `json:"type,omitempty"`
	Name         string   `json:"name,omitempty"`
	Symbol       string   `json:"symbol,omitempty"`
	Decimals     int      `json:"decimals"`
	Status       string   `json:"status,omitempty"`
	LogoURI      string   `json:"logoURI,omitempty"`
	LogoExists   bool     `json:"logoExists"`
	Explorer     string   `json:"explorer,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	Source       string   `json:"source,omitempty"`
}

type ManagedListItem struct {
	Token         ManagedToken `json:"token"`
	Slot          string       `json:"slot,omitempty"`
	Rank          int          `json:"rank,omitempty"`
	Enabled       bool         `json:"enabled"`
	Display       bool         `json:"display"`
	DisplayName   string       `json:"displayName,omitempty"`
	DisplaySymbol string       `json:"displaySymbol,omitempty"`
	Note          string       `json:"note,omitempty"`
	CreatedAt     string       `json:"createdAt,omitempty"`
	UpdatedAt     string       `json:"updatedAt,omitempty"`
}

type ManagedListOutput struct {
	Key         string             `json:"key"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	GeneratedAt string             `json:"generatedAt"`
	Version     int                `json:"version"`
	Tokens      []ManagedListToken `json:"tokens"`
}

type ManagedListToken struct {
	ID            string   `json:"id,omitempty"`
	Slot          string   `json:"slot,omitempty"`
	Kind          string   `json:"kind"`
	Chain         string   `json:"chain"`
	ChainName     string   `json:"chainName,omitempty"`
	ChainID       int      `json:"chainId,omitempty"`
	ChainLogoURI  string   `json:"chainLogoURI,omitempty"`
	Address       string   `json:"address"`
	AssetID       string   `json:"assetId,omitempty"`
	Type          string   `json:"type,omitempty"`
	Display       bool     `json:"display"`
	DisplayName   string   `json:"displayName,omitempty"`
	DisplaySymbol string   `json:"displaySymbol,omitempty"`
	Name          string   `json:"name,omitempty"`
	Symbol        string   `json:"symbol,omitempty"`
	Decimals      int      `json:"decimals"`
	Status        string   `json:"status,omitempty"`
	LogoURI       string   `json:"logoURI,omitempty"`
	LogoExists    bool     `json:"logoExists"`
	Explorer      string   `json:"explorer,omitempty"`
	Rank          int      `json:"rank,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	Source        string   `json:"source,omitempty"`
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

func NewManagedListService(dbPath, filesDir, tokenListSeedPath, homepageSeedPath string, store *Store) *ManagedListService {
	return &ManagedListService{
		dbPath:            dbPath,
		filesDir:          filesDir,
		tokenListSeedPath: tokenListSeedPath,
		homepageSeedPath:  homepageSeedPath,
		store:             store,
	}
}

func (s *ManagedListService) Init() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return migrateManagedListDB(db)
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
	return s.seedFromHomepage(db)
}

func (s *ManagedListService) ListLists() ([]ManagedList, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`select key, name, description, output_path, enabled, created_at, updated_at from lists order by key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []ManagedList
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

	row := db.QueryRow(`select key, name, description, output_path, enabled, created_at, updated_at from lists where key = ?`, key)
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
	if input.Name == "" {
		input.Name = input.Key
	}
	if input.OutputPath == "" {
		input.OutputPath = input.Key + ".json"
	}
	now := time.Now().UTC().Format(time.RFC3339)

	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	_, err = db.Exec(`
		insert into lists(key, name, description, output_path, enabled, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?)
		on conflict(key) do update set
			name = excluded.name,
			description = excluded.description,
			output_path = excluded.output_path,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, input.Key, input.Name, input.Description, input.OutputPath, boolToInt(input.Enabled), now, now)
	if err != nil {
		return nil, err
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
	result, err := db.Exec(`delete from lists where key = ?`, key)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return notFound("list not found")
	}
	return nil
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

	rows, err := db.Query(`
		select t.kind, t.chain, t.chain_name, t.chain_id, t.chain_logo_uri, t.address, t.asset_id, t.type,
			t.name, t.symbol, t.decimals, t.status, t.logo_uri, t.logo_exists, t.explorer, t.tags_json, t.source,
			li.slot, li.rank, li.enabled, li.display, li.display_name, li.display_symbol, li.note, li.created_at, li.updated_at
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

	var items []ManagedListItem
	for rows.Next() {
		item, err := scanManagedListItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *ManagedListService) UpsertItem(listKey string, input ManagedListItem) (*ManagedListItem, error) {
	listKey = normalizeListKey(listKey)
	if listKey == "" {
		return nil, invalidParams("list key is required")
	}
	token, err := s.resolveManagedToken(input.Token)
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
		return nil, err
	}

	var tokenID int64
	if err := tx.QueryRow(`select id from tokens where chain = ? and address = ?`, token.Chain, token.Address).Scan(&tokenID); err != nil {
		return nil, err
	}
	_, err = tx.Exec(`
		insert into list_items(list_id, token_id, slot, rank, enabled, display, display_name, display_symbol, note, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(list_id, token_id) do update set
			slot = excluded.slot,
			rank = excluded.rank,
			enabled = excluded.enabled,
			display = excluded.display,
			display_name = excluded.display_name,
			display_symbol = excluded.display_symbol,
			note = excluded.note,
			updated_at = excluded.updated_at
	`, listID, tokenID, normalizeSlot(input.Slot), input.Rank, boolToInt(input.Enabled), boolToInt(input.Display), input.DisplayName, input.DisplaySymbol, input.Note, now, now)
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
			t.name, t.symbol, t.decimals, t.status, t.logo_uri, t.logo_exists, t.explorer, t.tags_json, t.source,
			li.slot, li.rank, li.enabled, li.display, li.display_name, li.display_symbol, li.note, li.created_at, li.updated_at
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
	list, err := s.GetList(listKey)
	if err != nil {
		return nil, err
	}
	items, err := s.ListItems(list.Key)
	if err != nil {
		return nil, err
	}
	output := buildManagedListOutput(*list, items, time.Now().UTC().Format(time.RFC3339))
	return s.writePackedList(output, list.OutputPath)
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
		items, err := s.ListItems(list.Key)
		if err != nil {
			return nil, err
		}
		packed, err := s.writePackedList(buildManagedListOutput(list, items, manifest.GeneratedAt), list.OutputPath)
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
	db, err := sql.Open("sqlite3", s.dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`pragma foreign_keys = on`); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateManagedListDB(db); err != nil {
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
			token := s.enrichChainContext(managedTokenFromAsset(*detail))
			if input.Source != "" {
				token.Source = input.Source
			}
			return token, nil
		}
	}
	if input.Kind == "native" {
		if detail, err := s.store.readNativeAssetDetail(input.Chain, filepath.Join(s.store.root, "blockchains", input.Chain, "info")); err == nil {
			token := managedTokenFromAsset(*detail)
			if input.Source != "" {
				token.Source = input.Source
			}
			return s.enrichChainContext(token), nil
		}
	}
	if input.Kind == "" {
		input.Kind = "token"
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	input.Tags = appendUniqueStrings(nil, input.Tags...)
	return s.enrichChainContext(input), nil
}

func (s *ManagedListService) enrichChainContext(token ManagedToken) ManagedToken {
	if token.Chain == "" || s.store == nil {
		return token
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
	if err := os.MkdirAll(s.filesDir, 0o755); err != nil {
		return nil, err
	}
	relativePath, err := safePackOutputPath(output.Key, outputPath)
	if err != nil {
		return nil, err
	}
	jsonPath := filepath.Join(s.filesDir, relativePath)
	zstdPath := jsonPath + ".zst"
	if err := writeJSONAtomic(jsonPath, output); err != nil {
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
	if err := os.WriteFile(zstdPath, zstdBytes, 0o644); err != nil {
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
	jsonURL := "/files/" + filepath.ToSlash(relativePath)
	return &PackFile{
		ListKey:     output.Key,
		JSONPath:    jsonPath,
		ZstdPath:    zstdPath,
		JSONURL:     jsonURL,
		ZstdURL:     jsonURL + ".zst",
		JSONSize:    jsonInfo.Size(),
		ZstdSize:    zstdInfo.Size(),
		JSONSHA256:  sha256Hex(jsonBytes),
		ZstdSHA256:  sha256Hex(zstdBytes),
		TokenCount:  len(output.Tokens),
		GeneratedAt: output.GeneratedAt,
	}, nil
}

func safePackOutputPath(listKey, outputPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		outputPath = normalizeListKey(listKey) + ".json"
	}
	outputPath = filepath.Clean(outputPath)
	if filepath.IsAbs(outputPath) || outputPath == "." || strings.HasPrefix(outputPath, ".."+string(filepath.Separator)) || outputPath == ".." {
		return "", invalidParams("outputPath must stay inside managed list files directory")
	}
	if !strings.HasSuffix(outputPath, ".json") {
		outputPath += ".json"
	}
	return outputPath, nil
}

func migrateManagedListDB(db *sql.DB) error {
	stmts := []string{
		`create table if not exists lists (
			id integer primary key autoincrement,
			key text not null unique,
			name text not null default '',
			description text not null default '',
			output_path text not null default '',
			enabled integer not null default 1,
			created_at text not null,
			updated_at text not null
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
			source text not null default '',
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
			note text not null default '',
			created_at text not null,
			updated_at text not null,
			unique(list_id, token_id)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	alterStmts := []string{
		`alter table tokens add column chain_name text not null default ''`,
		`alter table tokens add column chain_id integer not null default 0`,
		`alter table tokens add column chain_logo_uri text not null default ''`,
		`alter table tokens add column explorer text not null default ''`,
		`alter table list_items add column slot text not null default ''`,
		`alter table list_items add column display integer not null default 1`,
	}
	for _, stmt := range alterStmts {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}
	return nil
}

func buildManagedListOutput(list ManagedList, items []ManagedListItem, generatedAt string) ManagedListOutput {
	tokens := make([]ManagedListToken, 0, len(items))
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
		}
		tokens = append(tokens, ManagedListToken{
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
			LogoURI:       token.LogoURI,
			LogoExists:    token.LogoExists,
			Explorer:      token.Explorer,
			Rank:          item.Rank,
			Tags:          token.Tags,
			Source:        token.Source,
		})
	}
	return ManagedListOutput{
		Key:         list.Key,
		Name:        list.Name,
		Description: list.Description,
		GeneratedAt: generatedAt,
		Version:     1,
		Tokens:      tokens,
	}
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
		Source:     "trustwallet-asset",
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
	err := scanner.Scan(&list.Key, &list.Name, &list.Description, &list.OutputPath, &enabled, &list.CreatedAt, &list.UpdatedAt)
	list.Enabled = enabled != 0
	return list, err
}

func scanManagedListItem(scanner managedListScanner) (ManagedListItem, error) {
	var item ManagedListItem
	var tagsJSON string
	var logoExists, enabled, display int
	err := scanner.Scan(
		&item.Token.Kind, &item.Token.Chain, &item.Token.ChainName, &item.Token.ChainID, &item.Token.ChainLogoURI,
		&item.Token.Address, &item.Token.AssetID, &item.Token.Type, &item.Token.Name, &item.Token.Symbol,
		&item.Token.Decimals, &item.Token.Status, &item.Token.LogoURI, &logoExists, &item.Token.Explorer,
		&tagsJSON, &item.Token.Source, &item.Slot, &item.Rank, &enabled, &display, &item.DisplayName,
		&item.DisplaySymbol, &item.Note, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return item, err
	}
	item.Token.LogoExists = logoExists != 0
	item.Enabled = enabled != 0
	item.Display = display != 0
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &item.Token.Tags)
	}
	return item, nil
}

func normalizeListKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, " ", "-")
	return key
}

func normalizeChain(chain string) string {
	return strings.ToLower(strings.TrimSpace(chain))
}

func normalizeSlot(slot string) string {
	return strings.ToLower(strings.TrimSpace(slot))
}

func isDuplicateColumnError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
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
