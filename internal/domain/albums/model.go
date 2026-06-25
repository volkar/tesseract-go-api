package albums

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"time"

	"github.com/google/uuid"
)

// Full album (stored in cache, standart type)

type Album struct {
	ID           uuid.UUID    `json:"id"`
	UserID       uuid.UUID    `json:"user_id"`
	Title        string       `json:"title"`
	Slug         string       `json:"slug"`
	Cover        string       `json:"cover"`
	Atlas        types.Atlas  `json:"atlas"`
	Access       types.Access `json:"access"`
	DirectToken  string       `json:"direct_token"`
	SharedEmails []string     `json:"shared_emails"`
	DateAt       time.Time    `json:"date_at"`
	IsActive     bool         `json:"is_active"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	DeletedAt    *time.Time   `json:"deleted_at"`
}

func FromDB(a db.Album) Album {
	var deletedAt *time.Time
	var directToken string
	if a.DeletedAt.Valid {
		deletedAt = &a.DeletedAt.Time
	}
	if a.DirectToken.Valid {
		directToken = a.DirectToken.UUID.String()
	}
	return Album{
		ID:           a.ID,
		UserID:       a.UserID,
		Title:        a.Title,
		Slug:         a.Slug,
		Cover:        a.Cover,
		Atlas:        a.Atlas,
		Access:       a.Access,
		DirectToken:  directToken,
		SharedEmails: a.SharedEmails,
		DateAt:       a.DateAt,
		IsActive:     a.IsActive,
		CreatedAt:    a.CreatedAt,
		UpdatedAt:    a.UpdatedAt,
		DeletedAt:    deletedAt,
	}
}

func FromDBList(albums []db.Album) []Album {
	albumsResponse := make([]Album, len(albums))
	for i := range albums {
		albumsResponse[i] = FromDB(albums[i])
	}
	return albumsResponse
}

// Album response

type AlbumResponse struct {
	ID           *uuid.UUID    `json:"id,omitempty"`
	Title        string        `json:"title"`
	Slug         *string       `json:"slug,omitempty"`
	Cover        string        `json:"cover"`
	Atlas        *types.Atlas  `json:"atlas,omitempty"`
	Access       *types.Access `json:"access,omitempty"`
	DirectToken  *string       `json:"direct_token,omitempty"`
	SharedEmails *[]string     `json:"shared_emails,omitempty"`
	DateAt       time.Time     `json:"date_at,omitempty"`
	IsActive     *bool         `json:"is_active,omitempty"`
	CreatedAt    *time.Time    `json:"created_at,omitempty"`
	UpdatedAt    *time.Time    `json:"updated_at,omitempty"`
	DeletedAt    *time.Time    `json:"deleted_at,omitempty"`
	IsOwner      bool          `json:"is_owner,omitempty"`
}

func ToMy(a Album) AlbumResponse {
	return AlbumResponse{
		ID:           &a.ID,
		Title:        a.Title,
		Slug:         &a.Slug,
		Cover:        a.Cover,
		Atlas:        &a.Atlas,
		Access:       &a.Access,
		DirectToken:  &a.DirectToken,
		SharedEmails: &a.SharedEmails,
		DateAt:       a.DateAt,
		IsActive:     &a.IsActive,
		IsOwner:      true,
	}
}

func ToPublic(a Album) AlbumResponse {
	return AlbumResponse{
		Title:   a.Title,
		Slug:    &a.Slug,
		Cover:   a.Cover,
		Atlas:   &a.Atlas,
		DateAt:  a.DateAt,
		IsOwner: false,
	}
}

func ToDirect(a Album) AlbumResponse {
	return AlbumResponse{
		Title:  a.Title,
		Cover:  a.Cover,
		Atlas:  &a.Atlas,
		DateAt: a.DateAt,
	}
}

func ToMyAlbumList(albums []Album) []AlbumResponse {
	albumsResponse := make([]AlbumResponse, len(albums))
	for i := range albums {
		albumsResponse[i] = AlbumResponse{
			ID:           &albums[i].ID,
			Title:        albums[i].Title,
			Slug:         &albums[i].Slug,
			Cover:        albums[i].Cover,
			Access:       &albums[i].Access,
			SharedEmails: &albums[i].SharedEmails,
			DateAt:       albums[i].DateAt,
			IsActive:     &albums[i].IsActive,
		}
	}
	return albumsResponse
}

func ToPublicAlbumList(albums []Album) []AlbumResponse {
	albumsResponse := make([]AlbumResponse, len(albums))
	for i := range albums {
		albumsResponse[i] = AlbumResponse{
			Title:  albums[i].Title,
			Slug:   &albums[i].Slug,
			Cover:  albums[i].Cover,
			DateAt: albums[i].DateAt,
		}
	}
	return albumsResponse
}
