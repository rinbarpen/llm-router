package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rinbarpen/llm-router/backend/internal/services"
)

func registerGeminiRoutes(r chi.Router, svc CatalogService) {
	r.Post("/v1beta/models/{model}:generateContent", geminiGenerateContentHandler(svc))
	r.Post("/v1beta/models/{model}:streamGenerateContent", geminiStreamGenerateContentHandler(svc))
}

func geminiGenerateContentHandler(svc CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if svc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "catalog service unavailable")
			return
		}
		modelName := chi.URLParam(req, "model")
		if modelName == "" {
			writeJSONError(w, http.StatusBadRequest, "model is required")
			return
		}
		payload, err := readJSONBody(req, false)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		out, err := svc.GeminiGenerateContent(req.Context(), modelName, payload)
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				writeJSONError(w, http.StatusNotFound, "model not found")
				return
			}
			if errors.Is(err, services.ErrNotImplemented) {
				writeJSONError(w, http.StatusNotImplemented, err.Error())
				return
			}
			writeJSONError(w, http.StatusBadGateway, "gemini generate content failed")
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func geminiStreamGenerateContentHandler(_ CatalogService) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSONError(w, http.StatusNotImplemented, "gemini streamGenerateContent not implemented yet in Go backend")
	}
}
