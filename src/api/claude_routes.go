package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rinbarpen/llm-router/src/services"
)

func registerClaudeRoutes(r chi.Router, svc CatalogService) {
	r.Post("/v1/messages", claudeMessagesHandler(svc))
	r.Post("/v1/messages/count_tokens", claudeCountTokensHandler(svc))
	r.Post("/v1/messages/batches", claudeMessageBatchesNotImplemented)
	r.Get("/v1/messages/batches/{batch_id}", claudeMessageBatchesNotImplemented)
	r.Post("/v1/messages/batches/{batch_id}/cancel", claudeMessageBatchesNotImplemented)
}

func claudeMessagesHandler(svc CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := svc.ClaudeMessages(req.Context(), payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "claude messages failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func claudeCountTokensHandler(svc CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := svc.ClaudeCountTokens(req.Context(), payload)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, "claude count_tokens failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func claudeMessageBatchesNotImplemented(w http.ResponseWriter, _ *http.Request) {
	writeJSONError(w, http.StatusNotImplemented, "claude message batches not implemented yet in Go backend")
}
