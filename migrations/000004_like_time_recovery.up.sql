-- Like の時間回復。3時間ごとに +1、1日2回まで（§8 を参照）。
--
-- 状態は participants ではなく personas に置く。000003 と同じ理由で、
-- persona 行はゲーム日ごとに作り直されるため 0:00 のリセットをデフォルト値が
-- そのまま担い、Like のトランザクションがすでに取っている FOR UPDATE に
-- 相乗りできる。

-- 残数を明示的な状態にする。これまでは 10 + bonus_likes - 送信済み Like で
-- 導出していたが、時間回復は「Like を送っていないのに残数が増える」ため、
-- 送信数からの導出では表せない。
--
-- like_recovery_anchor_at は次の回復を計る起点。初めて Like を消費した時刻が
-- 入り、以降は回復を1つ付与するたびに3時間進む。NULL は「まだ Like を
-- 使っていない」を意味し、この状態ではタイマーが動かない。
ALTER TABLE personas
    ADD COLUMN like_balance SMALLINT NOT NULL DEFAULT 10,
    ADD COLUMN time_recovery_count SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN like_recovery_anchor_at TIMESTAMPTZ NULL;

-- 当日の Persona が持っていた残数を引き継ぐ。0:00 を待たずに配信しても、
-- その日の Like が 10 に巻き戻らないようにするため。
UPDATE personas p
SET like_balance = GREATEST(0, LEAST(12,
        10 + p.bonus_likes - (SELECT COUNT(*) FROM likes l WHERE l.from_persona_id = p.id)
    ));

ALTER TABLE personas
    DROP CONSTRAINT IF EXISTS personas_bonus_likes_range,
    DROP COLUMN IF EXISTS bonus_likes;

-- 取りうる値の範囲は DB にも書いておく。上限そのものはアプリケーションが強制する。
ALTER TABLE personas
    ADD CONSTRAINT personas_like_balance_range CHECK (like_balance BETWEEN 0 AND 12),
    ADD CONSTRAINT personas_time_recovery_count_range CHECK (time_recovery_count BETWEEN 0 AND 2);
