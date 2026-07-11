package response

import (
	"fmt"
	"net/http"
)

// Response errors

type AppError struct {
	Code  int    `json:"-"`
	Slug  string `json:"slug"`
	Cause error  `json:"-"`
}

/* Return error message */
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Slug, e.Cause)
	}
	return e.Slug
}

/* Wrap original error */
func (e *AppError) Wrap(err error) *AppError {
	return &AppError{
		Code:  e.Code,
		Slug:  e.Slug,
		Cause: err,
	}
}

/* Unwrap error */
func (e *AppError) Unwrap() error {
	return e.Cause
}

/* Change error status code */
func (e *AppError) ChangeStatus(status int) {
	e.Code = status
}

var (
	ErrBadJSON            = &AppError{Code: http.StatusBadRequest, Slug: "bad_json"}                     /* 400 Bad Request */
	ErrInvalidImageURL    = &AppError{Code: http.StatusBadRequest, Slug: "invalid_image_url"}            /* 400 Bad Request */
	ErrRequestTooLarge    = &AppError{Code: http.StatusRequestEntityTooLarge, Slug: "request_too_large"} /* 413 Request Too Large */
	ErrBadUUID            = &AppError{Code: http.StatusBadRequest, Slug: "bad_uuid"}                     /* 400 Bad Request */
	ErrBadProvider        = &AppError{Code: http.StatusBadRequest, Slug: "bad_provider"}                 /* 400 Bad Request */
	ErrAccessTokenExpired = &AppError{Code: http.StatusUnauthorized, Slug: "access_token_expired"}       /* 401 Unauthorized */
	ErrNoClaims           = &AppError{Code: http.StatusUnauthorized, Slug: "no_claims"}                  /* 401 Unauthorized */
	ErrNoPermission       = &AppError{Code: http.StatusForbidden, Slug: "no_permission"}                 /* 403 Forbidden */
	ErrTokenConsumed      = &AppError{Code: http.StatusUnauthorized, Slug: "token_consumed"}             /* 401 Unauthorized */
	ErrTokenExpired       = &AppError{Code: http.StatusUnauthorized, Slug: "token_expired"}              /* 401 Unauthorized */
	ErrTokenGracePeriod   = &AppError{Code: http.StatusOK, Slug: "token_grace_period"}                   /* 200 OK */
	ErrNoRefreshToken     = &AppError{Code: http.StatusForbidden, Slug: "no_refresh_token"}              /* 403 Forbidden */
	ErrRefreshSession     = &AppError{Code: http.StatusForbidden, Slug: "refresh_session"}               /* 403 Forbidden */
	ErrOAuthNoEmail       = &AppError{Code: http.StatusBadRequest, Slug: "oauth_no_email"}               /* 400 Bad Request */
	ErrOAuthNoName        = &AppError{Code: http.StatusBadRequest, Slug: "oauth_no_name"}                /* 400 Bad Request */
	ErrAlbumNotFound      = &AppError{Code: http.StatusNotFound, Slug: "album_not_found"}                /* 404 Not Found */
	ErrAlbumsNotFound     = &AppError{Code: http.StatusNotFound, Slug: "albums_not_found"}               /* 404 Not Found */
	ErrAlbumSlugExists    = &AppError{Code: http.StatusConflict, Slug: "album_slug_exists"}              /* 409 Conflict */
	ErrUserNotFound       = &AppError{Code: http.StatusNotFound, Slug: "user_not_found"}                 /* 404 Not Found */
	ErrUserSlugExists     = &AppError{Code: http.StatusConflict, Slug: "user_slug_exists"}               /* 409 Conflict */
	ErrTooManyRequests    = &AppError{Code: http.StatusTooManyRequests, Slug: "too_many_requests"}       /* 429 Too Many Requests */
	ErrNoSecurityHeader   = &AppError{Code: http.StatusForbidden, Slug: "no_security_header"}            /* 403 Forbidden */
	ErrInvalidOrigin      = &AppError{Code: http.StatusForbidden, Slug: "invalid_origin"}                /* 403 Forbidden */
	ErrInvalidCursor      = &AppError{Code: http.StatusBadRequest, Slug: "invalid_cursor"}               /* 400 Bad Request */
	ErrUnknown            = &AppError{Code: http.StatusInternalServerError, Slug: "unknown"}             /* 500 Internal Server Error */
)

// Response messages

type AppSuccess struct {
	Code int    `json:"-"`
	Slug string `json:"slug"`
}

var (
	Success                     = &AppSuccess{Code: http.StatusOK, Slug: "success"}                /* 200 OK */
	SuccessAlbumCreated         = &AppSuccess{Code: http.StatusCreated, Slug: "album_created"}     /* 201 Created */
	SuccessAlbumUpdated         = &AppSuccess{Code: http.StatusOK, Slug: "album_updated"}          /* 200 OK */
	SuccessAlbumActive          = &AppSuccess{Code: http.StatusOK, Slug: "album_active"}           /* 200 OK */
	SuccessAlbumInactive        = &AppSuccess{Code: http.StatusOK, Slug: "album_inactive"}         /* 200 OK */
	SuccessDirectTokenGenerated = &AppSuccess{Code: http.StatusOK, Slug: "direct_token_generated"} /* 200 OK */
	SuccessDirectTokenRevoked   = &AppSuccess{Code: http.StatusOK, Slug: "direct_token_revoked"}   /* 200 OK */
	SuccessAlbumDeleted         = &AppSuccess{Code: http.StatusOK, Slug: "album_deleted"}          /* 200 OK */
	SuccessAlbumRestored        = &AppSuccess{Code: http.StatusOK, Slug: "album_restored"}         /* 200 OK */
	SuccessAlbumPurged          = &AppSuccess{Code: http.StatusOK, Slug: "album_purged"}           /* 200 OK */
	SuccessLoggedOut            = &AppSuccess{Code: http.StatusOK, Slug: "logged_out"}             /* 200 OK */
	SuccessLoggedOutOthers      = &AppSuccess{Code: http.StatusOK, Slug: "logged_out_others"}      /* 200 OK */
	SuccessSessionRevoked       = &AppSuccess{Code: http.StatusOK, Slug: "session_revoked"}        /* 200 OK */
	SuccessUserDeleted          = &AppSuccess{Code: http.StatusOK, Slug: "user_deleted"}           /* 200 OK */
	SuccessUserUpdated          = &AppSuccess{Code: http.StatusOK, Slug: "user_updated"}           /* 200 OK */
	SuccessUserRestored         = &AppSuccess{Code: http.StatusOK, Slug: "user_restored"}          /* 200 OK */
)
