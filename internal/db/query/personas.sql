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
SET like_balance = like_balance + sqlc.arg('amount'),
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
SET like_balance = like_balance + sqlc.arg('amount'),
    match_reward_count = match_reward_count + 1
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ConsumeLike :one
-- Like 1つの消費。残高を1つ減らし、あわせて時間回復のタイマーを開始する。
--
-- 起点を入れるのは初回だけ（COALESCE）。2つ目以降の Like で起点が今に
-- 進んでしまうと、Like を使うほど回復が遠のくことになる。
UPDATE personas
SET like_balance = like_balance - 1,
    like_recovery_anchor_at = COALESCE(like_recovery_anchor_at, sqlc.arg('now'))
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ApplyTimeRecovery :one
-- 時間回復の付与。回数の上限は無いので、数えるのは残数と起点だけ。
--
-- amount は所持上限で切り詰めた「実際に増える数」。経過した3時間のうち上限で
-- 受け取れなかった分は失われるため、amount が 0 のまま起点だけが進むことも
-- ある（満タンのまま3時間が過ぎた場合）。
--
-- 起点は呼び出し側が計算した値で上書きする。経過した3時間単位ぶんだけ進み、
-- 3時間に満たない余りはそのまま残る。時刻の計算をすべて clock 抽象の側に
-- 寄せておきたいので、SQL では interval を足さない。
UPDATE personas
SET like_balance = like_balance + sqlc.arg('amount'),
    like_recovery_anchor_at = sqlc.arg('anchor_at')
WHERE id = sqlc.arg('id')
RETURNING *;
