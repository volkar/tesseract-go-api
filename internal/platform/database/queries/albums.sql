-- name: GetAlbumBySlug :one
SELECT * FROM albums
WHERE user_id = @user_id AND slug = @album_slug AND deleted_at IS NULL;

-- name: GetAlbum :one
SELECT * FROM albums
WHERE id = @album_id AND user_id = @user_id;

-- name: GetAlbumByDirectToken :one
SELECT * FROM albums
WHERE direct_token = @direct_token AND is_active AND deleted_at IS NULL;

-- name: ListAvailableAlbumIDs :many
SELECT id, date_at
FROM albums
WHERE user_id = @user_id
  AND deleted_at IS NULL
  AND (
      (access = 'public' AND is_active)
      OR (access = 'shared' AND is_active AND @viewer_email::text = ANY(shared_emails))
  )
  AND (
      sqlc.narg('cursor_date_at')::timestamptz IS NULL
      OR date_at < sqlc.narg('cursor_date_at')::timestamptz
      OR (date_at = sqlc.narg('cursor_date_at')::timestamptz AND id < sqlc.narg('cursor_id')::uuid)
  )
ORDER BY date_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ListOwnedAlbumIDs :many
SELECT id, date_at
FROM albums
WHERE user_id = @user_id
  AND deleted_at IS NULL
  AND (
      sqlc.narg('cursor_date_at')::timestamptz IS NULL
      OR date_at < sqlc.narg('cursor_date_at')::timestamptz
      OR (date_at = sqlc.narg('cursor_date_at')::timestamptz AND id < sqlc.narg('cursor_id')::uuid)
  )
ORDER BY date_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ListTrashedAlbumIDs :many
SELECT id, date_at
FROM albums
WHERE user_id = @user_id
  AND deleted_at IS NOT NULL
  AND (
      sqlc.narg('cursor_date_at')::timestamptz IS NULL
      OR date_at < sqlc.narg('cursor_date_at')::timestamptz
      OR (date_at = sqlc.narg('cursor_date_at')::timestamptz AND id < sqlc.narg('cursor_id')::uuid)
  )
ORDER BY date_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: GetAlbumsByIDs :many
SELECT *
FROM albums a
WHERE id = ANY(@ids::uuid[]);

-- name: CreateAlbum :one
INSERT INTO albums (user_id, title, slug, cover, atlas, access, is_active, shared_emails, date_at)
SELECT u.id, @title, @slug, @cover, @atlas, @access, @is_active, @shared_emails, @date_at
FROM users u
WHERE u.id = @user_id AND u.deleted_at IS NULL
RETURNING *;

-- name: UpdateAlbum :one
WITH old_data AS (
  SELECT a.id, a.slug AS old_slug, u.slug AS user_slug
  FROM albums a
  JOIN users u ON a.user_id = u.id
  WHERE a.id = @album_id AND a.user_id = @user_id AND a.deleted_at IS NULL AND u.deleted_at IS NULL
  FOR UPDATE OF a
)
UPDATE albums
SET
  title = @title,
  slug = @slug,
  cover = @cover,
  atlas = @atlas,
  access = @access,
  shared_emails = @shared_emails,
  date_at = @date_at,
  is_active = @is_active,
  updated_at = NOW()
FROM old_data
WHERE albums.id = old_data.id
RETURNING sqlc.embed(albums), old_data.old_slug, old_data.user_slug;

-- name: UpdateAlbumDirectToken :one
WITH old_data AS (
  SELECT a.id, a.direct_token AS old_direct_token
  FROM albums a
  JOIN users u ON a.user_id = u.id
  WHERE a.id = @album_id AND a.user_id = @user_id AND a.deleted_at IS NULL AND u.deleted_at IS NULL
  FOR UPDATE OF a
)
UPDATE albums
SET
  direct_token = @direct_token,
  updated_at = NOW()
FROM old_data
WHERE albums.id = old_data.id
RETURNING sqlc.embed(albums), old_data.old_direct_token;

-- name: ToggleAlbumActive :one
UPDATE albums a
SET
    is_active = @is_active,
    updated_at = NOW()
FROM users u
WHERE a.user_id = u.id
  AND a.id = @album_id
  AND a.user_id = @user_id
  AND a.deleted_at IS NULL
  AND u.deleted_at IS NULL
RETURNING sqlc.embed(a);

-- name: SoftDeleteAlbum :one
WITH old_data AS (
  SELECT a.id, u.slug AS user_slug
  FROM albums a
  JOIN users u ON a.user_id = u.id
  WHERE a.id = @album_id AND a.user_id = @user_id AND a.deleted_at IS NULL AND u.deleted_at IS NULL
  FOR UPDATE OF a
)
UPDATE albums
SET deleted_at = NOW(), updated_at = NOW()
FROM old_data
WHERE albums.id = old_data.id
RETURNING sqlc.embed(albums), old_data.user_slug;

-- name: RestoreAlbum :one
UPDATE albums a
SET
  deleted_at = NULL,
  updated_at = NOW(),
  slug = CASE
    WHEN EXISTS (
      SELECT 1 FROM albums AS a_active
      WHERE a_active.user_id = a.user_id
        AND a_active.slug = a.slug
        AND a_active.deleted_at IS NULL
        AND a_active.id != a.id
    )
    THEN left(a.slug, 246) || '-' || substring(md5(random()::text) from 1 for 8)
    ELSE a.slug
  END
FROM users u
WHERE a.user_id = u.id
  AND a.id = @album_id
  AND a.user_id = @user_id
  AND a.deleted_at IS NOT NULL
  AND u.deleted_at IS NULL
RETURNING a.id;

-- name: HardDeleteAlbum :one
DELETE FROM albums a
USING users u
WHERE a.user_id = u.id
  AND a.id = @album_id
  AND a.user_id = @user_id
  AND a.deleted_at IS NOT NULL
  AND u.deleted_at IS NULL
RETURNING a.id;