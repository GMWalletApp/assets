package rpcserver

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	supportListKey      = "support"
	supportAssetBaseURI = "https://raw.githubusercontent.com/GMWalletApp/assets/main/support"
	staticAssetBaseURI  = "https://raw.githubusercontent.com/GMWalletApp/assets/main/data/static"
)

func validSupportCategory(category string) bool {
	return category == "exchanges" || category == "wallets"
}

func normalizeSupportEntryID(id string) string {
	return normalizeListKey(id)
}

func validateSupportEntry(category string, entry ManagedSupportEntry) (ManagedSupportEntry, error) {
	if !validSupportCategory(category) {
		return ManagedSupportEntry{}, invalidParams("support category must be exchanges or wallets")
	}
	entry.ID = normalizeSupportEntryID(entry.ID)
	entry.Name = strings.TrimSpace(entry.Name)
	entry.Type = strings.ToLower(strings.TrimSpace(entry.Type))
	entry.LogoURI = strings.TrimSpace(entry.LogoURI)
	if entry.ID == "" || !validListKey(entry.ID) {
		return ManagedSupportEntry{}, invalidParams("support entry id must be a lowercase identifier")
	}
	if entry.Name == "" {
		return ManagedSupportEntry{}, invalidParams("support entry name is required")
	}
	if entry.LogoURI == "" {
		return ManagedSupportEntry{}, invalidParams("support entry logoURI is required")
	}
	if entry.Rank < 0 {
		return ManagedSupportEntry{}, invalidParams("rank must be greater than or equal to zero")
	}
	if category == "exchanges" {
		if entry.Type != "cex" && entry.Type != "dex" {
			return ManagedSupportEntry{}, invalidParams("exchange type must be cex or dex")
		}
	} else if entry.Type != "" {
		return ManagedSupportEntry{}, invalidParams("wallet entries do not have a type")
	}
	return entry, nil
}

func (s *ManagedListService) GetSupportDocument() (*ManagedSupportDocument, error) {
	list, err := s.GetList(supportListKey)
	if err != nil {
		return nil, err
	}
	exchanges, err := s.ListSupportEntries("exchanges")
	if err != nil {
		return nil, err
	}
	wallets, err := s.ListSupportEntries("wallets")
	if err != nil {
		return nil, err
	}
	return &ManagedSupportDocument{
		ManagedList:   *list,
		SchemaVersion: 1,
		AssetBaseURI:  supportAssetBaseURI,
		Exchanges:     exchanges,
		Wallets:       wallets,
	}, nil
}

func (s *ManagedListService) ListSupportEntries(category string) ([]ManagedSupportEntry, error) {
	if !validSupportCategory(category) {
		return nil, invalidParams("support category must be exchanges or wallets")
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var exists int
	if err := db.QueryRow(`select 1 from lists where key = ?`, supportListKey).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("support list not found")
	} else if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		select se.entry_id, se.name, se.type, se.logo_uri, se.rank, se.enabled, se.created_at, se.updated_at
		from support_entries se join lists l on l.id = se.list_id
		where l.key = ? and se.category = ?
		order by se.rank asc, se.entry_id asc
	`, supportListKey, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []ManagedSupportEntry{}
	for rows.Next() {
		entry, err := scanManagedSupportEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (s *ManagedListService) GetSupportEntry(category, id string) (*ManagedSupportEntry, error) {
	if !validSupportCategory(category) {
		return nil, invalidParams("support category must be exchanges or wallets")
	}
	id = normalizeSupportEntryID(id)
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	entry, err := scanManagedSupportEntry(db.QueryRow(`
		select se.entry_id, se.name, se.type, se.logo_uri, se.rank, se.enabled, se.created_at, se.updated_at
		from support_entries se join lists l on l.id = se.list_id
		where l.key = ? and se.category = ? and se.entry_id = ?
	`, supportListKey, category, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("support entry not found")
	}
	return &entry, err
}

func (s *ManagedListService) SaveSupportEntry(category string, input ManagedSupportEntry) (*ManagedSupportEntry, error) {
	input, err := validateSupportEntry(category, input)
	if err != nil {
		return nil, err
	}
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var listID int64
	if err := db.QueryRow(`select id from lists where key = ?`, supportListKey).Scan(&listID); errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("support list not found")
	} else if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`
		insert into support_entries(list_id, category, entry_id, name, type, logo_uri, rank, enabled, created_at, updated_at)
		values(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(list_id, category, entry_id) do update set
			name = excluded.name,
			type = excluded.type,
			logo_uri = excluded.logo_uri,
			rank = excluded.rank,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, listID, category, input.ID, input.Name, input.Type, input.LogoURI, input.Rank, boolToInt(input.Enabled), now, now)
	if err != nil {
		return nil, err
	}
	return s.GetSupportEntry(category, input.ID)
}

func (s *ManagedListService) DeleteSupportEntry(category, id string) error {
	if !validSupportCategory(category) {
		return invalidParams("support category must be exchanges or wallets")
	}
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := db.Exec(`
		delete from support_entries
		where list_id = (select id from lists where key = ?) and category = ? and entry_id = ?
	`, supportListKey, category, normalizeSupportEntryID(id))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return notFound("support entry not found")
	}
	return err
}

type managedSupportScanner interface {
	Scan(dest ...any) error
}

func scanManagedSupportEntry(scanner managedSupportScanner) (ManagedSupportEntry, error) {
	var entry ManagedSupportEntry
	var enabled int
	err := scanner.Scan(&entry.ID, &entry.Name, &entry.Type, &entry.LogoURI, &entry.Rank, &enabled, &entry.CreatedAt, &entry.UpdatedAt)
	entry.Enabled = enabled != 0
	return entry, err
}

func (s *ManagedListService) seedSupportList(db *sql.DB) error {
	if s.store == nil {
		return nil
	}
	seedPath := filepath.Join(s.store.root, "support", "support.json")
	data, err := os.ReadFile(seedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var source PublishedSupportDocument
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}
	if err := seedList(db, supportListKey, "Support List", "Supported exchange, DEX and wallet logos.", "", "", "", "support.json"); err != nil {
		return err
	}
	for i, entry := range source.Exchanges {
		if err := seedSupportEntry(db, "exchanges", ManagedSupportEntry{ID: entry.ID, Name: entry.Name, Type: entry.Type, LogoURI: entry.LogoURI, Rank: i + 1, Enabled: true}); err != nil {
			return err
		}
	}
	for i, entry := range source.Wallets {
		if err := seedSupportEntry(db, "wallets", ManagedSupportEntry{ID: entry.ID, Name: entry.Name, LogoURI: entry.LogoURI, Rank: i + 1, Enabled: true}); err != nil {
			return err
		}
	}
	extraWallets := []ManagedSupportEntry{
		{ID: "bitget-wallet", Name: "Bitget Wallet", LogoURI: staticAssetBaseURI + "/bitget-wallet.svg", Rank: len(source.Wallets) + 1, Enabled: true},
		{ID: "uniswap", Name: "Uniswap Wallet", LogoURI: staticAssetBaseURI + "/uniswap-wallet.svg", Rank: len(source.Wallets) + 2, Enabled: true},
	}
	for _, entry := range extraWallets {
		if err := seedSupportEntry(db, "wallets", entry); err != nil {
			return err
		}
	}
	return nil
}

func seedSupportEntry(db *sql.DB, category string, entry ManagedSupportEntry) error {
	entry, err := validateSupportEntry(category, entry)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = db.Exec(`
		insert into support_entries(list_id, category, entry_id, name, type, logo_uri, rank, enabled, created_at, updated_at)
		select id, ?, ?, ?, ?, ?, ?, ?, ?, ? from lists where key = ?
		on conflict(list_id, category, entry_id) do nothing
	`, category, entry.ID, entry.Name, entry.Type, entry.LogoURI, entry.Rank, boolToInt(entry.Enabled), now, now, supportListKey)
	return err
}

func (s *ManagedListService) packSupportList(generatedAt string) (*PackFile, error) {
	list, err := s.GetList(supportListKey)
	if err != nil {
		return nil, err
	}
	exchanges, err := s.ListSupportEntries("exchanges")
	if err != nil {
		return nil, err
	}
	wallets, err := s.ListSupportEntries("wallets")
	if err != nil {
		return nil, err
	}
	publishedExchanges := make([]PublishedSupportEntry, 0, len(exchanges))
	publishedWallets := make([]PublishedSupportEntry, 0, len(wallets))
	for _, entry := range exchanges {
		if entry.Enabled {
			publishedExchanges = append(publishedExchanges, PublishedSupportEntry{ID: entry.ID, Name: entry.Name, Type: entry.Type, LogoURI: entry.LogoURI})
		}
	}
	for _, entry := range wallets {
		if entry.Enabled {
			publishedWallets = append(publishedWallets, PublishedSupportEntry{ID: entry.ID, Name: entry.Name, LogoURI: entry.LogoURI})
		}
	}
	document := PublishedSupportDocument{SchemaVersion: 1, AssetBaseURI: supportAssetBaseURI, Exchanges: publishedExchanges, Wallets: publishedWallets}
	return s.writePackedDocument(list.Key, list.OutputPath, document, len(publishedExchanges)+len(publishedWallets), generatedAt)
}
