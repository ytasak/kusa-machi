-- name: CountMatches :one
SELECT COUNT(*) FROM matches
WHERE persona_low_id = sqlc.arg('persona_id') OR persona_high_id = sqlc.arg('persona_id');

-- name: HasMatchesSince :one
SELECT EXISTS (
    SELECT 1 FROM matches
    WHERE (persona_low_id = sqlc.arg('persona_id') OR persona_high_id = sqlc.arg('persona_id'))
      AND created_at > COALESCE(sqlc.narg('since')::timestamptz, 'epoch'::timestamptz)
) AS has_unseen;

-- name: InsertMatch :one
-- Idempotent match creation on the normalised pair. The no-op DO UPDATE returns
-- the existing row so a retry yields the same match id instead of an error.
INSERT INTO matches (id, persona_low_id, persona_high_id)
VALUES (sqlc.arg('id'), sqlc.arg('persona_low_id'), sqlc.arg('persona_high_id'))
ON CONFLICT (persona_low_id, persona_high_id) DO UPDATE
SET persona_low_id = matches.persona_low_id
RETURNING id;
