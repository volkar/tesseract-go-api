package tokens

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"api/internal/platform/request"
	"api/internal/platform/response"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"time"

	"aidanwoods.dev/go-paseto"
	"github.com/google/uuid"
)

type Manager struct {
	key    paseto.V4SymmetricKey
	parser *paseto.Parser
	tokens *Repository
}

func NewService(secretKey string, repo *Repository, logger *slog.Logger) *Manager {
	// Use Paseto token with secret key
	key, err := paseto.V4SymmetricKeyFromBytes([]byte(secretKey))
	if err != nil {
		logger.Error("Failed to init token manager", "err", err)
		os.Exit(1)
	}

	parser := paseto.NewParser()

	return &Manager{
		key:    key,
		parser: &parser,
		tokens: repo,
	}
}

type ctxKey string

const userClaimsKey ctxKey = "user"

/* Generate random refresh token string */
func (m *Manager) GenerateRefreshString() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

/* Hash refresh token string */
func (m *Manager) Hash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

/* Get refresh token by hash */
func (m *Manager) GetRefreshByHash(ctx context.Context, hash string) (db.RefreshToken, error) {
	return m.tokens.GetByHash(ctx, hash)
}

/* Create refresh token */
func (m *Manager) CreateRefresh(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, meta request.Metadata) (uuid.UUID, error) {
	return m.tokens.Create(ctx, userID, tokenHash, expiresAt, meta.IP, meta.UserAgent, meta.Location)
}

/* Delete old token and create new one in single transaction */
func (m *Manager) ReplaceRefresh(ctx context.Context, userID uuid.UUID, oldHash string, newHash string, expiresAt time.Time, meta request.Metadata) error {
	return m.tokens.ReplaceInTransaction(ctx, userID, oldHash, newHash, expiresAt, meta.IP, meta.UserAgent, meta.Location)
}

/* Consume one token by hash */
func (m *Manager) ConsumeRefreshByHash(ctx context.Context, hash string) error {
	return m.tokens.ConsumeByHash(ctx, hash)
}

/* Get active refresh tokens for user, identifying the current one by token hash */
func (m *Manager) GetActiveRefreshes(ctx context.Context, userID uuid.UUID, currentHash string) ([]Session, error) {
	tokens, err := m.tokens.GetActiveForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Map to Sessions and return
	return FromDBList(tokens, currentHash), nil
}

/* Delete specific refresh token by ID */
func (m *Manager) DeleteRefreshByID(ctx context.Context, sessionID uuid.UUID, userID uuid.UUID) (string, error) {
	return m.tokens.DeleteByIDAndUser(ctx, sessionID, userID)
}

/* Delete other tokens for given user ID except given hash */
func (m *Manager) DeleteOtherRefreshForUser(ctx context.Context, userID uuid.UUID, hash string) error {
	return m.tokens.DeleteOtherForUser(ctx, userID, hash)
}

/* Delete all refresh tokens for given user ID */
func (m *Manager) DeleteAllRefreshForUser(ctx context.Context, userID uuid.UUID) error {
	return m.tokens.DeleteAllRefreshForUser(ctx, userID)
}

/* Create access token string */
func (m *Manager) CreateAccess(userID uuid.UUID, role types.Role, email string, ttl time.Duration) string {
	token := paseto.NewToken()

	now := time.Now()
	token.SetIssuedAt(now)
	token.SetNotBefore(now)
	token.SetExpiration(now.Add(ttl))

	// Set claims
	token.SetString("user_id", userID.String())
	token.SetString("role", string(role))
	token.SetString("email", email)

	return token.V4Encrypt(m.key, nil)
}

/* Parse Paseto access token string */
func (m *Manager) ParseAccess(tokenStr string) (types.UserClaims, error) {
	token, err := m.parser.ParseV4Local(m.key, tokenStr, nil)
	if err != nil {
		return types.UserClaims{}, err
	}

	// User ID
	userID, err := token.GetString("user_id")
	if err != nil {
		return types.UserClaims{}, err
	}
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return types.UserClaims{}, err
	}
	if userUUID == uuid.Nil {
		return types.UserClaims{}, response.ErrNoClaims
	}

	// Role
	role, err := token.GetString("role")
	if err != nil {
		return types.UserClaims{}, err
	}
	if types.Role(role) == "" {
		return types.UserClaims{}, response.ErrNoClaims
	}

	// Email
	email, err := token.GetString("email")
	if err != nil {
		return types.UserClaims{}, err
	}

	return types.UserClaims{
		UserID: userUUID,
		Role:   types.Role(role),
		Email:  email,
	}, nil
}
