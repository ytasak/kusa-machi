-- name: CountMatches :one
SELECT COUNT(*) FROM matches
WHERE persona_low_id = sqlc.arg('persona_id') OR persona_high_id = sqlc.arg('persona_id');

-- name: HasMatchesSince :one
SELECT EXISTS (
    SELECT 1 FROM matches
    WHERE (persona_low_id = sqlc.arg('persona_id') OR persona_high_id = sqlc.arg('persona_id'))
      AND created_at > COALESCE(sqlc.narg('since')::timestamptz, 'epoch'::timestamptz)
) AS has_unseen;
