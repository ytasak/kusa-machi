-- name: GetMatchForPersona :one
-- 指定 Match を、それが「自分が含まれる当日の Match」である場合だけ返す。
-- 相手 Persona も一緒に取るので、Match 詳細はこれ1本で組み立てられる。
--
-- 3つの確認をこの1本にまとめている:
--   - Match が存在する
--   - 自分がそのペアに含まれている（WHERE 節の OR）
--   - 当日有効である（相手 participant の game_date）
-- 呼び出し側の persona_id は当日の participant から引いたものなので、
-- 相手側の game_date を見れば両者が当日であることが決まる。
SELECT
    m.id AS match_id,
    sqlc.embed(cp)
FROM matches m
JOIN personas cp ON cp.id = CASE
    WHEN m.persona_low_id = sqlc.arg('persona_id') THEN m.persona_high_id
    ELSE m.persona_low_id
END
JOIN participants pa ON pa.id = cp.participant_id
WHERE m.id = sqlc.arg('match_id')
  AND (m.persona_low_id = sqlc.arg('persona_id') OR m.persona_high_id = sqlc.arg('persona_id'))
  AND pa.game_date = sqlc.arg('game_date');

-- name: GetMatchChild :one
SELECT * FROM match_children WHERE match_id = sqlc.arg('match_id');

-- name: InsertMatchChild :one
-- 冪等な子ガチャの保存。1 Match につき子は1人だけなので、同時実行で
-- INSERT に負けた側には保存済みの行がそのまま返る。よって多重クリックでも
-- 再送でも引き直しは起きず、全員が同じ子を見る。
--
-- DO UPDATE を無意味な代入にしているのは、Persona / Match の生成と同じ理由。
-- DO NOTHING にすると負けた側の RETURNING が空になり、呼び出し側が
-- 読み直しの分岐を持たなければならなくなる。
INSERT INTO match_children (
    id, match_id, gender, height_cm, education, occupation, annual_income
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (match_id)
DO UPDATE SET match_id = match_children.match_id
RETURNING *;
