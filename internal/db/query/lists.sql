-- name: ListReceivedLikes :many
-- Personas who liked the current persona, newest first. No timestamps and no
-- sender budget information are exposed.
SELECT p.*
FROM likes l
JOIN personas p ON p.id = l.from_persona_id
WHERE l.to_persona_id = sqlc.arg('persona_id')
ORDER BY l.created_at DESC, l.id DESC;

-- name: ListSentLikes :many
-- The day's like allocation history. Matched targets stay in the list and are
-- flagged so the screen can show a MATCH badge.
SELECT p.*, EXISTS (
    SELECT 1 FROM matches m
    WHERE (m.persona_low_id = sqlc.arg('persona_id') AND m.persona_high_id = p.id)
       OR (m.persona_low_id = p.id AND m.persona_high_id = sqlc.arg('persona_id'))
) AS matched
FROM likes l
JOIN personas p ON p.id = l.to_persona_id
WHERE l.from_persona_id = sqlc.arg('persona_id')
ORDER BY l.created_at DESC, l.id DESC;

-- name: ListMatches :many
-- Only the counterpart persona of each match, newest first.
SELECT p.*
FROM matches m
JOIN personas p ON p.id = CASE
    WHEN m.persona_low_id = sqlc.arg('persona_id') THEN m.persona_high_id
    ELSE m.persona_low_id
END
WHERE m.persona_low_id = sqlc.arg('persona_id')
   OR m.persona_high_id = sqlc.arg('persona_id')
ORDER BY m.created_at DESC, m.id DESC;
