package tokens

import (
	db "api/internal/platform/database/sqlc"
	"time"

	"github.com/google/uuid"
)

// User session (refresh token)
type Session struct {
	ID        uuid.UUID `json:"id"`
	IP        string    `json:"ip"`
	UA        string    `json:"ua"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
	IsCurrent bool      `json:"is_current"`
}

func FromDB(rt db.RefreshToken, currentHash string) Session {
	return Session{
		ID:        rt.ID,
		IP:        rt.Ip,
		UA:        rt.Ua,
		Location:  rt.Location,
		CreatedAt: rt.CreatedAt.Time,
		IsCurrent: rt.TokenHash == currentHash,
	}
}

func FromDBList(tokens []db.RefreshToken, currentHash string) []Session {
	sessionResponse := make([]Session, len(tokens))
	for i := range tokens {
		sessionResponse[i] = FromDB(tokens[i], currentHash)
	}
	return sessionResponse
}
