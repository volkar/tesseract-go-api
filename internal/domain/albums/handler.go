package albums

import (
	"api/internal/domain/shared/types"
	"api/internal/domain/tokens"
	"api/internal/domain/users"
	"api/internal/platform/request"
	"api/internal/platform/response"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	albums        *Service
	users         UserGetter
	response      *response.Response
	validator     *validator.Validate
	albumsPerPage int
}

func NewHandler(service *Service, users UserGetter, response *response.Response, val *validator.Validate, albumsPerPage int) *Handler {
	return &Handler{
		albums:        service,
		users:         users,
		response:      response,
		validator:     val,
		albumsPerPage: albumsPerPage,
	}
}

type UserGetter interface {
	GetAvailableBySlug(ctx context.Context, userSlug string) (users.User, error)
}

/* Get available album by user slug and album slug */
func (h *Handler) GetAvailable(w http.ResponseWriter, r *http.Request) {
	userSlug := chi.URLParam(r, "user_slug")
	albumSlug := chi.URLParam(r, "album_slug")

	// Get claims from context
	claims, _ := tokens.GetClaimsFromContext(r.Context())
	// Get user by slug
	user, err := h.users.GetAvailableBySlug(r.Context(), userSlug)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	// Get available album
	a, err := h.albums.GetAvailable(r.Context(), user.ID, albumSlug, claims.UserID, claims.Email)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	var album AlbumResponse
	if a.UserID == claims.UserID {
		// Self album, map to full data
		album = ToMy(a)
	} else {
		// Public album, map to public data
		album = ToPublic(a)
	}

	h.response.SuccessDataOnly(w, r, album)
}

// Get any owned album by id
func (h *Handler) GetOwned(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}
	albumID, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		h.response.Error(w, r, response.ErrAlbumNotFound)
		return
	}

	// Get owned album
	a, err := h.albums.GetOwned(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Map to my album
	myAlbum := ToMy(a)

	h.response.SuccessDataOnly(w, r, myAlbum)
}

func (h *Handler) GetByDirectToken(w http.ResponseWriter, r *http.Request) {
	token, err := uuid.Parse(chi.URLParam(r, "direct_token"))
	if err != nil {
		h.response.Error(w, r, response.ErrAlbumNotFound)
		return
	}

	a, err := h.albums.GetByDirectToken(r.Context(), token)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Map to direct album
	directAlbum := ToDirect(a)

	h.response.SuccessDataOnly(w, r, directAlbum)
}

/* Get album list by user slug */
func (h *Handler) AvailableList(w http.ResponseWriter, r *http.Request) {
	userSlug := chi.URLParam(r, "slug")
	// Get claims from context
	claims, _ := tokens.GetClaimsFromContext(r.Context())
	// Get user by slug
	user, err := h.users.GetAvailableBySlug(r.Context(), userSlug)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	// Parse pagination parameters
	query := r.URL.Query()
	cursor := query.Get("cursor")
	limit := h.albumsPerPage
	if limitParam := query.Get("limit"); limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil {
			limit = parsed
		}
	}

	var a []AlbumResponse
	var nextCursor string
	if claims.UserID == uuid.Nil || claims.UserID != user.ID {
		// Get public album list
		a, nextCursor, err = h.albums.ListAvailable(r.Context(), user.ID, claims.Email, cursor, limit)
	} else {
		// Get owned album list
		a, nextCursor, err = h.albums.ListOwned(r.Context(), claims.UserID, cursor, limit)
	}

	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Return albums
	h.response.Paginated(w, r, a, nextCursor)
}

/* Get authenticated user's album list */
func (h *Handler) OwnedList(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Parse pagination parameters
	query := r.URL.Query()
	cursor := query.Get("cursor")
	limit := h.albumsPerPage
	if limitParam := query.Get("limit"); limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil {
			limit = parsed
		}
	}

	// Get list of owned albums
	albums, nextCursor, err := h.albums.ListOwned(r.Context(), claims.UserID, cursor, limit)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	h.response.Paginated(w, r, albums, nextCursor)
}

/* Get authenticated user's list of deleted albums */
func (h *Handler) TrashedList(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Parse pagination parameters
	query := r.URL.Query()
	cursor := query.Get("cursor")
	limit := h.albumsPerPage
	if limitParam := query.Get("limit"); limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil {
			limit = parsed
		}
	}

	// Get owned list of deleted albums
	albums, nextCursor, err := h.albums.ListTrashed(r.Context(), claims.UserID, cursor, limit)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}
	h.response.Paginated(w, r, albums, nextCursor)
}

/* Create album */
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// JSON decode
	input := struct {
		Title        string       `json:"title" validate:"required,min=2,max=255"`
		Atlas        types.Atlas  `json:"atlas" validate:"required,min=1,dive"`
		Access       types.Access `json:"access" validate:"required"`
		SharedEmails []string     `json:"shared_emails"`
		Slug         string       `json:"slug" validate:"required,min=3,max=255,slug"`
		Cover        string       `json:"cover" validate:"required,url"`
		DateAt       time.Time    `json:"date_at" validate:"required"`
		IsActive     bool         `json:"is_active"`
	}{}
	if err := request.DecodeJSONBody(w, r, &input); err != nil {
		h.response.Error(w, r, response.ErrBadJSON.Wrap(err))
		return
	}

	// Validate input
	if err := h.validator.Struct(&input); err != nil {
		h.response.ValidationError(w, r, err)
		return
	}

	// Create album
	album, err := h.albums.Create(r.Context(), claims.UserID, input.Title, input.Slug, input.Cover, input.Atlas, input.Access, input.SharedEmails, input.IsActive, input.DateAt)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.SuccessWithData(w, r, response.SuccessAlbumCreated, album)
}

/* Update album */
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Album id from url
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// JSON decode
	input := struct {
		Title        string       `json:"title" validate:"required,min=2,max=255"`
		Atlas        types.Atlas  `json:"atlas" validate:"required,min=1,dive"`
		Access       types.Access `json:"access" validate:"required"`
		SharedEmails []string     `json:"shared_emails"`
		Slug         string       `json:"slug" validate:"required,min=3,max=255,slug"`
		Cover        string       `json:"cover" validate:"required,url"`
		DateAt       time.Time    `json:"date_at" validate:"required"`
		IsActive     bool         `json:"is_active"`
	}{}
	if err := request.DecodeJSONBody(w, r, &input); err != nil {
		h.response.Error(w, r, response.ErrBadJSON.Wrap(err))
		return
	}

	// Validate input
	if err := h.validator.Struct(&input); err != nil {
		h.response.ValidationError(w, r, err)
		return
	}

	// Update album
	album, err := h.albums.Update(r.Context(), claims.UserID, albumID, input.Title, input.Slug, input.Cover, input.Atlas, input.Access, input.SharedEmails, input.DateAt, input.IsActive)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.SuccessWithData(w, r, response.SuccessAlbumUpdated, album)
}

/* Generate direct token */
func (h *Handler) GenerateDirectToken(w http.ResponseWriter, r *http.Request) {
	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Album id from url
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// Generate direct token
	token, err := h.albums.GenerateDirectToken(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.SuccessWithData(w, r, response.SuccessDirectTokenGenerated, map[string]string{
		"direct_token": token.UUID.String(),
	})
}

/* Revoke direct token */
func (h *Handler) RevokeDirectToken(w http.ResponseWriter, r *http.Request) {
	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Album id from url
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// Generate direct token
	err = h.albums.RevokeDirectToken(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.Success(w, r, response.SuccessDirectTokenRevoked)
}

/* Update album */
func (h *Handler) ToggleActive(w http.ResponseWriter, r *http.Request) {
	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Album id from url
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// JSON decode
	input := struct {
		IsActive bool `json:"is_active"`
	}{}
	if err := request.DecodeJSONBody(w, r, &input); err != nil {
		h.response.Error(w, r, response.ErrBadJSON.Wrap(err))
		return
	}

	// Validate input
	if err := h.validator.Struct(&input); err != nil {
		h.response.ValidationError(w, r, err)
		return
	}

	// Update album
	album, err := h.albums.ToggleActive(r.Context(), claims.UserID, albumID, input.IsActive)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	if input.IsActive {
		h.response.SuccessWithData(w, r, response.SuccessAlbumActive, album)
	} else {
		h.response.SuccessWithData(w, r, response.SuccessAlbumInactive, album)
	}
}

/* Delete album */
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	// Parse UUID
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Delete album
	_, err = h.albums.Delete(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.Success(w, r, response.SuccessAlbumDeleted)
}

/* Restore deleted album */
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	// Parse UUID
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Restore deleted album
	_, err = h.albums.Restore(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.Success(w, r, response.SuccessAlbumRestored)
}

/* Purge deleted album */
func (h *Handler) Purge(w http.ResponseWriter, r *http.Request) {
	// Parse UUID
	idStr := chi.URLParam(r, "uuid")
	albumID, err := uuid.Parse(idStr)
	if err != nil {
		h.response.Error(w, r, response.ErrBadUUID.Wrap(err))
		return
	}

	// Get user claims
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}

	// Purge deleted album
	_, err = h.albums.Purge(r.Context(), claims.UserID, albumID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.Success(w, r, response.SuccessAlbumPurged)
}
