-- name: CountLikesSent :one
-- 当日消費した Like 数。前日の Persona は二度と行動できないため likes の行は
-- 常に当日の Persona を指す。したがって日付での絞り込みは不要。
SELECT COUNT(*) FROM likes WHERE from_persona_id = $1;

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
