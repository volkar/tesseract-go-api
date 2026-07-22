package albums

import (
	"api/internal/domain/shared/types"
	db "api/internal/platform/database/sqlc"
	"time"

	"github.com/google/uuid"
)

// Represents the API contract for the frontend
type AlbumResponse struct {
	ID           *uuid.UUID     `json:"id,omitempty"`
	Title        string         `json:"title"`
	Slug         *string        `json:"slug,omitempty"`
	Cover        string         `json:"cover"`
	Atlas        *types.Atlas   `json:"atlas,omitempty"`
	Access       *types.Access  `json:"access,omitempty"`
	DirectToken  *uuid.NullUUID `json:"direct_token,omitempty"`
	SharedEmails *[]string      `json:"shared_emails,omitempty"`
	DateAt       time.Time      `json:"date_at"`
	IsActive     *bool          `json:"is_active,omitempty"`
	CreatedAt    *time.Time     `json:"created_at,omitempty"`
	UpdatedAt    *time.Time     `json:"updated_at,omitempty"`
	DeletedAt    *time.Time     `json:"deleted_at,omitempty"`
	IsOwner      bool           `json:"is_owner,omitempty"`
}

func ToMy(a db.Album) AlbumResponse {
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

func ToPublic(a db.Album) AlbumResponse {
	return AlbumResponse{
		Title:   a.Title,
		Slug:    &a.Slug,
		Cover:   a.Cover,
		Atlas:   &a.Atlas,
		DateAt:  a.DateAt,
		IsOwner: false,
	}
}

func ToDirect(a db.Album) AlbumResponse {
	return AlbumResponse{
		Title:  a.Title,
		Cover:  a.Cover,
		Atlas:  &a.Atlas,
		DateAt: a.DateAt,
	}
}

func ToMyAlbumList(albums []db.Album) []AlbumResponse {
	albumsResponse := make([]AlbumResponse, len(albums))
	for i := range albums {
		albumsResponse[i] = AlbumResponse{
			ID:       &albums[i].ID,
			Title:    albums[i].Title,
			Slug:     &albums[i].Slug,
			Cover:    albums[i].Cover,
			Access:   &albums[i].Access,
			DateAt:   albums[i].DateAt,
			IsActive: &albums[i].IsActive,
		}
	}
	return albumsResponse
}

func ToPublicAlbumList(albums []db.Album) []AlbumResponse {
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
