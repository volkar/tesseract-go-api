package users

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"

	"github.com/google/uuid"
)

// Full user (stored in cache, standart type)

type User struct {
	ID       uuid.UUID  `json:"id"`
	Email    string     `json:"email"`
	Username string     `json:"username"`
	Avatar   string     `json:"avatar"`
	Slug     string     `json:"slug"`
	Role     types.Role `json:"role"`
}

func FromDB(u db.User) User {
	return User{
		ID:       u.ID,
		Email:    u.Email,
		Username: u.Username,
		Avatar:   u.Avatar,
		Slug:     u.Slug,
		Role:     u.Role,
	}
}

// Public user (returned to client)

type PublicUser struct {
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
	Slug     string `json:"slug"`
}

func ToPublic(u User) PublicUser {
	return PublicUser{
		Username: u.Username,
		Avatar:   u.Avatar,
		Slug:     u.Slug,
	}
}
