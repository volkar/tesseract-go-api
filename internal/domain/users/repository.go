package users

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Repository struct {
	q     db.Querier
	cache Cacher
	sf    singleflight.Group
}

func NewRepository(pool *pgxpool.Pool, cache Cacher) *Repository {
	return &Repository{
		q:     db.New(pool),
		cache: cache,
	}
}

type Cacher interface {
	Set(ctx context.Context, key string, data any) error
	Get(ctx context.Context, key string, target any) error
	Unlink(ctx context.Context, keys ...string) error
	RunScript(ctx context.Context, script *redis.Script, keys []string, args ...any) (any, error)
}

const (
	CachePrefixEntity = "c:u:"
	CachePrefixMapper = "c:u_m:"
)

//go:embed lua/get_user_by_slug.lua
var getUserBySlugLua string
var getUserBySlugScript = redis.NewScript(getUserBySlugLua)

//go:embed lua/invalidate_user_with_mapper.lua
var invalidateUserWithMapperLua string
var invalidateUserWithMapperScript = redis.NewScript(invalidateUserWithMapperLua)

/* Get non deleted user by id from cache */
func (r *Repository) GetAvailable(ctx context.Context, id uuid.UUID) (User, error) {
	// Get user from cache
	u, err := r.getUserFromCache(ctx, id)
	if err == nil {
		// User found in cache, map and return
		return FromDB(u), nil
	}

	// Not found in cache, get from the database
	dbUser, err := r.q.GetAvailableUser(ctx, id)
	if err != nil {
		return User{}, err
	}

	// Async set user to cache
	bgCtx := context.WithoutCancel(ctx)
	go func(user db.User) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.setUserToCache(timeoutCtx, user)
	}(dbUser)

	return FromDB(dbUser), nil
}

/* Get non deleted user by slug with cache */
func (r *Repository) GetAvailableBySlug(ctx context.Context, slug string) (User, error) {
	// Try to get user from cache
	user, _ := r.getUserBySlugFromCache(ctx, slug)
	if user.ID != uuid.Nil {
		return FromDB(user), nil
	}

	// User not found in cache, get from database
	// Use singleflight to prevent Cache Stampede
	sfKey := "sf:user:slug:" + slug
	val, sfErr, _ := r.sf.Do(sfKey, func() (any, error) {
		dbUser, dbErr := r.q.GetAvailableUserBySlug(ctx, slug)
		if dbErr != nil {
			return db.User{}, dbErr
		}
		// Async set user to cache
		bgCtx := context.WithoutCancel(ctx)
		go func(u db.User) {
			timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
			defer cancel()
			r.setUserToCache(timeoutCtx, u)
		}(dbUser)

		return dbUser, nil
	})

	if sfErr != nil {
		return User{}, sfErr
	}

	return FromDB(val.(db.User)), nil
}

/* Upsert user by email and username, auth process */
func (r *Repository) Upsert(ctx context.Context, email string, username string, avatar string) (User, error) {
	u, err := r.q.UpsertUser(ctx, db.UpsertUserParams{
		Email:    email,
		Username: username,
		Avatar:   avatar,
	})
	if err != nil {
		return User{}, err
	}

	// Async set new user to cache
	bgCtx := context.WithoutCancel(ctx)
	go func(user db.User) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.setUserToCache(timeoutCtx, user)
	}(u)

	user := FromDB(u)
	return user, nil
}

/* Create user */
func (r *Repository) Create(ctx context.Context, email string, username string, slug string, avatar string, role types.Role) (User, error) {
	u, err := r.q.CreateUser(ctx, db.CreateUserParams{
		Username: username,
		Slug:     slug,
		Email:    email,
		Avatar:   avatar,
		Role:     role,
	})
	if err != nil {
		slog.Error("err", "err", err)
		return User{}, err
	}

	// Async set new user to cache
	bgCtx := context.WithoutCancel(ctx)
	go func(user db.User) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.setUserToCache(timeoutCtx, user)
	}(u)

	return FromDB(u), nil
}

/* Update user */
func (r *Repository) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (User, error) {
	u, err := r.q.UpdateUser(ctx, db.UpdateUserParams{
		ID:       id,
		Username: req.Username,
		Slug:     req.Slug,
		Avatar:   req.Avatar,
	})

	if err != nil {
		return User{}, err
	}

	// Async update user cache
	bgCtx := context.WithoutCancel(ctx)
	go func(user db.User, oldUserSlug string) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		// User slug changed. Invalidate user mapper cache
		if oldUserSlug != user.Slug {
			r.invalidateUserMapperCache(timeoutCtx, oldUserSlug)
		}
		// Set new user cache
		r.setUserToCache(timeoutCtx, user)
	}(u.User, u.OldSlug)

	return FromDB(u.User), nil
}

/* Delete user */
func (r *Repository) Delete(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	id, err := r.q.SoftDeleteUser(ctx, id)
	if err != nil {
		return uuid.UUID{}, err
	}

	// Async invalidate user cache (entity + mapper)
	bgCtx := context.WithoutCancel(ctx)
	go func(userID uuid.UUID) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.invalidateUserCache(timeoutCtx, userID)
	}(id)

	return id, err
}

/* Hard delete user (with all albums via db onDelete) */
func (r *Repository) PurgeUser(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	id, err := r.q.HardDeleteUser(ctx, id)
	if err != nil {
		return uuid.UUID{}, err
	}

	// Async invalidate user cache (entity + mapper)
	bgCtx := context.WithoutCancel(ctx)
	go func(userID uuid.UUID) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.invalidateUserCache(timeoutCtx, userID)
	}(id)

	return id, err
}

/* Restore deleted user */
func (r *Repository) RestoreUser(ctx context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	res, err := r.q.RestoreUser(ctx, id)
	if err != nil {
		return uuid.Nil, "", err
	}
	return res.ID, res.Slug, nil
}

/* Get user from cache */
func (r *Repository) getUserFromCache(ctx context.Context, id uuid.UUID) (db.User, error) {
	var u db.User
	err := r.cache.Get(ctx, CachePrefixEntity+id.String(), &u)
	if err != nil {
		return db.User{}, err
	}

	return u, nil
}

/* Get user by slug from cache */
func (r *Repository) getUserBySlugFromCache(ctx context.Context, slug string) (db.User, error) {
	res, err := r.cache.RunScript(ctx, getUserBySlugScript,
		[]string{CachePrefixMapper + slug},
		CachePrefixEntity,
	)
	if err != nil || res == nil {
		return db.User{}, err
	}

	var user db.User
	if err := json.Unmarshal([]byte(res.(string)), &user); err != nil {
		return db.User{}, err
	}

	return user, nil
}

/* Set user to cache */
func (r *Repository) setUserToCache(ctx context.Context, u db.User) {
	r.cache.Set(ctx, CachePrefixEntity+u.ID.String(), u)
	r.cache.Set(ctx, CachePrefixMapper+u.Slug, u.ID.String())
}

/* Invalidates user cache (entity + mapper) */
func (r *Repository) invalidateUserCache(ctx context.Context, id uuid.UUID) {
	r.cache.RunScript(ctx, invalidateUserWithMapperScript,
		[]string{CachePrefixEntity + id.String()},
		CachePrefixMapper,
	)
}

/* Invalidates user mapper cache */
func (r *Repository) invalidateUserMapperCache(ctx context.Context, slug string) {
	r.cache.Unlink(ctx, CachePrefixMapper+slug)
}
