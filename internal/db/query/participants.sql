-- name: UpsertParticipant :one
-- Race-safe "ensure today's participant". The no-op DO UPDATE makes the
-- existing row visible to RETURNING when a concurrent request won the insert.
INSERT INTO participants (id, cookie_token, game_date, csrf_token)
VALUES ($1, $2, $3, $4)
ON CONFLICT (cookie_token, game_date)
DO UPDATE SET cookie_token = participants.cookie_token
RETURNING *;

-- name: GetParticipant :one
SELECT * FROM participants
WHERE cookie_token = $1 AND game_date = $2;

-- name: GetParticipantByCookieAndCSRF :one
SELECT * FROM participants
WHERE cookie_token = $1 AND csrf_token = $2;

-- name: MarkLikesSeen :exec
UPDATE participants SET likes_last_seen_at = NOW() WHERE id = $1;

-- name: MarkMatchesSeen :exec
UPDATE participants SET matches_last_seen_at = NOW() WHERE id = $1;

-- name: DeleteParticipantsBefore :execrows
DELETE FROM participants WHERE game_date < $1;
