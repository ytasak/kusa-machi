-- name: CountLikesSent :one
-- Today's spent like budget. Like rows always reference today's personas
-- because yesterday's persona can never act again, so no date filter is needed.
SELECT COUNT(*) FROM likes WHERE from_persona_id = $1;

-- name: CountLikesReceived :one
SELECT COUNT(*) FROM likes WHERE to_persona_id = $1;

-- name: HasLikesReceivedSince :one
SELECT EXISTS (
    SELECT 1 FROM likes
    WHERE to_persona_id = sqlc.arg('persona_id')
      AND created_at > COALESCE(sqlc.narg('since')::timestamptz, 'epoch'::timestamptz)
) AS has_unseen;

-- name: LikeExists :one
SELECT EXISTS (
    SELECT 1 FROM likes
    WHERE from_persona_id = sqlc.arg('from_persona_id')
      AND to_persona_id = sqlc.arg('to_persona_id')
) AS like_exists;

-- name: InsertLike :one
INSERT INTO likes (id, from_persona_id, to_persona_id)
VALUES (sqlc.arg('id'), sqlc.arg('from_persona_id'), sqlc.arg('to_persona_id'))
ON CONFLICT (from_persona_id, to_persona_id) DO NOTHING
RETURNING id;
