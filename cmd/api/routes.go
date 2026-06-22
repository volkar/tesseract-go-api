package main

import (
	"api/internal/domain/shared/types"
	"api/internal/platform/ratelimit"
	"fmt"

	"github.com/go-chi/chi/v5"
)

const ProviderRegex = `[a-zA-Z0-9]{2,20}`
const SlugRegex = `[a-zA-Z0-9_-]{1,255}`
const UUIDRegex = `[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}`

func (app *app) Routes(r *chi.Mux) {
	// "Auth" rate limit
	r.Group(func(r chi.Router) {
		r.Use(app.RateLimit(ratelimit.RuleAuth))
		// Authentication routes
		r.Get(fmt.Sprintf("/auth/{provider:%s}/provider", ProviderRegex), app.authHandler.OAuthProvider)
		r.Get(fmt.Sprintf("/auth/{provider:%s}/callback", ProviderRegex), app.authHandler.OAuthCallback)
		r.Post("/auth/logout", app.authHandler.Logout)
		r.Group(func(r chi.Router) {
			r.Use(app.RequireAuthentication)
			// Auth user only
			r.Post("/auth/logout-others", app.authHandler.TerminateOtherSessions)
		})
	})

	// "Refresh" rate limit
	r.Group(func(r chi.Router) {
		r.Use(app.RateLimit(ratelimit.RuleRefresh))
		r.Post("/auth/refresh", app.authHandler.RefreshSession)
	})

	// "Get" rate limit
	r.Group(func(r chi.Router) {
		r.Use(app.RateLimit(ratelimit.RuleGet))
		// Health
		r.Get("/health", app.healthHandler)
		// Me (authenticated user)
		r.Get("/me/info", app.usersHandler.Me)
		r.Get("/me/albums", app.albumsHandler.OwnedList)
		r.Get("/me/albums/trashed", app.albumsHandler.TrashedList)
		r.Get(fmt.Sprintf("/me/album/{uuid:%s}", UUIDRegex), app.albumsHandler.GetOwned)
		// Users
		r.Get(fmt.Sprintf("/users/{slug:%s}/albums", SlugRegex), app.albumsHandler.AvailableList)
		r.Get(fmt.Sprintf("/users/{slug:%s}/info", SlugRegex), app.usersHandler.Info)
		// Albums
		r.Get(fmt.Sprintf("/albums/{user_slug:%s}/{album_slug:%s}", SlugRegex, SlugRegex), app.albumsHandler.GetAvailable)
		r.Get(fmt.Sprintf("/albums/{direct_token:%s}", UUIDRegex), app.albumsHandler.GetByDirectToken)
	})

	// "Modify" rate limit, authentication required
	r.Group(func(r chi.Router) {
		r.Use(app.RateLimit(ratelimit.RuleModify))
		r.Use(app.RequireAuthentication)
		// Users
		r.Put(fmt.Sprintf("/users/{uuid:%s}", UUIDRegex), app.usersHandler.Update)
		r.Delete(fmt.Sprintf("/users/{uuid:%s}", UUIDRegex), app.usersHandler.Delete)
		// Albums
		r.Post("/albums", app.albumsHandler.Create)
		r.Put(fmt.Sprintf("/albums/{uuid:%s}", UUIDRegex), app.albumsHandler.Update)
		r.Delete(fmt.Sprintf("/albums/{uuid:%s}", UUIDRegex), app.albumsHandler.Delete)
		r.Post(fmt.Sprintf("/albums/{uuid:%s}/direct", UUIDRegex), app.albumsHandler.GenerateDirectToken)
		r.Delete(fmt.Sprintf("/albums/{uuid:%s}/direct", UUIDRegex), app.albumsHandler.RevokeDirectToken)
		r.Put(fmt.Sprintf("/albums/{uuid:%s}/active", UUIDRegex), app.albumsHandler.ToggleActive)
		r.Post(fmt.Sprintf("/albums/{uuid:%s}/restore", UUIDRegex), app.albumsHandler.Restore)
		r.Delete(fmt.Sprintf("/albums/{uuid:%s}/purge", UUIDRegex), app.albumsHandler.Purge)
	})

	// "Admin" rate limit. Admin role required
	r.Group(func(r chi.Router) {
		r.Use(app.RateLimit(ratelimit.RuleAdmin))
		r.Use(app.RequireRole(types.RoleAdmin))
		// Admin routes
		r.Post(fmt.Sprintf("/admin/users/{uuid:%s}/restore", UUIDRegex), app.adminHandler.RestoreUser)
		r.Delete(fmt.Sprintf("/admin/users/{uuid:%s}", UUIDRegex), app.adminHandler.PurgeUser)
	})
}
