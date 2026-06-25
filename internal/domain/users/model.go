package users

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"time"

	"github.com/google/uuid"
)

// Full user (stored in cache, standart type)

type User struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Username  string     `json:"username"`
	Avatar    string     `json:"avatar"`
	Slug      string     `json:"slug"`
	Role      types.Role `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at"`
}

func FromDB(u db.User) User {
	var deletedAt *time.Time
	if u.DeletedAt.Valid {
		deletedAt = &u.DeletedAt.Time
	}
	return User{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		Avatar:    u.Avatar,
		Slug:      u.Slug,
		Role:      u.Role,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
		DeletedAt: deletedAt,
	}
}

// User response

type UserResponse struct {
	ID        *uuid.UUID  `json:"id,omitempty"`
	Email     *string     `json:"email,omitempty"`
	Username  string      `json:"username"`
	Avatar    string      `json:"avatar"`
	Slug      string      `json:"slug"`
	Role      *types.Role `json:"role,omitempty"`
	CreatedAt *time.Time  `json:"created_at,omitempty"`
	UpdatedAt *time.Time  `json:"updated_at,omitempty"`
	DeletedAt *time.Time  `json:"deleted_at,omitempty"`
}

func ToMe(u User) UserResponse {
	return UserResponse{
		ID:        &u.ID,
		Email:     &u.Email,
		Username:  u.Username,
		Avatar:    u.Avatar,
		Slug:      u.Slug,
		Role:      &u.Role,
		CreatedAt: &u.CreatedAt,
	}
}

func ToPublic(u User) UserResponse {
	return UserResponse{
		Username: u.Username,
		Avatar:   u.Avatar,
		Slug:     u.Slug,
	}
}
