package config

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Addr            string `env:"APP_ADDRESS" env-default:"localhost:8000"`
	Backend         string `env:"APP_BACKEND_URL" env-default:"http://localhost:8000"`
	Frontend        string `env:"APP_FRONTEND_URL" env-default:"http://localhost:5173"`
	Env             string `env:"APP_ENV" env-default:"dev"`
	Lang            string `env:"APP_LANGUAGE" env-default:"en"`
	FallbackLang    string `env:"APP_FALLBACK_LANGUAGE" env-default:"en"`
	CursorSecretKey string `env:"CURSOR_SECRET_KEY" env-required:"true"`
	AlbumsPerPage   int    `env:"ALBUMS_PER_PAGE" env-required:"true"`
	Security        securityConfig
	Cache           cacheConfig
	Auth            authConfig
	DB              dbConfig
	Redis           redisConfig
	Cookie          cookieConfig
	Paseto          pasetoConfig
}

func MustLoad(logger *slog.Logger) *Config {
	var cfg Config

	if err := cleanenv.ReadConfig(".env", &cfg); err != nil {
		logger.Error("Cannot read config", "err", err)
		os.Exit(1)
	}
	return &cfg
}

type authConfig struct {
	RedirectSuccess  string      `env:"AUTH_SUCCESS_URL" env-default:"localhost:3000"`
	Google           oauthConfig `env-prefix:"GOOGLE_"`
	AllowedProviders []string    `env:"AUTH_ALLOWED_PROVIDERS" env-required:"true"`
}

type securityConfig struct {
	CheckHeaders bool `env:"SECURITY_CHECK_HEADERS" env-default:"true"`
	CheckOrigin  bool `env:"SECURITY_CHECK_ORIGIN" env-default:"true"`
}

type oauthConfig struct {
	ClientID     string `env:"CLIENT_ID" env-required:"true"`
	ClientSecret string `env:"CLIENT_SECRET" env-required:"true"`
	CallbackURL  string `env:"CALLBACK_URL" env-required:"true"`
}

type cacheConfig struct {
	TTL time.Duration `env:"CACHE_TTL" env-default:"10m"`
}

type dbConfig struct {
	DSN string `env:"GOOSE_DBSTRING" env-required:"true"`
}

type redisConfig struct {
	Addr     string `env:"REDIS_ADDR" env-required:"true"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" env-default:"0"`
}

type cookieConfig struct {
	SecretKey string        `env:"COOKIE_SECRET_KEY" env-required:"true"`
	MaxAge    int           `env:"COOKIE_MAX_AGE" env-default:"2592000"`
	Secure    bool          `env-default:"true"`
	SameSite  http.SameSite `env-default:"2"`
}

type pasetoConfig struct {
	SecretKey  string        `env:"PASETO_SECRET_KEY" env-required:"true"`
	AccessTTL  time.Duration `env:"ACCESS_TOKEN_TTL" env-default:"10m"`
	RefreshTTL time.Duration `env:"REFRESH_TOKEN_TTL" env-default:"720h"`
}
