package auth

import (
	"api/internal/domain/tokens"
	"api/internal/platform/cookies"
	"api/internal/platform/request"
	"api/internal/platform/response"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
	"github.com/markbates/goth/gothic"
)

type Handler struct {
	auth             *Service
	response         *response.Response
	cookies          *cookies.Manager
	logger           *slog.Logger
	redirectURL      string
	allowedProviders []string
}

func NewHandler(service *Service, response *response.Response, cookies *cookies.Manager, logger *slog.Logger, redirectURL string, allowedProviders []string) *Handler {
	return &Handler{
		auth:             service,
		response:         response,
		cookies:          cookies,
		logger:           logger,
		redirectURL:      redirectURL,
		allowedProviders: allowedProviders,
	}
}

type ctxKey string

const providerKey ctxKey = "provider"

/* OAuth provider handler (redirects to provider's auth form) */
func (h *Handler) OAuthProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	// Check if provider available
	if !slices.Contains(h.allowedProviders, provider) {
		h.response.Error(w, r, response.ErrBadProvider)
		return
	}
	// Put provider to params
	r = r.WithContext(context.WithValue(r.Context(), providerKey, provider))
	// Begin authentication process
	gothic.BeginAuthHandler(w, r)
}

/* OAuth callback handler */
func (h *Handler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	// Check if provider available
	if !slices.Contains(h.allowedProviders, provider) {
		h.response.Error(w, r, response.ErrBadProvider)
		return
	}
	// Put provider to params
	r = r.WithContext(context.WithValue(r.Context(), providerKey, provider))
	// Complete auth process
	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Upsert confirmed user to database
	user, err := h.auth.UpsertOAuthUser(r.Context(), gothUser)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Get metadata for refresh token
	meta := request.GetMetaFromRequest(r)
	// Generate access and refresh tokens
	access, refresh, err := h.auth.IssueSessionTokens(r.Context(), user, meta)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	// Set token cookies
	h.cookies.SetAccessCookie(w, access)
	h.cookies.SetRefreshCookie(w, refresh)
	// Redirect to frontend
	http.Redirect(w, r, h.redirectURL, http.StatusTemporaryRedirect)
}

/* Logout from current device */
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get old refresh token
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" {
		// Consume old refresh token
		err = h.auth.ConsumeRefreshToken(r.Context(), cookie.Value)
		if err != nil {
			h.logger.Warn("Logout: old refresh token consumption error", "error", err)
		}
	}

	// Delete cookies
	h.cookies.UnsetAccessCookie(w)
	h.cookies.UnsetRefreshCookie(w)

	h.response.Success(w, r, response.SuccessLoggedOut)
}

/* Logout from other devices */
func (h *Handler) TerminateOtherSessions(w http.ResponseWriter, r *http.Request) {
	// Get claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Get current refresh token
	cookie, err := r.Cookie("refresh_token")
	if err != nil || cookie.Value == "" {
		h.response.Error(w, r, response.ErrNoRefreshToken.Wrap(err))
		return
	}
	refresh := cookie.Value

	// Consume other refresh tokens (exit from other devices)
	h.auth.ConsumeOtherRefreshTokens(r.Context(), claims.UserID, refresh)

	h.response.Success(w, r, response.SuccessLoggedOutOthers)
}

/* Exchange old refresh token to new pair of tokens */
func (h *Handler) RefreshSession(w http.ResponseWriter, r *http.Request) {
	// Get old refresh token
	refreshCookie, err := r.Cookie("refresh_token")
	if err != nil {
		// Delete all cookies
		h.cookies.UnsetAccessCookie(w)
		h.cookies.UnsetRefreshCookie(w)
		h.response.Error(w, r, response.ErrNoRefreshToken.Wrap(err))
		return
	}
	oldRefreshToken := refreshCookie.Value

	// Get metadata for refresh token
	meta := request.GetMetaFromRequest(r)
	// Exchange old refresh token
	newAccess, newRefresh, err := h.auth.RotateRefreshToken(r.Context(), oldRefreshToken, meta)
	if err != nil {
		if errors.Is(err, response.ErrTokenGracePeriod) {
			// Refresh attempt in grace period, cookies was set in prev request
			w.WriteHeader(http.StatusOK)
			return
		}

		// Delete all cookies
		h.cookies.UnsetAccessCookie(w)
		h.cookies.UnsetRefreshCookie(w)
		h.response.Error(w, r, err)
		return
	}

	// Set cookies
	h.cookies.SetAccessCookie(w, newAccess)
	h.cookies.SetRefreshCookie(w, newRefresh)

	// Return 200 OK
	w.WriteHeader(http.StatusOK)
}
