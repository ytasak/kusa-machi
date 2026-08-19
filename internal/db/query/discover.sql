-- name: DiscoverCandidates :many
-- Discover 画面の候補選択。除外条件が散らばって食い違わないよう、ルールは
-- すべてここに集約している:
--   自分以外 / 当日のゲーム日 / Like済みでない / Match済みでない /
--   フロントのクールダウン一覧に含まれない。
-- Pass は候補から外さない。何度 Pass した相手でもまた出てくる。
-- 優先度は exposure_count が少ない順、同数ならランダム。
-- バッチを返すだけでは exposure_count を絶対に変えない。
SELECT p.*
FROM personas p
JOIN participants pa ON pa.id = p.participant_id
WHERE pa.game_date = sqlc.arg('game_date')
  AND p.id <> sqlc.arg('self_id')
  AND p.id <> ALL (sqlc.arg('exclude_ids')::uuid[])
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
