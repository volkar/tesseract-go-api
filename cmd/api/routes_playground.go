package main

import (
	"api/internal/domain/albums"
	"api/internal/domain/shared/types"
	"api/internal/platform/request"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

/* Routes for development, must be deleted in production */
func (app *app) RoutesPlayground(r *chi.Mux) {
	r.Route("/playground", func(r chi.Router) {
		// Create admin with albums
		r.Get("/create_admin", app.PlaygroundCreateAdmin)
		// Create user with albums
		r.Get("/create_user", app.PlaygroundCreateUser)
		// Create new refresh token and set both access and refresh tokens as cookie for user
		r.Get("/get_user_cookies", app.PlaygroundGetUserCookies)
		// Create new refresh token and set both access and refresh tokens as cookie for admin
		r.Get("/get_admin_cookies", app.PlaygroundGetAdminCookies)
		// Clear token cookies
		r.Get("/clear_cookies", app.PlaygroundClearCookies)
		// Clear redis cache
		r.Get("/clear_cache", app.PlaygroundClearCache)
	})
}

/* Creates admin with 4 albums */
func (app *app) PlaygroundCreateAdmin(w http.ResponseWriter, r *http.Request) {
	admin, err := app.usersService.Create(r.Context(), "admin@test.test", "Almighty Admin", "admin", "", types.RoleAdmin)
	if err != nil {
		// User may be already created. Ignore
		app.response.SuccessDataOnly(w, r, map[string]string{
			"playground": "already created",
		})
		return
	}

	atlas := GetDefaultAtlas()

	app.albumsService.Create(r.Context(), admin.ID, albums.CreateRequest{Title: "Admin private album", Slug: "private", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-1.jpg", Atlas: atlas, Access: types.AccessPrivate, SharedEmails: []string{}, IsActive: true, DateAt: time.Date(2021, 3, 1, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), admin.ID, albums.CreateRequest{Title: "Admin public album", Slug: "public", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-2.jpg", Atlas: atlas, Access: types.AccessPublic, SharedEmails: []string{}, IsActive: true, DateAt: time.Date(2022, 5, 13, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), admin.ID, albums.CreateRequest{Title: "Admin shared album", Slug: "shared", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-3.jpg", Atlas: atlas, Access: types.AccessShared, SharedEmails: []string{"user@test.test"}, IsActive: true, DateAt: time.Date(2023, 7, 22, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), admin.ID, albums.CreateRequest{Title: "Admin inactive album", Slug: "inactive", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-14.jpg", Atlas: atlas, Access: types.AccessPublic, SharedEmails: []string{}, IsActive: false, DateAt: time.Date(2024, 9, 1, 12, 30, 0, 0, time.UTC)})

	app.response.SuccessDataOnly(w, r, map[string]string{
		"playground": "admin created",
	})
}

/* Creates user with 4 albums */
func (app *app) PlaygroundCreateUser(w http.ResponseWriter, r *http.Request) {
	user, err := app.usersService.Create(r.Context(), "user@test.test", "Just User", "user", "", types.RoleUser)
	if err != nil {
		// User may be already created. Ignore
		app.response.SuccessDataOnly(w, r, map[string]string{
			"playground": "already created",
		})
		return
	}

	// Create user albums
	atlas := GetDefaultAtlas()

	app.albumsService.Create(r.Context(), user.ID, albums.CreateRequest{Title: "User private album", Slug: "private", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-4.jpg", Atlas: atlas, Access: types.AccessPrivate, SharedEmails: []string{}, IsActive: true, DateAt: time.Date(2021, 3, 1, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), user.ID, albums.CreateRequest{Title: "User public album", Slug: "public", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-5.jpg", Atlas: atlas, Access: types.AccessPublic, SharedEmails: []string{}, IsActive: true, DateAt: time.Date(2022, 5, 13, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), user.ID, albums.CreateRequest{Title: "User shared album", Slug: "shared", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-6.jpg", Atlas: atlas, Access: types.AccessShared, SharedEmails: []string{"admin@test.test"}, IsActive: true, DateAt: time.Date(2023, 7, 22, 12, 30, 0, 0, time.UTC)})
	app.albumsService.Create(r.Context(), user.ID, albums.CreateRequest{Title: "User inactive album", Slug: "inactive", Cover: "https://tesseract.syntheticsymbiosis.com/static/tesseract-12.jpg", Atlas: atlas, Access: types.AccessPublic, SharedEmails: []string{}, IsActive: false, DateAt: time.Date(2024, 9, 1, 12, 30, 0, 0, time.UTC)})

	app.response.SuccessDataOnly(w, r, map[string]string{
		"playground": "user created",
	})
}

/* Set access and refresh cookie for user */
func (app *app) PlaygroundGetUserCookies(w http.ResponseWriter, r *http.Request) {
	u, err := app.usersService.GetAvailableBySlug(r.Context(), "user")
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"user_tokens": "error, user not found",
		})
		return
	}

	// Get metadata for refresh token
	meta := request.GetMetaFromRequest(r)

	// Create new refresh token
	refresh, err := app.tokens.GenerateRefreshString()
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"user_tokens": "error, failed to generate refresh token",
		})
		return
	}
	hash := app.tokens.Hash(refresh)
	_, err = app.tokens.CreateRefresh(r.Context(), u.ID, hash, time.Now().Add(app.cfg.Paseto.RefreshTTL), meta)
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"user_tokens": "error, failed to create refresh token",
		})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   app.cfg.Cookie.Secure,
		SameSite: app.cfg.Cookie.SameSite,
		MaxAge:   int(app.cfg.Paseto.RefreshTTL.Seconds()),
	})

	// Create access token
	access := app.tokens.CreateAccess(u.ID, u.Role, u.Email, app.cfg.Paseto.AccessTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   app.cfg.Cookie.Secure,
		SameSite: app.cfg.Cookie.SameSite,
		MaxAge:   int(app.cfg.Paseto.RefreshTTL.Seconds()),
	})

	app.response.SuccessDataOnly(w, r, map[string]string{
		"user_tokens": "set",
	})
}

/* Set access and refresh cookie for admin */
func (app *app) PlaygroundGetAdminCookies(w http.ResponseWriter, r *http.Request) {
	u, err := app.usersService.GetAvailableBySlug(r.Context(), "admin")
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"admin_tokens": "error, admin not found",
		})
		return
	}

	// Get metadata for refresh token
	meta := request.GetMetaFromRequest(r)

	// Create new refresh token
	refresh, err := app.tokens.GenerateRefreshString()
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"admin_tokens": "error, failed to generate refresh token",
		})
		return
	}
	hash := app.tokens.Hash(refresh)
	_, err = app.tokens.CreateRefresh(r.Context(), u.ID, hash, time.Now().Add(app.cfg.Paseto.RefreshTTL), meta)
	if err != nil {
		app.response.SuccessDataOnly(w, r, map[string]string{
			"admin_tokens": "error, failed to create refresh token",
		})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refresh,
		Path:     "/",
		HttpOnly: true,
		Secure:   app.cfg.Cookie.Secure,
		SameSite: app.cfg.Cookie.SameSite,
		MaxAge:   int(app.cfg.Paseto.RefreshTTL.Seconds()),
	})

	// Create access token
	access := app.tokens.CreateAccess(u.ID, u.Role, u.Email, app.cfg.Paseto.AccessTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    access,
		Path:     "/",
		HttpOnly: true,
		Secure:   app.cfg.Cookie.Secure,
		SameSite: app.cfg.Cookie.SameSite,
		MaxAge:   int(app.cfg.Paseto.RefreshTTL.Seconds()),
	})

	app.response.SuccessDataOnly(w, r, map[string]string{
		"admin_tokens": "set",
	})
}

/* Clears current access and refresh token cookies */
func (app *app) PlaygroundClearCookies(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	app.response.SuccessDataOnly(w, r, map[string]string{
		"cookies": "clear",
	})
}

/* Clears redis cache */
func (app *app) PlaygroundClearCache(w http.ResponseWriter, r *http.Request) {
	app.cache.ClearFullCache(r.Context())

	app.response.SuccessDataOnly(w, r, map[string]string{
		"cache": "clear",
	})
}

func GetDefaultAtlas() []types.AtlasItem {
	return []types.AtlasItem{
		{
			Type: "title",
			Src:  "The busy bee has no time for sorrow.",
		},
		{
			Type: "text",
			Src:  "The woods are lovely, dark and deep, But I have promises to keep, And miles to go before I sleep, And miles to go before I sleep.",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-1.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1067},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-2.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1068},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-14.jpg",
			Meta: types.AtlasItemMeta{Width: 1069, Height: 1600},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-4.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1067},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-5.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1067},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-15.jpg",
			Meta: types.AtlasItemMeta{Width: 1058, Height: 1600},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-7.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1202},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-8.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1068},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-9.jpg",
			Meta: types.AtlasItemMeta{Width: 1067, Height: 1600},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-11.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1067},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-10.jpg",
			Meta: types.AtlasItemMeta{Width: 1067, Height: 1600},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-12.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1068},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-3.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 1067},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-13.jpg",
			Meta: types.AtlasItemMeta{Width: 1067, Height: 1600},
			Type: "image",
		},
		{
			Src:  "https://tesseract.syntheticsymbiosis.com/static/tesseract-6.jpg",
			Meta: types.AtlasItemMeta{Width: 1600, Height: 900},
			Type: "image",
		},
	}
}
