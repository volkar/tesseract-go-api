-- name: GetAvailableUser :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetAvailableUserBySlug :one
SELECT * FROM users WHERE slug = $1 AND deleted_at IS NULL;

-- name: UpsertUser :one
INSERT INTO users (email, username, avatar)
VALUES ($1, $2, $3)
ON CONFLICT (email)
DO UPDATE SET
    avatar = EXCLUDED.avatar
WHERE users.deleted_at IS NULL
RETURNING *;

-- name: UpdateUser :one
WITH old_data AS (
    SELECT id, slug FROM users
    WHERE users.id = $1 AND users.deleted_at IS NULL
    FOR UPDATE
) UPDATE users SET
    username = $2,
    slug = $3,
    updated_at = NOW()
FROM old_data
WHERE users.id = old_data.id
RETURNING sqlc.embed(users), old_data.slug AS old_slug;

-- name: CreateUser :one
INSERT INTO users (email, username, role, slug)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: SoftDeleteUser :one
UPDATE users
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = @user_id AND deleted_at IS NULL
RETURNING id;

-- name: RestoreUser :one
UPDATE users
SET
    deleted_at = NULL,
    updated_at = NOW(),
    slug = CASE
        WHEN EXISTS (
            SELECT 1 FROM users AS u_active
            WHERE u_active.slug = users.slug
              AND u_active.deleted_at IS NULL
              AND u_active.id != users.id
        )
        THEN left(users.slug, 246) || '-' || substring(md5(random()::text) from 1 for 8)
        ELSE users.slug
    END
WHERE users.id = @user_id AND users.deleted_at IS NOT NULL
RETURNING id, slug;

-- name: HardDeleteUser :one
DELETE FROM users WHERE id = @user_id RETURNING id;