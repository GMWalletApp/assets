package rpcserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
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
				s.handleCreateList(w, r)
			default:
				writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
			}
			return
		}

		listKey := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				s.handleGetList(w, listKey)
			case http.MethodPut:
				s.handleReplaceList(w, r, listKey)
			case http.MethodPatch:
				s.handlePatchList(w, r, listKey)
			case http.MethodDelete:
				s.handleDeleteList(w, listKey)
			default:
				writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
			}
			return
		}

		if parts[1] == "exchanges" || parts[1] == "wallets" {
			if normalizeListKey(listKey) != supportListKey {
				writeJSONError(w, http.StatusNotFound, "not found")
				return
			}
			category := parts[1]
			if len(parts) == 2 {
				switch r.Method {
				case http.MethodGet:
					s.handleListSupportEntries(w, category)
				case http.MethodPost:
					s.handleCreateSupportEntry(w, r, category)
				default:
					writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
				}
				return
			}
			if len(parts) != 3 {
				writeJSONError(w, http.StatusNotFound, "not found")
				return
			}
			id := parts[2]
			switch r.Method {
			case http.MethodGet:
				s.handleGetSupportEntry(w, category, id)
			case http.MethodPut:
				s.handleReplaceSupportEntry(w, r, category, id)
			case http.MethodPatch:
				s.handlePatchSupportEntry(w, r, category, id)
			case http.MethodDelete:
				s.handleDeleteSupportEntry(w, category, id)
			default:
				writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
			}
			return
		}

		if parts[1] == "includes" {
			if len(parts) == 2 {
				switch r.Method {
				case http.MethodGet:
					s.handleListIncludes(w, listKey)
				case http.MethodPost:
					s.handleCreateListInclude(w, r, listKey)
				default:
					writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
				}
				return
			}
			if len(parts) != 3 {
				writeJSONError(w, http.StatusNotFound, "not found")
				return
			}
			tag := parts[2]
			switch r.Method {
			case http.MethodGet:
				s.handleGetListInclude(w, listKey, tag)
			case http.MethodPut:
				s.handleReplaceListInclude(w, r, listKey, tag)
			case http.MethodPatch:
				s.handlePatchListInclude(w, r, listKey, tag)
			case http.MethodDelete:
				s.handleDeleteListInclude(w, listKey, tag)
			default:
				writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
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
				s.handleCreateListItem(w, r, listKey)
			default:
				writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
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
		case http.MethodGet:
			s.handleGetListItem(w, listKey, chain, address)
		case http.MethodPut:
			s.handleReplaceListItem(w, r, listKey, chain, address)
		case http.MethodPatch:
			s.handlePatchListItem(w, r, listKey, chain, address)
		case http.MethodDelete:
			s.handleDeleteListItem(w, listKey, chain, address)
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete)
		}
	})
}

func (s *Server) packAPIHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
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
	if normalizeListKey(key) == supportListKey {
		list, err := s.lists.GetSupportDocument()
		writeJSONResultOrError(w, list, err)
		return
	}
	list, err := s.lists.GetListDocument(key)
	writeJSONResultOrError(w, list, err)
}

type managedListWriteRequest struct {
	Key           *string `json:"key"`
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	DisplayName   *string `json:"displayName"`
	DisplaySymbol *string `json:"displaySymbol"`
	LogoURI       *string `json:"logoURI"`
	OutputPath    *string `json:"outputPath"`
	Enabled       *bool   `json:"enabled"`
}

func (input managedListWriteRequest) apply(list *ManagedList) {
	if input.Key != nil {
		list.Key = *input.Key
	}
	if input.Name != nil {
		list.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		list.Description = strings.TrimSpace(*input.Description)
	}
	if input.DisplayName != nil {
		list.DisplayName = strings.TrimSpace(*input.DisplayName)
	}
	if input.DisplaySymbol != nil {
		list.DisplaySymbol = strings.TrimSpace(*input.DisplaySymbol)
	}
	if input.LogoURI != nil {
		list.LogoURI = strings.TrimSpace(*input.LogoURI)
	}
	if input.OutputPath != nil {
		list.OutputPath = strings.TrimSpace(*input.OutputPath)
	}
	if input.Enabled != nil {
		list.Enabled = *input.Enabled
	}
}

func (input managedListWriteRequest) emptyPatch() bool {
	return input.Key == nil && input.Name == nil && input.Description == nil && input.DisplayName == nil && input.DisplaySymbol == nil && input.LogoURI == nil && input.OutputPath == nil && input.Enabled == nil
}

func (s *Server) handleCreateList(w http.ResponseWriter, r *http.Request) {
	input := managedListWriteRequest{}
	if err := decodeHTTPJSON(r, &input); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Key == nil || strings.TrimSpace(*input.Key) == "" {
		writeJSONError(w, http.StatusBadRequest, "key is required")
		return
	}
	key := normalizeListKey(*input.Key)
	if _, err := s.lists.GetList(key); err == nil {
		writeJSONResultOrErrorStatus(w, nil, conflict("list already exists"), http.StatusCreated)
		return
	} else if !isNotFoundError(err) {
		writeJSONResultOrError(w, nil, err)
		return
	}
	list := ManagedList{Key: key, Enabled: true}
	input.apply(&list)
	created, err := s.lists.UpsertList(list)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	w.Header().Set("Location", "/api/lists/"+created.Key)
	document, err := s.listDocumentForResponse(created.Key)
	writeJSONResultOrErrorStatus(w, document, err, http.StatusCreated)
}

func (s *Server) handleReplaceList(w http.ResponseWriter, r *http.Request, key string) {
	input := managedListWriteRequest{}
	if err := decodeHTTPJSON(r, &input); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Key != nil && normalizeListKey(*input.Key) != normalizeListKey(key) {
		writeJSONError(w, http.StatusBadRequest, "request key must match the path key")
		return
	}
	_, getErr := s.lists.GetList(key)
	status := http.StatusOK
	if isNotFoundError(getErr) {
		status = http.StatusCreated
	} else if getErr != nil {
		writeJSONResultOrError(w, nil, getErr)
		return
	}
	list := ManagedList{Key: key}
	input.apply(&list)
	list.Key = key
	updated, err := s.lists.UpsertList(list)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	if status == http.StatusCreated {
		w.Header().Set("Location", "/api/lists/"+updated.Key)
	}
	document, err := s.listDocumentForResponse(updated.Key)
	writeJSONResultOrErrorStatus(w, document, err, status)
}

func (s *Server) handlePatchList(w http.ResponseWriter, r *http.Request, key string) {
	existing, err := s.lists.GetList(key)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	input := managedListWriteRequest{}
	if err := decodeHTTPJSON(r, &input); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if input.Key != nil {
		writeJSONError(w, http.StatusBadRequest, "list key is immutable")
		return
	}
	if input.emptyPatch() {
		writeJSONError(w, http.StatusBadRequest, "at least one mutable list field is required")
		return
	}
	input.apply(existing)
	updated, err := s.lists.UpsertList(*existing)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	document, err := s.listDocumentForResponse(updated.Key)
	writeJSONResultOrError(w, document, err)
}

func (s *Server) listDocumentForResponse(key string) (any, error) {
	if normalizeListKey(key) == supportListKey {
		return s.lists.GetSupportDocument()
	}
	return s.lists.GetListDocument(key)
}

func (s *Server) handleDeleteList(w http.ResponseWriter, key string) {
	err := s.lists.DeleteList(key)
	writeNoContentOrError(w, err)
}

func (s *Server) handleListItems(w http.ResponseWriter, key string) {
	items, err := s.lists.ListItems(key)
	writeJSONResultOrError(w, items, err)
}

type managedListIncludeWriteRequest struct {
	Tag       *string `json:"tag"`
	Rank      *int    `json:"rank"`
	Enabled   *bool   `json:"enabled"`
	CreatedAt *string `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

func (s *Server) handleListIncludes(w http.ResponseWriter, key string) {
	includes, err := s.lists.ListIncludes(key)
	writeJSONResultOrError(w, includes, err)
}

func (s *Server) handleCreateListInclude(w http.ResponseWriter, r *http.Request, listKey string) {
	request := managedListIncludeWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Tag == nil || strings.TrimSpace(*request.Tag) == "" {
		writeJSONError(w, http.StatusBadRequest, "tag is required")
		return
	}
	tag := normalizeListKey(*request.Tag)
	if _, err := s.lists.GetInclude(listKey, tag); err == nil {
		writeJSONResultOrErrorStatus(w, nil, conflict("list include already exists"), http.StatusCreated)
		return
	} else if !isNotFoundError(err) {
		writeJSONResultOrError(w, nil, err)
		return
	}
	include := ManagedListInclude{Tag: tag, Enabled: true}
	applyListIncludeWrite(request, &include)
	created, err := s.lists.SaveInclude(listKey, include)
	if err == nil {
		w.Header().Set("Location", listIncludeLocation(listKey, tag))
	}
	writeJSONResultOrErrorStatus(w, created, err, http.StatusCreated)
}

func (s *Server) handleGetListInclude(w http.ResponseWriter, listKey, tag string) {
	include, err := s.lists.GetInclude(listKey, tag)
	writeJSONResultOrError(w, include, err)
}

func (s *Server) handleReplaceListInclude(w http.ResponseWriter, r *http.Request, listKey, tag string) {
	request := managedListIncludeWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	tag = normalizeListKey(tag)
	if request.Tag != nil && normalizeListKey(*request.Tag) != tag {
		writeJSONError(w, http.StatusBadRequest, "include tag must match the path tag")
		return
	}
	_, getErr := s.lists.GetInclude(listKey, tag)
	status := http.StatusOK
	if isNotFoundError(getErr) {
		status = http.StatusCreated
	} else if getErr != nil {
		writeJSONResultOrError(w, nil, getErr)
		return
	}
	include := ManagedListInclude{Tag: tag, Enabled: true}
	applyListIncludeWrite(request, &include)
	include.Tag = tag
	updated, err := s.lists.SaveInclude(listKey, include)
	if err == nil && status == http.StatusCreated {
		w.Header().Set("Location", listIncludeLocation(listKey, tag))
	}
	writeJSONResultOrErrorStatus(w, updated, err, status)
}

func (s *Server) handlePatchListInclude(w http.ResponseWriter, r *http.Request, listKey, tag string) {
	include, err := s.lists.GetInclude(listKey, tag)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	request := managedListIncludeWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Tag != nil {
		writeJSONError(w, http.StatusBadRequest, "include tag is immutable")
		return
	}
	if request.Rank == nil && request.Enabled == nil {
		writeJSONError(w, http.StatusBadRequest, "at least one mutable include field is required")
		return
	}
	applyListIncludeWrite(request, include)
	updated, err := s.lists.SaveInclude(listKey, *include)
	writeJSONResultOrError(w, updated, err)
}

func (s *Server) handleDeleteListInclude(w http.ResponseWriter, listKey, tag string) {
	writeNoContentOrError(w, s.lists.DeleteInclude(listKey, tag))
}

func applyListIncludeWrite(request managedListIncludeWriteRequest, include *ManagedListInclude) {
	if request.Tag != nil {
		include.Tag = normalizeListKey(*request.Tag)
	}
	if request.Rank != nil {
		include.Rank = *request.Rank
	}
	if request.Enabled != nil {
		include.Enabled = *request.Enabled
	}
}

func listIncludeLocation(listKey, tag string) string {
	return "/api/lists/" + normalizeListKey(listKey) + "/includes/" + normalizeListKey(tag)
}

type managedSupportEntryWriteRequest struct {
	ID        *string `json:"id"`
	Name      *string `json:"name"`
	Type      *string `json:"type"`
	LogoURI   *string `json:"logoURI"`
	Rank      *int    `json:"rank"`
	Enabled   *bool   `json:"enabled"`
	CreatedAt *string `json:"createdAt"`
	UpdatedAt *string `json:"updatedAt"`
}

func (input managedSupportEntryWriteRequest) apply(entry *ManagedSupportEntry) {
	if input.ID != nil {
		entry.ID = normalizeSupportEntryID(*input.ID)
	}
	if input.Name != nil {
		entry.Name = strings.TrimSpace(*input.Name)
	}
	if input.Type != nil {
		entry.Type = strings.TrimSpace(*input.Type)
	}
	if input.LogoURI != nil {
		entry.LogoURI = strings.TrimSpace(*input.LogoURI)
	}
	if input.Rank != nil {
		entry.Rank = *input.Rank
	}
	if input.Enabled != nil {
		entry.Enabled = *input.Enabled
	}
}

func (input managedSupportEntryWriteRequest) emptyPatch() bool {
	return input.ID == nil && input.Name == nil && input.Type == nil && input.LogoURI == nil && input.Rank == nil && input.Enabled == nil
}

func (s *Server) handleListSupportEntries(w http.ResponseWriter, category string) {
	entries, err := s.lists.ListSupportEntries(category)
	writeJSONResultOrError(w, entries, err)
}

func (s *Server) handleCreateSupportEntry(w http.ResponseWriter, r *http.Request, category string) {
	request := managedSupportEntryWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.ID == nil || request.Name == nil || request.LogoURI == nil {
		writeJSONError(w, http.StatusBadRequest, "id, name and logoURI are required")
		return
	}
	if category == "exchanges" && request.Type == nil {
		writeJSONError(w, http.StatusBadRequest, "exchange type is required")
		return
	}
	entry := ManagedSupportEntry{Enabled: true}
	request.apply(&entry)
	if _, err := s.lists.GetSupportEntry(category, entry.ID); err == nil {
		writeJSONResultOrErrorStatus(w, nil, conflict("support entry already exists"), http.StatusCreated)
		return
	} else if !isNotFoundError(err) {
		writeJSONResultOrError(w, nil, err)
		return
	}
	created, err := s.lists.SaveSupportEntry(category, entry)
	if err == nil {
		w.Header().Set("Location", supportEntryLocation(category, created.ID))
	}
	writeJSONResultOrErrorStatus(w, created, err, http.StatusCreated)
}

func (s *Server) handleGetSupportEntry(w http.ResponseWriter, category, id string) {
	entry, err := s.lists.GetSupportEntry(category, id)
	writeJSONResultOrError(w, entry, err)
}

func (s *Server) handleReplaceSupportEntry(w http.ResponseWriter, r *http.Request, category, id string) {
	request := managedSupportEntryWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	id = normalizeSupportEntryID(id)
	if request.ID != nil && normalizeSupportEntryID(*request.ID) != id {
		writeJSONError(w, http.StatusBadRequest, "support entry id must match the path id")
		return
	}
	if request.Name == nil || request.LogoURI == nil {
		writeJSONError(w, http.StatusBadRequest, "name and logoURI are required")
		return
	}
	if category == "exchanges" && request.Type == nil {
		writeJSONError(w, http.StatusBadRequest, "exchange type is required")
		return
	}
	_, getErr := s.lists.GetSupportEntry(category, id)
	status := http.StatusOK
	if isNotFoundError(getErr) {
		status = http.StatusCreated
	} else if getErr != nil {
		writeJSONResultOrError(w, nil, getErr)
		return
	}
	entry := ManagedSupportEntry{ID: id, Enabled: true}
	request.apply(&entry)
	entry.ID = id
	updated, err := s.lists.SaveSupportEntry(category, entry)
	if err == nil && status == http.StatusCreated {
		w.Header().Set("Location", supportEntryLocation(category, id))
	}
	writeJSONResultOrErrorStatus(w, updated, err, status)
}

func (s *Server) handlePatchSupportEntry(w http.ResponseWriter, r *http.Request, category, id string) {
	entry, err := s.lists.GetSupportEntry(category, id)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	request := managedSupportEntryWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.ID != nil {
		writeJSONError(w, http.StatusBadRequest, "support entry id is immutable")
		return
	}
	if request.emptyPatch() {
		writeJSONError(w, http.StatusBadRequest, "at least one mutable support entry field is required")
		return
	}
	request.apply(entry)
	updated, err := s.lists.SaveSupportEntry(category, *entry)
	writeJSONResultOrError(w, updated, err)
}

func (s *Server) handleDeleteSupportEntry(w http.ResponseWriter, category, id string) {
	writeNoContentOrError(w, s.lists.DeleteSupportEntry(category, id))
}

func supportEntryLocation(category, id string) string {
	return "/api/lists/support/" + category + "/" + normalizeSupportEntryID(id)
}

type managedListItemWriteRequest struct {
	Token          *ManagedToken `json:"token"`
	Slot           *string       `json:"slot"`
	Rank           *int          `json:"rank"`
	Enabled        *bool         `json:"enabled"`
	Display        *bool         `json:"display"`
	DisplayName    *string       `json:"displayName"`
	DisplaySymbol  *string       `json:"displaySymbol"`
	DisplayLogoURI *string       `json:"displayLogoURI"`
	CreatedAt      *string       `json:"createdAt"`
	UpdatedAt      *string       `json:"updatedAt"`
}

type managedTokenPatch struct {
	Kind         *string             `json:"kind"`
	Chain        *string             `json:"chain"`
	ChainName    *string             `json:"chainName"`
	ChainID      *int                `json:"chainId"`
	ChainLogoURI *string             `json:"chainLogoURI"`
	Address      *string             `json:"address"`
	AssetID      *string             `json:"assetId"`
	Type         *string             `json:"type"`
	Name         *string             `json:"name"`
	Symbol       *string             `json:"symbol"`
	Decimals     *int                `json:"decimals"`
	Status       *string             `json:"status"`
	LogoURI      *string             `json:"logoURI"`
	LogoExists   *bool               `json:"logoExists"`
	Explorer     *string             `json:"explorer"`
	Tags         *[]string           `json:"tags"`
	Hot          *bool               `json:"hot"`
	Market       *ManagedTokenMarket `json:"market"`
	Pairs        *[]TokenPair        `json:"pairs"`
	Links        *[]Link             `json:"links"`
}

type managedListItemPatch struct {
	Token          *managedTokenPatch `json:"token"`
	Slot           *string            `json:"slot"`
	Rank           *int               `json:"rank"`
	Enabled        *bool              `json:"enabled"`
	Display        *bool              `json:"display"`
	DisplayName    *string            `json:"displayName"`
	DisplaySymbol  *string            `json:"displaySymbol"`
	DisplayLogoURI *string            `json:"displayLogoURI"`
}

func (s *Server) handleCreateListItem(w http.ResponseWriter, r *http.Request, listKey string) {
	if _, err := s.lists.GetList(listKey); err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	request := managedListItemWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Token == nil {
		writeJSONError(w, http.StatusBadRequest, "token is required")
		return
	}
	input := ManagedListItem{Token: *request.Token, Enabled: true, Display: true}
	applyListItemWrite(request, &input)
	if _, err := s.lists.GetItem(listKey, input.Token.Chain, input.Token.Address); err == nil {
		writeJSONResultOrErrorStatus(w, nil, conflict("list item already exists"), http.StatusCreated)
		return
	} else if !isNotFoundError(err) {
		writeJSONResultOrError(w, nil, err)
		return
	}
	item, err := s.lists.UpsertItem(listKey, input)
	if err == nil {
		w.Header().Set("Location", listItemLocation(listKey, item.Token.Chain, item.Token.Address))
	}
	writeJSONResultOrErrorStatus(w, item, err, http.StatusCreated)
}

func (s *Server) handleGetListItem(w http.ResponseWriter, listKey, chain, address string) {
	item, err := s.lists.GetItem(listKey, chain, pathAddressToTokenAddress(address))
	writeJSONResultOrError(w, item, err)
}

func (s *Server) handleReplaceListItem(w http.ResponseWriter, r *http.Request, listKey, chain, address string) {
	request := managedListItemWriteRequest{}
	if err := decodeHTTPJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if request.Token == nil {
		writeJSONError(w, http.StatusBadRequest, "token is required")
		return
	}
	pathAddress := pathAddressToTokenAddress(address)
	if err := validateItemIdentity(*request.Token, chain, pathAddress); err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	_, getErr := s.lists.GetItem(listKey, chain, pathAddress)
	status := http.StatusOK
	if isNotFoundError(getErr) {
		status = http.StatusCreated
	} else if getErr != nil {
		writeJSONResultOrError(w, nil, getErr)
		return
	}
	input := ManagedListItem{Token: *request.Token}
	input.Token.Chain = chain
	input.Token.Address = pathAddress
	if pathAddress == "" && input.Token.Kind == "" {
		input.Token.Kind = "native"
	}
	applyListItemWrite(request, &input)
	item, err := s.lists.SaveItem(listKey, input)
	if err == nil && status == http.StatusCreated {
		w.Header().Set("Location", listItemLocation(listKey, item.Token.Chain, item.Token.Address))
	}
	writeJSONResultOrErrorStatus(w, item, err, status)
}

func (s *Server) handlePatchListItem(w http.ResponseWriter, r *http.Request, listKey, chain, address string) {
	pathAddress := pathAddressToTokenAddress(address)
	item, err := s.lists.GetItem(listKey, chain, pathAddress)
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	patch := managedListItemPatch{}
	if err := decodeHTTPJSON(r, &patch); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if patch.empty() {
		writeJSONError(w, http.StatusBadRequest, "at least one mutable item field is required")
		return
	}
	if err := applyListItemPatch(patch, item, chain, pathAddress); err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	updated, err := s.lists.SaveItem(listKey, *item)
	writeJSONResultOrError(w, updated, err)
}

func (s *Server) handleDeleteListItem(w http.ResponseWriter, listKey, chain, address string) {
	err := s.lists.DeleteItem(listKey, chain, pathAddressToTokenAddress(address))
	writeNoContentOrError(w, err)
}

func applyListItemWrite(request managedListItemWriteRequest, item *ManagedListItem) {
	if request.Slot != nil {
		item.Slot = strings.TrimSpace(*request.Slot)
	}
	if request.Rank != nil {
		item.Rank = *request.Rank
	}
	if request.Enabled != nil {
		item.Enabled = *request.Enabled
	}
	if request.Display != nil {
		item.Display = *request.Display
	}
	if request.DisplayName != nil {
		item.DisplayName = strings.TrimSpace(*request.DisplayName)
	}
	if request.DisplaySymbol != nil {
		item.DisplaySymbol = strings.TrimSpace(*request.DisplaySymbol)
	}
	if request.DisplayLogoURI != nil {
		item.DisplayLogoURI = strings.TrimSpace(*request.DisplayLogoURI)
	}
}

func (patch managedListItemPatch) empty() bool {
	return (patch.Token == nil || patch.Token.empty()) && patch.Slot == nil && patch.Rank == nil && patch.Enabled == nil && patch.Display == nil && patch.DisplayName == nil && patch.DisplaySymbol == nil && patch.DisplayLogoURI == nil
}

func (patch managedTokenPatch) empty() bool {
	return patch.Kind == nil && patch.Chain == nil && patch.ChainName == nil && patch.ChainID == nil && patch.ChainLogoURI == nil && patch.Address == nil && patch.AssetID == nil && patch.Type == nil && patch.Name == nil && patch.Symbol == nil && patch.Decimals == nil && patch.Status == nil && patch.LogoURI == nil && patch.LogoExists == nil && patch.Explorer == nil && patch.Tags == nil && patch.Hot == nil && patch.Market == nil && patch.Pairs == nil && patch.Links == nil
}

func applyListItemPatch(patch managedListItemPatch, item *ManagedListItem, chain, address string) error {
	if patch.Token != nil {
		if patch.Token.Chain != nil && normalizeChain(*patch.Token.Chain) != normalizeChain(chain) {
			return invalidParams("token chain is immutable and must match the path")
		}
		if patch.Token.Address != nil && !strings.EqualFold(strings.TrimSpace(*patch.Token.Address), address) {
			return invalidParams("token address is immutable and must match the path")
		}
		applyManagedTokenPatch(*patch.Token, &item.Token)
	}
	if patch.Slot != nil {
		item.Slot = strings.TrimSpace(*patch.Slot)
	}
	if patch.Rank != nil {
		item.Rank = *patch.Rank
	}
	if patch.Enabled != nil {
		item.Enabled = *patch.Enabled
	}
	if patch.Display != nil {
		item.Display = *patch.Display
	}
	if patch.DisplayName != nil {
		item.DisplayName = strings.TrimSpace(*patch.DisplayName)
	}
	if patch.DisplaySymbol != nil {
		item.DisplaySymbol = strings.TrimSpace(*patch.DisplaySymbol)
	}
	if patch.DisplayLogoURI != nil {
		item.DisplayLogoURI = strings.TrimSpace(*patch.DisplayLogoURI)
	}
	return nil
}

func applyManagedTokenPatch(patch managedTokenPatch, token *ManagedToken) {
	if patch.Kind != nil {
		token.Kind = *patch.Kind
	}
	if patch.ChainName != nil {
		token.ChainName = *patch.ChainName
	}
	if patch.ChainID != nil {
		token.ChainID = *patch.ChainID
	}
	if patch.ChainLogoURI != nil {
		token.ChainLogoURI = *patch.ChainLogoURI
	}
	if patch.AssetID != nil {
		token.AssetID = *patch.AssetID
	}
	if patch.Type != nil {
		token.Type = *patch.Type
	}
	if patch.Name != nil {
		token.Name = *patch.Name
	}
	if patch.Symbol != nil {
		token.Symbol = *patch.Symbol
	}
	if patch.Decimals != nil {
		token.Decimals = *patch.Decimals
	}
	if patch.Status != nil {
		token.Status = *patch.Status
	}
	if patch.LogoURI != nil {
		token.LogoURI = *patch.LogoURI
	}
	if patch.LogoExists != nil {
		token.LogoExists = *patch.LogoExists
	}
	if patch.Explorer != nil {
		token.Explorer = *patch.Explorer
	}
	if patch.Tags != nil {
		token.Tags = *patch.Tags
	}
	if patch.Hot != nil {
		token.Hot = *patch.Hot
	}
	if patch.Market != nil {
		token.Market = patch.Market
	}
	if patch.Pairs != nil {
		token.Pairs = *patch.Pairs
	}
	if patch.Links != nil {
		token.Links = *patch.Links
	}
}

func validateItemIdentity(token ManagedToken, chain, address string) error {
	if token.Chain != "" && normalizeChain(token.Chain) != normalizeChain(chain) {
		return invalidParams("token chain must match the path")
	}
	if token.Address != "" && !strings.EqualFold(strings.TrimSpace(token.Address), address) {
		return invalidParams("token address must match the path")
	}
	return nil
}

func listItemLocation(listKey, chain, address string) string {
	if address == "" {
		address = "native"
	}
	return "/api/lists/" + normalizeListKey(listKey) + "/items/" + normalizeChain(chain) + "/" + address
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
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return fmt.Errorf("Content-Type must be application/json")
	}
	limited := io.LimitReader(r.Body, maxRequestBodyBytes+1)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("request body must contain exactly one JSON object")
	}
	if counter, ok := limited.(*io.LimitedReader); ok && counter.N == 0 {
		return fmt.Errorf("request body too large")
	}
	return nil
}

func writeJSONResultOrError(w http.ResponseWriter, result any, err error) {
	writeJSONResultOrErrorStatus(w, result, err, http.StatusOK)
}

func writeJSONResultOrErrorStatus(w http.ResponseWriter, result any, err error, successStatus int) {
	if err != nil {
		status := http.StatusInternalServerError
		var rpcErr *RPCError
		if errors.As(err, &rpcErr) {
			switch rpcErr.Code {
			case ErrCodeInvalidParams:
				status = http.StatusBadRequest
			case ErrCodeNotFound:
				status = http.StatusNotFound
			case ErrCodeConflict:
				status = http.StatusConflict
			}
			writeJSONError(w, status, rpcErr.Message)
			return
		}
		writeJSONError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(successStatus)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		return
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
		"code":    httpErrorCode(status),
		"message": message,
	}})
}

func writeNoContentOrError(w http.ResponseWriter, err error) {
	if err != nil {
		writeJSONResultOrError(w, nil, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeMethodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func httpErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	default:
		return "internal_error"
	}
}

func isNotFoundError(err error) bool {
	var rpcErr *RPCError
	return errors.As(err, &rpcErr) && rpcErr.Code == ErrCodeNotFound
}

func managedFilesHandler(filesDir string) http.Handler {
	_ = mime.AddExtensionType(".zst", "application/zstd")
	return http.StripPrefix("/files/", http.FileServer(http.Dir(filepath.Clean(filesDir))))
}
