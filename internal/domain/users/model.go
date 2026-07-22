package users

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"time"

	"github.com/google/uuid"
)

// Represents the API contract for the frontend
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

func ToMe(u db.User) UserResponse {
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

func ToPublic(u db.User) UserResponse {
	return UserResponse{
		Username: u.Username,
		Avatar:   u.Avatar,
		Slug:     u.Slug,
	}
}
