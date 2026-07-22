package albums

import (
	db "api/internal/platform/database/sqlc"
	"api/internal/platform/response"
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Service struct {
	albums        *Repository
	albumsPerPage int
}

func NewService(repo *Repository, albumsPerPage int) *Service {
	return &Service{
		albums:        repo,
		albumsPerPage: albumsPerPage,
	}
}

/* Get available album by user slug and album slug */
func (s *Service) GetAvailable(ctx context.Context, userID uuid.UUID, albumSlug string, viewerID uuid.UUID, viewerEmail string) (db.Album, error) {
	a, err := s.albums.GetBySlug(ctx, userID, albumSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Album{}, response.ErrAlbumNotFound.Wrap(err)
		}
		return db.Album{}, err
	}
	// Album found in cache or database. Check access permissions
	isOwner := viewerID != uuid.Nil && viewerID == a.UserID
	if (!a.IsActive && !isOwner) || !a.Access.CanAccess(a.SharedEmails, viewerEmail, isOwner) {
		return db.Album{}, response.ErrAlbumNotFound
	}
	return a, nil
}

/* Get owned album by user id and album id */
func (s *Service) GetOwned(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (db.Album, error) {
	a, err := s.albums.GetByID(ctx, userID, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Album{}, response.ErrAlbumNotFound.Wrap(err)
		}
		return db.Album{}, err
	}
	// Album found in cache or database. Check ownership
	if a.UserID != userID {
		return db.Album{}, response.ErrAlbumNotFound
	}
	return a, nil
}

/* Get album by direct token (Bypasses normal Access Control) */
func (s *Service) GetByDirectToken(ctx context.Context, token uuid.UUID) (db.Album, error) {
	a, err := s.albums.GetByDirectToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Album{}, response.ErrAlbumNotFound.Wrap(err)
		}
		if err.Error() == "album_not_found" {
			// Error from redis script
			return db.Album{}, response.ErrAlbumNotFound.Wrap(err)
		}
		return db.Album{}, err
	}
	// Album found in cache or database. Check if it is active
	if !a.IsActive {
		return db.Album{}, response.ErrAlbumNotFound
	}
	return a, nil
}

/* Get list of available albums by user id */
func (s *Service) ListAvailable(ctx context.Context, userID uuid.UUID, viewerEmail string, cursor string, limit int) ([]db.Album, string, error) {
	if limit <= 0 || limit > s.albumsPerPage {
		limit = s.albumsPerPage
	}

	a, nextCursor, err := s.albums.ListAvailable(ctx, userID, viewerEmail, cursor, int32(limit))
	if err != nil {
		return []db.Album{}, "", err
	}
	return a, nextCursor, nil
}

/* Get list of all owned albums by user id */
func (s *Service) ListOwned(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]db.Album, string, error) {
	if limit <= 0 || limit > s.albumsPerPage {
		limit = s.albumsPerPage
	}

	a, nextCursor, err := s.albums.ListOwned(ctx, userID, cursor, int32(limit))
	if err != nil {
		return []db.Album{}, "", err
	}
	return a, nextCursor, nil
}

/* Get list of trashed albums by user id */
func (s *Service) ListTrashed(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]db.Album, string, error) {
	if limit <= 0 || limit > s.albumsPerPage {
		limit = s.albumsPerPage
	}
	a, nextCursor, err := s.albums.ListTrashed(ctx, userID, cursor, int32(limit))
	if err != nil {
		return []db.Album{}, "", err
	}
	return a, nextCursor, nil
}

/* Create album */
func (s *Service) Create(ctx context.Context, userID uuid.UUID, req CreateRequest) (db.Album, error) {
	a, err := s.albums.Create(ctx, userID, req)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.Is(err, pgx.ErrNoRows) {
			// User deleted or not existed
			return db.Album{}, response.ErrNoPermission.Wrap(err)
		}
		if errors.As(err, &pgErr) {
			if (pgErr.Code == "23505") && (pgErr.ConstraintName == "idx_albums_user_slug_active") {
				return db.Album{}, response.ErrAlbumSlugExists.Wrap(err)
			}
		}
		return db.Album{}, err
	}
	return a, nil
}

/* Update album */
func (s *Service) Update(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, req UpdateRequest) (db.Album, error) {
	a, err := s.albums.Update(ctx, userID, albumID, req)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Album not found or user is deleted
			return db.Album{}, response.ErrNoPermission.Wrap(err)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if (pgErr.Code == "23505") && (pgErr.ConstraintName == "idx_albums_user_slug_active") {
				// Slug conflict
				return db.Album{}, response.ErrAlbumSlugExists.Wrap(err)
			}
		}
		return db.Album{}, err
	}
	return a, nil
}

/* Generate new direct share link for album */
func (s *Service) GenerateDirectToken(ctx context.Context, userID, albumID uuid.UUID) (uuid.NullUUID, error) {
	newToken := uuid.NullUUID{UUID: uuid.New(), Valid: true}

	_, err := s.albums.UpdateDirectToken(ctx, userID, albumID, newToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Album not found or user is deleted
			return uuid.NullUUID{}, response.ErrNoPermission.Wrap(err)
		}
		return uuid.NullUUID{}, err
	}

	return newToken, nil
}

/* Revoke direct share link */
func (s *Service) RevokeDirectToken(ctx context.Context, userID, albumID uuid.UUID) error {
	tokenNull := uuid.NullUUID{Valid: false}
	_, err := s.albums.UpdateDirectToken(ctx, userID, albumID, tokenNull)
	if errors.Is(err, pgx.ErrNoRows) {
		// Album not found or user is deleted
		return response.ErrNoPermission.Wrap(err)
	}
	return err
}

/* Toggle album active state */
func (s *Service) ToggleActive(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, isActive bool) (uuid.UUID, error) {
	id, err := s.albums.ToggleActive(ctx, userID, albumID, isActive)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Album not found or user is deleted
			return uuid.UUID{}, response.ErrNoPermission.Wrap(err)
		}
		return uuid.UUID{}, err
	}
	return id, nil
}

/* Delete album */
func (s *Service) Delete(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	a, err := s.albums.Delete(ctx, userID, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Album not found or user is deleted
			return uuid.Nil, response.ErrNoPermission.Wrap(err)
		}
		return uuid.Nil, err
	}
	return a, nil
}

/* Restore deleted album */
func (s *Service) Restore(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	a, err := s.albums.Restore(ctx, userID, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User is deleted or album not found
			return uuid.Nil, response.ErrNoPermission.Wrap(err)
		}
		return uuid.Nil, err
	}
	return a, nil
}

/* Purge deleted album */
func (s *Service) Purge(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	a, err := s.albums.Purge(ctx, userID, albumID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User is deleted or album not found
			return uuid.Nil, response.ErrNoPermission.Wrap(err)
		}
		return uuid.Nil, err
	}
	return a, nil
}
