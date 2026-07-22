package users

import (
	"api/internal/domain/tokens"
	"api/internal/platform/cookies"
	"api/internal/platform/request"
	"api/internal/platform/response"
	"api/internal/platform/utils"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type UpdateRequest struct {
	Username string `json:"username" validate:"required,min=2,max=255"`
	Slug     string `json:"slug" validate:"required,min=3,max=255,slug,notreserved"`
	Avatar   string `json:"avatar" validate:"max=255"`
}

type Handler struct {
	users     *Service
	response  *response.Response
	validator *validator.Validate
	cookies   *cookies.Manager
}

func NewHandler(service *Service, response *response.Response, val *validator.Validate, cookies *cookies.Manager) *Handler {
	return &Handler{
		users:     service,
		response:  response,
		validator: val,
		cookies:   cookies,
	}
}

/* Get authenticated user info */
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	// Get claims from context
	claims, ok := tokens.GetClaimsFromContext(r.Context())
	if !ok {
		h.response.Error(w, r, response.ErrNoClaims)
		return
	}
	// Get user by ID
	u, err := h.users.GetAvailable(r.Context(), claims.UserID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Map and return
	h.response.SuccessDataOnly(w, r, ToMe(u))
}

/* Get user info by user slug */
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	userSlug := chi.URLParam(r, "slug")
	// Get user
	u, err := h.users.GetAvailableBySlug(r.Context(), userSlug)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Map and return user info
	h.response.SuccessDataOnly(w, r, ToPublic(u))
}

/* Update user info */
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	// Parse UUID
	idStr := chi.URLParam(r, "uuid")
	updatingUserID, err := uuid.Parse(idStr)
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

	// Check if user have permission
	if claims.UserID != updatingUserID {
		h.response.Error(w, r, response.ErrNoPermission)
		return
	}

	var req UpdateRequest

	// JSON decode
	if err := request.DecodeJSONBody(w, r, &req); err != nil {
		h.response.Error(w, r, response.ErrBadJSON.Wrap(err))
		return
	}

	// Validate input
	if err := h.validator.Struct(&req); err != nil {
		h.response.ValidationError(w, r, err)
		return
	}

	avatar := req.Avatar
	// Avatar URL check
	if req.Avatar != "" {
		securedAvatar, secErr := utils.ValidateAndSanitizeURL(req.Avatar)
		if secErr != nil {
			// Return 400 Bad Request to the user
			h.response.Error(w, r, response.ErrInvalidImageURL.Wrap(secErr))
		}
		avatar = securedAvatar
	}
	req.Avatar = avatar

	// Update user
	u, err := h.users.Update(r.Context(), updatingUserID, req)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	h.response.SuccessWithData(w, r, response.SuccessUserUpdated, ToMe(u))
}

/* Delete user */
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	// User id from url
	idStr := chi.URLParam(r, "uuid")
	deletingUserID, err := uuid.Parse(idStr)
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

	// Check if user have permission
	if claims.UserID != deletingUserID {
		h.response.Error(w, r, response.ErrNoPermission)
		return
	}

	// Delete user
	_, err = h.users.Delete(r.Context(), deletingUserID)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	// Delete cookies
	h.cookies.UnsetAccessCookie(w)
	h.cookies.UnsetRefreshCookie(w)

	h.response.Success(w, r, response.SuccessUserDeleted)
}
