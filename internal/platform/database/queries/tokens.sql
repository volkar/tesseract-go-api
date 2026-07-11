-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (user_id, token_hash, expires_at, ip, ua, location)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetRefreshTokenByHash :one
SELECT * FROM refresh_tokens
WHERE token_hash = $1
LIMIT 1;

-- name: ConsumeRefreshTokenByHash :execresult
UPDATE refresh_tokens
SET is_consumed = true, updated_at = NOW()
WHERE token_hash = $1 AND is_consumed = false;

-- name: GetActiveRefreshTokensForUser :many
SELECT *
FROM refresh_tokens
WHERE user_id = $1 AND is_consumed = false AND expires_at > NOW()
ORDER BY created_at DESC;

-- name: DeleteRefreshTokenByID :one
DELETE FROM refresh_tokens
WHERE id = $1 AND user_id = $2
RETURNING token_hash;

-- name: DeleteOtherRefreshTokensForUser :exec
DELETE FROM refresh_tokens
WHERE user_id = $1 AND token_hash != $2;

-- name: DeleteAllRefreshTokensForUser :exec
DELETE FROM refresh_tokens WHERE user_id = $1;

-- name: CleanupRefreshTokens :exec
DELETE FROM refresh_tokens
WHERE expires_at < NOW()
   OR (is_consumed = true AND updated_at < NOW() - INTERVAL '24 hours');