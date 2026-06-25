package main

import (
	"api/internal/domain/admin"
	"api/internal/domain/albums"
	"api/internal/domain/tokens"
	"api/internal/domain/users"
	"api/internal/platform/auth"
	"api/internal/platform/cache"
	"api/internal/platform/config"
	"api/internal/platform/cookies"
	"api/internal/platform/cursor"
	"api/internal/platform/i18n"
	"api/internal/platform/ratelimit"
	"api/internal/platform/response"
	"api/internal/platform/validator"
	"api/internal/worker"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
)

type app struct {
	cfg              *config.Config
	pool             *pgxpool.Pool
	redis            *redis.Client
	logger           *slog.Logger
	ratelimit        *ratelimit.Limiter
	cache            *cache.Manager
	response         *response.Response
	tokens           *tokens.Manager
	cookies          *cookies.Manager
	authHandler      *auth.Handler
	albumsHandler    *albums.Handler
	albumsService    *albums.Service
	usersHandler     *users.Handler
	usersService     *users.Service
	adminHandler     *admin.Handler
	adminService     *admin.Service
	cursorManager    *cursor.Cursor
	allowedOrigins   []string
	allowedProviders []string
}

func main() {
	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Default logger with tint
	handler := tint.NewHandler(os.Stdout, &tint.Options{
		TimeFormat: time.TimeOnly,
		Level:      slog.LevelDebug,
	})
	logger := slog.New(handler)

	// Env config loader
	cfg := config.MustLoad(logger)
	if cfg.Env == "dev" {
		cfg.Cookie.Secure = false
	}

	// Check if both secret keys are long enough
	if len(cfg.Cookie.SecretKey) < 32 || len(cfg.Paseto.SecretKey) < 32 {
		logger.Error("Secret keys should be at least 32 characters long")
		os.Exit(1)
	}

	// Database pool
	pool, err := pgxpool.New(ctx, cfg.DB.DSN)
	if err != nil {
		logger.Error("Failed to connect to database", "err", err)
		os.Exit(1)
	}
	// Explicitly ping the database to verify connection
	if err := pool.Ping(ctx); err != nil {
		logger.Error("Database is unreachable", "err", err)
		os.Exit(1)
	}

	defer pool.Close()
	logger.Info("Successfully connected to database")

	// Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Explicitly ping Redis to verify connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Error("Redis is unreachable", "err", err)
		os.Exit(1)
	}

	logger.Info("Successfully connected to Redis")

	// Rate limiter
	limiter := ratelimit.New(redisClient)

	// Cache manager
	cache := cache.New(redisClient, cfg.Cache.TTL)

	// Internationalization
	i18n := i18n.New(cfg.FallbackLang, logger)

	// Response
	resp := response.New(logger, i18n)

	// Validator wrapper
	val := validator.New()

	// Cookie manager
	cookiesManager := cookies.New(cfg.Cookie.Secure, cfg.Cookie.SameSite, cfg.Paseto.AccessTTL, cfg.Paseto.RefreshTTL)

	// Cursor manager
	cursorManager := cursor.New([]byte(cfg.CursorSecretKey), logger)

	// Token manager
	tokensRepo := tokens.NewRepository(pool)
	tokenManager := tokens.NewService(cfg.Paseto.SecretKey, tokensRepo, logger)

	// Users package
	usersRepo := users.NewRepository(pool, cache)
	usersService := users.NewService(usersRepo, tokenManager)
	usersHandler := users.NewHandler(usersService, resp, val, cookiesManager)

	// Albums package
	albumsRepo := albums.NewRepository(pool, cache, cursorManager)
	albumsService := albums.NewService(albumsRepo, cfg.AlbumsPerPage)
	albumsHandler := albums.NewHandler(albumsService, usersService, resp, val, cfg.AlbumsPerPage)

	// Admin package
	adminService := admin.NewService(usersService)
	adminHandler := admin.NewHandler(adminService, resp)

	// Auth package
	authService := auth.NewService(tokenManager, usersService, logger, cfg.Paseto.AccessTTL, cfg.Paseto.RefreshTTL)
	authHandler := auth.NewHandler(authService, resp, cookiesManager, logger, cfg.Auth.RedirectSuccess, cfg.Auth.AllowedProviders)

	// OAuth cookie store settings
	store := sessions.NewCookieStore([]byte(cfg.Cookie.SecretKey))
	store.MaxAge(cfg.Cookie.MaxAge)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = cfg.Cookie.Secure
	store.Options.SameSite = cfg.Cookie.SameSite
	gothic.Store = store

	// Oauth providers
	goth.UseProviders(
		google.New(cfg.Auth.Google.ClientID, cfg.Auth.Google.ClientSecret, cfg.Auth.Google.CallbackURL, "email", "profile"),
	)

	// App dependencies
	a := app{
		cfg:              cfg,
		pool:             pool,
		redis:            redisClient,
		ratelimit:        limiter,
		cache:            cache,
		logger:           logger,
		response:         resp,
		tokens:           tokenManager,
		cookies:          cookiesManager,
		authHandler:      authHandler,
		albumsHandler:    albumsHandler,
		albumsService:    albumsService,
		usersHandler:     usersHandler,
		usersService:     usersService,
		adminHandler:     adminHandler,
		adminService:     adminService,
		cursorManager:    cursorManager,
		allowedOrigins:   []string{cfg.Frontend, cfg.Backend},
		allowedProviders: cfg.Auth.AllowedProviders,
	}

	// Run cleanup worker
	go worker.StartCleanupWorker(ctx, tokensRepo, logger, 24*time.Hour)

	// Mount and run application server
	if err := a.run(ctx, a.mount()); err != nil {
		slog.Error("Server stopped with error", "err", err)
		os.Exit(1)
	}
}

/* Mount application routes */
func (app *app) mount() http.Handler {
	r := chi.NewRouter()
	// Base middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(5 * time.Second))
	r.Use(middleware.RequestSize(1 << 20))
	// Logger middleware
	loggerOptions := &httplog.Options{}
	loggerOptions.Schema = httplog.SchemaECS.Concise(true)
	if app.cfg.Env == "dev" {
		loggerOptions.LogRequestHeaders = []string{"Cookie"}
		loggerOptions.Level = slog.LevelDebug
	}
	r.Use(httplog.RequestLogger(app.logger, loggerOptions))
	// Security middlewares
	crs := cors.New(cors.Options{
		AllowedOrigins:   app.allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	})
	r.Use(crs.Handler)
	r.Use(app.AddSecurityHeaders)
	if app.cfg.Security.CheckHeaders {
		r.Use(app.CheckXHeaders)
	}
	if app.cfg.Security.CheckOrigin {
		r.Use(app.CheckOrigin)
	}
	// Data extraction middlewares
	r.Use(app.InsertLanguageToContext)
	r.Use(app.InsertClaimsToContext)
	// ETag checker / setter
	r.Use(app.ETagChecker)

	app.Routes(r)

	//TODO: Delete in production. Playground routes for development
	if app.cfg.Env == "dev" {
		app.RoutesPlayground(r)
	}

	return r
}

/* Run application server */
func (app *app) run(ctx context.Context, h http.Handler) error {
	srv := &http.Server{
		Addr:              app.cfg.Addr,
		Handler:           h,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 18,
	}

	shutdownError := make(chan error, 1)

	go func() {
		app.logger.Info("Starting server", "addr", app.cfg.Addr)
		// ListenAndServe always returns non-nil error.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			shutdownError <- err
		}
	}()

	// Graceful shutdown
	select {
	case err := <-shutdownError:
		return err
	case <-ctx.Done():
		app.logger.Info("Shutting down server...")

		// 5 seconds timeout for graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			app.logger.Error("Shutdown error", "err", err)
			return err
		}

		// Close database and redis connections
		app.pool.Close()
		app.redis.Close()

		app.logger.Info("Server stopped gracefully")
	}

	return nil
}
