ALTER TABLE personas
    ADD COLUMN bonus_likes SMALLINT NOT NULL DEFAULT 0;

-- 残数から回復ぶんを復元する。送信済み Like を足し戻し、初期予算 10 を引く。
UPDATE personas p
SET bonus_likes = GREATEST(0, LEAST(5,
        p.like_balance + (SELECT COUNT(*) FROM likes l WHERE l.from_persona_id = p.id) - 10
    ));

ALTER TABLE personas
    ADD CONSTRAINT personas_bonus_likes_range CHECK (bonus_likes BETWEEN 0 AND 5);

ALTER TABLE personas
    DROP CONSTRAINT IF EXISTS personas_like_balance_range,
    DROP CONSTRAINT IF EXISTS personas_time_recovery_count_range,
    DROP COLUMN IF EXISTS like_balance,
    DROP COLUMN IF EXISTS time_recovery_count,
    DROP COLUMN IF EXISTS like_recovery_anchor_at;
