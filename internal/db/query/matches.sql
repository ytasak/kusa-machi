-- name: InsertMatch :one
-- 正規化済みペアに対する冪等な Match 生成。DO UPDATE を無意味な代入にすることで
-- 既存行が返るため、リトライしてもエラーにならず同じ match id が得られる。
INSERT INTO matches (id, persona_low_id, persona_high_id)
VALUES (sqlc.arg('id'), sqlc.arg('persona_low_id'), sqlc.arg('persona_high_id'))
ON CONFLICT (persona_low_id, persona_high_id) DO UPDATE
SET persona_low_id = matches.persona_low_id
RETURNING id;
