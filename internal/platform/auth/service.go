package auth

import (
	"api/internal/domain/tokens"
	"api/internal/domain/users"
	"api/internal/platform/request"
	"api/internal/platform/response"
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/markbates/goth"
)

type Service struct {
	tokens     *tokens.Manager
	users      UsersService
	logger     *slog.Logger
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(tokens *tokens.Manager, users UsersService, logger *slog.Logger, accessTTL time.Duration, refreshTTL time.Duration) *Service {
	return &Service{
		tokens:     tokens,
		users:      users,
		logger:     logger,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

type UsersService interface {
	Upsert(ctx context.Context, email string, username string, avatar string) (users.User, error)
	GetAvailable(ctx context.Context, userID uuid.UUID) (users.User, error)
}

/* Get email and name from OAuth user, creates access and refresh tokens */
func (s *Service) UpsertOAuthUser(ctx context.Context, oauthUser goth.User) (users.User, error) {
	if oauthUser.Email == "" {
		return users.User{}, response.ErrOAuthNoEmail
	}
	if oauthUser.Name == "" {
		return users.User{}, response.ErrOAuthNoName
	}
	// Upsert allows register new users
	// Can be replaced with getting user by email, to allows authentication only
	u, err := s.users.Upsert(ctx, oauthUser.Email, oauthUser.Name, oauthUser.AvatarURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// User deleted
			return users.User{}, response.ErrNoPermission.Wrap(err)
		}
		return users.User{}, err
	}
	return u, nil
}

/* Creates access and refresh tokens for user */
func (s *Service) IssueSessionTokens(ctx context.Context, user users.User, meta request.Metadata) (string, string, error) {
	// Generate refresh token random string
	refresh, err := s.tokens.GenerateRefreshString()
	if err != nil {
		return "", "", err
	}

	// Hash the refresh token string and write it to the database
	hash := s.tokens.Hash(refresh)
	_, err = s.tokens.CreateRefresh(ctx, user.ID, hash, time.Now().Add(s.refreshTTL), meta)
	if err != nil {
		return "", "", err
	}

	// Create access token
	access := s.tokens.CreateAccess(user.ID, user.Role, user.Email, s.accessTTL)

	return access, refresh, nil
}

/* Consume refresh token */
func (s *Service) ConsumeSessionToken(ctx context.Context, token string) error {
	hash := s.tokens.Hash(token)
	return s.tokens.ConsumeRefreshByHash(ctx, hash)
}

/* Revoke other sessions (exit from all devices) */
func (s *Service) SessionList(ctx context.Context, userID uuid.UUID, currentRefresh string) ([]tokens.Session, error) {
	currentHash := s.tokens.Hash(currentRefresh)
	return s.tokens.GetActiveRefreshes(ctx, userID, currentHash)
}

/* Revoke other sessions (exit from all devices) */
func (s *Service) RevokeSession(ctx context.Context, tokenID uuid.UUID, userID uuid.UUID, currentRefresh string) (bool, error) {
	deletedHash, err := s.tokens.DeleteRefreshByID(ctx, tokenID, userID)
	if err != nil {
		return false, err
	}
	currentHash := s.tokens.Hash(currentRefresh)
	return currentHash == deletedHash, nil
}

/* Delete other refresh tokens (exit from all devices) */
func (s *Service) DeleteOtherSessions(ctx context.Context, userID uuid.UUID, refresh string) error {
	hash := s.tokens.Hash(refresh)
	return s.tokens.DeleteOtherRefreshForUser(ctx, userID, hash)
}

/* Delete old refresh token and get a new one */
func (s *Service) RefreshSession(ctx context.Context, oldRefreshToken string, meta request.Metadata) (string, string, error) {
	// Old refresh token hash
	oldHash := s.tokens.Hash(oldRefreshToken)

	// Find old refresh token by hash
	oldToken, err := s.tokens.GetRefreshByHash(ctx, oldHash)
	if err != nil {
		return "", "", response.ErrRefreshSession.Wrap(err)
	}

	// Check if token is expired
	if oldToken.ExpiresAt.Before(time.Now()) {
		return "", "", response.ErrTokenExpired
	}

	// Check if token is consumed
	if oldToken.IsConsumed {
		// Calculate time since the token was marked as consumed
		timeSinceConsumption := time.Since(oldToken.UpdatedAt.Time)

		// 5-second grace period for network retries
		if timeSinceConsumption < 5*time.Second {
			return "", "", response.ErrTokenConsumed
		}

		// Real reuse detected outside grace period. Possible token stealing.
		// Invalidate all refresh tokens for this user.
		s.tokens.DeleteAllRefreshForUser(ctx, oldToken.UserID)
		s.logger.Warn("Potential refresh token reuse detected", "user_id", oldToken.UserID, "reason", "Token already consumed outside grace period")

		return "", "", response.ErrTokenConsumed
	}

	// New refresh token
	newRefresh, err := s.tokens.GenerateRefreshString()
	if err != nil {
		return "", "", response.ErrRefreshSession.Wrap(err)
	}
	newHash := s.tokens.Hash(newRefresh)
	newHashTTL := time.Now().Add(s.refreshTTL)

	// Replace tokens
	err = s.tokens.ReplaceRefresh(ctx, oldToken.UserID, oldHash, newHash, newHashTTL, meta)
	if err != nil {
		if err == response.ErrTokenConsumed {
			// Reuse detected in transaction. Possible token stealing. Delete all refresh tokens
			s.tokens.DeleteAllRefreshForUser(ctx, oldToken.UserID)
			// Log it
			s.logger.Warn("Potential refresh token reuse detected", "user_id", oldToken.UserID, "reason", "Transaction rows affected zero")
		}
		return "", "", response.ErrRefreshSession.Wrap(err)
	}

	// Get user
	user, err := s.users.GetAvailable(ctx, oldToken.UserID)
	if err != nil {
		return "", "", response.ErrRefreshSession.Wrap(err)
	}

	// New access token
	newAccess := s.tokens.CreateAccess(user.ID, user.Role, user.Email, s.accessTTL)

	return newAccess, newRefresh, nil
}
