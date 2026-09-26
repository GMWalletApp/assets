package rpcserver

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

const relayXLayerLogoURI = "https://assets.relay.link/icons/196/light.png"

func TestEnrichChainContextForcesXLayerLogo(t *testing.T) {
	root := t.TempDir()
	service := NewManagedListService(filepath.Join(root, "lists.sqlite"), filepath.Join(root, "files"), "", "", "", "", NewStore(root, "https://cdn.example"))
	ethereumLogo := "https://cdn.example/blockchains/ethereum/info/logo.png"

	tests := []struct {
		name  string
		chain string
		in    string
		want  string
	}{
		{name: "stored xlayer relay logo", chain: "xlayer", in: relayXLayerLogoURI, want: DefaultXLayerLogoURI},
		{name: "empty xlayer logo", chain: "xlayer", in: "", want: DefaultXLayerLogoURI},
		{name: "xlayer case and spaces", chain: " XLayer ", in: relayXLayerLogoURI, want: DefaultXLayerLogoURI},
		{name: "ethereum keeps existing logo", chain: "ethereum", in: ethereumLogo, want: ethereumLogo},
		{name: "polygon still forced", chain: "polygon", in: relayXLayerLogoURI, want: DefaultPolygonLogoURI},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.enrichChainContext(ManagedToken{
				Kind:         "token",
				Chain:        tt.chain,
				Address:      "0xabc",
				Symbol:       "USDC",
				ChainLogoURI: tt.in,
			})
			if got.ChainLogoURI != tt.want {
				t.Fatalf("ChainLogoURI = %q, want %q", got.ChainLogoURI, tt.want)
			}
		})
	}
}

func TestManagedListReadsForcedXLayerLogo(t *testing.T) {
	root := t.TempDir()
	addNativeChain(t, root, "xlayer", map[string]any{"name": "X Layer", "symbol": "OKB", "type": "coin", "decimals": 18, "status": "active"})
	addNativeChain(t, root, "ethereum", map[string]any{"name": "Ethereum", "symbol": "ETH", "type": "coin", "decimals": 18, "status": "active"})
	addNativeChain(t, root, "polygon", map[string]any{"name": "Polygon", "symbol": "POL", "type": "coin", "decimals": 18, "status": "active"})
	dbPath := filepath.Join(root, "managed.sqlite")
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   dbPath,
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	if _, err := server.lists.UpsertList(ManagedList{Key: "wallet", Name: "Wallet", Enabled: true}); err != nil {
		t.Fatalf("upsert list: %v", err)
	}

	ethereumLogo := "https://cdn.example/blockchains/ethereum/info/logo.png"
	cases := []struct {
		name    string
		chain   string
		address string
		in      string
		want    string
	}{
		{name: "stored relay logo", chain: "xlayer", address: "0x111", in: relayXLayerLogoURI, want: DefaultXLayerLogoURI},
		{name: "empty logo", chain: "xlayer", address: "0x222", in: "", want: DefaultXLayerLogoURI},
		{name: "ethereum unchanged", chain: "ethereum", address: "0x333", in: ethereumLogo, want: ethereumLogo},
		{name: "polygon still forced", chain: "polygon", address: "0x444", in: relayXLayerLogoURI, want: DefaultPolygonLogoURI},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			saved, err := server.lists.SaveItem("wallet", ManagedListItem{
				Token: ManagedToken{
					Kind:         "token",
					Chain:        tt.chain,
					Address:      tt.address,
					Name:         "USD Coin",
					Symbol:       "USDC",
					Decimals:     6,
					ChainLogoURI: tt.in,
				},
				Rank:    1,
				Enabled: true,
				Display: true,
			})
			if err != nil {
				t.Fatalf("save item: %v", err)
			}
			if saved.Token.ChainLogoURI != tt.want {
				t.Fatalf("saved ChainLogoURI = %q, want %q", saved.Token.ChainLogoURI, tt.want)
			}
			read, err := server.lists.GetItem("wallet", tt.chain, tt.address)
			if err != nil {
				t.Fatalf("get item: %v", err)
			}
			if read.Token.ChainLogoURI != tt.want {
				t.Fatalf("read ChainLogoURI = %q, want %q", read.Token.ChainLogoURI, tt.want)
			}
		})
	}

	t.Run("legacy relay row is replaced on the next enrich", func(t *testing.T) {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatal(err)
		}
		result, err := db.Exec(`update tokens set chain_logo_uri = ? where chain = ? and address = ?`, relayXLayerLogoURI, "xlayer", "0x111")
		if err != nil {
			db.Close()
			t.Fatal(err)
		}
		updated, err := result.RowsAffected()
		db.Close()
		if err != nil {
			t.Fatal(err)
		}
		if updated != 1 {
			t.Fatalf("expected to poison one stored xlayer row, updated %d", updated)
		}
		legacy, err := server.lists.GetItem("wallet", "xlayer", "0x111")
		if err != nil {
			t.Fatal(err)
		}
		if legacy.Token.ChainLogoURI != relayXLayerLogoURI {
			t.Fatalf("legacy row was not stored, got %q", legacy.Token.ChainLogoURI)
		}
		rewritten, err := server.lists.SaveItem("wallet", *legacy)
		if err != nil {
			t.Fatal(err)
		}
		if rewritten.Token.ChainLogoURI != DefaultXLayerLogoURI {
			t.Fatalf("rewritten ChainLogoURI = %q, want %q", rewritten.Token.ChainLogoURI, DefaultXLayerLogoURI)
		}
		read, err := server.lists.GetItem("wallet", "xlayer", "0x111")
		if err != nil {
			t.Fatal(err)
		}
		if read.Token.ChainLogoURI != DefaultXLayerLogoURI {
			t.Fatalf("read after enrich ChainLogoURI = %q, want %q", read.Token.ChainLogoURI, DefaultXLayerLogoURI)
		}
	})
}

func TestManagedListAPIForcesXLayerChainLogo(t *testing.T) {
	root := t.TempDir()
	addNativeChain(t, root, "xlayer", map[string]any{"name": "X Layer", "symbol": "OKB", "type": "coin", "decimals": 18, "status": "active"})
	server := NewServer(Config{
		Root:                root,
		AssetBaseURL:        "https://cdn.example",
		ManagedListDBPath:   filepath.Join(root, "managed.sqlite"),
		ManagedListFilesDir: filepath.Join(root, "files"),
	})
	handler := server.listAPIHandler()
	rec := doHTTP(t, handler, http.MethodPost, "/api/lists", `{"key":"wallet","name":"Wallet","enabled":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create list status=%d body=%s", rec.Code, rec.Body.String())
	}

	createBody := `{"token":{"kind":"token","chain":"xlayer","address":"0xabc","name":"USD Coin","symbol":"USDC","decimals":6,"chainLogoURI":"` + relayXLayerLogoURI + `"},"rank":1,"enabled":true,"display":true}`
	rec = doHTTP(t, handler, http.MethodPost, "/api/lists/wallet/items", createBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item status=%d body=%s", rec.Code, rec.Body.String())
	}
	item := decodeManagedListItem(t, rec.Body.Bytes())
	if item.Token.ChainLogoURI != DefaultXLayerLogoURI || item.Token.ChainName != "X Layer" {
		t.Fatalf("create should force repository logo and keep chain name, got %+v", item.Token)
	}

	rec = doHTTP(t, handler, http.MethodPatch, "/api/lists/wallet/items/xlayer/0xabc", `{"token":{"chainLogoURI":"`+relayXLayerLogoURI+`"}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch item status=%d body=%s", rec.Code, rec.Body.String())
	}
	item = decodeManagedListItem(t, rec.Body.Bytes())
	if item.Token.ChainLogoURI != DefaultXLayerLogoURI {
		t.Fatalf("patch ChainLogoURI = %q, want %q", item.Token.ChainLogoURI, DefaultXLayerLogoURI)
	}
	rec = doHTTP(t, handler, http.MethodGet, "/api/lists/wallet/items/xlayer/0xabc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get item status=%d body=%s", rec.Code, rec.Body.String())
	}
	item = decodeManagedListItem(t, rec.Body.Bytes())
	if item.Token.ChainLogoURI != DefaultXLayerLogoURI || item.Token.Name != "USD Coin" || item.Rank != 1 {
		t.Fatalf("get after patch changed more than the chain logo, got %+v", item)
	}
}

func decodeManagedListItem(t *testing.T, body []byte) ManagedListItem {
	t.Helper()
	var item ManagedListItem
	if err := json.Unmarshal(body, &item); err != nil {
		t.Fatalf("decode item: %v body=%s", err, body)
	}
	return item
}
