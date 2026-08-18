-- name: InsertMatch :one
-- 正規化済みペアに対する冪等な Match 生成。DO UPDATE を無意味な代入にすることで
-- 既存行が返るため、リトライしてもエラーにならず同じ match id が得られる。
INSERT INTO matches (id, persona_low_id, persona_high_id)
VALUES (sqlc.arg('id'), sqlc.arg('persona_low_id'), sqlc.arg('persona_high_id'))
ON CONFLICT (persona_low_id, persona_high_id) DO UPDATE
SET persona_low_id = matches.persona_low_id
RETURNING id;

-- name: MatchExists :one
-- 正規化済みペアの Match がすでにあるか。Match 報酬を同じ Match に対して
-- 二度払わないための判定に使う。
SELECT EXISTS (
    SELECT 1 FROM matches
    WHERE persona_low_id = sqlc.arg('persona_low_id')
      AND persona_high_id = sqlc.arg('persona_high_id')
) AS match_exists;
