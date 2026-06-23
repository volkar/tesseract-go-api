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

// Raw album in list (stored in cache, standart type)

type AlbumInList struct {
	ID           uuid.UUID    `json:"id"`
	Title        string       `json:"title"`
	Slug         string       `json:"slug"`
	Cover        string       `json:"cover"`
	Access       types.Access `json:"access"`
	SharedEmails []string     `json:"shared_emails"`
	DateAt       time.Time    `json:"date_at"`
	IsActive     bool         `json:"is_active"`
}

func ToAlbumList(albums []Album) []AlbumInList {
	albumsResponse := make([]AlbumInList, len(albums))
	for i := range albums {
		albumsResponse[i] = AlbumInList{
			ID:           albums[i].ID,
			Title:        albums[i].Title,
			Slug:         albums[i].Slug,
			Cover:        albums[i].Cover,
			Access:       albums[i].Access,
			SharedEmails: albums[i].SharedEmails,
			DateAt:       albums[i].DateAt,
			IsActive:     albums[i].IsActive,
		}
	}
	return albumsResponse
}

// Public album (returned to client)

type PublicAlbum struct {
	Title  string      `json:"title"`
	Slug   string      `json:"slug"`
	Cover  string      `json:"cover"`
	Atlas  types.Atlas `json:"atlas"`
	DateAt time.Time   `json:"date_at"`
}

func ToPublic(a Album) PublicAlbum {
	return PublicAlbum{
		Title:  a.Title,
		Slug:   a.Slug,
		Cover:  a.Cover,
		Atlas:  a.Atlas,
		DateAt: a.DateAt,
	}
}

// Direct shared album (returned to client)

type DirectAlbum struct {
	Title  string      `json:"title"`
	Cover  string      `json:"cover"`
	Atlas  types.Atlas `json:"atlas"`
	DateAt time.Time   `json:"date_at"`
}

func ToDirect(a Album) DirectAlbum {
	return DirectAlbum{
		Title:  a.Title,
		Cover:  a.Cover,
		Atlas:  a.Atlas,
		DateAt: a.DateAt,
	}
}

// Public album in list (returned to client)

type PublicAlbumInList struct {
	Title  string    `json:"title"`
	Slug   string    `json:"slug"`
	Cover  string    `json:"cover"`
	DateAt time.Time `json:"date_at"`
}

func ToPublicAlbumList(albums []AlbumInList) []PublicAlbumInList {
	albumsResponse := make([]PublicAlbumInList, len(albums))
	for i := range albums {
		albumsResponse[i] = PublicAlbumInList{
			Title:  albums[i].Title,
			Slug:   albums[i].Slug,
			Cover:  albums[i].Cover,
			DateAt: albums[i].DateAt,
		}
	}
	return albumsResponse
}
