package rpcserver

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

const maxRequestBodyBytes = 1 << 20

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
		if err != nil {
			writeRPCResponse(w, rpcResponse{
				JSONRPC: "2.0",
				Error:   internalError("failed to read request body"),
				ID:      json.RawMessage("null"),
			})
			return
		}

		body = bytes.TrimSpace(body)
		if len(body) == 0 {
			writeRPCResponse(w, rpcResponse{
				JSONRPC: "2.0",
				Error:   &RPCError{Code: ErrCodeParseError, Message: "parse error"},
				ID:      json.RawMessage("null"),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if body[0] == '[' {
			s.handleBatch(w, body)
			return
		}

		response, ok := s.handleSingle(body)
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeRPCResponse(w, response)
	})
}

func (s *Server) handleBatch(w http.ResponseWriter, body []byte) {
	var requests []json.RawMessage
	if err := json.Unmarshal(body, &requests); err != nil {
		writeRPCResponse(w, rpcResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: ErrCodeParseError, Message: "parse error"},
			ID:      json.RawMessage("null"),
		})
		return
	}
	if len(requests) == 0 {
		writeRPCResponse(w, rpcResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: ErrCodeInvalidRequest, Message: "invalid request"},
			ID:      json.RawMessage("null"),
		})
		return
	}

	responses := make([]rpcResponse, 0, len(requests))
	for _, raw := range requests {
		response, ok := s.handleSingle(raw)
		if ok {
			responses = append(responses, response)
		}
	}

	if len(responses) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := json.NewEncoder(w).Encode(responses); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (s *Server) handleSingle(body []byte) (rpcResponse, bool) {
	var request rpcRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: ErrCodeParseError, Message: "parse error"},
			ID:      json.RawMessage("null"),
		}, true
	}

	var rawObject map[string]json.RawMessage
	_ = json.Unmarshal(body, &rawObject)
	_, hasID := rawObject["id"]
	if request.ID == nil {
		request.ID = json.RawMessage("null")
	}
	isNotification := !hasID

	if request.JSONRPC != "2.0" || request.Method == "" {
		if isNotification {
			return rpcResponse{}, false
		}
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   &RPCError{Code: ErrCodeInvalidRequest, Message: "invalid request"},
			ID:      request.ID,
		}, true
	}

	result, rpcErr := s.call(request.Method, request.Params)
	if isNotification {
		return rpcResponse{}, false
	}
	if rpcErr != nil {
		return rpcResponse{
			JSONRPC: "2.0",
			Error:   rpcErr,
			ID:      request.ID,
		}, true
	}

	return rpcResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}, true
}

func writeRPCResponse(w http.ResponseWriter, response rpcResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
