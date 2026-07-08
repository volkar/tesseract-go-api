package main

import (
	"api/internal/domain/shared/types"
	"api/internal/domain/tokens"
	"api/internal/platform/i18n"
	"api/internal/platform/ratelimit"
	"api/internal/platform/response"
	"bytes"
	"crypto/sha1"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type responseRecorder struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	return r.body.Write(b)
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func (app *app) ETagChecker(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Borrow buffer from pool
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		// Defer returning the buffer to the pool
		defer bufferPool.Put(buf)

		// Response recorder
		recorder := &responseRecorder{
			ResponseWriter: w,
			body:           buf,
			status:         http.StatusOK,
		}

		// Next middleware and logic
		next.ServeHTTP(recorder, r)

		// Only success get requests
		if r.Method != http.MethodGet || recorder.status != http.StatusOK {
			w.WriteHeader(recorder.status)
			w.Write(recorder.body.Bytes())
			return
		}

		// Calculate hash from response
		data := recorder.body.Bytes()
		etag := fmt.Sprintf(`"%x"`, sha1.Sum(data))

		// Check if client sent If-None-Match header
		if r.Header.Get("If-None-Match") == etag {
			// ETag match, return 304 Not Modified header
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// No match, send data with new ETag
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(recorder.status)
		w.Write(data)
	})
}

/* Parse and validate access token and insert user claims to context middleware */
func (app *app) ParseAccessToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from cookie
		cookie, err := r.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			// No token, continue without claims (guest)
			next.ServeHTTP(w, r)
			return
		}

		// Validate token integrity and expiration
		claims, err := app.tokens.ParseAccess(cookie.Value)
		if err != nil || claims.UserID == uuid.Nil {

			// Define exceptions: routes that should gracefully ignore a dead access token
			path := r.URL.Path
			isAuthException := path == "/auth/refresh" ||
				path == "/auth/logout" ||
				(strings.HasPrefix(r.URL.Path, "/auth/") && strings.HasSuffix(path, "/provider")) ||
				(strings.HasPrefix(r.URL.Path, "/auth/") && strings.HasSuffix(path, "/callback"))

			if isAuthException {
				// Soft fail: let the request through without claims.
				// The specific handler will do its job (e.g. refresh will use refresh_token).
				next.ServeHTTP(w, r)
				return
			}

			// Hard block: unset the dead cookie and return 401 Unauthorized
			// The frontend will fetch a new token
			app.cookies.UnsetAccessCookie(w)
			app.response.Error(w, r, response.ErrAccessTokenExpired)
			return
		}

		// Insert user claims to context
		ctx := tokens.InsertClaimsToContext(r.Context(), claims)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/* Require authentication middleware. Get user claims from context */
func (app *app) RequireAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check claims in context
		claims, ok := tokens.GetClaimsFromContext(r.Context())

		if !ok || claims.UserID == uuid.Nil {
			app.response.Error(w, r, response.ErrNoClaims)
			return
		}

		next.ServeHTTP(w, r)
	})
}

/* Require role middleware. Check if user has required role */
func (app *app) RequireRole(targetRole types.Role) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims, ok := tokens.GetClaimsFromContext(r.Context())

			if !ok {
				app.response.Error(w, r, response.ErrNoClaims)
				return
			}
			if claims.Role != targetRole {
				app.response.Error(w, r, response.ErrNoPermission)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/* Get accept-language header and insert it to context middleware */
func (app *app) InsertLanguageToContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get accept-language header
		lang := r.Header.Get("Accept-Language")

		// Default API response language
		userLang := app.cfg.Lang

		// Get first two characters of language code
		if len(lang) >= 2 {
			userLang = strings.ToLower(lang[:2])
		}

		// Insert language code to context
		ctx := i18n.InsertLanguageToContext(r.Context(), userLang)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

/* Rate limiter middleware */
func (app *app) RateLimit(rule ratelimit.Rule) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Identifier. UserID or IP
			identifier := r.RemoteAddr
			if claims, ok := tokens.GetClaimsFromContext(r.Context()); ok {
				identifier = claims.UserID.String()
			}

			key := fmt.Sprintf("rate_limit:%s:%s", rule.Key, identifier)

			result, err := app.ratelimit.Allow(r.Context(), key, rule.Limit, rule.Window)
			if err != nil {
				// Redis error
				app.response.Error(w, r, err)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rule.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.Reset).Unix(), 10))

			if !result.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(int64(result.Reset.Seconds()), 10))

				app.response.Error(w, r, response.ErrTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

/* Add basic security headers middleware */
func (app *app) AddSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

/* Simple CORS header check middleware */
func (app *app) CheckXHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pass safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Custom header check. Must be sent from frontend
		if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
			app.response.Error(w, r, response.ErrNoSecurityHeader)
			return
		}

		next.ServeHTTP(w, r)
	})
}

/* Check origin middleware */
func (app *app) CheckOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Pass safe methods
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Origin / Referer check
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = r.Header.Get("Referer")
		}

		// Empty origin for not safe method
		if origin == "" {
			app.response.Error(w, r, response.ErrInvalidOrigin)
			return
		}

		u, err := url.Parse(origin)
		if err != nil {
			app.response.Error(w, r, response.ErrInvalidOrigin.Wrap(err))
			return
		}

		cleanOrigin := u.Scheme + "://" + u.Host
		if !slices.Contains(app.allowedOrigins, cleanOrigin) {
			app.response.Error(w, r, response.ErrInvalidOrigin)
			return
		}

		next.ServeHTTP(w, r)
	})
}
