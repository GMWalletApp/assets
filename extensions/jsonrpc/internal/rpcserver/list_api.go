package rpcserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *Server) listAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/lists"), "/")
		parts := []string{}
		if path != "" {
			parts = strings.Split(path, "/")
		}

		if len(parts) == 0 {
			switch r.Method {
			case http.MethodGet:
				s.handleListLists(w)
			case http.MethodPost:
				s.handleUpsertList(w, r, "")
			default:
				writeMethodNotAllowed(w)
			}
			return
		}

		listKey := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				s.handleGetList(w, listKey)
			case http.MethodPatch, http.MethodPut:
				s.handleUpsertList(w, r, listKey)
			case http.MethodDelete:
				s.handleDeleteList(w, listKey)
			default:
				writeMethodNotAllowed(w)
			}
			return
		}

		if parts[1] != "items" {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}

		if len(parts) == 2 {
			switch r.Method {
			case http.MethodGet:
				s.handleListItems(w, listKey)
			case http.MethodPost:
				s.handleUpsertListItem(w, r, listKey, "", "")
			default:
				writeMethodNotAllowed(w)
			}
			return
		}

		if len(parts) != 4 {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		chain := parts[2]
		address := parts[3]
		switch r.Method {
		case http.MethodPatch, http.MethodPut:
			s.handleUpsertListItem(w, r, listKey, chain, address)
		case http.MethodDelete:
			s.handleDeleteListItem(w, listKey, chain, address)
		default:
			writeMethodNotAllowed(w)
		}
	})
}

func (s *Server) packAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		listKey := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/pack"), "/")
		if listKey == "" {
			writeJSONError(w, http.StatusNotFound, "not found")
			return
		}
		if listKey == "all" {
			manifest, err := s.lists.PackAll()
			writeJSONResultOrError(w, manifest, err)
			return
		}
		packed, err := s.lists.PackList(listKey)
		writeJSONResultOrError(w, packed, err)
	})
}

func (s *Server) handleListLists(w http.ResponseWriter) {
	lists, err := s.lists.ListLists()
	writeJSONResultOrError(w, lists, err)
}

func (s *Server) handleGetList(w http.ResponseWriter, key string) {
	list, err := s.lists.GetList(key)
	writeJSONResultOrError(w, list, err)
}

func (s *Server) handleUpsertList(w http.ResponseWriter, r *http.Request, key string) {
	var input ManagedList
	if err := decodeHTTPJSON(r, &input); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if key != "" {
		input.Key = key
	}
	if r.Method == http.MethodPost && !input.Enabled {
		input.Enabled = true
	}
	list, err := s.lists.UpsertList(input)
	writeJSONResultOrError(w, list, err)
}

func (s *Server) handleDeleteList(w http.ResponseWriter, key string) {
	err := s.lists.DeleteList(key)
	writeJSONResultOrError(w, map[string]bool{"deleted": err == nil}, err)
}

func (s *Server) handleListItems(w http.ResponseWriter, key string) {
	items, err := s.lists.ListItems(key)
	writeJSONResultOrError(w, items, err)
}

func (s *Server) handleUpsertListItem(w http.ResponseWriter, r *http.Request, listKey, chain, address string) {
	var request managedListItemRequest
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	input := request.toManagedListItem()
	if chain != "" {
		input.Token.Chain = chain
	}
	if address != "" {
		input.Token.Address = pathAddressToTokenAddress(address)
		if input.Token.Address == "" && input.Token.Kind == "" {
			input.Token.Kind = "native"
		}
	}
	if r.Method == http.MethodPost && request.Enabled == nil {
		input.Enabled = true
	}
	if r.Method == http.MethodPost && request.Display == nil {
		input.Display = true
	}
	item, err := s.lists.UpsertItem(listKey, input)
	writeJSONResultOrError(w, item, err)
}

func (s *Server) handleDeleteListItem(w http.ResponseWriter, listKey, chain, address string) {
	err := s.lists.DeleteItem(listKey, chain, pathAddressToTokenAddress(address))
	writeJSONResultOrError(w, map[string]bool{"deleted": err == nil}, err)
}

type managedListItemRequest struct {
	Token         ManagedToken `json:"token"`
	Slot          string       `json:"slot,omitempty"`
	Rank          int          `json:"rank,omitempty"`
	Enabled       *bool        `json:"enabled,omitempty"`
	Display       *bool        `json:"display,omitempty"`
	DisplayName   string       `json:"displayName,omitempty"`
	DisplaySymbol string       `json:"displaySymbol,omitempty"`
	Note          string       `json:"note,omitempty"`
}

func (r managedListItemRequest) toManagedListItem() ManagedListItem {
	item := ManagedListItem{
		Token:         r.Token,
		Slot:          r.Slot,
		Rank:          r.Rank,
		DisplayName:   r.DisplayName,
		DisplaySymbol: r.DisplaySymbol,
		Note:          r.Note,
	}
	if r.Enabled != nil {
		item.Enabled = *r.Enabled
	}
	if r.Display != nil {
		item.Display = *r.Display
	}
	return item
}

func pathAddressToTokenAddress(address string) string {
	address = strings.TrimSpace(address)
	if strings.EqualFold(address, "native") || address == "_" {
		return ""
	}
	return address
}

func decodeHTTPJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, maxRequestBodyBytes+1)
	if err := json.NewDecoder(limited).Decode(target); err != nil {
		return err
	}
	if counter, ok := limited.(*io.LimitedReader); ok && counter.N == 0 {
		return fmt.Errorf("request body too large")
	}
	return nil
}

func writeJSONResultOrError(w http.ResponseWriter, result any, err error) {
	if err != nil {
		status := http.StatusInternalServerError
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) {
			switch rpcErr.Code {
			case ErrCodeInvalidParams:
				status = http.StatusBadRequest
			case ErrCodeNotFound:
				status = http.StatusNotFound
			}
			writeJSONError(w, status, rpcErr.Message)
			return
		}
		writeJSONError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func managedFilesHandler(filesDir string) http.Handler {
	return http.StripPrefix("/files/", http.FileServer(http.Dir(filepath.Clean(filesDir))))
}
