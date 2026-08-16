-- name: UpsertPass :one
-- First pass inserts, later passes increment. The conditional DO UPDATE means a
-- fourth pass updates nothing and returns no row, which the caller reports as
-- PassLimitReached instead of silently capping.
INSERT INTO passes (id, from_persona_id, to_persona_id)
VALUES (sqlc.arg('id'), sqlc.arg('from_persona_id'), sqlc.arg('to_persona_id'))
ON CONFLICT (from_persona_id, to_persona_id) DO UPDATE
SET pass_count = passes.pass_count + 1,
    last_passed_at = NOW()
WHERE passes.pass_count < 3
RETURNING pass_count;

-- name: GetPassCount :one
SELECT pass_count FROM passes
WHERE from_persona_id = $1 AND to_persona_id = $2;
