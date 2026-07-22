package users

import (
	"api/internal/domain/shared/types"
	"api/internal/domain/tokens"
	db "api/internal/platform/database/sqlc"
	"api/internal/platform/response"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	users  *Repository
	tokens *tokens.Manager
}

func NewService(repo *Repository, tokens *tokens.Manager) *Service {
	return &Service{
		users:  repo,
		tokens: tokens,
	}
}

/* Get non deleted user info by id */
func (s *Service) GetAvailable(ctx context.Context, id uuid.UUID) (db.User, error) {
	u, err := s.users.GetAvailable(ctx, id)
	if err != nil {
		return db.User{}, response.ErrUserNotFound.Wrap(err)
	}
	return u, nil
}

/* Get non deleted user info by slug */
func (s *Service) GetAvailableBySlug(ctx context.Context, slug string) (db.User, error) {
	u, err := s.users.GetAvailableBySlug(ctx, slug)
	if err != nil {
		return db.User{}, response.ErrUserNotFound.Wrap(err)
	}
	return u, nil
}

/* Upsert confirmed user */
func (s *Service) Upsert(ctx context.Context, email string, username string, avatar string) (db.User, error) {
	return s.users.Upsert(ctx, email, username, avatar)
}

/* Create user. Development only! Use with caution! Users must be created with Upsert function via OAuth process and have validated email */
func (s *Service) Create(ctx context.Context, email string, username string, slug string, avatar string, role types.Role) (db.User, error) {
	return s.users.Create(ctx, email, username, slug, avatar, role)
}

/* Update user info */
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (db.User, error) {
	u, err := s.users.Update(ctx, id, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User not found
			return db.User{}, response.ErrUserNotFound.Wrap(err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if (pgErr.Code == "23505") && (pgErr.ConstraintName == "idx_users_slug_active") {
				// Slug already exists
				return db.User{}, response.ErrUserSlugExists.Wrap(err)
			}
		}
		return db.User{}, err
	}
	return u, nil
}

/* Delete user */
func (s *Service) Delete(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	// Delete user
	id, err := s.users.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, response.ErrNoPermission.Wrap(err)
		}
		return uuid.Nil, err
	}

	// Delete all user tokens
	s.tokens.DeleteAllRefreshForUser(ctx, id)

	return id, err
}

/* Hard delete user (with all albums via db onDelete) */
func (s *Service) PurgeUser(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.users.PurgeUser(ctx, id)
}

/* Restore deleted user */
func (s *Service) RestoreUser(ctx context.Context, id uuid.UUID) (uuid.UUID, string, error) {
	return s.users.RestoreUser(ctx, id)
}
