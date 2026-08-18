-- name: InsertPersona :one
-- 冪等な Persona 生成。1 Participant につき Persona は1つだけなので、
-- 同時実行で負けた側には保存済みの行がそのまま返る。
INSERT INTO personas (
    id, participant_id, age, gender, height_cm, education, occupation, annual_income
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (participant_id)
DO UPDATE SET participant_id = personas.participant_id
RETURNING *;

-- name: GetPersonaByParticipant :one
SELECT * FROM personas WHERE participant_id = $1;

-- name: GetPersonaByID :one
SELECT * FROM personas WHERE id = $1;

-- name: GetActivePersona :one
-- Persona が操作対象として有効なのは、その Persona 自身のゲーム日のみ。
SELECT p.* FROM personas p
JOIN participants pa ON pa.id = p.participant_id
WHERE p.id = sqlc.arg('persona_id') AND pa.game_date = sqlc.arg('game_date');

-- name: LockPersona :one
-- Like / Pass / プロフィール更新のトランザクションを直列化する。Like と Pass は
-- 必ずペアを正規化した (low, high) の順でロックする。これにより Like 予算の
-- 正しさと相互Like検出の確実性を同時に満たし、かつデッドロックが起きない。
--
-- 行全体を返すのは、報酬の受け取り状態をロックの内側で読み直すため。
-- トランザクションの外で読んだ persona は、並行して回復が入っていると古い。
SELECT * FROM personas WHERE id = $1 FOR UPDATE;

-- name: IncrementExposure :exec
-- exposure は「実際に評価されたプロフィール」を数える。よって Like か Pass が
-- 成功した後だけ実行し、Discover でバッチを返したときには実行しない。
UPDATE personas SET exposure_count = exposure_count + 1 WHERE id = $1;

-- name: UpdatePersonaProfile :one
UPDATE personas
SET name = $2, hobby = $3, bio = $4
WHERE id = $1
RETURNING *;

-- name: SetPersonaPhoto :one
UPDATE personas SET photo_updated_at = NOW() WHERE id = $1 RETURNING photo_updated_at;

-- name: ClearPersonaPhoto :exec
UPDATE personas SET photo_updated_at = NULL WHERE id = $1;

-- name: GetHomeState :one
-- ホーム（マイページ）に必要な情報を1往復で取得する。
-- 以前は Persona 取得と5つの集計で6回の往復をしていた。
SELECT
    sqlc.embed(p),
    (SELECT COUNT(*) FROM likes l WHERE l.from_persona_id = p.id) AS likes_sent,
    (SELECT COUNT(*) FROM likes l WHERE l.to_persona_id = p.id) AS likes_received,
    (
        SELECT COUNT(*) FROM matches m
        WHERE m.persona_low_id = p.id OR m.persona_high_id = p.id
    ) AS match_count,
    EXISTS (
        SELECT 1 FROM likes l
        WHERE l.to_persona_id = p.id
          AND l.created_at > COALESCE(sqlc.narg('likes_last_seen_at')::timestamptz, 'epoch'::timestamptz)
    ) AS has_unseen_likes,
    EXISTS (
        SELECT 1 FROM matches m
        WHERE (m.persona_low_id = p.id OR m.persona_high_id = p.id)
          AND m.created_at > COALESCE(sqlc.narg('matches_last_seen_at')::timestamptz, 'epoch'::timestamptz)
    ) AS has_unseen_matches
FROM personas p
WHERE p.participant_id = sqlc.arg('participant_id');

-- name: ClaimProfileReward :one
-- プロフィール完成報酬の付与。amount は所持上限で切り詰めた「実際に増える数」で、
-- 上限に達していて 0 のときも受け取り済みにする。仕様どおり溢れた分は失われ、
-- 同じ日にもう一度もらえることはない。
--
-- 呼び出し側は LockPersona でこの行を押さえてから、フラグを読んで amount を
-- 決める。よってページ再読み込みや同じ PATCH の再送では二度目の付与が起きない。
UPDATE personas
SET bonus_likes = bonus_likes + sqlc.arg('amount'),
    profile_reward_claimed = TRUE
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ClaimMatchReward :one
-- Match 報酬の付与。回数は1日2回で打ち止めなので、上限判定を済ませた
-- 呼び出し側だけがこれを実行する。amount が 0 でも回数は消費する。
--
-- Like のトランザクションが lockPair でこの行を FOR UPDATE しているため、
-- 同時 Like でも回数が2を超えることはない。
UPDATE personas
SET bonus_likes = bonus_likes + sqlc.arg('amount'),
    match_reward_count = match_reward_count + 1
WHERE id = sqlc.arg('id')
RETURNING bonus_likes;
