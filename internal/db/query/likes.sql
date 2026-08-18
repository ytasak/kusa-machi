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
