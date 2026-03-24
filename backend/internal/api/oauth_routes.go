package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type OAuthRouteService interface {
	GetAuthorizeURL(providerType string, providerName string, callbackURL string) (string, string, error)
	HandleCallback(providerType string, code string, state string) (string, error)
	HasOAuthCredential(providerName string) (bool, error)
	RevokeOAuthCredential(providerName string) (bool, error)
}

func registerOAuthRoutes(r chi.Router, oauthSvc OAuthRouteService) {
	r.Get("/auth/oauth/{provider}/authorize", oauthAuthorizeHandler(oauthSvc))
	r.Get("/auth/oauth/{provider}/callback", oauthCallbackHandler(oauthSvc))
	r.Get("/auth/oauth/{provider}/status", oauthStatusHandler(oauthSvc))
	r.Post("/auth/oauth/{provider}/revoke", oauthRevokeHandler(oauthSvc))
}

func oauthAuthorizeHandler(oauthSvc OAuthRouteService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if oauthSvc == nil {
			writeJSONError(w, http.StatusNotImplemented, "oauth service not configured")
			return
		}
		provider := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		if provider == "" {
			writeJSONError(w, http.StatusBadRequest, "provider is required")
			return
		}
		providerName := req.URL.Query().Get("provider_name")
		if strings.TrimSpace(providerName) == "" {
			providerName = provider
		}
		callbackURL := req.URL.Query().Get("callback_url")
		if strings.TrimSpace(callbackURL) == "" {
			callbackURL = strings.TrimRight(req.Host, "/")
		}
		url, state, err := oauthSvc.GetAuthorizeURL(provider, providerName, callbackURL)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"url":   url,
			"state": state,
		})
	}
}

func oauthCallbackHandler(oauthSvc OAuthRouteService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if oauthSvc == nil {
			writeJSONError(w, http.StatusNotImplemented, "oauth service not configured")
			return
		}
		provider := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		code := strings.TrimSpace(req.URL.Query().Get("code"))
		state := strings.TrimSpace(req.URL.Query().Get("state"))
		if provider == "" || code == "" || state == "" {
			writeJSONError(w, http.StatusBadRequest, "provider, code and state are required")
			return
		}
		redirectURL, err := oauthSvc.HandleCallback(provider, code, state)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if strings.TrimSpace(redirectURL) == "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			return
		}
		http.Redirect(w, req, redirectURL, http.StatusFound)
	}
}

func oauthStatusHandler(oauthSvc OAuthRouteService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if oauthSvc == nil {
			writeJSONError(w, http.StatusNotImplemented, "oauth service not configured")
			return
		}
		provider := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		providerName := req.URL.Query().Get("provider_name")
		if strings.TrimSpace(providerName) == "" {
			providerName = provider
		}
		hasCred, err := oauthSvc.HasOAuthCredential(providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"has_oauth":     hasCred,
		})
	}
}

func oauthRevokeHandler(oauthSvc OAuthRouteService) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if oauthSvc == nil {
			writeJSONError(w, http.StatusNotImplemented, "oauth service not configured")
			return
		}
		provider := strings.ToLower(strings.TrimSpace(chi.URLParam(req, "provider")))
		providerName := provider
		if payload, err := readJSONBody(req, true); err == nil {
			if value, ok := payload["provider_name"].(string); ok && strings.TrimSpace(value) != "" {
				providerName = strings.TrimSpace(value)
			}
		}
		revoked, err := oauthSvc.RevokeOAuthCredential(providerName)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"provider_name": providerName,
			"revoked":       revoked,
		})
	}
}
