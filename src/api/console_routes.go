package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rinbarpen/llm-router/src/schemas"
	"github.com/rinbarpen/llm-router/src/services"
)

const consoleSessionCookieName = "console_session"

type consoleAuthService interface {
	ConsolePasswordLogin(ctx context.Context, email string, password string, remoteAddr string, userAgent string) (schemas.ConsoleSession, error)
	GetConsoleSession(ctx context.Context, token string) (schemas.ConsoleSession, error)
	DeleteConsoleSession(ctx context.Context, token string) error
}

type consoleUserService interface {
	ListConsoleUsers(ctx context.Context) ([]schemas.User, error)
	UpdateConsoleUser(ctx context.Context, id int64, in schemas.UserUpdate) (schemas.User, error)
}

type consoleTeamService interface {
	ListConsoleTeams(ctx context.Context) ([]schemas.Team, error)
	CreateConsoleTeam(ctx context.Context, in schemas.TeamCreate, ownerUserID *int64) (schemas.Team, error)
	ListTeamMembers(ctx context.Context, teamID int64) ([]schemas.TeamMember, error)
	AddTeamMember(ctx context.Context, teamID int64, in schemas.TeamMemberCreate, invitedByUserID *int64) (schemas.TeamMember, error)
	UpdateTeamMember(ctx context.Context, teamID int64, userID int64, in schemas.TeamMemberUpdate) (schemas.TeamMember, error)
	CreateTeamInvite(ctx context.Context, teamID int64, in schemas.TeamInviteCreate, invitedByUserID *int64) (schemas.TeamInvite, error)
	ListTeamInvites(ctx context.Context, teamID int64) ([]schemas.TeamInvite, error)
	AcceptTeamInvite(ctx context.Context, inviteToken string, userID int64) (schemas.TeamMember, error)
}

type consoleWalletService interface {
	GetWalletSummary(ctx context.Context, ownerType string, ownerID int64) (schemas.Wallet, error)
}

type consoleAPIKeyService interface {
	ListAPIKeys(ctx context.Context, includeInactive bool) ([]schemas.APIKey, error)
	CreateAPIKey(ctx context.Context, in schemas.APIKeyCreate) (schemas.APIKey, error)
	GetAPIKey(ctx context.Context, id int64) (schemas.APIKey, error)
	UpdateAPIKey(ctx context.Context, id int64, in schemas.APIKeyUpdate) (schemas.APIKey, error)
	DeleteAPIKey(ctx context.Context, id int64) error
}

type consoleOrderService interface {
	ListRechargeOrders(ctx context.Context, ownerType string, ownerID int64) ([]schemas.RechargeOrder, error)
	CreateRechargeOrder(ctx context.Context, ownerType string, ownerID int64, amount float64, currency string, provider string, createdByUserID *int64) (schemas.RechargeOrder, map[string]any, error)
	GetRechargeOrder(ctx context.Context, orderNo string) (schemas.RechargeOrder, error)
	MarkRechargeOrderPaid(ctx context.Context, provider string, eventID string, orderNo string, providerTradeNo string, payload map[string]any) (schemas.RechargeOrder, bool, error)
}

func registerConsoleRoutes(r chi.Router, svc CatalogService) {
	r.Route("/console", func(r chi.Router) {
		r.Post("/auth/login", func(w http.ResponseWriter, req *http.Request) {
			authSvc, ok := svc.(consoleAuthService)
			if !ok {
				writeJSONError(w, http.StatusNotImplemented, "console auth unavailable")
				return
			}
			var payload struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				writeJSONError(w, http.StatusBadRequest, "invalid json body")
				return
			}
			session, err := authSvc.ConsolePasswordLogin(req.Context(), payload.Email, payload.Password, req.RemoteAddr, req.UserAgent())
			if err != nil {
				status := http.StatusBadGateway
				if errors.Is(err, services.ErrUnauthorized) {
					status = http.StatusUnauthorized
				}
				writeJSONError(w, status, err.Error())
				return
			}
			writeConsoleCookie(w, session)
			writeJSON(w, http.StatusOK, session)
		})
		r.Group(func(r chi.Router) {
			r.Use(consoleSessionMiddleware(svc))
			r.Get("/auth/me", func(w http.ResponseWriter, req *http.Request) {
				session, ok := getConsoleSession(req.Context())
				if !ok {
					writeJSONError(w, http.StatusUnauthorized, "console session required")
					return
				}
				writeJSON(w, http.StatusOK, session)
			})
			r.Post("/auth/logout", func(w http.ResponseWriter, req *http.Request) {
				authSvc, ok := svc.(consoleAuthService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console auth unavailable")
					return
				}
				token, _ := readConsoleCookie(req)
				_ = authSvc.DeleteConsoleSession(req.Context(), token)
				clearConsoleCookie(w)
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			})
			r.Post("/invites/accept", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				var payload struct {
					InviteToken string `json:"invite_token"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				item, err := teamSvc.AcceptTeamInvite(req.Context(), payload.InviteToken, session.User.ID)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Get("/users", func(w http.ResponseWriter, req *http.Request) {
				userSvc, ok := svc.(consoleUserService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console users unavailable")
					return
				}
				items, err := userSvc.ListConsoleUsers(req.Context())
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
			})
			r.Patch("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
				userSvc, ok := svc.(consoleUserService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console users unavailable")
					return
				}
				id, err := parseIDParam(req, "id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid id")
					return
				}
				var payload schemas.UserUpdate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := userSvc.UpdateConsoleUser(req.Context(), id, payload)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Get("/teams", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				items, err := teamSvc.ListConsoleTeams(req.Context())
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
			})
			r.Post("/teams", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				var payload schemas.TeamCreate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				item, err := teamSvc.CreateConsoleTeam(req.Context(), payload, &session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, item)
			})
			r.Get("/teams/{team_id}/members", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				items, err := teamSvc.ListTeamMembers(req.Context(), teamID)
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
			})
			r.Post("/teams/{team_id}/members", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				var payload schemas.TeamMemberCreate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				item, err := teamSvc.AddTeamMember(req.Context(), teamID, payload, &session.User.ID)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, item)
			})
			r.Patch("/teams/{team_id}/members/{user_id}", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				userID, err := parseIDParam(req, "user_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid user_id")
					return
				}
				var payload schemas.TeamMemberUpdate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := teamSvc.UpdateTeamMember(req.Context(), teamID, userID, payload)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Get("/teams/{team_id}/invites", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				items, err := teamSvc.ListTeamInvites(req.Context(), teamID)
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
			})
			r.Post("/teams/{team_id}/invites", func(w http.ResponseWriter, req *http.Request) {
				teamSvc, ok := svc.(consoleTeamService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console teams unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				var payload schemas.TeamInviteCreate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				item, err := teamSvc.CreateTeamInvite(req.Context(), teamID, payload, &session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, item)
			})
			r.Get("/wallets/me", func(w http.ResponseWriter, req *http.Request) {
				walletSvc, ok := svc.(consoleWalletService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console wallets unavailable")
					return
				}
				session, _ := getConsoleSession(req.Context())
				item, err := walletSvc.GetWalletSummary(req.Context(), "user", session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Get("/wallets/teams/{team_id}", func(w http.ResponseWriter, req *http.Request) {
				walletSvc, ok := svc.(consoleWalletService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console wallets unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				item, err := walletSvc.GetWalletSummary(req.Context(), "team", teamID)
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Get("/api-keys", func(w http.ResponseWriter, req *http.Request) {
				apiKeySvc, ok := svc.(consoleAPIKeyService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console api keys unavailable")
					return
				}
				items, err := apiKeySvc.ListAPIKeys(req.Context(), strings.EqualFold(strings.TrimSpace(req.URL.Query().Get("include_inactive")), "true"))
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, items)
			})
			r.Post("/api-keys", func(w http.ResponseWriter, req *http.Request) {
				apiKeySvc, ok := svc.(consoleAPIKeyService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console api keys unavailable")
					return
				}
				var payload schemas.APIKeyCreate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := apiKeySvc.CreateAPIKey(req.Context(), payload)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, item)
			})
			r.Get("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
				apiKeySvc, ok := svc.(consoleAPIKeyService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console api keys unavailable")
					return
				}
				id, err := parseIDParam(req, "id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid id")
					return
				}
				item, err := apiKeySvc.GetAPIKey(req.Context(), id)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Patch("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
				apiKeySvc, ok := svc.(consoleAPIKeyService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console api keys unavailable")
					return
				}
				id, err := parseIDParam(req, "id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid id")
					return
				}
				var payload schemas.APIKeyUpdate
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				item, err := apiKeySvc.UpdateAPIKey(req.Context(), id, payload)
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, item)
			})
			r.Delete("/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
				apiKeySvc, ok := svc.(consoleAPIKeyService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console api keys unavailable")
					return
				}
				id, err := parseIDParam(req, "id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid id")
					return
				}
				if err := apiKeySvc.DeleteAPIKey(req.Context(), id); err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"ok": true})
			})
			r.Get("/orders", func(w http.ResponseWriter, req *http.Request) {
				orderSvc, ok := svc.(consoleOrderService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console orders unavailable")
					return
				}
				session, _ := getConsoleSession(req.Context())
				items, err := orderSvc.ListRechargeOrders(req.Context(), "user", session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadGateway, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, map[string]any{"items": items})
			})
			r.Post("/orders/recharge", func(w http.ResponseWriter, req *http.Request) {
				orderSvc, ok := svc.(consoleOrderService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console orders unavailable")
					return
				}
				var payload struct {
					Amount          float64 `json:"amount"`
					Currency        string  `json:"currency"`
					PaymentProvider string  `json:"payment_provider"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				order, checkout, err := orderSvc.CreateRechargeOrder(req.Context(), "user", session.User.ID, payload.Amount, payload.Currency, payload.PaymentProvider, &session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, map[string]any{"order": order, "checkout": checkout})
			})
			r.Post("/teams/{team_id}/orders/recharge", func(w http.ResponseWriter, req *http.Request) {
				orderSvc, ok := svc.(consoleOrderService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console orders unavailable")
					return
				}
				teamID, err := parseIDParam(req, "team_id")
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid team_id")
					return
				}
				var payload struct {
					Amount          float64 `json:"amount"`
					Currency        string  `json:"currency"`
					PaymentProvider string  `json:"payment_provider"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					writeJSONError(w, http.StatusBadRequest, "invalid json body")
					return
				}
				session, _ := getConsoleSession(req.Context())
				order, checkout, err := orderSvc.CreateRechargeOrder(req.Context(), "team", teamID, payload.Amount, payload.Currency, payload.PaymentProvider, &session.User.ID)
				if err != nil {
					writeJSONError(w, http.StatusBadRequest, err.Error())
					return
				}
				writeJSON(w, http.StatusCreated, map[string]any{"order": order, "checkout": checkout})
			})
			r.Get("/orders/{order_no}", func(w http.ResponseWriter, req *http.Request) {
				orderSvc, ok := svc.(consoleOrderService)
				if !ok {
					writeJSONError(w, http.StatusNotImplemented, "console orders unavailable")
					return
				}
				order, err := orderSvc.GetRechargeOrder(req.Context(), chi.URLParam(req, "order_no"))
				if err != nil {
					status := http.StatusBadGateway
					if errors.Is(err, services.ErrNotFound) {
						status = http.StatusNotFound
					}
					writeJSONError(w, status, err.Error())
					return
				}
				writeJSON(w, http.StatusOK, order)
			})
		})
	})
	r.Post("/payments/callback/{provider}", func(w http.ResponseWriter, req *http.Request) {
		orderSvc, ok := svc.(consoleOrderService)
		if !ok {
			writeJSONError(w, http.StatusNotImplemented, "payments unavailable")
			return
		}
		payload, err := readJSONBody(req, true)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		orderNo, _ := payload["order_no"].(string)
		eventID, _ := payload["event_id"].(string)
		tradeNo, _ := payload["provider_trade_no"].(string)
		order, applied, err := orderSvc.MarkRechargeOrderPaid(req.Context(), chi.URLParam(req, "provider"), eventID, orderNo, tradeNo, payload)
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, services.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeJSONError(w, status, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"order": order, "applied": applied})
	})
}

type consoleSessionContextKey struct{}

func consoleSessionMiddleware(svc CatalogService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			authSvc, ok := svc.(consoleAuthService)
			if !ok {
				writeJSONError(w, http.StatusNotImplemented, "console auth unavailable")
				return
			}
			token, ok := readConsoleCookie(req)
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "console session required")
				return
			}
			session, err := authSvc.GetConsoleSession(req.Context(), token)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid console session")
				return
			}
			ctx := context.WithValue(req.Context(), consoleSessionContextKey{}, session)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	}
}

func getConsoleSession(ctx context.Context) (schemas.ConsoleSession, bool) {
	item, ok := ctx.Value(consoleSessionContextKey{}).(schemas.ConsoleSession)
	return item, ok
}

func readConsoleCookie(req *http.Request) (string, bool) {
	cookie, err := req.Cookie(consoleSessionCookieName)
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(cookie.Value)
	return token, token != ""
}

func writeConsoleCookie(w http.ResponseWriter, session schemas.ConsoleSession) {
	expiresAt := time.Now().Add(services.ConsoleSessionTTL)
	if session.ExpiresAt != nil {
		expiresAt = *session.ExpiresAt
	}
	http.SetCookie(w, &http.Cookie{
		Name:     consoleSessionCookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
}

func clearConsoleCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     consoleSessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
	})
}
