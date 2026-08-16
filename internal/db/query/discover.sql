-- name: DiscoverCandidates :many
-- Candidate selection for the discover screen. Every rule lives here so the
-- exclusions cannot drift apart from each other:
--   not self / today's game day / not already liked / not already matched /
--   pass_count < 3 / not in the frontend's cooldown list.
-- Priority is the least exposed persona first, then random among equals.
-- Returning a batch must never change exposure_count.
SELECT p.*
FROM personas p
JOIN participants pa ON pa.id = p.participant_id
LEFT JOIN passes ps
       ON ps.from_persona_id = sqlc.arg('self_id')
      AND ps.to_persona_id = p.id
WHERE pa.game_date = sqlc.arg('game_date')
  AND p.id <> sqlc.arg('self_id')
  AND p.id <> ALL (sqlc.arg('exclude_ids')::uuid[])
  AND COALESCE(ps.pass_count, 0) < 3
  AND NOT EXISTS (
      SELECT 1 FROM likes l
      WHERE l.from_persona_id = sqlc.arg('self_id')
        AND l.to_persona_id = p.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM matches m
      WHERE (m.persona_low_id = sqlc.arg('self_id') AND m.persona_high_id = p.id)
         OR (m.persona_low_id = p.id AND m.persona_high_id = sqlc.arg('self_id'))
  )
ORDER BY p.exposure_count ASC, random()
LIMIT sqlc.arg('limit_count');
